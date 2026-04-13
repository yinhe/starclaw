package mutalisk

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Mutalisk Adapter HTTP Handlers — /v1/mutalisk/*
// ════════════════════════════════════════════════════════════

// HandleMutaliskStats handles GET /mutalisk/stats
func HandleMutaliskStats(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  a.Stats(),
		"safety": a.SafetyConfig(),
		"config": map[string]interface{}{
			"mqtt_broker": a.config.MQTTBroker,
		},
	})
}

// HandleMutaliskFleet handles GET /mutalisk/fleet
func HandleMutaliskFleet(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"fleet": a.Fleet(),
		"count": len(a.Fleet()),
	})
}

// HandleMutaliskStatus handles GET /mutalisk/status?sn=xxx
func HandleMutaliskStatus(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	sn := r.URL.Query().Get("sn")
	tel, err := a.Status(sn)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tel)
}

// HandleMutaliskTakeoff handles POST /mutalisk/takeoff
func HandleMutaliskTakeoff(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN     string  `json:"sn"`
		Height float64 `json:"height"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Height == 0 {
		req.Height = 20
	}
	result, err := a.Takeoff(req.SN, req.Height)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskLand handles POST /mutalisk/land
func HandleMutaliskLand(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN string `json:"sn"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	result, err := a.Land(req.SN)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskRTH handles POST /mutalisk/rth
func HandleMutaliskRTH(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN     string  `json:"sn"`
		Height float64 `json:"height"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Height == 0 {
		req.Height = 50
	}
	result, err := a.ReturnHome(req.SN, req.Height)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskGoto handles POST /mutalisk/goto
func HandleMutaliskGoto(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN       string  `json:"sn"`
		Latitude float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Altitude float64 `json:"altitude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Goto(req.SN, req.Latitude, req.Longitude, req.Altitude)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskDRC handles POST /mutalisk/drc
func HandleMutaliskDRC(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN string  `json:"sn"`
		X  float64 `json:"x"` // forward/backward m/s
		Y  float64 `json:"y"` // left/right m/s
		H  float64 `json:"h"` // up/down m/s
		W  float64 `json:"w"` // yaw °/s
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.DRCControl(req.SN, req.X, req.Y, req.H, req.W)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskStop handles POST /mutalisk/stop
func HandleMutaliskStop(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN string `json:"sn"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	result, err := a.Stop(req.SN)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskPhoto handles POST /mutalisk/photo
func HandleMutaliskPhoto(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN       string `json:"sn"`
		CameraID string `json:"camera_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Photo(req.SN, req.CameraID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskVideoStart handles POST /mutalisk/video/start
func HandleMutaliskVideoStart(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN       string `json:"sn"`
		CameraID string `json:"camera_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.VideoStart(req.SN, req.CameraID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskVideoStop handles POST /mutalisk/video/stop
func HandleMutaliskVideoStop(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN       string `json:"sn"`
		CameraID string `json:"camera_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.VideoStop(req.SN, req.CameraID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskGimbal handles POST /mutalisk/gimbal
func HandleMutaliskGimbal(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN    string  `json:"sn"`
		Pitch float64 `json:"pitch"`
		Yaw   float64 `json:"yaw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	result, err := a.GimbalRotate(req.SN, req.Pitch, req.Yaw)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskLive handles POST /mutalisk/live
func HandleMutaliskLive(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN  string `json:"sn"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, `{"error":"url required"}`, http.StatusBadRequest)
		return
	}
	result, err := a.LiveStart(req.SN, req.URL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskWayline handles POST /mutalisk/wayline
func HandleMutaliskWayline(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN        string     `json:"sn"`
		Waypoints []Waypoint `json:"waypoints"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if len(req.Waypoints) == 0 {
		http.Error(w, `{"error":"waypoints required"}`, http.StatusBadRequest)
		return
	}
	result, err := a.Wayline(req.SN, req.Waypoints)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMutaliskSafety handles GET /mutalisk/safety?limit=20
func HandleMutaliskSafety(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
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

// HandleMutaliskRegister handles POST /mutalisk/register
func HandleMutaliskRegister(w http.ResponseWriter, r *http.Request) {
	a := GetAdapter()
	if a == nil {
		http.Error(w, `{"error":"mutalisk adapter not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SN    string     `json:"sn"`
		Model DroneModel `json:"model"`
		Name  string     `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.SN == "" {
		http.Error(w, `{"error":"sn required"}`, http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		req.Model = ModelMavic3E
	}
	a.RegisterDrone(req.SN, req.Model, req.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"sn": req.SN,
	})
}
