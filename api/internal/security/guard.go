package security

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

// ════════════════════════════════════════════════════════════
// Agent Security v1 — Execution Guard
//
// Pre-execution interceptor that enforces trust-level access
// control on tool calls. Integrates into the tool.Registry
// via the ExecuteHook chain.
//
// Flow: Agent → Guard.Check(trust, tool) → Sandbox.Execute() → Tool
// ════════════════════════════════════════════════════════════

// Guard is the central security enforcement component
type Guard struct {
	TrustRegistry *AgentTrustRegistry
	Classifier    *ToolClassifier
	Sandbox       *Sandbox

	mu             sync.RWMutex
	toolWhitelists map[string]map[string]bool // agent_id → tool_name → allowed
	toolBlacklists map[string]map[string]bool // agent_id → tool_name → blocked
}

// NewGuard creates a fully initialized security guard
func NewGuard(cfg *SandboxConfig) *Guard {
	return &Guard{
		TrustRegistry:  NewAgentTrustRegistry(),
		Classifier:     NewToolClassifier(),
		Sandbox:        NewSandbox(cfg),
		toolWhitelists: make(map[string]map[string]bool),
		toolBlacklists: make(map[string]map[string]bool),
	}
}

// singleton
var (
	globalGuard *Guard
	guardOnce   sync.Once
)

// InitGuard initializes the global security guard
func InitGuard(cfg *SandboxConfig) *Guard {
	guardOnce.Do(func() {
		globalGuard = NewGuard(cfg)
		log.Printf("[security] guard initialized (timeout=%s, longRunTimeout=%s, longRunTools=%d, maxConcurrent=%d, maxOutput=%d)",
			globalGuard.Sandbox.config.DefaultTimeout,
			globalGuard.Sandbox.config.LongRunTimeout,
			len(globalGuard.Sandbox.config.LongRunningTools),
			globalGuard.Sandbox.config.MaxConcurrent,
			globalGuard.Sandbox.config.MaxOutputSize)
	})
	return globalGuard
}

// GetGuard returns the global guard instance
func GetGuard() *Guard {
	return globalGuard
}

// CheckAccess verifies if an agent is allowed to execute a tool.
// Returns nil if allowed, error describing the denial reason otherwise.
func (g *Guard) CheckAccess(agentID, toolName string) error {
	// 1. Check explicit blacklist
	if g.isBlacklisted(agentID, toolName) {
		reason := fmt.Sprintf("tool %s is blacklisted for agent %s", toolName, agentID)
		g.Sandbox.RecordTrustDenial(agentID, toolName, reason)
		return fmt.Errorf("security: %s", reason)
	}

	// 2. Check explicit whitelist (bypasses trust check)
	if g.isWhitelisted(agentID, toolName) {
		return nil
	}

	// 3. Trust-level check via permission matrix
	category := g.Classifier.Classify(toolName)
	level := g.TrustRegistry.GetLevel(agentID)

	if !g.TrustRegistry.IsAllowed(agentID, category) {
		reason := fmt.Sprintf("agent %s (trust=%s) cannot access %s tool %s (category=%s)",
			agentID, level, category, toolName, category)
		g.Sandbox.RecordTrustDenial(agentID, toolName, reason)
		return fmt.Errorf("security: %s", reason)
	}

	return nil
}

// SetWhitelist adds tools to an agent's whitelist (bypass trust check)
func (g *Guard) SetWhitelist(agentID string, tools []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	wl := make(map[string]bool, len(tools))
	for _, t := range tools {
		wl[t] = true
	}
	g.toolWhitelists[agentID] = wl
}

// SetBlacklist adds tools to an agent's blacklist (always denied)
func (g *Guard) SetBlacklist(agentID string, tools []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	bl := make(map[string]bool, len(tools))
	for _, t := range tools {
		bl[t] = true
	}
	g.toolBlacklists[agentID] = bl
}

func (g *Guard) isWhitelisted(agentID, toolName string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if wl, ok := g.toolWhitelists[agentID]; ok {
		return wl[toolName]
	}
	return false
}

func (g *Guard) isBlacklisted(agentID, toolName string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if bl, ok := g.toolBlacklists[agentID]; ok {
		return bl[toolName]
	}
	return false
}

// Stats returns combined security statistics
func (g *Guard) Stats() map[string]interface{} {
	return map[string]interface{}{
		"trust":   g.TrustRegistry.Stats(),
		"sandbox": g.Sandbox.GetStats(),
	}
}

// ── HTTP Handlers ──

// HandleSecurityStats handles GET /security/stats
func HandleSecurityStats(w http.ResponseWriter, r *http.Request) {
	guard := GetGuard()
	if guard == nil {
		http.Error(w, `{"error":"security guard not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(guard.Stats())
}

// HandleSecurityViolations handles GET /security/violations?limit=50
func HandleSecurityViolations(w http.ResponseWriter, r *http.Request) {
	guard := GetGuard()
	if guard == nil {
		http.Error(w, `{"error":"security guard not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			limit = n
		}
	}
	violations := guard.Sandbox.GetViolations(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"violations": violations,
		"count":      len(violations),
	})
}

// HandleTrustList handles GET /security/trust — list all agents with trust levels
func HandleTrustList(w http.ResponseWriter, r *http.Request) {
	guard := GetGuard()
	if guard == nil {
		http.Error(w, `{"error":"security guard not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	agents := guard.TrustRegistry.ListAgents()
	type agentEntry struct {
		AgentID string `json:"agent_id"`
		Level   int    `json:"level"`
		Label   string `json:"label"`
	}
	var list []agentEntry
	for id, level := range agents {
		list = append(list, agentEntry{AgentID: id, Level: int(level), Label: level.String()})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"agents": list, "count": len(list)})
}

// HandleTrustSet handles POST /security/trust — set an agent's trust level
func HandleTrustSet(w http.ResponseWriter, r *http.Request) {
	guard := GetGuard()
	if guard == nil {
		http.Error(w, `{"error":"security guard not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		AgentID string `json:"agent_id"`
		Level   string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.AgentID == "" {
		http.Error(w, `{"error":"agent_id required"}`, http.StatusBadRequest)
		return
	}
	level := ParseTrustLevel(req.Level)
	guard.TrustRegistry.SetLevel(req.AgentID, level)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": req.AgentID,
		"level":    int(level),
		"label":    level.String(),
	})
}

// HandleTrustCheck handles GET /security/trust/check?agent=...&tool=...
func HandleTrustCheck(w http.ResponseWriter, r *http.Request) {
	guard := GetGuard()
	if guard == nil {
		http.Error(w, `{"error":"security guard not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	agentID := r.URL.Query().Get("agent")
	toolName := r.URL.Query().Get("tool")
	if agentID == "" || toolName == "" {
		http.Error(w, `{"error":"agent and tool query params required"}`, http.StatusBadRequest)
		return
	}

	err := guard.CheckAccess(agentID, toolName)
	category := guard.Classifier.Classify(toolName)
	level := guard.TrustRegistry.GetLevel(agentID)

	result := map[string]interface{}{
		"agent_id":      agentID,
		"tool":          toolName,
		"tool_category": category,
		"agent_trust":   level.String(),
		"allowed":       err == nil,
	}
	if err != nil {
		result["reason"] = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleWhitelistSet handles POST /security/whitelist
func HandleWhitelistSet(w http.ResponseWriter, r *http.Request) {
	guard := GetGuard()
	if guard == nil {
		http.Error(w, `{"error":"security guard not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		AgentID string   `json:"agent_id"`
		Tools   []string `json:"tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	guard.SetWhitelist(req.AgentID, req.Tools)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "agent_id": req.AgentID, "whitelisted": len(req.Tools)})
}

// HandlePermissionMatrix handles GET /security/permissions
func HandlePermissionMatrix(w http.ResponseWriter, r *http.Request) {
	type row struct {
		Level      string          `json:"level"`
		LevelNum   int             `json:"level_num"`
		Categories map[string]bool `json:"categories"`
	}
	var matrix []row
	for level := TrustUntrusted; level <= TrustSystem; level++ {
		cats := make(map[string]bool)
		for _, cat := range []ToolCategory{CatReadOnly, CatStandard, CatSensitive, CatPrivileged, CatDangerous} {
			if perms, ok := PermissionMatrix[level]; ok {
				cats[string(cat)] = perms[cat]
			}
		}
		matrix = append(matrix, row{
			Level:      level.String(),
			LevelNum:   int(level),
			Categories: cats,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"matrix": matrix})
}

// HandleToolClassify handles GET /security/classify?tool=...
func HandleToolClassify(w http.ResponseWriter, r *http.Request) {
	guard := GetGuard()
	if guard == nil {
		http.Error(w, `{"error":"security guard not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	toolName := r.URL.Query().Get("tool")
	if toolName == "" {
		http.Error(w, `{"error":"tool query param required"}`, http.StatusBadRequest)
		return
	}

	// Support comma-separated tool names
	tools := strings.Split(toolName, ",")
	type classResult struct {
		Tool     string `json:"tool"`
		Category string `json:"category"`
	}
	var results []classResult
	for _, t := range tools {
		t = strings.TrimSpace(t)
		if t != "" {
			results = append(results, classResult{Tool: t, Category: string(guard.Classifier.Classify(t))})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"classifications": results})
}
