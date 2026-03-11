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
	OwnerToken    *string        `json:"-" gorm:"type:varchar(70);uniqueIndex"`
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

// AuthorizedDevice tracks devices that have logged in via API token.
// One token per user, but multiple devices can use it.
// Individual devices can be revoked without affecting others.
type AuthorizedDevice struct {
	ID         string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID     string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	DeviceID   string     `json:"device_id" gorm:"type:varchar(36);not null"`
	DeviceName string     `json:"device_name" gorm:"type:varchar(100)"`
	Revoked    bool       `json:"revoked" gorm:"default:false"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (d *AuthorizedDevice) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
