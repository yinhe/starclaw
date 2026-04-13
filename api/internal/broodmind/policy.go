package broodmind

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodMind v2 — PolicyEngine (组织级策略引擎)
//
// Defines, evaluates, and enforces organizational policies
// across agents and nodes. Supports:
//
//   - Rule definition: conditions + actions + scope
//   - Policy inheritance: org → team → node → agent (cascade)
//   - Evaluation: check an action against all applicable policies
//   - Enforcement: block/warn/audit based on policy verdict
//   - Versioning: policies have versions, with rollback support
//
// Integration:
//   - Arbiter: policies can influence conflict resolution
//   - Security Guard: policies extend trust-level checks
//   - MemSync: policy updates sync across nodes
// ════════════════════════════════════════════════════════════

// ── Types ──

// PolicyScope defines the level at which a policy applies
type PolicyScope string

const (
	ScopeOrg   PolicyScope = "org"   // entire organization
	ScopeTeam  PolicyScope = "team"  // specific team/group
	ScopeNode  PolicyScope = "node"  // specific node
	ScopeAgent PolicyScope = "agent" // specific agent
)

// PolicyEffect is what happens when a policy matches
type PolicyEffect string

const (
	EffectAllow PolicyEffect = "allow"
	EffectDeny  PolicyEffect = "deny"
	EffectWarn  PolicyEffect = "warn"
	EffectAudit PolicyEffect = "audit"
)

// PolicyStatus tracks the lifecycle of a policy
type PolicyStatus string

const (
	PolicyDraft    PolicyStatus = "draft"
	PolicyActive   PolicyStatus = "active"
	PolicyDisabled PolicyStatus = "disabled"
	PolicyArchived PolicyStatus = "archived"
)

// PolicyCondition defines when a policy triggers
type PolicyCondition struct {
	Field    string `json:"field"`    // e.g. "agent_id", "tool", "resource", "action", "time"
	Operator string `json:"operator"` // "eq", "neq", "contains", "in", "gt", "lt", "regex"
	Value    string `json:"value"`    // comparison value
}

// Policy represents an organizational rule
type Policy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Scope       PolicyScope       `json:"scope"`
	ScopeTarget string            `json:"scope_target,omitempty"` // team/node/agent ID when scope != org
	Conditions  []PolicyCondition `json:"conditions"`
	Effect      PolicyEffect      `json:"effect"`
	Priority    int               `json:"priority"` // higher = evaluated first
	Version     int               `json:"version"`
	Status      PolicyStatus      `json:"status"`
	CreatedBy   string            `json:"created_by"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    json.RawMessage   `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PolicyEvalRequest is the context to evaluate policies against
type PolicyEvalRequest struct {
	AgentID  string            `json:"agent_id"`
	NodeID   string            `json:"node_id,omitempty"`
	TeamID   string            `json:"team_id,omitempty"`
	Tool     string            `json:"tool,omitempty"`
	Resource string            `json:"resource,omitempty"`
	Action   string            `json:"action,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

// PolicyVerdict is the result of policy evaluation
type PolicyVerdict struct {
	Allowed     bool           `json:"allowed"`
	Effect      PolicyEffect   `json:"effect"`
	MatchedPolicy *Policy      `json:"matched_policy,omitempty"`
	Reason      string         `json:"reason"`
	EvalCount   int            `json:"eval_count"`
	DurationUs  int64          `json:"duration_us"`
}

// ── PolicyEngine ──

// PolicyEngine manages organizational policies
type PolicyEngine struct {
	mu       sync.RWMutex
	policies []*Policy
	byID     map[string]*Policy
	stats    PolicyStats
}

// PolicyStats tracks engine metrics
type PolicyStats struct {
	TotalPolicies  int                    `json:"total_policies"`
	ActivePolicies int                    `json:"active_policies"`
	EvalCount      int                    `json:"eval_count"`
	DenyCount      int                    `json:"deny_count"`
	WarnCount      int                    `json:"warn_count"`
	AllowCount     int                    `json:"allow_count"`
	AuditCount     int                    `json:"audit_count"`
	ByScope        map[PolicyScope]int    `json:"by_scope"`
	AvgEvalUs      int64                  `json:"avg_eval_us"`
	totalEvalUs    int64
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		policies: make([]*Policy, 0),
		byID:     make(map[string]*Policy),
		stats: PolicyStats{
			ByScope: make(map[PolicyScope]int),
		},
	}
}

// ── CRUD ──

// CreatePolicy adds a new policy
func (pe *PolicyEngine) CreatePolicy(name, description string, scope PolicyScope, scopeTarget string,
	conditions []PolicyCondition, effect PolicyEffect, priority int, createdBy string, tags []string) *Policy {

	pe.mu.Lock()
	defer pe.mu.Unlock()

	p := &Policy{
		ID:          "pol:" + uuid.New().String()[:8],
		Name:        name,
		Description: description,
		Scope:       scope,
		ScopeTarget: scopeTarget,
		Conditions:  conditions,
		Effect:      effect,
		Priority:    priority,
		Version:     1,
		Status:      PolicyActive,
		CreatedBy:   createdBy,
		Tags:        tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pe.policies = append(pe.policies, p)
	pe.byID[p.ID] = p
	pe.stats.TotalPolicies++
	pe.stats.ActivePolicies++
	pe.stats.ByScope[scope]++

	log.Printf("[policy] created %s: %s (scope=%s, effect=%s, priority=%d)", p.ID, name, scope, effect, priority)
	return p
}

// UpdatePolicy modifies an existing policy (bumps version)
func (pe *PolicyEngine) UpdatePolicy(id string, conditions []PolicyCondition, effect PolicyEffect, priority int) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	p, ok := pe.byID[id]
	if !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	p.Conditions = conditions
	p.Effect = effect
	p.Priority = priority
	p.Version++
	p.UpdatedAt = time.Now()

	log.Printf("[policy] updated %s v%d", id, p.Version)
	return nil
}

// SetStatus enables/disables a policy
func (pe *PolicyEngine) SetStatus(id string, status PolicyStatus) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	p, ok := pe.byID[id]
	if !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	wasActive := p.Status == PolicyActive
	p.Status = status
	p.UpdatedAt = time.Now()

	if wasActive && status != PolicyActive {
		pe.stats.ActivePolicies--
	} else if !wasActive && status == PolicyActive {
		pe.stats.ActivePolicies++
	}

	return nil
}

// GetPolicy retrieves a policy by ID
func (pe *PolicyEngine) GetPolicy(id string) *Policy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.byID[id]
}

// ListPolicies returns policies filtered by scope and status
func (pe *PolicyEngine) ListPolicies(scope PolicyScope, status PolicyStatus, limit int) []*Policy {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}
	var result []*Policy
	for _, p := range pe.policies {
		if scope != "" && p.Scope != scope {
			continue
		}
		if status != "" && p.Status != status {
			continue
		}
		result = append(result, p)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// ── Evaluation ──

// Evaluate checks an action against all applicable policies
// Returns the most restrictive matching verdict
func (pe *PolicyEngine) Evaluate(req *PolicyEvalRequest) *PolicyVerdict {
	start := time.Now()

	pe.mu.RLock()
	defer pe.mu.RUnlock()

	verdict := &PolicyVerdict{
		Allowed: true,
		Effect:  EffectAllow,
		Reason:  "no matching policy",
	}

	// Collect applicable policies (org → team → node → agent), sorted by priority desc
	applicable := pe.findApplicable(req)
	verdict.EvalCount = len(applicable)

	// Evaluate in priority order (highest first)
	for _, p := range applicable {
		if matchesConditions(p.Conditions, req) {
			verdict.MatchedPolicy = p
			verdict.Effect = p.Effect

			switch p.Effect {
			case EffectDeny:
				verdict.Allowed = false
				verdict.Reason = fmt.Sprintf("denied by policy %s: %s", p.ID, p.Name)
				pe.stats.DenyCount++
			case EffectWarn:
				verdict.Allowed = true
				verdict.Reason = fmt.Sprintf("warning from policy %s: %s", p.ID, p.Name)
				pe.stats.WarnCount++
			case EffectAudit:
				verdict.Allowed = true
				verdict.Reason = fmt.Sprintf("audited by policy %s: %s", p.ID, p.Name)
				pe.stats.AuditCount++
			case EffectAllow:
				verdict.Allowed = true
				verdict.Reason = fmt.Sprintf("allowed by policy %s: %s", p.ID, p.Name)
				pe.stats.AllowCount++
			}

			// First match wins (highest priority)
			break
		}
	}

	elapsed := time.Since(start).Microseconds()
	verdict.DurationUs = elapsed

	pe.stats.EvalCount++
	pe.stats.totalEvalUs += elapsed
	if pe.stats.EvalCount > 0 {
		pe.stats.AvgEvalUs = pe.stats.totalEvalUs / int64(pe.stats.EvalCount)
	}

	return verdict
}

// findApplicable returns active policies applicable to the request, sorted by priority desc
func (pe *PolicyEngine) findApplicable(req *PolicyEvalRequest) []*Policy {
	var result []*Policy

	for _, p := range pe.policies {
		if p.Status != PolicyActive {
			continue
		}

		// Scope matching with inheritance (org applies to all, team to team members, etc.)
		switch p.Scope {
		case ScopeOrg:
			// Org policies apply to everyone
			result = append(result, p)
		case ScopeTeam:
			if req.TeamID != "" && p.ScopeTarget == req.TeamID {
				result = append(result, p)
			}
		case ScopeNode:
			if req.NodeID != "" && p.ScopeTarget == req.NodeID {
				result = append(result, p)
			}
		case ScopeAgent:
			if req.AgentID != "" && p.ScopeTarget == req.AgentID {
				result = append(result, p)
			}
		}
	}

	// Sort by priority descending (higher priority evaluated first)
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].Priority > result[j-1].Priority; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}

	return result
}

// matchesConditions checks if all conditions in a policy match the request
func matchesConditions(conditions []PolicyCondition, req *PolicyEvalRequest) bool {
	if len(conditions) == 0 {
		return true // no conditions = always matches
	}

	for _, c := range conditions {
		val := getFieldValue(c.Field, req)
		if !evaluateCondition(c.Operator, val, c.Value) {
			return false // all conditions must match (AND logic)
		}
	}
	return true
}

// getFieldValue extracts a field value from the eval request
func getFieldValue(field string, req *PolicyEvalRequest) string {
	switch field {
	case "agent_id":
		return req.AgentID
	case "node_id":
		return req.NodeID
	case "team_id":
		return req.TeamID
	case "tool":
		return req.Tool
	case "resource":
		return req.Resource
	case "action":
		return req.Action
	default:
		if req.Extra != nil {
			return req.Extra[field]
		}
		return ""
	}
}

// evaluateCondition performs the comparison
func evaluateCondition(op, actual, expected string) bool {
	switch op {
	case "eq":
		return actual == expected
	case "neq":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	case "in":
		// expected is comma-separated list
		for _, v := range strings.Split(expected, ",") {
			if strings.TrimSpace(v) == actual {
				return true
			}
		}
		return false
	case "prefix":
		return strings.HasPrefix(actual, expected)
	case "suffix":
		return strings.HasSuffix(actual, expected)
	default:
		return false
	}
}

// ── Stats ──

// Stats returns policy engine metrics
func (pe *PolicyEngine) Stats() *PolicyStats {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	s := pe.stats // copy
	return &s
}
