package broodmind

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ════════════════════════════════════════════════════════════
// PolicyEngine HTTP Handlers — exposed via Claw /v1/broodmind/policy/*
// ════════════════════════════════════════════════════════════

// HandlePolicyStats handles GET /broodmind/policy/stats
func HandlePolicyStats(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Policy == nil {
		http.Error(w, `{"error":"policy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instance.Policy.Stats())
}

// HandlePolicyList handles GET /broodmind/policy/list?scope=...&status=...
func HandlePolicyList(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Policy == nil {
		http.Error(w, `{"error":"policy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	scope := PolicyScope(r.URL.Query().Get("scope"))
	status := PolicyStatus(r.URL.Query().Get("status"))
	policies := instance.Policy.ListPolicies(scope, status, 100)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"policies": policies,
		"count":    len(policies),
	})
}

// HandlePolicyGet handles GET /broodmind/policy/:id
func HandlePolicyGet(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Policy == nil {
		http.Error(w, `{"error":"policy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	id := parts[len(parts)-1]

	p := instance.Policy.GetPolicy(id)
	if p == nil {
		http.Error(w, `{"error":"policy not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// HandlePolicyCreate handles POST /broodmind/policy
func HandlePolicyCreate(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Policy == nil {
		http.Error(w, `{"error":"policy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Scope       PolicyScope       `json:"scope"`
		ScopeTarget string            `json:"scope_target"`
		Conditions  []PolicyCondition `json:"conditions"`
		Effect      PolicyEffect      `json:"effect"`
		Priority    int               `json:"priority"`
		CreatedBy   string            `json:"created_by"`
		Tags        []string          `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	if req.Scope == "" {
		req.Scope = ScopeOrg
	}
	if req.Effect == "" {
		req.Effect = EffectAudit
	}
	if req.CreatedBy == "" {
		req.CreatedBy = "admin"
	}

	p := instance.Policy.CreatePolicy(
		req.Name, req.Description, req.Scope, req.ScopeTarget,
		req.Conditions, req.Effect, req.Priority, req.CreatedBy, req.Tags,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// HandlePolicyUpdate handles PUT /broodmind/policy/:id
func HandlePolicyUpdate(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Policy == nil {
		http.Error(w, `{"error":"policy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	id := parts[len(parts)-1]

	var req struct {
		Conditions []PolicyCondition `json:"conditions"`
		Effect     PolicyEffect      `json:"effect"`
		Priority   int               `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := instance.Policy.UpdatePolicy(id, req.Conditions, req.Effect, req.Priority); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	p := instance.Policy.GetPolicy(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// HandlePolicySetStatus handles POST /broodmind/policy/:id/status
func HandlePolicySetStatus(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Policy == nil {
		http.Error(w, `{"error":"policy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	// expect .../policy/{id}/status
	if len(parts) < 3 {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	id := parts[len(parts)-2]

	var req struct {
		Status PolicyStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := instance.Policy.SetStatus(id, req.Status); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandlePolicyEvaluate handles POST /broodmind/policy/evaluate
func HandlePolicyEvaluate(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Policy == nil {
		http.Error(w, `{"error":"policy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req PolicyEvalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.AgentID == "" {
		http.Error(w, `{"error":"agent_id required"}`, http.StatusBadRequest)
		return
	}

	verdict := instance.Policy.Evaluate(&req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(verdict)
}
