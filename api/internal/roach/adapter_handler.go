package roach

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Roach Adapter HTTP Handlers — /v1/roach/*
// ════════════════════════════════════════════════════════════

// HandleRoachStats handles GET /roach/stats
func HandleRoachStats(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  a.Stats(),
		"safety": a.SafetyConfig(),
		"config": map[string]interface{}{
			"bridge_url": a.config.BridgeURL,
			"chassis":    a.config.Chassis,
		},
	})
}

// HandleRoachStatus handles GET /roach/status
func HandleRoachStatus(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a.Status())
}

// HandleRoachMove handles POST /roach/move
func HandleRoachMove(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Linear     float64 `json:"linear"`
		Angular    float64 `json:"angular"`
		DurationMs int     `json:"duration_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Move(req.Linear, req.Angular, req.DurationMs)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachGoto handles POST /roach/goto
func HandleRoachGoto(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
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

// HandleRoachPatrol handles POST /roach/patrol
func HandleRoachPatrol(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Waypoints []Pose2D `json:"waypoints"`
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

// HandleRoachStop handles POST /roach/stop
func HandleRoachStop(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
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

// HandleRoachSpin handles POST /roach/spin
func HandleRoachSpin(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		AngleDeg float64 `json:"angle_deg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Spin(req.AngleDeg)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachBackup handles POST /roach/backup
func HandleRoachBackup(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		DistanceM float64 `json:"distance_m"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.DistanceM <= 0 {
		req.DistanceM = 0.5
	}
	result, err := a.Backup(req.DistanceM)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachMapSave handles POST /roach/map/save
func HandleRoachMapSave(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Filename == "" {
		req.Filename = "roach_map"
	}
	result, err := a.MapSave(req.Filename)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachMapLoad handles POST /roach/map/load
func HandleRoachMapLoad(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Filename == "" {
		http.Error(w, `{"error":"filename required"}`, http.StatusBadRequest)
		return
	}
	result, err := a.MapLoad(req.Filename)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachSetPose handles POST /roach/set_pose
func HandleRoachSetPose(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
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
	result, err := a.SetPose(req.X, req.Y, req.Yaw)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachCamera handles GET /roach/camera?id=front
func HandleRoachCamera(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	cameraID := r.URL.Query().Get("id")
	result, err := a.GetCamera(cameraID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachLidar handles GET /roach/lidar
func HandleRoachLidar(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	result, err := a.GetLidar()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoachSafety handles GET /roach/safety?limit=20
func HandleRoachSafety(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"roach adapter not initialized"}`, http.StatusServiceUnavailable)
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
