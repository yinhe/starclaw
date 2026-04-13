package autonomy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// ════════════════════════════════════════════════════════════
// Autonomy HTTP Handlers — /v1/autonomy/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /autonomy/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eng.Stats())
}

// HandleGetLevel handles GET /autonomy/level
func HandleGetLevel(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"level":      eng.GetLevel(),
		"level_name": eng.GetLevel().String(),
	})
}

// HandleSetLevel handles POST /autonomy/level
func HandleSetLevel(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Level      int    `json:"level"`
		ApprovedBy string `json:"approved_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Level < 0 || req.Level > 3 {
		http.Error(w, `{"error":"level must be 0-3"}`, http.StatusBadRequest)
		return
	}
	if req.ApprovedBy == "" {
		req.ApprovedBy = "api"
	}
	eng.SetLevel(Level(req.Level), req.ApprovedBy)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"level":      eng.GetLevel(),
		"level_name": eng.GetLevel().String(),
	})
}

// HandleDecisions handles GET /autonomy/decisions?limit=20
func HandleDecisions(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	decisions := eng.ListDecisions(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decisions": decisions,
		"count":     len(decisions),
	})
}

// HandleGetDecision handles GET /autonomy/decision?id=xxx
func HandleGetDecision(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	d := eng.GetDecision(id)
	if d == nil {
		http.Error(w, `{"error":"decision not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}

// HandlePropose handles POST /autonomy/propose
func HandlePropose(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Type        string         `json:"type"`
		Trigger     string         `json:"trigger"`
		Description string         `json:"description"`
		Reasoning   string         `json:"reasoning"`
		Risk        string         `json:"risk"`
		Action      ProposedAction `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Action.WorkerType == "" || req.Action.Action == "" {
		http.Error(w, `{"error":"action.worker_type and action.action required"}`, http.StatusBadRequest)
		return
	}
	if req.Risk == "" {
		req.Risk = "medium"
	}
	d := eng.ProposeDecision(req.Type, req.Trigger, req.Description, req.Reasoning, req.Risk, req.Action)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

// HandleApprove handles POST /autonomy/approve
func HandleApprove(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID         string `json:"id"`
		ApprovedBy string `json:"approved_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.ApprovedBy == "" {
		req.ApprovedBy = "api"
	}
	d, err := eng.ApproveDecision(req.ID, req.ApprovedBy)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}

// HandleVeto handles POST /autonomy/veto
func HandleVeto(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID     string `json:"id"`
		VetoBy string `json:"veto_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.VetoBy == "" {
		req.VetoBy = "api"
	}
	d, err := eng.VetoDecision(req.ID, req.VetoBy)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}

// HandleRules handles GET /autonomy/rules
func HandleRules(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	rules := eng.ListRules()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
		"count": len(rules),
	})
}

// HandleAddRule handles POST /autonomy/rules
func HandleAddRule(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Conditions  []RuleCondition `json:"conditions"`
		MinLevel    int             `json:"min_level"`
		Action      ProposedAction  `json:"action"`
		Risk        string          `json:"risk"`
		CooldownSec int             `json:"cooldown_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" || len(req.Conditions) == 0 {
		http.Error(w, `{"error":"name and conditions required"}`, http.StatusBadRequest)
		return
	}
	cooldown := 5 * 60 // default 5min
	if req.CooldownSec > 0 {
		cooldown = req.CooldownSec
	}
	rule := eng.AddRule(req.Name, req.Description, req.Conditions, Level(req.MinLevel), req.Action, req.Risk, time.Duration(cooldown)*time.Second)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// HandleEvaluate handles POST /autonomy/evaluate
func HandleEvaluate(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Metrics map[string]float64 `json:"metrics"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	decisionIDs := eng.EvaluateRules(req.Metrics)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decisions_proposed": decisionIDs,
		"count":              len(decisionIDs),
	})
}

// HandleSnapshot handles POST /autonomy/snapshot
func HandleSnapshot(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	snap := eng.TakeSnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

// HandleSnapshots handles GET /autonomy/snapshots?limit=20
func HandleSnapshots(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	snaps := eng.ListSnapshots(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"snapshots": snaps,
		"count":     len(snaps),
	})
}

// HandleDiagnose handles POST /autonomy/diagnose
func HandleDiagnose(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	insights := eng.Diagnose()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"new_insights": insights,
		"count":        len(insights),
	})
}

// HandleInsights handles GET /autonomy/insights?limit=20
func HandleInsights(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"autonomy engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	insights := eng.ListInsights(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"insights": insights,
		"count":    len(insights),
	})
}
