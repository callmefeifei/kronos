package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/pstrr/kronos/internal/apiclient"
	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/executor"
	"github.com/pstrr/kronos/internal/notifier"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
)

// StartStdio runs the MCP server over stdio (stdin/stdout).
//
// It probes the daemon health endpoint to decide the operating mode:
//   - proxy mode: daemon is running, proxy all tool calls via the REST API.
//   - embedded mode: no daemon, open the DB directly (read/write, no cron scheduling).
func StartStdio(cfg *config.Config, version string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT / SIGTERM for clean shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	mcpSrv := server.NewMCPServer(
		"kronos",
		version,
		server.WithInstructions("Kronos scheduled task manager. Use these tools to create, manage, and monitor scheduled tasks including reminders, scripts, and agent tasks."),
	)

	daemonUp := probeDaemon(cfg.Server.Port)

	var cleanupFn func()
	if daemonUp {
		// --- Proxy mode ---
		fmt.Fprintf(os.Stderr, "kronos mcp: running in proxy mode\n")

		baseURL := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
		client, err := apiclient.New(baseURL, cfg.Auth.JWTSecret)
		if err != nil {
			return fmt.Errorf("create API client: %w", err)
		}
		h := NewProxyHandler(client)
		registerProxyTools(mcpSrv, h)
	} else {
		// --- Embedded mode ---
		fmt.Fprintf(os.Stderr, "kronos mcp: running in embedded mode\n")

		db, err := store.NewDatabase(cfg.Database)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		cleanupFn = func() {
			_ = db.Close()
		}

		taskStore := store.NewTaskStore(db.DB)
		taskRunStore := store.NewTaskRunStore(db.DB)
		notifStore := store.NewNotificationStore(db.DB)
		notifManager := notifier.NewManager(cfg.Notifier, notifStore)
		runner := executor.NewRunner(taskStore, taskRunStore, notifManager)
		sched := scheduler.New(runner, taskStore, cfg.Scheduler.MaxConcurrent)
		// NOTE: we intentionally do NOT call sched.Start() — no cron scheduling
		// in embedded mode. The scheduler is only used for RunNowWithTrigger.

		h := NewDirectHandler(taskStore, taskRunStore, sched, version)
		registerDirectTools(mcpSrv, h)
	}

	stdioSrv := server.NewStdioServer(mcpSrv)

	err := stdioSrv.Listen(ctx, os.Stdin, os.Stdout)

	if cleanupFn != nil {
		cleanupFn()
	}

	return err
}

// probeDaemon checks whether the Kronos daemon is reachable.
func probeDaemon(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/health", port)
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// registerProxyTools adds all MCP tools backed by the proxy handler.
func registerProxyTools(mcpSrv *server.MCPServer, h *ProxyHandler) {
	mcpSrv.AddTool(listTasksTool(), h.handleListTasks)
	mcpSrv.AddTool(createTaskTool(), h.handleCreateTask)
	mcpSrv.AddTool(updateTaskTool(), h.handleUpdateTask)
	mcpSrv.AddTool(deleteTaskTool(), h.handleDeleteTask)
	mcpSrv.AddTool(runTaskTool(), h.handleRunTask)
	mcpSrv.AddTool(getTaskStatusTool(), h.handleGetTaskStatus)
	mcpSrv.AddTool(getTaskHistoryTool(), h.handleGetTaskHistory)
	mcpSrv.AddTool(enableTaskTool(), h.handleEnableTask)
	mcpSrv.AddTool(disableTaskTool(), h.handleDisableTask)
	mcpSrv.AddTool(serverStatusTool(), h.handleServerStatus)
}

// registerDirectTools adds all MCP tools backed by the direct handler.
func registerDirectTools(mcpSrv *server.MCPServer, h *DirectHandler) {
	mcpSrv.AddTool(listTasksTool(), h.handleListTasks)
	mcpSrv.AddTool(createTaskTool(), h.handleCreateTask)
	mcpSrv.AddTool(updateTaskTool(), h.handleUpdateTask)
	mcpSrv.AddTool(deleteTaskTool(), h.handleDeleteTask)
	mcpSrv.AddTool(runTaskTool(), h.handleRunTask)
	mcpSrv.AddTool(getTaskStatusTool(), h.handleGetTaskStatus)
	mcpSrv.AddTool(getTaskHistoryTool(), h.handleGetTaskHistory)
	mcpSrv.AddTool(enableTaskTool(), h.handleEnableTask)
	mcpSrv.AddTool(disableTaskTool(), h.handleDisableTask)
	mcpSrv.AddTool(serverStatusTool(), h.handleServerStatus)
}
