package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WeChatWatch stores auto-reply watch configuration for a WeChat group/contact.
// It is used by the background watcher to detect screenshot changes and enqueue
// customer-service tasks for a designated agent.
type WeChatWatch struct {
	ID               string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID           string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID          string         `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	Target           string         `json:"target" gorm:"type:varchar(200);index;not null"`
	WindowTitle      string         `json:"window_title" gorm:"type:varchar(200);default:'微信'"`
	Mode             string         `json:"mode" gorm:"type:varchar(20);default:'suggest_only'"`
	PollIntervalSec  int            `json:"poll_interval_sec" gorm:"default:20"`
	Enabled          bool           `json:"enabled" gorm:"default:true;index"`
	LastImageHash    string         `json:"last_image_hash" gorm:"type:varchar(64)"`
	LastImageURL     string         `json:"last_image_url" gorm:"type:varchar(500)"`
	LastTriggeredAt  *time.Time     `json:"last_triggered_at"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (w *WeChatWatch) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	if w.WindowTitle == "" {
		w.WindowTitle = "微信"
	}
	if w.Mode == "" {
		w.Mode = "suggest_only"
	}
	if w.PollIntervalSec <= 0 {
		w.PollIntervalSec = 20
	}
	return nil
}
