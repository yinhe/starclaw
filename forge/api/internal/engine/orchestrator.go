package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
	"starclaw.net/forge/internal/config"
	"starclaw.net/forge/internal/model"
)

// Orchestrator manages Sprint execution: dependency DAG, parallel dispatch, completion callbacks.
type Orchestrator struct {
	DB  *gorm.DB
	Cfg *config.Config
	mu  sync.Mutex
}

// StartSprint activates a sprint and dispatches all ready issues.
func (o *Orchestrator) StartSprint(sprintID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	var sprint model.ForgeSprint
	if err := o.DB.First(&sprint, "id = ?", sprintID).Error; err != nil {
		return fmt.Errorf("sprint not found: %w", err)
	}
	if sprint.Status == "active" {
		return fmt.Errorf("sprint already active")
	}

	now := time.Now()
	o.DB.Model(&sprint).Updates(map[string]interface{}{
		"status":     "active",
		"start_date": &now,
	})

	// Move all backlog issues to todo
	o.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ? AND status = 'backlog'", sprintID).Update("status", "todo")

	// Dispatch ready issues (no unresolved dependencies)
	o.dispatchReady(sprintID)

	o.DB.Create(&model.ForgeActivity{
		ProjectID: sprint.ProjectID,
		Type:      "issue",
		Actor:     "orchestrator",
		Summary:   fmt.Sprintf("Sprint started: %s", sprint.Name),
		Source:    "forge",
	})

	log.Printf("[Orchestrator] Sprint %s started, dispatching ready issues", sprint.Name)
	return nil
}

// OnIssueComplete is called when an issue is marked done. It checks if downstream issues can start.
func (o *Orchestrator) OnIssueComplete(issueID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var issue model.ForgeIssue
	if err := o.DB.First(&issue, "id = ?", issueID).Error; err != nil {
		return
	}
	if issue.SprintID == "" {
		return
	}

	log.Printf("[Orchestrator] Issue %s completed, checking downstream", issue.Key)

	// Update agent status
	if issue.Assignee != "" {
		o.DB.Model(&model.ForgeAgent{}).Where("name = ?", issue.Assignee).Updates(map[string]interface{}{
			"status":        "idle",
			"current_issue": "",
		})
	}

	// Try to dispatch newly unblocked issues
	o.dispatchReady(issue.SprintID)

	// Check if sprint is complete
	o.checkSprintComplete(issue.SprintID)
}

// dispatchReady finds all issues in the sprint that have no unresolved dependencies and are not yet dispatched.
func (o *Orchestrator) dispatchReady(sprintID string) {
	var issues []model.ForgeIssue
	o.DB.Where("sprint_id = ? AND status = 'todo'", sprintID).Find(&issues)

	// Build done set
	var doneIssues []model.ForgeIssue
	o.DB.Where("sprint_id = ? AND status IN ('done','closed')", sprintID).Find(&doneIssues)
	doneSet := make(map[string]bool)
	for _, d := range doneIssues {
		doneSet[d.ID] = true
	}

	// Also count in_progress and review as "started" (don't re-dispatch)
	var startedIssues []model.ForgeIssue
	o.DB.Where("sprint_id = ? AND status IN ('in_progress','review')", sprintID).Find(&startedIssues)
	startedSet := make(map[string]bool)
	for _, s := range startedIssues {
		startedSet[s.ID] = true
	}

	for _, issue := range issues {
		if startedSet[issue.ID] {
			continue
		}

		// Check dependencies
		deps := parseDeps(issue.DependsOn)
		allMet := true
		for _, dep := range deps {
			if !doneSet[dep] {
				allMet = false
				break
			}
		}

		if allMet {
			o.dispatchIssue(&issue)
		}
	}
}

// dispatchIssue assigns an issue to an available agent and updates status.
func (o *Orchestrator) dispatchIssue(issue *model.ForgeIssue) {
	// Find available agent
	agent := o.findAgent(issue)

	now := time.Now()
	updates := map[string]interface{}{
		"status":        "in_progress",
		"dispatched_at": &now,
	}
	if agent != nil {
		updates["assignee"] = agent.Name
		o.DB.Model(agent).Updates(map[string]interface{}{
			"status":        "busy",
			"current_issue": issue.ID,
		})
		log.Printf("[Orchestrator] Dispatched %s → %s (%s)", issue.Key, agent.Name, agent.Type)
	} else {
		log.Printf("[Orchestrator] Dispatched %s (no agent available, awaiting manual pickup)", issue.Key)
	}

	o.DB.Model(issue).Updates(updates)

	o.DB.Create(&model.ForgeActivity{
		ProjectID: issue.ProjectID,
		IssueID:   issue.ID,
		Type:      "issue",
		Actor:     "orchestrator",
		Summary:   fmt.Sprintf("Dispatched %s: %s → %s", issue.Key, issue.Title, issue.Assignee),
		Service:   issue.Service,
		Source:    "forge",
	})
}

// findAgent selects the best available agent for an issue.
func (o *Orchestrator) findAgent(issue *model.ForgeIssue) *model.ForgeAgent {
	var agents []model.ForgeAgent
	o.DB.Where("status = 'idle'").Find(&agents)

	if len(agents) == 0 {
		return nil
	}

	// Prefer agents that match the issue's task_type and service
	for i := range agents {
		a := &agents[i]
		// Check if agent supports the service
		var services []string
		json.Unmarshal([]byte(a.Services), &services)
		for _, s := range services {
			if s == issue.Service {
				return a
			}
		}
	}

	// Fallback: return first idle agent
	return &agents[0]
}

// checkSprintComplete checks if all issues in the sprint are done.
func (o *Orchestrator) checkSprintComplete(sprintID string) {
	var total, done int64
	o.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ?", sprintID).Count(&total)
	o.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ? AND status IN ('done','closed')", sprintID).Count(&done)

	if total > 0 && done == total {
		now := time.Now()
		var sprint model.ForgeSprint
		o.DB.First(&sprint, "id = ?", sprintID)
		o.DB.Model(&sprint).Updates(map[string]interface{}{
			"status":   "completed",
			"end_date": &now,
		})

		// Calculate velocity
		var totalPts struct{ Sum int }
		o.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ?", sprintID).Select("COALESCE(SUM(story_points),0) as sum").Scan(&totalPts)
		o.DB.Model(&sprint).Update("velocity", totalPts.Sum)

		o.DB.Create(&model.ForgeActivity{
			ProjectID: sprint.ProjectID,
			Type:      "issue",
			Actor:     "orchestrator",
			Summary:   fmt.Sprintf("Sprint completed: %s (velocity: %d pts)", sprint.Name, totalPts.Sum),
			Source:    "forge",
		})

		log.Printf("[Orchestrator] Sprint %s completed! Velocity: %d pts", sprint.Name, totalPts.Sum)
	}
}

// Status returns current orchestrator state.
func (o *Orchestrator) Status() map[string]interface{} {
	var activeSprints []model.ForgeSprint
	o.DB.Where("status = 'active'").Find(&activeSprints)

	var busyAgents, idleAgents int64
	o.DB.Model(&model.ForgeAgent{}).Where("status = 'busy'").Count(&busyAgents)
	o.DB.Model(&model.ForgeAgent{}).Where("status = 'idle'").Count(&idleAgents)

	var dispatchedIssues int64
	o.DB.Model(&model.ForgeIssue{}).Where("status = 'in_progress'").Count(&dispatchedIssues)

	return map[string]interface{}{
		"active_sprints":   len(activeSprints),
		"busy_agents":      busyAgents,
		"idle_agents":      idleAgents,
		"dispatched_issues": dispatchedIssues,
	}
}

func parseDeps(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var deps []string
	json.Unmarshal([]byte(s), &deps)
	return deps
}
