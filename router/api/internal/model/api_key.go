package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
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
	KeyEnc    string         `json:"-" gorm:"type:varchar(200)"`            // AES-GCM encrypted full key
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

var keyEncSecret []byte // 32-byte AES key, loaded from env

func getKeyEncSecret() []byte {
	if keyEncSecret != nil {
		return keyEncSecret
	}
	s := os.Getenv("API_KEY_SECRET")
	if s == "" {
		s = "starclaw-api-key-encrypt-secret!" // 32 bytes default
	}
	keyEncSecret = []byte(s)[:32]
	return keyEncSecret
}

// EncryptKey encrypts an API key for storage.
func EncryptKey(plainKey string) string {
	block, err := aes.NewCipher(getKeyEncSecret())
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ct := gcm.Seal(nonce, nonce, []byte(plainKey), nil)
	return base64.StdEncoding.EncodeToString(ct)
}

// DecryptKey decrypts a stored API key.
func DecryptKey(enc string) string {
	if enc == "" {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(getKeyEncSecret())
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return ""
	}
	plain, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return ""
	}
	return string(plain)
}
