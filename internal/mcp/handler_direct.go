package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
)

// DirectHandler implements MCP tool handlers that operate directly on the
// task store and scheduler. It is used by both the SSE server and the stdio
// server so that tool logic is shared.
type DirectHandler struct {
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
	scheduler    *scheduler.Scheduler
	version      string
}

// NewDirectHandler creates a DirectHandler with the given dependencies.
func NewDirectHandler(
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	sched *scheduler.Scheduler,
	version string,
) *DirectHandler {
	return &DirectHandler{
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
		scheduler:    sched,
		version:      version,
	}
}

func (h *DirectHandler) handleListTasks(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filter := &store.TaskFilter{}
	if t := mcp.ParseString(req, "type", ""); t != "" {
		filter.Type = t
	}
	if _, ok := req.GetArguments()["enabled"]; ok {
		enabled := mcp.ParseBoolean(req, "enabled", true)
		filter.Enabled = &enabled
	}

	tasks, total, err := h.taskStore.List(1, 100, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list tasks: %v", err)), nil
	}

	return jsonResult(map[string]any{
		"total": total,
		"tasks": tasks,
	})
}

func (h *DirectHandler) handleCreateTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	if err := h.taskStore.Create(task); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create task: %v", err)), nil
	}

	if task.Enabled {
		if err := h.scheduler.AddTask(task); err != nil {
			slog.Error("failed to schedule MCP-created task", "task_id", task.ID, "error", err)
		}
	}

	return jsonResult(task)
}

func (h *DirectHandler) handleUpdateTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := h.taskStore.GetByID(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	args := req.GetArguments()
	scheduleChanged := false

	if v, ok := args["name"]; ok {
		if s, ok := v.(string); ok {
			task.Name = s
		}
	}
	if v, ok := args["type"]; ok {
		if s, ok := v.(string); ok {
			task.Type = s
		}
	}
	if v, ok := args["schedule_type"]; ok {
		if s, ok := v.(string); ok {
			if s != task.ScheduleType {
				scheduleChanged = true
			}
			task.ScheduleType = s
		}
	}
	if v, ok := args["schedule_expr"]; ok {
		if s, ok := v.(string); ok {
			if s != task.ScheduleExpr {
				scheduleChanged = true
			}
			task.ScheduleExpr = s
		}
	}
	if v, ok := args["target"]; ok {
		if s, ok := v.(string); ok {
			task.Target = s
		}
	}
	if v, ok := args["timeout"]; ok {
		if n, ok := v.(float64); ok {
			task.Timeout = int(n)
		}
	}
	if v, ok := args["retry_count"]; ok {
		if n, ok := v.(float64); ok {
			task.RetryCount = int(n)
		}
	}

	enabledChanged := false
	if v, ok := args["enabled"]; ok {
		if b, ok := v.(bool); ok {
			if b != task.Enabled {
				enabledChanged = true
			}
			task.Enabled = b
		}
	}

	if err := h.taskStore.Update(task); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update task: %v", err)), nil
	}

	if scheduleChanged || enabledChanged {
		if err := h.scheduler.UpdateTask(task); err != nil {
			slog.Error("failed to update scheduler after task update", "task_id", task.ID, "error", err)
		}
	}

	return jsonResult(map[string]any{
		"message": "task updated",
		"task":    task,
	})
}

func (h *DirectHandler) handleDeleteTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	if _, err := h.taskStore.GetByID(taskID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	h.scheduler.RemoveTask(taskID)

	if err := h.taskStore.Delete(taskID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete task: %v", err)), nil
	}

	return jsonResult(map[string]any{
		"message": fmt.Sprintf("task %d deleted", taskID),
		"task_id": taskID,
	})
}

func (h *DirectHandler) handleRunTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := h.taskStore.GetByID(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	if err := h.scheduler.RunNowWithTrigger(taskID, "mcp"); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run task: %v", err)), nil
	}

	return jsonResult(map[string]any{
		"message": fmt.Sprintf("task %d (%s) triggered", task.ID, task.Name),
		"task_id": task.ID,
	})
}

func (h *DirectHandler) handleGetTaskStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := h.taskStore.GetByID(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %v", err)), nil
	}

	result := map[string]any{
		"task": task,
	}

	latestRun, err := h.taskRunStore.GetLatestByTask(taskID)
	if err == nil {
		result["latest_run"] = latestRun
	}

	return jsonResult(result)
}

func (h *DirectHandler) handleGetTaskHistory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	limit := mcp.ParseInt(req, "limit", 10)
	if limit <= 0 {
		limit = 10
	}

	runs, total, err := h.taskRunStore.ListByTask(taskID, 1, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get task history: %v", err)), nil
	}

	return jsonResult(map[string]any{
		"task_id": taskID,
		"total":   total,
		"runs":    runs,
	})
}

func (h *DirectHandler) handleEnableTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := h.taskStore.GetByID(taskID)
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
	if err := h.taskStore.Update(task); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("enable task: %v", err)), nil
	}

	if err := h.scheduler.UpdateTask(task); err != nil {
		slog.Error("failed to update scheduler after enabling task", "task_id", task.ID, "error", err)
	}

	return jsonResult(map[string]any{
		"message": "task enabled",
		"task":    task,
	})
}

func (h *DirectHandler) handleDisableTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := mcp.ParseInt64(req, "task_id", 0)
	if taskID == 0 {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task, err := h.taskStore.GetByID(taskID)
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
	if err := h.taskStore.Update(task); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("disable task: %v", err)), nil
	}

	if err := h.scheduler.UpdateTask(task); err != nil {
		slog.Error("failed to update scheduler after disabling task", "task_id", task.ID, "error", err)
	}

	return jsonResult(map[string]any{
		"message": "task disabled",
		"task":    task,
	})
}

func (h *DirectHandler) handleServerStatus(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, total, err := h.taskStore.List(1, 1, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get task counts: %v", err)), nil
	}

	enabledTrue := true
	_, enabledCount, err := h.taskStore.List(1, 1, &store.TaskFilter{Enabled: &enabledTrue})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get enabled count: %v", err)), nil
	}

	return jsonResult(map[string]any{
		"version":        h.version,
		"total_tasks":    total,
		"enabled_tasks":  enabledCount,
		"disabled_tasks": total - enabledCount,
	})
}
