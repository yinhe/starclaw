package broodnet

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodOS Network — NodeReputation (节点信誉系统)
//
// Every Claw node accumulates reputation through its behavior:
//   - Task completion rate and quality
//   - Response speed (bid-to-start latency)
//   - Uptime stability (heartbeat consistency)
//   - Economic trustworthiness (settlement reliability)
//
// Reputation feeds into:
//   - TaskMarket bid scoring (higher rep → better scores)
//   - Orchestrator routing (prefer reputable nodes)
//   - Formation membership (captains can filter by rep)
//   - Star energy pricing (reputable nodes can charge more)
//
// Trust tiers: Newcomer → Reliable → Veteran → Elite → Legendary
// Score decays 2% per day without activity (prevents stale high scores)
// ════════════════════════════════════════════════════════════

// ── Trust Tiers ──

type TrustTier string

const (
	TierNewcomer  TrustTier = "newcomer"   // 0-199
	TierReliable  TrustTier = "reliable"   // 200-499
	TierVeteran   TrustTier = "veteran"    // 500-799
	TierElite     TrustTier = "elite"      // 800-949
	TierLegendary TrustTier = "legendary"  // 950+
)

func tierFromScore(score float64) TrustTier {
	switch {
	case score >= 950:
		return TierLegendary
	case score >= 800:
		return TierElite
	case score >= 500:
		return TierVeteran
	case score >= 200:
		return TierReliable
	default:
		return TierNewcomer
	}
}

// ── Reputation Event Types ──

type RepEventType string

const (
	RepTaskCompleted   RepEventType = "task_completed"
	RepTaskFailed      RepEventType = "task_failed"
	RepTaskTimeout     RepEventType = "task_timeout"
	RepBidAccepted     RepEventType = "bid_accepted"
	RepSettlementOK    RepEventType = "settlement_ok"
	RepSettlementFail  RepEventType = "settlement_fail"
	RepHeartbeatOK     RepEventType = "heartbeat_ok"
	RepHeartbeatMiss   RepEventType = "heartbeat_miss"
	RepFastResponse    RepEventType = "fast_response"   // responded < 5s
	RepSlowResponse    RepEventType = "slow_response"   // responded > 30s
	RepManualBoost     RepEventType = "manual_boost"    // admin/formation captain boost
	RepManualPenalty   RepEventType = "manual_penalty"  // admin penalty
)

// Point values for each event type
var repPointMap = map[RepEventType]float64{
	RepTaskCompleted:  +10.0,
	RepTaskFailed:     -15.0,
	RepTaskTimeout:    -20.0,
	RepBidAccepted:    +3.0,
	RepSettlementOK:   +5.0,
	RepSettlementFail: -25.0,
	RepHeartbeatOK:    +0.5,
	RepHeartbeatMiss:  -5.0,
	RepFastResponse:   +2.0,
	RepSlowResponse:   -1.0,
	RepManualBoost:    +50.0,
	RepManualPenalty:  -50.0,
}

// ── Data Types ──

// RepEvent records a single reputation-affecting event
type RepEvent struct {
	ID        string       `json:"id"`
	NodeID    string       `json:"node_id"`
	Type      RepEventType `json:"type"`
	Points    float64      `json:"points"`
	Context   string       `json:"context,omitempty"` // e.g. task ID, bid ID
	CreatedAt time.Time    `json:"created_at"`
}

// NodeProfile is the reputation profile for a single node
type NodeProfile struct {
	NodeID          string    `json:"node_id"`
	Score           float64   `json:"score"`
	Tier            TrustTier `json:"tier"`
	TasksCompleted  int       `json:"tasks_completed"`
	TasksFailed     int       `json:"tasks_failed"`
	TasksTimedOut   int       `json:"tasks_timed_out"`
	SettlementsOK   int       `json:"settlements_ok"`
	SettlementsFail int       `json:"settlements_fail"`
	BidsAccepted    int       `json:"bids_accepted"`
	HeartbeatsOK    int       `json:"heartbeats_ok"`
	HeartbeatsMiss  int       `json:"heartbeats_miss"`
	FastResponses   int       `json:"fast_responses"`
	SlowResponses   int       `json:"slow_responses"`
	CompletionRate  float64   `json:"completion_rate"`  // tasks_completed / (completed+failed+timeout)
	AvgResponseMs   int64     `json:"avg_response_ms"`
	LastActivity    time.Time `json:"last_activity"`
	LastDecay       time.Time `json:"last_decay"`
	CreatedAt       time.Time `json:"created_at"`
}

// ── Reputation Engine ──

// ReputationConfig holds engine settings
type ReputationConfig struct {
	MaxEvents      int     `json:"max_events"`       // per node
	MaxNodes       int     `json:"max_nodes"`
	DecayRate      float64 `json:"decay_rate"`        // daily decay (0.02 = 2%)
	MinScore       float64 `json:"min_score"`         // floor
	MaxScore       float64 `json:"max_score"`         // ceiling
	InitialScore   float64 `json:"initial_score"`     // new node starting score
	DecayGraceDays int     `json:"decay_grace_days"`  // days before decay kicks in
}

// DefaultReputationConfig returns production defaults
func DefaultReputationConfig() *ReputationConfig {
	return &ReputationConfig{
		MaxEvents:      200,
		MaxNodes:       2000,
		DecayRate:      0.02,
		MinScore:       0,
		MaxScore:       1000,
		InitialScore:   100,
		DecayGraceDays: 3,
	}
}

// ReputationEngine manages node reputation scoring
type ReputationEngine struct {
	mu       sync.RWMutex
	config   *ReputationConfig
	profiles map[string]*NodeProfile
	events   map[string][]*RepEvent // nodeID → events
	stats    RepStats
}

// RepStats tracks overall reputation metrics
type RepStats struct {
	TotalNodes     int                  `json:"total_nodes"`
	TotalEvents    int                  `json:"total_events"`
	AvgScore       float64              `json:"avg_score"`
	ByTier         map[TrustTier]int    `json:"by_tier"`
	ByEventType    map[RepEventType]int `json:"by_event_type"`
	DecaysApplied  int                  `json:"decays_applied"`
}

var (
	globalRep *ReputationEngine
	repOnce   sync.Once
)

// InitReputation creates the global reputation engine
func InitReputation(cfg *ReputationConfig) *ReputationEngine {
	if cfg == nil {
		cfg = DefaultReputationConfig()
	}
	repOnce.Do(func() {
		globalRep = &ReputationEngine{
			config:   cfg,
			profiles: make(map[string]*NodeProfile),
			events:   make(map[string][]*RepEvent),
			stats: RepStats{
				ByTier:      make(map[TrustTier]int),
				ByEventType: make(map[RepEventType]int),
			},
		}
		log.Printf("[broodnet/rep] reputation engine ready (decay=%.0f%%/day, grace=%dd)",
			cfg.DecayRate*100, cfg.DecayGraceDays)
	})
	return globalRep
}

// GetReputation returns the global engine
func GetReputation() *ReputationEngine {
	return globalRep
}

// ── Core Operations ──

// RecordEvent records a reputation event for a node
func (re *ReputationEngine) RecordEvent(nodeID string, eventType RepEventType, context string) (*RepEvent, error) {
	points, ok := repPointMap[eventType]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	// Ensure profile exists
	profile := re.ensureProfile(nodeID)

	// Create event
	evt := &RepEvent{
		ID:        "rep:" + uuid.New().String()[:8],
		NodeID:    nodeID,
		Type:      eventType,
		Points:    points,
		Context:   context,
		CreatedAt: time.Now(),
	}

	// Apply points
	profile.Score = math.Max(re.config.MinScore,
		math.Min(re.config.MaxScore, profile.Score+points))
	profile.Tier = tierFromScore(profile.Score)
	profile.LastActivity = time.Now()

	// Update counters
	switch eventType {
	case RepTaskCompleted:
		profile.TasksCompleted++
	case RepTaskFailed:
		profile.TasksFailed++
	case RepTaskTimeout:
		profile.TasksTimedOut++
	case RepBidAccepted:
		profile.BidsAccepted++
	case RepSettlementOK:
		profile.SettlementsOK++
	case RepSettlementFail:
		profile.SettlementsFail++
	case RepHeartbeatOK:
		profile.HeartbeatsOK++
	case RepHeartbeatMiss:
		profile.HeartbeatsMiss++
	case RepFastResponse:
		profile.FastResponses++
	case RepSlowResponse:
		profile.SlowResponses++
	}

	// Recompute completion rate
	total := profile.TasksCompleted + profile.TasksFailed + profile.TasksTimedOut
	if total > 0 {
		profile.CompletionRate = float64(profile.TasksCompleted) / float64(total)
	}

	// Store event (cap per node)
	re.events[nodeID] = append(re.events[nodeID], evt)
	if len(re.events[nodeID]) > re.config.MaxEvents {
		re.events[nodeID] = re.events[nodeID][1:]
	}

	re.stats.TotalEvents++
	re.stats.ByEventType[eventType]++

	return evt, nil
}

// ensureProfile creates a profile if it doesn't exist (caller must hold lock)
func (re *ReputationEngine) ensureProfile(nodeID string) *NodeProfile {
	profile, ok := re.profiles[nodeID]
	if !ok {
		now := time.Now()
		profile = &NodeProfile{
			NodeID:    nodeID,
			Score:     re.config.InitialScore,
			Tier:      tierFromScore(re.config.InitialScore),
			LastDecay: now,
			CreatedAt: now,
		}
		re.profiles[nodeID] = profile
		re.events[nodeID] = make([]*RepEvent, 0)
		re.stats.TotalNodes++
		re.stats.ByTier[profile.Tier]++
	}
	return profile
}

// ApplyDecay decays inactive nodes' scores (call periodically, e.g. once per hour)
func (re *ReputationEngine) ApplyDecay() int {
	re.mu.Lock()
	defer re.mu.Unlock()

	now := time.Now()
	decayed := 0

	for _, p := range re.profiles {
		daysSinceActivity := now.Sub(p.LastActivity).Hours() / 24
		if daysSinceActivity < float64(re.config.DecayGraceDays) {
			continue // still within grace period
		}

		daysSinceDecay := now.Sub(p.LastDecay).Hours() / 24
		if daysSinceDecay < 1.0 {
			continue // already decayed today
		}

		// Apply daily decay
		oldTier := p.Tier
		decayDays := int(daysSinceDecay)
		for i := 0; i < decayDays; i++ {
			p.Score = math.Max(re.config.MinScore, p.Score*(1-re.config.DecayRate))
		}
		p.Tier = tierFromScore(p.Score)
		p.LastDecay = now

		if oldTier != p.Tier {
			re.stats.ByTier[oldTier]--
			re.stats.ByTier[p.Tier]++
		}
		re.stats.DecaysApplied++
		decayed++
	}

	return decayed
}

// ── Query ──

// GetProfile returns a node's reputation profile
func (re *ReputationEngine) GetProfile(nodeID string) *NodeProfile {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.profiles[nodeID]
}

// GetScore returns a node's current score (0 if unknown)
func (re *ReputationEngine) GetScore(nodeID string) float64 {
	re.mu.RLock()
	defer re.mu.RUnlock()
	if p, ok := re.profiles[nodeID]; ok {
		return p.Score
	}
	return 0
}

// GetTier returns a node's current trust tier
func (re *ReputationEngine) GetTier(nodeID string) TrustTier {
	re.mu.RLock()
	defer re.mu.RUnlock()
	if p, ok := re.profiles[nodeID]; ok {
		return p.Tier
	}
	return TierNewcomer
}

// GetEvents returns recent events for a node
func (re *ReputationEngine) GetEvents(nodeID string, limit int) []*RepEvent {
	re.mu.RLock()
	defer re.mu.RUnlock()

	events := re.events[nodeID]
	if limit <= 0 || limit > len(events) {
		limit = len(events)
	}
	// Return most recent
	if limit == 0 {
		return nil
	}
	start := len(events) - limit
	result := make([]*RepEvent, limit)
	copy(result, events[start:])
	return result
}

// Leaderboard returns top N nodes by score
func (re *ReputationEngine) Leaderboard(limit int) []*NodeProfile {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	// Collect all profiles
	all := make([]*NodeProfile, 0, len(re.profiles))
	for _, p := range re.profiles {
		all = append(all, p)
	}

	// Sort by score descending (simple insertion sort, fine for ~2000 nodes)
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].Score > all[j-1].Score; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}

	if limit > len(all) {
		limit = len(all)
	}
	return all[:limit]
}

// ListProfiles returns all profiles filtered by tier
func (re *ReputationEngine) ListProfiles(tier TrustTier, limit int) []*NodeProfile {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*NodeProfile
	for _, p := range re.profiles {
		if tier != "" && p.Tier != tier {
			continue
		}
		result = append(result, p)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// Stats returns engine-wide metrics
func (re *ReputationEngine) Stats() *RepStats {
	re.mu.RLock()
	defer re.mu.RUnlock()

	s := re.stats
	// Compute average score
	if len(re.profiles) > 0 {
		total := 0.0
		for _, p := range re.profiles {
			total += p.Score
		}
		s.AvgScore = total / float64(len(re.profiles))
	}
	return &s
}

// Config returns current config
func (re *ReputationEngine) RepConfig() ReputationConfig {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return *re.config
}
