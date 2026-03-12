package node

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/pion/stun/v3"
)

// NATType describes the type of NAT this node is behind.
type NATType string

const (
	NATNone            NATType = "none"             // public IP, no NAT
	NATFullCone        NATType = "full_cone"        // easiest to punch through
	NATRestrictedCone  NATType = "restricted_cone"  // need to send first
	NATPortRestricted  NATType = "port_restricted"  // need to send to exact port
	NATSymmetric       NATType = "symmetric"        // hardest, different port each time
	NATUnknown         NATType = "unknown"
)

// Default public STUN servers (used if no custom server configured).
var DefaultSTUNServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
	"stun.cloudflare.com:3478",
	"stun.stunprotocol.org:3478",
}

// STUNResult holds the result of a STUN probe.
type STUNResult struct {
	PublicIP   net.IP  `json:"public_ip"`
	PublicPort int     `json:"public_port"`
	NATType    NATType `json:"nat_type"`
	LocalIP    net.IP  `json:"local_ip"`
	LocalPort  int     `json:"local_port"`
	ProbeTime  int64   `json:"probe_time"` // Unix timestamp
}

// STUNProber discovers public IP and NAT type via STUN.
type STUNProber struct {
	servers []string
	mu      sync.RWMutex
	last    *STUNResult
}

// NewSTUNProber creates a prober with the given STUN servers.
// If none provided, uses defaults.
func NewSTUNProber(servers ...string) *STUNProber {
	if len(servers) == 0 {
		servers = DefaultSTUNServers
	}
	return &STUNProber{servers: servers}
}

// Probe performs a STUN binding request to discover public IP and port.
func (p *STUNProber) Probe() (*STUNResult, error) {
	var lastErr error

	for _, server := range p.servers {
		result, err := p.probeServer(server)
		if err != nil {
			lastErr = err
			continue
		}

		p.mu.Lock()
		p.last = result
		p.mu.Unlock()

		log.Printf("[nydus/stun] discovered public endpoint %s:%d via %s (NAT: %s)",
			result.PublicIP, result.PublicPort, server, result.NATType)

		return result, nil
	}

	return nil, fmt.Errorf("all STUN servers failed, last error: %v", lastErr)
}

// probeServer sends a STUN binding request to a single server.
func (p *STUNProber) probeServer(server string) (*STUNResult, error) {
	// Open a UDP socket
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// Resolve STUN server
	serverAddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", server, err)
	}

	// Build STUN Binding Request
	msg, err := stun.Build(stun.TransactionID, stun.BindingRequest)
	if err != nil {
		return nil, fmt.Errorf("build STUN message: %w", err)
	}

	// Send
	if _, err := conn.WriteTo(msg.Raw, serverAddr); err != nil {
		return nil, fmt.Errorf("send to %s: %w", server, err)
	}

	// Read response (timeout 3s)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1500)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		return nil, fmt.Errorf("read from %s: %w", server, err)
	}

	// Parse STUN response
	resp := new(stun.Message)
	resp.Raw = buf[:n]
	if err := resp.Decode(); err != nil {
		return nil, fmt.Errorf("decode STUN response: %w", err)
	}

	// Extract XOR-MAPPED-ADDRESS
	var xorAddr stun.XORMappedAddress
	if err := xorAddr.GetFrom(resp); err != nil {
		// Try MAPPED-ADDRESS as fallback
		var mappedAddr stun.MappedAddress
		if err2 := mappedAddr.GetFrom(resp); err2 != nil {
			return nil, fmt.Errorf("no mapped address in response: %v / %v", err, err2)
		}
		xorAddr.IP = mappedAddr.IP
		xorAddr.Port = mappedAddr.Port
	}

	// Determine NAT type (simplified: compare local vs public)
	natType := p.classifyNAT(localAddr, &net.UDPAddr{IP: xorAddr.IP, Port: xorAddr.Port})

	return &STUNResult{
		PublicIP:   xorAddr.IP,
		PublicPort: xorAddr.Port,
		NATType:    natType,
		LocalIP:    localAddr.IP,
		LocalPort:  localAddr.Port,
		ProbeTime:  time.Now().Unix(),
	}, nil
}

// classifyNAT provides a basic NAT classification.
// Full RFC 3489 classification requires multiple STUN servers; this is a practical simplification.
func (p *STUNProber) classifyNAT(local, public *net.UDPAddr) NATType {
	// If public IP matches a local interface IP → no NAT
	if isLocalIP(public.IP) {
		return NATNone
	}

	// If ports match → likely Full Cone or no NAT
	if local.Port == public.Port {
		return NATFullCone
	}

	// Port changed → at least Port Restricted
	// To distinguish Restricted Cone vs Symmetric, we'd need a second probe
	// from a different STUN server. For now, assume Port Restricted (most common).
	return NATPortRestricted
}

// ProbeNATType performs two STUN probes to different servers to detect Symmetric NAT.
// If the mapped port differs between servers → Symmetric NAT.
func (p *STUNProber) ProbeNATType() (*STUNResult, error) {
	if len(p.servers) < 2 {
		return p.Probe()
	}

	// Open a single UDP socket and probe two different STUN servers
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	var results [2]*net.UDPAddr

	for i := 0; i < 2 && i < len(p.servers); i++ {
		addr, err := p.probeWith(conn, p.servers[i])
		if err != nil {
			// If second probe fails, use first result with simple classification
			if i == 1 && results[0] != nil {
				break
			}
			return nil, fmt.Errorf("probe %s failed: %w", p.servers[i], err)
		}
		results[i] = addr
	}

	if results[0] == nil {
		return nil, fmt.Errorf("no STUN response received")
	}

	natType := NATPortRestricted
	if isLocalIP(results[0].IP) {
		natType = NATNone
	} else if localAddr.Port == results[0].Port {
		natType = NATFullCone
	} else if results[1] != nil && results[0].Port != results[1].Port {
		natType = NATSymmetric
	}

	result := &STUNResult{
		PublicIP:   results[0].IP,
		PublicPort: results[0].Port,
		NATType:    natType,
		LocalIP:    localAddr.IP,
		LocalPort:  localAddr.Port,
		ProbeTime:  time.Now().Unix(),
	}

	p.mu.Lock()
	p.last = result
	p.mu.Unlock()

	log.Printf("[nydus/stun] NAT type: %s, public=%s:%d local=:%d",
		natType, result.PublicIP, result.PublicPort, result.LocalPort)

	return result, nil
}

// probeWith sends a STUN binding request on an existing connection.
func (p *STUNProber) probeWith(conn net.PacketConn, server string) (*net.UDPAddr, error) {
	serverAddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return nil, err
	}

	msg, err := stun.Build(stun.TransactionID, stun.BindingRequest)
	if err != nil {
		return nil, err
	}

	if _, err := conn.WriteTo(msg.Raw, serverAddr); err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1500)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		return nil, err
	}

	resp := new(stun.Message)
	resp.Raw = buf[:n]
	if err := resp.Decode(); err != nil {
		return nil, err
	}

	var xorAddr stun.XORMappedAddress
	if err := xorAddr.GetFrom(resp); err != nil {
		var mappedAddr stun.MappedAddress
		if err2 := mappedAddr.GetFrom(resp); err2 != nil {
			return nil, fmt.Errorf("no address: %v / %v", err, err2)
		}
		return &net.UDPAddr{IP: mappedAddr.IP, Port: mappedAddr.Port}, nil
	}

	return &net.UDPAddr{IP: xorAddr.IP, Port: xorAddr.Port}, nil
}

// LastResult returns the most recent STUN probe result.
func (p *STUNProber) LastResult() *STUNResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.last
}

// isLocalIP checks if an IP is a local/private address.
func isLocalIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Check private ranges
	privateRanges := []struct{ start, end net.IP }{
		{net.ParseIP("10.0.0.0"), net.ParseIP("10.255.255.255")},
		{net.ParseIP("172.16.0.0"), net.ParseIP("172.31.255.255")},
		{net.ParseIP("192.168.0.0"), net.ParseIP("192.168.255.255")},
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	for _, r := range privateRanges {
		if bytesInRange(ip4, r.start.To4(), r.end.To4()) {
			return true
		}
	}
	return false
}

func bytesInRange(ip, start, end net.IP) bool {
	for i := range ip {
		if ip[i] < start[i] {
			return false
		}
		if ip[i] > end[i] {
			return false
		}
	}
	return true
}
