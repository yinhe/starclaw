package broodmind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ════════════════════════════════════════════════════════════
// BroodMind v3 — Planner HTTP Handlers (10 API)
// ════════════════════════════════════════════════════════════

// HandlePlannerStats handles GET /broodmind/planner/stats
func HandlePlannerStats(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instance.Planner.Stats())
}

// HandlePlannerCreateGoal handles POST /broodmind/planner/goal
func HandlePlannerCreateGoal(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Context     string   `json:"context"`
		AgentID     string   `json:"agent_id"`
		UserID      string   `json:"user_id"`
		Constraints []string `json:"constraints"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
		return
	}

	g := instance.Planner.CreateGoal(req.Title, req.Description, req.Context, req.AgentID, req.UserID, req.Constraints)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

// HandlePlannerGetGoal handles GET /broodmind/planner/goal/:id
func HandlePlannerGetGoal(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	id := parts[len(parts)-1]
	g := instance.Planner.GetGoal(id)
	if g == nil {
		http.Error(w, `{"error":"goal not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

// HandlePlannerListGoals handles GET /broodmind/planner/goals?status=...&limit=20
func HandlePlannerListGoals(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			limit = n
		}
	}
	goals := instance.Planner.ListGoals(status, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"goals": goals, "count": len(goals)})
}

// HandlePlannerDecompose handles POST /broodmind/planner/decompose
func HandlePlannerDecompose(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		GoalID string     `json:"goal_id"`
		Steps  []PlanStep `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.GoalID == "" || len(req.Steps) == 0 {
		http.Error(w, `{"error":"goal_id and steps required"}`, http.StatusBadRequest)
		return
	}

	if err := instance.Planner.Decompose(req.GoalID, req.Steps); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	g := instance.Planner.GetGoal(req.GoalID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

// HandlePlannerSelectResources handles POST /broodmind/planner/resources
func HandlePlannerSelectResources(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		GoalID    string              `json:"goal_id"`
		StepID    string              `json:"step_id"`
		Resources []ResourceSelection `json:"resources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := instance.Planner.SelectResources(req.GoalID, req.StepID, req.Resources); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandlePlannerStart handles POST /broodmind/planner/start
func HandlePlannerStart(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		GoalID string `json:"goal_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	ready, err := instance.Planner.StartExecution(req.GoalID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ready_steps": ready, "count": len(ready)})
}

// HandlePlannerCompleteStep handles POST /broodmind/planner/step/complete
func HandlePlannerCompleteStep(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		GoalID string                 `json:"goal_id"`
		StepID string                 `json:"step_id"`
		Output map[string]interface{} `json:"output"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	ready, err := instance.Planner.CompleteStep(req.GoalID, req.StepID, req.Output)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	g := instance.Planner.GetGoal(req.GoalID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"goal":        g,
		"ready_steps": ready,
	})
}

// HandlePlannerFailStep handles POST /broodmind/planner/step/fail
func HandlePlannerFailStep(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		GoalID string `json:"goal_id"`
		StepID string `json:"step_id"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := instance.Planner.FailStep(req.GoalID, req.StepID, req.Error); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	g := instance.Planner.GetGoal(req.GoalID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

// HandlePlannerReflect handles POST /broodmind/planner/reflect
func HandlePlannerReflect(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.Planner == nil {
		http.Error(w, `{"error":"planner not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		GoalID       string   `json:"goal_id"`
		Success      bool     `json:"success"`
		Summary      string   `json:"summary"`
		Lessons      []string `json:"lessons"`
		Improvements []string `json:"improvements"`
		Quality      float64  `json:"quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := instance.Planner.Reflect(req.GoalID, req.Success, req.Summary, req.Lessons, req.Improvements, req.Quality); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	g := instance.Planner.GetGoal(req.GoalID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}
