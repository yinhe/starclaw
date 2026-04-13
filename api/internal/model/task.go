package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusWaiting   TaskStatus = "waiting" // waiting for a scheduled time
)

// TaskPriority represents the priority level
type TaskPriority string

const (
	TaskPriorityUrgent TaskPriority = "urgent"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityNormal TaskPriority = "normal"
	TaskPriorityLow    TaskPriority = "low"
)

// Task represents an autonomous background task that an Agent executes
type Task struct {
	ID             string       `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string       `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID        string       `json:"agent_id" gorm:"type:varchar(36);index"`
	ConversationID string       `json:"conversation_id" gorm:"type:varchar(36);index"` // links task to originating conversation
	ParentTaskID   string       `json:"parent_task_id" gorm:"type:varchar(36);index"`  // for sub-tasks
	Title          string       `json:"title" gorm:"type:varchar(500);not null"`
	Description    string       `json:"description" gorm:"type:longtext"`
	Goal           string       `json:"goal" gorm:"type:longtext;not null"` // the message sent to the agent
	Status         TaskStatus   `json:"status" gorm:"type:varchar(20);index;default:'pending'"`
	Priority       TaskPriority `json:"priority" gorm:"type:varchar(20);default:'normal'"`
	Result         string       `json:"result" gorm:"type:longtext"` // agent's final output
	ErrorMsg       string       `json:"error_msg" gorm:"type:text"`
	Progress       int          `json:"progress" gorm:"default:0"`      // 0-100
	ProgressNote   string       `json:"progress_note" gorm:"type:text"` // human-readable progress
	MaxRetries     int          `json:"max_retries" gorm:"default:3"`
	RetryCount     int          `json:"retry_count" gorm:"default:0"`
	ScheduledAt    *time.Time   `json:"scheduled_at" gorm:"index"`                 // nil = run immediately
	ToolsOverride  string       `json:"tools_override" gorm:"type:text"`           // JSON array of tool names; if set, overrides agent's tools list
	ScheduleID     string       `json:"schedule_id" gorm:"type:varchar(36);index"` // from which schedule
	StartedAt      *time.Time   `json:"started_at"`
	CompletedAt    *time.Time   `json:"completed_at"`
	Heartbeat      *time.Time   `json:"heartbeat" gorm:"index"` // last heartbeat from worker
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Status == "" {
		t.Status = TaskStatusPending
	}
	if t.Priority == "" {
		t.Priority = TaskPriorityNormal
	}
	return nil
}
