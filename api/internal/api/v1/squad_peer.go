package v1

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"gorm.io/gorm"
)

// SquadPeerHandler handles cross-node Squad protocol endpoints.
// These are called by remote Claw nodes via Nydus relay or direct connection.
type SquadPeerHandler struct {
	db       *gorm.DB
	identity *node.Identity
}

// NewSquadPeerHandler creates the handler.
func NewSquadPeerHandler(db *gorm.DB, identity *node.Identity) *SquadPeerHandler {
	return &SquadPeerHandler{db: db, identity: identity}
}

// ── Invite ──

// SquadInviteRequest is sent by a captain node to invite this node.
type SquadInviteRequest struct {
	SquadID     string `json:"squad_id"`
	SquadName   string `json:"squad_name"`
	CaptainNode string `json:"captain_node"` // claw:xxx of captain
	Specialty   string `json:"specialty"`
	Signature   string `json:"signature"`
}

// HandleInvite receives a squad invitation from a remote captain node.
func (h *SquadPeerHandler) HandleInvite(c *gin.Context) {
	var req SquadInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[squad/peer] received invite from %s for squad %s (%s)",
		req.CaptainNode, req.SquadName, req.SquadID)

	// Auto-accept for now (future: require user confirmation)
	// Store as a local squad membership record
	squad := model.Squad{
		ID:          req.SquadID,
		Name:        req.SquadName,
		CaptainNode: req.CaptainNode,
		Status:      "active",
	}
	h.db.Where("id = ?", req.SquadID).FirstOrCreate(&squad)

	member := model.SquadMember{
		SquadID:   req.SquadID,
		NodeID:    h.identity.NodeID,
		Role:      "member",
		Specialty: req.Specialty,
		Status:    "online",
		JoinedAt:  time.Now(),
	}
	h.db.Where("squad_id = ? AND node_id = ?", req.SquadID, h.identity.NodeID).FirstOrCreate(&member)

	c.JSON(http.StatusOK, gin.H{
		"status":  "accepted",
		"node_id": h.identity.NodeID,
	})
}

// ── Agent Discovery ──

// HandleAgents returns the agents available on this node for squad collaboration.
func (h *SquadPeerHandler) HandleAgents(c *gin.Context) {
	// Return a summary of local agents (not full details for security)
	type AgentSummary struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Specialty   string `json:"specialty"`
		Available   bool   `json:"available"`
	}

	var agents []model.Agent
	h.db.Select("id, name, description").Find(&agents)

	summaries := make([]AgentSummary, 0, len(agents))
	for _, ag := range agents {
		summaries = append(summaries, AgentSummary{
			ID:          ag.ID,
			Name:        ag.Name,
			Description: ag.Description,
			Available:   true,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"node_id": h.identity.NodeID,
		"agents":  summaries,
	})
}

// ── Execute (receive delegated task) ──

// SquadExecuteRequest is sent by the captain to delegate a mission step.
type SquadExecuteRequest struct {
	MissionID   string `json:"mission_id"`
	StepID      string `json:"step_id"`
	Task        string `json:"task"`
	Context     string `json:"context"`      // upstream outputs as context
	AgentName   string `json:"agent_name"`   // preferred agent to use
	CallbackURL string `json:"callback_url"` // where to POST result
	RepoPath    string `json:"repo_path"`    // Git bare repo path (file:// for local)
	Branch      string `json:"branch"`       // Git branch for this step
	Signature   string `json:"signature"`
}

// HandleExecute receives a delegated mission step from the captain node.
func (h *SquadPeerHandler) HandleExecute(c *gin.Context) {
	var req SquadExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[squad/peer] received execute: mission=%s step=%s agent=%s task=%.100s",
		req.MissionID, req.StepID, req.AgentName, req.Task)

	// Find the best matching local agent
	var agent model.Agent
	if req.AgentName != "" {
		h.db.Where("name LIKE ?", "%"+req.AgentName+"%").First(&agent)
	}
	if agent.ID == "" {
		// Fallback to first agent
		h.db.First(&agent)
	}

	if agent.ID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "no agent available",
			"step_id": req.StepID,
		})
		return
	}

	// Build goal with Git context if available
	var goalParts []string
	if req.Context != "" {
		goalParts = append(goalParts, "## 上下文\n"+req.Context)
	}
	if req.RepoPath != "" && req.Branch != "" {
		goalParts = append(goalParts, "## Git 工作区\n"+
			"- 仓库: "+req.RepoPath+"\n"+
			"- 分支: "+req.Branch+"\n\n"+
			"请使用 code 工具编写代码，使用 git 工具提交和推送代码。\n"+
			"工作流程: git clone → git checkout → 编写代码 → git add → git commit → git push")
	}
	goalParts = append(goalParts, "## 任务\n"+req.Task)
	goal := ""
	for i, p := range goalParts {
		if i > 0 {
			goal += "\n\n"
		}
		goal += p
	}

	task := model.Task{
		AgentID:  agent.ID,
		Title:    "🤝 战队任务: " + req.Task[:min(100, len(req.Task))],
		Goal:     goal,
		Status:   model.TaskStatusPending,
		Priority: model.TaskPriorityNormal,
	}
	// Set user_id from the agent's owner
	if agent.UserID != "" {
		task.UserID = agent.UserID
	}

	if err := h.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	// Store callback info for when the task completes
	callbackInfo := SquadCallbackInfo{
		StepID:      req.StepID,
		MissionID:   req.MissionID,
		CallbackURL: req.CallbackURL,
		TaskID:      task.ID,
	}
	callbackJSON, _ := json.Marshal(callbackInfo)

	// Store in task metadata (use parent_task_id field as a holder, or a dedicated field)
	// For now, store as a notification that the squad engine watches
	h.db.Create(&model.Notification{
		UserID:  task.UserID,
		Type:    "squad_callback",
		Title:   "Squad callback pending",
		Content: string(callbackJSON),
	})

	log.Printf("[squad/peer] created local task %s for step %s (agent: %s)", task.ID, req.StepID, agent.Name)

	c.JSON(http.StatusOK, gin.H{
		"status":   "accepted",
		"step_id":  req.StepID,
		"task_id":  task.ID,
		"agent_id": agent.ID,
	})
}

// SquadCallbackInfo is stored locally to track pending callbacks.
type SquadCallbackInfo struct {
	StepID      string `json:"step_id"`
	MissionID   string `json:"mission_id"`
	CallbackURL string `json:"callback_url"`
	TaskID      string `json:"task_id"`
}

// ── Callback (receive result from remote node) ──

// SquadCallbackRequest is sent by a member node when it finishes executing a step.
type SquadCallbackRequest struct {
	StepID    string `json:"step_id"`
	MissionID string `json:"mission_id"`
	Output    string `json:"output"`
	Status    string `json:"status"` // done / failed
	ErrorMsg  string `json:"error_msg"`
	NodeID    string `json:"node_id"`
	Signature string `json:"signature"`
}

// HandleCallback receives execution results from a member node (captain-side).
func (h *SquadPeerHandler) HandleCallback(c *gin.Context) {
	var req SquadCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[squad/peer] callback: step=%s mission=%s status=%s from=%s",
		req.StepID, req.MissionID, req.Status, req.NodeID)

	// Update the mission step
	now := time.Now()
	updates := map[string]interface{}{
		"status":       req.Status,
		"output":       req.Output,
		"completed_at": &now,
	}
	if req.ErrorMsg != "" {
		updates["error_msg"] = req.ErrorMsg
	}

	result := h.db.Model(&model.MissionStep{}).Where("id = ? AND mission_id = ?", req.StepID, req.MissionID).Updates(updates)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "step not found"})
		return
	}

	// Update mission progress
	if req.Status == "done" {
		h.db.Model(&model.Mission{}).Where("id = ?", req.MissionID).
			Update("done_steps", gorm.Expr("done_steps + 1"))
	}

	// Check if all steps are done → complete the mission
	var mission model.Mission
	if err := h.db.Where("id = ?", req.MissionID).First(&mission).Error; err == nil {
		if mission.DoneSteps+1 >= mission.TotalSteps && mission.TotalSteps > 0 {
			// All steps done — mark as reviewing (SquadEngine will finalize)
			h.db.Model(&mission).Updates(map[string]interface{}{
				"status":     "reviewing",
				"done_steps": mission.DoneSteps + 1,
			})
			log.Printf("[squad/peer] mission %s all steps complete → reviewing", req.MissionID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

// ── Heartbeat ──

// SquadHeartbeatRequest is sent periodically by squad members.
type SquadHeartbeatRequest struct {
	SquadID     string `json:"squad_id"`
	NodeID      string `json:"node_id"`
	Status      string `json:"status"` // online / busy / offline
	ActiveTasks int    `json:"active_tasks"`
	AgentCount  int    `json:"agent_count"`
}

// StartCallbackWatcher runs a background loop that watches for completed squad tasks
// and sends callbacks to the captain node.
func (h *SquadPeerHandler) StartCallbackWatcher() {
	go func() {
		httpC := &http.Client{Timeout: 15 * time.Second}
		for {
			time.Sleep(5 * time.Second)

			// Find squad_callback notifications
			var notifications []model.Notification
			h.db.Where("type = ?", "squad_callback").Limit(20).Find(&notifications)

			for _, n := range notifications {
				var info SquadCallbackInfo
				if err := json.Unmarshal([]byte(n.Content), &info); err != nil {
					h.db.Delete(&n)
					continue
				}

				// Check if the task is done
				var task model.Task
				if err := h.db.Where("id = ?", info.TaskID).First(&task).Error; err != nil {
					continue
				}

				if task.Status != model.TaskStatusCompleted && task.Status != model.TaskStatusFailed {
					continue // still running
				}

				// Send callback to captain
				status := "done"
				errorMsg := ""
				output := task.Result
				if task.Status == model.TaskStatusFailed {
					status = "failed"
					errorMsg = task.ErrorMsg
				}

				if info.CallbackURL != "" {
					callbackBody := SquadCallbackRequest{
						StepID:    info.StepID,
						MissionID: info.MissionID,
						Output:    output,
						Status:    status,
						ErrorMsg:  errorMsg,
						NodeID:    h.identity.NodeID,
					}
					bodyBytes, _ := json.Marshal(callbackBody)

					resp, err := httpC.Post(info.CallbackURL, "application/json", bytes.NewReader(bodyBytes))
					if err != nil {
						log.Printf("[squad/peer] callback failed for step %s: %v", info.StepID, err)
						continue // retry next loop
					}
					resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						log.Printf("[squad/peer] callback sent for step %s (status: %s)", info.StepID, status)
						h.db.Delete(&n) // remove processed notification
					} else {
						log.Printf("[squad/peer] callback returned %d for step %s", resp.StatusCode, info.StepID)
					}
				} else {
					// No callback URL, just clean up
					log.Printf("[squad/peer] no callback URL for step %s, cleaning up", info.StepID)
					h.db.Delete(&n)
				}
			}
		}
	}()
	log.Println("[squad/peer] callback watcher started")
}

// HandleHeartbeat receives a heartbeat from a squad member, updating status.
func (h *SquadPeerHandler) HandleHeartbeat(c *gin.Context) {
	var req SquadHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&model.SquadMember{}).
		Where("squad_id = ? AND node_id = ?", req.SquadID, req.NodeID).
		Updates(map[string]interface{}{
			"status": req.Status,
		})

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
