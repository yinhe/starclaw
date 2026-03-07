package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationType represents the category of notification
type NotificationType string

const (
	NotifyTaskComplete NotificationType = "task_complete"
	NotifyTaskFailed   NotificationType = "task_failed"
	NotifyTaskProgress NotificationType = "task_progress"
	NotifyInfo         NotificationType = "info"
	NotifyWarning      NotificationType = "warning"
	NotifySuccess      NotificationType = "success"
)

// Notification represents a message sent to the user from background tasks
type Notification struct {
	ID        string           `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string           `json:"user_id" gorm:"type:varchar(36);index;not null"`
	TaskID    string           `json:"task_id" gorm:"type:varchar(36);index"`
	Type      NotificationType `json:"type" gorm:"type:varchar(30);index;default:'info'"`
	Title     string           `json:"title" gorm:"type:varchar(500);not null"`
	Content   string           `json:"content" gorm:"type:longtext"`
	IsRead    bool             `json:"is_read" gorm:"default:false;index"`
	CreatedAt time.Time        `json:"created_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}
