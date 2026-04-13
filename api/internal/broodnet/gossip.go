package broodnet

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// BroodOS Network — GossipNet (P2P节点发现与状态传播)
//
// Lightweight gossip protocol for decentralized node discovery:
//   - Nodes announce their capabilities and status
//   - State propagates via peer exchange (pull-based gossip)
//   - Failure detection via heartbeat timeout
//   - Capability-based routing (find nodes with specific skills)
//
// Integration:
//   - TaskMarket: discover capable nodes for task matching
//   - Orchestrator: find best routes for sub-tasks
//   - Formation: discover potential formation members
//   - Reputation: cross-reference node profiles
//
// Pull model: each node periodically pulls state from known peers
// No central coordinator — fully decentralized
// ════════════════════════════════════════════════════════════

// ── Node States ──

type NodeState string

const (
	NodeAlive    NodeState = "alive"
	NodeSuspect  NodeState = "suspect"  // missed heartbeats
	NodeDead     NodeState = "dead"     // confirmed unreachable
	NodeLeft     NodeState = "left"     // graceful departure
)

// ── Data Types ──

// PeerInfo represents a discovered node in the gossip network
type PeerInfo struct {
	NodeID       string            `json:"node_id"`
	Address      string            `json:"address"`
	ClawID       string            `json:"claw_id,omitempty"`
	State        NodeState         `json:"state"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Categories   []TaskCategory    `json:"categories,omitempty"`  // task categories this node handles
	Metadata     map[string]string `json:"metadata,omitempty"`    // free-form key/value
	RepScore     float64           `json:"rep_score,omitempty"`
	RepTier      TrustTier         `json:"rep_tier,omitempty"`
	Version      int64             `json:"version"`               // monotonic, bump on change
	LastSeen     time.Time         `json:"last_seen"`
	JoinedAt     time.Time         `json:"joined_at"`
}

// GossipMessage is exchanged between nodes during gossip rounds
type GossipMessage struct {
	SenderID  string      `json:"sender_id"`
	Peers     []*PeerInfo `json:"peers"`
	Timestamp time.Time   `json:"timestamp"`
}

// ── GossipNet Engine ──

// GossipConfig holds gossip protocol settings
type GossipConfig struct {
	MaxPeers          int           `json:"max_peers"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	SuspectTimeout    time.Duration `json:"suspect_timeout"`   // alive → suspect
	DeadTimeout       time.Duration `json:"dead_timeout"`      // suspect → dead
	PruneInterval     time.Duration `json:"prune_interval"`    // remove dead nodes
}

// DefaultGossipConfig returns production defaults
func DefaultGossipConfig() *GossipConfig {
	return &GossipConfig{
		MaxPeers:          500,
		HeartbeatInterval: 30 * time.Second,
		SuspectTimeout:    90 * time.Second,  // 3 missed heartbeats
		DeadTimeout:       5 * time.Minute,
		PruneInterval:     30 * time.Minute,
	}
}

// GossipNet manages decentralized node discovery
type GossipNet struct {
	mu       sync.RWMutex
	config   *GossipConfig
	selfID   string
	selfAddr string
	selfCaps []string
	selfCats []TaskCategory
	selfMeta map[string]string
	peers    map[string]*PeerInfo
	version  int64
	stats    GossipStats
}

// GossipStats tracks gossip metrics
type GossipStats struct {
	TotalPeers     int                  `json:"total_peers"`
	AlivePeers     int                  `json:"alive_peers"`
	SuspectPeers   int                  `json:"suspect_peers"`
	DeadPeers      int                  `json:"dead_peers"`
	GossipRounds   int                  `json:"gossip_rounds"`
	MergesApplied  int                  `json:"merges_applied"`
	PeersPruned    int                  `json:"peers_pruned"`
	ByCapability   map[string]int       `json:"by_capability"`
	ByCategory     map[TaskCategory]int `json:"by_category"`
}

var (
	globalGossip *GossipNet
	gossipOnce   sync.Once
)

// InitGossip creates the global gossip network
func InitGossip(nodeID, address string, caps []string, cats []TaskCategory, cfg *GossipConfig) *GossipNet {
	if cfg == nil {
		cfg = DefaultGossipConfig()
	}
	gossipOnce.Do(func() {
		globalGossip = &GossipNet{
			config:   cfg,
			selfID:   nodeID,
			selfAddr: address,
			selfCaps: caps,
			selfCats: cats,
			selfMeta: make(map[string]string),
			peers:    make(map[string]*PeerInfo),
			version:  1,
			stats: GossipStats{
				ByCapability: make(map[string]int),
				ByCategory:   make(map[TaskCategory]int),
			},
		}

		// Register self
		now := time.Now()
		globalGossip.peers[nodeID] = &PeerInfo{
			NodeID:       nodeID,
			Address:      address,
			Capabilities: caps,
			Categories:   cats,
			State:        NodeAlive,
			Version:      1,
			LastSeen:     now,
			JoinedAt:     now,
		}
		globalGossip.stats.TotalPeers = 1
		globalGossip.stats.AlivePeers = 1

		log.Printf("[broodnet/gossip] network ready (node=%s, caps=%v, cats=%v)",
			nodeID, caps, cats)
	})
	return globalGossip
}

// GetGossip returns the global gossip net
func GetGossip() *GossipNet {
	return globalGossip
}

// ── Announce & Merge ──

// Announce registers or updates a peer in the local view
func (gn *GossipNet) Announce(peer *PeerInfo) error {
	if peer.NodeID == "" {
		return nil
	}

	gn.mu.Lock()
	defer gn.mu.Unlock()

	existing, ok := gn.peers[peer.NodeID]
	if ok {
		// Only accept newer versions
		if peer.Version <= existing.Version {
			return nil
		}
		// Update existing
		existing.Address = peer.Address
		existing.Capabilities = peer.Capabilities
		existing.Categories = peer.Categories
		existing.Metadata = peer.Metadata
		existing.Version = peer.Version
		existing.State = NodeAlive
		existing.LastSeen = time.Now()
		if peer.RepScore > 0 {
			existing.RepScore = peer.RepScore
			existing.RepTier = peer.RepTier
		}
	} else {
		// New peer
		if len(gn.peers) >= gn.config.MaxPeers {
			gn.pruneOldest()
		}
		peer.State = NodeAlive
		peer.LastSeen = time.Now()
		if peer.JoinedAt.IsZero() {
			peer.JoinedAt = time.Now()
		}
		gn.peers[peer.NodeID] = peer
		gn.stats.TotalPeers++
		gn.stats.AlivePeers++

		// Update capability/category indexes
		for _, c := range peer.Capabilities {
			gn.stats.ByCapability[c]++
		}
		for _, cat := range peer.Categories {
			gn.stats.ByCategory[cat]++
		}
	}

	gn.stats.MergesApplied++
	return nil
}

// MergeGossip processes a gossip message from a peer
func (gn *GossipNet) MergeGossip(msg *GossipMessage) int {
	merged := 0
	for _, peer := range msg.Peers {
		if err := gn.Announce(peer); err == nil {
			merged++
		}
	}
	gn.mu.Lock()
	gn.stats.GossipRounds++
	gn.mu.Unlock()
	return merged
}

// PrepareGossip builds a gossip message with our current peer view
func (gn *GossipNet) PrepareGossip() *GossipMessage {
	gn.mu.RLock()
	defer gn.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(gn.peers))
	for _, p := range gn.peers {
		if p.State == NodeAlive || p.State == NodeSuspect {
			peers = append(peers, p)
		}
	}

	return &GossipMessage{
		SenderID:  gn.selfID,
		Peers:     peers,
		Timestamp: time.Now(),
	}
}

// Leave gracefully removes this node from the network
func (gn *GossipNet) Leave() {
	gn.mu.Lock()
	defer gn.mu.Unlock()

	if self, ok := gn.peers[gn.selfID]; ok {
		self.State = NodeLeft
		self.Version++
	}
}

// ── Health Check ──

// HealthSweep updates peer states based on timeouts
func (gn *GossipNet) HealthSweep() (suspects, dead int) {
	gn.mu.Lock()
	defer gn.mu.Unlock()

	now := time.Now()
	for _, p := range gn.peers {
		if p.NodeID == gn.selfID {
			p.LastSeen = now // self is always alive
			continue
		}

		sinceLastSeen := now.Sub(p.LastSeen)

		switch p.State {
		case NodeAlive:
			if sinceLastSeen > gn.config.SuspectTimeout {
				p.State = NodeSuspect
				gn.stats.AlivePeers--
				gn.stats.SuspectPeers++
				suspects++
			}
		case NodeSuspect:
			if sinceLastSeen > gn.config.DeadTimeout {
				p.State = NodeDead
				gn.stats.SuspectPeers--
				gn.stats.DeadPeers++
				dead++
				log.Printf("[broodnet/gossip] node %s declared dead (last seen %s ago)",
					p.NodeID, sinceLastSeen.Round(time.Second))
			}
		}
	}

	return suspects, dead
}

// Prune removes dead/left nodes older than prune interval
func (gn *GossipNet) Prune() int {
	gn.mu.Lock()
	defer gn.mu.Unlock()

	cutoff := time.Now().Add(-gn.config.PruneInterval)
	pruned := 0
	for id, p := range gn.peers {
		if id == gn.selfID {
			continue
		}
		if (p.State == NodeDead || p.State == NodeLeft) && p.LastSeen.Before(cutoff) {
			delete(gn.peers, id)
			pruned++
		}
	}
	gn.stats.PeersPruned += pruned
	return pruned
}

// pruneOldest removes the oldest dead/left peer to make room (caller holds lock)
func (gn *GossipNet) pruneOldest() {
	var oldestID string
	var oldestTime time.Time

	// First try to prune dead/left
	for id, p := range gn.peers {
		if id == gn.selfID {
			continue
		}
		if p.State == NodeDead || p.State == NodeLeft {
			if oldestID == "" || p.LastSeen.Before(oldestTime) {
				oldestID = id
				oldestTime = p.LastSeen
			}
		}
	}

	// If no dead/left, prune oldest suspect
	if oldestID == "" {
		for id, p := range gn.peers {
			if id == gn.selfID {
				continue
			}
			if p.State == NodeSuspect {
				if oldestID == "" || p.LastSeen.Before(oldestTime) {
					oldestID = id
					oldestTime = p.LastSeen
				}
			}
		}
	}

	if oldestID != "" {
		delete(gn.peers, oldestID)
		gn.stats.PeersPruned++
	}
}

// ── Discovery (capability-based routing) ──

// FindByCapability returns alive nodes that have a specific capability
func (gn *GossipNet) FindByCapability(capability string) []*PeerInfo {
	gn.mu.RLock()
	defer gn.mu.RUnlock()

	var result []*PeerInfo
	for _, p := range gn.peers {
		if p.State != NodeAlive {
			continue
		}
		for _, c := range p.Capabilities {
			if c == capability {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// FindByCategory returns alive nodes that handle a specific task category
func (gn *GossipNet) FindByCategory(category TaskCategory) []*PeerInfo {
	gn.mu.RLock()
	defer gn.mu.RUnlock()

	var result []*PeerInfo
	for _, p := range gn.peers {
		if p.State != NodeAlive {
			continue
		}
		for _, cat := range p.Categories {
			if cat == category {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// FindByTier returns alive nodes at or above a trust tier
func (gn *GossipNet) FindByTier(minTier TrustTier) []*PeerInfo {
	gn.mu.RLock()
	defer gn.mu.RUnlock()

	minScore := tierMinScore(minTier)
	var result []*PeerInfo
	for _, p := range gn.peers {
		if p.State != NodeAlive {
			continue
		}
		if p.RepScore >= minScore {
			result = append(result, p)
		}
	}
	return result
}

func tierMinScore(tier TrustTier) float64 {
	switch tier {
	case TierLegendary:
		return 950
	case TierElite:
		return 800
	case TierVeteran:
		return 500
	case TierReliable:
		return 200
	default:
		return 0
	}
}

// ── Query ──

// GetPeer returns info about a specific peer
func (gn *GossipNet) GetPeer(nodeID string) *PeerInfo {
	gn.mu.RLock()
	defer gn.mu.RUnlock()
	return gn.peers[nodeID]
}

// ListPeers returns all peers filtered by state
func (gn *GossipNet) ListPeers(state NodeState) []*PeerInfo {
	gn.mu.RLock()
	defer gn.mu.RUnlock()

	var result []*PeerInfo
	for _, p := range gn.peers {
		if state != "" && p.State != state {
			continue
		}
		result = append(result, p)
	}
	return result
}

// AlivePeers returns only alive peers
func (gn *GossipNet) AlivePeers() []*PeerInfo {
	return gn.ListPeers(NodeAlive)
}

// UpdateSelfMeta updates this node's metadata and bumps version
func (gn *GossipNet) UpdateSelfMeta(meta map[string]string) {
	gn.mu.Lock()
	defer gn.mu.Unlock()

	gn.version++
	gn.selfMeta = meta
	if self, ok := gn.peers[gn.selfID]; ok {
		self.Metadata = meta
		self.Version = gn.version
		self.LastSeen = time.Now()
	}
}

// Topology returns a JSON-serializable view of the network topology
func (gn *GossipNet) Topology() map[string]interface{} {
	gn.mu.RLock()
	defer gn.mu.RUnlock()

	nodes := make([]map[string]interface{}, 0, len(gn.peers))
	for _, p := range gn.peers {
		nodes = append(nodes, map[string]interface{}{
			"id":           p.NodeID,
			"state":        p.State,
			"capabilities": p.Capabilities,
			"categories":   p.Categories,
			"rep_score":    p.RepScore,
			"rep_tier":     p.RepTier,
			"last_seen":    p.LastSeen,
			"is_self":      p.NodeID == gn.selfID,
		})
	}

	return map[string]interface{}{
		"self_id": gn.selfID,
		"nodes":   nodes,
		"total":   len(gn.peers),
		"alive":   gn.stats.AlivePeers,
		"suspect": gn.stats.SuspectPeers,
		"dead":    gn.stats.DeadPeers,
	}
}

// Stats returns gossip metrics
func (gn *GossipNet) GossipStats() *GossipStats {
	gn.mu.RLock()
	defer gn.mu.RUnlock()
	s := gn.stats
	return &s
}

// Config returns current config
func (gn *GossipNet) GossipConfig() GossipConfig {
	gn.mu.RLock()
	defer gn.mu.RUnlock()
	return *gn.config
}

// SelfInfo returns this node's peer info for serialization
func (gn *GossipNet) SelfInfo() json.RawMessage {
	gn.mu.RLock()
	defer gn.mu.RUnlock()
	if self, ok := gn.peers[gn.selfID]; ok {
		data, _ := json.Marshal(self)
		return data
	}
	return nil
}
