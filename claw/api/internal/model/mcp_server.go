package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MCPServer stores a connected MCP tool server
type MCPServer struct {
	ID        string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name      string         `json:"name" gorm:"type:varchar(200);not null"`
	BaseURL   string         `json:"base_url" gorm:"type:varchar(500);not null"`
	APIKey    string         `json:"api_key,omitempty" gorm:"type:varchar(500)"`
	Status    string         `json:"status" gorm:"type:varchar(20);default:'active'"` // active, error, disabled
	ToolCount int            `json:"tool_count" gorm:"default:0"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (m *MCPServer) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
