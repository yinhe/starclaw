package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Workflow struct {
	ID             string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID string         `json:"conversation_id" gorm:"type:varchar(36);index"`
	Name           string         `json:"name" gorm:"type:varchar(200);not null"`
	Description    string         `json:"description" gorm:"type:text"`
	Definition     string         `json:"definition" gorm:"type:longtext"` // JSON: {nodes, edges}
	IsPublic       bool           `json:"is_public" gorm:"default:false"`
	WebhookToken   *string        `json:"webhook_token,omitempty" gorm:"type:varchar(64);uniqueIndex"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (w *Workflow) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return nil
}
