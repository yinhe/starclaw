package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkspaceFolder tracks conversation-level folder metadata (lock state, etc.)
type WorkspaceFolder struct {
	ID             string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID string         `json:"conversation_id" gorm:"type:varchar(36);uniqueIndex;not null"`
	Locked         bool           `json:"locked" gorm:"default:false"`
	LockedAt       *time.Time     `json:"locked_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (f *WorkspaceFolder) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}
