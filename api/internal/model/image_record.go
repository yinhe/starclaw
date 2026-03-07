package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImageRecord struct {
	ID             string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID string         `json:"conversation_id" gorm:"type:varchar(36);index"`
	TaskID         string         `json:"task_id" gorm:"type:varchar(100);index"`
	Model          string         `json:"model" gorm:"type:varchar(100)"`
	Prompt         string         `json:"prompt" gorm:"type:text"`
	NegativePrompt string         `json:"negative_prompt" gorm:"type:text"`
	Size           string         `json:"size" gorm:"type:varchar(20)"`
	Style          string         `json:"style" gorm:"type:varchar(50)"`
	ImageURL       string         `json:"image_url" gorm:"type:varchar(2000)"`
	LocalURL       string         `json:"local_url" gorm:"type:varchar(500)"`
	Status         string         `json:"status" gorm:"type:varchar(20);default:running"`
	Scene          string         `json:"scene" gorm:"type:varchar(50)"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (r *ImageRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
