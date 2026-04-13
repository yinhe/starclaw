package hydralisk_v

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Hydralisk Vehicle Adapter HTTP Handlers — /v1/hydralisk_v/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /hydralisk_v/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  a.Stats(),
		"safety": a.SafetyConfig(),
		"config": map[string]interface{}{
			"bridge_url":   a.config.BridgeURL,
			"vehicle_type": a.config.VehicleType,
		},
	})
}

// HandleStatus handles GET /hydralisk_v/status
func HandleStatus(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a.Status())
}

// HandleDrive handles POST /hydralisk_v/drive
func HandleDrive(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Speed    float64 `json:"speed_mps"`
		Steering float64 `json:"steering_deg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Drive(req.Speed, req.Steering)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleGoto handles POST /hydralisk_v/goto
func HandleGoto(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
		SpeedLimit float64 `json:"speed_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Goto(req.Latitude, req.Longitude, req.SpeedLimit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleRoute handles POST /hydralisk_v/route
func HandleRoute(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Waypoints  []GPSPosition `json:"waypoints"`
		SpeedLimit float64       `json:"speed_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if len(req.Waypoints) == 0 {
		http.Error(w, `{"error":"waypoints required"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Route(req.Waypoints, req.SpeedLimit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleStop handles POST /hydralisk_v/stop
func HandleStop(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
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

// HandlePark handles POST /hydralisk_v/park
func HandlePark(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	result, err := a.Park()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleGear handles POST /hydralisk_v/gear
func HandleGear(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Gear GearPosition `json:"gear"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.SetGear(req.Gear)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleLights handles POST /hydralisk_v/lights
func HandleLights(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Headlight   bool `json:"headlight"`
		LeftSignal  bool `json:"left_signal"`
		RightSignal bool `json:"right_signal"`
		Hazard      bool `json:"hazard"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.SetLights(req.Headlight, req.LeftSignal, req.RightSignal, req.Hazard)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleHorn handles POST /hydralisk_v/horn
func HandleHorn(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		DurationMs int `json:"duration_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Horn(req.DurationMs)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleCargo handles POST /hydralisk_v/cargo
func HandleCargo(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Cargo(req.Action)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleCamera handles GET /hydralisk_v/camera?id=front
func HandleCamera(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
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

// HandleSpeedLimit handles POST /hydralisk_v/speed_limit
func HandleSpeedLimit(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		MaxSpeedMps float64 `json:"max_speed_mps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.MaxSpeedMps <= 0 || req.MaxSpeedMps > 20.0 {
		http.Error(w, `{"error":"max_speed_mps must be 0-20"}`, http.StatusBadRequest)
		return
	}
	a.SetSpeedLimit(req.MaxSpeedMps)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":            true,
		"max_speed_mps": req.MaxSpeedMps,
		"max_speed_kmh": req.MaxSpeedMps * 3.6,
	})
}

// HandleSafety handles GET /hydralisk_v/safety?limit=20
func HandleSafety(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"hydralisk vehicle adapter not initialized"}`, http.StatusServiceUnavailable)
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
