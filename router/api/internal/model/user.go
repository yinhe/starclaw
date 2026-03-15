package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a star-ai.net user (linked to Queen unified user system)
type User struct {
	ID           string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	Email        string         `json:"email" gorm:"type:varchar(200);uniqueIndex"`
	Phone        string         `json:"phone" gorm:"type:varchar(20);uniqueIndex"`
	PasswordHash string         `json:"-" gorm:"type:varchar(100)"`
	Name         string         `json:"name" gorm:"type:varchar(100)"`
	Balance      int64          `json:"balance" gorm:"default:0"`          // balance in cents (分)
	FreeQuota    int64          `json:"free_quota" gorm:"default:1000000"` // free tokens for new users
	IsAdmin      bool           `json:"is_admin" gorm:"default:false"`
	Status       string         `json:"status" gorm:"type:varchar(20);default:'active'"`
	ClawID       string         `json:"claw_id" gorm:"type:varchar(60);uniqueIndex"` // claw:xxxx address (Ed25519 identity)
	QueenUID     string         `json:"queen_uid" gorm:"type:varchar(36);index"`     // linked Queen user ID
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}
