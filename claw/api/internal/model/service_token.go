package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ServiceToken grants external services access to Claw API capabilities.
// Created when a Claw owner approves an auth-request from an external service.
// Works like an API key: long-lived, revocable, scoped.
type ServiceToken struct {
	ID          string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	Token       string         `json:"-" gorm:"type:varchar(64);uniqueIndex;not null"`
	Name        string         `json:"name" gorm:"type:varchar(100)"`
	Origin      string         `json:"origin" gorm:"type:varchar(200)"`
	Permissions string         `json:"permissions" gorm:"type:varchar(500);default:'chat'"` // chat, tools, all
	UserID      string         `json:"user_id" gorm:"type:varchar(36);index"`
	Revoked     bool           `json:"revoked" gorm:"default:false"`
	LastUsedAt  *time.Time     `json:"last_used_at"`
	ExpiresAt   *time.Time     `json:"expires_at"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (t *ServiceToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Token == "" {
		t.Token = GenerateServiceToken()
	}
	return nil
}

// IsValid checks if the token is active (not revoked, not expired).
func (t *ServiceToken) IsValid() bool {
	if t.Revoked {
		return false
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return false
	}
	return true
}

// GenerateServiceToken creates a random 32-byte hex token prefixed with "svc-".
func GenerateServiceToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "svc-" + hex.EncodeToString(b)
}
