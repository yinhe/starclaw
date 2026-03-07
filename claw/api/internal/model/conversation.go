package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Conversation struct {
	ID        string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID   string         `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	Title     string         `json:"title" gorm:"type:varchar(500)"`
	IsPinned  bool           `json:"is_pinned" gorm:"default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Messages []Message `json:"messages,omitempty" gorm:"foreignKey:ConversationID"`
}

func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

type Message struct {
	ID             string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ConversationID string    `json:"conversation_id" gorm:"type:varchar(36);index;not null"`
	Role           string    `json:"role" gorm:"type:varchar(20);not null"` // user, assistant, system, tool
	Content        string    `json:"content" gorm:"type:longtext"`
	ToolCalls      string    `json:"tool_calls,omitempty" gorm:"type:json"`
	ToolCallID     string    `json:"tool_call_id,omitempty" gorm:"type:varchar(100)"`
	TokensUsed     int       `json:"tokens_used" gorm:"default:0"`
	Feedback       int       `json:"feedback" gorm:"default:0"` // 0=none, 1=up, -1=down
	CreatedAt      time.Time `json:"created_at"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.ToolCalls == "" {
		m.ToolCalls = "[]"
	}
	return nil
}
