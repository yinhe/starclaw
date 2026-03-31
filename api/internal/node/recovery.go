package node

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// ── Mnemonic-based identity recovery ──
//
// Every Claw node has a 24-word BIP-39 mnemonic that can regenerate its Ed25519 key.
// Flow:
//   New install  → LoadOrCreateIdentity → SeedToMnemonic(seed) → show to user → user saves words
//   Recovery     → user enters 24 words → MnemonicToSeed → ed25519.NewKeyFromSeed → same node_id
//   Cloud backup → encrypt(agents+config, mnemonic-derived AES key) → upload to Queen
//   Restore      → enter mnemonic → download encrypted blob from Queen → decrypt → import

// IdentityMnemonic returns the 24-word mnemonic for the given identity.
// This is the "master password" — losing it means losing the identity.
func IdentityMnemonic(id *Identity) (string, error) {
	seed := id.PrivateKey.Seed()
	return SeedToMnemonic(seed)
}

// IdentityFromMnemonic restores an Identity from a 24-word BIP-39 mnemonic.
func IdentityFromMnemonic(mnemonic string) (*Identity, error) {
	seed, err := MnemonicToSeed(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("invalid mnemonic: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed size mismatch: got %d, want %d", len(seed), ed25519.SeedSize)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
		NodeID:     deriveNodeID(pub),
	}, nil
}

// RestoreIdentityFromMnemonic restores identity from mnemonic and persists to disk.
// Returns the restored identity. Caller should restart the server for full effect.
func RestoreIdentityFromMnemonic(mnemonic string) (*Identity, error) {
	id, err := IdentityFromMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}

	keyFile := getKeyFile()
	stored := struct {
		PrivateKey []byte `json:"private_key"`
		PublicKey  []byte `json:"public_key"`
	}{id.PrivateKey, id.PublicKey}
	data, _ := json.Marshal(stored)
	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to persist restored key: %w", err)
	}

	log.Printf("[node] identity restored from mnemonic: %s", id.NodeID)
	return id, nil
}

// ── Encrypted cloud backup ──

// BackupLookupKey derives a lookup key from the mnemonic for Queen storage.
// Queen stores backups keyed by this hash — it cannot reverse to the mnemonic.
// lookup = hex(SHA256("starclaw-backup-lookup:" + mnemonic))[:32]
func BackupLookupKey(mnemonic string) string {
	h := sha256.Sum256([]byte("starclaw-backup-lookup:" + mnemonic))
	return hex.EncodeToString(h[:16])
}

// backupAESKey derives an AES-256 key from the mnemonic for encrypting backups.
func backupAESKey(mnemonic string) []byte {
	return pbkdf2.Key([]byte(mnemonic), []byte("starclaw-cloud-backup"), 4096, 32, sha256.New)
}

// EncryptBackup encrypts a backup payload with a mnemonic-derived AES-256-GCM key.
func EncryptBackup(payload []byte, mnemonic string) ([]byte, error) {
	key := backupAESKey(mnemonic)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, payload, nil), nil
}

// DecryptBackup decrypts a backup blob using the mnemonic-derived AES-256-GCM key.
func DecryptBackup(encrypted []byte, mnemonic string) ([]byte, error) {
	key := backupAESKey(mnemonic)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(encrypted) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := encrypted[:gcm.NonceSize()]
	ciphertext := encrypted[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ── Backup payload structure ──

// BackupPayload is the JSON structure encrypted and uploaded to Queen.
type BackupPayload struct {
	NodeID    string          `json:"node_id"`
	Version   int             `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
	Identity  BackupKey       `json:"identity"`
	Agents    []BackupAgent   `json:"agents"`
	Config    json.RawMessage `json:"config,omitempty"`
}

// BackupKey holds the essential identity data for recovery.
type BackupKey struct {
	PrivateKey []byte `json:"private_key"`
	PublicKey  []byte `json:"public_key"`
}

// BackupAgent holds a single agent's exportable data.
type BackupAgent struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	Tools        string `json:"tools"`
	Config       string `json:"config"`
	Icon         string `json:"icon"`
	SourceID     string `json:"source_id,omitempty"`
}

// BuildBackupPayload creates the backup payload from current identity and agent list.
func BuildBackupPayload(id *Identity, agents []BackupAgent, config json.RawMessage) *BackupPayload {
	return &BackupPayload{
		NodeID:    id.NodeID,
		Version:   1,
		Timestamp: time.Now(),
		Identity: BackupKey{
			PrivateKey: id.PrivateKey,
			PublicKey:  id.PublicKey,
		},
		Agents: agents,
		Config: config,
	}
}

// ── Recovery status tracking ──

// RecoveryStatus tracks what recovery steps the user has completed.
type RecoveryStatus struct {
	MnemonicSaved bool   `json:"mnemonic_saved"`
	PhoneBound    bool   `json:"phone_bound"`
	Phone         string `json:"phone,omitempty"` // masked: 138****1234
	BackupExists  bool   `json:"backup_exists"`
	BackupTime    string `json:"backup_time,omitempty"`
}

// recoveryStatusFile is the legacy status filename (now uses getKeyFile() + ".recovery").
const _ = ".recovery_status"

// LoadRecoveryStatus reads the local recovery status.
func LoadRecoveryStatus() *RecoveryStatus {
	statusFile := getKeyFile() + ".recovery"
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return &RecoveryStatus{}
	}
	var s RecoveryStatus
	json.Unmarshal(data, &s)
	return &s
}

// SaveRecoveryStatus persists recovery status to disk.
func SaveRecoveryStatus(s *RecoveryStatus) {
	statusFile := getKeyFile() + ".recovery"
	data, _ := json.Marshal(s)
	os.WriteFile(statusFile, data, 0600)
}
