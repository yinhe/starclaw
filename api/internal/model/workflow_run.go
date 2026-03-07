package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowRun stores execution history for a workflow
type WorkflowRun struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	WorkflowID string    `json:"workflow_id" gorm:"type:varchar(36);index;not null"`
	UserID     string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Input      string    `json:"input" gorm:"type:text"`
	Output     string    `json:"output" gorm:"type:longtext"`
	Status     string    `json:"status" gorm:"type:varchar(20);default:'running'"` // running, success, error
	Error      string    `json:"error,omitempty" gorm:"type:text"`
	DurationMs int64     `json:"duration_ms" gorm:"default:0"`
	NodeLogs   string    `json:"node_logs,omitempty" gorm:"type:longtext"` // JSON array of node execution logs
	CreatedAt  time.Time `json:"created_at"`
}

func (r *WorkflowRun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
