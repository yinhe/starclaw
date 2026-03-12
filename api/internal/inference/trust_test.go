package inference

import (
	"testing"
)

func TestTrustScore_DefaultComposite(t *testing.T) {
	ts := DefaultTrustScore()
	ts.Recalculate()

	// Default should have a reasonable composite score
	if ts.Composite < 0.3 || ts.Composite > 1.0 {
		t.Errorf("default composite = %f, want 0.3–1.0", ts.Composite)
	}
	if ts.Level != TrustAny {
		t.Errorf("default level = %s, want %s", ts.Level, TrustAny)
	}
}

func TestTrustScore_RecordSuccess(t *testing.T) {
	ts := DefaultTrustScore()
	for i := 0; i < 50; i++ {
		ts.RecordSuccess(200) // 200ms latency
	}

	if ts.TotalJobs != 50 {
		t.Errorf("total_jobs = %d, want 50", ts.TotalJobs)
	}
	if ts.SuccessfulJobs != 50 {
		t.Errorf("successful_jobs = %d, want 50", ts.SuccessfulJobs)
	}
	if ts.SuccessRate < 0.99 {
		t.Errorf("success_rate = %f, want >= 0.99", ts.SuccessRate)
	}
	if ts.LatencyScore < 0.5 {
		t.Errorf("latency_score = %f, want >= 0.5 (200ms is fast)", ts.LatencyScore)
	}
}

func TestTrustScore_RecordFailure(t *testing.T) {
	ts := DefaultTrustScore()
	for i := 0; i < 8; i++ {
		ts.RecordSuccess(500)
	}
	for i := 0; i < 2; i++ {
		ts.RecordFailure()
	}

	if ts.TotalJobs != 10 {
		t.Errorf("total_jobs = %d, want 10", ts.TotalJobs)
	}
	if ts.SuccessRate != 0.8 {
		t.Errorf("success_rate = %f, want 0.8", ts.SuccessRate)
	}
}

func TestTrustScore_SpotCheckImpact(t *testing.T) {
	ts := DefaultTrustScore()

	// Pass a few spot checks
	for i := 0; i < 5; i++ {
		ts.RecordSpotCheck(true)
	}
	if ts.SpotCheckScore < 0.99 {
		t.Errorf("spot_check_score after 5 passes = %f, want ~1.0", ts.SpotCheckScore)
	}

	scoreBefore := ts.Composite

	// Fail a spot check
	ts.RecordSpotCheck(false)
	if ts.SpotCheckScore >= 1.0 {
		t.Errorf("spot_check_score should decrease after failure")
	}
	if ts.Composite >= scoreBefore {
		t.Errorf("composite should decrease after spot-check failure")
	}
}

func TestTrustScore_MeetsLevel(t *testing.T) {
	ts := DefaultTrustScore()

	// New contributor with TrustAny should meet TrustAny
	if !ts.MeetsLevel(TrustAny) {
		t.Error("TrustAny contributor should meet TrustAny requirement")
	}

	// Should not meet TrustVerified without enough jobs
	if ts.MeetsLevel(TrustVerified) {
		t.Error("new contributor should not meet TrustVerified")
	}
}

func TestTrustScore_PromotionToVerified(t *testing.T) {
	ts := DefaultTrustScore()

	// Simulate a reliable contributor with 100+ jobs
	for i := 0; i < 120; i++ {
		ts.RecordSuccess(300)
		ts.RecordHeartbeat()
	}
	// Pass some spot-checks
	for i := 0; i < 10; i++ {
		ts.RecordSpotCheck(true)
	}

	ts.Recalculate()

	if ts.Composite < 0.85 {
		t.Errorf("reliable contributor composite = %f, want >= 0.85", ts.Composite)
	}
	if ts.Level != TrustVerified {
		t.Errorf("reliable contributor level = %s, want %s", ts.Level, TrustVerified)
	}
	if !ts.MeetsLevel(TrustVerified) {
		t.Error("should meet TrustVerified after 100+ successful jobs")
	}
}

func TestTrustScore_DemotionOnSpotCheckFail(t *testing.T) {
	ts := DefaultTrustScore()

	// Build up trust
	for i := 0; i < 120; i++ {
		ts.RecordSuccess(300)
		ts.RecordHeartbeat()
	}
	for i := 0; i < 10; i++ {
		ts.RecordSpotCheck(true)
	}
	ts.Recalculate()

	// Massive spot-check failures should demote
	for i := 0; i < 15; i++ {
		ts.RecordSpotCheck(false)
	}

	if ts.SpotCheckScore >= 0.5 {
		t.Errorf("spot_check_score = %f, want < 0.5 after many failures", ts.SpotCheckScore)
	}
	if ts.Level != TrustAny {
		t.Errorf("level = %s, want %s after spot-check failures", ts.Level, TrustAny)
	}
}

func TestSelectContributorWithTrust(t *testing.T) {
	reg := NewContributorRegistry()

	// Register a low-trust contributor
	low := &ContributorInfo{
		NodeID: "claw:low_trust_node0", Address: "http://10.0.0.1:8080",
		Models: []string{"gpt-4"}, MaxJobs: 4,
		Trust: DefaultTrustScore(),
	}
	low.Trust.Composite = 0.3
	low.Trust.Level = TrustAny
	reg.Register(low)

	// Register a high-trust contributor
	high := &ContributorInfo{
		NodeID: "claw:high_trust_nod0", Address: "http://10.0.0.2:8080",
		Models: []string{"gpt-4"}, MaxJobs: 4,
		Trust: DefaultTrustScore(),
	}
	high.Trust.Composite = 0.95
	high.Trust.Level = TrustVerified
	reg.Register(high)

	// With TrustAny, should prefer high-trust (higher composite score)
	selected := reg.SelectContributor("gpt-4")
	if selected == nil {
		t.Fatal("expected a contributor to be selected")
	}
	if selected.NodeID != "claw:high_trust_nod0" {
		t.Errorf("expected high-trust node, got %s", selected.NodeID)
	}

	// With TrustVerified, should only select the verified one
	selected = reg.SelectContributorWithTrust("gpt-4", TrustVerified)
	if selected == nil {
		t.Fatal("expected a verified contributor to be selected")
	}
	if selected.NodeID != "claw:high_trust_nod0" {
		t.Errorf("expected verified node, got %s", selected.NodeID)
	}
}

func TestTextSimilarity(t *testing.T) {
	tests := []struct {
		a, b   string
		minSim float64
		maxSim float64
	}{
		{"", "", 1.0, 1.0},
		{"hello world", "", 0.0, 0.0},
		{"hello world", "hello world", 1.0, 1.0},
		{"the quick brown fox", "the slow brown fox", 0.5, 0.9},
		{"completely different", "nothing in common here", 0.0, 0.15},
	}

	for _, tt := range tests {
		sim := textSimilarity(tt.a, tt.b)
		if sim < tt.minSim || sim > tt.maxSim {
			t.Errorf("textSimilarity(%q, %q) = %f, want [%f, %f]",
				tt.a, tt.b, sim, tt.minSim, tt.maxSim)
		}
	}
}
