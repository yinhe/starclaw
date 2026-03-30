package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkspaceFile tracks files created by agents, linked to conversations
type WorkspaceFile struct {
	ID             string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID string         `json:"conversation_id" gorm:"type:varchar(36);index"`
	WorkspaceID    string         `json:"workspace_id" gorm:"type:varchar(100);index;not null"`
	Path           string         `json:"path" gorm:"type:varchar(500);not null"`
	Name           string         `json:"name" gorm:"type:varchar(255);not null"`
	Category       string         `json:"category" gorm:"type:varchar(20);default:'document'"` // code, document
	Size           int64          `json:"size"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (f *WorkspaceFile) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	return nil
}
