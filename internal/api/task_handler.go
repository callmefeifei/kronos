package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pstrr/kronos/internal/auth"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
)

// TaskHandler handles task CRUD and control endpoints.
type TaskHandler struct {
	taskStore *store.TaskStore
	scheduler *scheduler.Scheduler
}

// NewTaskHandler creates a new TaskHandler.
func NewTaskHandler(taskStore *store.TaskStore, sched *scheduler.Scheduler) *TaskHandler {
	return &TaskHandler{
		taskStore: taskStore,
		scheduler: sched,
	}
}

type createTaskRequest struct {
	Name          string  `json:"name" binding:"required"`
	Type          string  `json:"type" binding:"required,oneof=remind script agent"`
	ScheduleType  string  `json:"schedule_type" binding:"required,oneof=cron interval once"`
	ScheduleExpr  string  `json:"schedule_expr" binding:"required"`
	Target        string  `json:"target"`
	Args          *any    `json:"args"`
	Timeout       *int    `json:"timeout"`
	RetryCount    *int    `json:"retry_count"`
	RetryInterval *int    `json:"retry_interval"`
	NotifyOn      *any    `json:"notify_on"`
	NotifyChannel string  `json:"notify_channel"`
	Enabled       *bool   `json:"enabled"`
}

type updateTaskRequest struct {
	Name          *string `json:"name"`
	Type          *string `json:"type" binding:"omitempty,oneof=remind script agent"`
	ScheduleType  *string `json:"schedule_type" binding:"omitempty,oneof=cron interval once"`
	ScheduleExpr  *string `json:"schedule_expr"`
	Target        *string `json:"target"`
	Args          *any    `json:"args"`
	Timeout       *int    `json:"timeout"`
	RetryCount    *int    `json:"retry_count"`
	RetryInterval *int    `json:"retry_interval"`
	NotifyOn      *any    `json:"notify_on"`
	NotifyChannel *string `json:"notify_channel"`
	Enabled       *bool   `json:"enabled"`
}

// List returns a paginated list of tasks. Admin sees all; regular users see their own.
func (h *TaskHandler) List(c *gin.Context) {
	page, size := parsePagination(c)
	user := auth.GetCurrentUser(c)

	filter := &store.TaskFilter{
		Type: c.Query("type"),
	}
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		enabled := enabledStr == "true" || enabledStr == "1"
		filter.Enabled = &enabled
	}

	var tasks []model.Task
	var total int64
	var err error

	if user.Role == "admin" {
		tasks, total, err = h.taskStore.List(page, size, filter)
	} else {
		tasks, total, err = h.taskStore.ListByUser(user.ID, page, size, filter)
	}

	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list tasks")
		return
	}

	PagedSuccess(c, tasks, total, page, size)
}

// Create creates a new task.
func (h *TaskHandler) Create(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
		return
	}

	user := auth.GetCurrentUser(c)

	task := &model.Task{
		Name:          req.Name,
		Type:          req.Type,
		ScheduleType:  req.ScheduleType,
		ScheduleExpr:  req.ScheduleExpr,
		Target:        req.Target,
		NotifyChannel: req.NotifyChannel,
		UserID:        user.ID,
		Enabled:       true,
	}

	if req.Args != nil {
		task.Args = marshalJSON(req.Args)
	}
	if req.NotifyOn != nil {
		task.NotifyOn = marshalJSON(req.NotifyOn)
	}
	if req.Timeout != nil {
		task.Timeout = *req.Timeout
	}
	if req.RetryCount != nil {
		task.RetryCount = *req.RetryCount
	}
	if req.RetryInterval != nil {
		task.RetryInterval = *req.RetryInterval
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}

	if err := h.taskStore.Create(task); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to create task")
		return
	}

	// Register with scheduler if enabled.
	if task.Enabled {
		if err := h.scheduler.AddTask(task); err != nil {
			// Task created in DB but scheduling failed — not fatal, log and continue.
			Error(c, http.StatusInternalServerError, 500, "task created but scheduling failed: "+err.Error())
			return
		}
	}

	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "ok",
		Data:    task,
	})
}

// Get returns a single task by ID.
func (h *TaskHandler) Get(c *gin.Context) {
	task, ok := h.getTaskWithAccess(c)
	if !ok {
		return
	}
	Success(c, task)
}

// Update modifies an existing task.
func (h *TaskHandler) Update(c *gin.Context) {
	task, ok := h.getTaskWithAccess(c)
	if !ok {
		return
	}

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
		return
	}

	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Type != nil {
		task.Type = *req.Type
	}
	if req.ScheduleType != nil {
		task.ScheduleType = *req.ScheduleType
	}
	if req.ScheduleExpr != nil {
		task.ScheduleExpr = *req.ScheduleExpr
	}
	if req.Target != nil {
		task.Target = *req.Target
	}
	if req.Args != nil {
		task.Args = marshalJSON(req.Args)
	}
	if req.Timeout != nil {
		task.Timeout = *req.Timeout
	}
	if req.RetryCount != nil {
		task.RetryCount = *req.RetryCount
	}
	if req.RetryInterval != nil {
		task.RetryInterval = *req.RetryInterval
	}
	if req.NotifyOn != nil {
		task.NotifyOn = marshalJSON(req.NotifyOn)
	}
	if req.NotifyChannel != nil {
		task.NotifyChannel = *req.NotifyChannel
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}

	if err := h.taskStore.Update(task); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to update task")
		return
	}

	// Update scheduler entry.
	if err := h.scheduler.UpdateTask(task); err != nil {
		Error(c, http.StatusInternalServerError, 500, "task updated but rescheduling failed: "+err.Error())
		return
	}

	Success(c, task)
}

// Delete soft-deletes a task.
func (h *TaskHandler) Delete(c *gin.Context) {
	task, ok := h.getTaskWithAccess(c)
	if !ok {
		return
	}

	// Remove from scheduler first.
	h.scheduler.RemoveTask(task.ID)

	if err := h.taskStore.Delete(task.ID); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to delete task")
		return
	}

	Success(c, nil)
}

// Run triggers an immediate execution of the task.
func (h *TaskHandler) Run(c *gin.Context) {
	task, ok := h.getTaskWithAccess(c)
	if !ok {
		return
	}

	user := auth.GetCurrentUser(c)
	triggeredBy := fmt.Sprintf("api:%d", user.ID)

	// RunNow re-fetches from store internally; we pass triggeredBy via scheduler.
	// Since RunNow currently uses "manual", we call it and it will execute.
	_ = triggeredBy // RunNow uses a fixed "manual" trigger; see note below.
	if err := h.scheduler.RunNow(task.ID); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to trigger task run: "+err.Error())
		return
	}

	Success(c, gin.H{"message": "task execution triggered"})
}

// Enable sets task enabled=true.
func (h *TaskHandler) Enable(c *gin.Context) {
	task, ok := h.getTaskWithAccess(c)
	if !ok {
		return
	}

	task.Enabled = true
	if err := h.taskStore.Update(task); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to enable task")
		return
	}

	if err := h.scheduler.AddTask(task); err != nil {
		Error(c, http.StatusInternalServerError, 500, "task enabled but scheduling failed: "+err.Error())
		return
	}

	Success(c, task)
}

// Disable sets task enabled=false and removes from scheduler.
func (h *TaskHandler) Disable(c *gin.Context) {
	task, ok := h.getTaskWithAccess(c)
	if !ok {
		return
	}

	task.Enabled = false
	if err := h.taskStore.Update(task); err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to disable task")
		return
	}

	h.scheduler.RemoveTask(task.ID)

	Success(c, task)
}

// getTaskWithAccess loads a task by :id param and checks that the current
// user is either admin or the task owner. Returns false if the response was
// already written (error case).
func (h *TaskHandler) getTaskWithAccess(c *gin.Context) (*model.Task, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid task id")
		return nil, false
	}

	task, err := h.taskStore.GetByID(id)
	if err != nil {
		Error(c, http.StatusNotFound, 404, "task not found")
		return nil, false
	}

	user := auth.GetCurrentUser(c)
	if user.Role != "admin" && task.UserID != user.ID {
		Error(c, http.StatusForbidden, 403, "access denied")
		return nil, false
	}

	return task, true
}

// marshalJSON converts an arbitrary value to datatypes.JSON.
func marshalJSON(v interface{}) []byte {
	if v == nil {
		return nil
	}
	data, _ := json.Marshal(v)
	return data
}
