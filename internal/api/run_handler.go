package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pstrr/kronos/internal/auth"
	"github.com/pstrr/kronos/internal/store"
)

// RunHandler handles task run endpoints.
type RunHandler struct {
	taskStore    *store.TaskStore
	taskRunStore *store.TaskRunStore
}

// NewRunHandler creates a new RunHandler.
func NewRunHandler(taskStore *store.TaskStore, taskRunStore *store.TaskRunStore) *RunHandler {
	return &RunHandler{
		taskStore:    taskStore,
		taskRunStore: taskRunStore,
	}
}

// ListByTask returns a paginated list of runs for a specific task.
func (h *RunHandler) ListByTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid task id")
		return
	}

	// Verify task exists and user has access.
	task, err := h.taskStore.GetByID(taskID)
	if err != nil {
		Error(c, http.StatusNotFound, 404, "task not found")
		return
	}

	user := auth.GetCurrentUser(c)
	if user.Role != "admin" && task.UserID != user.ID {
		Error(c, http.StatusForbidden, 403, "access denied")
		return
	}

	page, size := parsePagination(c)
	runs, total, err := h.taskRunStore.ListByTask(taskID, page, size)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list runs")
		return
	}

	PagedSuccess(c, runs, total, page, size)
}

// Get returns a single run by ID.
func (h *RunHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid run id")
		return
	}

	run, err := h.taskRunStore.GetByID(id)
	if err != nil {
		Error(c, http.StatusNotFound, 404, "run not found")
		return
	}

	// Check access: load the task and verify ownership.
	task, err := h.taskStore.GetByID(run.TaskID)
	if err != nil {
		Error(c, http.StatusNotFound, 404, "associated task not found")
		return
	}

	user := auth.GetCurrentUser(c)
	if user.Role != "admin" && task.UserID != user.ID {
		Error(c, http.StatusForbidden, 403, "access denied")
		return
	}

	Success(c, run)
}

// Output returns the full output of a run as text/plain.
func (h *RunHandler) Output(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid run id")
		return
	}

	run, err := h.taskRunStore.GetByID(id)
	if err != nil {
		Error(c, http.StatusNotFound, 404, "run not found")
		return
	}

	// Check access.
	task, err := h.taskStore.GetByID(run.TaskID)
	if err != nil {
		Error(c, http.StatusNotFound, 404, "associated task not found")
		return
	}

	user := auth.GetCurrentUser(c)
	if user.Role != "admin" && task.UserID != user.ID {
		Error(c, http.StatusForbidden, 403, "access denied")
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(run.Output))
}
