package broodmind

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodMind v2 — Arbiter Engine (多Agent仲裁)
//
// When multiple agents propose competing actions on the same
// resource/topic, the Arbiter detects conflicts and resolves
// them via configurable strategies:
//
//   - Priority:   highest-trust / highest-score agent wins
//   - Voting:     agents vote, majority wins
//   - Consensus:  all must agree or escalate
//   - Delegate:   forward to a designated arbitrator agent
//
// Flow:  Proposal → Conflict Detection → Resolution → Record
// ════════════════════════════════════════════════════════════

// ── Types ──

// ArbiterStrategy defines how conflicts are resolved
type ArbiterStrategy string

const (
	StrategyPriority  ArbiterStrategy = "priority"
	StrategyVoting    ArbiterStrategy = "voting"
	StrategyConsensus ArbiterStrategy = "consensus"
	StrategyDelegate  ArbiterStrategy = "delegate"
)

// ProposalStatus tracks the lifecycle of a proposal
type ProposalStatus string

const (
	ProposalPending   ProposalStatus = "pending"
	ProposalAccepted  ProposalStatus = "accepted"
	ProposalRejected  ProposalStatus = "rejected"
	ProposalConflict  ProposalStatus = "conflict"
	ProposalEscalated ProposalStatus = "escalated"
	ProposalMerged    ProposalStatus = "merged"
)

// ConflictType classifies what kind of conflict was detected
type ConflictType string

const (
	ConflictResource    ConflictType = "resource"     // same resource targeted
	ConflictAction      ConflictType = "action"       // contradictory actions
	ConflictPriority    ConflictType = "priority"     // competing priorities
	ConflictConstraint  ConflictType = "constraint"   // violates org policy
)

// Proposal represents an agent's proposed action
type Proposal struct {
	ID          string         `json:"id"`
	AgentID     string         `json:"agent_id"`
	NodeID      string         `json:"node_id,omitempty"`
	Resource    string         `json:"resource"`
	Action      string         `json:"action"`
	Params      json.RawMessage `json:"params,omitempty"`
	Priority    int            `json:"priority"`
	Confidence  float64        `json:"confidence"`
	Reason      string         `json:"reason"`
	Status      ProposalStatus `json:"status"`
	ResolvedBy  string         `json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// Vote represents one agent's vote on a conflict
type Vote struct {
	AgentID    string    `json:"agent_id"`
	ProposalID string    `json:"proposal_id"`
	Approve    bool      `json:"approve"`
	Weight     float64   `json:"weight"`
	Reason     string    `json:"reason,omitempty"`
	VotedAt    time.Time `json:"voted_at"`
}

// Conflict represents a detected conflict between proposals
type Conflict struct {
	ID           string         `json:"id"`
	Type         ConflictType   `json:"type"`
	Resource     string         `json:"resource"`
	ProposalIDs  []string       `json:"proposal_ids"`
	Strategy     ArbiterStrategy `json:"strategy"`
	WinnerID     string         `json:"winner_id,omitempty"`
	Votes        []Vote         `json:"votes,omitempty"`
	Explanation  string         `json:"explanation"`
	Status       string         `json:"status"` // open, resolved, escalated
	DetectedAt   time.Time      `json:"detected_at"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
}

// ArbiterConfig holds configuration for the arbiter engine
type ArbiterConfig struct {
	DefaultStrategy   ArbiterStrategy        `json:"default_strategy"`
	ResourceStrategies map[string]ArbiterStrategy `json:"resource_strategies,omitempty"`
	VotingQuorum      float64                `json:"voting_quorum"`
	ConflictWindowSec int                    `json:"conflict_window_sec"`
	MaxProposals      int                    `json:"max_proposals"`
	MaxConflicts      int                    `json:"max_conflicts"`
	DelegateAgent     string                 `json:"delegate_agent,omitempty"`
	AgentWeights      map[string]float64     `json:"agent_weights,omitempty"`
}

// DefaultArbiterConfig returns sensible defaults
func DefaultArbiterConfig() *ArbiterConfig {
	return &ArbiterConfig{
		DefaultStrategy:    StrategyPriority,
		ResourceStrategies: make(map[string]ArbiterStrategy),
		VotingQuorum:       0.6,
		ConflictWindowSec:  30,
		MaxProposals:       2000,
		MaxConflicts:       500,
		AgentWeights:       make(map[string]float64),
	}
}

// ── Arbiter Engine ──

// Arbiter detects and resolves multi-agent conflicts
type Arbiter struct {
	mu        sync.RWMutex
	config    *ArbiterConfig
	proposals []*Proposal
	byID      map[string]*Proposal
	byRes     map[string][]*Proposal // resource → pending proposals
	conflicts []*Conflict
	conflByID map[string]*Conflict
	stats     ArbiterStats
}

// ArbiterStats tracks operational metrics
type ArbiterStats struct {
	TotalProposals   int            `json:"total_proposals"`
	TotalConflicts   int            `json:"total_conflicts"`
	ResolvedCount    int            `json:"resolved_count"`
	EscalatedCount   int            `json:"escalated_count"`
	ByStrategy       map[ArbiterStrategy]int `json:"by_strategy"`
	ByConflictType   map[ConflictType]int    `json:"by_conflict_type"`
	AvgResolutionMs  int64          `json:"avg_resolution_ms"`
	totalResMs       int64
}

// NewArbiter creates a new arbiter engine
func NewArbiter(cfg *ArbiterConfig) *Arbiter {
	if cfg == nil {
		cfg = DefaultArbiterConfig()
	}
	return &Arbiter{
		config:    cfg,
		proposals: make([]*Proposal, 0, cfg.MaxProposals),
		byID:      make(map[string]*Proposal),
		byRes:     make(map[string][]*Proposal),
		conflicts: make([]*Conflict, 0, cfg.MaxConflicts),
		conflByID: make(map[string]*Conflict),
		stats: ArbiterStats{
			ByStrategy:     make(map[ArbiterStrategy]int),
			ByConflictType: make(map[ConflictType]int),
		},
	}
}

// Propose submits a new proposal and checks for conflicts
// Returns the proposal and any detected conflict
func (a *Arbiter) Propose(agentID, nodeID, resource, action, reason string, params json.RawMessage, priority int, confidence float64) (*Proposal, *Conflict) {
	a.mu.Lock()
	defer a.mu.Unlock()

	p := &Proposal{
		ID:         "prop:" + uuid.New().String()[:8],
		AgentID:    agentID,
		NodeID:     nodeID,
		Resource:   resource,
		Action:     action,
		Params:     params,
		Priority:   priority,
		Confidence: confidence,
		Reason:     reason,
		Status:     ProposalPending,
		CreatedAt:  time.Now(),
	}

	a.proposals = append(a.proposals, p)
	a.byID[p.ID] = p
	a.byRes[resource] = append(a.byRes[resource], p)
	a.stats.TotalProposals++

	// Evict old proposals
	a.evictProposals()

	// Detect conflict
	conflict := a.detectConflict(p)
	if conflict != nil {
		a.conflicts = append(a.conflicts, conflict)
		a.conflByID[conflict.ID] = conflict
		a.stats.TotalConflicts++
		a.stats.ByConflictType[conflict.Type]++

		// Auto-resolve if strategy allows
		a.tryAutoResolve(conflict)

		// Evict old conflicts
		a.evictConflicts()
	}

	return p, conflict
}

// detectConflict checks if a new proposal conflicts with existing pending ones
func (a *Arbiter) detectConflict(p *Proposal) *Conflict {
	window := time.Duration(a.config.ConflictWindowSec) * time.Second
	cutoff := time.Now().Add(-window)

	var competing []*Proposal
	for _, existing := range a.byRes[p.Resource] {
		if existing.ID == p.ID {
			continue
		}
		if existing.Status != ProposalPending {
			continue
		}
		if existing.CreatedAt.Before(cutoff) {
			continue
		}
		competing = append(competing, existing)
	}

	if len(competing) == 0 {
		return nil
	}

	// Classify conflict type
	conflictType := classifyConflict(p, competing)

	// Determine strategy for this resource
	strategy := a.config.DefaultStrategy
	if rs, ok := a.config.ResourceStrategies[p.Resource]; ok {
		strategy = rs
	}

	ids := make([]string, 0, len(competing)+1)
	ids = append(ids, p.ID)
	for _, c := range competing {
		ids = append(ids, c.ID)
		c.Status = ProposalConflict
	}
	p.Status = ProposalConflict

	conflict := &Conflict{
		ID:          "conf:" + uuid.New().String()[:8],
		Type:        conflictType,
		Resource:    p.Resource,
		ProposalIDs: ids,
		Strategy:    strategy,
		Status:      "open",
		DetectedAt:  time.Now(),
		Explanation: fmt.Sprintf("%d agents competing for resource %q", len(ids), p.Resource),
	}

	log.Printf("[broodmind/arbiter] conflict detected: %s on resource %q (%d proposals, strategy=%s)",
		conflict.ID, p.Resource, len(ids), strategy)

	return conflict
}

// classifyConflict determines the conflict type
func classifyConflict(p *Proposal, competing []*Proposal) ConflictType {
	for _, c := range competing {
		// Same resource, contradictory actions → action conflict
		if c.Action != p.Action {
			return ConflictAction
		}
		// Same resource, same action but different priority → priority conflict
		if c.Priority != p.Priority {
			return ConflictPriority
		}
	}
	return ConflictResource
}

// tryAutoResolve attempts automatic resolution based on strategy
func (a *Arbiter) tryAutoResolve(conflict *Conflict) {
	switch conflict.Strategy {
	case StrategyPriority:
		a.resolvePriority(conflict)
	case StrategyVoting:
		// Voting requires explicit votes; cannot auto-resolve
		return
	case StrategyConsensus:
		// Consensus requires all parties; cannot auto-resolve
		return
	case StrategyDelegate:
		a.resolveDelegate(conflict)
	}
}

// resolvePriority picks the winner by priority, then confidence, then agent weight
func (a *Arbiter) resolvePriority(conflict *Conflict) {
	var best *Proposal
	bestScore := -1.0

	for _, pid := range conflict.ProposalIDs {
		p := a.byID[pid]
		if p == nil {
			continue
		}
		// Composite score: priority * 100 + confidence * 10 + agent weight
		weight := a.config.AgentWeights[p.AgentID]
		if weight <= 0 {
			weight = 1.0
		}
		score := float64(p.Priority)*100 + p.Confidence*10 + weight
		if score > bestScore {
			bestScore = score
			best = p
		}
	}

	if best == nil {
		return
	}

	now := time.Now()
	conflict.WinnerID = best.ID
	conflict.Status = "resolved"
	conflict.ResolvedAt = &now
	conflict.Explanation = fmt.Sprintf("priority winner: agent=%s score=%.1f", best.AgentID, bestScore)

	// Update proposal statuses
	for _, pid := range conflict.ProposalIDs {
		p := a.byID[pid]
		if p == nil {
			continue
		}
		if p.ID == best.ID {
			p.Status = ProposalAccepted
		} else {
			p.Status = ProposalRejected
		}
		p.ResolvedBy = "arbiter:priority"
		p.ResolvedAt = &now
	}

	a.stats.ResolvedCount++
	a.stats.ByStrategy[StrategyPriority]++
	resMs := now.Sub(conflict.DetectedAt).Milliseconds()
	a.stats.totalResMs += resMs
	a.stats.AvgResolutionMs = a.stats.totalResMs / int64(a.stats.ResolvedCount)

	log.Printf("[broodmind/arbiter] resolved %s via priority: winner=%s (%s)", conflict.ID, best.ID, best.AgentID)
}

// resolveDelegate escalates to a designated arbiter agent
func (a *Arbiter) resolveDelegate(conflict *Conflict) {
	if a.config.DelegateAgent == "" {
		conflict.Status = "escalated"
		a.stats.EscalatedCount++
		return
	}

	// Mark as escalated to delegate — actual resolution happens when delegate responds
	conflict.Status = "escalated"
	conflict.Explanation = fmt.Sprintf("escalated to delegate agent: %s", a.config.DelegateAgent)
	a.stats.EscalatedCount++

	log.Printf("[broodmind/arbiter] escalated %s to delegate: %s", conflict.ID, a.config.DelegateAgent)
}

// CastVote records a vote on a conflict (for voting/consensus strategies)
func (a *Arbiter) CastVote(conflictID, agentID, proposalID string, approve bool, reason string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	conflict, ok := a.conflByID[conflictID]
	if !ok {
		return fmt.Errorf("conflict %s not found", conflictID)
	}
	if conflict.Status != "open" && conflict.Status != "escalated" {
		return fmt.Errorf("conflict %s already resolved", conflictID)
	}

	// Determine vote weight
	weight := a.config.AgentWeights[agentID]
	if weight <= 0 {
		weight = 1.0
	}

	vote := Vote{
		AgentID:    agentID,
		ProposalID: proposalID,
		Approve:    approve,
		Weight:     weight,
		Reason:     reason,
		VotedAt:    time.Now(),
	}
	conflict.Votes = append(conflict.Votes, vote)

	// Check if quorum reached for voting strategy
	if conflict.Strategy == StrategyVoting {
		a.checkVotingQuorum(conflict)
	} else if conflict.Strategy == StrategyConsensus {
		a.checkConsensus(conflict)
	}

	return nil
}

// checkVotingQuorum resolves a conflict if enough votes are in
func (a *Arbiter) checkVotingQuorum(conflict *Conflict) {
	// Tally votes per proposal
	tally := make(map[string]float64) // proposalID → weighted votes
	totalWeight := 0.0

	for _, v := range conflict.Votes {
		totalWeight += v.Weight
		if v.Approve {
			tally[v.ProposalID] += v.Weight
		}
	}

	// Need quorum participation
	expectedVoters := float64(len(conflict.ProposalIDs))
	if totalWeight < expectedVoters*a.config.VotingQuorum {
		return // not enough votes yet
	}

	// Find proposal with highest weighted approval
	var winnerPID string
	maxVotes := 0.0
	for pid, w := range tally {
		if w > maxVotes {
			maxVotes = w
			winnerPID = pid
		}
	}

	if winnerPID == "" {
		return
	}

	now := time.Now()
	conflict.WinnerID = winnerPID
	conflict.Status = "resolved"
	conflict.ResolvedAt = &now
	conflict.Explanation = fmt.Sprintf("voting resolved: winner=%s votes=%.1f/%.1f", winnerPID, maxVotes, totalWeight)

	for _, pid := range conflict.ProposalIDs {
		p := a.byID[pid]
		if p == nil {
			continue
		}
		if p.ID == winnerPID {
			p.Status = ProposalAccepted
		} else {
			p.Status = ProposalRejected
		}
		p.ResolvedBy = "arbiter:voting"
		p.ResolvedAt = &now
	}

	a.stats.ResolvedCount++
	a.stats.ByStrategy[StrategyVoting]++
	resMs := now.Sub(conflict.DetectedAt).Milliseconds()
	a.stats.totalResMs += resMs
	a.stats.AvgResolutionMs = a.stats.totalResMs / int64(a.stats.ResolvedCount)

	log.Printf("[broodmind/arbiter] voting resolved %s: winner=%s", conflict.ID, winnerPID)
}

// checkConsensus resolves only if ALL voters agree on one proposal
func (a *Arbiter) checkConsensus(conflict *Conflict) {
	if len(conflict.Votes) < len(conflict.ProposalIDs) {
		return // need all parties to vote
	}

	// Check if all votes approve the same proposal
	approvals := make(map[string]int)
	for _, v := range conflict.Votes {
		if v.Approve {
			approvals[v.ProposalID]++
		}
	}

	totalVoters := len(conflict.Votes)
	now := time.Now()

	for pid, count := range approvals {
		if count == totalVoters {
			// Full consensus
			conflict.WinnerID = pid
			conflict.Status = "resolved"
			conflict.ResolvedAt = &now
			conflict.Explanation = fmt.Sprintf("consensus reached: %d/%d for %s", count, totalVoters, pid)

			for _, ppid := range conflict.ProposalIDs {
				p := a.byID[ppid]
				if p == nil {
					continue
				}
				if p.ID == pid {
					p.Status = ProposalAccepted
				} else {
					p.Status = ProposalRejected
				}
				p.ResolvedBy = "arbiter:consensus"
				p.ResolvedAt = &now
			}

			a.stats.ResolvedCount++
			a.stats.ByStrategy[StrategyConsensus]++
			resMs := now.Sub(conflict.DetectedAt).Milliseconds()
			a.stats.totalResMs += resMs
			a.stats.AvgResolutionMs = a.stats.totalResMs / int64(a.stats.ResolvedCount)

			log.Printf("[broodmind/arbiter] consensus resolved %s", conflict.ID)
			return
		}
	}

	// No consensus — escalate
	conflict.Status = "escalated"
	conflict.ResolvedAt = &now
	conflict.Explanation = "no consensus reached, escalating"
	a.stats.EscalatedCount++
	log.Printf("[broodmind/arbiter] consensus failed for %s, escalated", conflict.ID)
}

// ResolveManually allows an admin or delegate agent to force-resolve a conflict
func (a *Arbiter) ResolveManually(conflictID, winnerProposalID, resolvedBy, explanation string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	conflict, ok := a.conflByID[conflictID]
	if !ok {
		return fmt.Errorf("conflict %s not found", conflictID)
	}

	now := time.Now()
	conflict.WinnerID = winnerProposalID
	conflict.Status = "resolved"
	conflict.ResolvedAt = &now
	conflict.Explanation = explanation

	for _, pid := range conflict.ProposalIDs {
		p := a.byID[pid]
		if p == nil {
			continue
		}
		if p.ID == winnerProposalID {
			p.Status = ProposalAccepted
		} else {
			p.Status = ProposalRejected
		}
		p.ResolvedBy = resolvedBy
		p.ResolvedAt = &now
	}

	a.stats.ResolvedCount++
	resMs := now.Sub(conflict.DetectedAt).Milliseconds()
	a.stats.totalResMs += resMs
	if a.stats.ResolvedCount > 0 {
		a.stats.AvgResolutionMs = a.stats.totalResMs / int64(a.stats.ResolvedCount)
	}

	log.Printf("[broodmind/arbiter] manually resolved %s: winner=%s by=%s", conflictID, winnerProposalID, resolvedBy)
	return nil
}

// ── Query Methods ──

// GetProposal retrieves a proposal by ID
func (a *Arbiter) GetProposal(id string) *Proposal {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.byID[id]
}

// GetConflict retrieves a conflict by ID
func (a *Arbiter) GetConflict(id string) *Conflict {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.conflByID[id]
}

// ListProposals returns recent proposals filtered by status/resource
func (a *Arbiter) ListProposals(status ProposalStatus, resource string, limit int) []*Proposal {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*Proposal
	for i := len(a.proposals) - 1; i >= 0 && len(result) < limit; i-- {
		p := a.proposals[i]
		if status != "" && p.Status != status {
			continue
		}
		if resource != "" && p.Resource != resource {
			continue
		}
		result = append(result, p)
	}
	return result
}

// ListConflicts returns recent conflicts filtered by status
func (a *Arbiter) ListConflicts(status string, limit int) []*Conflict {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*Conflict
	for i := len(a.conflicts) - 1; i >= 0 && len(result) < limit; i-- {
		c := a.conflicts[i]
		if status != "" && c.Status != status {
			continue
		}
		result = append(result, c)
	}
	return result
}

// OpenConflicts returns unresolved conflicts
func (a *Arbiter) OpenConflicts() []*Conflict {
	return a.ListConflicts("open", 100)
}

// Stats returns arbiter metrics
func (a *Arbiter) Stats() *ArbiterStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.stats // copy
	return &s
}

// SetStrategy updates the strategy for a specific resource
func (a *Arbiter) SetStrategy(resource string, strategy ArbiterStrategy) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if resource == "" {
		a.config.DefaultStrategy = strategy
	} else {
		a.config.ResourceStrategies[resource] = strategy
	}
}

// SetAgentWeight sets the voting/priority weight for an agent
func (a *Arbiter) SetAgentWeight(agentID string, weight float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.AgentWeights[agentID] = math.Max(0.1, math.Min(10.0, weight))
}

// Config returns a copy of the current config
func (a *Arbiter) Config() ArbiterConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return *a.config
}

// ── Eviction ──

func (a *Arbiter) evictProposals() {
	for len(a.proposals) > a.config.MaxProposals {
		old := a.proposals[0]
		a.proposals = a.proposals[1:]
		delete(a.byID, old.ID)
		// Remove from resource index
		if rs, ok := a.byRes[old.Resource]; ok {
			for i, p := range rs {
				if p.ID == old.ID {
					a.byRes[old.Resource] = append(rs[:i], rs[i+1:]...)
					break
				}
			}
		}
	}
}

func (a *Arbiter) evictConflicts() {
	for len(a.conflicts) > a.config.MaxConflicts {
		old := a.conflicts[0]
		a.conflicts = a.conflicts[1:]
		delete(a.conflByID, old.ID)
	}
}
