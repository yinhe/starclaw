package chitin

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Chitin v1 HTTP Handlers — /v1/chitin/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /chitin/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  e.Stats(),
		"config": e.Config(),
	})
}

// HandleCreateInstance handles POST /chitin/instances
func HandleCreateInstance(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name          string            `json:"name"`
		AgentID       string            `json:"agent_id"`
		Mode          RuntimeMode       `json:"mode"`
		Image         string            `json:"image"`
		Command       string            `json:"command"`
		Env           map[string]string `json:"env"`
		Limits        *ResourceLimits   `json:"limits"`
		RestartPolicy RestartPolicy     `json:"restart_policy"`
		HealthCheck   string            `json:"health_check"`
		Port          int               `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	inst, err := e.CreateInstance(req.Name, req.AgentID, req.Mode, req.Image, req.Command, req.Env, req.Limits, req.RestartPolicy, req.HealthCheck, req.Port)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(inst)
}

// HandleListInstances handles GET /chitin/instances?status=running
func HandleListInstances(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	instances := e.ListInstances(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
	})
}

// HandleGetInstance handles GET /chitin/instances/detail?id=xxx
func HandleGetInstance(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	instID := r.URL.Query().Get("id")
	if instID == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	inst, err := e.GetInstance(instID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inst)
}

// HandleStartInstance handles POST /chitin/instances/start
func HandleStartInstance(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.StartInstance(req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": req.ID})
}

// HandleStopInstance handles POST /chitin/instances/stop
func HandleStopInstance(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.StopInstance(req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": req.ID})
}

// HandleRestartInstance handles POST /chitin/instances/restart
func HandleRestartInstance(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.RestartInstance(req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": req.ID})
}

// HandleDestroyInstance handles POST /chitin/instances/destroy
func HandleDestroyInstance(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.DestroyInstance(req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": req.ID})
}

// HandleHealthCheck handles POST /chitin/health
func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Healthy bool   `json:"healthy"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.RecordHealthCheck(req.ID, req.Healthy, req.Message); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleEvents handles GET /chitin/events?limit=20
func HandleEvents(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"chitin engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events := e.ListEvents(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}
