package forge

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// Bridge syncs Squad lifecycle events into Forge project management.
// Called by squad engine and handlers to keep Issues in sync with Missions.

// OnMissionCreated creates an epic-type Issue in the squad's Forge project.
// If no project exists for the squad, one is auto-created.
func OnMissionCreated(db *gorm.DB, mission model.Mission) {
	project := ensureForgeProject(db, mission.SquadID, mission.UserID)
	if project == nil {
		return
	}

	// Auto-increment issue number
	var maxNum int
	db.Model(&model.ForgeIssue{}).Where("project_id = ?", project.ID).
		Select("COALESCE(MAX(number), 0)").Scan(&maxNum)

	issue := model.ForgeIssue{
		ProjectID:    project.ID,
		Number:       maxNum + 1,
		Title:        mission.Title,
		Body:         mission.Goal,
		Type:         "epic",
		Priority:     "high",
		Status:       "open",
		ReporterNode: mission.CaptainNode,
		MissionID:    mission.ID,
		Labels:       "mission,auto",
	}
	if err := db.Create(&issue).Error; err != nil {
		log.Printf("[forge-bridge] failed to create epic for mission %s: %v", mission.ID, err)
		return
	}
	log.Printf("[forge-bridge] mission %s → epic #%d in project %s", mission.ID, issue.Number, project.Name)
}

// OnMissionStepCreated creates a task-type Issue linked to the mission's epic.
func OnMissionStepCreated(db *gorm.DB, step model.MissionStep, mission model.Mission) {
	project := findProjectBySquad(db, mission.SquadID)
	if project == nil {
		return
	}

	// Find the epic issue for this mission
	var epic model.ForgeIssue
	db.Where("project_id = ? AND mission_id = ? AND type = ?", project.ID, mission.ID, "epic").First(&epic)

	var maxNum int
	db.Model(&model.ForgeIssue{}).Where("project_id = ?", project.ID).
		Select("COALESCE(MAX(number), 0)").Scan(&maxNum)

	taskTitle := step.Task
	if len(taskTitle) > 200 {
		taskTitle = taskTitle[:200] + "..."
	}

	issue := model.ForgeIssue{
		ProjectID:    project.ID,
		Number:       maxNum + 1,
		Title:        fmt.Sprintf("[Step %d] %s", step.Sequence, taskTitle),
		Body:         step.Task,
		Type:         "task",
		Priority:     "medium",
		Status:       "open",
		AssigneeNode: step.TargetNode,
		MissionID:    mission.ID,
		Branch:       step.Branch,
		Labels:       "step,auto",
	}
	if epic.ID != "" {
		issue.Body = fmt.Sprintf("Epic: #%d %s\n\n%s", epic.Number, epic.Title, step.Task)
	}
	db.Create(&issue)
}

// OnStepStatusChanged syncs step status to the corresponding Forge Issue.
func OnStepStatusChanged(db *gorm.DB, step model.MissionStep, mission model.Mission) {
	project := findProjectBySquad(db, mission.SquadID)
	if project == nil {
		return
	}

	// Map step status → issue status
	statusMap := map[string]string{
		"pending":    "open",
		"dispatched": "in_progress",
		"running":    "in_progress",
		"reviewing":  "review",
		"done":       "done",
		"failed":     "closed",
	}
	issueStatus, ok := statusMap[step.Status]
	if !ok {
		return
	}

	// Find the step's issue by branch match
	var issue model.ForgeIssue
	if err := db.Where("project_id = ? AND mission_id = ? AND branch = ?",
		project.ID, mission.ID, step.Branch).First(&issue).Error; err != nil {
		return
	}

	db.Model(&issue).Update("status", issueStatus)
}

// OnMissionCompleted closes all open issues for the mission and marks the epic done.
func OnMissionCompleted(db *gorm.DB, mission model.Mission) {
	project := findProjectBySquad(db, mission.SquadID)
	if project == nil {
		return
	}

	// Close epic
	db.Model(&model.ForgeIssue{}).
		Where("project_id = ? AND mission_id = ? AND type = ? AND status != ?",
			project.ID, mission.ID, "epic", "done").
		Update("status", "done")

	// Close remaining open step issues
	db.Model(&model.ForgeIssue{}).
		Where("project_id = ? AND mission_id = ? AND status IN ?",
			project.ID, mission.ID, []string{"open", "in_progress", "review"}).
		Update("status", "done")

	log.Printf("[forge-bridge] mission %s completed → all issues closed", mission.ID)
}

// ensureForgeProject finds or creates a Forge project for a squad.
func ensureForgeProject(db *gorm.DB, squadID, userID string) *model.ForgeProject {
	if squadID == "" {
		return nil
	}

	var project model.ForgeProject
	if err := db.Where("squad_id = ? AND status = ?", squadID, "active").First(&project).Error; err == nil {
		return &project
	}

	// Auto-create from squad
	var squad model.Squad
	if err := db.Where("id = ?", squadID).First(&squad).Error; err != nil {
		return nil
	}

	defaultColumns, _ := json.Marshal([]gin.H{
		{"name": "Backlog", "status": "open"},
		{"name": "In Progress", "status": "in_progress"},
		{"name": "Review", "status": "review"},
		{"name": "Done", "status": "done"},
	})

	project = model.ForgeProject{
		Name:        squad.Name + " — Forge",
		Description: "Auto-created project for squad: " + squad.Name,
		SquadID:     squadID,
		UserID:      userID,
		Visibility:  "team",
		Tags:        "squad,auto",
	}
	db.Create(&project)

	db.Create(&model.ForgeBoard{
		ProjectID: project.ID,
		Name:      "Sprint Board",
		Columns:   string(defaultColumns),
		IsDefault: true,
	})

	log.Printf("[forge-bridge] auto-created project '%s' for squad %s", project.Name, squadID)
	return &project
}

func findProjectBySquad(db *gorm.DB, squadID string) *model.ForgeProject {
	if squadID == "" {
		return nil
	}
	var project model.ForgeProject
	if err := db.Where("squad_id = ? AND status = ?", squadID, "active").First(&project).Error; err != nil {
		return nil
	}
	return &project
}
