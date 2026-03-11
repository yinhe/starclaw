package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhook represents a notification endpoint for Overlord events
type Webhook struct {
	ID     string `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name   string `json:"name" gorm:"type:varchar(200);not null"`
	URL    string `json:"url" gorm:"type:varchar(500);not null"`
	Secret string `json:"-" gorm:"type:varchar(128)"` // HMAC signing secret
	TeamID string `json:"team_id" gorm:"type:varchar(36);index"`

	// Event filter (comma-separated): node.online, node.offline, node.feral, task.assigned, molt.approved, molt.failed, tunnel.error
	Events string `json:"events" gorm:"type:varchar(500);default:*"`
	Status string `json:"status" gorm:"type:varchar(20);default:active"` // active, paused, disabled

	// Stats
	TotalSent   int        `json:"total_sent" gorm:"default:0"`
	TotalFailed int        `json:"total_failed" gorm:"default:0"`
	LastSentAt  *time.Time `json:"last_sent_at"`
	LastError   string     `json:"last_error" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return nil
}

// WebhookLog records each delivery attempt
type WebhookLog struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	WebhookID  string `json:"webhook_id" gorm:"type:varchar(36);index;not null"`
	Event      string `json:"event" gorm:"type:varchar(50)"`
	Payload    string `json:"payload" gorm:"type:text"`
	StatusCode int    `json:"status_code" gorm:"default:0"`
	Error      string `json:"error" gorm:"type:text"`
	DurationMs int    `json:"duration_ms" gorm:"default:0"`
	CreatedAt  time.Time `json:"created_at"`
}
