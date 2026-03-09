package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// PeerInfo is the gossip-exchanged peer record
type PeerInfo struct {
	NodeID    string `json:"node_id"`
	Address   string `json:"address"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Region    string `json:"region"`
	PublicKey string `json:"public_key"`
	LastSeen  int64  `json:"last_seen"`
}

// GossipEngine periodically shares known peers with all neighbors
type GossipEngine struct {
	identity *Identity
	address  string // this node's address
	peers    map[string]*PeerInfo // node_id -> info
	mu       sync.RWMutex
	httpC    *http.Client
	stopCh   chan struct{}
	onChange func(peers []PeerInfo) // callback when peer list changes
}

// NewGossipEngine creates the gossip engine
func NewGossipEngine(identity *Identity, address string, onChange func(peers []PeerInfo)) *GossipEngine {
	return &GossipEngine{
		identity: identity,
		address:  address,
		peers:    make(map[string]*PeerInfo),
		httpC:    &http.Client{Timeout: 10 * time.Second},
		stopCh:   make(chan struct{}),
		onChange: onChange,
	}
}

// SetAddress updates this node's advertised address
func (g *GossipEngine) SetAddress(addr string) {
	g.mu.Lock()
	g.address = addr
	g.mu.Unlock()
}

// Start begins the gossip loop
func (g *GossipEngine) Start(interval time.Duration) {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	go func() {
		// Initial gossip after short delay
		time.Sleep(5 * time.Second)
		g.gossipRound()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				g.gossipRound()
			case <-g.stopCh:
				return
			}
		}
	}()
	log.Printf("[gossip] started with interval %s", interval)
}

// Stop halts the gossip loop
func (g *GossipEngine) Stop() {
	select {
	case <-g.stopCh:
	default:
		close(g.stopCh)
	}
}

// AddPeer adds or updates a peer in the gossip table
func (g *GossipEngine) AddPeer(info PeerInfo) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	existing, ok := g.peers[info.NodeID]
	if ok && existing.LastSeen >= info.LastSeen {
		return false // already have fresher info
	}

	g.peers[info.NodeID] = &info
	return true
}

// RemovePeer removes a peer from the gossip table
func (g *GossipEngine) RemovePeer(nodeID string) {
	g.mu.Lock()
	delete(g.peers, nodeID)
	g.mu.Unlock()
}

// GetPeers returns a snapshot of all known peers
func (g *GossipEngine) GetPeers() []PeerInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]PeerInfo, 0, len(g.peers))
	for _, p := range g.peers {
		result = append(result, *p)
	}
	return result
}

// gossipRound sends this node's peer list to all known peers
func (g *GossipEngine) gossipRound() {
	g.mu.RLock()
	myPeers := make([]PeerInfo, 0, len(g.peers)+1)
	// Include self
	myPeers = append(myPeers, PeerInfo{
		NodeID:    g.identity.NodeID,
		Address:   g.address,
		PublicKey: g.identity.PublicKeyHex(),
		LastSeen:  time.Now().Unix(),
	})
	// Include all known peers
	for _, p := range g.peers {
		myPeers = append(myPeers, *p)
	}
	targets := make([]string, 0)
	for _, p := range g.peers {
		if p.Address != "" {
			targets = append(targets, p.Address)
		}
	}
	g.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	// Sign the gossip payload
	challenge, signature := g.identity.SignChallenge()
	payload := map[string]interface{}{
		"from_node_id": g.identity.NodeID,
		"public_key":   g.identity.PublicKeyHex(),
		"challenge":    challenge,
		"signature":    signature,
		"peers":        myPeers,
	}
	data, _ := json.Marshal(payload)

	for _, addr := range targets {
		go g.sendGossip(addr, data)
	}
}

// sendGossip sends gossip to a single peer
func (g *GossipEngine) sendGossip(address string, data []byte) {
	url := address + "/v1/peer/gossip"
	resp, err := g.httpC.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return // silently fail, peer might be offline
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var result struct {
			Peers []PeerInfo `json:"peers"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil {
			changed := false
			for _, p := range result.Peers {
				if p.NodeID == g.identity.NodeID {
					continue // skip self
				}
				if g.AddPeer(p) {
					changed = true
				}
			}
			if changed && g.onChange != nil {
				g.onChange(g.GetPeers())
			}
		}
	}
}

// HandleGossip processes incoming gossip from a remote peer
func (g *GossipEngine) HandleGossip(fromNodeID, publicKey, challenge, signature string, remotePeers []PeerInfo) ([]PeerInfo, error) {
	// Verify signature
	if !VerifySignature(publicKey, []byte(challenge), signature) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Verify node_id matches public key
	derivedID, err := DeriveNodeIDFromPubKey(publicKey)
	if err != nil || derivedID != fromNodeID {
		return nil, fmt.Errorf("node_id does not match public key")
	}

	// Merge remote peers into our table
	changed := false
	for _, p := range remotePeers {
		if p.NodeID == g.identity.NodeID {
			continue // skip self
		}
		if g.AddPeer(p) {
			changed = true
		}
	}

	if changed && g.onChange != nil {
		g.onChange(g.GetPeers())
	}

	// Return our peer list
	return g.GetPeers(), nil
}
