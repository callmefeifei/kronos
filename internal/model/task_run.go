package model

import "time"

// TaskRun represents a single execution of a task.
type TaskRun struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID       int64      `gorm:"not null;index:idx_task_started,priority:1" json:"task_id"`
	TriggeredBy  string     `gorm:"type:varchar(64);not null;column:triggered_by" json:"triggered_by"` // scheduler, api:user_id, mcp, cli
	Status       string     `gorm:"type:varchar(16);not null" json:"status"`                           // running, success, failed, timeout, cancelled
	StartedAt    time.Time  `gorm:"not null;index:idx_task_started,priority:2,sort:desc;column:started_at" json:"started_at"`
	FinishedAt   *time.Time `gorm:"column:finished_at" json:"finished_at"`
	DurationMs   int        `gorm:"column:duration_ms" json:"duration_ms"`
	ExitCode     *int       `gorm:"column:exit_code" json:"exit_code"` // script type only
	Output       string     `gorm:"type:mediumtext" json:"output"`     // app-level truncate 64KB
	Error        string     `gorm:"type:text" json:"error"`
	RetryAttempt int        `gorm:"default:0;column:retry_attempt" json:"retry_attempt"`

	// Associations
	Task *Task `gorm:"foreignKey:TaskID" json:"task,omitempty"`
}

func (TaskRun) TableName() string { return "task_runs" }
