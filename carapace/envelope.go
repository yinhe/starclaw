package carapace

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
)

// DataKey represents a versioned encryption key, encrypted by the master key.
type DataKey struct {
	Version      int
	EncryptedKey string // base64(AES-GCM(masterKey, rawKey))
	Status       string // "active", "decrypt_only", "retired"
	rawKey       []byte // decrypted key, cached in memory
}

// envelope manages the master key → data key → data encryption chain.
type envelope struct {
	mu         sync.RWMutex
	masterKey  []byte
	dataKeys   map[int]*DataKey // version → DataKey
	currentVer int              // latest active version
}

func newEnvelope(masterKey []byte) *envelope {
	return &envelope{
		masterKey: masterKey,
		dataKeys:  make(map[int]*DataKey),
	}
}

// initDataKey creates the first data key (v1) if none exist.
func (e *envelope) initDataKey() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentVer > 0 {
		return nil // already initialized
	}

	return e.createDataKeyLocked(1)
}

// createDataKeyLocked generates a new data key at the given version.
// Caller must hold e.mu write lock.
func (e *envelope) createDataKeyLocked(version int) error {
	// Generate random 32-byte data key
	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		return fmt.Errorf("carapace: generate data key: %w", err)
	}

	// Encrypt data key with master key
	encrypted, err := aesEncrypt(e.masterKey, rawKey)
	if err != nil {
		return fmt.Errorf("carapace: encrypt data key: %w", err)
	}

	dk := &DataKey{
		Version:      version,
		EncryptedKey: base64.StdEncoding.EncodeToString(encrypted),
		Status:       "active",
		rawKey:       rawKey,
	}

	// Demote previous active key
	if prev, ok := e.dataKeys[e.currentVer]; ok && prev.Status == "active" {
		prev.Status = "decrypt_only"
	}

	e.dataKeys[version] = dk
	e.currentVer = version
	return nil
}

// rotateDataKey creates a new data key version.
func (e *envelope) rotateDataKey() (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	newVer := e.currentVer + 1
	if err := e.createDataKeyLocked(newVer); err != nil {
		return 0, err
	}
	return newVer, nil
}

// seal encrypts plaintext with the current data key + purpose derivation.
func (e *envelope) seal(purpose, plaintext string) (string, error) {
	e.mu.RLock()
	dk := e.dataKeys[e.currentVer]
	ver := e.currentVer
	e.mu.RUnlock()

	if dk == nil {
		return "", fmt.Errorf("carapace: no active data key")
	}

	// Derive purpose-specific key from data key
	derived := deriveKey(dk.rawKey, purpose)

	// Encrypt
	encoded, err := aesEncryptBase64(derived, plaintext)
	if err != nil {
		return "", fmt.Errorf("carapace: seal: %w", err)
	}

	return formatSealed(ver, encoded), nil
}

// unseal decrypts a stored ciphertext. Handles all formats:
// - enc:vN:{base64} → envelope decryption with data key vN
// - enc:{base64}    → legacy decryption with master key directly
// - plaintext       → returned as-is (migration support)
func (e *envelope) unseal(purpose, stored string) (string, error) {
	sf, encrypted := parseSealed(stored)
	if !encrypted {
		return stored, nil // plaintext passthrough
	}

	if sf.Version == 0 {
		// Legacy format: decrypt with master key directly (Phase 1-2 compat)
		derived := deriveKeySimple(e.masterKey, purpose)
		plain, err := aesDecryptBase64(derived, sf.Payload)
		if err != nil {
			return "", fmt.Errorf("carapace: unseal legacy: %w", err)
		}
		return plain, nil
	}

	// Versioned format: use data key
	e.mu.RLock()
	dk := e.dataKeys[sf.Version]
	e.mu.RUnlock()

	if dk == nil {
		return "", fmt.Errorf("carapace: data key v%d not found", sf.Version)
	}

	derived := deriveKey(dk.rawKey, purpose)
	plain, err := aesDecryptBase64(derived, sf.Payload)
	if err != nil {
		return "", fmt.Errorf("carapace: unseal v%d: %w", sf.Version, err)
	}
	return plain, nil
}

// reEncrypt decrypts a value and re-encrypts with the current data key.
func (e *envelope) reEncrypt(purpose, stored string) (string, error) {
	plain, err := e.unseal(purpose, stored)
	if err != nil {
		return "", err
	}
	return e.seal(purpose, plain)
}

// loadDataKey loads a data key from its encrypted form (from DB).
func (e *envelope) loadDataKey(version int, encryptedKeyBase64, status string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Decrypt the data key with master key
	encBytes, err := base64.StdEncoding.DecodeString(encryptedKeyBase64)
	if err != nil {
		return fmt.Errorf("carapace: decode data key v%d: %w", version, err)
	}

	rawKey, err := aesDecrypt(e.masterKey, encBytes)
	if err != nil {
		return fmt.Errorf("carapace: decrypt data key v%d: %w", version, err)
	}

	e.dataKeys[version] = &DataKey{
		Version:      version,
		EncryptedKey: encryptedKeyBase64,
		Status:       status,
		rawKey:       rawKey,
	}

	if version > e.currentVer && status == "active" {
		e.currentVer = version
	}

	return nil
}
