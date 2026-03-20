package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// APIKey represents a user's API key for star-ai.net (sk-star-xxx format)
type APIKey struct {
	ID        string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ClawID    string         `json:"claw_id" gorm:"type:varchar(60);index"` // bound Claw instance (claw:<hash>), empty for manual keys
	Name      string         `json:"name" gorm:"type:varchar(100)"`         // user-defined label
	KeyHash   string         `json:"-" gorm:"type:varchar(64);uniqueIndex"` // sha256 of the key
	KeyPrefix string         `json:"key_prefix" gorm:"type:varchar(20)"`    // "sk-star-a1b2" for display
	IsEnabled bool           `json:"is_enabled" gorm:"default:true"`
	LastUsed  *time.Time     `json:"last_used"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (k *APIKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	return nil
}

// GenerateAPIKey creates a new sk-star-xxx key (returned once, only hash stored)
func GenerateAPIKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return "sk-star-" + hex.EncodeToString(b)
}
