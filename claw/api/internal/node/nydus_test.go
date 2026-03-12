package node

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestIsLocalIP(t *testing.T) {
	tests := []struct {
		ip    string
		local bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.5", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"127.0.0.1", true},
		{"8.8.8.8", false},
		{"1.2.3.4", false},
		{"172.32.0.1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		got := isLocalIP(ip)
		if got != tt.local {
			t.Errorf("isLocalIP(%s) = %v, want %v", tt.ip, got, tt.local)
		}
	}
}

func TestBytesInRange(t *testing.T) {
	ip := net.ParseIP("192.168.1.100").To4()
	start := net.ParseIP("192.168.0.0").To4()
	end := net.ParseIP("192.168.255.255").To4()

	if !bytesInRange(ip, start, end) {
		t.Error("192.168.1.100 should be in 192.168.0.0-192.168.255.255")
	}

	outside := net.ParseIP("10.0.0.1").To4()
	if bytesInRange(outside, start, end) {
		t.Error("10.0.0.1 should not be in 192.168.0.0-192.168.255.255")
	}
}

func TestSTUNProber_ClassifyNAT(t *testing.T) {
	prober := NewSTUNProber()

	// Same port, public IP → Full Cone
	local := &net.UDPAddr{IP: net.ParseIP("192.168.1.5"), Port: 12345}
	public := &net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 12345}
	if got := prober.classifyNAT(local, public); got != NATFullCone {
		t.Errorf("same port different IP = %s, want full_cone", got)
	}

	// Different port → Port Restricted
	public2 := &net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 54321}
	if got := prober.classifyNAT(local, public2); got != NATPortRestricted {
		t.Errorf("different port = %s, want port_restricted", got)
	}

	// Local IP as public → No NAT
	localPublic := &net.UDPAddr{IP: net.ParseIP("192.168.1.5"), Port: 12345}
	if got := prober.classifyNAT(local, localPublic); got != NATNone {
		t.Errorf("local IP = %s, want none", got)
	}
}

func TestCanPunch(t *testing.T) {
	tests := []struct {
		local, remote NATType
		expected      bool
	}{
		{NATNone, NATNone, true},
		{NATNone, NATSymmetric, true},
		{NATFullCone, NATPortRestricted, true},
		{NATPortRestricted, NATPortRestricted, true},
		{NATRestrictedCone, NATRestrictedCone, true},
		{NATSymmetric, NATSymmetric, false},
		{NATSymmetric, NATPortRestricted, false},
	}

	for _, tt := range tests {
		got := CanPunch(tt.local, tt.remote)
		if got != tt.expected {
			t.Errorf("CanPunch(%s, %s) = %v, want %v", tt.local, tt.remote, got, tt.expected)
		}
	}
}

func TestPunchProbe_BuildAndVerify(t *testing.T) {
	id := LoadOrCreateIdentity()
	prober := NewSTUNProber()
	puncher := NewHolePuncher(id, prober)

	nonce := generateNonce()
	probe := puncher.buildProbe(nonce)

	// Should be valid JSON
	var p punchProbe
	if err := json.Unmarshal(probe, &p); err != nil {
		t.Fatalf("probe is not valid JSON: %v", err)
	}

	if p.Type != "nydus_punch" {
		t.Errorf("probe type = %s, want nydus_punch", p.Type)
	}
	if p.NodeID != id.NodeID {
		t.Errorf("probe nodeID = %s, want %s", p.NodeID, id.NodeID)
	}
	if p.Nonce != nonce {
		t.Errorf("probe nonce mismatch")
	}

	// Verify should fail for own probes (self-detection)
	if puncher.verifyProbe(probe, nonce) {
		t.Error("verifyProbe should reject own probes")
	}

	// Simulate a remote probe
	remote := punchProbe{
		Type:   "nydus_punch",
		NodeID: "claw:remote_node_0000",
		Nonce:  nonce,
		Ts:     time.Now().UnixMilli(),
	}
	remoteData, _ := json.Marshal(remote)
	if !puncher.verifyProbe(remoteData, nonce) {
		t.Error("verifyProbe should accept valid remote probe")
	}

	// Wrong nonce
	if puncher.verifyProbe(remoteData, "wrong_nonce") {
		t.Error("verifyProbe should reject wrong nonce")
	}

	// Expired probe
	expired := punchProbe{
		Type:   "nydus_punch",
		NodeID: "claw:remote_node_0000",
		Nonce:  nonce,
		Ts:     time.Now().Add(-60 * time.Second).UnixMilli(),
	}
	expiredData, _ := json.Marshal(expired)
	if puncher.verifyProbe(expiredData, nonce) {
		t.Error("verifyProbe should reject expired probe")
	}
}

func TestRelayHandler_EnqueueDequeue(t *testing.T) {
	rh := NewRelayHandler()

	msg1 := &RelayMessage{
		FromNodeID: "claw:aaa",
		ToNodeID:   "claw:bbb",
		Type:       "punch_request",
		Timestamp:  time.Now().Unix(),
	}
	msg2 := &RelayMessage{
		FromNodeID: "claw:ccc",
		ToNodeID:   "claw:bbb",
		Type:       "data",
		Timestamp:  time.Now().Unix(),
	}

	rh.Enqueue(msg1)
	rh.Enqueue(msg2)

	// Dequeue for bbb should return both
	msgs := rh.Dequeue("claw:bbb")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	// Second dequeue should be empty
	msgs2 := rh.Dequeue("claw:bbb")
	if len(msgs2) != 0 {
		t.Fatalf("expected 0 messages after dequeue, got %d", len(msgs2))
	}
}

func TestRelayHandler_MaxPerNode(t *testing.T) {
	rh := NewRelayHandler()
	rh.maxPerNode = 3

	for i := 0; i < 5; i++ {
		rh.Enqueue(&RelayMessage{
			FromNodeID: "claw:sender",
			ToNodeID:   "claw:target",
			Type:       "data",
			Timestamp:  time.Now().Unix(),
		})
	}

	msgs := rh.Dequeue("claw:target")
	if len(msgs) > 3 {
		t.Errorf("expected max 3 messages, got %d", len(msgs))
	}
}

func TestRelayHandler_Stats(t *testing.T) {
	rh := NewRelayHandler()

	rh.Enqueue(&RelayMessage{ToNodeID: "claw:a", Timestamp: time.Now().Unix()})
	rh.Enqueue(&RelayMessage{ToNodeID: "claw:a", Timestamp: time.Now().Unix()})
	rh.Enqueue(&RelayMessage{ToNodeID: "claw:b", Timestamp: time.Now().Unix()})

	stats := rh.Stats()
	if stats["pending_nodes"].(int) != 2 {
		t.Errorf("expected 2 pending nodes, got %v", stats["pending_nodes"])
	}
	if stats["pending_messages"].(int) != 3 {
		t.Errorf("expected 3 pending messages, got %v", stats["pending_messages"])
	}
}

func TestNydusManager_Stats(t *testing.T) {
	id := LoadOrCreateIdentity()
	nm := NewNydusManager(id, NydusConfig{})

	stats := nm.Stats()
	conns := stats["connections"].(map[string]int)
	if conns["total"] != 0 {
		t.Errorf("expected 0 connections, got %d", conns["total"])
	}
	if stats["nat_type"].(NATType) != NATUnknown {
		t.Errorf("expected unknown NAT type before probe")
	}
}

func TestNydusManager_PublicEndpoint(t *testing.T) {
	id := LoadOrCreateIdentity()
	nm := NewNydusManager(id, NydusConfig{})

	// Before STUN probe, should be empty
	if ep := nm.PublicEndpoint(); ep != "" {
		t.Errorf("expected empty endpoint before probe, got %s", ep)
	}

	// Simulate a STUN result
	nm.mu.Lock()
	nm.stunResult = &STUNResult{
		PublicIP:   net.ParseIP("1.2.3.4"),
		PublicPort: 12345,
		NATType:    NATFullCone,
	}
	nm.mu.Unlock()

	if ep := nm.PublicEndpoint(); ep != "1.2.3.4:12345" {
		t.Errorf("expected 1.2.3.4:12345, got %s", ep)
	}
	if nt := nm.GetNATType(); nt != NATFullCone {
		t.Errorf("expected full_cone, got %s", nt)
	}
}

func TestGenerateNonce(t *testing.T) {
	n1 := generateNonce()
	n2 := generateNonce()

	if len(n1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("nonce length = %d, want 32", len(n1))
	}
	if n1 == n2 {
		t.Error("two nonces should not be equal")
	}
}

func TestNATTypeStrings(t *testing.T) {
	tests := []struct {
		nat NATType
		str string
	}{
		{NATNone, "none"},
		{NATFullCone, "full_cone"},
		{NATRestrictedCone, "restricted_cone"},
		{NATPortRestricted, "port_restricted"},
		{NATSymmetric, "symmetric"},
		{NATUnknown, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.nat) != tt.str {
			t.Errorf("NATType %v string = %s, want %s", tt.nat, string(tt.nat), tt.str)
		}
	}
}
