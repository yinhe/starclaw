package broodmind

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodMind v2 — Cross-Node Memory Sync
//
// Synchronizes memories across Claw nodes via HTTP push/pull
// and Pheromone event bus. Handles:
//
//   - Push:    local memory changes → broadcast to peers
//   - Pull:    request missing memories from peers
//   - Merge:   conflict resolution (LWW / newer wins / manual)
//   - Privacy: per-entry sync policy (local_only / node_group / global)
//
// Integration points:
//   - Pheromone: publish memory.sync.* events for real-time notification
//   - Hive Discovery: find peer nodes to sync with
//   - MemoryStore: hook into Store/Delete for auto-sync
// ════════════════════════════════════════════════════════════

// ── Types ──

// SyncScope controls who can see a memory
type SyncScope string

const (
	ScopeLocalOnly SyncScope = "local_only" // never leaves this node
	ScopeNodeGroup SyncScope = "node_group" // shared within owner's node group
	ScopeGlobal    SyncScope = "global"     // shared across all nodes
)

// SyncOp represents a memory mutation operation
type SyncOp string

const (
	SyncOpStore  SyncOp = "store"
	SyncOpDelete SyncOp = "delete"
	SyncOpUpdate SyncOp = "update"
)

// SyncEntry wraps a MemoryEntry with sync metadata
type SyncEntry struct {
	MemoryID  string      `json:"memory_id"`
	Op        SyncOp      `json:"op"`
	Scope     SyncScope   `json:"scope"`
	Entry     *MemoryEntry `json:"entry,omitempty"`
	OriginNode string     `json:"origin_node"`
	VectorClock int64     `json:"vector_clock"` // logical timestamp for ordering
	Timestamp  time.Time  `json:"timestamp"`
}

// SyncPeer represents a known peer node for sync
type SyncPeer struct {
	NodeID    string    `json:"node_id"`
	Address   string    `json:"address"`
	LastSync  time.Time `json:"last_sync"`
	SyncCount int       `json:"sync_count"`
	Errors    int       `json:"errors"`
	Online    bool      `json:"online"`
}

// SyncConflict records when two nodes modify the same memory
type SyncConflict struct {
	ID         string       `json:"id"`
	MemoryID   string       `json:"memory_id"`
	LocalEntry *MemoryEntry `json:"local_entry"`
	RemoteEntry *MemoryEntry `json:"remote_entry"`
	RemoteNode string       `json:"remote_node"`
	Resolution string       `json:"resolution"` // "", "local_wins", "remote_wins", "merged"
	ResolvedAt *time.Time   `json:"resolved_at,omitempty"`
	DetectedAt time.Time    `json:"detected_at"`
}

// MemSyncConfig holds sync configuration
type MemSyncConfig struct {
	Enabled         bool          `json:"enabled"`
	DefaultScope    SyncScope     `json:"default_scope"`
	SyncIntervalSec int           `json:"sync_interval_sec"`
	PheromoneURL    string        `json:"pheromone_url,omitempty"`
	MaxPendingOps   int           `json:"max_pending_ops"`
	MaxConflicts    int           `json:"max_conflicts"`
	ConflictPolicy  string        `json:"conflict_policy"` // "lww" (last-writer-wins), "local", "remote", "manual"
}

// DefaultMemSyncConfig returns sensible defaults
func DefaultMemSyncConfig() *MemSyncConfig {
	return &MemSyncConfig{
		Enabled:         true,
		DefaultScope:    ScopeNodeGroup,
		SyncIntervalSec: 30,
		MaxPendingOps:   5000,
		MaxConflicts:    200,
		ConflictPolicy:  "lww",
	}
}

// ── MemSync Engine ──

// MemSync manages cross-node memory synchronization
type MemSync struct {
	mu          sync.RWMutex
	config      *MemSyncConfig
	nodeID      string
	clock       int64 // logical clock (monotonically increasing)
	peers       map[string]*SyncPeer
	pendingOps  []*SyncEntry
	conflicts   []*SyncConflict
	conflByID   map[string]*SyncConflict
	stats       MemSyncStats
	pheromoneURL string
	httpClient  *http.Client
}

// MemSyncStats tracks sync metrics
type MemSyncStats struct {
	PushCount       int   `json:"push_count"`
	PullCount       int   `json:"pull_count"`
	ConflictCount   int   `json:"conflict_count"`
	ResolvedCount   int   `json:"resolved_count"`
	PendingOps      int   `json:"pending_ops"`
	PeerCount       int   `json:"peer_count"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	ErrorCount      int   `json:"error_count"`
}

// NewMemSync creates a new memory sync engine
func NewMemSync(nodeID string, cfg *MemSyncConfig) *MemSync {
	if cfg == nil {
		cfg = DefaultMemSyncConfig()
	}
	ms := &MemSync{
		config:     cfg,
		nodeID:     nodeID,
		peers:      make(map[string]*SyncPeer),
		pendingOps: make([]*SyncEntry, 0, cfg.MaxPendingOps),
		conflicts:  make([]*SyncConflict, 0),
		conflByID:  make(map[string]*SyncConflict),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	if cfg.PheromoneURL != "" {
		ms.pheromoneURL = cfg.PheromoneURL
	}
	return ms
}

// ── Push Operations ──

// RecordOp records a local memory operation for sync to peers
func (ms *MemSync) RecordOp(op SyncOp, entry *MemoryEntry, scope SyncScope) {
	if !ms.config.Enabled {
		return
	}
	if scope == ScopeLocalOnly {
		return // don't sync local-only memories
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.clock++
	se := &SyncEntry{
		MemoryID:    entry.ID,
		Op:          op,
		Scope:       scope,
		Entry:       entry,
		OriginNode:  ms.nodeID,
		VectorClock: ms.clock,
		Timestamp:   time.Now(),
	}
	ms.pendingOps = append(ms.pendingOps, se)

	// Evict old pending ops
	if len(ms.pendingOps) > ms.config.MaxPendingOps {
		ms.pendingOps = ms.pendingOps[len(ms.pendingOps)-ms.config.MaxPendingOps:]
	}

	ms.stats.PushCount++
}

// FlushToPeers pushes pending operations to all known peers
func (ms *MemSync) FlushToPeers() int {
	ms.mu.Lock()
	ops := make([]*SyncEntry, len(ms.pendingOps))
	copy(ops, ms.pendingOps)
	ms.pendingOps = ms.pendingOps[:0]
	peers := make([]*SyncPeer, 0, len(ms.peers))
	for _, p := range ms.peers {
		if p.Online {
			peers = append(peers, p)
		}
	}
	ms.mu.Unlock()

	if len(ops) == 0 || len(peers) == 0 {
		return 0
	}

	pushed := 0
	for _, peer := range peers {
		if err := ms.pushToPeer(peer, ops); err != nil {
			log.Printf("[memsync] push to %s failed: %v", peer.NodeID, err)
			ms.mu.Lock()
			peer.Errors++
			ms.stats.ErrorCount++
			ms.mu.Unlock()
		} else {
			ms.mu.Lock()
			peer.LastSync = time.Now()
			peer.SyncCount++
			ms.mu.Unlock()
			pushed++
		}
	}

	ms.mu.Lock()
	now := time.Now()
	ms.stats.LastSyncAt = &now
	ms.mu.Unlock()

	if pushed > 0 {
		log.Printf("[memsync] flushed %d ops to %d/%d peers", len(ops), pushed, len(peers))
	}

	// Also publish to Pheromone if configured
	if ms.pheromoneURL != "" {
		ms.publishPheromone("memory.sync.push", map[string]interface{}{
			"node_id": ms.nodeID,
			"ops":     len(ops),
			"peers":   pushed,
		})
	}

	return pushed
}

// pushToPeer sends sync entries to a specific peer
func (ms *MemSync) pushToPeer(peer *SyncPeer, ops []*SyncEntry) error {
	payload := map[string]interface{}{
		"origin":  ms.nodeID,
		"entries": ops,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/v1/broodmind/sync/receive", peer.Address)
	resp, err := ms.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer %s returned %d", peer.NodeID, resp.StatusCode)
	}
	return nil
}

// ── Receive Operations ──

// ReceiveFromPeer processes incoming sync entries from a remote node
func (ms *MemSync) ReceiveFromPeer(originNode string, entries []*SyncEntry) (applied int, conflicts int) {
	if !ms.config.Enabled {
		return 0, 0
	}
	if instance == nil {
		return 0, 0
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, se := range entries {
		if se.OriginNode == ms.nodeID {
			continue // skip our own echoes
		}

		// Update logical clock
		if se.VectorClock > ms.clock {
			ms.clock = se.VectorClock
		}
		ms.clock++

		switch se.Op {
		case SyncOpStore, SyncOpUpdate:
			if se.Entry == nil {
				continue
			}
			// Check for conflict
			existing := instance.Memory.Retrieve(se.MemoryID)
			if existing != nil && existing.NodeID != se.OriginNode {
				// Conflict detected
				conflict := ms.resolveConflict(existing, se.Entry, se.OriginNode)
				if conflict != nil {
					conflicts++
				}
			} else {
				// No conflict — apply directly
				se.Entry.NodeID = se.OriginNode
				instance.Memory.Store(se.Entry)
				applied++
			}

		case SyncOpDelete:
			instance.Memory.Delete(se.MemoryID)
			applied++
		}
	}

	ms.stats.PullCount += applied

	if applied > 0 || conflicts > 0 {
		log.Printf("[memsync] received from %s: %d applied, %d conflicts", originNode, applied, conflicts)
	}

	return applied, conflicts
}

// resolveConflict handles a memory conflict based on configured policy
func (ms *MemSync) resolveConflict(local, remote *MemoryEntry, remoteNode string) *SyncConflict {
	conflict := &SyncConflict{
		ID:          "mconf:" + uuid.New().String()[:8],
		MemoryID:    local.ID,
		LocalEntry:  local,
		RemoteEntry: remote,
		RemoteNode:  remoteNode,
		DetectedAt:  time.Now(),
	}

	switch ms.config.ConflictPolicy {
	case "lww": // Last-Writer-Wins
		if remote.CreatedAt.After(local.CreatedAt) || remote.LastAccess.After(local.LastAccess) {
			// Remote is newer — apply it
			remote.NodeID = remoteNode
			instance.Memory.Store(remote)
			conflict.Resolution = "remote_wins"
		} else {
			conflict.Resolution = "local_wins"
		}
		now := time.Now()
		conflict.ResolvedAt = &now
		ms.stats.ResolvedCount++

	case "local":
		conflict.Resolution = "local_wins"
		now := time.Now()
		conflict.ResolvedAt = &now
		ms.stats.ResolvedCount++

	case "remote":
		remote.NodeID = remoteNode
		instance.Memory.Store(remote)
		conflict.Resolution = "remote_wins"
		now := time.Now()
		conflict.ResolvedAt = &now
		ms.stats.ResolvedCount++

	default: // "manual"
		conflict.Resolution = ""
	}

	ms.conflicts = append(ms.conflicts, conflict)
	ms.conflByID[conflict.ID] = conflict
	ms.stats.ConflictCount++

	// Evict old conflicts
	for len(ms.conflicts) > ms.config.MaxConflicts {
		old := ms.conflicts[0]
		ms.conflicts = ms.conflicts[1:]
		delete(ms.conflByID, old.ID)
	}

	log.Printf("[memsync] conflict on %s: local(%s) vs remote(%s) → %s",
		local.ID, local.NodeID, remoteNode, conflict.Resolution)

	return conflict
}

// ── Peer Management ──

// AddPeer registers a peer node for sync
func (ms *MemSync) AddPeer(nodeID, address string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.peers[nodeID] = &SyncPeer{
		NodeID:  nodeID,
		Address: address,
		Online:  true,
	}
	ms.stats.PeerCount = len(ms.peers)
}

// RemovePeer unregisters a peer
func (ms *MemSync) RemovePeer(nodeID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.peers, nodeID)
	ms.stats.PeerCount = len(ms.peers)
}

// SetPeerOnline updates peer online status
func (ms *MemSync) SetPeerOnline(nodeID string, online bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if p, ok := ms.peers[nodeID]; ok {
		p.Online = online
	}
}

// ListPeers returns all registered peers
func (ms *MemSync) ListPeers() []*SyncPeer {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	result := make([]*SyncPeer, 0, len(ms.peers))
	for _, p := range ms.peers {
		result = append(result, p)
	}
	return result
}

// ── Conflict Management ──

// ListConflicts returns unresolved conflicts
func (ms *MemSync) ListConflicts(resolved bool, limit int) []*SyncConflict {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*SyncConflict
	for i := len(ms.conflicts) - 1; i >= 0 && len(result) < limit; i-- {
		c := ms.conflicts[i]
		if resolved && c.Resolution != "" {
			result = append(result, c)
		} else if !resolved && c.Resolution == "" {
			result = append(result, c)
		}
	}
	return result
}

// ResolveConflict manually resolves a sync conflict
func (ms *MemSync) ResolveConflict(conflictID, winner string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	c, ok := ms.conflByID[conflictID]
	if !ok {
		return fmt.Errorf("conflict %s not found", conflictID)
	}
	if c.Resolution != "" {
		return fmt.Errorf("conflict already resolved: %s", c.Resolution)
	}

	now := time.Now()
	switch winner {
	case "local":
		c.Resolution = "local_wins"
	case "remote":
		if c.RemoteEntry != nil {
			c.RemoteEntry.NodeID = c.RemoteNode
			instance.Memory.Store(c.RemoteEntry)
		}
		c.Resolution = "remote_wins"
	case "merge":
		// Merge: keep local content but add remote tags
		if c.LocalEntry != nil && c.RemoteEntry != nil {
			merged := *c.LocalEntry
			for _, tag := range c.RemoteEntry.Tags {
				if !containsTag(merged.Tags, tag) {
					merged.Tags = append(merged.Tags, tag)
				}
			}
			// Append remote content if different
			if c.RemoteEntry.Content != c.LocalEntry.Content {
				merged.Content = merged.Content + "\n---\n" + c.RemoteEntry.Content
			}
			instance.Memory.Store(&merged)
		}
		c.Resolution = "merged"
	default:
		return fmt.Errorf("winner must be 'local', 'remote', or 'merge'")
	}

	c.ResolvedAt = &now
	ms.stats.ResolvedCount++
	return nil
}

// ── Stats ──

// Stats returns sync engine metrics
func (ms *MemSync) Stats() *MemSyncStats {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	s := ms.stats
	s.PendingOps = len(ms.pendingOps)
	s.PeerCount = len(ms.peers)
	return &s
}

// Config returns a copy of the current config
func (ms *MemSync) SyncConfig() MemSyncConfig {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return *ms.config
}

// ── Sync Loop ──

// StartSyncLoop begins periodic sync with peers
func (ms *MemSync) StartSyncLoop() {
	if !ms.config.Enabled || ms.config.SyncIntervalSec <= 0 {
		return
	}
	interval := time.Duration(ms.config.SyncIntervalSec) * time.Second
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			ms.FlushToPeers()
		}
	}()
	log.Printf("[memsync] sync loop started (interval=%s, peers=%d)", interval, len(ms.peers))
}

// ── Pheromone Integration ──

func (ms *MemSync) publishPheromone(topic string, data interface{}) {
	if ms.pheromoneURL == "" {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"topic": topic,
		"data":  data,
	})
	resp, err := ms.httpClient.Post(ms.pheromoneURL+"/events/publish", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[memsync] pheromone publish failed: %v", err)
		return
	}
	resp.Body.Close()
}
