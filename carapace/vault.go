package carapace

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
)

// MasterKeyBackend provides the master key from various sources.
type MasterKeyBackend interface {
	LoadMasterKey() ([]byte, error)
	Name() string
}

// VaultInfo contains vault status information.
type VaultInfo struct {
	Backend           string `json:"backend"`
	MasterFingerprint string `json:"master_fingerprint"`
	CurrentKeyVersion int    `json:"current_key_version"`
	TotalKeyVersions  int    `json:"total_key_versions"`
	Initialized       bool   `json:"initialized"`
}

// Config for creating a new Vault.
type Config struct {
	Backend MasterKeyBackend
	Service string // caller identity for audit: "hive", "queen", etc.
}

// Vault is the main secret management interface.
type Vault struct {
	mu       sync.RWMutex
	env      *envelope
	audit    *AuditLog
	backend  MasterKeyBackend
	service  string
	masterFP string // fingerprint of master key
}

// New creates a new Vault with the given configuration.
// If backend is nil, generates an ephemeral master key (WARNING: data lost on restart).
func New(cfg Config) (*Vault, error) {
	var masterKey []byte
	var backendName string

	if cfg.Backend != nil {
		var err error
		masterKey, err = cfg.Backend.LoadMasterKey()
		if err != nil {
			return nil, fmt.Errorf("carapace: load master key: %w", err)
		}
		backendName = cfg.Backend.Name()
	} else {
		// Ephemeral key — for dev/testing only
		masterKey = make([]byte, 32)
		if _, err := rand.Read(masterKey); err != nil {
			return nil, err
		}
		backendName = "ephemeral"
		log.Printf("[carapace] WARNING: using ephemeral master key — secrets will be lost on restart!")
	}

	service := cfg.Service
	if service == "" {
		service = "unknown"
	}

	fp := sha256.Sum256(masterKey)

	v := &Vault{
		env:      newEnvelope(masterKey),
		audit:    newAuditLog(service),
		backend:  cfg.Backend,
		service:  service,
		masterFP: hex.EncodeToString(fp[:8]),
	}

	// Initialize first data key
	if err := v.env.initDataKey(); err != nil {
		return nil, err
	}

	log.Printf("[carapace] vault ready (backend=%s, fingerprint=%s, service=%s)", backendName, v.masterFP, service)
	return v, nil
}

// Seal encrypts a plaintext secret for storage.
// Returns formatted ciphertext: "enc:v{N}:{base64}"
func (v *Vault) Seal(purpose, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	result, err := v.env.seal(purpose, plaintext)

	v.env.mu.RLock()
	ver := v.env.currentVer
	v.env.mu.RUnlock()
	v.audit.record("seal", purpose, ver, err)

	return result, err
}

// Unseal decrypts a stored secret.
// Handles all formats: enc:vN:, enc:, and plaintext (migration).
func (v *Vault) Unseal(purpose, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	result, err := v.env.unseal(purpose, ciphertext)

	sf, encrypted := parseSealed(ciphertext)
	ver := 0
	if encrypted {
		ver = sf.Version
	}
	v.audit.record("unseal", purpose, ver, err)

	return result, err
}

// RotateDataKey creates a new data key version.
// Old keys are retained for decryption only.
// Returns the new version number and the encrypted data key (for DB storage).
func (v *Vault) RotateDataKey() (version int, encryptedKey string, err error) {
	ver, err := v.env.rotateDataKey()
	v.audit.record("rotate", "*", ver, err)
	if err != nil {
		return 0, "", err
	}

	v.env.mu.RLock()
	dk := v.env.dataKeys[ver]
	v.env.mu.RUnlock()

	log.Printf("[carapace] rotated to data key v%d", ver)
	return ver, dk.EncryptedKey, nil
}

// ReEncrypt decrypts a value and re-encrypts with the current data key.
// Use for gradual migration of old-format secrets.
func (v *Vault) ReEncrypt(purpose, ciphertext string) (string, error) {
	result, err := v.env.reEncrypt(purpose, ciphertext)
	v.audit.record("re-encrypt", purpose, v.env.currentVer, err)
	return result, err
}

// LoadDataKey loads a previously stored data key (from DB) into the vault.
// Call this at startup to restore data keys from persistent storage.
func (v *Vault) LoadDataKey(version int, encryptedKeyBase64, status string) error {
	return v.env.loadDataKey(version, encryptedKeyBase64, status)
}

// ExportDataKeys returns all data keys in their encrypted form (for DB persistence).
func (v *Vault) ExportDataKeys() []DataKey {
	v.env.mu.RLock()
	defer v.env.mu.RUnlock()

	keys := make([]DataKey, 0, len(v.env.dataKeys))
	for _, dk := range v.env.dataKeys {
		keys = append(keys, DataKey{
			Version:      dk.Version,
			EncryptedKey: dk.EncryptedKey,
			Status:       dk.Status,
		})
	}
	return keys
}

// Info returns vault status.
func (v *Vault) Info() VaultInfo {
	v.env.mu.RLock()
	defer v.env.mu.RUnlock()

	backendName := "ephemeral"
	if v.backend != nil {
		backendName = v.backend.Name()
	}

	return VaultInfo{
		Backend:           backendName,
		MasterFingerprint: v.masterFP,
		CurrentKeyVersion: v.env.currentVer,
		TotalKeyVersions:  len(v.env.dataKeys),
		Initialized:       v.env.currentVer > 0,
	}
}

// Audit returns the audit log for inspection.
func (v *Vault) Audit() *AuditLog {
	return v.audit
}

// Fingerprint returns the master key fingerprint (safe to display).
func (v *Vault) Fingerprint() string {
	return v.masterFP
}
