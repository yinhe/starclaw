package service

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

// CryptoService provides AES-256-GCM encryption for sensitive fields.
// Used to protect DBPassword, JWTSecret, etc. in the Hive database.
type CryptoService struct {
	mu        sync.RWMutex
	masterKey []byte // 32 bytes for AES-256
}

func NewCryptoService() (*CryptoService, error) {
	cs := &CryptoService{}

	envKey := os.Getenv("HIVE_MASTER_KEY")
	if envKey != "" {
		key, err := hex.DecodeString(envKey)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("HIVE_MASTER_KEY must be 64 hex chars (32 bytes)")
		}
		cs.masterKey = key
	} else {
		// Generate a random key — WARNING: data unreadable after restart if not persisted
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		cs.masterKey = key
		fmt.Printf("[hive] ⚠️  HIVE_MASTER_KEY not set, generated ephemeral key: %s\n", hex.EncodeToString(key))
		fmt.Println("[hive] ⚠️  Set HIVE_MASTER_KEY in .env to persist encryption across restarts!")
	}

	return cs, nil
}

// deriveKey derives a purpose-specific 32-byte key from the master key.
func (cs *CryptoService) deriveKey(purpose string) []byte {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	h := sha256.New()
	h.Write(cs.masterKey)
	h.Write([]byte(purpose))
	return h.Sum(nil)
}

// Encrypt encrypts plaintext with AES-256-GCM using a purpose-derived key.
// Returns base64-encoded ciphertext (nonce prepended).
func (cs *CryptoService) Encrypt(plaintext, purpose string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key := cs.deriveKey(purpose)
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

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value encrypted by Encrypt.
// If the value doesn't have the "enc:" prefix, it's returned as-is (plaintext migration).
func (cs *CryptoService) Decrypt(encoded, purpose string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	// Support plaintext values (for migration from unencrypted → encrypted)
	if len(encoded) < 4 || encoded[:4] != "enc:" {
		return encoded, nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded[4:])
	if err != nil {
		return "", err
	}

	key := cs.deriveKey(purpose)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Fingerprint returns a safe identifier for the current master key.
func (cs *CryptoService) Fingerprint() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	h := sha256.Sum256(cs.masterKey)
	return hex.EncodeToString(h[:8])
}

// GenerateMasterKey generates a random 32-byte key and prints it as hex.
// Use this to create a HIVE_MASTER_KEY value.
func GenerateMasterKey() string {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)
}
