package nerve

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// ════════════════════════════════════════════════════════════
// Nerve Bus HTTP Handlers — /v1/nerve/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /nerve/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	bus := GetBus()
	if bus == nil {
		http.Error(w, `{"error":"nerve bus not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":   bus.Stats(),
		"workers": bus.RegisteredWorkers(),
	})
}

// HandleEvents handles GET /nerve/events?limit=20
func HandleEvents(w http.ResponseWriter, r *http.Request) {
	bus := GetBus()
	if bus == nil {
		http.Error(w, `{"error":"nerve bus not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events := bus.RecentEvents(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// HandleTasks handles GET /nerve/tasks?limit=20
func HandleTasks(w http.ResponseWriter, r *http.Request) {
	bus := GetBus()
	if bus == nil {
		http.Error(w, `{"error":"nerve bus not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	tasks := bus.RecentTasks(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// HandlePublish handles POST /nerve/publish
func HandlePublish(w http.ResponseWriter, r *http.Request) {
	bus := GetBus()
	if bus == nil {
		http.Error(w, `{"error":"nerve bus not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Type    string                 `json:"type"`
		Source  string                 `json:"source"`
		Payload map[string]interface{} `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, `{"error":"type required"}`, http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		req.Source = "api"
	}
	eventID := bus.Publish(req.Type, req.Source, req.Payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"event_id": eventID,
		"type":     req.Type,
	})
}

// HandleDispatch handles POST /nerve/dispatch
func HandleDispatch(w http.ResponseWriter, r *http.Request) {
	bus := GetBus()
	if bus == nil {
		http.Error(w, `{"error":"nerve bus not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		WorkerType string                 `json:"worker_type"`
		Action     string                 `json:"action"`
		Params     map[string]interface{} `json:"params"`
		Priority   string                 `json:"priority"`
		TimeoutSec int                    `json:"timeout_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.WorkerType == "" || req.Action == "" {
		http.Error(w, `{"error":"worker_type and action required"}`, http.StatusBadRequest)
		return
	}

	timeout := 30 * time.Second
	if req.TimeoutSec > 0 {
		timeout = time.Duration(req.TimeoutSec) * time.Second
	}

	taskReq := TaskRequest{
		WorkerType:  req.WorkerType,
		Action:      req.Action,
		Params:      req.Params,
		Priority:    req.Priority,
		RequestedBy: "api",
		Timeout:     timeout,
	}

	result, err := bus.Dispatch(taskReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleWorkers handles GET /nerve/workers
func HandleWorkers(w http.ResponseWriter, r *http.Request) {
	bus := GetBus()
	if bus == nil {
		http.Error(w, `{"error":"nerve bus not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	workers := bus.RegisteredWorkers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workers": workers,
		"count":   len(workers),
	})
}
