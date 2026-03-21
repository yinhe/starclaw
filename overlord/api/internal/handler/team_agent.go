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
