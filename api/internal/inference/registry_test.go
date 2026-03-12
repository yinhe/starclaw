package inference

import (
	"testing"
)

func TestContributorRegistry_RegisterAndSelect(t *testing.T) {
	r := NewContributorRegistry()

	// No miners → nil
	if m := r.SelectContributor("gpt-4"); m != nil {
		t.Error("expected nil when no contributors")
	}

	// Register a contributor
	r.Register(&ContributorInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"gpt-4", "gpt-3.5"},
		MaxJobs: 4,
	})

	// Should find it for gpt-4
	m := r.SelectContributor("gpt-4")
	if m == nil {
		t.Fatal("expected to find contributor for gpt-4")
	}
	if m.NodeID != "claw:aaaa000000000000000000000000000000000000" {
		t.Errorf("unexpected node_id: %s", m.NodeID)
	}

	// Should not find for unknown model
	if r.SelectContributor("llama-99") != nil {
		t.Error("expected nil for unknown model")
	}
}

func TestContributorRegistry_SelectByLoad(t *testing.T) {
	r := NewContributorRegistry()

	r.Register(&ContributorInfo{
		NodeID:     "claw:aaaa000000000000000000000000000000000000",
		Address:    "http://10.0.0.1:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    4,
		ActiveJobs: 3, // 75% load
	})
	r.Register(&ContributorInfo{
		NodeID:     "claw:bbbb000000000000000000000000000000000000",
		Address:    "http://10.0.0.2:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    4,
		ActiveJobs: 1, // 25% load
	})

	m := r.SelectContributor("gpt-4")
	if m == nil {
		t.Fatal("expected a contributor")
	}
	// Should pick the less loaded one
	if m.NodeID != "claw:bbbb000000000000000000000000000000000000" {
		t.Errorf("expected less-loaded contributor, got %s", m.NodeID)
	}
}

func TestContributorRegistry_AtCapacity(t *testing.T) {
	r := NewContributorRegistry()

	r.Register(&ContributorInfo{
		NodeID:     "claw:aaaa000000000000000000000000000000000000",
		Address:    "http://10.0.0.1:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    2,
		ActiveJobs: 2, // full
	})

	// At capacity → nil
	if r.SelectContributor("gpt-4") != nil {
		t.Error("expected nil when contributor at capacity")
	}
}

func TestContributorRegistry_Heartbeat(t *testing.T) {
	r := NewContributorRegistry()

	// Heartbeat for unregistered contributor
	if r.Heartbeat("claw:unknown", 0, 100) {
		t.Error("expected false for unknown contributor")
	}

	r.Register(&ContributorInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"gpt-4"},
		MaxJobs: 4,
	})

	if !r.Heartbeat("claw:aaaa000000000000000000000000000000000000", 2, 50) {
		t.Error("expected true for known contributor")
	}
}

func TestContributorRegistry_WildcardModel(t *testing.T) {
	r := NewContributorRegistry()

	r.Register(&ContributorInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"*"}, // accepts any model
		MaxJobs: 4,
	})

	if r.SelectContributor("any-model-name") == nil {
		t.Error("expected wildcard contributor to match any model")
	}
}

func TestContributorRegistry_Stats(t *testing.T) {
	r := NewContributorRegistry()

	r.Register(&ContributorInfo{
		NodeID:     "claw:aaaa000000000000000000000000000000000000",
		Address:    "http://10.0.0.1:8080",
		Models:     []string{"gpt-4"},
		MaxJobs:    4,
		ActiveJobs: 1,
	})

	stats := r.Stats()
	if stats["total_contributors"].(int) != 1 {
		t.Errorf("expected 1 contributor, got %v", stats["total_contributors"])
	}
	if stats["online"].(int) != 1 {
		t.Errorf("expected 1 online, got %v", stats["online"])
	}
}

func TestContributorRegistry_Unregister(t *testing.T) {
	r := NewContributorRegistry()

	r.Register(&ContributorInfo{
		NodeID:  "claw:aaaa000000000000000000000000000000000000",
		Address: "http://10.0.0.1:8080",
		Models:  []string{"gpt-4"},
		MaxJobs: 4,
	})

	r.Unregister("claw:aaaa000000000000000000000000000000000000")

	if r.SelectContributor("gpt-4") != nil {
		t.Error("expected nil after unregister")
	}
}
