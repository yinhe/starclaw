package abathur

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Abathur v1 HTTP Handlers — /v1/abathur/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /abathur/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":        e.Stats(),
		"config":       e.Config(),
		"worker_stats": e.WorkerStats(),
	})
}

// HandleListPlans handles GET /abathur/plans?status=xxx
func HandleListPlans(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	plans := e.ListPlans(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"plans": plans,
		"count": len(plans),
	})
}

// HandleGetPlan handles GET /abathur/plans/detail?id=xxx
func HandleGetPlan(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	planID := r.URL.Query().Get("id")
	if planID == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	plan, err := e.GetPlan(planID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// HandleCreatePlan handles POST /abathur/plans
func HandleCreatePlan(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Title           string     `json:"title"`
		Description     string     `json:"description"`
		Goals           []string   `json:"goals"`
		Tasks           []PlanTask `json:"tasks"`
		SuccessCriteria []string   `json:"success_criteria"`
		RollbackPlan    string     `json:"rollback_plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
		return
	}
	plan := e.CreatePlan(req.Title, req.Description, req.Goals, req.Tasks, req.SuccessCriteria, req.RollbackPlan)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(plan)
}

// HandleApprovePlan handles POST /abathur/plans/approve
func HandleApprovePlan(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		PlanID   string `json:"plan_id"`
		Approver string `json:"approver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Approver == "" {
		req.Approver = "human"
	}
	if err := e.ApprovePlan(req.PlanID, req.Approver); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "plan_id": req.PlanID})
}

// HandleListSprints handles GET /abathur/sprints?status=xxx
func HandleListSprints(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	sprints := e.ListSprints(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sprints": sprints,
		"count":   len(sprints),
	})
}

// HandleCreateSprint handles POST /abathur/sprints
func HandleCreateSprint(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		PlanID string `json:"plan_id"`
		Title  string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	sprint, err := e.CreateSprint(req.PlanID, req.Title)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sprint)
}

// HandleGetSprint handles GET /abathur/sprints/detail?id=xxx
func HandleGetSprint(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	sprintID := r.URL.Query().Get("id")
	if sprintID == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	sprint, err := e.GetSprint(sprintID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sprint)
}

// HandleDispatchTask handles POST /abathur/tasks
func HandleDispatchTask(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SprintID    string       `json:"sprint_id"`
		Title       string       `json:"title"`
		Description string       `json:"description"`
		Assignee    WorkerType   `json:"assignee"`
		Priority    TaskPriority `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Priority == "" {
		req.Priority = PriorityP2
	}
	task, err := e.DispatchTask(req.SprintID, req.Title, req.Description, req.Assignee, req.Priority)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// HandleUpdateTask handles PATCH /abathur/tasks
func HandleUpdateTask(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		TaskID string     `json:"task_id"`
		Status TaskStatus `json:"status"`
		Result string     `json:"result"`
		Error  string     `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.UpdateTask(req.TaskID, req.Status, req.Result, req.Error); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "task_id": req.TaskID, "status": req.Status})
}

// HandleHotfix handles POST /abathur/hotfix
func HandleHotfix(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Severity    HotfixSeverity `json:"severity"`
		Source      string         `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Severity == "" {
		req.Severity = SevP2
	}
	hf := e.TriageHotfix(req.Title, req.Description, req.Severity, req.Source)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(hf)
}

// HandleListHotfixes handles GET /abathur/hotfixes
func HandleListHotfixes(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	hfs := e.ListHotfixes()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hotfixes": hfs,
		"count":    len(hfs),
	})
}

// HandleCapability handles GET /abathur/capability
func HandleCapability(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	gaps := e.AssessCapability()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"gaps":  gaps,
		"count": len(gaps),
	})
}

// HandleDistillLesson handles POST /abathur/lessons
func HandleDistillLesson(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SprintID string `json:"sprint_id"`
		Type     string `json:"type"`
		Content  string `json:"content"`
		Impact   string `json:"impact"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `{"error":"content required"}`, http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = "observation"
	}
	lesson := e.DistillLesson(req.SprintID, req.Type, req.Content, req.Impact)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(lesson)
}

// HandleListLessons handles GET /abathur/lessons?limit=20
func HandleListLessons(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"abathur engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	lessons := e.ListLessons(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"lessons": lessons,
		"count":   len(lessons),
	})
}
