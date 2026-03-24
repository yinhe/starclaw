package node

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sort"
	"sync"
	"time"
)

// DHT constants (Kademlia parameters)
const (
	DHTKeyBits   = 160            // SHA-256 truncated to 160 bits (matching node ID space)
	DHTKBucketK  = 20             // max entries per k-bucket
	DHTAlpha     = 3              // concurrency factor for iterative lookups
	DHTMaxStored = 10000          // max key-value pairs stored locally
	DHTTTL       = 24 * time.Hour // default TTL for stored values
)

// DHTNode represents a node in the DHT routing table.
type DHTNode struct {
	ID       [20]byte  `json:"id"`      // 160-bit node ID (binary)
	NodeID   string    `json:"node_id"` // "claw:xxxx" string ID
	Address  string    `json:"address"` // network address (host:port or URL)
	LastSeen time.Time `json:"last_seen"`
}

// DHTValue is a stored key-value pair in the DHT.
type DHTValue struct {
	Key       [20]byte      `json:"key"`
	Value     []byte        `json:"value"`
	Publisher string        `json:"publisher"` // node ID of original publisher
	StoredAt  time.Time     `json:"stored_at"`
	TTL       time.Duration `json:"ttl"`
}

// KBucket holds up to K nodes at a specific XOR distance range.
type KBucket struct {
	nodes []*DHTNode
	mu    sync.RWMutex
}

// DHT implements a Kademlia-style distributed hash table for decentralized node discovery.
type DHT struct {
	self     DHTNode
	selfID   [20]byte
	buckets  [DHTKeyBits]*KBucket
	store    map[[20]byte]*DHTValue
	storeMu  sync.RWMutex
	httpC    *http.Client
	identity *Identity
	gossip   *GossipEngine // existing gossip engine for fallback
	stopCh   chan struct{}
	started  bool
	mu       sync.Mutex
}

// NewDHT creates a new Kademlia DHT instance.
func NewDHT(identity *Identity, address string, gossip *GossipEngine) *DHT {
	selfID := nodeIDToBytes(identity.NodeID)
	d := &DHT{
		self: DHTNode{
			ID:       selfID,
			NodeID:   identity.NodeID,
			Address:  address,
			LastSeen: time.Now(),
		},
		selfID:   selfID,
		store:    make(map[[20]byte]*DHTValue),
		httpC:    &http.Client{Timeout: 10 * time.Second},
		identity: identity,
		gossip:   gossip,
		stopCh:   make(chan struct{}),
	}
	for i := range d.buckets {
		d.buckets[i] = &KBucket{}
	}
	return d
}

// nodeIDToBytes converts a "claw:xxxx" node ID to 160-bit binary.
func nodeIDToBytes(nodeID string) [20]byte {
	// Strip "claw:" prefix to get the 40 hex chars
	hexPart := nodeID
	if len(nodeID) > 5 && nodeID[:5] == "claw:" {
		hexPart = nodeID[5:]
	}
	var id [20]byte
	decoded, err := hex.DecodeString(hexPart)
	if err == nil && len(decoded) >= 20 {
		copy(id[:], decoded[:20])
	} else {
		// Fallback: hash the full string
		hash := sha256.Sum256([]byte(nodeID))
		copy(id[:], hash[:20])
	}
	return id
}

// hashKey produces a 160-bit key from arbitrary data.
func hashKey(data []byte) [20]byte {
	hash := sha256.Sum256(data)
	var key [20]byte
	copy(key[:], hash[:20])
	return key
}

// xorDistance computes the XOR distance between two 160-bit IDs.
func xorDistance(a, b [20]byte) [20]byte {
	var result [20]byte
	for i := 0; i < 20; i++ {
		result[i] = a[i] ^ b[i]
	}
	return result
}

// bucketIndex returns which k-bucket a node falls into (0..159).
// This is the index of the highest bit in xorDistance(self, other).
func (d *DHT) bucketIndex(other [20]byte) int {
	dist := xorDistance(d.selfID, other)
	for i := 0; i < 20; i++ {
		for bit := 7; bit >= 0; bit-- {
			if dist[i]&(1<<uint(bit)) != 0 {
				return i*8 + (7 - bit)
			}
		}
	}
	return DHTKeyBits - 1 // same node (distance = 0)
}

// distanceLess returns true if a is closer to target than b.
func distanceLess(target, a, b [20]byte) bool {
	da := xorDistance(target, a)
	db := xorDistance(target, b)
	daBig := new(big.Int).SetBytes(da[:])
	dbBig := new(big.Int).SetBytes(db[:])
	return daBig.Cmp(dbBig) < 0
}

// ── K-Bucket operations ──

// addNode adds or updates a node in the appropriate k-bucket.
func (d *DHT) addNode(n *DHTNode) {
	if n.ID == d.selfID {
		return // don't add self
	}
	idx := d.bucketIndex(n.ID)
	bucket := d.buckets[idx]

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Check if already in bucket
	for i, existing := range bucket.nodes {
		if existing.ID == n.ID {
			// Move to tail (most recently seen)
			bucket.nodes = append(bucket.nodes[:i], bucket.nodes[i+1:]...)
			n.LastSeen = time.Now()
			bucket.nodes = append(bucket.nodes, n)
			return
		}
	}

	// Bucket not full: add to tail
	if len(bucket.nodes) < DHTKBucketK {
		n.LastSeen = time.Now()
		bucket.nodes = append(bucket.nodes, n)
		return
	}

	// Bucket full: ping head (least recently seen), if dead replace
	head := bucket.nodes[0]
	if !d.pingNode(head) {
		bucket.nodes = bucket.nodes[1:]
		n.LastSeen = time.Now()
		bucket.nodes = append(bucket.nodes, n)
	}
	// If head is alive, discard new node (Kademlia preference for old nodes)
}

// closestNodes returns the K closest nodes to a target ID from the routing table.
func (d *DHT) closestNodes(target [20]byte, count int) []*DHTNode {
	var all []*DHTNode
	for _, bucket := range d.buckets {
		bucket.mu.RLock()
		all = append(all, bucket.nodes...)
		bucket.mu.RUnlock()
	}

	sort.Slice(all, func(i, j int) bool {
		return distanceLess(target, all[i].ID, all[j].ID)
	})

	if len(all) > count {
		return all[:count]
	}
	return all
}

// ── Core DHT operations ──

// Start begins the DHT maintenance loops.
func (d *DHT) Start() {
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return
	}
	d.started = true
	d.mu.Unlock()

	// Seed from existing gossip peers
	if d.gossip != nil {
		for _, p := range d.gossip.GetPeers() {
			d.addNode(&DHTNode{
				ID:       nodeIDToBytes(p.NodeID),
				NodeID:   p.NodeID,
				Address:  p.Address,
				LastSeen: time.Unix(p.LastSeen, 0),
			})
		}
	}

	go d.refreshLoop()
	log.Printf("[dht] started with %d initial peers", d.routingTableSize())
}

// Stop halts the DHT.
func (d *DHT) Stop() {
	select {
	case <-d.stopCh:
	default:
		close(d.stopCh)
	}
}

// Bootstrap connects to known bootstrap nodes and populates the routing table.
func (d *DHT) Bootstrap(bootstrapAddrs []string) {
	for _, addr := range bootstrapAddrs {
		// FIND_NODE on ourselves to populate our neighborhood
		peers, err := d.remoteFindNode(addr, d.selfID)
		if err != nil {
			log.Printf("[dht] bootstrap %s failed: %v", addr, err)
			continue
		}
		for _, p := range peers {
			d.addNode(&p)
		}
		log.Printf("[dht] bootstrap from %s: got %d peers", addr, len(peers))
	}

	// Do a lookup on ourselves to fill our k-buckets
	d.IterativeFindNode(d.selfID)
}

// IterativeFindNode performs the Kademlia iterative lookup for a target ID.
// Returns the K closest nodes found.
func (d *DHT) IterativeFindNode(target [20]byte) []*DHTNode {
	closest := d.closestNodes(target, DHTKBucketK)
	if len(closest) == 0 {
		return nil
	}

	queried := make(map[[20]byte]bool)
	queried[d.selfID] = true

	for rounds := 0; rounds < 10; rounds++ {
		// Pick alpha unqueried nodes closest to target
		var toQuery []*DHTNode
		for _, n := range closest {
			if !queried[n.ID] {
				toQuery = append(toQuery, n)
				if len(toQuery) >= DHTAlpha {
					break
				}
			}
		}

		if len(toQuery) == 0 {
			break // all closest nodes have been queried
		}

		// Query in parallel
		type result struct {
			peers []DHTNode
		}
		results := make(chan result, len(toQuery))
		for _, n := range toQuery {
			queried[n.ID] = true
			go func(addr string) {
				peers, err := d.remoteFindNode(addr, target)
				if err != nil {
					results <- result{}
					return
				}
				results <- result{peers: peers}
			}(n.Address)
		}

		// Collect results
		for range toQuery {
			r := <-results
			for i := range r.peers {
				d.addNode(&r.peers[i])
			}
		}

		// Rebuild closest list
		closest = d.closestNodes(target, DHTKBucketK)
	}

	return closest
}

// Store stores a key-value pair in the DHT.
// The value is stored on the K closest nodes to the key.
func (d *DHT) Store(key []byte, value []byte) error {
	keyHash := hashKey(key)

	// Store locally
	d.storeLocal(keyHash, value)

	// Find K closest nodes and store on them
	closest := d.IterativeFindNode(keyHash)
	var lastErr error
	stored := 0
	for _, n := range closest {
		if n.ID == d.selfID {
			continue // already stored locally
		}
		if err := d.remoteStore(n.Address, keyHash, value); err != nil {
			lastErr = err
			continue
		}
		stored++
	}

	log.Printf("[dht] stored key %s on %d/%d nodes", hex.EncodeToString(keyHash[:8]), stored+1, len(closest)+1)
	if stored == 0 && len(closest) > 0 {
		return fmt.Errorf("failed to store on any remote node: %w", lastErr)
	}
	return nil
}

// FindValue looks up a value by key in the DHT.
func (d *DHT) FindValue(key []byte) ([]byte, bool) {
	keyHash := hashKey(key)

	// Check local store first
	d.storeMu.RLock()
	if v, ok := d.store[keyHash]; ok {
		d.storeMu.RUnlock()
		return v.Value, true
	}
	d.storeMu.RUnlock()

	// Iterative lookup with value checks
	closest := d.closestNodes(keyHash, DHTKBucketK)
	queried := make(map[[20]byte]bool)
	queried[d.selfID] = true

	for rounds := 0; rounds < 10; rounds++ {
		var toQuery []*DHTNode
		for _, n := range closest {
			if !queried[n.ID] {
				toQuery = append(toQuery, n)
				if len(toQuery) >= DHTAlpha {
					break
				}
			}
		}
		if len(toQuery) == 0 {
			break
		}

		type findResult struct {
			value []byte
			found bool
			peers []DHTNode
		}
		results := make(chan findResult, len(toQuery))

		for _, n := range toQuery {
			queried[n.ID] = true
			go func(addr string) {
				val, peers, err := d.remoteFindValue(addr, keyHash)
				if err != nil {
					results <- findResult{}
					return
				}
				if val != nil {
					results <- findResult{value: val, found: true}
				} else {
					results <- findResult{peers: peers}
				}
			}(n.Address)
		}

		for range toQuery {
			r := <-results
			if r.found {
				// Cache on closest node that didn't have it
				return r.value, true
			}
			for i := range r.peers {
				d.addNode(&r.peers[i])
			}
		}

		closest = d.closestNodes(keyHash, DHTKBucketK)
	}

	return nil, false
}

// ── Local store ──

func (d *DHT) storeLocal(key [20]byte, value []byte) {
	d.storeMu.Lock()
	defer d.storeMu.Unlock()

	d.store[key] = &DHTValue{
		Key:       key,
		Value:     value,
		Publisher: d.self.NodeID,
		StoredAt:  time.Now(),
		TTL:       DHTTTL,
	}

	// Evict if over limit (remove oldest)
	if len(d.store) > DHTMaxStored {
		var oldestKey [20]byte
		oldestTime := time.Now()
		for k, v := range d.store {
			if v.StoredAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.StoredAt
			}
		}
		delete(d.store, oldestKey)
	}
}

// ── Remote RPC operations ──

// remoteFindNode asks a remote node for its closest nodes to target.
func (d *DHT) remoteFindNode(address string, target [20]byte) ([]DHTNode, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"target":  hex.EncodeToString(target[:]),
		"from_id": d.self.NodeID,
		"address": d.self.Address,
	})

	resp, err := d.httpC.Post(address+"/v1/peer/dht/find_node", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Nodes []DHTNode `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Nodes, nil
}

// remoteStore asks a remote node to store a value.
func (d *DHT) remoteStore(address string, key [20]byte, value []byte) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"key":       hex.EncodeToString(key[:]),
		"value":     hex.EncodeToString(value),
		"publisher": d.self.NodeID,
	})

	resp, err := d.httpC.Post(address+"/v1/peer/dht/store", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// remoteFindValue asks a remote node for a value by key.
func (d *DHT) remoteFindValue(address string, key [20]byte) ([]byte, []DHTNode, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"key":     hex.EncodeToString(key[:]),
		"from_id": d.self.NodeID,
		"address": d.self.Address,
	})

	resp, err := d.httpC.Post(address+"/v1/peer/dht/find_value", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Found bool      `json:"found"`
		Value string    `json:"value"` // hex
		Nodes []DHTNode `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}

	if result.Found {
		val, _ := hex.DecodeString(result.Value)
		return val, nil, nil
	}
	return nil, result.Nodes, nil
}

// pingNode checks if a node is still alive.
func (d *DHT) pingNode(n *DHTNode) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", n.Address+"/v1/peer/dht/ping", nil)
	resp, err := d.httpC.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// ── HTTP handlers (called by other nodes) ──

// HandleFindNode handles incoming FIND_NODE requests.
func (d *DHT) HandleFindNode(targetHex, fromID, fromAddr string) []DHTNode {
	// Add requester to our routing table
	if fromID != "" && fromAddr != "" {
		d.addNode(&DHTNode{
			ID:      nodeIDToBytes(fromID),
			NodeID:  fromID,
			Address: fromAddr,
		})
	}

	var target [20]byte
	decoded, _ := hex.DecodeString(targetHex)
	if len(decoded) >= 20 {
		copy(target[:], decoded[:20])
	}

	closest := d.closestNodes(target, DHTKBucketK)
	result := make([]DHTNode, len(closest))
	for i, n := range closest {
		result[i] = *n
	}
	return result
}

// HandleStore handles incoming STORE requests.
func (d *DHT) HandleStore(keyHex string, valueHex string, publisher string) error {
	var key [20]byte
	decoded, err := hex.DecodeString(keyHex)
	if err != nil || len(decoded) < 20 {
		return fmt.Errorf("invalid key")
	}
	copy(key[:], decoded[:20])

	value, err := hex.DecodeString(valueHex)
	if err != nil {
		return fmt.Errorf("invalid value")
	}

	d.storeMu.Lock()
	d.store[key] = &DHTValue{
		Key:       key,
		Value:     value,
		Publisher: publisher,
		StoredAt:  time.Now(),
		TTL:       DHTTTL,
	}
	d.storeMu.Unlock()

	return nil
}

// HandleFindValue handles incoming FIND_VALUE requests.
func (d *DHT) HandleFindValue(keyHex, fromID, fromAddr string) ([]byte, []DHTNode) {
	// Add requester
	if fromID != "" && fromAddr != "" {
		d.addNode(&DHTNode{
			ID:      nodeIDToBytes(fromID),
			NodeID:  fromID,
			Address: fromAddr,
		})
	}

	var key [20]byte
	decoded, _ := hex.DecodeString(keyHex)
	if len(decoded) >= 20 {
		copy(key[:], decoded[:20])
	}

	// Check local store
	d.storeMu.RLock()
	if v, ok := d.store[key]; ok {
		d.storeMu.RUnlock()
		return v.Value, nil
	}
	d.storeMu.RUnlock()

	// Return closest nodes instead
	closest := d.closestNodes(key, DHTKBucketK)
	result := make([]DHTNode, len(closest))
	for i, n := range closest {
		result[i] = *n
	}
	return nil, result
}

// ── Maintenance ──

func (d *DHT) refreshLoop() {
	refreshTicker := time.NewTicker(15 * time.Minute)
	cleanTicker := time.NewTicker(1 * time.Hour)
	defer refreshTicker.Stop()
	defer cleanTicker.Stop()

	for {
		select {
		case <-refreshTicker.C:
			d.refreshBuckets()
		case <-cleanTicker.C:
			d.cleanExpiredStore()
		case <-d.stopCh:
			return
		}
	}
}

// refreshBuckets performs a random lookup in each bucket that hasn't been touched recently.
func (d *DHT) refreshBuckets() {
	for i, bucket := range d.buckets {
		bucket.mu.RLock()
		needsRefresh := len(bucket.nodes) > 0
		bucket.mu.RUnlock()

		if needsRefresh {
			// Generate a random ID in this bucket's range
			var target [20]byte
			copy(target[:], d.selfID[:])
			byteIdx := i / 8
			bitIdx := uint(7 - (i % 8))
			if byteIdx < 20 {
				target[byteIdx] ^= (1 << bitIdx) // flip the bit for this bucket
			}
			d.IterativeFindNode(target)
		}
	}
}

func (d *DHT) cleanExpiredStore() {
	d.storeMu.Lock()
	defer d.storeMu.Unlock()
	now := time.Now()
	for k, v := range d.store {
		if now.Sub(v.StoredAt) > v.TTL {
			delete(d.store, k)
		}
	}
}

// routingTableSize returns the total number of nodes in the routing table.
func (d *DHT) routingTableSize() int {
	total := 0
	for _, bucket := range d.buckets {
		bucket.mu.RLock()
		total += len(bucket.nodes)
		bucket.mu.RUnlock()
	}
	return total
}

// Stats returns DHT statistics.
func (d *DHT) Stats() map[string]interface{} {
	bucketSizes := make([]int, 0)
	for _, bucket := range d.buckets {
		bucket.mu.RLock()
		if len(bucket.nodes) > 0 {
			bucketSizes = append(bucketSizes, len(bucket.nodes))
		}
		bucket.mu.RUnlock()
	}

	d.storeMu.RLock()
	storeSize := len(d.store)
	d.storeMu.RUnlock()

	return map[string]interface{}{
		"node_id":        d.self.NodeID,
		"routing_table":  d.routingTableSize(),
		"active_buckets": len(bucketSizes),
		"bucket_sizes":   bucketSizes,
		"store_size":     storeSize,
	}
}
