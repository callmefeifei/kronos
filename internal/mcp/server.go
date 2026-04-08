package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
)

// PendingTask represents a fired agent task waiting for a client to pick up.
type PendingTask struct {
	TaskID   int64          `json:"task_id"`
	TaskName string         `json:"task_name"`
	Prompt   string         `json:"prompt"`
	Args     map[string]any `json:"args,omitempty"`
	FiredAt  string         `json:"fired_at"`
}

// Server wraps an MCP SSE server with Kronos task management tools.
type Server struct {
	cfg          config.MCPConfig
	mcpSrv       *server.MCPServer
	sse          *server.SSEServer
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
	scheduler    *scheduler.Scheduler
	version      string

	// Session tracking for notifications.
	mu       sync.RWMutex
	sessions map[string]server.ClientSession // sessionID → session

	// Pending task queue for poll_pending_tasks.
	pendingMu sync.Mutex
	pending   []PendingTask
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
		sessions:     make(map[string]server.ClientSession),
	}
}

// Start initializes the MCP SSE server and begins listening. Non-blocking.
func (s *Server) Start() error {
	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(_ context.Context, sess server.ClientSession) {
		s.mu.Lock()
		s.sessions[sess.SessionID()] = sess
		s.mu.Unlock()
		slog.Info("mcp client connected", "session_id", sess.SessionID())
	})
	hooks.AddOnUnregisterSession(func(_ context.Context, sess server.ClientSession) {
		s.mu.Lock()
		delete(s.sessions, sess.SessionID())
		s.mu.Unlock()
		slog.Info("mcp client disconnected", "session_id", sess.SessionID())
	})

	s.mcpSrv = server.NewMCPServer(
		"kronos",
		s.version,
		server.WithInstructions("Kronos task scheduler — manage scheduled tasks via MCP tools."),
		server.WithHooks(hooks),
	)

	s.registerTools(s.mcpSrv)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var sseOpts []server.SSEOption
	if s.cfg.Token != "" {
		sseOpts = append(sseOpts, server.WithSSEContextFunc(s.authMiddleware))
	}

	s.sse = server.NewSSEServer(s.mcpSrv, sseOpts...)

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

// NotifyTaskFired sends a notification to all connected MCP clients
// and enqueues the task for polling via poll_pending_tasks.
func (s *Server) NotifyTaskFired(task *model.Task) {
	// Build pending task entry.
	pt := PendingTask{
		TaskID:   task.ID,
		TaskName: task.Name,
		Prompt:   task.Target,
		FiredAt:  time.Now().Format(time.RFC3339),
	}
	if task.Args != nil {
		var args map[string]any
		if json.Unmarshal(task.Args, &args) == nil {
			pt.Args = args
		}
	}

	// Enqueue for polling.
	s.pendingMu.Lock()
	s.pending = append(s.pending, pt)
	s.pendingMu.Unlock()

	// Also push notification to connected clients.
	if s.mcpSrv != nil {
		s.mu.RLock()
		count := len(s.sessions)
		s.mu.RUnlock()

		if count > 0 {
			params := map[string]any{
				"task_id":   pt.TaskID,
				"task_name": pt.TaskName,
				"prompt":    pt.Prompt,
				"args":      pt.Args,
				"fired_at":  pt.FiredAt,
			}
			s.mcpSrv.SendNotificationToAllClients("kronos/task_fired", params)
			slog.Info("sent task_fired notification", "task_id", task.ID, "clients", count)
		} else {
			slog.Info("task queued for polling (no clients connected)", "task_id", task.ID)
		}
	}
}

// DrainPendingTasks returns and clears all pending tasks.
func (s *Server) DrainPendingTasks() []PendingTask {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	tasks := s.pending
	s.pending = nil
	return tasks
}

// ConnectedClients returns the number of connected MCP sessions.
func (s *Server) ConnectedClients() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// authMiddleware validates Bearer token from the Authorization header.
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
	mcpSrv.AddTool(pollPendingTasksTool(), withAuth(s.handlePollPendingTasks))
}

func (s *Server) handlePollPendingTasks(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tasks := s.DrainPendingTasks()
	return jsonResult(map[string]any{
		"count": len(tasks),
		"tasks": tasks,
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
