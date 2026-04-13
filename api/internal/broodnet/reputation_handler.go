package broodnet

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ════════════════════════════════════════════════════════════
// NodeReputation HTTP Handlers — /v1/broodnet/rep/*
// ════════════════════════════════════════════════════════════

// HandleRepStats handles GET /broodnet/rep/stats
func HandleRepStats(w http.ResponseWriter, r *http.Request) {
	re := GetReputation()
	if re == nil {
		http.Error(w, `{"error":"reputation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  re.Stats(),
		"config": re.RepConfig(),
	})
}

// HandleRepRecord handles POST /broodnet/rep/record
func HandleRepRecord(w http.ResponseWriter, r *http.Request) {
	re := GetReputation()
	if re == nil {
		http.Error(w, `{"error":"reputation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		NodeID  string       `json:"node_id"`
		Type    RepEventType `json:"type"`
		Context string       `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.NodeID == "" {
		http.Error(w, `{"error":"node_id required"}`, http.StatusBadRequest)
		return
	}

	evt, err := re.RecordEvent(req.NodeID, req.Type, req.Context)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(evt)
}

// HandleRepProfile handles GET /broodnet/rep/profile/:id
func HandleRepProfile(w http.ResponseWriter, r *http.Request) {
	re := GetReputation()
	if re == nil {
		http.Error(w, `{"error":"reputation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	nodeID := parts[len(parts)-1]

	profile := re.GetProfile(nodeID)
	if profile == nil {
		http.Error(w, `{"error":"node not found"}`, http.StatusNotFound)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("events"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events := re.GetEvents(nodeID, limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"profile": profile,
		"events":  events,
	})
}

// HandleRepLeaderboard handles GET /broodnet/rep/leaderboard?limit=20
func HandleRepLeaderboard(w http.ResponseWriter, r *http.Request) {
	re := GetReputation()
	if re == nil {
		http.Error(w, `{"error":"reputation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	board := re.Leaderboard(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"leaderboard": board,
		"count":       len(board),
	})
}

// HandleRepList handles GET /broodnet/rep/nodes?tier=...
func HandleRepList(w http.ResponseWriter, r *http.Request) {
	re := GetReputation()
	if re == nil {
		http.Error(w, `{"error":"reputation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	tier := TrustTier(r.URL.Query().Get("tier"))
	profiles := re.ListProfiles(tier, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": profiles,
		"count": len(profiles),
	})
}

// HandleRepDecay handles POST /broodnet/rep/decay (manual trigger)
func HandleRepDecay(w http.ResponseWriter, r *http.Request) {
	re := GetReputation()
	if re == nil {
		http.Error(w, `{"error":"reputation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	decayed := re.ApplyDecay()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decayed": decayed,
	})
}
