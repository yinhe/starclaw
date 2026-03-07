package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MusicRecord struct {
	ID             string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID string         `json:"conversation_id" gorm:"type:varchar(36);index"`
	RequestID      string         `json:"request_id" gorm:"type:varchar(200);index"`      // fal.ai queue request_id
	Model          string         `json:"model" gorm:"type:varchar(100)"`                 // ace-step, minimax-music-v2, diffrhythm, etc.
	Prompt         string         `json:"prompt" gorm:"type:text"`                        // style/genre description
	Lyrics         string         `json:"lyrics" gorm:"type:text"`                        // song lyrics with structure tags
	AudioURL       string         `json:"audio_url" gorm:"type:varchar(1000)"`            // remote URL from fal.ai
	LocalURL       string         `json:"local_url" gorm:"type:varchar(1000)"`            // local served URL (/v1/music/...)
	Duration       int            `json:"duration" gorm:"default:60"`                     // requested duration in seconds
	Status         string         `json:"status" gorm:"type:varchar(20);default:pending"` // pending, running, succeeded, failed
	ErrorMsg       string         `json:"error_msg" gorm:"type:text"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (m *MusicRecord) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
