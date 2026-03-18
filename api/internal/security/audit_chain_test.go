package security

import (
	"testing"
	"time"
)

func TestAuditChainEntry_ComputeHash(t *testing.T) {
	entry := &AuditChainEntry{
		Sequence:  1,
		PrevHash:  "genesis",
		Action:    "login",
		Actor:     "user-123",
		Target:    "session",
		Data:      `{"ip":"10.0.0.1"}`,
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	hash := entry.ComputeHash()
	if len(hash) != 64 { // SHA-256 = 64 hex chars
		t.Errorf("expected 64-char hash, got %d: %s", len(hash), hash)
	}

	// Same input → same hash (deterministic)
	hash2 := entry.ComputeHash()
	if hash != hash2 {
		t.Error("ComputeHash should be deterministic")
	}
}

func TestAuditChainEntry_DifferentDataDifferentHash(t *testing.T) {
	base := AuditChainEntry{
		Sequence:  1,
		PrevHash:  "genesis",
		Action:    "login",
		Actor:     "user-123",
		Target:    "session",
		Data:      `{"ip":"10.0.0.1"}`,
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	hash1 := base.ComputeHash()

	// Change action
	modified := base
	modified.Action = "logout"
	hash2 := modified.ComputeHash()
	if hash1 == hash2 {
		t.Error("different action should produce different hash")
	}

	// Change actor
	modified = base
	modified.Actor = "user-456"
	hash3 := modified.ComputeHash()
	if hash1 == hash3 {
		t.Error("different actor should produce different hash")
	}

	// Change prev_hash (chain linkage)
	modified = base
	modified.PrevHash = "abc123"
	hash4 := modified.ComputeHash()
	if hash1 == hash4 {
		t.Error("different prev_hash should produce different hash")
	}

	// Change sequence
	modified = base
	modified.Sequence = 2
	hash5 := modified.ComputeHash()
	if hash1 == hash5 {
		t.Error("different sequence should produce different hash")
	}

	// Change timestamp
	modified = base
	modified.CreatedAt = time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	hash6 := modified.ComputeHash()
	if hash1 == hash6 {
		t.Error("different timestamp should produce different hash")
	}
}

func TestAuditChainEntry_ChainLinkage(t *testing.T) {
	// Simulate a 3-entry chain
	now := time.Now().UTC()

	entry1 := &AuditChainEntry{
		Sequence:  1,
		PrevHash:  "genesis",
		Action:    "create",
		Actor:     "admin",
		Target:    "agent/a1",
		Data:      "{}",
		CreatedAt: now,
	}
	entry1.Hash = entry1.ComputeHash()

	entry2 := &AuditChainEntry{
		Sequence:  2,
		PrevHash:  entry1.Hash,
		Action:    "update",
		Actor:     "admin",
		Target:    "agent/a1",
		Data:      `{"name":"new"}`,
		CreatedAt: now.Add(time.Second),
	}
	entry2.Hash = entry2.ComputeHash()

	entry3 := &AuditChainEntry{
		Sequence:  3,
		PrevHash:  entry2.Hash,
		Action:    "delete",
		Actor:     "admin",
		Target:    "agent/a1",
		Data:      "{}",
		CreatedAt: now.Add(2 * time.Second),
	}
	entry3.Hash = entry3.ComputeHash()

	// Verify chain: each entry's hash includes prev_hash
	chain := []*AuditChainEntry{entry1, entry2, entry3}
	for i := 1; i < len(chain); i++ {
		if chain[i].PrevHash != chain[i-1].Hash {
			t.Errorf("entry %d prev_hash doesn't match entry %d hash", i+1, i)
		}
	}

	// Verify tampering detection: modify entry2 data
	original := entry2.Data
	entry2.Data = `{"name":"tampered"}`
	newHash := entry2.ComputeHash()
	if newHash == entry2.Hash {
		t.Error("tampered data should produce different hash")
	}
	entry2.Data = original // restore
}
