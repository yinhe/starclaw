package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UsageRecord tracks every API call for billing and analytics
type UsageRecord struct {
	ID               string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID           string    `json:"user_id" gorm:"type:varchar(80);index;not null"`
	APIKeyID         string    `json:"api_key_id" gorm:"type:varchar(36);index"`
	Provider         string    `json:"provider" gorm:"type:varchar(50)"`  // openai, qwen, fal, grok, etc.
	Model            string    `json:"model" gorm:"type:varchar(100)"`    // openai/gpt-4o, qwen/qwen-max
	Endpoint         string    `json:"endpoint" gorm:"type:varchar(100)"` // /v1/chat/completions
	PromptTokens     int       `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int       `json:"completion_tokens" gorm:"default:0"`
	TotalTokens      int       `json:"total_tokens" gorm:"default:0"`
	CostCents        float64   `json:"cost_cents" gorm:"type:double;default:0"`     // cost in cents (分), supports sub-cent precision
	UpstreamCost     float64   `json:"upstream_cost" gorm:"type:double;default:0"`  // what we paid provider, in cents (分)
	Duration         int       `json:"duration" gorm:"default:0"`                   // ms
	Status           string    `json:"status" gorm:"type:varchar(20);default:'ok'"` // ok, error
	ErrorMsg         string    `json:"error_msg,omitempty" gorm:"type:text"`
	Via              string    `json:"via" gorm:"type:varchar(20);default:'direct'"` // direct or proxy
	CreatedAt        time.Time `json:"created_at" gorm:"index"`
}

func (u *UsageRecord) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}
