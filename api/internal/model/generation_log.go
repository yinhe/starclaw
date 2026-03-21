package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GenerationLog is a unified audit trail for ALL media generation (image/video/audio).
// Mirrors Router's generations table for easy reconciliation.
type GenerationLog struct {
	ID             string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID string     `json:"conversation_id" gorm:"type:varchar(36);index"`
	SuperProvider  string     `json:"super_provider" gorm:"type:varchar(50);index"` // "starai" = via StarAI Router proxy (needs reconciliation), "direct" = direct API key
	Provider       string     `json:"provider" gorm:"type:varchar(50)"`             // upstream: fal, dashscope, minimax
	Model          string     `json:"model" gorm:"type:varchar(100)"`               // veo3, flux-dev, wan2.6-t2v, etc.
	Type           string     `json:"type" gorm:"type:varchar(20);index"`           // image, video, audio
	TaskID         string     `json:"task_id" gorm:"type:varchar(200);index"`       // fal request_id or dashscope task_id
	RecordID       string     `json:"record_id" gorm:"type:varchar(36);index"`      // FK to ImageRecord/VideoRecord/MusicRecord
	Prompt         string     `json:"prompt" gorm:"type:text"`
	Status         string     `json:"status" gorm:"type:varchar(20);index;default:pending"` // pending, running, succeeded, failed
	ResultURL      string     `json:"result_url" gorm:"type:varchar(2000)"`
	CostFen        float64    `json:"cost_fen" gorm:"default:0"` // upstream cost in 分 (for reconciliation with Router)
	Duration       int        `json:"duration" gorm:"default:0"` // video/audio duration in seconds
	Width          int        `json:"width" gorm:"default:0"`
	Height         int        `json:"height" gorm:"default:0"`
	ErrorMsg       string     `json:"error_msg" gorm:"type:text"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (g *GenerationLog) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	if g.StartedAt.IsZero() {
		g.StartedAt = time.Now()
	}
	return nil
}
