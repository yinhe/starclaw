package node

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// ConnMethod describes how a connection to a peer was established.
type ConnMethod string

const (
	ConnDirect ConnMethod = "direct" // peer has public IP, no NAT traversal needed
	ConnPunch  ConnMethod = "punch"  // UDP hole-punching succeeded
	ConnRelay  ConnMethod = "relay"  // traffic relayed through a public node
)

// PeerConn represents an established connection to a remote peer.
type PeerConn struct {
	NodeID     string         `json:"node_id"`
	RemoteAddr *net.UDPAddr   `json:"remote_addr"`
	Method     ConnMethod     `json:"method"`
	Conn       net.PacketConn `json:"-"`
	RelayURL   string         `json:"relay_url,omitempty"` // set if Method == ConnRelay
	CreatedAt  time.Time      `json:"created_at"`
	LastUsed   time.Time      `json:"last_used"`
}

// NydusManager orchestrates NAT traversal: STUN probe → hole punch → relay fallback.
// It maintains a pool of established peer connections and handles reconnection.
type NydusManager struct {
	identity *Identity
	stun     *STUNProber
	puncher  *HolePuncher
	relay    *RelayClient

	mu          sync.RWMutex
	connections map[string]*PeerConn // nodeID -> connection
	stunResult  *STUNResult
	started     bool
	stopCh      chan struct{}

	// Callbacks
	onNATDiscovered func(result *STUNResult)
}

// NydusConfig holds configuration for the NAT traversal manager.
type NydusConfig struct {
	STUNServers []string // custom STUN servers
	RelayURLs   []string // relay fallback URLs
}

// NewNydusManager creates the NAT traversal manager.
func NewNydusManager(identity *Identity, cfg NydusConfig) *NydusManager {
	stunProber := NewSTUNProber(cfg.STUNServers...)
	relayClient := NewRelayClient(identity, cfg.RelayURLs...)

	return &NydusManager{
		identity:    identity,
		stun:        stunProber,
		puncher:     NewHolePuncher(identity, stunProber),
		relay:       relayClient,
		connections: make(map[string]*PeerConn),
		stopCh:      make(chan struct{}),
	}
}

// Start begins the NAT traversal lifecycle:
// 1. STUN probe to discover public IP and NAT type
// 2. Periodic re-probe to detect IP changes
func (nm *NydusManager) Start() error {
	nm.mu.Lock()
	if nm.started {
		nm.mu.Unlock()
		return nil
	}
	nm.started = true
	nm.mu.Unlock()

	// Initial STUN probe
	result, err := nm.stun.ProbeNATType()
	if err != nil {
		log.Printf("[nydus] initial STUN probe failed: %v (will retry)", err)
	} else {
		nm.mu.Lock()
		nm.stunResult = result
		nm.mu.Unlock()

		log.Printf("[nydus] started: public=%s:%d NAT=%s",
			result.PublicIP, result.PublicPort, result.NATType)

		if nm.onNATDiscovered != nil {
			nm.onNATDiscovered(result)
		}
	}

	// Background: periodic re-probe and connection maintenance
	go nm.maintenanceLoop()

	return nil
}

// Stop shuts down the NAT traversal manager and closes all connections.
func (nm *NydusManager) Stop() {
	close(nm.stopCh)

	nm.mu.Lock()
	defer nm.mu.Unlock()

	for _, pc := range nm.connections {
		if pc.Conn != nil {
			pc.Conn.Close()
		}
	}
	nm.connections = make(map[string]*PeerConn)
	nm.started = false
	log.Println("[nydus] stopped")
}

// Connect establishes a connection to a remote peer, trying punch then relay.
func (nm *NydusManager) Connect(ctx context.Context, remoteNodeID string, remoteInfo *PunchRequest) (*PeerConn, error) {
	// Check for existing connection
	nm.mu.RLock()
	if existing, ok := nm.connections[remoteNodeID]; ok {
		existing.LastUsed = time.Now()
		nm.mu.RUnlock()
		return existing, nil
	}
	nm.mu.RUnlock()

	nm.mu.RLock()
	localSTUN := nm.stunResult
	nm.mu.RUnlock()

	// If we don't have STUN result, try probing now
	if localSTUN == nil {
		var err error
		localSTUN, err = nm.stun.ProbeNATType()
		if err != nil {
			log.Printf("[nydus] STUN probe failed, going directly to relay: %v", err)
			return nm.connectViaRelay(ctx, remoteNodeID)
		}
		nm.mu.Lock()
		nm.stunResult = localSTUN
		nm.mu.Unlock()
	}

	// Decide strategy based on NAT types
	if remoteInfo != nil && CanPunch(localSTUN.NATType, remoteInfo.NATType) {
		// Try hole punching
		log.Printf("[nydus] attempting punch to %s (local=%s, remote=%s)",
			remoteNodeID[:min(16, len(remoteNodeID))], localSTUN.NATType, remoteInfo.NATType)

		result, err := nm.puncher.Punch(remoteInfo, 5*time.Second)
		if err == nil && result.Success {
			pc := &PeerConn{
				NodeID:     remoteNodeID,
				RemoteAddr: result.RemoteAddr,
				Method:     ConnMethod(result.Method),
				Conn:       result.Conn,
				CreatedAt:  time.Now(),
				LastUsed:   time.Now(),
			}
			nm.mu.Lock()
			nm.connections[remoteNodeID] = pc
			nm.mu.Unlock()

			log.Printf("[nydus] punch succeeded to %s via %s (%dms)",
				remoteNodeID[:min(16, len(remoteNodeID))], result.Method, result.LatencyMs)
			return pc, nil
		}

		log.Printf("[nydus] punch failed to %s: %v, falling back to relay", remoteNodeID[:min(16, len(remoteNodeID))], err)
	}

	// Fallback: relay
	return nm.connectViaRelay(ctx, remoteNodeID)
}

// connectViaRelay establishes a relay-based connection.
func (nm *NydusManager) connectViaRelay(ctx context.Context, remoteNodeID string) (*PeerConn, error) {
	// Send a ping through relay to verify it works
	err := nm.relay.SendPunchRequest(ctx, &PunchRequest{
		FromNodeID: nm.identity.NodeID,
		ToNodeID:   remoteNodeID,
		NATType:    NATUnknown,
		Nonce:      generateNonce(),
	})
	if err != nil {
		return nil, fmt.Errorf("relay connection failed: %w", err)
	}

	pc := &PeerConn{
		NodeID:    remoteNodeID,
		Method:    ConnRelay,
		RelayURL:  nm.relay.ActiveRelay(),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}

	nm.mu.Lock()
	nm.connections[remoteNodeID] = pc
	nm.mu.Unlock()

	log.Printf("[nydus] relay connection to %s via %s",
		remoteNodeID[:min(16, len(remoteNodeID))], pc.RelayURL)
	return pc, nil
}

// Disconnect removes a peer connection.
func (nm *NydusManager) Disconnect(nodeID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if pc, ok := nm.connections[nodeID]; ok {
		if pc.Conn != nil {
			pc.Conn.Close()
		}
		delete(nm.connections, nodeID)
	}
}

// GetConnection returns an existing connection to a peer, if any.
func (nm *NydusManager) GetConnection(nodeID string) *PeerConn {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.connections[nodeID]
}

// PublicEndpoint returns the discovered public IP:port, or empty if unknown.
func (nm *NydusManager) PublicEndpoint() string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	if nm.stunResult == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", nm.stunResult.PublicIP, nm.stunResult.PublicPort)
}

// NATType returns the detected NAT type.
func (nm *NydusManager) GetNATType() NATType {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	if nm.stunResult == nil {
		return NATUnknown
	}
	return nm.stunResult.NATType
}

// STUNResult returns the latest STUN probe result.
func (nm *NydusManager) GetSTUNResult() *STUNResult {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.stunResult
}

// OnNATDiscovered sets a callback for when NAT type is discovered or changes.
func (nm *NydusManager) OnNATDiscovered(fn func(*STUNResult)) {
	nm.onNATDiscovered = fn
}

// RelayClient returns the relay client for forwarding data.
func (nm *NydusManager) RelayClient() *RelayClient {
	return nm.relay
}

// Stats returns current Nydus connection statistics.
func (nm *NydusManager) Stats() map[string]interface{} {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	direct, punched, relayed := 0, 0, 0
	for _, pc := range nm.connections {
		switch pc.Method {
		case ConnDirect:
			direct++
		case ConnPunch:
			punched++
		case ConnRelay:
			relayed++
		}
	}

	natType := NATUnknown
	publicEndpoint := ""
	if nm.stunResult != nil {
		natType = nm.stunResult.NATType
		publicEndpoint = fmt.Sprintf("%s:%d", nm.stunResult.PublicIP, nm.stunResult.PublicPort)
	}

	return map[string]interface{}{
		"nat_type":        natType,
		"public_endpoint": publicEndpoint,
		"connections": map[string]int{
			"total":   len(nm.connections),
			"direct":  direct,
			"punched": punched,
			"relayed": relayed,
		},
	}
}

// maintenanceLoop periodically re-probes STUN and cleans stale connections.
func (nm *NydusManager) maintenanceLoop() {
	stunTicker := time.NewTicker(5 * time.Minute)
	cleanTicker := time.NewTicker(1 * time.Minute)
	defer stunTicker.Stop()
	defer cleanTicker.Stop()

	for {
		select {
		case <-stunTicker.C:
			// Re-probe STUN to detect IP changes
			result, err := nm.stun.ProbeNATType()
			if err != nil {
				log.Printf("[nydus] periodic STUN probe failed: %v", err)
				continue
			}

			nm.mu.Lock()
			oldResult := nm.stunResult
			nm.stunResult = result
			nm.mu.Unlock()

			// Check if public IP changed
			if oldResult != nil && !oldResult.PublicIP.Equal(result.PublicIP) {
				log.Printf("[nydus] public IP changed: %s → %s", oldResult.PublicIP, result.PublicIP)
				if nm.onNATDiscovered != nil {
					nm.onNATDiscovered(result)
				}
			}

		case <-cleanTicker.C:
			// Remove connections unused for > 10 minutes
			nm.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for nodeID, pc := range nm.connections {
				if pc.LastUsed.Before(cutoff) {
					if pc.Conn != nil {
						pc.Conn.Close()
					}
					delete(nm.connections, nodeID)
					log.Printf("[nydus] cleaned stale connection to %s", nodeID[:min(16, len(nodeID))])
				}
			}
			nm.mu.Unlock()

		case <-nm.stopCh:
			return
		}
	}
}

// generateNonce creates a random 16-byte hex nonce for punch coordination.
func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
