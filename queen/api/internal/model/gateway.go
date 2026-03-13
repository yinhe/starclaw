package model

import "time"

// GatewayUsageLog records each API call through the star-ai.net gateway
type GatewayUsageLog struct {
	ID               string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	APIKeyID         string    `json:"api_key_id" gorm:"type:varchar(36);index"`
	UserID           string    `json:"user_id" gorm:"type:varchar(36);index"`
	Model            string    `json:"model" gorm:"type:varchar(100)"`
	Provider         string    `json:"provider" gorm:"type:varchar(50)"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CostFen          int64     `json:"cost_fen"` // cost in 分
	DurationMs       int64     `json:"duration_ms"`
	StatusCode       int       `json:"status_code"`
	CreatedAt        time.Time `json:"created_at"`
}
