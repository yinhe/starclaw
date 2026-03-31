package v1

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

const superAgentSystemPrompt = `你是 StarClaw 的总管大臣——用户的首席 AI 助手。你统领所有专业 Agent，总揽全局，为用户分忧解难。

## 身份定位
- 你是虫群（Claw 节点）的**总管**，是用户与所有 AI 能力之间的唯一入口
- 简单任务亲自上手，复杂任务委派给麾下专业 Agent
- 你了解用户的偏好和记忆，每次对话都在进化

## 基本规则
- **始终使用中文回复**
- **通过 function call 执行操作**，不要用文字描述计划
- 一次调一个工具，等结果后再下一步
- 所有生成类工具消耗星能，**首次调用前提醒用户确认**
- 不伪造结果，不暴露第三方地址（fal.ai/DashScope等）

## 决策流程
1. **理解意图** → 分析用户需求的类型和复杂度
2. **路由决策** → 简单任务直接执行 / 专业任务委派 Agent
3. **执行或委派** → 调用工具完成任务
4. **质量检查** → 验证结果是否符合预期
5. **交付汇报** → 展示结果，提供后续建议
6. **记忆归档** → 有价值的信息存入长期记忆

## 委派策略
以下专业任务**必须委派**（delegate_to_agent）：
- MV/音乐视频 → "MV创作Agent"
- 短剧/短片/微电影 → "短剧导演"
- 漫剧/漫画视频 → "漫剧创作Agent"
- 商业计划书/BP → "商业计划书Agent"

## 能力域

### 信息获取
- web_search: 搜索互联网
- browser: 打开网页、点击、截图、提取内容
- http_request: HTTP 请求、调用第三方 API

### 内容创作
- video_generation: 多模型视频生成（wan/veo3/sora2/kling/luma）
- music_generation: AI 作曲（ACE-Step/MiniMax/DiffRhythm）
- image_generation: AI 绘画（Flux/DALL-E）
- dubbing: TTS 配音 + 字幕
- mv_production: 节拍同步 MV 合成
- comic_production: 漫剧制作
- audio_analysis: 音频 BPM/节拍分析

### 编程开发
- code: 14种语言编写/运行/调试/部署 Web 应用

### 文档处理
- document: 对话总结、Word 文档导出

### 系统管理
- system: Agent 编排、任务调度、工作流、通知

### 桌面操控（本地 Spore 模式可用）
- desktop: 截图/点击/输入/操控桌面应用
- 微信发消息首选: desktop(action="wechat_send", title="联系人", text="内容")
- UI 自动化优先于视觉模式，视觉模式优先于 MCP Bridge

## 工作原则
1. **直接执行**：自己有工具就亲自做，不推诿
2. **果断行动**：不反复确认，该做就做
3. **自动纠错**：出错时自动修复，不等用户催
4. **完整交付**：总结成果 + 后续建议
5. **节约资源**：重新合成用 merge_videos，不重新生成
6. **代码可运行**：写完代码给出 bash 运行命令，用户可一键执行`

type AgentHandler struct {
	db *gorm.DB
}

func NewAgentHandler(db *gorm.DB) *AgentHandler {
	return &AgentHandler{db: db}
}

type CreateAgentRequest struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	SystemPrompt    string `json:"system_prompt"`
	ModelID         string `json:"model_id"`
	Tools           string `json:"tools"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	Config          string `json:"config"`
	IsPublic        bool   `json:"is_public"`
}

func (h *AgentHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")

	var agents []model.Agent
	// Return user's own agents + system built-in agents
	if err := h.db.Where("user_id = ? OR (user_id = 'system' AND is_builtin = ?)", userID, true).Order("is_builtin DESC, created_at DESC").Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch agents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

func (h *AgentHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tools := req.Tools
	if tools == "" {
		tools = "[]"
	}
	config := req.Config
	if config == "" {
		config = "{}"
	}

	agent := model.Agent{
		UserID:          userID,
		Name:            req.Name,
		Description:     req.Description,
		SystemPrompt:    req.SystemPrompt,
		ModelID:         req.ModelID,
		Tools:           tools,
		KnowledgeBaseID: req.KnowledgeBaseID,
		Config:          config,
		IsPublic:        req.IsPublic,
	}

	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create agent"})
		return
	}

	// Auto-create workflow for this agent
	wfDef := generateWorkflowFromTools(agent.Name, agent.Tools)
	if wfDef != "" {
		wfTag := fmt.Sprintf("[agent:%s]", agent.Name)
		wf := model.Workflow{
			UserID:      userID,
			Name:        agent.Name + " 工作流",
			Description: fmt.Sprintf("%s 的标准工作流 %s", agent.Name, wfTag),
			Definition:  wfDef,
		}
		h.db.Create(&wf)
	}

	// Auto-create swarm unit (虫群成员) for this agent
	CreateSwarmUnitFromAgent(h.db, userID, agent)

	// Stardust reward for creating a new agent
	go NewStardustEngine(h.db).RewardAgentCreated(userID)

	c.JSON(http.StatusCreated, agent)
}

func (h *AgentHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var agent model.Agent
	if err := h.db.Preload("Skills").Where("id = ? AND (user_id = ? OR is_public = ?)", id, userID, true).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Find associated workflow: DB ↀbuilt-in ↀauto-generate from tools
	var workflowDef string
	var workflowID string
	wfTag := fmt.Sprintf("[agent:%s]", agent.Name)
	var wf model.Workflow
	if err := h.db.Where("user_id = ? AND description LIKE ?", userID, "%"+wfTag+"%").First(&wf).Error; err == nil {
		// If existing workflow has empty definition, regenerate it
		if wf.Definition == "" || wf.Definition == "{}" {
			var regen string
			for _, def := range builtinAgents {
				if def.Name == agent.Name && def.Workflow != "" {
					regen = def.Workflow
					break
				}
			}
			if regen == "" {
				regen = generateWorkflowFromTools(agent.Name, agent.Tools)
			}
			if regen != "" {
				h.db.Model(&wf).Update("definition", regen)
				wf.Definition = regen
			}
		}
		workflowDef = wf.Definition
		workflowID = wf.ID
	} else {
		// Try built-in definition first
		var builtinDef string
		for _, def := range builtinAgents {
			if def.Name == agent.Name && def.Workflow != "" {
				builtinDef = def.Workflow
				break
			}
		}
		// If no built-in, auto-generate from agent's tool list
		if builtinDef == "" {
			builtinDef = generateWorkflowFromTools(agent.Name, agent.Tools)
		}
		if builtinDef != "" {
			wf = model.Workflow{
				UserID:      userID,
				Name:        agent.Name + " 工作流",
				Description: fmt.Sprintf("%s 的标准工作流 %s", agent.Name, wfTag),
				Definition:  builtinDef,
			}
			if err := h.db.Create(&wf).Error; err == nil {
				workflowDef = wf.Definition
				workflowID = wf.ID
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":        agent,
		"workflow_def": workflowDef,
		"workflow_id":  workflowID,
	})
}

// GetWorkflow ensures a workflow exists for an agent and returns its ID
func (h *AgentHandler) GetWorkflow(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var agent model.Agent
	if err := h.db.Where("id = ? AND (user_id = ? OR is_public = ?)", id, userID, true).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	wfTag := fmt.Sprintf("[agent:%s]", agent.Name)
	var wf model.Workflow

	// Check DB first
	if err := h.db.Where("user_id = ? AND description LIKE ?", userID, "%"+wfTag+"%").First(&wf).Error; err == nil {
		// Found - regenerate if empty
		if wf.Definition == "" || wf.Definition == "{}" {
			def := ""
			for _, d := range builtinAgents {
				if d.Name == agent.Name && d.Workflow != "" {
					def = d.Workflow
					break
				}
			}
			if def == "" {
				def = generateWorkflowFromTools(agent.Name, agent.Tools)
			}
			if def != "" {
				h.db.Model(&wf).Updates(map[string]interface{}{"definition": def, "name": agent.Name + " 工作流"})
			}
		}
		c.JSON(http.StatusOK, gin.H{"workflow_id": wf.ID})
		return
	}

	// Not found - create new
	def := ""
	for _, d := range builtinAgents {
		if d.Name == agent.Name && d.Workflow != "" {
			def = d.Workflow
			break
		}
	}
	if def == "" {
		def = generateWorkflowFromTools(agent.Name, agent.Tools)
	}
	if def == "" {
		def = `{"nodes":[{"id":"start","type":"start","position":{"x":300,"y":100},"data":{"label":"开始"}},{"id":"end","type":"end","position":{"x":300,"y":400},"data":{"label":"结束"}}],"edges":[{"id":"e-start-end","source":"start","target":"end"}]}`
	}

	wf = model.Workflow{
		UserID:      userID,
		Name:        agent.Name + " 工作流",
		Description: fmt.Sprintf("%s 的标准工作流 %s", agent.Name, wfTag),
		Definition:  def,
	}
	if err := h.db.Create(&wf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workflow_id": wf.ID})
}

func (h *AgentHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var agent model.Agent
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&agent).Updates(model.Agent{
		Name:            req.Name,
		Description:     req.Description,
		SystemPrompt:    req.SystemPrompt,
		ModelID:         req.ModelID,
		Tools:           req.Tools,
		KnowledgeBaseID: req.KnowledgeBaseID,
		Config:          req.Config,
		IsPublic:        req.IsPublic,
	})

	c.JSON(http.StatusOK, agent)
}

// Marketplace: list all public agents from all users
func (h *AgentHandler) Export(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Param("id")

	var agent model.Agent
	if err := h.db.Where("id = ? AND (user_id = ? OR is_public = ?)", agentID, userID, true).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	export := gin.H{
		"name":              agent.Name,
		"description":       agent.Description,
		"system_prompt":     agent.SystemPrompt,
		"model_id":          agent.ModelID,
		"tools":             agent.Tools,
		"knowledge_base_id": agent.KnowledgeBaseID,
		"is_public":         agent.IsPublic,
		"version":           "1.0",
		"platform":          "starclaw",
	}

	c.Header("Content-Disposition", "attachment; filename=agent_"+agent.Name+".json")
	c.JSON(http.StatusOK, export)
}

func (h *AgentHandler) Import(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name            string `json:"name" binding:"required"`
		Description     string `json:"description"`
		SystemPrompt    string `json:"system_prompt"`
		ModelID         string `json:"model_id"`
		Tools           string `json:"tools"`
		KnowledgeBaseID string `json:"knowledge_base_id"`
		IsPublic        bool   `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent := model.Agent{
		UserID:          userID,
		Name:            req.Name + " (导入)",
		Description:     req.Description,
		SystemPrompt:    req.SystemPrompt,
		ModelID:         req.ModelID,
		Tools:           req.Tools,
		KnowledgeBaseID: req.KnowledgeBaseID,
		IsPublic:        req.IsPublic,
	}
	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import agent"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

func (h *AgentHandler) ListPublic(c *gin.Context) {
	var agents []model.Agent
	h.db.Where("is_public = ?", true).Order("created_at DESC").Preload("User").Find(&agents)
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// Clone a public agent to the current user's collection
func (h *AgentHandler) Clone(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var source model.Agent
	if err := h.db.Where("id = ? AND is_public = ?", id, true).First(&source).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "public agent not found"})
		return
	}

	clone := model.Agent{
		UserID:          userID,
		Name:            source.Name + " (副本)",
		Description:     source.Description,
		SystemPrompt:    source.SystemPrompt,
		ModelID:         source.ModelID,
		Tools:           source.Tools,
		KnowledgeBaseID: "", // don't copy KB
		Config:          source.Config,
		IsPublic:        false,
	}
	if err := h.db.Create(&clone).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clone agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agent": clone})
}

// ShareAgent generates a public share token for an agent
func (h *AgentHandler) Share(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Param("id")

	var agent model.Agent
	if err := h.db.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Make agent public if not already
	if !agent.IsPublic {
		h.db.Model(&agent).Update("is_public", true)
	}

	// Share token is just the agent ID (public agents are accessible)
	proto := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		proto = "https"
	}
	shareURL := fmt.Sprintf("%s://%s/v1/agents/shared/%s", proto, c.Request.Host, agent.ID)
	c.JSON(http.StatusOK, gin.H{
		"share_url": shareURL,
		"agent_id":  agent.ID,
	})
}

// GetSharedAgent returns a public agent by ID (no auth required)
func (h *AgentHandler) GetShared(c *gin.Context) {
	id := c.Param("id")

	var agent model.Agent
	if err := h.db.Where("id = ? AND is_public = ?", id, true).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "shared agent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent": gin.H{
			"id":            agent.ID,
			"name":          agent.Name,
			"description":   agent.Description,
			"system_prompt": agent.SystemPrompt,
			"tools":         agent.Tools,
			"config":        agent.Config,
			"platform":      "starclaw",
		},
	})
}

// EnsureSuperAgent creates system-level built-in agents (visible to all users)
func (h *AgentHandler) EnsureSuperAgent(c *gin.Context) {
	ownerID := getOwnerOrSystemID(h.db)

	superDesc := "智能路由编排 + 全能执行者。自动识别需求并委派给专业Agent（MV创作、视频、音乐、漫剧、编程、研究），也可直接执行任何任务。"
	superTools := `["code","system","browser","web_search","http_request","video_generation","dubbing","mv_production","comic_production","music_generation","image_generation","feishu"]`

	// Ensure SuperAgent (system-level)
	var superAgent model.Agent
	created := false
	if err := h.db.Where("(user_id = ? OR user_id = ?) AND name = ?", ownerID, model.SystemUserID, "全能助手").First(&superAgent).Error; err == nil {
		h.db.Model(&superAgent).Updates(map[string]interface{}{
			"system_prompt": superAgentSystemPrompt,
			"tools":         superTools,
			"description":   superDesc,
			"is_builtin":    true,
		})
		h.db.Where("id = ?", superAgent.ID).First(&superAgent)
	} else {
		superAgent = model.Agent{
			UserID:       ownerID,
			Name:         "全能助手",
			Description:  superDesc,
			Tools:        superTools,
			Config:       `{"temperature":0.3,"max_tokens":8192}`,
			IsPublic:     true,
			IsBuiltin:    true,
			SystemPrompt: superAgentSystemPrompt,
		}
		h.db.Create(&superAgent)
		created = true
	}

	// Specialist agents are now installed from Queen marketplace, not seeded locally.

	if created {
		c.JSON(http.StatusCreated, gin.H{"agent": superAgent, "created": true})
	} else {
		c.JSON(http.StatusOK, gin.H{"agent": superAgent, "created": false})
	}
}

func (h *AgentHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	// Only protect the SuperAgent (全能助手) from deletion
	var agent model.Agent
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err == nil {
		if agent.IsBuiltin && agent.Name == "全能助手" {
			c.JSON(http.StatusForbidden, gin.H{"error": "全能助手不可卸载"})
			return
		}
	}

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Agent{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "agent deleted"})
}

// InstalledSourceIDs returns the list of marketplace source_ids that the user has installed
func (h *AgentHandler) InstalledSourceIDs(c *gin.Context) {
	userID := c.GetString("user_id")

	var ids []string
	h.db.Model(&model.Agent{}).
		Where("user_id = ? AND source_id != '' AND source_id IS NOT NULL", userID).
		Pluck("source_id", &ids)

	c.JSON(http.StatusOK, gin.H{"source_ids": ids})
}

// InstallFromMarketplace creates an agent from a Queen marketplace item config
func (h *AgentHandler) InstallFromMarketplace(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		SourceID     string `json:"source_id" binding:"required"` // marketplace item ID
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
		Tools        string `json:"tools"`
		Config       string `json:"config"`
		Icon         string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if already installed
	var exists int64
	h.db.Model(&model.Agent{}).Where("user_id = ? AND source_id = ?", userID, req.SourceID).Count(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该智能体已安装"})
		return
	}

	tools := req.Tools
	if tools == "" {
		tools = "[]"
	}
	config := req.Config
	if config == "" {
		config = `{"temperature":0.3,"max_tokens":8192}`
	}

	agent := model.Agent{
		UserID:       userID,
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Tools:        tools,
		Config:       config,
		IsPublic:     true,
		SourceID:     req.SourceID,
	}
	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "安装失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

// UninstallBySourceID removes an agent installed from marketplace by source_id
func (h *AgentHandler) UninstallBySourceID(c *gin.Context) {
	userID := c.GetString("user_id")
	sourceID := c.Param("source_id")

	result := h.db.Where("user_id = ? AND source_id = ?", userID, sourceID).Delete(&model.Agent{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该智能体"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已卸载"})
}
