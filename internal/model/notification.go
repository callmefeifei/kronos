package model

import "time"

// Notification records a notification sent for a task run.
type Notification struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID    int64     `gorm:"not null;index:idx_notif_task" json:"task_id"`       // redundant for queries
	TaskRunID int64     `gorm:"not null;index:idx_notif_task_run;column:task_run_id" json:"task_run_id"`
	Channel   string    `gorm:"type:varchar(32);not null" json:"channel"` // feishu, webhook
	Status    string    `gorm:"type:varchar(16);not null" json:"status"`  // sent, failed
	Payload   string    `gorm:"type:text" json:"payload"`
	Response  string    `gorm:"type:text" json:"response"`
	CreatedAt time.Time `json:"created_at"`

	// Associations
	Task    *Task    `gorm:"foreignKey:TaskID" json:"task,omitempty"`
	TaskRun *TaskRun `gorm:"foreignKey:TaskRunID" json:"task_run,omitempty"`
}

func (Notification) TableName() string { return "notifications" }
