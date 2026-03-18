package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

// CRDT types for conflict-free replicated data
const (
	CRDTLWWRegister = "lww_register" // Last-Writer-Wins Register
	CRDTGCounter    = "g_counter"    // Grow-only Counter
	CRDTORSet       = "or_set"       // Observed-Remove Set
)

// CreepEntry is a single CRDT entry in the Creep store.
type CreepEntry struct {
	Key       string          `json:"key"`
	Type      string          `json:"type"`      // lww_register | g_counter | or_set
	Namespace string          `json:"namespace"` // e.g. "agent_config", "knowledge", "workflow"
	Value     json.RawMessage `json:"value"`
	Clock     VectorClock     `json:"clock"`
	UpdatedAt int64           `json:"updated_at"` // Unix nanos for LWW
	NodeID    string          `json:"node_id"`    // last writer
	Hash      string          `json:"hash"`       // SHA-256 of value for merkle
}

// VectorClock tracks causal ordering across nodes.
type VectorClock map[string]uint64

// Increment increments the clock for a node.
func (vc VectorClock) Increment(nodeID string) {
	vc[nodeID]++
}

// Merge combines two vector clocks (element-wise max).
func (vc VectorClock) Merge(other VectorClock) {
	for k, v := range other {
		if v > vc[k] {
			vc[k] = v
		}
	}
}

// HappensBefore returns true if vc < other (strict causal ordering).
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	atLeastOne := false
	for k, v := range vc {
		if v > other[k] {
			return false
		}
		if v < other[k] {
			atLeastOne = true
		}
	}
	for k, v := range other {
		if _, ok := vc[k]; !ok && v > 0 {
			atLeastOne = true
		}
	}
	return atLeastOne
}

// Concurrent returns true if neither clock happens before the other.
func (vc VectorClock) Concurrent(other VectorClock) bool {
	return !vc.HappensBefore(other) && !other.HappensBefore(vc)
}

// ── LWW Register ──

// LWWValue is the value of a Last-Writer-Wins Register.
type LWWValue struct {
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"` // Unix nanos
	NodeID    string          `json:"node_id"`
}

// ── G-Counter ──

// GCounterValue is a grow-only counter (per-node counts).
type GCounterValue map[string]int64

// Total returns the sum of all node counters.
func (gc GCounterValue) Total() int64 {
	var sum int64
	for _, v := range gc {
		sum += v
	}
	return sum
}

// Merge merges two G-Counters (element-wise max).
func (gc GCounterValue) Merge(other GCounterValue) {
	for k, v := range other {
		if v > gc[k] {
			gc[k] = v
		}
	}
}

// ── OR-Set ──

// ORSetElement is an element in an Observed-Remove Set.
type ORSetElement struct {
	Value string `json:"value"`
	Tag   string `json:"tag"`    // unique tag per add operation
	Added bool   `json:"added"`  // true=add, false=remove
}

// ORSetValue holds the observed-remove set state.
type ORSetValue struct {
	Elements []ORSetElement `json:"elements"`
}

// Lookup returns the current members of the OR-Set.
func (s *ORSetValue) Lookup() []string {
	active := make(map[string]map[string]bool) // value -> set of active tags
	for _, e := range s.Elements {
		if _, ok := active[e.Value]; !ok {
			active[e.Value] = make(map[string]bool)
		}
		if e.Added {
			active[e.Value][e.Tag] = true
		} else {
			delete(active[e.Value], e.Tag)
		}
	}
	var result []string
	for v, tags := range active {
		if len(tags) > 0 {
			result = append(result, v)
		}
	}
	sort.Strings(result)
	return result
}

// Merge merges two OR-Sets (union of all operations).
func (s *ORSetValue) Merge(other *ORSetValue) {
	seen := make(map[string]bool)
	for _, e := range s.Elements {
		seen[e.Tag+":"+fmt.Sprintf("%v", e.Added)] = true
	}
	for _, e := range other.Elements {
		key := e.Tag + ":" + fmt.Sprintf("%v", e.Added)
		if !seen[key] {
			s.Elements = append(s.Elements, e)
			seen[key] = true
		}
	}
}

// ── Merkle Tree for anti-entropy ──

// MerkleNode represents a node in the merkle hash tree.
type MerkleNode struct {
	Hash     string       `json:"hash"`
	Prefix   string       `json:"prefix"`
	Children []MerkleNode `json:"children,omitempty"`
	Keys     []string     `json:"keys,omitempty"` // leaf keys
}

// ── Creep Engine ──

// CreepEngine manages CRDT-based cross-node data synchronization.
type CreepEngine struct {
	nodeID   string
	store    map[string]*CreepEntry // key -> entry
	mu       sync.RWMutex
	httpC    *http.Client
	gossip   *GossipEngine
	onChange func(key string, entry *CreepEntry)
	stopCh   chan struct{}
	started  bool
}

// NewCreepEngine creates the Creep data sync engine.
func NewCreepEngine(nodeID string, gossip *GossipEngine) *CreepEngine {
	return &CreepEngine{
		nodeID: nodeID,
		store:  make(map[string]*CreepEntry),
		httpC:  &http.Client{Timeout: 10 * time.Second},
		gossip: gossip,
		stopCh: make(chan struct{}),
	}
}

// OnChange sets a callback for when a value is updated via sync.
func (ce *CreepEngine) OnChange(fn func(string, *CreepEntry)) {
	ce.onChange = fn
}

// Start begins the sync loops.
func (ce *CreepEngine) Start(syncInterval time.Duration) {
	if syncInterval < 30*time.Second {
		syncInterval = 60 * time.Second
	}
	ce.started = true
	go ce.syncLoop(syncInterval)
	log.Printf("[creep] started with sync interval %s", syncInterval)
}

// Stop halts the sync engine.
func (ce *CreepEngine) Stop() {
	select {
	case <-ce.stopCh:
	default:
		close(ce.stopCh)
	}
}

// ── Write operations ──

// SetRegister sets a LWW register value.
func (ce *CreepEngine) SetRegister(namespace, key string, data json.RawMessage) {
	fullKey := namespace + "/" + key
	now := time.Now().UnixNano()

	lwv := LWWValue{Data: data, Timestamp: now, NodeID: ce.nodeID}
	valBytes, _ := json.Marshal(lwv)

	ce.mu.Lock()
	existing, exists := ce.store[fullKey]
	clock := make(VectorClock)
	if exists {
		clock = existing.Clock
	}
	clock.Increment(ce.nodeID)

	entry := &CreepEntry{
		Key:       fullKey,
		Type:      CRDTLWWRegister,
		Namespace: namespace,
		Value:     valBytes,
		Clock:     clock,
		UpdatedAt: now,
		NodeID:    ce.nodeID,
		Hash:      hashValue(valBytes),
	}
	ce.store[fullKey] = entry
	ce.mu.Unlock()
}

// IncrementCounter increments a G-Counter for this node.
func (ce *CreepEngine) IncrementCounter(namespace, key string, delta int64) {
	fullKey := namespace + "/" + key

	ce.mu.Lock()
	existing, exists := ce.store[fullKey]

	var gc GCounterValue
	if exists {
		json.Unmarshal(existing.Value, &gc)
	}
	if gc == nil {
		gc = make(GCounterValue)
	}
	gc[ce.nodeID] += delta

	clock := make(VectorClock)
	if exists {
		clock = existing.Clock
	}
	clock.Increment(ce.nodeID)

	valBytes, _ := json.Marshal(gc)
	entry := &CreepEntry{
		Key:       fullKey,
		Type:      CRDTGCounter,
		Namespace: namespace,
		Value:     valBytes,
		Clock:     clock,
		UpdatedAt: time.Now().UnixNano(),
		NodeID:    ce.nodeID,
		Hash:      hashValue(valBytes),
	}
	ce.store[fullKey] = entry
	ce.mu.Unlock()
}

// AddToSet adds an element to an OR-Set.
func (ce *CreepEngine) AddToSet(namespace, key, value string) {
	fullKey := namespace + "/" + key
	tag := fmt.Sprintf("%s:%d", ce.nodeID, time.Now().UnixNano())

	ce.mu.Lock()
	existing, exists := ce.store[fullKey]

	var orset ORSetValue
	if exists {
		json.Unmarshal(existing.Value, &orset)
	}
	orset.Elements = append(orset.Elements, ORSetElement{Value: value, Tag: tag, Added: true})

	clock := make(VectorClock)
	if exists {
		clock = existing.Clock
	}
	clock.Increment(ce.nodeID)

	valBytes, _ := json.Marshal(orset)
	entry := &CreepEntry{
		Key:       fullKey,
		Type:      CRDTORSet,
		Namespace: namespace,
		Value:     valBytes,
		Clock:     clock,
		UpdatedAt: time.Now().UnixNano(),
		NodeID:    ce.nodeID,
		Hash:      hashValue(valBytes),
	}
	ce.store[fullKey] = entry
	ce.mu.Unlock()
}

// RemoveFromSet removes an element from an OR-Set (observed-remove).
func (ce *CreepEngine) RemoveFromSet(namespace, key, value string) {
	fullKey := namespace + "/" + key

	ce.mu.Lock()
	existing, exists := ce.store[fullKey]
	if !exists {
		ce.mu.Unlock()
		return
	}

	var orset ORSetValue
	json.Unmarshal(existing.Value, &orset)

	// Find all active tags for this value and mark as removed
	for _, e := range orset.Elements {
		if e.Value == value && e.Added {
			orset.Elements = append(orset.Elements, ORSetElement{Value: value, Tag: e.Tag, Added: false})
		}
	}

	existing.Clock.Increment(ce.nodeID)
	valBytes, _ := json.Marshal(orset)
	existing.Value = valBytes
	existing.UpdatedAt = time.Now().UnixNano()
	existing.Hash = hashValue(valBytes)
	ce.mu.Unlock()
}

// Get retrieves an entry by key.
func (ce *CreepEngine) Get(namespace, key string) *CreepEntry {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.store[namespace+"/"+key]
}

// ── Merge (conflict resolution) ──

// MergeEntry merges a remote entry into the local store using CRDT rules.
func (ce *CreepEngine) MergeEntry(remote *CreepEntry) bool {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	local, exists := ce.store[remote.Key]
	if !exists {
		// New key: accept remote
		ce.store[remote.Key] = remote
		return true
	}

	// Same hash: no change
	if local.Hash == remote.Hash {
		return false
	}

	merged := false
	switch remote.Type {
	case CRDTLWWRegister:
		// Last-Writer-Wins: highest timestamp wins
		if remote.UpdatedAt > local.UpdatedAt {
			ce.store[remote.Key] = remote
			merged = true
		} else if remote.UpdatedAt == local.UpdatedAt && remote.NodeID > local.NodeID {
			// Tie-break by node ID
			ce.store[remote.Key] = remote
			merged = true
		}

	case CRDTGCounter:
		// Merge counters (element-wise max)
		var localGC, remoteGC GCounterValue
		json.Unmarshal(local.Value, &localGC)
		json.Unmarshal(remote.Value, &remoteGC)
		if localGC == nil {
			localGC = make(GCounterValue)
		}
		localGC.Merge(remoteGC)
		valBytes, _ := json.Marshal(localGC)
		local.Value = valBytes
		local.Clock.Merge(remote.Clock)
		local.Hash = hashValue(valBytes)
		local.UpdatedAt = time.Now().UnixNano()
		merged = true

	case CRDTORSet:
		// Merge OR-Sets (union of operations)
		var localSet, remoteSet ORSetValue
		json.Unmarshal(local.Value, &localSet)
		json.Unmarshal(remote.Value, &remoteSet)
		localSet.Merge(&remoteSet)
		valBytes, _ := json.Marshal(localSet)
		local.Value = valBytes
		local.Clock.Merge(remote.Clock)
		local.Hash = hashValue(valBytes)
		local.UpdatedAt = time.Now().UnixNano()
		merged = true
	}

	if merged {
		local.Clock.Merge(remote.Clock)
		if ce.onChange != nil {
			go ce.onChange(remote.Key, ce.store[remote.Key])
		}
	}
	return merged
}

// ── Sync protocol (anti-entropy with merkle tree) ──

// BuildMerkleRoot computes a merkle hash over all entries in a namespace.
func (ce *CreepEngine) BuildMerkleRoot(namespace string) string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	var hashes []string
	for _, entry := range ce.store {
		if namespace == "" || entry.Namespace == namespace {
			hashes = append(hashes, entry.Hash)
		}
	}
	sort.Strings(hashes)

	h := sha256.New()
	for _, hash := range hashes {
		h.Write([]byte(hash))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GetSyncDigest returns a compact digest for anti-entropy comparison.
func (ce *CreepEngine) GetSyncDigest(namespace string) map[string]string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	digest := make(map[string]string)
	for key, entry := range ce.store {
		if namespace == "" || entry.Namespace == namespace {
			digest[key] = entry.Hash
		}
	}
	return digest
}

// SyncWithPeer performs anti-entropy sync with a remote peer.
func (ce *CreepEngine) SyncWithPeer(peerAddr string) (int, error) {
	// 1. Get local digest
	localDigest := ce.GetSyncDigest("")

	// 2. Send digest to peer, get diff
	reqBody, _ := json.Marshal(map[string]interface{}{
		"digest":  localDigest,
		"node_id": ce.nodeID,
	})

	resp, err := ce.httpC.Post(peerAddr+"/v1/peer/creep/sync", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("sync status %d", resp.StatusCode)
	}

	var syncResp struct {
		Entries []CreepEntry `json:"entries"` // entries the peer has that we don't / differ
		Need    []string     `json:"need"`    // keys the peer needs from us
	}
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return 0, err
	}

	// 3. Merge received entries
	merged := 0
	for i := range syncResp.Entries {
		if ce.MergeEntry(&syncResp.Entries[i]) {
			merged++
		}
	}

	// 4. Send entries the peer needs
	if len(syncResp.Need) > 0 {
		ce.mu.RLock()
		var toSend []CreepEntry
		for _, key := range syncResp.Need {
			if entry, ok := ce.store[key]; ok {
				toSend = append(toSend, *entry)
			}
		}
		ce.mu.RUnlock()

		if len(toSend) > 0 {
			pushBody, _ := json.Marshal(map[string]interface{}{
				"entries": toSend,
				"node_id": ce.nodeID,
			})
			ce.httpC.Post(peerAddr+"/v1/peer/creep/push", "application/json", bytes.NewReader(pushBody))
		}
	}

	return merged, nil
}

// ── HTTP handlers ──

// HandleSync processes an incoming sync request: compares digests, returns diff.
func (ce *CreepEngine) HandleSync(remoteDigest map[string]string) (entries []CreepEntry, need []string) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	// Entries we have that the remote doesn't or are different
	for key, entry := range ce.store {
		remoteHash, ok := remoteDigest[key]
		if !ok || remoteHash != entry.Hash {
			entries = append(entries, *entry)
		}
	}

	// Keys the remote has that we don't
	for key := range remoteDigest {
		if _, ok := ce.store[key]; !ok {
			need = append(need, key)
		}
	}

	return entries, need
}

// HandlePush processes incoming entries pushed by a peer.
func (ce *CreepEngine) HandlePush(entries []CreepEntry) int {
	merged := 0
	for i := range entries {
		if ce.MergeEntry(&entries[i]) {
			merged++
		}
	}
	return merged
}

// ── Sync loop ──

func (ce *CreepEngine) syncLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if ce.gossip == nil {
				continue
			}
			peers := ce.gossip.GetPeers()
			for _, p := range peers {
				if p.Address == "" {
					continue
				}
				merged, err := ce.SyncWithPeer(p.Address)
				if err != nil {
					continue
				}
				if merged > 0 {
					log.Printf("[creep] synced %d entries from %s", merged, p.NodeID[:min(16, len(p.NodeID))])
				}
			}
		case <-ce.stopCh:
			return
		}
	}
}

// Stats returns Creep engine statistics.
func (ce *CreepEngine) Stats() map[string]interface{} {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	nsByCount := make(map[string]int)
	typeCount := make(map[string]int)
	for _, entry := range ce.store {
		nsByCount[entry.Namespace]++
		typeCount[entry.Type]++
	}

	return map[string]interface{}{
		"total_entries": len(ce.store),
		"namespaces":    nsByCount,
		"types":         typeCount,
		"merkle_root":   ce.BuildMerkleRoot(""),
	}
}

// hashValue computes SHA-256 hex of raw bytes.
func hashValue(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
