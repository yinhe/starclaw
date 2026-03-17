package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LicenseKey represents an activated license for this Overlord instance.
type LicenseKey struct {
	ID          string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	Key         string     `json:"key" gorm:"type:varchar(255);uniqueIndex;not null"` // the license key string
	Tier        string     `json:"tier" gorm:"type:varchar(20);not null;default:community"` // community/starter/pro/enterprise/whitelabel
	Holder      string     `json:"holder" gorm:"type:varchar(255)"`        // company/person name
	Email       string     `json:"email" gorm:"type:varchar(255)"`
	MaxNodes    int        `json:"max_nodes" gorm:"default:10"`            // 0 = unlimited
	MaxTeams    int        `json:"max_teams" gorm:"default:1"`             // 0 = unlimited
	IssuedAt    time.Time  `json:"issued_at"`
	ExpiresAt   *time.Time `json:"expires_at"`                             // nil = perpetual
	Status      string     `json:"status" gorm:"type:varchar(20);default:active"` // active, expired, revoked
	Fingerprint string     `json:"fingerprint" gorm:"type:varchar(255)"`   // machine/instance fingerprint
	Signature   string     `json:"signature" gorm:"type:varchar(512)"`     // HMAC signature for offline verification
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (l *LicenseKey) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

// IsValid checks if the license is currently valid
func (l *LicenseKey) IsValid() bool {
	if l.Status != "active" {
		return false
	}
	if l.ExpiresAt != nil && l.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// DaysRemaining returns days until expiration, or -1 if perpetual
func (l *LicenseKey) DaysRemaining() int {
	if l.ExpiresAt == nil {
		return -1
	}
	d := time.Until(*l.ExpiresAt).Hours() / 24
	if d < 0 {
		return 0
	}
	return int(d)
}

// GenerateLicenseSignature creates an HMAC-SHA256 signature for offline verification
func GenerateLicenseSignature(key, tier, holder string, maxNodes, maxTeams int, expiresAt *time.Time, secret string) string {
	exp := "perpetual"
	if expiresAt != nil {
		exp = expiresAt.Format(time.RFC3339)
	}
	payload := fmt.Sprintf("%s|%s|%s|%d|%d|%s", key, tier, holder, maxNodes, maxTeams, exp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyLicenseSignature checks the HMAC signature
func VerifyLicenseSignature(l *LicenseKey, secret string) bool {
	expected := GenerateLicenseSignature(l.Key, l.Tier, l.Holder, l.MaxNodes, l.MaxTeams, l.ExpiresAt, secret)
	return hmac.Equal([]byte(l.Signature), []byte(expected))
}
