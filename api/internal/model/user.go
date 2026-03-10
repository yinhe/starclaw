package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID            string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	Email         *string        `json:"email" gorm:"type:varchar(255);uniqueIndex"`
	Phone         *string        `json:"phone" gorm:"type:varchar(20);uniqueIndex"`
	Username      string         `json:"username" gorm:"type:varchar(100);uniqueIndex;not null"`
	Password      string         `json:"-" gorm:"type:varchar(255);not null"`
	Avatar        string         `json:"avatar" gorm:"type:varchar(500)"`
	Role          string         `json:"role" gorm:"type:varchar(20);default:user"`
	TenantID      string         `json:"tenant_id" gorm:"type:varchar(36);index"`
	OAuthProvider string         `json:"oauth_provider" gorm:"type:varchar(20);index"`
	OAuthID       string         `json:"oauth_id" gorm:"type:varchar(100);index"`
	TokenIssuedAt *time.Time     `json:"token_issued_at,omitempty" gorm:"type:datetime"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}
