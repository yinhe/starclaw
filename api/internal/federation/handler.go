package federation

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Federation HTTP Handlers — /v1/federation/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /federation/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eng.Stats())
}

// HandleRegisterSwarm handles POST /federation/swarm
func HandleRegisterSwarm(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID           string            `json:"id"`
		Name         string            `json:"name"`
		Endpoint     string            `json:"endpoint"`
		Region       string            `json:"region"`
		Capabilities []string          `json:"capabilities"`
		NodeCount    int               `json:"node_count"`
		AgentCount   int               `json:"agent_count"`
		Metadata     map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.Name == "" {
		http.Error(w, `{"error":"id and name required"}`, http.StatusBadRequest)
		return
	}
	node := eng.RegisterSwarm(req.ID, req.Name, req.Endpoint, req.Region, req.Capabilities, req.NodeCount, req.AgentCount, req.Metadata)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// HandleListSwarms handles GET /federation/swarms?status=
func HandleListSwarms(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	swarms := eng.ListSwarms(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"swarms": swarms,
		"count":  len(swarms),
	})
}

// HandleGetSwarm handles GET /federation/swarm?id=
func HandleGetSwarm(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	s := eng.GetSwarm(id)
	if s == nil {
		http.Error(w, `{"error":"swarm not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

// HandleHeartbeat handles POST /federation/heartbeat
func HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SwarmID    string `json:"swarm_id"`
		NodeCount  int    `json:"node_count"`
		AgentCount int    `json:"agent_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.Heartbeat(req.SwarmID, req.NodeCount, req.AgentCount); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleInitHandshake handles POST /federation/handshake
func HandleInitHandshake(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		TargetSwarmID string `json:"target_swarm_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	hs, err := eng.InitHandshake(req.TargetSwarmID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(hs)
}

// HandleCompleteHandshake handles POST /federation/handshake/complete
func HandleCompleteHandshake(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		HandshakeID string `json:"handshake_id"`
		Response    string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.CompleteHandshake(req.HandshakeID, req.Response); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
}

// HandleProposeRoute handles POST /federation/route
func HandleProposeRoute(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		TargetSwarm string                 `json:"target_swarm"`
		TaskType    string                 `json:"task_type"`
		Description string                 `json:"description"`
		Priority    string                 `json:"priority"`
		Params      map[string]interface{} `json:"params"`
		Bid         float64                `json:"bid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	route, err := eng.ProposeRoute(req.TargetSwarm, req.TaskType, req.Description, req.Priority, req.Params, req.Bid)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(route)
}

// HandleAcceptRoute handles POST /federation/route/accept
func HandleAcceptRoute(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		RouteID string `json:"route_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.AcceptRoute(req.RouteID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// HandleCompleteRoute handles POST /federation/route/complete
func HandleCompleteRoute(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		RouteID string                 `json:"route_id"`
		Success bool                   `json:"success"`
		Result  map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.CompleteRoute(req.RouteID, req.Result, req.Success); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
}

// HandleListRoutes handles GET /federation/routes?status=&limit=20
func HandleListRoutes(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	routes := eng.ListRoutes(status, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"routes": routes,
		"count":  len(routes),
	})
}

// HandleFindBestSwarm handles GET /federation/find?capability=
func HandleFindBestSwarm(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	cap := r.URL.Query().Get("capability")
	if cap == "" {
		http.Error(w, `{"error":"capability required"}`, http.StatusBadRequest)
		return
	}
	best := eng.FindBestSwarm(cap)
	if best == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "no swarm found with capability: " + cap})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(best)
}

// HandleTrustEvents handles GET /federation/trust?swarm_id=&limit=20
func HandleTrustEvents(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	swarmID := r.URL.Query().Get("swarm_id")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events := eng.ListTrustEvents(swarmID, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// HandleSuspendSwarm handles POST /federation/suspend
func HandleSuspendSwarm(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"federation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SwarmID string `json:"swarm_id"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.SuspendSwarm(req.SwarmID, req.Reason); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "suspended"})
}
