package node

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ── m-of-n Multi-Signature ──
//
// MultiSig allows m out of n claw nodes to jointly authorize an operation.
// Each signer signs the same message independently with their Ed25519 key.
// The verifier collects at least m valid signatures to consider the operation authorized.
//
// Use cases:
//   - Large compute credit transfers require 2-of-3 node approval
//   - Governance votes on swarm parameters
//   - Emergency key rotation with quorum

// MultiSigPolicy defines the m-of-n requirements for a multi-sig group.
type MultiSigPolicy struct {
	Threshold  int      `json:"threshold"`   // m: minimum signatures required
	Signers    []string `json:"signers"`     // n: list of authorized claw addresses (node IDs)
	Label      string   `json:"label"`       // human-readable name for this policy
	CreatedAt  int64    `json:"created_at"`
}

// MultiSigSignature is a single signer's contribution to a multi-sig operation.
type MultiSigSignature struct {
	SignerID  string `json:"signer_id"`  // claw address of the signer
	PublicKey string `json:"public_key"` // hex-encoded Ed25519 public key
	Signature string `json:"signature"`  // hex-encoded signature
	Timestamp int64  `json:"timestamp"`
}

// MultiSigRequest represents a pending multi-sig operation awaiting signatures.
type MultiSigRequest struct {
	ID         string               `json:"id"`
	Policy     *MultiSigPolicy      `json:"policy"`
	Message    []byte               `json:"message"`     // the message/transaction to be signed
	MessageHex string               `json:"message_hex"` // hex for display
	Signatures []MultiSigSignature  `json:"signatures"`
	Status     string               `json:"status"`      // "pending", "approved", "expired"
	CreatedAt  int64                `json:"created_at"`
	ExpiresAt  int64                `json:"expires_at"`
}

// NewMultiSigPolicy creates a new m-of-n policy.
func NewMultiSigPolicy(threshold int, signers []string, label string) (*MultiSigPolicy, error) {
	if threshold < 1 {
		return nil, fmt.Errorf("threshold must be >= 1")
	}
	if threshold > len(signers) {
		return nil, fmt.Errorf("threshold (%d) cannot exceed number of signers (%d)", threshold, len(signers))
	}
	if len(signers) < 2 {
		return nil, fmt.Errorf("multi-sig requires at least 2 signers")
	}

	// Deduplicate and sort signers for deterministic ordering
	seen := make(map[string]bool)
	unique := []string{}
	for _, s := range signers {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	sort.Strings(unique)

	return &MultiSigPolicy{
		Threshold: threshold,
		Signers:   unique,
		Label:     label,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// NewMultiSigRequest creates a new pending multi-sig request.
func NewMultiSigRequest(policy *MultiSigPolicy, message []byte, ttlSeconds int64) *MultiSigRequest {
	now := time.Now().Unix()
	msgHash := sha256.Sum256(message)

	return &MultiSigRequest{
		ID:         hex.EncodeToString(msgHash[:8]),
		Policy:     policy,
		Message:    message,
		MessageHex: hex.EncodeToString(message),
		Signatures: []MultiSigSignature{},
		Status:     "pending",
		CreatedAt:  now,
		ExpiresAt:  now + ttlSeconds,
	}
}

// Sign adds a signature from a signer to the request.
func (r *MultiSigRequest) Sign(identity *Identity) error {
	if r.Status != "pending" {
		return fmt.Errorf("request is %s, cannot sign", r.Status)
	}
	if time.Now().Unix() > r.ExpiresAt {
		r.Status = "expired"
		return fmt.Errorf("request has expired")
	}

	// Check signer is authorized
	if !r.Policy.IsAuthorized(identity.NodeID) {
		return fmt.Errorf("signer %s is not in the policy", identity.NodeID)
	}

	// Check not already signed by this signer
	for _, sig := range r.Signatures {
		if sig.SignerID == identity.NodeID {
			return fmt.Errorf("already signed by %s", identity.NodeID)
		}
	}

	// Sign the message
	signature := ed25519.Sign(identity.PrivateKey, r.Message)

	r.Signatures = append(r.Signatures, MultiSigSignature{
		SignerID:  identity.NodeID,
		PublicKey: hex.EncodeToString(identity.PublicKey),
		Signature: hex.EncodeToString(signature),
		Timestamp: time.Now().Unix(),
	})

	// Check if threshold met
	if len(r.Signatures) >= r.Policy.Threshold {
		r.Status = "approved"
	}

	return nil
}

// IsAuthorized checks if a node ID is in the signer list.
func (p *MultiSigPolicy) IsAuthorized(nodeID string) bool {
	for _, s := range p.Signers {
		if s == nodeID {
			return true
		}
	}
	return false
}

// Verify checks all collected signatures and returns true if the threshold is met.
func (r *MultiSigRequest) Verify() (bool, error) {
	validCount := 0

	for _, sig := range r.Signatures {
		// Check signer is in policy
		if !r.Policy.IsAuthorized(sig.SignerID) {
			continue
		}

		// Decode public key
		pubKey, err := hex.DecodeString(sig.PublicKey)
		if err != nil || len(pubKey) != ed25519.PublicKeySize {
			continue
		}

		// Verify the public key matches the claimed signer ID
		expectedID := deriveNodeID(pubKey)
		if expectedID != sig.SignerID {
			continue // public key doesn't match claimed identity
		}

		// Verify signature
		sigBytes, err := hex.DecodeString(sig.Signature)
		if err != nil || len(sigBytes) != ed25519.SignatureSize {
			continue
		}

		if ed25519.Verify(pubKey, r.Message, sigBytes) {
			validCount++
		}
	}

	return validCount >= r.Policy.Threshold, nil
}

// PendingCount returns how many more signatures are needed.
func (r *MultiSigRequest) PendingCount() int {
	remaining := r.Policy.Threshold - len(r.Signatures)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ToJSON serializes the multi-sig request for transport.
func (r *MultiSigRequest) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// MultiSigRequestFromJSON deserializes a multi-sig request.
func MultiSigRequestFromJSON(data []byte) (*MultiSigRequest, error) {
	var r MultiSigRequest
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
