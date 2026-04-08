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
	version      string
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
		version:      "1.0.0",
	}
}

// Start initializes the MCP SSE server and begins listening. Non-blocking.
func (s *Server) Start() error {
	mcpSrv := server.NewMCPServer(
		"kronos",
		s.version,
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

// withAuth wraps a tool handler with the SSE-level auth check.
func withAuth(handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if r := checkAuth(ctx); r != nil {
			return r, nil
		}
		return handler(ctx, req)
	}
}

// registerTools adds all Kronos MCP tools to the server.
func (s *Server) registerTools(mcpSrv *server.MCPServer) {
	h := NewDirectHandler(s.taskStore, s.taskRunStore, s.scheduler, s.version)

	mcpSrv.AddTool(listTasksTool(), withAuth(h.handleListTasks))
	mcpSrv.AddTool(createTaskTool(), withAuth(h.handleCreateTask))
	mcpSrv.AddTool(updateTaskTool(), withAuth(h.handleUpdateTask))
	mcpSrv.AddTool(deleteTaskTool(), withAuth(h.handleDeleteTask))
	mcpSrv.AddTool(runTaskTool(), withAuth(h.handleRunTask))
	mcpSrv.AddTool(getTaskStatusTool(), withAuth(h.handleGetTaskStatus))
	mcpSrv.AddTool(getTaskHistoryTool(), withAuth(h.handleGetTaskHistory))
	mcpSrv.AddTool(enableTaskTool(), withAuth(h.handleEnableTask))
	mcpSrv.AddTool(disableTaskTool(), withAuth(h.handleDisableTask))
	mcpSrv.AddTool(serverStatusTool(), withAuth(h.handleServerStatus))
}

// jsonResult marshals data to JSON and returns it as a text tool result.
func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
