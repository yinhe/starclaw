package broodmind

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// BroodMind v0 — Memory Subsystem
//
// 3-layer memory architecture:
//   Sensory   → raw tool outputs, screenshots, API responses (short-lived, seconds)
//   Working   → current task context, active goals, recent conversation (minutes-hours)
//   LongTerm  → distilled knowledge, user preferences, learned patterns (persistent)
//
// 7 memory types per architecture spec:
//   episodic, semantic, procedural, spatial, emotional, social, meta
// ════════════════════════════════════════════════════════════

type MemoryType string

const (
	MemEpisodic   MemoryType = "episodic"   // Events and experiences
	MemSemantic   MemoryType = "semantic"   // Facts and knowledge
	MemProcedural MemoryType = "procedural" // How-to and skills
	MemSpatial    MemoryType = "spatial"    // Locations and layouts
	MemEmotional  MemoryType = "emotional"  // Sentiment and preferences
	MemSocial     MemoryType = "social"     // Relationships and contacts
	MemMeta       MemoryType = "meta"       // Self-knowledge and capabilities
)

type MemoryLayer string

const (
	LayerSensory  MemoryLayer = "sensory"
	LayerWorking  MemoryLayer = "working"
	LayerLongTerm MemoryLayer = "long_term"
)

type MemoryEntry struct {
	ID         string      `json:"id"`
	Layer      MemoryLayer `json:"layer"`
	Type       MemoryType  `json:"type"`
	Content    string      `json:"content"`
	Room       string      `json:"room,omitempty"`
	Anchor     string      `json:"anchor,omitempty"`
	Path       string      `json:"path,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	Source     string      `json:"source,omitempty"`  // which agent/tool produced it
	NodeID     string      `json:"node_id,omitempty"` // originating claw node
	Score      float64     `json:"score,omitempty"`   // relevance score (for search results)
	CreatedAt  time.Time   `json:"created_at"`
	ExpiresAt  *time.Time  `json:"expires_at,omitempty"`
	AccessCnt  int         `json:"access_count"`
	LastAccess time.Time   `json:"last_access"`
}

// MemoryStore is the v0 in-memory implementation.
// Future versions will use vector DB (Qdrant/Milvus) + SQLite for persistence.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*MemoryEntry
	maxSize int
}

func NewMemoryStore(maxSize int) *MemoryStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	ms := &MemoryStore{
		entries: make(map[string]*MemoryEntry),
		maxSize: maxSize,
	}
	go ms.gcLoop()
	return ms
}

// Store adds or updates a memory entry
func (ms *MemoryStore) Store(entry *MemoryEntry) string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if entry.ID == "" {
		entry.ID = memoryHash(entry.Content + string(entry.Type))
	}
	entry.LastAccess = time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	// Set default expiry based on layer
	if entry.ExpiresAt == nil {
		switch entry.Layer {
		case LayerSensory:
			t := time.Now().Add(5 * time.Minute)
			entry.ExpiresAt = &t
		case LayerWorking:
			t := time.Now().Add(24 * time.Hour)
			entry.ExpiresAt = &t
			// LongTerm: no expiry
		}
	}

	ms.entries[entry.ID] = entry

	// Evict if over capacity
	if len(ms.entries) > ms.maxSize {
		ms.evictOldest()
	}

	return entry.ID
}

// Retrieve gets a specific memory entry by ID
func (ms *MemoryStore) Retrieve(id string) *MemoryEntry {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	entry, ok := ms.entries[id]
	if !ok {
		return nil
	}
	entry.AccessCnt++
	entry.LastAccess = time.Now()
	return entry
}

// Search finds memories matching the query (simple keyword + tag matching for v0)
func (ms *MemoryStore) Search(query string, memType MemoryType, layer MemoryLayer, limit int) []*MemoryEntry {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	var results []*MemoryEntry
	for _, entry := range ms.entries {
		if memType != "" && entry.Type != memType {
			continue
		}
		if layer != "" && entry.Layer != layer {
			continue
		}
		// Expired entries excluded
		if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
			continue
		}

		// Simple relevance scoring
		score := 0.0
		contentLower := strings.ToLower(entry.Content)
		for _, w := range queryWords {
			if strings.Contains(contentLower, w) {
				score += 1.0
			}
			for _, tag := range entry.Tags {
				if strings.Contains(strings.ToLower(tag), w) {
					score += 0.5
				}
			}
		}
		// Boost recent entries
		age := time.Since(entry.CreatedAt).Hours()
		if age < 1 {
			score += 0.5
		} else if age < 24 {
			score += 0.2
		}
		// Boost frequently accessed
		if entry.AccessCnt > 5 {
			score += 0.3
		}

		if score > 0 {
			e := *entry // copy
			e.Score = score
			results = append(results, &e)
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Delete removes a memory entry
func (ms *MemoryStore) Delete(id string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	_, ok := ms.entries[id]
	delete(ms.entries, id)
	return ok
}

// Distill promotes high-value working memories to long-term storage.
// Called periodically or on demand.
func (ms *MemoryStore) Distill() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	promoted := 0
	for _, entry := range ms.entries {
		if entry.Layer != LayerWorking {
			continue
		}
		// Promote if accessed frequently or explicitly tagged important
		if entry.AccessCnt >= 3 || containsTag(entry.Tags, "important") {
			entry.Layer = LayerLongTerm
			entry.ExpiresAt = nil // no expiry for long-term
			promoted++
		}
	}
	if promoted > 0 {
		log.Printf("[broodmind] distilled %d memories from working → long-term", promoted)
	}
	return promoted
}

// Stats returns memory store statistics
func (ms *MemoryStore) Stats() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	layers := map[MemoryLayer]int{}
	types := map[MemoryType]int{}
	for _, e := range ms.entries {
		layers[e.Layer]++
		types[e.Type]++
	}
	return map[string]interface{}{
		"total":  len(ms.entries),
		"layers": layers,
		"types":  types,
		"max":    ms.maxSize,
	}
}

// gcLoop periodically removes expired entries and runs distillation
func (ms *MemoryStore) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	distillTicker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	defer distillTicker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.mu.Lock()
			now := time.Now()
			removed := 0
			for id, entry := range ms.entries {
				if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
					delete(ms.entries, id)
					removed++
				}
			}
			ms.mu.Unlock()
			if removed > 0 {
				log.Printf("[broodmind] GC: removed %d expired memories", removed)
			}
		case <-distillTicker.C:
			ms.Distill()
		}
	}
}

func (ms *MemoryStore) evictOldest() {
	var oldest *MemoryEntry
	for _, e := range ms.entries {
		if e.Layer == LayerLongTerm {
			continue // never evict long-term
		}
		if oldest == nil || e.LastAccess.Before(oldest.LastAccess) {
			oldest = e
		}
	}
	if oldest != nil {
		delete(ms.entries, oldest.ID)
	}
}

func memoryHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return "mem:" + hex.EncodeToString(h[:8])
}

func containsTag(tags []string, target string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, target) {
			return true
		}
	}
	return false
}
