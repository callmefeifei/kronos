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
	"github.com/pstrr/kronos/internal/executor"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
)

// PendingTask represents a fired task waiting for a remote client to pick up and execute.
type PendingTask struct {
	RunID    int64          `json:"run_id"`   // TaskRun ID — must be passed back in report_result
	TaskID   int64          `json:"task_id"`
	UserID   int64          `json:"user_id"`  // owner — used by Hub to route to the right agent
	TaskName string         `json:"task_name"`
	Type     string         `json:"type"`   // remind, script, agent
	Target   string         `json:"target"` // command / prompt / reminder text
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

	// Result channels — keyed by TaskRun.ID. Runner blocks until remote agent
	// calls report_result, which sends into the channel.
	resultMu sync.Mutex
	resultChs map[int64]chan *executor.RunResult
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
		resultChs:    make(map[int64]chan *executor.RunResult),
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

// ── executor.TaskNotifier implementation ──────────────────────────────────────

// NotifyTaskFired enqueues the task for polling and pushes a task_fired SSE
// notification to all connected MCP clients.
func (s *Server) NotifyTaskFired(task *model.Task, runID int64) {
	pt := PendingTask{
		RunID:    runID,
		TaskID:   task.ID,
		UserID:   task.UserID,
		TaskName: task.Name,
		Type:     task.Type,
		Target:   task.Target,
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

	// Also push SSE notification to connected clients.
	if s.mcpSrv != nil {
		s.mu.RLock()
		count := len(s.sessions)
		s.mu.RUnlock()

		if count > 0 {
			params := map[string]any{
				"run_id":    pt.RunID,
				"task_id":   pt.TaskID,
				"user_id":   pt.UserID,
				"task_name": pt.TaskName,
				"type":      pt.Type,
				"target":    pt.Target,
				"args":      pt.Args,
				"fired_at":  pt.FiredAt,
			}
			s.mcpSrv.SendNotificationToAllClients("kronos/task_fired", params)
			slog.Info("sent task_fired notification", "task_id", task.ID, "run_id", runID, "clients", count)
		} else {
			slog.Info("task queued for polling (no clients connected)", "task_id", task.ID, "run_id", runID)
		}
	}
}

// ConnectedClients returns the number of connected MCP sessions.
func (s *Server) ConnectedClients() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// ── executor.ResultReceiver implementation ────────────────────────────────────

// RegisterResultChannel creates a buffered channel for the given run ID.
// The Runner blocks on this channel until the remote agent calls report_result.
func (s *Server) RegisterResultChannel(runID int64) <-chan *executor.RunResult {
	ch := make(chan *executor.RunResult, 1)
	s.resultMu.Lock()
	s.resultChs[runID] = ch
	s.resultMu.Unlock()
	return ch
}

// UnregisterResultChannel removes the channel for the given run ID (cleanup on timeout/cancel).
func (s *Server) UnregisterResultChannel(runID int64) {
	s.resultMu.Lock()
	delete(s.resultChs, runID)
	s.resultMu.Unlock()
}

// DeliverResult sends a result to the waiting Runner goroutine (if still waiting).
// Returns true if the channel was found and the result delivered, false otherwise.
func (s *Server) DeliverResult(runID int64, result *executor.RunResult) bool {
	s.resultMu.Lock()
	ch, ok := s.resultChs[runID]
	s.resultMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- result:
		return true
	default:
		return false
	}
}

// ── poll_pending_tasks ────────────────────────────────────────────────────────

// DrainPendingTasks returns and clears all pending tasks.
func (s *Server) DrainPendingTasks() []PendingTask {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	tasks := s.pending
	s.pending = nil
	return tasks
}

func (s *Server) handlePollPendingTasks(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tasks := s.DrainPendingTasks()
	return jsonResult(map[string]any{
		"count": len(tasks),
		"tasks": tasks,
	})
}

// ── report_result ─────────────────────────────────────────────────────────────

// handleReportResult is called by the remote agent after it finishes executing a task.
// It unblocks the Runner goroutine waiting for the result.
func (s *Server) handleReportResult(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID := mcp.ParseInt64(req, "run_id", 0)
	if runID == 0 {
		return mcp.NewToolResultError("run_id is required"), nil
	}
	status := mcp.ParseString(req, "status", "")
	if status == "" {
		return mcp.NewToolResultError("status is required"), nil
	}
	switch status {
	case "success", "failed", "timeout":
	default:
		return mcp.NewToolResultError("status must be one of: success, failed, timeout"), nil
	}

	output := mcp.ParseString(req, "output", "")
	errMsg := mcp.ParseString(req, "error", "")
	exitCode := mcp.ParseInt(req, "exit_code", 0)

	result := &executor.RunResult{
		Status:   status,
		Output:   truncateOutput(output),
		Error:    errMsg,
		ExitCode: exitCode,
	}

	delivered := s.DeliverResult(runID, result)

	if delivered {
		slog.Info("report_result delivered", "run_id", runID, "status", status)
		return jsonResult(map[string]any{
			"ok":     true,
			"run_id": runID,
		})
	}

	// Runner already timed out — update the DB record directly so the result
	// is not lost entirely.
	slog.Warn("report_result: runner already timed out, updating DB directly", "run_id", runID)
	run, err := s.taskRunStore.GetByID(runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run %d not found: %v", runID, err)), nil
	}
	now := time.Now()
	run.Status = status
	run.Output = truncateOutput(output)
	run.Error = errMsg
	if status == "success" || status == "failed" || status == "timeout" {
		exitC := exitCode
		run.ExitCode = &exitC
	}
	run.FinishedAt = &now
	if err := s.taskRunStore.Update(run); err != nil {
		slog.Error("report_result: failed to update DB", "run_id", runID, "error", err)
	}

	return jsonResult(map[string]any{
		"ok":     true,
		"run_id": runID,
		"note":   "runner had already timed out; result written directly to DB",
	})
}

// ── auth ──────────────────────────────────────────────────────────────────────

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

func checkAuth(ctx context.Context) *mcp.CallToolResult {
	if errMsg, ok := ctx.Value(authErrorKey{}).(string); ok {
		return mcp.NewToolResultError(errMsg)
	}
	return nil
}

func withAuth(handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if r := checkAuth(ctx); r != nil {
			return r, nil
		}
		return handler(ctx, req)
	}
}

// ── tool registration ─────────────────────────────────────────────────────────

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
	mcpSrv.AddTool(reportResultTool(), withAuth(s.handleReportResult))
}

// jsonResult marshals data to JSON and returns it as a text tool result.
func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// truncateOutput keeps the last maxOutputBytes of string s.
func truncateOutput(s string) string {
	const max = 64 * 1024
	if len(s) <= max {
		return s
	}
	return "[...truncated...]\n" + s[len(s)-max:]
}
