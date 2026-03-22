package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/claw"
	"github.com/yinhe/starclaw-overlord/api/internal/middleware"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"github.com/yinhe/starclaw-overlord/api/internal/ws"
	"gorm.io/gorm"
)

// ── Types for template JSON fields ──

type TeamRole struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Model        string   `json:"model"`
	Tools        []string `json:"tools"`
	MaxInstances int      `json:"max_instances"`
}

type TopologyFlow struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // pipeline | fan_out | fan_in | review_loop
}

type TopologyConfig struct {
	Type string         `json:"type"` // pipeline | dag
	Flow []TopologyFlow `json:"flow"`
}

type QualityGateConfig struct {
	ReviewThreshold float64 `json:"review_threshold"` // minimum score to pass (e.g. 7.0)
	MaxRetries      int     `json:"max_retries"`      // max review loop retries
	TestRequired    bool    `json:"test_required"`
	BuildRequired   bool    `json:"build_required"`
}

type EscalationConfig struct {
	OnMaxRetries   string `json:"on_max_retries"`   // bounty | pause_notify | human_review
	OnBudgetExceed string `json:"on_budget_exceed"` // pause_notify | hard_stop
}

// ── Handler ──

type TeamAgentHandler struct {
	db            *gorm.DB
	clawClient    *claw.Client
	overlordToken string // shared secret for Overlord→Claw auth
	wsHub         *ws.Hub
}

// SetWSHub wires the WebSocket hub for real-time push.
func (h *TeamAgentHandler) SetWSHub(hub *ws.Hub) {
	h.wsHub = hub
}

func NewTeamAgentHandler(db *gorm.DB) *TeamAgentHandler {
	token := os.Getenv("OVERLORD_CLAW_TOKEN")
	if token == "" {
		token = "overlord-internal-default"
	}
	return &TeamAgentHandler{
		db:            db,
		clawClient:    claw.NewClient(),
		overlordToken: token,
	}
}

// SeedOfficialTemplates inserts built-in templates if they don't exist.
func (h *TeamAgentHandler) SeedOfficialTemplates() {
	var count int64
	h.db.Model(&model.TeamAgentTemplate{}).Where("is_official = ?", true).Count(&count)
	if count > 0 {
		return
	}

	templates := buildOfficialTemplates()
	for _, t := range templates {
		if err := h.db.Create(&t).Error; err != nil {
			log.Printf("[team-agent] failed to seed template %s: %v", t.Name, err)
		}
	}
	log.Printf("[team-agent] seeded %d official templates", len(templates))
}

// ── Template endpoints ──

// GET /brood/team-agent/templates
func (h *TeamAgentHandler) ListTemplates(c *gin.Context) {
	var templates []model.TeamAgentTemplate
	q := h.db.Order("is_official DESC, name ASC")
	if cat := c.Query("category"); cat != "" {
		q = q.Where("category = ?", cat)
	}
	q.Find(&templates)
	c.JSON(http.StatusOK, gin.H{"templates": templates, "total": len(templates)})
}

// GET /brood/team-agent/templates/:id
func (h *TeamAgentHandler) GetTemplate(c *gin.Context) {
	var t model.TeamAgentTemplate
	if err := h.db.First(&t, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": t})
}

// ── Instance endpoints ──

// POST /brood/team-agent/instances
func (h *TeamAgentHandler) CreateInstance(c *gin.Context) {
	var req struct {
		TemplateID   string `json:"template_id" binding:"required"`
		ClawNodeID   string `json:"claw_node_id" binding:"required"`
		Name         string `json:"name" binding:"required"`
		Goal         string `json:"goal"`
		EnergyBudget int    `json:"energy_budget"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify template exists
	var tmpl model.TeamAgentTemplate
	if err := h.db.First(&tmpl, "id = ?", req.TemplateID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "template not found"})
		return
	}

	// Verify claw node is online
	var node model.ClawNode
	if err := h.db.First(&node, "id = ?", req.ClawNodeID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "claw node not found"})
		return
	}
	if node.Status == "offline" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "claw node is offline"})
		return
	}

	// Determine team scope
	teamID := ""
	if tid, ok := c.Get("admin_team"); ok {
		if s, ok := tid.(string); ok {
			teamID = s
		}
	}

	instance := model.TeamInstance{
		TemplateID:   tmpl.ID,
		TemplateName: tmpl.Name,
		TeamID:       teamID,
		ClawNodeID:   node.ID,
		UserID:       middleware.GetAdminActor(c),
		Name:         req.Name,
		Goal:         req.Goal,
		Status:       "forming",
		EnergyBudget: req.EnergyBudget,
	}
	if err := h.db.Create(&instance).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create instance"})
		return
	}

	// Bridge: call Claw node to create a Squad
	var roles []TeamRole
	json.Unmarshal([]byte(tmpl.Roles), &roles)
	roleNames := make([]string, 0, len(roles))
	for _, r := range roles {
		roleNames = append(roleNames, r.Code)
	}

	clawResp, err := h.clawClient.CreateSquad(node.Address, h.overlordToken, claw.CreateSquadReq{
		Name:        fmt.Sprintf("[TeamAgent] %s", req.Name),
		Description: fmt.Sprintf("Overlord Team Agent: %s (%s)", tmpl.Name, req.Goal),
		MaxMembers:  10,
		Tags:        roleNames,
		OverlordRef: instance.ID,
	})
	if err != nil {
		log.Printf("[team-agent] claw bridge failed for instance %s: %v (non-fatal)", instance.ID, err)
		// Non-fatal: instance is created in Overlord, Claw sync can retry later
	} else {
		// Store Claw Squad ID in config for future reference
		configMap := map[string]string{"claw_squad_id": clawResp.Squad.ID}
		configJSON, _ := json.Marshal(configMap)
		h.db.Model(&instance).Updates(map[string]interface{}{
			"config": string(configJSON),
			"status": "ready",
		})
		instance.Status = "ready"
		instance.Config = string(configJSON)
	}

	audit(h.db, c, "create_team_instance", instance.ID,
		fmt.Sprintf("team instance created: %s (template: %s, node: %s)", req.Name, tmpl.Name, node.Name))

	c.JSON(http.StatusCreated, gin.H{"instance": instance})
}

// GET /brood/team-agent/instances
func (h *TeamAgentHandler) ListInstances(c *gin.Context) {
	var instances []model.TeamInstance
	q := middleware.TeamScope(c, h.db).Order("created_at DESC")
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if tmplID := c.Query("template_id"); tmplID != "" {
		q = q.Where("template_id = ?", tmplID)
	}
	q.Find(&instances)
	c.JSON(http.StatusOK, gin.H{"instances": instances, "total": len(instances)})
}

// GET /brood/team-agent/instances/:id
func (h *TeamAgentHandler) GetInstance(c *gin.Context) {
	var inst model.TeamInstance
	if err := h.db.First(&inst, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"instance": inst})
}

// GET /brood/team-agent/instances/:id/dashboard
func (h *TeamAgentHandler) GetDashboard(c *gin.Context) {
	instID := c.Param("id")
	var inst model.TeamInstance
	if err := h.db.First(&inst, "id = ?", instID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}

	// Aggregate mission stats
	var missions []model.TeamMission
	h.db.Where("instance_id = ?", instID).Order("created_at DESC").Find(&missions)

	activeMission := ""
	activeMissionTitle := ""
	totalSteps := 0
	doneSteps := 0
	for i, m := range missions {
		if m.Status == "executing" || m.Status == "planning" || m.Status == "confirming" || m.Status == "reviewing" {
			activeMission = m.ID
			activeMissionTitle = m.Title

			// Live fetch from Claw for active mission
			if m.ClawMissionID != "" {
				var node model.ClawNode
				if err := h.db.First(&node, "id = ?", inst.ClawNodeID).Error; err == nil {
					if resp, err := h.clawClient.GetMission(node.Address, h.overlordToken, m.ClawMissionID); err == nil {
						missions[i].TotalSteps = resp.Mission.TotalSteps
						missions[i].DoneSteps = resp.Mission.DoneSteps
						missions[i].Status = mapClawMissionStatus(resp.Mission.Status)
						if resp.Mission.PreviewURL != "" {
							missions[i].PreviewURL = resp.Mission.PreviewURL
						}
						// Persist the fresh data
						h.db.Model(&model.TeamMission{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
							"total_steps": resp.Mission.TotalSteps,
							"done_steps":  resp.Mission.DoneSteps,
							"status":      missions[i].Status,
							"preview_url": missions[i].PreviewURL,
						})
					}
				}
			}

			totalSteps = missions[i].TotalSteps
			doneSteps = missions[i].DoneSteps
			break
		}
	}

	progress := 0
	if totalSteps > 0 {
		progress = doneSteps * 100 / totalSteps
	}

	// Parse roles from template
	var tmpl model.TeamAgentTemplate
	h.db.First(&tmpl, "id = ?", inst.TemplateID)
	var roles []TeamRole
	json.Unmarshal([]byte(tmpl.Roles), &roles)

	type roleStatus struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	roleStatuses := make([]roleStatus, 0, len(roles))
	for _, r := range roles {
		roleStatuses = append(roleStatuses, roleStatus{Code: r.Code, Name: r.Name})
	}

	energyRate := 0.0
	if inst.EnergyUsed > 0 && !inst.CreatedAt.IsZero() {
		elapsed := time.Since(inst.CreatedAt).Minutes()
		if elapsed > 0 {
			energyRate = float64(inst.EnergyUsed) / elapsed
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"team_id":       inst.ID,
		"team_name":     inst.Name,
		"template_name": inst.TemplateName,
		"status":        inst.Status,
		"mission_id":    activeMission,
		"mission_title": activeMissionTitle,
		"total_steps":   totalSteps,
		"done_steps":    doneSteps,
		"progress":      progress,
		"roles":         roleStatuses,
		"energy_budget": inst.EnergyBudget,
		"energy_used":   inst.EnergyUsed,
		"energy_rate":   energyRate,
		"mission_count": inst.MissionCount,
		"avg_score":     inst.AvgScore,
		"missions":      missions,
	})
}

// POST /brood/team-agent/instances/:id/disband
func (h *TeamAgentHandler) DisbandInstance(c *gin.Context) {
	instID := c.Param("id")

	// Load instance first for Claw bridge call
	var inst model.TeamInstance
	if err := h.db.First(&inst, "id = ? AND status != ?", instID, "disbanded").Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found or already disbanded"})
		return
	}

	now := time.Now()
	h.db.Model(&inst).Updates(map[string]interface{}{
		"status":       "disbanded",
		"disbanded_at": &now,
	})

	// Bridge: disband Squad on Claw node
	clawSquadID := extractConfig(inst.Config, "claw_squad_id")
	if clawSquadID != "" {
		var node model.ClawNode
		if err := h.db.First(&node, "id = ?", inst.ClawNodeID).Error; err == nil {
			if err := h.clawClient.DisbandSquad(node.Address, h.overlordToken, clawSquadID); err != nil {
				log.Printf("[team-agent] claw disband failed for squad %s: %v (non-fatal)", clawSquadID, err)
			}
		}
	}

	audit(h.db, c, "disband_team_instance", instID, "team instance disbanded")
	c.JSON(http.StatusOK, gin.H{"message": "team instance disbanded"})
}

// ── Mission endpoints ──

// POST /brood/team-agent/instances/:id/missions
func (h *TeamAgentHandler) CreateMission(c *gin.Context) {
	instID := c.Param("id")
	var inst model.TeamInstance
	if err := h.db.First(&inst, "id = ?", instID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	if inst.Status == "disbanded" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instance is disbanded"})
		return
	}

	var req struct {
		Goal        string `json:"goal" binding:"required"`
		AutoConfirm bool   `json:"auto_confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mission := model.TeamMission{
		InstanceID: instID,
		Title:      truncate(req.Goal, 200),
		Goal:       req.Goal,
		Status:     "planning",
	}
	if err := h.db.Create(&mission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create mission"})
		return
	}

	// Bridge: call Claw node to create + start a Mission in the Squad
	clawSquadID := extractConfig(inst.Config, "claw_squad_id")
	if clawSquadID != "" {
		var node model.ClawNode
		if err := h.db.First(&node, "id = ?", inst.ClawNodeID).Error; err == nil {
			clawResp, err := h.clawClient.CreateMission(node.Address, h.overlordToken, claw.CreateMissionReq{
				SquadID: clawSquadID,
				Title:   mission.Title,
				Goal:    req.Goal,
			})
			if err != nil {
				log.Printf("[team-agent] claw mission bridge failed: %v (non-fatal)", err)
			} else {
				// Store Claw Mission ID and start it
				h.db.Model(&mission).Update("claw_mission_id", clawResp.Mission.ID)
				mission.ClawMissionID = clawResp.Mission.ID

				// Auto-start the mission on Claw
				if err := h.clawClient.StartMission(node.Address, h.overlordToken, clawResp.Mission.ID); err != nil {
					log.Printf("[team-agent] claw mission start failed: %v (non-fatal)", err)
				} else {
					h.db.Model(&mission).Update("status", "executing")
					mission.Status = "executing"
				}
			}
		}
	}

	// Update instance status
	h.db.Model(&inst).Updates(map[string]interface{}{
		"status":        "running",
		"mission_count": gorm.Expr("mission_count + 1"),
	})

	audit(h.db, c, "create_team_mission", mission.ID,
		fmt.Sprintf("mission created for instance %s: %s", inst.Name, truncate(req.Goal, 100)))

	c.JSON(http.StatusCreated, gin.H{"mission": mission})
}

// GET /brood/team-agent/instances/:id/missions
func (h *TeamAgentHandler) ListMissions(c *gin.Context) {
	instID := c.Param("id")
	var missions []model.TeamMission
	h.db.Where("instance_id = ?", instID).Order("created_at DESC").Find(&missions)
	c.JSON(http.StatusOK, gin.H{"missions": missions, "total": len(missions)})
}

// GET /brood/team-agent/instances/:id/missions/:mid
func (h *TeamAgentHandler) GetMission(c *gin.Context) {
	var m model.TeamMission
	if err := h.db.First(&m, "id = ? AND instance_id = ?", c.Param("mid"), c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mission not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mission": m})
}

// ── Direct Chat (no instance/template required) ──

const directInstanceID = "direct"

// POST /brood/chat — direct chat without team instance
func (h *TeamAgentHandler) SendDirectChat(c *gin.Context) {
	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminUser, _ := c.Get("admin_user")
	var adminUserID string
	if u, ok := adminUser.(*model.AdminUser); ok {
		adminUserID = u.ID
	}

	// Save user message
	userMsg := model.ChatMessage{
		InstanceID: directInstanceID,
		UserID:     adminUserID,
		Role:       "user",
		Content:    req.Message,
	}
	h.db.Create(&userMsg)

	// Build conversation context from recent history (last 20 messages)
	var history []model.ChatMessage
	h.db.Where("instance_id = ? AND user_id = ?", directInstanceID, adminUserID).
		Order("created_at DESC").Limit(20).Find(&history)
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	systemPrompt := "你是 StarClaw AI 助手。请用专业、简洁的方式回答用户问题。支持中英文。"
	clawMessages := []claw.ChatMessage{{Role: "system", Content: systemPrompt}}
	for _, m := range history {
		clawMessages = append(clawMessages, claw.ChatMessage{Role: m.Role, Content: m.Content})
	}

	// Pick any online Claw node
	var node model.ClawNode
	if err := h.db.Where("status = ?", "online").Order("tasks_running ASC").First(&node).Error; err != nil {
		// No online node — try feral
		if err2 := h.db.Where("status = ?", "feral").First(&node).Error; err2 != nil {
			assistantMsg := model.ChatMessage{
				InstanceID: directInstanceID,
				UserID:     adminUserID,
				Role:       "assistant",
				Content:    "⚠️ 暂无可用的 AI 节点，请联系管理员添加 Claw 节点。",
			}
			h.db.Create(&assistantMsg)
			c.JSON(http.StatusOK, gin.H{"message": assistantMsg})
			return
		}
	}

	start := time.Now()
	chatResp, err := h.clawClient.ChatCompletion(node.Address, h.overlordToken, claw.ChatCompletionReq{
		Model:    "deepseek-chat",
		Messages: clawMessages,
		Stream:   false,
	})
	durationMs := int(time.Since(start).Milliseconds())

	var assistantContent string
	var tokensIn, tokensOut int
	var respModel string

	if err != nil {
		log.Printf("[direct-chat] completion failed: %v", err)
		assistantContent = "⚠️ AI 响应失败，请稍后重试。错误: " + err.Error()
	} else if len(chatResp.Choices) > 0 {
		assistantContent = chatResp.Choices[0].Message.Content
		tokensIn = chatResp.Usage.PromptTokens
		tokensOut = chatResp.Usage.CompletionTokens
		respModel = chatResp.Model
	} else {
		assistantContent = "⚠️ AI 返回了空响应。"
	}

	assistantMsg := model.ChatMessage{
		InstanceID: directInstanceID,
		UserID:     adminUserID,
		Role:       "assistant",
		Content:    assistantContent,
		Model:      respModel,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		DurationMs: durationMs,
	}
	h.db.Create(&assistantMsg)

	// Track usage
	if tokensIn > 0 || tokensOut > 0 {
		teamID := ""
		if tid, ok := c.Get("admin_team"); ok {
			if s, ok := tid.(string); ok {
				teamID = s
			}
		}
		usage := model.UsageRecord{
			TeamID:       teamID,
			UserID:       adminUserID,
			ClawID:       node.ID,
			ModelName:    respModel,
			InputTokens:  int64(tokensIn),
			OutputTokens: int64(tokensOut),
			TotalTokens:  int64(tokensIn + tokensOut),
			RequestType:  "chat",
			DurationMs:   durationMs,
			Date:         time.Now().Format("2006-01-02"),
		}
		h.db.Create(&usage)
	}

	c.JSON(http.StatusOK, gin.H{"message": assistantMsg})
}

// GET /brood/chat/history — direct chat history
func (h *TeamAgentHandler) GetDirectChatHistory(c *gin.Context) {
	adminUser, _ := c.Get("admin_user")
	var adminUserID string
	if u, ok := adminUser.(*model.AdminUser); ok {
		adminUserID = u.ID
	}

	var messages []model.ChatMessage
	h.db.Where("instance_id = ? AND user_id = ?", directInstanceID, adminUserID).
		Order("created_at ASC").Limit(100).Find(&messages)

	c.JSON(http.StatusOK, gin.H{"messages": messages, "total": len(messages)})
}

// ── Chat endpoints ──

// POST /brood/team-agent/instances/:id/chat
func (h *TeamAgentHandler) SendChat(c *gin.Context) {
	instID := c.Param("id")
	var inst model.TeamInstance
	if err := h.db.First(&inst, "id = ?", instID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	if inst.Status == "disbanded" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instance is disbanded"})
		return
	}

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current user
	userID := middleware.GetAdminActor(c)
	adminUser, _ := c.Get("admin_user")
	var adminUserID string
	if u, ok := adminUser.(*model.AdminUser); ok {
		adminUserID = u.ID
	}

	// Save user message
	userMsg := model.ChatMessage{
		InstanceID: instID,
		UserID:     adminUserID,
		Role:       "user",
		Content:    req.Message,
	}
	h.db.Create(&userMsg)

	// Build conversation context from recent history (last 20 messages)
	var history []model.ChatMessage
	h.db.Where("instance_id = ? AND user_id = ?", instID, adminUserID).
		Order("created_at DESC").Limit(20).Find(&history)
	// Reverse to chronological order
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	// Build messages array for Claw chat API
	// Load template for system prompt
	var tmpl model.TeamAgentTemplate
	h.db.First(&tmpl, "id = ?", inst.TemplateID)
	var roles []TeamRole
	json.Unmarshal([]byte(tmpl.Roles), &roles)

	systemPrompt := fmt.Sprintf("你是 %s (%s) 团队的 AI 助手。团队模板: %s。\n",
		inst.Name, tmpl.Name, tmpl.Description)
	if len(roles) > 0 {
		systemPrompt += "团队成员:\n"
		for _, r := range roles {
			systemPrompt += fmt.Sprintf("- %s (%s)\n", r.Name, r.Code)
		}
	}
	systemPrompt += "\n请根据你的团队专业领域，为用户提供专业、详细的回答。"

	clawMessages := []claw.ChatMessage{{Role: "system", Content: systemPrompt}}
	for _, m := range history {
		clawMessages = append(clawMessages, claw.ChatMessage{Role: m.Role, Content: m.Content})
	}

	// Determine model from template's primary role
	modelName := "deepseek-chat"
	if len(roles) > 0 && roles[0].Model != "" {
		modelName = roles[0].Model
	}

	// Proxy to Claw node
	var node model.ClawNode
	if err := h.db.First(&node, "id = ?", inst.ClawNodeID).Error; err != nil {
		// Fallback: return a stub response if Claw node is unavailable
		assistantMsg := model.ChatMessage{
			InstanceID: instID,
			UserID:     adminUserID,
			Role:       "assistant",
			Content:    "⚠️ AI 节点暂时不可用，请稍后重试。",
		}
		h.db.Create(&assistantMsg)
		c.JSON(http.StatusOK, gin.H{"message": assistantMsg})
		return
	}

	start := time.Now()
	chatResp, err := h.clawClient.ChatCompletion(node.Address, h.overlordToken, claw.ChatCompletionReq{
		Model:    modelName,
		Messages: clawMessages,
		Stream:   false,
	})
	durationMs := int(time.Since(start).Milliseconds())

	var assistantContent string
	var tokensIn, tokensOut int
	var respModel string

	if err != nil {
		log.Printf("[team-agent] chat completion failed for instance %s: %v", instID, err)
		assistantContent = "⚠️ AI 响应失败，请稍后重试。错误: " + err.Error()
	} else if len(chatResp.Choices) > 0 {
		assistantContent = chatResp.Choices[0].Message.Content
		tokensIn = chatResp.Usage.PromptTokens
		tokensOut = chatResp.Usage.CompletionTokens
		respModel = chatResp.Model
	} else {
		assistantContent = "⚠️ AI 返回了空响应。"
	}

	// Save assistant response
	assistantMsg := model.ChatMessage{
		InstanceID: instID,
		UserID:     adminUserID,
		Role:       "assistant",
		Content:    assistantContent,
		Model:      respModel,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		DurationMs: durationMs,
	}
	h.db.Create(&assistantMsg)

	// Track usage in UsageRecord
	if tokensIn > 0 || tokensOut > 0 {
		usage := model.UsageRecord{
			TeamID:       inst.TeamID,
			UserID:       adminUserID,
			ClawID:       inst.ClawNodeID,
			ModelName:    respModel,
			InputTokens:  int64(tokensIn),
			OutputTokens: int64(tokensOut),
			TotalTokens:  int64(tokensIn + tokensOut),
			RequestType:  "chat",
			DurationMs:   durationMs,
			Date:         time.Now().Format("2006-01-02"),
		}
		h.db.Create(&usage)

		// Update instance energy used (rough: 1 energy per 1000 tokens)
		energyDelta := (tokensIn + tokensOut) / 1000
		if energyDelta < 1 {
			energyDelta = 1
		}
		h.db.Model(&inst).Update("energy_used", gorm.Expr("energy_used + ?", energyDelta))
	}

	_ = userID // for audit
	c.JSON(http.StatusOK, gin.H{"message": assistantMsg})
}

// GET /brood/team-agent/instances/:id/chat
func (h *TeamAgentHandler) GetChatHistory(c *gin.Context) {
	instID := c.Param("id")
	adminUser, _ := c.Get("admin_user")
	var adminUserID string
	if u, ok := adminUser.(*model.AdminUser); ok {
		adminUserID = u.ID
	}

	var messages []model.ChatMessage
	q := h.db.Where("instance_id = ?", instID)
	// Non-admin users only see their own messages
	role, _ := c.Get("admin_role")
	roleStr, _ := role.(string)
	if roleStr == "viewer" || roleStr == "operator" {
		q = q.Where("user_id = ?", adminUserID)
	}
	q.Order("created_at ASC").Limit(100).Find(&messages)

	c.JSON(http.StatusOK, gin.H{"messages": messages, "total": len(messages)})
}

// GET /brood/team-agent/usage/by-user — per-employee usage breakdown (for console)
func (h *TeamAgentHandler) UsageByUser(c *gin.Context) {
	type UserUsage struct {
		UserID       string `json:"user_id"`
		Username     string `json:"username"`
		MessageCount int64  `json:"message_count"`
		TotalTokens  int64  `json:"total_tokens"`
		InputTokens  int64  `json:"input_tokens"`
		OutputTokens int64  `json:"output_tokens"`
	}

	var results []UserUsage
	h.db.Raw(`
		SELECT u.user_id, a.username,
			COUNT(*) as message_count,
			SUM(u.total_tokens) as total_tokens,
			SUM(u.input_tokens) as input_tokens,
			SUM(u.output_tokens) as output_tokens
		FROM usage_records u
		LEFT JOIN admin_users a ON a.id = u.user_id
		WHERE u.request_type = 'chat'
		GROUP BY u.user_id, a.username
		ORDER BY total_tokens DESC
	`).Scan(&results)

	c.JSON(http.StatusOK, gin.H{"users": results, "total": len(results)})
}

// ── Stats ──

// GET /brood/team-agent/stats
func (h *TeamAgentHandler) Stats(c *gin.Context) {
	q := middleware.TeamScope(c, h.db)

	var totalInstances, activeInstances, totalMissions, totalEnergy int64
	q.Model(&model.TeamInstance{}).Count(&totalInstances)
	q.Model(&model.TeamInstance{}).Where("status IN ?", []string{"forming", "ready", "running", "maintenance"}).Count(&activeInstances)
	h.db.Model(&model.TeamMission{}).Count(&totalMissions)
	h.db.Model(&model.TeamInstance{}).Select("COALESCE(SUM(energy_used), 0)").Scan(&totalEnergy)

	var templateCount int64
	h.db.Model(&model.TeamAgentTemplate{}).Count(&templateCount)

	c.JSON(http.StatusOK, gin.H{
		"total_instances":  totalInstances,
		"active_instances": activeInstances,
		"total_missions":   totalMissions,
		"total_energy":     totalEnergy,
		"template_count":   templateCount,
	})
}

// ── Status Syncer (background goroutine) ──

// StartStatusSyncer polls Claw nodes every 30s to sync mission status.
func (h *TeamAgentHandler) StartStatusSyncer() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			h.syncMissionStatuses()
		}
	}()
	log.Printf("[team-agent] status syncer started (30s interval)")
}

func (h *TeamAgentHandler) syncMissionStatuses() {
	// Find active missions that have a Claw mission ID
	var missions []model.TeamMission
	h.db.Where("claw_mission_id != '' AND status IN ?",
		[]string{"planning", "confirming", "executing", "reviewing"}).
		Find(&missions)

	if len(missions) == 0 {
		return
	}

	// Group by instance to batch node lookups
	instanceIDs := make(map[string]bool)
	for _, m := range missions {
		instanceIDs[m.InstanceID] = true
	}

	ids := make([]string, 0, len(instanceIDs))
	for id := range instanceIDs {
		ids = append(ids, id)
	}

	var instances []model.TeamInstance
	h.db.Where("id IN ?", ids).Find(&instances)
	instMap := make(map[string]*model.TeamInstance)
	for i := range instances {
		instMap[instances[i].ID] = &instances[i]
	}

	// Cache node addresses
	nodeCache := make(map[string]string) // nodeID → address

	for _, m := range missions {
		inst := instMap[m.InstanceID]
		if inst == nil {
			continue
		}

		nodeAddr, ok := nodeCache[inst.ClawNodeID]
		if !ok {
			var node model.ClawNode
			if err := h.db.First(&node, "id = ?", inst.ClawNodeID).Error; err != nil {
				continue
			}
			nodeAddr = node.Address
			nodeCache[inst.ClawNodeID] = nodeAddr
		}

		resp, err := h.clawClient.GetMission(nodeAddr, h.overlordToken, m.ClawMissionID)
		if err != nil {
			continue // skip silently, will retry next tick
		}

		// Map Claw status → Overlord status
		newStatus := mapClawMissionStatus(resp.Mission.Status)
		updates := map[string]interface{}{}

		if newStatus != m.Status {
			updates["status"] = newStatus
		}
		if resp.Mission.TotalSteps != m.TotalSteps {
			updates["total_steps"] = resp.Mission.TotalSteps
		}
		if resp.Mission.DoneSteps != m.DoneSteps {
			updates["done_steps"] = resp.Mission.DoneSteps
		}
		if resp.Mission.PreviewURL != "" && resp.Mission.PreviewURL != m.PreviewURL {
			updates["preview_url"] = resp.Mission.PreviewURL
		}

		if len(updates) > 0 {
			h.db.Model(&model.TeamMission{}).Where("id = ?", m.ID).Updates(updates)

			// Push real-time update via WebSocket
			if h.wsHub != nil {
				updates["mission_id"] = m.ID
				updates["instance_id"] = m.InstanceID
				updates["title"] = m.Title
				if inst := instMap[m.InstanceID]; inst != nil {
					h.wsHub.SendToTeam(inst.TeamID, ws.EventTeamMissionUpdate, updates)
				}
				h.wsHub.SendToTeam("global", ws.EventTeamMissionUpdate, updates)
			}
		}

		// If mission completed/failed, update instance metrics
		if newStatus == "completed" || newStatus == "failed" {
			now := time.Now()
			h.db.Model(&model.TeamMission{}).Where("id = ?", m.ID).Update("completed_at", &now)

			// Recalc instance status
			var activeMissions int64
			h.db.Model(&model.TeamMission{}).Where("instance_id = ? AND status IN ?",
				m.InstanceID, []string{"planning", "confirming", "executing", "reviewing"}).
				Count(&activeMissions)
			if activeMissions == 0 {
				h.db.Model(&model.TeamInstance{}).Where("id = ?", m.InstanceID).
					Update("status", "ready")
			}
		}
	}
}

func mapClawMissionStatus(clawStatus string) string {
	switch clawStatus {
	case "planning":
		return "planning"
	case "executing":
		return "executing"
	case "reviewing":
		return "reviewing"
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	default:
		return clawStatus
	}
}

// ── Helpers ──

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

func extractConfig(configJSON, key string) string {
	if configJSON == "" {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(configJSON), &m); err != nil {
		return ""
	}
	return m[key]
}

// ── Official Templates ──

func buildOfficialTemplates() []model.TeamAgentTemplate {
	return []model.TeamAgentTemplate{
		buildDevClaw(),
		buildMarketClaw(),
		buildSupportClaw(),
		buildDataClaw(),
		buildMedClaw(),
		buildEcomClaw(),
		buildDramaClaw(),
		buildSalesClaw(),
		buildOpsClaw(),
	}
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func buildDevClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{
			Code:         "architect",
			Name:         "设计虫",
			SystemPrompt: "你是首席架构师。负责技术方案设计、架构决策、技术选型。输出格式: 结构化设计文档 + 技术栈选择 + 模块划分 + API 设计。你的设计方案会交给编码虫执行，必须足够具体可执行。",
			Model:        "gpt-4o",
			Tools:        []string{"code", "web_search", "document_read"},
			MaxInstances: 1,
		},
		{
			Code:         "drone",
			Name:         "编码虫",
			SystemPrompt: "你是全栈开发工程师。负责编写实际可运行的代码。工作流程: 读取设计方案 → 编码 → git add → git commit → git push。你的代码会被审查虫审查，注意代码质量和可测试性。",
			Model:        "deepseek-chat",
			Tools:        []string{"code", "git", "web_search"},
			MaxInstances: 3,
		},
		{
			Code:         "tester",
			Name:         "测试虫",
			SystemPrompt: "你是测试工程师。负责编写和执行测试用例。覆盖: 单元测试 + 集成测试 + 边界条件。输出: 测试代码 + 测试报告 + 覆盖率。",
			Model:        "deepseek-chat",
			Tools:        []string{"code", "git"},
			MaxInstances: 1,
		},
		{
			Code:         "reviewer",
			Name:         "审查虫",
			SystemPrompt: "你是代码审查专家。审查标准: 安全性 > 正确性 > 可读性 > 性能。输出 JSON: { verdict: approved/changes_requested, score: 1-10, issues: [], suggestions: [] }。评分 ≥ 7 通过，< 7 退回重写。严格但务实。",
			Model:        "gpt-4o",
			Tools:        []string{"code", "git"},
			MaxInstances: 1,
		},
		{
			Code:         "docbot",
			Name:         "文档虫",
			SystemPrompt: "你是技术文档工程师。负责编写 README、API 文档、部署文档。风格: 简洁、有示例、面向新手。",
			Model:        "deepseek-chat",
			Tools:        []string{"code", "document_write"},
			MaxInstances: 1,
		},
	}

	topology := TopologyConfig{
		Type: "dag",
		Flow: []TopologyFlow{
			{From: "start", To: "architect", Type: "pipeline"},
			{From: "architect", To: "drone", Type: "fan_out"},
			{From: "drone", To: "tester", Type: "fan_in"},
			{From: "tester", To: "reviewer", Type: "pipeline"},
			{From: "reviewer", To: "docbot", Type: "pipeline"},
		},
	}

	qualityGate := QualityGateConfig{
		ReviewThreshold: 7.0,
		MaxRetries:      3,
		TestRequired:    true,
		BuildRequired:   true,
	}

	escalation := EscalationConfig{
		OnMaxRetries:   "pause_notify",
		OnBudgetExceed: "pause_notify",
	}

	return model.TeamAgentTemplate{
		Name:        "DevClaw",
		Category:    "development",
		Description: "开发团队智能体 — Architect + Drone×3 + Tester + Reviewer + DocBot。适合功能开发、Bug 修复、技术文档。",
		Icon:        "code",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(qualityGate),
		Escalation:  mustJSON(escalation),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildMarketClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "strategist", Name: "策略虫", SystemPrompt: "你是首席营销策略师。分析产品特点、目标用户、市场竞争。输出: 营销策略方案 + 渠道选择 + 预算建议 + KPI 目标。", Model: "gpt-4o", Tools: []string{"web_search", "document_read"}, MaxInstances: 1},
		{Code: "copywriter", Name: "文案虫", SystemPrompt: "你是资深营销文案。根据策略方案撰写各渠道文案。风格: 简洁有力、有画面感、带行动号召。输出: 公众号长文/小红书短文/朋友圈/短视频脚本。", Model: "claude-sonnet-4-20250514", Tools: []string{"web_search", "document_write"}, MaxInstances: 2},
		{Code: "designer", Name: "设计虫", SystemPrompt: "你是视觉设计师。根据文案生成配图和视觉素材。输出: 海报/Banner/社交媒体图片/短视频分镜。", Model: "gpt-4o", Tools: []string{"image_generation"}, MaxInstances: 1},
		{Code: "analyst", Name: "分析虫", SystemPrompt: "你是数据分析师。分析营销效果，提出优化建议。输出: 渠道 ROI 报告 + A/B 测试建议 + 用户画像更新。", Model: "deepseek-chat", Tools: []string{"document_read", "web_search", "code"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "strategist", Type: "pipeline"},
		{From: "strategist", To: "copywriter", Type: "fan_out"},
		{From: "strategist", To: "designer", Type: "fan_out"},
		{From: "copywriter", To: "analyst", Type: "fan_in"},
		{From: "designer", To: "analyst", Type: "fan_in"},
	}}
	return model.TeamAgentTemplate{
		Name:        "MarketClaw",
		Category:    "marketing",
		Description: "营销团队智能体 — Strategist + Copywriter×2 + Designer + Analyst。适合文案创作、活动策划、数据分析。",
		Icon:        "megaphone",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 7.0, MaxRetries: 2}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "pause_notify", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildSupportClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "dispatcher", Name: "调度虫", SystemPrompt: "你是客服调度员。分析工单类型、紧急程度，分配给合适的客服。技术问题→Responder-Tech，账务问题→Responder-Billing，一般咨询→Responder-General，VIP/紧急→Escalator。", Model: "deepseek-chat", Tools: []string{"http_request"}, MaxInstances: 1},
		{Code: "responder", Name: "客服虫", SystemPrompt: "你是客服专员。根据知识库和FAQ回复客户问题。保持专业、友善、高效。无法解决的问题及时升级。", Model: "deepseek-chat", Tools: []string{"knowledge_base_search", "http_request"}, MaxInstances: 3},
		{Code: "escalator", Name: "升级虫", SystemPrompt: "你是客服主管。处理VIP客户、紧急问题、投诉。无法解决时使用bounty工具发布人工任务。", Model: "gpt-4o", Tools: []string{"bounty", "http_request", "knowledge_base_search"}, MaxInstances: 1},
		{Code: "analyst", Name: "分析虫", SystemPrompt: "你是客服分析师。定期分析工单趋势，优化知识库。输出: 周报（Top问题+满意度+优化建议）。", Model: "deepseek-chat", Tools: []string{"code", "document_write"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "dispatcher", Type: "pipeline"},
		{From: "dispatcher", To: "responder", Type: "fan_out"},
		{From: "responder", To: "escalator", Type: "pipeline"},
	}}
	return model.TeamAgentTemplate{
		Name:        "SupportClaw",
		Category:    "support",
		Description: "客服团队智能体 — Dispatcher + Responder×3 + Escalator + Analyst。适合工单处理、FAQ维护、客户分析。",
		Icon:        "headphones",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 6.0, MaxRetries: 2}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "bounty", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildDataClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "architect", Name: "架构虫", SystemPrompt: "你是数据架构师。设计数据管道、ETL流程、数据仓库结构。输出: 数据架构方案 + 表结构 + ETL流程图。", Model: "gpt-4o", Tools: []string{"code", "document_read"}, MaxInstances: 1},
		{Code: "etl_bot", Name: "ETL虫", SystemPrompt: "你是ETL工程师。编写数据清洗、转换、加载脚本。技术栈: Python + pandas + SQL。", Model: "deepseek-chat", Tools: []string{"code", "http_request"}, MaxInstances: 2},
		{Code: "analyst", Name: "分析虫", SystemPrompt: "你是数据分析师。从清洗后的数据中发现洞察。输出: 数据报告 + 可视化 + 趋势分析 + 改进建议。", Model: "deepseek-chat", Tools: []string{"code", "document_write"}, MaxInstances: 2},
		{Code: "reporter", Name: "报表虫", SystemPrompt: "你是报表工程师。生成格式化的数据报告和仪表盘。输出: PDF/Markdown报告 + 图表 + 关键指标汇总。", Model: "deepseek-chat", Tools: []string{"code", "document_write"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "architect", Type: "pipeline"},
		{From: "architect", To: "etl_bot", Type: "fan_out"},
		{From: "etl_bot", To: "analyst", Type: "fan_in"},
		{From: "analyst", To: "reporter", Type: "pipeline"},
	}}
	return model.TeamAgentTemplate{
		Name:        "DataClaw",
		Category:    "data",
		Description: "数据团队智能体 — Architect + ETL-Bot×2 + Analyst×2 + Reporter。适合数据清洗、报表生成、趋势分析。",
		Icon:        "bar_chart",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 7.0, MaxRetries: 2, TestRequired: true}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "pause_notify", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildQuantClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "strategist", Name: "策略虫", SystemPrompt: "你是量化策略研究员。负责提出交易策略假设、定义因子逻辑、设定信号规则。输出结构化策略描述(JSON): 因子定义、入场/出场条件、仓位管理规则、适用市场。", Model: "gpt-4o", Tools: []string{"web_search", "document_read", "code"}, MaxInstances: 1},
		{Code: "researcher", Name: "研究虫", SystemPrompt: "你是金融数据研究员。负责因子挖掘、市场微观结构分析、另类数据探索。从公开数据源获取行情/财报/舆情数据，清洗并构建因子特征。输出: 因子库(Python DataFrame) + 相关性矩阵 + IC分析报告。", Model: "deepseek-chat", Tools: []string{"code", "web_search", "http_request"}, MaxInstances: 1},
		{Code: "drone", Name: "编码虫", SystemPrompt: "你是量化开发工程师。将策略描述转化为可执行的交易策略代码。技术栈: Python + backtrader/vnpy。输出: 策略源码 + 数据接口 + 配置文件。策略参数化、支持回测和实盘切换、完整注释。", Model: "deepseek-chat", Tools: []string{"code", "git"}, MaxInstances: 1},
		{Code: "tester", Name: "回测虫", SystemPrompt: "你是量化回测专家。对策略代码进行历史回测和压力测试。输出JSON回测报告: sharpe, max_drawdown, annual_return, win_rate, profit_factor, calmar_ratio, trade_count, monthly_returns[]。必须包含不同市场环境分段测试(牛市/熊市/震荡)、参数敏感性分析。", Model: "deepseek-chat", Tools: []string{"code", "document_write"}, MaxInstances: 1},
		{Code: "reviewer", Name: "风控虫", SystemPrompt: "你是量化风控官。审查策略的风险暴露和合规性。检查: 最大回撤>20%→拒绝, Sharpe<1.0→警告, 单品种集中度>30%→警告, 过拟合检测(训练/测试Sharpe偏差>50%→拒绝)。输出JSON: { verdict: approved/rejected/warning, risk_score: 1-10, issues: [], recommendations: [] }。", Model: "gpt-4o", Tools: []string{"code", "document_read"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "strategist", Type: "pipeline"},
		{From: "strategist", To: "researcher", Type: "fan_out"},
		{From: "strategist", To: "drone", Type: "fan_out"},
		{From: "researcher", To: "drone", Type: "pipeline"},
		{From: "drone", To: "tester", Type: "pipeline"},
		{From: "tester", To: "reviewer", Type: "pipeline"},
	}}
	return model.TeamAgentTemplate{
		Name:        "QuantClaw",
		Category:    "finance",
		Description: "量化团队智能体 — Strategist + Researcher + Coder + Backtester + RiskGuard。适合策略研发、因子挖掘、回测验证、风控审查。",
		Icon:        "trending_up",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 8.0, MaxRetries: 3, TestRequired: true}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "pause_notify", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildEcomClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "strategist", Name: "选品虫", SystemPrompt: "你是电商选品经理。分析市场趋势、竞品数据、用户需求，确定产品定位和卖点。输出产品策划(JSON): product_name, category, target_audience, key_selling_points[], price_range, competitor_urls[], platform, style_tone。核心: 提炼差异化卖点，找到用户痛点。", Model: "gpt-4o", Tools: []string{"web_search", "document_read"}, MaxInstances: 1},
		{Code: "copywriter", Name: "文案虫", SystemPrompt: "你是电商文案专家。根据产品策划撰写全套电商文案。输出: 商品标题(含关键词≤30字) + 五点描述 + 详情页文案(FABE法则) + 短视频脚本(15s/30s/60s) + 直播话术要点。适配平台: 淘宝/京东/拼多多/抖音/小红书。", Model: "claude-sonnet-4-20250514", Tools: []string{"document_write", "web_search"}, MaxInstances: 1},
		{Code: "designer", Name: "设计虫", SystemPrompt: "你是电商视觉设计师。生成商品主图和详情页视觉。输出: 商品主图(800×800白底) + 场景图(3-5张) + 详情页长图(竖版) + 短视频封面(9:16)。风格与品牌调性一致，突出卖点。", Model: "gpt-4o", Tools: []string{"image_generation"}, MaxInstances: 1},
		{Code: "drone", Name: "优化虫", SystemPrompt: "你是电商SEO/投流专家。优化商品在各平台的搜索排名和付费投放。输出: 关键词库(核心词+长尾词+竞品词) + 标题优化建议 + 投流计划(直通车/千川) + A/B测试方案 + 评价管理策略。", Model: "deepseek-chat", Tools: []string{"web_search", "code"}, MaxInstances: 1},
		{Code: "analyst", Name: "分析虫", SystemPrompt: "你是电商数据分析师。分析销售数据，提出优化建议。输出: 转化率漏斗分析(曝光→点击→加购→下单→付款) + 竞品价格监控 + ROI分析(各渠道投入产出) + 库存预警。", Model: "deepseek-chat", Tools: []string{"code", "document_write"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "strategist", Type: "pipeline"},
		{From: "strategist", To: "copywriter", Type: "fan_out"},
		{From: "strategist", To: "designer", Type: "fan_out"},
		{From: "strategist", To: "drone", Type: "fan_out"},
		{From: "copywriter", To: "analyst", Type: "fan_in"},
		{From: "designer", To: "analyst", Type: "fan_in"},
		{From: "drone", To: "analyst", Type: "fan_in"},
	}}
	return model.TeamAgentTemplate{
		Name:        "EcomClaw",
		Category:    "ecommerce",
		Description: "电商团队智能体 — ProductMgr + Copywriter + Designer + Optimizer + Analyst。适合商品文案、主图详情页、SEO优化、销售分析。",
		Icon:        "shopping_cart",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 7.0, MaxRetries: 2}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "pause_notify", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildDramaClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "strategist", Name: "导演虫", SystemPrompt: "你是短剧总导演。根据用户需求确定短剧类型、风格、节奏。输出创意大纲(JSON): title, genre, target_audience, episode_count, tone, hook_strategy, monetization_model, platform。短剧核心: 前3秒必须有钩子，每集结尾必须有悬念。", Model: "gpt-4o", Tools: []string{"web_search", "document_read"}, MaxInstances: 1},
		{Code: "copywriter", Name: "编剧虫", SystemPrompt: "你是短剧编剧。根据导演大纲撰写分集剧本。每集60-90秒，8-20集。每集结构: 开头钩子(0-3s冲突/悬念) → 主体(3-50s情节推进) → 结尾钩子(悬念/反转→引导看下一集)。输出: 分集剧本(含对白、场景描述、情绪指导、BGM建议)。", Model: "claude-sonnet-4-20250514", Tools: []string{"document_write", "web_search"}, MaxInstances: 1},
		{Code: "designer", Name: "分镜虫", SystemPrompt: "你是分镜设计师。将剧本转化为视觉分镜。每个镜头输出: scene_id, duration_sec, camera_angle, shot_type, visual_description, character_action, dialogue, transition, bgm_mood, image_prompt。image_prompt适配AI视频生成工具。注意: 短剧节奏快，平均镜头2-4秒。", Model: "gpt-4o", Tools: []string{"document_write", "image_generation"}, MaxInstances: 1},
		{Code: "drone", Name: "视频虫", SystemPrompt: "你是AI视频制作师。根据分镜的image_prompt生成视频片段。工作流: 每个镜头→调用video_generation生成2-5s视频，角色一致性: 使用image_to_video保持主角外貌。输出: 按scene_id命名的视频文件列表。参数: 1080×1920竖屏(9:16)，24fps。", Model: "deepseek-chat", Tools: []string{"video_generation", "image_generation"}, MaxInstances: 2},
		{Code: "reporter", Name: "剪辑虫", SystemPrompt: "你是短剧剪辑师。将视频片段剪辑成完整单集。工作流: 按分镜顺序拼接 → 添加字幕(对白) → 添加BGM → 转场 → 片头Logo(1s) + 片尾(关注引导3s)。输出: 完整单集MP4(60-90s, 9:16, 1080p)。", Model: "deepseek-chat", Tools: []string{"code", "audio_generation"}, MaxInstances: 1},
		{Code: "analyst", Name: "分发虫", SystemPrompt: "你是短剧运营专家。为成品短剧制作分发素材。输出: 每集封面图(竖版带标题) + 每集标题描述(适配抖音/快手/小红书) + 前3秒高光预告 + 发布时间建议 + 投流关键词 + 系列简介+Hashtag。", Model: "gpt-4o", Tools: []string{"image_generation", "document_write", "web_search"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "strategist", Type: "pipeline"},
		{From: "strategist", To: "copywriter", Type: "pipeline"},
		{From: "copywriter", To: "designer", Type: "pipeline"},
		{From: "designer", To: "drone", Type: "fan_out"},
		{From: "drone", To: "reporter", Type: "fan_in"},
		{From: "reporter", To: "analyst", Type: "pipeline"},
	}}
	return model.TeamAgentTemplate{
		Name:        "DramaClaw",
		Category:    "content",
		Description: "短剧团队智能体 — Director + Screenwriter + Storyboarder + VideoMaker×2 + Editor + Distributor。AI短剧批量生产，4小时出10集。",
		Icon:        "film",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 7.0, MaxRetries: 2}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "pause_notify", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildSalesClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "strategist", Name: "拓客虫", SystemPrompt: "你是B2B销售线索专家。根据ICP(理想客户画像)搜索目标企业，分析官网/新闻/招聘/天眼查数据，判断购买意向信号(招聘AI岗位/数字化转型/融资等)。输出线索卡片(JSON): company, industry, size, decision_makers[], pain_points[], intent_signals[], score:1-100。", Model: "deepseek-chat", Tools: []string{"web_search", "http_request"}, MaxInstances: 1},
		{Code: "reviewer", Name: "评估虫", SystemPrompt: "你是商机评估专家。用BANT框架评估线索质量。Budget:预算匹配? Authority:有决策权? Need:需求明确且紧迫? Timeline:3个月内采购? 输出: { bant_score, qualification: hot/warm/cold, recommended_action, talking_points[] }。", Model: "gpt-4o", Tools: []string{"web_search", "document_read"}, MaxInstances: 1},
		{Code: "copywriter", Name: "方案虫", SystemPrompt: "你是解决方案专家。为qualified商机撰写定制方案。输出: 客户痛点分析 + 解决方案匹配(功能→需求映射) + 实施计划 + ROI测算 + 报价方案(套餐推荐+折扣) + PPT大纲。", Model: "gpt-4o", Tools: []string{"document_write", "document_read"}, MaxInstances: 1},
		{Code: "drone", Name: "谈判虫", SystemPrompt: "你是销售谈判顾问。为销售人员提供谈判策略和话术。输出: 异议处理话术(LSCPA) + 竞品差异化对比 + 让步策略(底线+替代方案+增值) + 促单话术(紧迫感+稀缺性+社会证明)。", Model: "gpt-4o", Tools: []string{"web_search", "document_read"}, MaxInstances: 1},
		{Code: "analyst", Name: "分析虫", SystemPrompt: "你是销售数据分析师。分析pipeline和转化数据。输出: Pipeline看板(各阶段商机数+金额) + 转化率分析(线索→商机→方案→谈判→成交) + 销售预测 + 丢单分析(原因分布+改进)。", Model: "deepseek-chat", Tools: []string{"code", "document_write"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "strategist", Type: "pipeline"},
		{From: "strategist", To: "reviewer", Type: "pipeline"},
		{From: "reviewer", To: "copywriter", Type: "pipeline"},
		{From: "copywriter", To: "drone", Type: "pipeline"},
		{From: "drone", To: "analyst", Type: "pipeline"},
	}}
	return model.TeamAgentTemplate{
		Name:        "SalesClaw",
		Category:    "sales",
		Description: "销售团队智能体 — Prospector + Qualifier + ProposalWriter + Negotiator + Analyst。B2B线索挖掘、商机评估、方案撰写。",
		Icon:        "target",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 7.0, MaxRetries: 2}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "pause_notify", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildMedClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{
			Code:         "diagnostician",
			Name:         "主诊虫",
			SystemPrompt: "你是资深全科医生(AI辅助诊断)。根据患者症状描述进行初步分析。工作流程: 1.收集主诉+现病史+既往史 2.症状分析+鉴别诊断(列出可能性从高到低) 3.建议检查项目 4.给出初步诊断意见。重要: 你是AI辅助工具，必须提醒用户以专业医生面诊为准，不替代医疗诊断。对急危重症必须建议立即就医。",
			Model:        "gpt-4o",
			Tools:        []string{"web_search", "document_read"},
			MaxInstances: 1,
		},
		{
			Code:         "pharmacist",
			Name:         "药理虫",
			SystemPrompt: "你是临床药师。根据诊断建议提供用药参考。职责: 1.常用药物方案(通用名+规格+用法用量) 2.药物相互作用检查 3.禁忌症和注意事项 4.特殊人群用药调整(老人/儿童/孕妇/肝肾功能不全)。重要: 仅供参考，实际用药须遵医嘱。OTC和处方药须区分标注。",
			Model:        "deepseek-chat",
			Tools:        []string{"web_search", "document_read"},
			MaxInstances: 1,
		},
		{
			Code:         "triage",
			Name:         "分诊虫",
			SystemPrompt: "你是急诊分诊护士(AI)。评估患者紧急程度并建议就医科室。分级: Level-1(立即急诊: 胸痛/呼吸困难/大出血/意识障碍) Level-2(尽快就诊: 高热/剧烈疼痛/外伤) Level-3(常规就诊: 慢性病复查/轻微不适) Level-4(自我观察: 轻微感冒/小伤口)。输出: 紧急等级 + 建议科室 + 就医时间建议 + 就医前注意事项。",
			Model:        "deepseek-chat",
			Tools:        []string{"web_search"},
			MaxInstances: 1,
		},
		{
			Code:         "researcher",
			Name:         "文献虫",
			SystemPrompt: "你是循证医学研究员。检索最新医学文献和临床指南，为诊断和治疗方案提供证据支持。输出: 相关临床指南摘要 + 最新研究进展 + 证据等级(A/B/C) + 参考文献列表。数据来源: PubMed、Cochrane、UpToDate、各学会指南。",
			Model:        "gpt-4o",
			Tools:        []string{"web_search", "document_read"},
			MaxInstances: 1,
		},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "triage", Type: "pipeline"},
		{From: "triage", To: "diagnostician", Type: "pipeline"},
		{From: "diagnostician", To: "pharmacist", Type: "fan_out"},
		{From: "diagnostician", To: "researcher", Type: "fan_out"},
	}}
	return model.TeamAgentTemplate{
		Name:        "MedClaw",
		Category:    "medical",
		Description: "医疗智能体团队 — 分诊虫 + 主诊虫 + 药理虫 + 文献虫。症状分析、鉴别诊断、用药参考、文献检索。仅供参考，不替代专业医疗。",
		Icon:        "heart_pulse",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 8.0, MaxRetries: 2}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "pause_notify", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}

func buildOpsClaw() model.TeamAgentTemplate {
	roles := []TeamRole{
		{Code: "dispatcher", Name: "监控虫", SystemPrompt: "你是运维监控专家。持续监控系统指标(CPU/内存/磁盘/网络/响应时间)，识别异常和告警。发现问题立即分派给修复虫。输出: 告警事件(JSON): service, metric, threshold, current_value, severity:critical/warning/info, timestamp。", Model: "deepseek-chat", Tools: []string{"code", "http_request"}, MaxInstances: 1},
		{Code: "drone", Name: "修复虫", SystemPrompt: "你是运维修复工程师。接收告警后自动诊断和修复。修复流程: 1.收集日志→2.定位根因→3.执行修复(重启/扩容/回滚/配置修改)→4.验证恢复。只执行安全操作，高危操作(删除数据/切换主库)发布bounty给人工确认。", Model: "deepseek-chat", Tools: []string{"code", "http_request", "git"}, MaxInstances: 2},
		{Code: "reviewer", Name: "安全虫", SystemPrompt: "你是安全审查官。审查修复方案的安全性和影响范围。检查: 操作是否可回滚? 影响面是否可控? 是否引入新风险? 是否符合变更管理流程? 输出: { verdict: approved/rejected, risk_level: 1-5, concerns: [], rollback_plan }。", Model: "gpt-4o", Tools: []string{"code", "document_read"}, MaxInstances: 1},
		{Code: "reporter", Name: "报告虫", SystemPrompt: "你是运维报告专家。生成运维报告和SLA统计。输出: 日报(告警数/修复数/MTTR) + 周报(可用性/性能趋势/容量预测) + 事故RCA(根因分析+改进措施)。", Model: "deepseek-chat", Tools: []string{"code", "document_write"}, MaxInstances: 1},
	}
	topology := TopologyConfig{Type: "dag", Flow: []TopologyFlow{
		{From: "start", To: "dispatcher", Type: "pipeline"},
		{From: "dispatcher", To: "drone", Type: "fan_out"},
		{From: "drone", To: "reviewer", Type: "pipeline"},
		{From: "reviewer", To: "reporter", Type: "pipeline"},
	}}
	return model.TeamAgentTemplate{
		Name:        "OpsClaw",
		Category:    "ops",
		Description: "运维团队智能体 — Monitor + Medic×2 + Guardian + Reporter。监控告警、故障诊断、自动修复、SLA报告。",
		Icon:        "shield",
		Roles:       mustJSON(roles),
		Topology:    mustJSON(topology),
		QualityGate: mustJSON(QualityGateConfig{ReviewThreshold: 8.0, MaxRetries: 2}),
		Escalation:  mustJSON(EscalationConfig{OnMaxRetries: "bounty", OnBudgetExceed: "pause_notify"}),
		IsOfficial:  true,
		Version:     "v1",
	}
}
