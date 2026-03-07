package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Memory represents a cross-session memory entry for an agent
type Memory struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID   string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	Key       string    `json:"key" gorm:"type:varchar(200);not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	Category  string    `json:"category" gorm:"type:varchar(50);index"` // fact, preference, context
	Importance float64  `json:"importance" gorm:"default:0.5"`          // 0.0 - 1.0
	AccessCount int     `json:"access_count" gorm:"default:0"`
	LastAccessAt time.Time `json:"last_access_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *Memory) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
