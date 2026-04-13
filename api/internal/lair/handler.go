package lair

import (
	"encoding/json"
	"net/http"
)

// ════════════════════════════════════════════════════════════
// Lair v1 HTTP Handlers — /v1/lair/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /lair/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  e.Stats(),
		"config": e.Config(),
	})
}

// HandleRegisterNode handles POST /lair/nodes
func HandleRegisterNode(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name         string            `json:"name"`
		Address      string            `json:"address"`
		BrooddPort   int               `json:"broodd_port"`
		MaxInstances int               `json:"max_instances"`
		Labels       map[string]string `json:"labels"`
		Resources    NodeResources     `json:"resources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	if req.BrooddPort <= 0 {
		req.BrooddPort = 6601
	}
	if req.MaxInstances <= 0 {
		req.MaxInstances = 10
	}
	node, err := e.RegisterNode(req.Name, req.Address, req.BrooddPort, req.MaxInstances, req.Labels, req.Resources)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// HandleListNodes handles GET /lair/nodes?status=online
func HandleListNodes(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	nodes := e.ListNodes(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

// HandleHeartbeat handles POST /lair/heartbeat
func HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		NodeID    string         `json:"node_id"`
		Resources *NodeResources `json:"resources"`
		Instances int            `json:"instances"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.Heartbeat(req.NodeID, req.Resources, req.Instances); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleSetNodeStatus handles POST /lair/nodes/status
func HandleSetNodeStatus(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		NodeID string     `json:"node_id"`
		Status NodeStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.SetNodeStatus(req.NodeID, req.Status); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleRemoveNode handles POST /lair/nodes/remove
func HandleRemoveNode(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.RemoveNode(req.NodeID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleDeploy handles POST /lair/deploy
func HandleDeploy(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name     string `json:"name"`
		AgentID  string `json:"agent_id"`
		Version  string `json:"version"`
		NodeID   string `json:"node_id"`
		Image    string `json:"image"`
		Command  string `json:"command"`
		Replicas int    `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	dep, err := e.Deploy(req.Name, req.AgentID, req.Version, req.NodeID, req.Image, req.Command, req.Replicas)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dep)
}

// HandleListDeployments handles GET /lair/deployments?status=running
func HandleListDeployments(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	deps := e.ListDeployments(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deployments": deps,
		"count":       len(deps),
	})
}

// HandleCreateRollout handles POST /lair/rollout
func HandleCreateRollout(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name      string   `json:"name"`
		AgentID   string   `json:"agent_id"`
		Version   string   `json:"version"`
		Strategy  string   `json:"strategy"`
		NodeIDs   []string `json:"node_ids"`
		BatchSize int      `json:"batch_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	rollout, err := e.CreateRollout(req.Name, req.AgentID, req.Version, req.Strategy, req.NodeIDs, req.BatchSize)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rollout)
}

// HandleListRollouts handles GET /lair/rollouts
func HandleListRollouts(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"lair engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	rollouts := e.ListRollouts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rollouts": rollouts,
		"count":    len(rollouts),
	})
}
