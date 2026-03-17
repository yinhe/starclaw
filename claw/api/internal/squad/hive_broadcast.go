package squad

import (
	"log"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/ws"
	"gorm.io/gorm"
)

// HiveBroadcaster aggregates Squad mission state and pushes real-time
// updates to connected clients via the WebSocket hub.
// It runs in a background goroutine alongside the Squad Engine.
type HiveBroadcaster struct {
	db     *gorm.DB
	hub    *ws.Hub
	stopCh chan struct{}
}

// NewHiveBroadcaster creates a new broadcaster.
func NewHiveBroadcaster(db *gorm.DB) *HiveBroadcaster {
	return &HiveBroadcaster{
		db:     db,
		hub:    ws.GetHub(),
		stopCh: make(chan struct{}),
	}
}

// Start begins the broadcast loop (call in a goroutine).
func (b *HiveBroadcaster) Start() {
	log.Println("[hive-broadcast] started")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.broadcastAll()
		}
	}
}

// Stop shuts down the broadcaster.
func (b *HiveBroadcaster) Stop() {
	close(b.stopCh)
}

// HiveNodeStatus represents a single node's status for the dashboard.
type HiveNodeStatus struct {
	NodeID      string `json:"node_id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CurrentTask string `json:"current_task"`
	Progress    int    `json:"progress"`
	Branch      string `json:"branch"`
	LastAction  string `json:"last_action"`
}

// HiveSprintStatus represents sprint progress for the dashboard.
type HiveSprintStatus struct {
	MissionID    string `json:"mission_id"`
	MissionTitle string `json:"mission_title"`
	SprintNumber int    `json:"sprint_number"`
	SprintGoal   string `json:"sprint_goal"`
	SprintStatus string `json:"sprint_status"`
	DoneSteps    int    `json:"done_steps"`
	TotalSteps   int    `json:"total_steps"`
	Progress     int    `json:"progress"`
}

// HiveStepUpdate represents a mission step update for the dashboard.
type HiveStepUpdate struct {
	StepID     string `json:"step_id"`
	MissionID  string `json:"mission_id"`
	Task       string `json:"task"`
	Status     string `json:"status"`
	Branch     string `json:"branch"`
	TargetNode string `json:"target_node"`
	Sequence   int    `json:"sequence"`
	Output     string `json:"output,omitempty"`
}

// broadcastAll aggregates current state and pushes to all connected clients.
func (b *HiveBroadcaster) broadcastAll() {
	// Find all active missions (executing or recently completed)
	var missions []model.Mission
	b.db.Where("status IN ?", []string{"executing", "completed"}).
		Order("updated_at DESC").Limit(5).Find(&missions)

	if len(missions) == 0 {
		return
	}

	for _, mission := range missions {
		// Get steps for this mission
		var steps []model.MissionStep
		b.db.Where("mission_id = ?", mission.ID).Order("sequence ASC").Find(&steps)

		// Build step updates
		stepUpdates := make([]HiveStepUpdate, 0, len(steps))
		for _, s := range steps {
			taskPreview := s.Task
			if len(taskPreview) > 120 {
				taskPreview = taskPreview[:120] + "..."
			}
			outputPreview := s.Output
			if len(outputPreview) > 200 {
				outputPreview = outputPreview[:200] + "..."
			}
			stepUpdates = append(stepUpdates, HiveStepUpdate{
				StepID:     s.ID,
				MissionID:  s.MissionID,
				Task:       taskPreview,
				Status:     s.Status,
				Branch:     s.Branch,
				TargetNode: s.TargetNode,
				Sequence:   s.Sequence,
				Output:     outputPreview,
			})
		}

		// Sprint info
		var sprint model.Sprint
		sprintStatus := HiveSprintStatus{
			MissionID:    mission.ID,
			MissionTitle: mission.Title,
			TotalSteps:   mission.TotalSteps,
			DoneSteps:    int(mission.DoneSteps),
		}

		if err := b.db.Where("mission_id = ?", mission.ID).
			Order("number DESC").First(&sprint).Error; err == nil {
			sprintStatus.SprintNumber = sprint.Number
			sprintStatus.SprintGoal = sprint.Goal
			sprintStatus.SprintStatus = sprint.Status
			sprintStatus.DoneSteps = int(sprint.DoneSteps)
			sprintStatus.TotalSteps = int(sprint.TotalSteps)
		}

		if sprintStatus.TotalSteps > 0 {
			sprintStatus.Progress = sprintStatus.DoneSteps * 100 / sprintStatus.TotalSteps
		}

		// Push to the mission owner
		if mission.UserID != "" {
			b.hub.SendToUser(mission.UserID, ws.EventHiveStepUpdate, map[string]interface{}{
				"mission_id": mission.ID,
				"steps":      stepUpdates,
			})

			b.hub.SendToUser(mission.UserID, ws.EventHiveSprint, sprintStatus)
		}
	}
}
