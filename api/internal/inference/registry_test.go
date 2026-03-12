package inference

import (
	"testing"
)

func TestMinerRegistry_RegisterAndSelect(t *testing.T) {
	r := NewMinerRegistry()

	// No miners → nil
	if m := r.SelectMiner("gpt-4"); m != nil {
		t.Error("expected nil when no miners")
	}

	// Register a miner
	r.Register(&MinerInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"gpt-4", "gpt-3.5"},
		MaxJobs: 4,
	})

	// Should find it for gpt-4
	m := r.SelectMiner("gpt-4")
	if m == nil {
		t.Fatal("expected to find miner for gpt-4")
	}
	if m.NodeID != "claw:aaaa000000000000000000000000000000000000" {
		t.Errorf("unexpected node_id: %s", m.NodeID)
	}

	// Should not find for unknown model
	if r.SelectMiner("llama-99") != nil {
		t.Error("expected nil for unknown model")
	}
}

func TestMinerRegistry_SelectByLoad(t *testing.T) {
	r := NewMinerRegistry()

	r.Register(&MinerInfo{
		NodeID:     "claw:aaaa000000000000000000000000000000000000",
		Address:    "http://10.0.0.1:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    4,
		ActiveJobs: 3, // 75% load
	})
	r.Register(&MinerInfo{
		NodeID:     "claw:bbbb000000000000000000000000000000000000",
		Address:    "http://10.0.0.2:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    4,
		ActiveJobs: 1, // 25% load
	})

	m := r.SelectMiner("gpt-4")
	if m == nil {
		t.Fatal("expected a miner")
	}
	// Should pick the less loaded one
	if m.NodeID != "claw:bbbb000000000000000000000000000000000000" {
		t.Errorf("expected less-loaded miner, got %s", m.NodeID)
	}
}

func TestMinerRegistry_AtCapacity(t *testing.T) {
	r := NewMinerRegistry()

	r.Register(&MinerInfo{
		NodeID:     "claw:aaaa000000000000000000000000000000000000",
		Address:    "http://10.0.0.1:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    2,
		ActiveJobs: 2, // full
	})

	// At capacity → nil
	if r.SelectMiner("gpt-4") != nil {
		t.Error("expected nil when miner at capacity")
	}
}

func TestMinerRegistry_Heartbeat(t *testing.T) {
	r := NewMinerRegistry()

	// Heartbeat for unregistered miner
	if r.Heartbeat("claw:unknown", 0, 100) {
		t.Error("expected false for unknown miner")
	}

	r.Register(&MinerInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"gpt-4"},
		MaxJobs: 4,
	})

	if !r.Heartbeat("claw:aaaa000000000000000000000000000000000000", 2, 50) {
		t.Error("expected true for known miner")
	}
}

func TestMinerRegistry_WildcardModel(t *testing.T) {
	r := NewMinerRegistry()

	r.Register(&MinerInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"*"}, // accepts any model
		MaxJobs: 4,
	})

	if r.SelectMiner("any-model-name") == nil {
		t.Error("expected wildcard miner to match any model")
	}
}

func TestMinerRegistry_Stats(t *testing.T) {
	r := NewMinerRegistry()

	r.Register(&MinerInfo{
		NodeID:     "claw:aaaa000000000000000000000000000000000000",
		Address:    "http://10.0.0.1:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    4,
		ActiveJobs: 1,
	})

	stats := r.Stats()
	if stats["total_miners"].(int) != 1 {
		t.Errorf("expected 1 miner, got %v", stats["total_miners"])
	}
	if stats["online"].(int) != 1 {
		t.Errorf("expected 1 online, got %v", stats["online"])
	}
}

func TestMinerRegistry_Unregister(t *testing.T) {
	r := NewMinerRegistry()

	r.Register(&MinerInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"gpt-4"},
		MaxJobs: 4,
	})

	r.Unregister("claw:aaaa000000000000000000000000000000000000")

	if r.SelectMiner("gpt-4") != nil {
		t.Error("expected nil after unregister")
	}
}
