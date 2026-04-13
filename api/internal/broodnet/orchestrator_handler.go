package broodnet

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ════════════════════════════════════════════════════════════
// SwarmOrchestrator HTTP Handlers — /v1/broodnet/workflows/*
// ════════════════════════════════════════════════════════════

// HandleOrchStats handles GET /broodnet/orch/stats
func HandleOrchStats(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o.Stats())
}

// HandleOrchPlan handles POST /broodnet/workflows
func HandleOrchPlan(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		FormationID string         `json:"formation_id"`
		SubTasks    []SubTaskInput `json:"sub_tasks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	wf, err := o.PlanWorkflow(req.Name, req.Description, req.FormationID, req.SubTasks)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

// HandleOrchList handles GET /broodnet/workflows?status=...
func HandleOrchList(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	status := WorkflowStatus(r.URL.Query().Get("status"))
	workflows := o.ListWorkflows(status, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
	})
}

// HandleOrchGet handles GET /broodnet/workflows/:id
func HandleOrchGet(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	id := parts[len(parts)-1]
	wf := o.GetWorkflow(id)
	if wf == nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

// HandleOrchStart handles POST /broodnet/workflows/start
func HandleOrchStart(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		WorkflowID string `json:"workflow_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := o.StartWorkflow(req.WorkflowID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	wf := o.GetWorkflow(req.WorkflowID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

// HandleOrchAdvance handles POST /broodnet/workflows/advance
func HandleOrchAdvance(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		WorkflowID string        `json:"workflow_id"`
		SubTaskID  string        `json:"sub_task_id"`
		Status     SubTaskStatus `json:"status"`
		Result     string        `json:"result"`
		Error      string        `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := o.AdvanceSubTask(req.WorkflowID, req.SubTaskID, req.Status, req.Result, req.Error); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	wf := o.GetWorkflow(req.WorkflowID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

// HandleOrchReady handles GET /broodnet/workflows/:id/ready
func HandleOrchReady(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	// expect .../workflows/{id}/ready
	if len(parts) < 3 {
		http.Error(w, `{"error":"workflow id required"}`, http.StatusBadRequest)
		return
	}
	id := parts[len(parts)-2]

	ready := o.ReadySubTasks(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ready": ready,
		"count": len(ready),
	})
}

// HandleOrchCancel handles POST /broodnet/workflows/cancel
func HandleOrchCancel(w http.ResponseWriter, r *http.Request) {
	o := GetOrchestrator()
	if o == nil {
		http.Error(w, `{"error":"orchestrator not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		WorkflowID string `json:"workflow_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := o.CancelWorkflow(req.WorkflowID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
