package node

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// PunchRequest is sent via a signaling channel (HTTP/Gossip) to coordinate hole punching.
type PunchRequest struct {
	FromNodeID string  `json:"from_node_id"`
	ToNodeID   string  `json:"to_node_id"`
	PublicIP   string  `json:"public_ip"`
	PublicPort int     `json:"public_port"`
	LocalIP    string  `json:"local_ip"`
	LocalPort  int     `json:"local_port"`
	NATType    NATType `json:"nat_type"`
	Nonce      string  `json:"nonce"` // shared nonce for verification
}

// PunchResult reports the outcome of a hole-punch attempt.
type PunchResult struct {
	Success    bool           `json:"success"`
	RemoteAddr *net.UDPAddr   `json:"remote_addr,omitempty"`
	Conn       net.PacketConn `json:"-"`      // the punched-through UDP connection
	Method     string         `json:"method"` // "direct", "punch", "relay"
	LatencyMs  int64          `json:"latency_ms"`
}

// HolePuncher manages UDP hole-punching attempts.
type HolePuncher struct {
	identity *Identity
	stun     *STUNProber
	mu       sync.Mutex
}

// NewHolePuncher creates a new hole puncher.
func NewHolePuncher(identity *Identity, stunProber *STUNProber) *HolePuncher {
	return &HolePuncher{
		identity: identity,
		stun:     stunProber,
	}
}

// BuildPunchRequest creates a PunchRequest from our current STUN result.
func (hp *HolePuncher) BuildPunchRequest(toNodeID string, stunResult *STUNResult, nonce string) *PunchRequest {
	return &PunchRequest{
		FromNodeID: hp.identity.NodeID,
		ToNodeID:   toNodeID,
		PublicIP:   stunResult.PublicIP.String(),
		PublicPort: stunResult.PublicPort,
		LocalIP:    stunResult.LocalIP.String(),
		LocalPort:  stunResult.LocalPort,
		NATType:    stunResult.NATType,
		Nonce:      nonce,
	}
}

// Punch attempts to establish a direct UDP connection with a remote peer.
// It tries multiple strategies in order:
//  1. Direct connection (if peer has public IP / Full Cone NAT)
//  2. Simultaneous UDP hole punching (both sides send packets)
//  3. Returns failure (caller should fall back to relay)
func (hp *HolePuncher) Punch(remote *PunchRequest, timeout time.Duration) (*PunchResult, error) {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Parse remote addresses
	publicAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", remote.PublicIP, remote.PublicPort))
	if err != nil {
		return nil, fmt.Errorf("invalid remote public address: %w", err)
	}

	// Open local UDP socket (use same port as STUN probe if possible)
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	start := time.Now()

	// Strategy 1: If remote has no NAT or Full Cone, direct connect
	if remote.NATType == NATNone || remote.NATType == NATFullCone {
		log.Printf("[nydus/punch] trying direct connect to %s (NAT: %s)", publicAddr, remote.NATType)
		if result := hp.tryDirect(conn, publicAddr, remote.Nonce, timeout/2); result != nil {
			result.LatencyMs = time.Since(start).Milliseconds()
			return result, nil
		}
	}

	// Strategy 2: Simultaneous hole punch
	// Both sides send UDP packets to each other's public endpoint simultaneously.
	// NAT will create a mapping when outgoing packet is sent, allowing incoming packet through.
	log.Printf("[nydus/punch] trying simultaneous punch to %s (NAT: %s)", publicAddr, remote.NATType)

	// Also try local address if on same LAN
	var localAddr *net.UDPAddr
	if remote.LocalIP != "" && remote.LocalPort > 0 {
		localAddr, _ = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", remote.LocalIP, remote.LocalPort))
	}

	result := hp.simultaneousPunch(conn, publicAddr, localAddr, remote.Nonce, timeout)
	if result != nil {
		result.LatencyMs = time.Since(start).Milliseconds()
		return result, nil
	}

	conn.Close()
	return &PunchResult{Success: false, Method: "failed"}, fmt.Errorf("hole punch failed to %s", remote.FromNodeID[:min(16, len(remote.FromNodeID))])
}

// tryDirect sends a probe packet and waits for a response.
func (hp *HolePuncher) tryDirect(conn net.PacketConn, addr *net.UDPAddr, nonce string, timeout time.Duration) *PunchResult {
	probe := hp.buildProbe(nonce)

	// Send probe
	if _, err := conn.WriteTo(probe, addr); err != nil {
		return nil
	}

	// Wait for response
	conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 512)
	n, remoteAddr, err := conn.ReadFrom(buf)
	if err != nil {
		return nil
	}

	if hp.verifyProbe(buf[:n], nonce) {
		return &PunchResult{
			Success:    true,
			RemoteAddr: remoteAddr.(*net.UDPAddr),
			Conn:       conn,
			Method:     "direct",
		}
	}
	return nil
}

// simultaneousPunch sends rapid probe packets to create NAT mappings.
func (hp *HolePuncher) simultaneousPunch(conn net.PacketConn, publicAddr, localAddr *net.UDPAddr, nonce string, timeout time.Duration) *PunchResult {
	probe := hp.buildProbe(nonce)
	deadline := time.Now().Add(timeout)

	// Send bursts of packets to both public and local addresses
	// This creates NAT port mappings that allow return traffic
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			{
				if time.Now().After(deadline) {
					return
				}
				conn.WriteTo(probe, publicAddr)
				if localAddr != nil {
					conn.WriteTo(probe, localAddr)
				}
			}
		}
	}()

	// Listen for incoming probes
	conn.SetReadDeadline(deadline)
	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		n, remoteAddr, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		if hp.verifyProbe(buf[:n], nonce) {
			// Send acknowledgment
			conn.WriteTo(probe, remoteAddr)

			return &PunchResult{
				Success:    true,
				RemoteAddr: remoteAddr.(*net.UDPAddr),
				Conn:       conn,
				Method:     "punch",
			}
		}
	}

	return nil
}

// punchProbe is the wire format for hole-punch probes.
type punchProbe struct {
	Type   string `json:"type"`
	NodeID string `json:"node_id"`
	Nonce  string `json:"nonce"`
	Ts     int64  `json:"ts"`
}

// buildProbe creates a signed probe packet.
func (hp *HolePuncher) buildProbe(nonce string) []byte {
	p := punchProbe{
		Type:   "nydus_punch",
		NodeID: hp.identity.NodeID,
		Nonce:  nonce,
		Ts:     time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(p)
	return data
}

// verifyProbe checks that a received probe is valid.
func (hp *HolePuncher) verifyProbe(data []byte, expectedNonce string) bool {
	var p punchProbe
	if err := json.Unmarshal(data, &p); err != nil {
		return false
	}
	if p.Type != "nydus_punch" {
		return false
	}
	if p.Nonce != expectedNonce {
		return false
	}
	// Reject probes older than 30 seconds
	if time.Since(time.UnixMilli(p.Ts)) > 30*time.Second {
		return false
	}
	// Don't accept our own probes
	if p.NodeID == hp.identity.NodeID {
		return false
	}
	return true
}

// CanPunch estimates whether hole punching is likely to succeed
// based on the NAT types of both sides.
func CanPunch(local, remote NATType) bool {
	// At least one side needs to be punchable
	if local == NATNone || remote == NATNone {
		return true
	}
	if local == NATFullCone || remote == NATFullCone {
		return true
	}
	// Both restricted cone → usually works
	if (local == NATRestrictedCone || local == NATPortRestricted) &&
		(remote == NATRestrictedCone || remote == NATPortRestricted) {
		return true
	}
	// Symmetric on both sides → very unlikely
	if local == NATSymmetric && remote == NATSymmetric {
		return false
	}
	// One symmetric + one restricted → sometimes works
	if local == NATSymmetric || remote == NATSymmetric {
		return false // conservative: recommend relay
	}
	return true
}
