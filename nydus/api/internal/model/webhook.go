package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhook defines a callback URL that receives events for a repository.
type Webhook struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RepoID    string    `json:"repo_id" gorm:"type:varchar(36);index;not null"`
	URL       string    `json:"url" gorm:"type:varchar(500);not null"`                 // callback URL
	Secret    string    `json:"secret" gorm:"type:varchar(100)"`                       // HMAC secret for payload signing
	Events    string    `json:"events" gorm:"type:varchar(200);default:push"`          // comma-separated: push,pr_opened,pr_merged,pr_closed,review
	Active    bool      `json:"active" gorm:"default:true"`
	CreatedBy string    `json:"created_by" gorm:"type:varchar(80)"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return nil
}

// WebhookDelivery records each webhook delivery attempt.
type WebhookDelivery struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	WebhookID  string    `json:"webhook_id" gorm:"type:varchar(36);index;not null"`
	Event      string    `json:"event" gorm:"type:varchar(30)"`             // push / pr_opened / pr_merged / review
	Payload    string    `json:"payload" gorm:"type:text"`                  // JSON payload sent
	StatusCode int       `json:"status_code" gorm:"default:0"`             // HTTP response code
	Response   string    `json:"response" gorm:"type:text"`                // response body (truncated)
	Duration   int64     `json:"duration_ms"`                              // request duration in ms
	Success    bool      `json:"success" gorm:"default:false"`
	CreatedAt  time.Time `json:"created_at"`
}

func (d *WebhookDelivery) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
