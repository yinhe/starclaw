package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// ════════════════════════════════════════════════════════════════
//  AES-256-GCM Encryption Engine
// ════════════════════════════════════════════════════════════════

// KeyManager manages encryption keys for data-at-rest encryption.
type KeyManager struct {
	mu        sync.RWMutex
	masterKey []byte // 32 bytes for AES-256
}

// NewKeyManager creates a key manager. It loads the master key from
// the STARCLAW_MASTER_KEY environment variable, or generates one if not set.
func NewKeyManager() (*KeyManager, error) {
	km := &KeyManager{}

	envKey := os.Getenv("STARCLAW_MASTER_KEY")
	if envKey != "" {
		key, err := hex.DecodeString(envKey)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("STARCLAW_MASTER_KEY must be 64 hex chars (32 bytes)")
		}
		km.masterKey = key
	} else {
		// Generate a random key (WARNING: data will be unreadable after restart if not persisted)
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		km.masterKey = key
	}

	return km, nil
}

// MasterKeyFingerprint returns a safe identifier for the current master key.
func (km *KeyManager) MasterKeyFingerprint() string {
	km.mu.RLock()
	defer km.mu.RUnlock()
	h := sha256.Sum256(km.masterKey)
	return hex.EncodeToString(h[:8]) // first 8 bytes
}

// DeriveKey derives a purpose-specific key from the master key using HKDF-like approach.
func (km *KeyManager) DeriveKey(purpose string) []byte {
	km.mu.RLock()
	defer km.mu.RUnlock()
	h := sha256.New()
	h.Write(km.masterKey)
	h.Write([]byte(purpose))
	return h.Sum(nil) // 32 bytes
}

// ════════════════════════════════════════════════════════════════
//  Encrypt / Decrypt
// ════════════════════════════════════════════════════════════════

// Encrypt encrypts plaintext using AES-256-GCM with the derived key for the given purpose.
// Returns base64-encoded ciphertext (nonce prepended).
func (km *KeyManager) Encrypt(plaintext []byte, purpose string) (string, error) {
	key := km.DeriveKey(purpose)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil) // nonce || ciphertext || tag
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM.
func (km *KeyManager) Decrypt(encoded string, purpose string) ([]byte, error) {
	key := km.DeriveKey(purpose)

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptString is a convenience wrapper for string data.
func (km *KeyManager) EncryptString(s, purpose string) (string, error) {
	return km.Encrypt([]byte(s), purpose)
}

// DecryptString is a convenience wrapper for string data.
func (km *KeyManager) DecryptString(encoded, purpose string) (string, error) {
	b, err := km.Decrypt(encoded, purpose)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ════════════════════════════════════════════════════════════════
//  Package-level singleton for API key encryption
// ════════════════════════════════════════════════════════════════

var globalKeyMgr *KeyManager

// SetGlobalKeyManager sets the package-level KeyManager singleton.
// Call this once at startup after creating the KeyManager.
func SetGlobalKeyManager(km *KeyManager) {
	globalKeyMgr = km
}

// GetGlobalKeyManager returns the package-level KeyManager singleton.
func GetGlobalKeyManager() *KeyManager {
	return globalKeyMgr
}

// EncryptAPIKey encrypts an API key for storage. Returns "enc:" prefixed ciphertext.
// If KeyManager is not set or key is empty, returns the original string unchanged.
func EncryptAPIKey(apiKey string) string {
	if globalKeyMgr == nil || apiKey == "" {
		return apiKey
	}
	// Already encrypted — don't double-encrypt
	if len(apiKey) > 4 && apiKey[:4] == "enc:" {
		return apiKey
	}
	enc, err := globalKeyMgr.EncryptString(apiKey, "api_key")
	if err != nil {
		return apiKey // fallback to plaintext on error
	}
	return "enc:" + enc
}

// DecryptAPIKey decrypts an API key from storage. Handles both encrypted ("enc:" prefix)
// and plaintext values (for seamless migration from unencrypted to encrypted).
func DecryptAPIKey(stored string) string {
	if globalKeyMgr == nil || stored == "" {
		return stored
	}
	// Not encrypted — return as-is (migration support)
	if len(stored) < 4 || stored[:4] != "enc:" {
		return stored
	}
	dec, err := globalKeyMgr.DecryptString(stored[4:], "api_key")
	if err != nil {
		return "" // corrupted — return empty rather than garbage
	}
	return dec
}
