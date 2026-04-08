package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Task represents a scheduled task.
type Task struct {
	ID            int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string         `gorm:"type:varchar(255);not null" json:"name"`
	Type          string         `gorm:"type:varchar(16);not null" json:"type"`                    // remind, script, agent
	ScheduleType  string         `gorm:"type:varchar(16);not null;column:schedule_type" json:"schedule_type"` // cron, interval, once
	ScheduleExpr  string         `gorm:"type:varchar(255);not null;column:schedule_expr" json:"schedule_expr"`
	Target        string         `gorm:"type:text" json:"target"`
	Args          datatypes.JSON `gorm:"type:json" json:"args"`
	Timeout       int            `gorm:"default:300" json:"timeout"`          // seconds
	RetryCount    int            `gorm:"default:3;column:retry_count" json:"retry_count"`
	RetryInterval int            `gorm:"default:5;column:retry_interval" json:"retry_interval"` // seconds
	NotifyOn      datatypes.JSON `gorm:"type:json;column:notify_on" json:"notify_on"`           // {"success": false, "fail": true}
	NotifyChannel string         `gorm:"type:varchar(32);column:notify_channel" json:"notify_channel"` // feishu, webhook
	Enabled       bool           `gorm:"default:true;index:idx_user_enabled,priority:2" json:"enabled"`
	NextRunAt     *time.Time     `gorm:"column:next_run_at;index:idx_next_run_at" json:"next_run_at"`
	LastRunAt     *time.Time     `gorm:"column:last_run_at" json:"last_run_at"`
	LastStatus    string         `gorm:"type:varchar(16);column:last_status" json:"last_status"`
	UserID        int64          `gorm:"not null;index:idx_user_enabled,priority:1" json:"user_id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"deleted_at,omitempty"`

	// Associations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Task) TableName() string { return "tasks" }
