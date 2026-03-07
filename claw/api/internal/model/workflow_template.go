package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowTemplate represents a shared workflow template in the marketplace
type WorkflowTemplate struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Category    string    `json:"category" gorm:"type:varchar(50);index"` // e.g. automation, data, content
	Definition  string    `json:"definition" gorm:"type:longtext"`
	CloneCount  int       `json:"clone_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (t *WorkflowTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}
