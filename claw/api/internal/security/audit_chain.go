package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Immutable Audit Chain (Merkle-linked)
// ════════════════════════════════════════════════════════════════

// AuditChainEntry is a single entry in the tamper-evident audit log.
// Each entry's hash includes the previous entry's hash, forming a chain.
type AuditChainEntry struct {
	ID            string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Sequence      int64     `json:"sequence" gorm:"uniqueIndex;autoIncrement:false"`
	PrevHash      string    `json:"prev_hash" gorm:"type:varchar(64);not null"` // SHA-256 of previous entry
	Hash          string    `json:"hash" gorm:"type:varchar(64);not null;index"` // SHA-256(sequence + prev_hash + action + actor + target + data + timestamp)
	Action        string    `json:"action" gorm:"type:varchar(100);index;not null"` // login, logout, create, update, delete, export, admin_action
	Actor         string    `json:"actor" gorm:"type:varchar(100);index;not null"` // user_id or system
	ActorIP       string    `json:"actor_ip" gorm:"type:varchar(45)"`
	Target        string    `json:"target" gorm:"type:varchar(200);index"` // resource type + ID
	Data          string    `json:"data" gorm:"type:json"` // JSON details of what changed
	Severity      string    `json:"severity" gorm:"type:varchar(20);default:info;index"` // info, warning, critical
	CreatedAt     time.Time `json:"created_at" gorm:"index;not null"`
}

func (e *AuditChainEntry) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return nil
}

// ComputeHash calculates the SHA-256 hash for this entry.
func (e *AuditChainEntry) ComputeHash() string {
	data := struct {
		Seq       int64  `json:"seq"`
		PrevHash  string `json:"prev"`
		Action    string `json:"action"`
		Actor     string `json:"actor"`
		Target    string `json:"target"`
		Data      string `json:"data"`
		Timestamp string `json:"ts"`
	}{
		Seq:       e.Sequence,
		PrevHash:  e.PrevHash,
		Action:    e.Action,
		Actor:     e.Actor,
		Target:    e.Target,
		Data:      e.Data,
		Timestamp: e.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	b, _ := json.Marshal(data)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// AuditChain manages the immutable audit log.
type AuditChain struct {
	db *gorm.DB
}

// NewAuditChain creates a new audit chain.
func NewAuditChain(db *gorm.DB) *AuditChain {
	return &AuditChain{db: db}
}

// Append adds a new entry to the chain.
func (ac *AuditChain) Append(action, actor, actorIP, target, data, severity string) error {
	// Get the last entry's hash and sequence
	var last AuditChainEntry
	prevHash := "genesis"
	var nextSeq int64 = 1

	if err := ac.db.Order("sequence DESC").First(&last).Error; err == nil {
		prevHash = last.Hash
		nextSeq = last.Sequence + 1
	}

	entry := AuditChainEntry{
		Sequence:  nextSeq,
		PrevHash:  prevHash,
		Action:    action,
		Actor:     actor,
		ActorIP:   actorIP,
		Target:    target,
		Data:      data,
		Severity:  severity,
		CreatedAt: time.Now().UTC(),
	}
	if entry.Severity == "" {
		entry.Severity = "info"
	}
	if entry.Data == "" {
		entry.Data = "{}"
	}

	entry.Hash = entry.ComputeHash()

	return ac.db.Create(&entry).Error
}

// Verify checks the integrity of the entire chain from genesis.
func (ac *AuditChain) Verify() (bool, int64, string) {
	var entries []AuditChainEntry
	ac.db.Order("sequence ASC").Find(&entries)

	if len(entries) == 0 {
		return true, 0, "chain is empty"
	}

	prevHash := "genesis"
	for _, entry := range entries {
		// Check chain linkage
		if entry.PrevHash != prevHash {
			return false, entry.Sequence, "chain broken: prev_hash mismatch"
		}

		// Recompute hash
		expected := entry.ComputeHash()
		if entry.Hash != expected {
			return false, entry.Sequence, "tampered: hash mismatch"
		}

		prevHash = entry.Hash
	}

	return true, int64(len(entries)), "chain intact"
}

// Query returns paginated audit entries with filters.
func (ac *AuditChain) Query(action, actor, target, severity string, since time.Time, page, pageSize int) ([]AuditChainEntry, int64) {
	q := ac.db.Model(&AuditChainEntry{})
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if actor != "" {
		q = q.Where("actor = ?", actor)
	}
	if target != "" {
		q = q.Where("target LIKE ?", "%"+target+"%")
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if !since.IsZero() {
		q = q.Where("created_at >= ?", since)
	}

	var total int64
	q.Count(&total)

	var entries []AuditChainEntry
	q.Order("sequence DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries)

	return entries, total
}

// Export returns all entries as JSON for external audit.
func (ac *AuditChain) Export(since time.Time) ([]byte, error) {
	var entries []AuditChainEntry
	q := ac.db.Order("sequence ASC")
	if !since.IsZero() {
		q = q.Where("created_at >= ?", since)
	}
	q.Find(&entries)
	return json.MarshalIndent(entries, "", "  ")
}

// Stats returns audit chain statistics.
func (ac *AuditChain) Stats() map[string]interface{} {
	var total int64
	ac.db.Model(&AuditChainEntry{}).Count(&total)

	dayAgo := time.Now().Add(-24 * time.Hour)
	var todayCount int64
	ac.db.Model(&AuditChainEntry{}).Where("created_at >= ?", dayAgo).Count(&todayCount)

	var criticalCount int64
	ac.db.Model(&AuditChainEntry{}).Where("severity = ?", "critical").Count(&criticalCount)

	valid, _, msg := ac.Verify()

	return map[string]interface{}{
		"total_entries":    total,
		"entries_today":    todayCount,
		"critical_entries": criticalCount,
		"chain_valid":      valid,
		"chain_status":     msg,
	}
}
