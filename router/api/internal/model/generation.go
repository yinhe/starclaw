package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Generation tracks async media generation tasks (video, image, audio)
type Generation struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID       string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ClawID       string    `json:"claw_id" gorm:"type:varchar(60);index"`
	Provider     string    `json:"provider" gorm:"type:varchar(50)"`          // dashscope, fal
	Model        string    `json:"model" gorm:"type:varchar(100)"`            // wan2.6-t2v, kling-v3, veo3
	Type         string    `json:"type" gorm:"type:varchar(20)"`              // video, image, audio
	TaskID       string    `json:"task_id" gorm:"type:varchar(200);index"`    // upstream async task ID
	Prompt       string    `json:"prompt" gorm:"type:text"`
	Status       string    `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending, running, succeeded, failed
	ResultURL    string    `json:"result_url" gorm:"type:varchar(500)"`
	ThumbnailURL string    `json:"thumbnail_url" gorm:"type:varchar(500)"`
	CostCents    float64   `json:"cost_cents" gorm:"type:double;default:0"`
	Duration     int       `json:"duration" gorm:"default:0"`                // requested duration in seconds (for video)
	Width        int       `json:"width" gorm:"default:0"`
	Height       int       `json:"height" gorm:"default:0"`
	ErrorMsg     string    `json:"error_msg,omitempty" gorm:"type:text"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (g *Generation) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}
