package node

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Identity holds this node's cryptographic identity (Ed25519 keypair).
// Node ID = "claw:" + first 40 hex chars of SHA-256(publicKey) = 160 bits, same as Bitcoin address space.
type Identity struct {
	PrivateKey ed25519.PrivateKey `json:"-"`
	PublicKey  ed25519.PublicKey  `json:"public_key"`
	NodeID     string             `json:"node_id"`
	mu         sync.RWMutex
}

// getKeyFile returns the path to the node key file.
// Supports NODE_KEY_PATH env var for Docker volume persistence.
func getKeyFile() string {
	if p := os.Getenv("NODE_KEY_PATH"); p != "" {
		return p
	}
	return ".node_key"
}

// LoadOrCreateIdentity loads keypair from disk, or generates a new one.
func LoadOrCreateIdentity() *Identity {
	keyFile := getKeyFile()
	id := &Identity{}

	// Try loading existing key
	if data, err := os.ReadFile(keyFile); err == nil {
		var stored struct {
			PrivateKey []byte `json:"private_key"`
			PublicKey  []byte `json:"public_key"`
		}
		if json.Unmarshal(data, &stored) == nil && len(stored.PrivateKey) == ed25519.PrivateKeySize {
			id.PrivateKey = stored.PrivateKey
			id.PublicKey = stored.PublicKey
			id.NodeID = deriveNodeID(id.PublicKey)
			log.Printf("[node] loaded identity: %s (fingerprint: %s)", id.NodeID, id.Fingerprint())
			return id
		}
	}

	// Generate new Ed25519 keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("[node] failed to generate keypair: %v", err)
	}

	id.PrivateKey = priv
	id.PublicKey = pub
	id.NodeID = deriveNodeID(pub)

	// Persist
	stored := struct {
		PrivateKey []byte `json:"private_key"`
		PublicKey  []byte `json:"public_key"`
	}{
		PrivateKey: priv,
		PublicKey:  pub,
	}
	data, _ := json.Marshal(stored)
	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		log.Printf("[node] warning: failed to persist key: %v", err)
	}

	log.Printf("[node] generated new identity: %s (fingerprint: %s)", id.NodeID, id.Fingerprint())
	return id
}

// deriveNodeID returns "claw:" + first 40 hex chars of SHA-256(publicKey) = 160 bits
// 160-bit address space supports ~10^24 unique IDs without collision (same as Bitcoin)
func deriveNodeID(pub ed25519.PublicKey) string {
	hash := sha256.Sum256(pub)
	return "claw:" + hex.EncodeToString(hash[:])[:40]
}

// Fingerprint returns a human-readable fingerprint of the public key (like SSH)
func (id *Identity) Fingerprint() string {
	hash := sha256.Sum256(id.PublicKey)
	fp := hex.EncodeToString(hash[:])
	// Format as colon-separated pairs (first 16 bytes = 32 hex chars)
	result := ""
	for i := 0; i < 32 && i < len(fp); i += 2 {
		if result != "" {
			result += ":"
		}
		result += fp[i : i+2]
	}
	return result
}

// PublicKeyHex returns the hex-encoded public key
func (id *Identity) PublicKeyHex() string {
	return hex.EncodeToString(id.PublicKey)
}

// Sign signs a message with this node's private key
func (id *Identity) Sign(message []byte) []byte {
	return ed25519.Sign(id.PrivateKey, message)
}

// SignChallenge creates a signed challenge containing timestamp + node_id
func (id *Identity) SignChallenge() (challenge string, signature string) {
	ch := fmt.Sprintf("%s:%d", id.NodeID, time.Now().Unix())
	sig := id.Sign([]byte(ch))
	return ch, hex.EncodeToString(sig)
}

// VerifySignature verifies a message signature from a remote node
func VerifySignature(publicKeyHex string, message []byte, signatureHex string) bool {
	pubKey, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		return false
	}
	sig, err := hex.DecodeString(signatureHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pubKey, message, sig)
}

// DeriveNodeIDFromPubKey derives node ID from a hex-encoded public key
func DeriveNodeIDFromPubKey(publicKeyHex string) (string, error) {
	pubKey, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid public key")
	}
	return deriveNodeID(pubKey), nil
}

// ── API Token (HMAC-SHA256, server-bound, compact) ──
//
// Format: base64url(uid[16] + iat[4] + hmac[16]) = 36 bytes → 48 chars
// HMAC key = SHA-256(ed25519_private_key + nodeID) — binds token to this server.
// One token per user. Multiple devices can use the same token.
// Device tracking is handled separately via AuthorizedDevice table.

// TokenPayload is the decoded content of an API token.
type TokenPayload struct {
	UserID   string
	IssuedAt int64
}

// tokenHMACKey derives the HMAC signing key from this node's identity.
// key = SHA-256(private_key_bytes + nodeID_string)
func (id *Identity) tokenHMACKey() []byte {
	h := sha256.New()
	h.Write(id.PrivateKey)
	h.Write([]byte(id.NodeID))
	return h.Sum(nil)
}

// GenerateAPIToken creates a compact, server-bound API token for a user.
func (id *Identity) GenerateAPIToken(userID string) string {
	uidBytes, err := uuidToBytes(userID)
	if err != nil {
		return ""
	}

	iat := uint32(time.Now().Unix())
	var iatBuf [4]byte
	iatBuf[0] = byte(iat >> 24)
	iatBuf[1] = byte(iat >> 16)
	iatBuf[2] = byte(iat >> 8)
	iatBuf[3] = byte(iat)

	// payload = uid[16] + iat[4]
	payload := make([]byte, 20)
	copy(payload[:16], uidBytes)
	copy(payload[16:], iatBuf[:])

	// HMAC-SHA256, truncated to 16 bytes (128-bit security)
	mac := hmac.New(sha256.New, id.tokenHMACKey())
	mac.Write(payload)
	sig := mac.Sum(nil)[:16]

	// token = base64url(36 bytes) = 48 chars
	raw := append(payload, sig...)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// VerifyAPIToken verifies a compact token and returns the payload.
// Returns nil if invalid, forged, or meant for a different server.
func (id *Identity) VerifyAPIToken(token string) *TokenPayload {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 36 {
		return nil
	}

	payload := raw[:20]
	sig := raw[20:]

	// Recompute HMAC and compare
	mac := hmac.New(sha256.New, id.tokenHMACKey())
	mac.Write(payload)
	expected := mac.Sum(nil)[:16]
	if !hmac.Equal(sig, expected) {
		return nil
	}

	userID := bytesToUUID(payload[:16])
	iat := int64(payload[16])<<24 | int64(payload[17])<<16 | int64(payload[18])<<8 | int64(payload[19])

	return &TokenPayload{UserID: userID, IssuedAt: iat}
}

// uuidToBytes converts "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" → 16 bytes
func uuidToBytes(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return nil, fmt.Errorf("invalid uuid")
	}
	return hex.DecodeString(s)
}

// bytesToUUID converts 16 bytes → "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
func bytesToUUID(b []byte) string {
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
