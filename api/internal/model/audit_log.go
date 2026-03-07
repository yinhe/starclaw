package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog records significant user actions for security and debugging
type AuditLog struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Action    string    `json:"action" gorm:"type:varchar(50);not null;index"` // e.g. login, create_agent, delete_workflow
	Resource  string    `json:"resource" gorm:"type:varchar(50)"`              // e.g. agent, workflow, conversation
	ResourceID string   `json:"resource_id" gorm:"type:varchar(36)"`
	Detail    string    `json:"detail" gorm:"type:text"`
	IP        string    `json:"ip" gorm:"type:varchar(45)"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
