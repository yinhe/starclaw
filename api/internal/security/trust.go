package security

import (
	"fmt"
	"strings"
	"sync"
)

// ════════════════════════════════════════════════════════════
// Agent Security v1 — Trust Level System
//
// 4-tier trust model for agent tool access control:
//   L0 Untrusted  — third-party/marketplace agents, read-only tools
//   L1 Basic      — installed agents with verified manifest
//   L2 Trusted    — system agents or admin-approved
//   L3 System     — core system agents (Abathur, OpsClaw), unrestricted
//
// Each level defines which tool categories are accessible.
// ════════════════════════════════════════════════════════════

// AgentTrustLevel defines the trust tier of an agent
type AgentTrustLevel int

const (
	TrustUntrusted AgentTrustLevel = 0 // marketplace / unknown agents
	TrustBasic     AgentTrustLevel = 1 // installed, manifest verified
	TrustTrusted   AgentTrustLevel = 2 // system or admin-approved
	TrustSystem    AgentTrustLevel = 3 // core system agents
)

func (l AgentTrustLevel) String() string {
	switch l {
	case TrustUntrusted:
		return "untrusted"
	case TrustBasic:
		return "basic"
	case TrustTrusted:
		return "trusted"
	case TrustSystem:
		return "system"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

// ParseTrustLevel converts a string to AgentTrustLevel
func ParseTrustLevel(s string) AgentTrustLevel {
	switch strings.ToLower(s) {
	case "untrusted", "0":
		return TrustUntrusted
	case "basic", "1":
		return TrustBasic
	case "trusted", "2":
		return TrustTrusted
	case "system", "3":
		return TrustSystem
	default:
		return TrustUntrusted
	}
}

// ToolCategory classifies tools by sensitivity
type ToolCategory string

const (
	CatReadOnly   ToolCategory = "read_only"  // search, list, info queries
	CatStandard   ToolCategory = "standard"   // general purpose tools
	CatSensitive  ToolCategory = "sensitive"  // file write, network, code exec
	CatPrivileged ToolCategory = "privileged" // system config, admin, billing
	CatDangerous  ToolCategory = "dangerous"  // shell exec, arbitrary code, disk wipe
)

// PermissionMatrix defines which tool categories each trust level can access
var PermissionMatrix = map[AgentTrustLevel]map[ToolCategory]bool{
	TrustUntrusted: {
		CatReadOnly: true,
	},
	TrustBasic: {
		CatReadOnly: true,
		CatStandard: true,
	},
	TrustTrusted: {
		CatReadOnly:  true,
		CatStandard:  true,
		CatSensitive: true,
	},
	TrustSystem: {
		CatReadOnly:   true,
		CatStandard:   true,
		CatSensitive:  true,
		CatPrivileged: true,
		CatDangerous:  true,
	},
}

// ToolClassifier maps tool names to categories
type ToolClassifier struct {
	mu       sync.RWMutex
	explicit map[string]ToolCategory // explicit per-tool overrides
	prefixes map[string]ToolCategory // prefix-based classification
}

// NewToolClassifier creates a classifier with default rules
func NewToolClassifier() *ToolClassifier {
	tc := &ToolClassifier{
		explicit: make(map[string]ToolCategory),
		prefixes: map[string]ToolCategory{
			"mcp_":   CatStandard, // MCP bridge tools default to standard
			"search": CatReadOnly,
			"web_":   CatReadOnly, // web_search, web_browse are read-only
			"list":   CatReadOnly,
			"get":    CatReadOnly,
			"query":  CatReadOnly,
		},
	}

	// Classify known sensitive/privileged/dangerous tools
	sensitive := []string{
		"file_write", "file_create", "file_delete",
		"http_request", "code_execute",
		"screen_capture", "keyboard_type", "mouse_click",
		"browser_navigate", "browser_evaluate",
	}
	for _, t := range sensitive {
		tc.explicit[t] = CatSensitive
	}

	privileged := []string{
		"system_config", "admin_command",
		"billing_charge", "billing_refund",
		"node_manage", "agent_deploy",
	}
	for _, t := range privileged {
		tc.explicit[t] = CatPrivileged
	}

	dangerous := []string{
		"shell_exec", "eval", "disk_format",
		"process_kill", "registry_edit",
	}
	for _, t := range dangerous {
		tc.explicit[t] = CatDangerous
	}

	return tc
}

// Classify returns the category for a tool name
func (tc *ToolClassifier) Classify(toolName string) ToolCategory {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// Check explicit mapping first
	if cat, ok := tc.explicit[toolName]; ok {
		return cat
	}

	// Check prefix-based rules
	for prefix, cat := range tc.prefixes {
		if strings.HasPrefix(toolName, prefix) {
			return cat
		}
	}

	// Default: standard (most tools are general purpose)
	return CatStandard
}

// SetCategory explicitly classifies a tool
func (tc *ToolClassifier) SetCategory(toolName string, cat ToolCategory) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.explicit[toolName] = cat
}

// AgentTrustRegistry tracks agent trust levels
type AgentTrustRegistry struct {
	mu     sync.RWMutex
	agents map[string]AgentTrustLevel

	// System agents that are always TrustSystem
	systemAgents map[string]bool
}

// NewAgentTrustRegistry creates a new trust registry with default system agents
func NewAgentTrustRegistry() *AgentTrustRegistry {
	return &AgentTrustRegistry{
		agents: make(map[string]AgentTrustLevel),
		systemAgents: map[string]bool{
			"abathur":    true,
			"ops_claw":   true,
			"dev_team":   true,
			"test_claw":  true,
			"scout_claw": true,
			"sense_claw": true,
			"assistant":  true, // built-in general assistant
		},
	}
}

// GetLevel returns the trust level for an agent
func (r *AgentTrustRegistry) GetLevel(agentID string) AgentTrustLevel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// System agents are always max trust
	if r.systemAgents[agentID] {
		return TrustSystem
	}

	if level, ok := r.agents[agentID]; ok {
		return level
	}

	// Default: basic trust for user-created/installed agents
	// L0 (untrusted) is reserved for explicitly demoted agents
	return TrustBasic
}

// SetLevel sets the trust level for an agent
func (r *AgentTrustRegistry) SetLevel(agentID string, level AgentTrustLevel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agentID] = level
}

// Promote increases an agent's trust level by one tier (max TrustTrusted for non-system)
func (r *AgentTrustRegistry) Promote(agentID string) AgentTrustLevel {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.systemAgents[agentID] {
		return TrustSystem
	}

	current, ok := r.agents[agentID]
	if !ok {
		current = TrustUntrusted
	}
	if current < TrustTrusted {
		current++
	}
	r.agents[agentID] = current
	return current
}

// Demote decreases an agent's trust level by one tier (min TrustUntrusted)
func (r *AgentTrustRegistry) Demote(agentID string) AgentTrustLevel {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.systemAgents[agentID] {
		return TrustSystem // cannot demote system agents
	}

	current, ok := r.agents[agentID]
	if !ok {
		return TrustUntrusted
	}
	if current > TrustUntrusted {
		current--
	}
	r.agents[agentID] = current
	return current
}

// IsAllowed checks if an agent can use a tool given the permission matrix
func (r *AgentTrustRegistry) IsAllowed(agentID string, toolCategory ToolCategory) bool {
	level := r.GetLevel(agentID)
	perms, ok := PermissionMatrix[level]
	if !ok {
		return false
	}
	return perms[toolCategory]
}

// ListAgents returns all registered agents with their trust levels
func (r *AgentTrustRegistry) ListAgents() map[string]AgentTrustLevel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]AgentTrustLevel, len(r.agents)+len(r.systemAgents))
	for id := range r.systemAgents {
		result[id] = TrustSystem
	}
	for id, level := range r.agents {
		result[id] = level
	}
	return result
}

// Stats returns trust registry statistics
func (r *AgentTrustRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	byLevel := map[string]int{}
	for _, level := range r.agents {
		byLevel[level.String()]++
	}
	byLevel["system"] = len(r.systemAgents)

	return map[string]interface{}{
		"total_agents":  len(r.agents) + len(r.systemAgents),
		"system_agents": len(r.systemAgents),
		"custom_agents": len(r.agents),
		"by_level":      byLevel,
	}
}
