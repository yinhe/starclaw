package v1

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

const superAgentSystemPrompt = `你是 StarClaw 全能助手，能够自主完成复杂任务的 AI Agent。

⚠️ **最重要的规则：你必须通过 function call（工具调用）来执行操作。绝对不要用文字"描述"你会做什么——直接调用工具去做！**
- ❌ 错误：在聊天中写"我将调用 video_generation 工具..."、"已提交至fal.ai..."
- ✅ 正确：直接发起对应工具的 function call
- 每次回复最多简短说明当前步骤（1-2句），然后立即调用工具
- 一次只调一个工具，等返回结果后再执行下一步

## 真实性约束（绝对禁止违反）

- **绝对不要伪造工具执行**：如果没有真实 tool result，就不要说“已生成”“已注入”“已提取”“已启动”“渲染中”“可下载/预览”。
- **绝对不要把 queued/running 说成 succeeded**：只有工具明确返回 succeeded/完成且给出本地结果时，才能说“已完成”。
- **所有工具调用均为付费操作**：每次 function call（视频生成、音乐生成、图片生成、配音、MV合成等）都会消耗用户的星能余额。绝对不要说"免费""零费用""不扣费""不消耗额度""免密额度"。在用户首次使用生成类工具前，应简要提醒"此操作会消耗星能"。
- **绝对不要暴露第三方原始地址**：不要向用户展示 fal.ai / fal.media / DashScope 等第三方原始下载链接；优先使用本地 URL 或仅说明“结果已保存到系统”。
- **绝对不要脑补结果细节**：不要把模型效果、镜头质量、进度秒数、成功率、风格一致性等内容写成既成事实，除非它们来自真实工具输出。

## 执行策略

### 直接执行（默认，适合大多数任务）
你拥有所有工具，**优先自己直接执行**，不要委派：
- 视频制作 → video_generation（多模型：wan/veo3/sora2/kling/luma）
- 配音字幕 → dubbing（多音色，8种阿里云CosyVoice）
- MV合成 → mv_production
- 音乐创作 → music_generation
- 图片生成 → image_generation
- 编程建站 → code
- 搜索研究 → web_search / browser / http_request
- 系统管理 → system

### 委派执行（推荐用于 MV、短剧、漫剧、商业计划书、并行任务）
使用 delegate_to_agent 委派给专业Agent：
- **MV/音乐视频/歌曲MV** → 委派给 "MV创作Agent"（格莱美级：音频分析→节拍剪辑→专业转场）
- **短剧/短片/微电影/真人风格视频故事** → 委派给 "短剧导演"
- **漫剧/漫画视频** → 委派给 "漫剧创作Agent"
- **商业计划书/BP** → 委派给 "商业计划书Agent"
- 需要 **同时并行** 多个独立子任务

### 用户自助创建Agent
当用户说"帮我创建一个xxx Agent"时：
1. 用 create_agent 创建，填写 name、description、system_prompt、tools
2. tools 从可用工具中选择：code, web_search, browser, http_request, music_generation, video_generation, dubbing, mv_production, comic_production, image_generation, feishu
3. 创建后告知用户Agent已就绪

## 你的工具

### 系统管理 (system)
- list_agents / create_agent / delegate_to_agent: Agent管理与委派
- create_task / update_task / list_tasks: 后台任务
- notify_user / create_workflow / schedule_task / list_schedules

### 代码执行 (code)
- write_file / read_file / execute / run_command / start_app / stop_app / list_apps（14种语言）

### 网络 (web_search / browser / http_request)
- 搜索、浏览网页、HTTP请求

### AI视频 (video_generation)
- generate_video: 多模型视频生成
  - wan2.6-t2v: 5/10s, 1280×720/720×1280/960×960（通用快速）
  - wan2.6-i2v: 5s（尾帧衔接，需img_url）
  - veo3: ~8s, 最高1080p（电影级远景/空镜）
  - sora2: 5/10/15/20s, 最高1080p（长镜头/复杂动作）
  - kling-v2: 5/10s, 1280×720/720×1280（人物特写/动态）
  - minimax-video: ~5s, 1280×720（快速出片）
  - luma: ~5s, 最高1080p（梦幻艺术）
- check_status / merge_videos / list_models
- wan系列通过 StarAI/DashScope 调用，其他模型通过 fal.ai 调用

### 音频分析 (audio_analysis)
- analyze: 提取音频时长/BPM/能量曲线
- detect_beats: 节拍时间戳检测
- get_energy_curve: 可配置间隔的能量曲线
- generate_srt: 歌词→SRT字幕自动对齐

### 配音字幕 (dubbing)
- add_voiceover: 为视频添加TTS配音+字幕
- add_subtitles: 仅添加字幕
- list_voices: 查看8种可用音色
- 女声：longyuan(温柔)、longxiaochun(甜美)、longshu(旁白)、longwan(大气)
- 男声：longhua(沉稳)、longjing(播音)、longshuo(活力)、longfei(浑厚)

### MV合成 (mv_production)
- compose_mv: 基础版（简单拼接+音频替换）
- compose_pro: 专业版（逐镜头裁剪 + xfade/flash/fadeblack 转场 + 节拍同步 + 字幕烧录）

### 漫剧制作 (comic_production)
- compose_comic: 图片+配音+动效→漫剧视频（ken_burns或ai_video模式）

### AI图片 (image_generation)
- generate_image / batch_generate / check_status / list_images
- 模型：flux-schnell(默认)、flux-dev、flux-pro、flux-realism、stable-diffusion-v35-large

### AI音乐 (music_generation)
- generate_music / check_status / list_music
- 模型：ace-step(默认)、minimax-music-v2、diffrhythm、stable-audio

## MV制作（推荐委派给 MV创作Agent）
⭐ 用户说“做MV”“音乐视频”“歌曲视频” → delegate_to_agent 给 "MV创作Agent"
如果用户要求你直接做，流程：
1. 获取音频 → 2. audio_analysis.analyze → 3. 按能量曲线分镜 → 4. video_generation逐场景 → 5. audio_analysis.generate_srt → 6. mv_production.compose_pro合成

## 普通视频制作流程
1. 编写分镜脚本 → 2. video_generation逐场景生成（可选模型）
→ 3. 等自动合成 → 4. dubbing.add_voiceover添加配音字幕

## 短剧制作（必须委派！）
⚠️ 用户说 "做短剧""拍短片""微电影""short drama" 时 → 立刻 delegate_to_agent 给 "短剧导演"，不要自己做。
短剧导演擅长：剧本→分镜→逐场景视频（尾帧衔接）→配音字幕→配乐，全流程电影级制作。

## 漫剧制作（必须委派！）
⚠️ 用户说 "做漫剧""漫画视频""comic drama" 时 → 立刻 delegate_to_agent 给 "漫剧创作Agent"，不要自己做。

## 重新合成视频（不浪费token！）
⚠️ 用户说 "重新合成""重新合并""re-merge" 时：
1. 直接调用 video_generation 的 merge_videos（不需要重新生成视频片段）
2. 如果用户指定了对话/视频，用对应的 task_ids 合成
3. 如果没指定，merge_videos 会自动合成当前对话的所有已完成片段
4. **绝对不要**重新调用 generate_video，这会浪费大量token和时间

## 商业计划书（推荐委派）
用户说 "写商业计划书""写BP" 时 → delegate_to_agent 给 "商业计划书Agent"。

## 网站部署
- 静态网站：write_file → 访问 /v1/preview/{workspace_id}/index.html
- 全栈应用：write_file → run_command → start_app(监听PORT) → /v1/app/{workspace_id}/

## 工作原则
1. 直接执行：自己有工具就直接做
2. 主动执行：不要反复确认
3. 自动纠错：出错时自动修复
4. 完整交付：给出总结和结果
5. 不要重复：已提交的任务不重复生成
6. 漫剧专属：漫剧请求只委派给漫剧创作Agent
7. **代码运行提示**：写完代码后用 bash 代码块给出运行命令（如 python xxx.py），用户可点击代码块的 ▶ 运行按钮执行`

type AgentHandler struct {
	db *gorm.DB
}

func NewAgentHandler(db *gorm.DB) *AgentHandler {
	return &AgentHandler{db: db}
}

type CreateAgentRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	ModelID      string `json:"model_id"`
	Tools        string `json:"tools"`
	Config       string `json:"config"`
	IsPublic     bool   `json:"is_public"`
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
		UserID:       userID,
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		ModelID:      req.ModelID,
		Tools:        tools,
		Config:       config,
		IsPublic:     req.IsPublic,
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

	c.JSON(http.StatusCreated, agent)
}

func (h *AgentHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var agent model.Agent
	if err := h.db.Where("id = ? AND (user_id = ? OR is_public = ?)", id, userID, true).First(&agent).Error; err != nil {
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
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		ModelID:      req.ModelID,
		Tools:        req.Tools,
		Config:       req.Config,
		IsPublic:     req.IsPublic,
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
		"name":          agent.Name,
		"description":   agent.Description,
		"system_prompt": agent.SystemPrompt,
		"model_id":      agent.ModelID,
		"tools":         agent.Tools,
		"is_public":     agent.IsPublic,
		"version":       "1.0",
		"platform":      "starclaw",
	}

	c.Header("Content-Disposition", "attachment; filename=agent_"+agent.Name+".json")
	c.JSON(http.StatusOK, export)
}

func (h *AgentHandler) Import(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
		ModelID      string `json:"model_id"`
		Tools        string `json:"tools"`
		IsPublic     bool   `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent := model.Agent{
		UserID:       userID,
		Name:         req.Name + " (导入)",
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		ModelID:      req.ModelID,
		Tools:        req.Tools,
		IsPublic:     req.IsPublic,
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
