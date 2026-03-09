package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Identity holds this node's cryptographic identity (Ed25519 keypair).
// Node ID = first 16 hex chars of SHA-256(publicKey), similar to a blockchain address.
type Identity struct {
	PrivateKey ed25519.PrivateKey `json:"-"`
	PublicKey  ed25519.PublicKey  `json:"public_key"`
	NodeID    string             `json:"node_id"`
	mu        sync.RWMutex
}

const keyFile = ".node_key"

// LoadOrCreateIdentity loads keypair from disk, or generates a new one.
func LoadOrCreateIdentity() *Identity {
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
		PublicKey:   pub,
	}
	data, _ := json.Marshal(stored)
	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		log.Printf("[node] warning: failed to persist key: %v", err)
	}

	log.Printf("[node] generated new identity: %s (fingerprint: %s)", id.NodeID, id.Fingerprint())
	return id
}

// deriveNodeID returns first 16 hex chars of SHA-256(publicKey)
func deriveNodeID(pub ed25519.PublicKey) string {
	hash := sha256.Sum256(pub)
	return hex.EncodeToString(hash[:])[:16]
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
