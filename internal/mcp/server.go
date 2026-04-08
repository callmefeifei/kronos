package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
)

// Server wraps an MCP SSE server with Kronos task management tools.
type Server struct {
	cfg          config.MCPConfig
	sse          *server.SSEServer
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
	scheduler    *scheduler.Scheduler
}

// New creates a new MCP Server.
func New(
	cfg config.MCPConfig,
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	sched *scheduler.Scheduler,
) *Server {
	return &Server{
		cfg:          cfg,
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
		scheduler:    sched,
	}
}

// Start initializes the MCP SSE server and begins listening. Non-blocking.
func (s *Server) Start() error {
	mcpSrv := server.NewMCPServer(
		"kronos",
		"1.0.0",
		server.WithInstructions("Kronos task scheduler — manage scheduled tasks via MCP tools."),
	)

	s.registerTools(mcpSrv)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var sseOpts []server.SSEOption
	if s.cfg.Token != "" {
		sseOpts = append(sseOpts, server.WithSSEContextFunc(s.authMiddleware))
	}

	s.sse = server.NewSSEServer(mcpSrv, sseOpts...)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("mcp server listening", "addr", addr)
		if err := s.sse.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("mcp server error", "error", err)
			errCh <- err
		}
	}()

	// Give a brief moment for startup errors.
	select {
	case err := <-errCh:
		return fmt.Errorf("mcp server start: %w", err)
	default:
		return nil
	}
}

// Shutdown gracefully stops the MCP SSE server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.sse == nil {
		return nil
	}
	return s.sse.Shutdown(ctx)
}

// authMiddleware validates Bearer token from the Authorization header.
// On failure it returns a context with a sentinel error value that tools check.
type authErrorKey struct{}

func (s *Server) authMiddleware(ctx context.Context, r *http.Request) context.Context {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return context.WithValue(ctx, authErrorKey{}, "missing or invalid Authorization header")
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token != s.cfg.Token {
		return context.WithValue(ctx, authErrorKey{}, "invalid bearer token")
	}
	return ctx
}

// checkAuth returns an error result if the request is not authenticated.
func checkAuth(ctx context.Context) *mcp.CallToolResult {
	if errMsg, ok := ctx.Value(authErrorKey{}).(string); ok {
		return mcp.NewToolResultError(errMsg)
	}
	return nil
}

// registerTools adds all Kronos MCP tools to the server.
func (s *Server) registerTools(mcpSrv *server.MCPServer) {
	mcpSrv.AddTool(listTasksTool(), s.handleListTasks)
	mcpSrv.AddTool(createTaskTool(), s.handleCreateTask)
	mcpSrv.AddTool(runTaskTool(), s.handleRunTask)
	mcpSrv.AddTool(getTaskStatusTool(), s.handleGetTaskStatus)
	mcpSrv.AddTool(getTaskHistoryTool(), s.handleGetTaskHistory)
	mcpSrv.AddTool(enableTaskTool(), s.handleEnableTask)
	mcpSrv.AddTool(disableTaskTool(), s.handleDisableTask)
}

// --- Tool Definitions ---

func listTasksTool() mcp.Tool {
	return mcp.NewTool("list_tasks",
		mcp.WithDescription("List scheduled tasks with optional filters"),
		mcp.WithString("type", mcp.Description("Filter by task type: remind, script, agent"), mcp.Enum("remind", "script", "agent")),
		mcp.WithBoolean("enabled", mcp.Description("Filter by enabled status")),
	)
}

func createTaskTool() mcp.Tool {
	return mcp.NewTool("create_task",
		mcp.WithDescription("Create a new scheduled task"),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
		mcp.WithString("type", mcp.Description("Task type: remind, script, agent"), mcp.Required(), mcp.Enum("remind", "script", "agent")),
		mcp.WithString("schedule_type", mcp.Description("Schedule type: cron, interval, once"), mcp.Required(), mcp.Enum("cron", "interval", "once")),
		mcp.WithString("schedule_expr", mcp.Description("Schedule expression (cron expr, duration, or RFC3339 time)"), mcp.Required()),
		mcp.WithString("target", mcp.Description("Execution target (script path, reminder text, etc.)"), mcp.Required()),
		mcp.WithNumber("timeout", mcp.Description("Timeout in seconds (default 300)")),
		mcp.WithNumber("retry_count", mcp.Description("Number of retries (default 3)")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the task is enabled (default true)")),
	)
}

func runTaskTool() mcp.Tool {
	return mcp.NewTool("run_task",
		mcp.WithDescription("Run a task immediately, bypassing its schedule"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to run"), mcp.Required()),
	)
}

func getTaskStatusTool() mcp.Tool {
	return mcp.NewTool("get_task_status",
		mcp.WithDescription("Get task info combined with its latest run info"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task"), mcp.Required()),
	)
}

func getTaskHistoryTool() mcp.Tool {
	return mcp.NewTool("get_task_history",
		mcp.WithDescription("Get execution history for a task"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Number of runs to return (default 10)")),
	)
}

func enableTaskTool() mcp.Tool {
	return mcp.NewTool("enable_task",
		mcp.WithDescription("Enable a disabled task"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to enable"), mcp.Required()),
	)
}

func disableTaskTool() mcp.Tool {
	return mcp.NewTool("disable_task",
		mcp.WithDescription("Disable an enabled task"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to disable"), mcp.Required()),
	)
}

// --- Tool Handlers ---

func (s *Server) handleListTasks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if r := checkAuth(ctx); r != nil {
		return r, nil
	}

	filter := &store.TaskFilter{}
	if t := mcp.ParseString(req, "type", ""); t != "" {
		filter.Type = t
	}
	// ParseBoolean always returns a value; check if the param was actually provided.
	if _, ok := req.GetArguments()["enabled"]; ok {
		enabled := mcp.ParseBoolean(req, "enabled", true)
		filter.Enabled = &enabled
	}

	tasks, total, err := s.taskStore.List(1, 100, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list tasks: %v", err)), nil
	}

	result := map[string]any{
		"total": total,
		"tasks": tasks,
	}
	return jsonResult(result)
}

func (s *Server) handleCreateTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if r := checkAuth(ctx); r != nil {
		return r, nil
	}

	task := &model.Task{
		Name:         mcp.ParseString(req, "name", ""),
		Type:         mcp.ParseString(req, "type", ""),
		ScheduleType: mcp.ParseString(req, "schedule_type", ""),
		ScheduleExpr: mcp.ParseString(req, "schedule_expr", ""),
		Target:       mcp.ParseString(req, "target", ""),
		Timeout:      mcp.ParseInt(req, "timeout", 300),
		RetryCount:   mcp.ParseInt(req, "retry_count", 3),
		Enabled:      mcp.ParseBoolean(req, "enabled", true),
		UserID:       0, // MCP-created tasks have no user owner
	}

	if err := s.taskStore.Create(task); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create task: %v", err)), nil
	}

	// If enabled, register with scheduler.
	if task.Enabled {
		if err := s.scheduler.AddTask(task); err != nil {
			slog.Error("failed to schedule MCP-created task", "task_id", task.ID, "error", err)
		}
	}

	return jsonResult(task)
}

func (s *Server) handleRunTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if r := checkAuth(ctx); r != nil {
		return r, nil
	}

	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := s.taskStore.GetByID(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	if err := s.scheduler.RunNowWithTrigger(taskID, "mcp"); err != nil {
		// Fallback: RunNow uses "manual" by default. If RunNowWithTrigger doesn't exist,
		// we handle it differently.
		return mcp.NewToolResultError(fmt.Sprintf("run task: %v", err)), nil
	}

	return jsonResult(map[string]any{
		"message": fmt.Sprintf("task %d (%s) triggered", task.ID, task.Name),
		"task_id": task.ID,
	})
}

func (s *Server) handleGetTaskStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if r := checkAuth(ctx); r != nil {
		return r, nil
	}

	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := s.taskStore.GetByID(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	result := map[string]any{
		"task": task,
	}

	latestRun, err := s.taskRunStore.GetLatestByTask(taskID)
	if err == nil {
		result["latest_run"] = latestRun
	}
	// If no runs exist, latest_run is simply absent.

	return jsonResult(result)
}

func (s *Server) handleGetTaskHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if r := checkAuth(ctx); r != nil {
		return r, nil
	}

	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	limit := mcp.ParseInt(req, "limit", 10)
	if limit <= 0 {
		limit = 10
	}

	runs, total, err := s.taskRunStore.ListByTask(taskID, 1, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get task history: %v", err)), nil
	}

	return jsonResult(map[string]any{
		"task_id": taskID,
		"total":   total,
		"runs":    runs,
	})
}

func (s *Server) handleEnableTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if r := checkAuth(ctx); r != nil {
		return r, nil
	}

	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := s.taskStore.GetByID(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	if task.Enabled {
		return jsonResult(map[string]any{
			"message": "task is already enabled",
			"task":    task,
		})
	}

	task.Enabled = true
	if err := s.taskStore.Update(task); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("enable task: %v", err)), nil
	}

	if err := s.scheduler.UpdateTask(task); err != nil {
		slog.Error("failed to update scheduler after enabling task", "task_id", task.ID, "error", err)
	}

	return jsonResult(map[string]any{
		"message": "task enabled",
		"task":    task,
	})
}

func (s *Server) handleDisableTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if r := checkAuth(ctx); r != nil {
		return r, nil
	}

	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := s.taskStore.GetByID(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	if !task.Enabled {
		return jsonResult(map[string]any{
			"message": "task is already disabled",
			"task":    task,
		})
	}

	task.Enabled = false
	if err := s.taskStore.Update(task); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("disable task: %v", err)), nil
	}

	if err := s.scheduler.UpdateTask(task); err != nil {
		slog.Error("failed to update scheduler after disabling task", "task_id", task.ID, "error", err)
	}

	return jsonResult(map[string]any{
		"message": "task disabled",
		"task":    task,
	})
}

// jsonResult marshals data to JSON and returns it as a text tool result.
func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
