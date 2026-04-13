package broodmind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ════════════════════════════════════════════════════════════
// Arbiter HTTP Handlers — exposed via Claw /v1/broodmind/arbiter/*
// ════════════════════════════════════════════════════════════

// HandleArbiterStats handles GET /broodmind/arbiter/stats
func HandleArbiterStats(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  instance.Arbiter.Stats(),
		"config": instance.Arbiter.Config(),
	})
}

// HandleArbiterPropose handles POST /broodmind/arbiter/propose
func HandleArbiterPropose(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		AgentID    string          `json:"agent_id"`
		NodeID     string          `json:"node_id"`
		Resource   string          `json:"resource"`
		Action     string          `json:"action"`
		Params     json.RawMessage `json:"params"`
		Priority   int             `json:"priority"`
		Confidence float64         `json:"confidence"`
		Reason     string          `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.AgentID == "" || req.Resource == "" || req.Action == "" {
		http.Error(w, `{"error":"agent_id, resource, and action required"}`, http.StatusBadRequest)
		return
	}
	if req.NodeID == "" {
		req.NodeID = instance.nodeID
	}
	if req.Confidence <= 0 {
		req.Confidence = 0.5
	}

	proposal, conflict := instance.Arbiter.Propose(
		req.AgentID, req.NodeID, req.Resource, req.Action, req.Reason,
		req.Params, req.Priority, req.Confidence,
	)

	resp := map[string]interface{}{
		"proposal": proposal,
	}
	if conflict != nil {
		resp["conflict"] = conflict
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleArbiterProposals handles GET /broodmind/arbiter/proposals?status=...&resource=...
func HandleArbiterProposals(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	status := ProposalStatus(r.URL.Query().Get("status"))
	resource := r.URL.Query().Get("resource")
	limit := 50

	proposals := instance.Arbiter.ListProposals(status, resource, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"proposals": proposals,
		"count":     len(proposals),
	})
}

// HandleArbiterConflicts handles GET /broodmind/arbiter/conflicts?status=...
func HandleArbiterConflicts(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	status := r.URL.Query().Get("status")
	conflicts := instance.Arbiter.ListConflicts(status, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conflicts": conflicts,
		"count":     len(conflicts),
	})
}

// HandleArbiterVote handles POST /broodmind/arbiter/vote
func HandleArbiterVote(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ConflictID string `json:"conflict_id"`
		AgentID    string `json:"agent_id"`
		ProposalID string `json:"proposal_id"`
		Approve    bool   `json:"approve"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.ConflictID == "" || req.AgentID == "" || req.ProposalID == "" {
		http.Error(w, `{"error":"conflict_id, agent_id, and proposal_id required"}`, http.StatusBadRequest)
		return
	}

	if err := instance.Arbiter.CastVote(req.ConflictID, req.AgentID, req.ProposalID, req.Approve, req.Reason); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Return updated conflict
	conflict := instance.Arbiter.GetConflict(req.ConflictID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"conflict": conflict,
	})
}

// HandleArbiterResolve handles POST /broodmind/arbiter/resolve
func HandleArbiterResolve(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ConflictID string `json:"conflict_id"`
		WinnerID   string `json:"winner_id"`
		ResolvedBy string `json:"resolved_by"`
		Explanation string `json:"explanation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.ConflictID == "" || req.WinnerID == "" {
		http.Error(w, `{"error":"conflict_id and winner_id required"}`, http.StatusBadRequest)
		return
	}
	if req.ResolvedBy == "" {
		req.ResolvedBy = "admin"
	}

	if err := instance.Arbiter.ResolveManually(req.ConflictID, req.WinnerID, req.ResolvedBy, req.Explanation); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	conflict := instance.Arbiter.GetConflict(req.ConflictID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"conflict": conflict,
	})
}

// HandleArbiterStrategy handles POST /broodmind/arbiter/strategy
func HandleArbiterStrategy(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Resource string          `json:"resource"`
		Strategy ArbiterStrategy `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	valid := map[ArbiterStrategy]bool{
		StrategyPriority: true, StrategyVoting: true,
		StrategyConsensus: true, StrategyDelegate: true,
	}
	if !valid[req.Strategy] {
		http.Error(w, fmt.Sprintf(`{"error":"invalid strategy, use: %s"}`,
			strings.Join([]string{string(StrategyPriority), string(StrategyVoting), string(StrategyConsensus), string(StrategyDelegate)}, ",")),
			http.StatusBadRequest)
		return
	}

	instance.Arbiter.SetStrategy(req.Resource, req.Strategy)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"config": instance.Arbiter.Config(),
	})
}

// HandleArbiterWeight handles POST /broodmind/arbiter/weight
func HandleArbiterWeight(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Arbiter == nil {
		http.Error(w, `{"error":"arbiter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		AgentID string  `json:"agent_id"`
		Weight  float64 `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.AgentID == "" {
		http.Error(w, `{"error":"agent_id required"}`, http.StatusBadRequest)
		return
	}

	instance.Arbiter.SetAgentWeight(req.AgentID, req.Weight)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
