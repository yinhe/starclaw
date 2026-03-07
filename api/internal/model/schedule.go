package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Schedule represents a cron job that periodically creates tasks
type Schedule struct {
	ID             string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	WorkflowID     string     `json:"workflow_id" gorm:"type:varchar(36);index"`     // optional workflow
	AgentID        string     `json:"agent_id" gorm:"type:varchar(36);index"`        // agent to run the task
	ConversationID string     `json:"conversation_id" gorm:"type:varchar(36);index"` // optional: link to conversation
	Title          string     `json:"title" gorm:"type:varchar(500)"`                // task title
	Goal           string     `json:"goal" gorm:"type:longtext"`                     // task goal/instructions
	Description    string     `json:"description" gorm:"type:text"`                  // human-readable description
	CronExpr       string     `json:"cron_expr" gorm:"type:varchar(100);not null"`   // e.g. "0 9 * * *"
	Input          string     `json:"input" gorm:"type:text"`                        // legacy: label/name
	Enabled        bool       `json:"enabled" gorm:"default:true"`
	MaxInstances   int        `json:"max_instances" gorm:"default:1"` // max concurrent tasks from this schedule
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (s *Schedule) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}
