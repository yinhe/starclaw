package inference

import (
	"math"
	"time"
)

// TrustLevel defines how much a contributor is trusted for routing decisions.
type TrustLevel string

const (
	TrustSelf     TrustLevel = "self"     // only route to own nodes
	TrustBrood    TrustLevel = "brood"    // enterprise-internal nodes only
	TrustVerified TrustLevel = "verified" // deposit + audit verified contributors
	TrustAny      TrustLevel = "any"      // any contributor (cheapest, least private)
)

// TrustScore tracks a contributor's reputation based on observed behavior.
// Scores range from 0.0 (untrusted) to 1.0 (fully trusted).
type TrustScore struct {
	// Core metrics (all 0.0–1.0)
	UptimeRatio      float64 `json:"uptime_ratio"`       // heartbeat consistency
	SuccessRate      float64 `json:"success_rate"`        // successful jobs / total jobs
	LatencyScore     float64 `json:"latency_score"`       // lower latency variance = higher score
	SpotCheckScore   float64 `json:"spot_check_score"`    // spot-check pass rate (1.0 if never checked)
	LongevityBonus   float64 `json:"longevity_bonus"`     // bonus for long-term reliable service

	// Raw counters
	TotalJobs        int64   `json:"total_jobs"`
	SuccessfulJobs   int64   `json:"successful_jobs"`
	FailedJobs       int64   `json:"failed_jobs"`
	SpotChecks       int64   `json:"spot_checks"`         // total spot-checks performed
	SpotChecksPassed int64   `json:"spot_checks_passed"`  // spot-checks that passed
	HeartbeatsRecv   int64   `json:"heartbeats_recv"`     // total heartbeats received
	HeartbeatsMissed int64   `json:"heartbeats_missed"`   // expected but missed heartbeats
	FirstSeen        int64   `json:"first_seen"`          // Unix timestamp of first registration

	// Composite
	Composite        float64 `json:"composite"`           // weighted composite score (0.0–1.0)
	Level            TrustLevel `json:"level"`             // derived trust level
}

// DefaultTrustScore returns the initial trust score for a new contributor.
func DefaultTrustScore() *TrustScore {
	return &TrustScore{
		UptimeRatio:    1.0, // benefit of the doubt
		SuccessRate:    1.0,
		LatencyScore:   0.5, // neutral until measured
		SpotCheckScore: 1.0, // clean until proven otherwise
		LongevityBonus: 0.0,
		FirstSeen:      time.Now().Unix(),
		Level:          TrustAny,
	}
}

// Weights for the composite trust score calculation.
const (
	weightUptime    = 0.20
	weightSuccess   = 0.30
	weightLatency   = 0.15
	weightSpotCheck = 0.25
	weightLongevity = 0.10
)

// Recalculate updates the composite score from individual metrics.
func (t *TrustScore) Recalculate() {
	// Uptime ratio
	totalHB := t.HeartbeatsRecv + t.HeartbeatsMissed
	if totalHB > 0 {
		t.UptimeRatio = float64(t.HeartbeatsRecv) / float64(totalHB)
	}

	// Success rate
	if t.TotalJobs > 0 {
		t.SuccessRate = float64(t.SuccessfulJobs) / float64(t.TotalJobs)
	}

	// Spot-check score
	if t.SpotChecks > 0 {
		t.SpotCheckScore = float64(t.SpotChecksPassed) / float64(t.SpotChecks)
	}

	// Longevity bonus: logarithmic growth, caps at 1.0 after ~90 days
	daysSinceFirst := float64(time.Now().Unix()-t.FirstSeen) / 86400.0
	if daysSinceFirst > 0 {
		t.LongevityBonus = math.Min(1.0, math.Log1p(daysSinceFirst/10.0)/math.Log1p(9.0))
	}

	// Weighted composite
	t.Composite = weightUptime*t.UptimeRatio +
		weightSuccess*t.SuccessRate +
		weightLatency*t.LatencyScore +
		weightSpotCheck*t.SpotCheckScore +
		weightLongevity*t.LongevityBonus

	// Clamp to [0, 1]
	t.Composite = math.Max(0.0, math.Min(1.0, t.Composite))

	// Derive trust level from composite score
	t.Level = deriveTrustLevel(t.Composite, t.SpotCheckScore, t.TotalJobs)
}

// deriveTrustLevel maps composite score + behavior to a trust level.
func deriveTrustLevel(composite, spotCheckScore float64, totalJobs int64) TrustLevel {
	// Spot-check failure instantly downgrades
	if spotCheckScore < 0.5 {
		return TrustAny
	}

	// Need minimum job count for higher trust levels
	if totalJobs < 10 {
		return TrustAny
	}

	if composite >= 0.85 && totalJobs >= 100 {
		return TrustVerified
	}
	if composite >= 0.70 {
		return TrustBrood
	}
	return TrustAny
}

// RecordSuccess records a successful job completion.
func (t *TrustScore) RecordSuccess(latencyMs int64) {
	t.TotalJobs++
	t.SuccessfulJobs++

	// Update latency score: penalize high latency, reward consistency
	// Score based on latency being under 5 seconds (ideal for inference)
	latencyFactor := math.Max(0.0, 1.0-float64(latencyMs)/5000.0)
	// Exponential moving average
	t.LatencyScore = t.LatencyScore*0.8 + latencyFactor*0.2

	t.Recalculate()
}

// RecordFailure records a failed job.
func (t *TrustScore) RecordFailure() {
	t.TotalJobs++
	t.FailedJobs++
	t.Recalculate()
}

// RecordHeartbeat records a received heartbeat.
func (t *TrustScore) RecordHeartbeat() {
	t.HeartbeatsRecv++
}

// RecordMissedHeartbeat records an expected but missed heartbeat.
func (t *TrustScore) RecordMissedHeartbeat() {
	t.HeartbeatsMissed++
	t.Recalculate()
}

// RecordSpotCheck records a spot-check result.
func (t *TrustScore) RecordSpotCheck(passed bool) {
	t.SpotChecks++
	if passed {
		t.SpotChecksPassed++
	}
	t.Recalculate()
}

// MeetsLevel returns true if this contributor meets the minimum trust requirement.
func (t *TrustScore) MeetsLevel(required TrustLevel) bool {
	return trustLevelRank(t.Level) >= trustLevelRank(required)
}

// trustLevelRank returns a numeric rank for comparison.
func trustLevelRank(level TrustLevel) int {
	switch level {
	case TrustSelf:
		return 3
	case TrustBrood:
		return 2
	case TrustVerified:
		return 1
	case TrustAny:
		return 0
	default:
		return 0
	}
}
