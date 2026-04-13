package zergling

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Zergling Adapter HTTP Handlers — /v1/zergling/*
// ════════════════════════════════════════════════════════════

// HandleZerglingStats handles GET /zergling/stats
func HandleZerglingStats(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  a.Stats(),
		"safety": a.SafetyConfig(),
		"config": map[string]interface{}{
			"bridge_url": a.config.BridgeURL,
			"model":      a.config.Model,
		},
	})
}

// HandleZerglingStatus handles GET /zergling/status
func HandleZerglingStatus(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := a.Status()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleZerglingMove handles POST /zergling/move
func HandleZerglingMove(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Vx         float64 `json:"vx"`
		Vy         float64 `json:"vy"`
		Vyaw       float64 `json:"vyaw"`
		DurationMs int     `json:"duration_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	result, err := a.Move(req.Vx, req.Vy, req.Vyaw, req.DurationMs)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingGoto handles POST /zergling/goto
func HandleZerglingGoto(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		X   float64 `json:"x"`
		Y   float64 `json:"y"`
		Yaw float64 `json:"yaw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	result, err := a.Goto(req.X, req.Y, req.Yaw)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingAction handles POST /zergling/action
func HandleZerglingAction(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	result, err := a.Action(req.Name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingStop handles POST /zergling/stop
func HandleZerglingStop(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	result, err := a.Stop()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingGait handles POST /zergling/gait
func HandleZerglingGait(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Type GaitType `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	result, err := a.SetGait(req.Type)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingObstacle handles POST /zergling/obstacle
func HandleZerglingObstacle(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	result, err := a.SetObstacleAvoidance(req.Enable)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingPatrol handles POST /zergling/patrol
func HandleZerglingPatrol(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Waypoints []Vec3 `json:"waypoints"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if len(req.Waypoints) == 0 {
		http.Error(w, `{"error":"waypoints required"}`, http.StatusBadRequest)
		return
	}

	result, err := a.Patrol(req.Waypoints)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingCamera handles GET /zergling/camera?format=jpeg
func HandleZerglingCamera(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	format := r.URL.Query().Get("format")
	result, err := a.GetCamera(format)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleZerglingSafety handles GET /zergling/safety?limit=20
func HandleZerglingSafety(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"zergling adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	violations := a.SafetyViolations(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"violations": violations,
		"config":     a.SafetyConfig(),
	})
}
