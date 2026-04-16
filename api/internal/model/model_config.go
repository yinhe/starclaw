package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ModelConfig struct {
	ID          string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID      string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Provider    string         `json:"provider" gorm:"type:varchar(50);not null"`    // openai, anthropic, deepseek, etc.
	ModelName   string         `json:"model_name" gorm:"type:varchar(100);not null"` // gpt-4o, claude-3.5-sonnet, etc.
	DisplayName string         `json:"display_name" gorm:"type:varchar(200)"`
	APIKey      string         `json:"-" gorm:"type:varchar(500)"`
	BaseURL     string         `json:"base_url" gorm:"type:varchar(500)"`
	MaxTokens   int            `json:"max_tokens" gorm:"default:4096"`
	Temperature float64        `json:"temperature" gorm:"type:decimal(3,2);default:0.7"`
	IsPlatform  bool           `json:"is_platform" gorm:"default:false"` // true = platform shared key
	IsEnabled   bool           `json:"is_enabled" gorm:"default:true"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (m *ModelConfig) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
