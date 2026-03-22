package carapace

import (
	"log"
	"sync"
	"time"
)

// AuditEntry records a secret access event.
type AuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Operation  string    `json:"operation"`   // "seal", "unseal", "rotate", "re-encrypt"
	Purpose    string    `json:"purpose"`     // "api_key", "db_password", etc.
	KeyVersion int       `json:"key_version"`
	Caller     string    `json:"caller"`      // service name: "hive", "queen", etc.
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// AuditLog collects secret access events. Default implementation logs to stdout.
// Can be replaced with a DB-backed or external sink.
type AuditLog struct {
	mu      sync.Mutex
	entries []AuditEntry
	maxSize int
	caller  string
}

func newAuditLog(caller string) *AuditLog {
	return &AuditLog{
		maxSize: 10000,
		caller:  caller,
	}
}

func (a *AuditLog) record(op, purpose string, keyVer int, err error) {
	entry := AuditEntry{
		Timestamp:  time.Now().UTC(),
		Operation:  op,
		Purpose:    purpose,
		KeyVersion: keyVer,
		Caller:     a.caller,
		Success:    err == nil,
	}
	if err != nil {
		entry.Error = err.Error()
		log.Printf("[carapace] AUDIT %s %s purpose=%s key=v%d ERROR: %v", a.caller, op, purpose, keyVer, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.entries = append(a.entries, entry)
	// Ring buffer: drop oldest when full
	if len(a.entries) > a.maxSize {
		a.entries = a.entries[len(a.entries)-a.maxSize:]
	}
}

// Recent returns the last N audit entries.
func (a *AuditLog) Recent(n int) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	if n > len(a.entries) {
		n = len(a.entries)
	}
	result := make([]AuditEntry, n)
	copy(result, a.entries[len(a.entries)-n:])
	return result
}

// Stats returns aggregate counts by operation.
func (a *AuditLog) Stats() map[string]int {
	a.mu.Lock()
	defer a.mu.Unlock()

	stats := make(map[string]int)
	for _, e := range a.entries {
		stats[e.Operation]++
		if !e.Success {
			stats[e.Operation+"_errors"]++
		}
	}
	stats["total"] = len(a.entries)
	return stats
}
