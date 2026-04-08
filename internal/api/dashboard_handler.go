package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DashboardHandler handles dashboard endpoints.
type DashboardHandler struct {
	db *gorm.DB
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

type dashboardStats struct {
	TotalTasks  int64   `json:"total_tasks"`
	ActiveTasks int64   `json:"active_tasks"`
	TodayRuns   int64   `json:"today_runs"`
	SuccessRate float64 `json:"success_rate"`
}

// Stats returns overall dashboard statistics.
func (h *DashboardHandler) Stats(c *gin.Context) {
	var stats dashboardStats

	// Total tasks (non-deleted).
	h.db.Table("tasks").Where("deleted_at IS NULL").Count(&stats.TotalTasks)

	// Active tasks (enabled and non-deleted).
	h.db.Table("tasks").Where("enabled = ? AND deleted_at IS NULL", true).Count(&stats.ActiveTasks)

	// Today's runs.
	todayStart := time.Now().Truncate(24 * time.Hour)
	h.db.Table("task_runs").Where("started_at >= ?", todayStart).Count(&stats.TodayRuns)

	// Success rate for today.
	if stats.TodayRuns > 0 {
		var successCount int64
		h.db.Table("task_runs").
			Where("started_at >= ? AND status = ?", todayStart, "success").
			Count(&successCount)
		stats.SuccessRate = float64(successCount) / float64(stats.TodayRuns) * 100
	}

	Success(c, stats)
}

type timelineEntry struct {
	ID          int64      `json:"id"`
	TaskID      int64      `json:"task_id"`
	TaskName    string     `json:"task_name"`
	Status      string     `json:"status"`
	TriggeredBy string     `json:"triggered_by"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	DurationMs  int        `json:"duration_ms"`
}

// Timeline returns the last 20 runs with task name.
func (h *DashboardHandler) Timeline(c *gin.Context) {
	var entries []timelineEntry

	h.db.Table("task_runs").
		Select("task_runs.id, task_runs.task_id, tasks.name as task_name, task_runs.status, task_runs.triggered_by, task_runs.started_at, task_runs.finished_at, task_runs.duration_ms").
		Joins("LEFT JOIN tasks ON tasks.id = task_runs.task_id").
		Order("task_runs.started_at DESC").
		Limit(20).
		Scan(&entries)

	if entries == nil {
		entries = []timelineEntry{}
	}

	Success(c, entries)
}
