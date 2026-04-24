package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VideoRecord struct {
	ID               string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID           string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID   string         `json:"conversation_id" gorm:"type:varchar(36);index"`
	TaskID           string         `json:"task_id" gorm:"type:varchar(100);index"` // DashScope task_id
	Model            string         `json:"model" gorm:"type:varchar(100)"`
	Prompt           string         `json:"prompt" gorm:"type:text"`
	VideoURL         string         `json:"video_url" gorm:"type:varchar(1000)"`
	ImgURL           string         `json:"img_url" gorm:"type:varchar(4000)"`
	Size             string         `json:"size" gorm:"type:varchar(20)"`
	Duration         int            `json:"duration" gorm:"default:5"`
	Scene            string         `json:"scene" gorm:"type:varchar(50)"`
	Status           string         `json:"status" gorm:"type:varchar(20);default:pending"`   // pending, running, succeeded, failed, cancelled
	Type             string         `json:"type" gorm:"type:varchar(20);default:clip"`        // clip, merged, narrated, mv
	Category         string         `json:"category" gorm:"type:varchar(30);default:general"` // general, ad, short_drama, short_film, mv, tutorial
	AutonomousTaskID string         `json:"autonomous_task_id" gorm:"type:varchar(36);index"` // links to model.Task for background task tracking
	ClipIDs          string         `json:"clip_ids" gorm:"type:text"`                        // JSON array of clip record IDs (for merged videos)
	Narration        string         `json:"narration" gorm:"type:text"`                       // Narration text for this scene (auto-TTS when clip completes)
	NarrationVoice   string         `json:"narration_voice" gorm:"type:varchar(50)"`          // TTS voice for narration
	NarratedURL      string         `json:"narrated_url" gorm:"type:varchar(1000)"`           // URL of narrated version (with voiceover + subtitles)
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (v *VideoRecord) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	return nil
}
