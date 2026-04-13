package swarm

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// ════════════════════════════════════════════════════════════
// Formation HTTP Handlers — exposed via Claw /v1/swarm/*
// ════════════════════════════════════════════════════════════

var (
	globalFormation *FormationEngine
	formationOnce   sync.Once
)

// InitFormation initializes the global formation engine
func InitFormation(nodeID, clawID, localAddr string) *FormationEngine {
	formationOnce.Do(func() {
		globalFormation = NewFormationEngine(nodeID, clawID, localAddr)
	})
	return globalFormation
}

// GetFormationEngine returns the global formation engine
func GetFormationEngine() *FormationEngine {
	return globalFormation
}

// HandleFormationStats handles GET /v1/swarm/formation/stats
func HandleFormationStats(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fe.Stats())
}

// HandleFormationList handles GET /v1/swarm/formations
func HandleFormationList(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	formations := fe.ListFormations()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"formations": formations, "count": len(formations)})
}

// HandleFormationCreate handles POST /v1/swarm/formations
func HandleFormationCreate(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = "default"
	}
	f := fe.CreateFormation(req.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

// HandleFormationGet handles GET /v1/swarm/formations?id=...
func HandleFormationGet(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		// List all
		HandleFormationList(w, r)
		return
	}
	f, ok := fe.GetFormation(id)
	if !ok {
		http.Error(w, `{"error":"formation not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

// HandleFormationJoin handles POST /v1/swarm/formations/join
func HandleFormationJoin(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		FormationID string   `json:"formation_id"`
		NodeID      string   `json:"node_id"`
		ClawID      string   `json:"claw_id"`
		Address     string   `json:"address"`
		Agents      []string `json:"agents"`
		Role        string   `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	role := FormationRole(req.Role)
	if role == "" {
		role = RoleWorker
	}
	if err := fe.JoinFormation(req.FormationID, req.NodeID, req.ClawID, req.Address, req.Agents, role); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleFormationDisband handles POST /v1/swarm/formations/disband
func HandleFormationDisband(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		FormationID string `json:"formation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := fe.DisbandFormation(req.FormationID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleSwarmDispatch handles POST /v1/swarm/dispatch — dispatch task via Hive discovery
func HandleSwarmDispatch(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		AgentID  string `json:"agent_id"`
		TaskType string `json:"task_type"`
		Payload  string `json:"payload"`
		Target   string `json:"target"` // optional: direct target address
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	var task *SwarmTask
	var err error
	if req.Target != "" {
		task, err = fe.DispatchTask(r.Context(), req.Target, req.AgentID, req.TaskType, req.Payload)
	} else {
		task, err = fe.DispatchViaHive(r.Context(), req.AgentID, req.TaskType, req.Payload)
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "task": task})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// HandleSwarmTaskIncoming handles POST /v1/swarm/task — receive delegated task from another node
func HandleSwarmTaskIncoming(w http.ResponseWriter, r *http.Request) {
	var task SwarmTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, `{"error":"invalid task payload"}`, http.StatusBadRequest)
		return
	}
	log.Printf("[formation] incoming task %s from %s (agent=%s, type=%s)", task.ID, task.SourceNode, task.AgentID, task.TaskType)

	// TODO: Actually execute the task via local agent runtime
	// For now, acknowledge receipt
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": task.ID,
		"status":  "accepted",
	})
}

// HandleSwarmTaskList handles GET /v1/swarm/tasks?status=...
func HandleSwarmTaskList(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	tasks := fe.ListTasks(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks, "count": len(tasks)})
}

// HandleSwarmTaskComplete handles POST /v1/swarm/tasks/complete
func HandleSwarmTaskComplete(w http.ResponseWriter, r *http.Request) {
	fe := GetFormationEngine()
	if fe == nil {
		http.Error(w, `{"error":"formation engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		TaskID string `json:"task_id"`
		Result string `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if !fe.CompleteTask(req.TaskID, req.Result, req.Error) {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
