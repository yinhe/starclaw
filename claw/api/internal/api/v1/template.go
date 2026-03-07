package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type TemplateHandler struct {
	db *gorm.DB
}

func NewTemplateHandler(db *gorm.DB) *TemplateHandler {
	return &TemplateHandler{db: db}
}

// List returns marketplace templates with optional category/search filter
func (h *TemplateHandler) List(c *gin.Context) {
	category := c.Query("category")
	search := c.Query("q")
	featured := c.Query("featured")

	q := h.db.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username")
	}).Order("featured DESC, install_count DESC, created_at DESC")

	if category != "" {
		q = q.Where("category = ?", category)
	}
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if featured == "true" {
		q = q.Where("featured = ?", true)
	}

	var templates []model.AgentTemplate
	if err := q.Limit(100).Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// Get returns a single template by ID
func (h *TemplateHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var tpl model.AgentTemplate
	if err := h.db.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username")
	}).First(&tpl, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": tpl})
}

// Publish creates a template from the user's agent
func (h *TemplateHandler) Publish(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		AgentID     string `json:"agent_id" binding:"required"`
		Category    string `json:"category"`
		Tags        string `json:"tags"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var agent model.Agent
	if err := h.db.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	desc := req.Description
	if desc == "" {
		desc = agent.Description
	}

	tpl := model.AgentTemplate{
		AuthorID:     userID,
		Name:         agent.Name,
		Description:  desc,
		Category:     req.Category,
		Tags:         req.Tags,
		SystemPrompt: agent.SystemPrompt,
		Tools:        agent.Tools,
		Config:       agent.Config,
		Icon:         req.Icon,
	}
	if tpl.Tags == "" {
		tpl.Tags = "[]"
	}
	if tpl.Category == "" {
		tpl.Category = "assistant"
	}

	if err := h.db.Create(&tpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish template"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"template": tpl})
}

// Install creates an agent from a template for the current user
func (h *TemplateHandler) Install(c *gin.Context) {
	userID := c.GetString("user_id")
	tplID := c.Param("id")

	var tpl model.AgentTemplate
	if err := h.db.First(&tpl, "id = ?", tplID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	agent := model.Agent{
		UserID:       userID,
		Name:         tpl.Name,
		Description:  tpl.Description,
		SystemPrompt: tpl.SystemPrompt,
		Tools:        tpl.Tools,
		Config:       tpl.Config,
	}
	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to install template"})
		return
	}

	// Increment install count
	h.db.Model(&tpl).UpdateColumn("install_count", gorm.Expr("install_count + 1"))

	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

// Rate adds a rating to a template
func (h *TemplateHandler) Rate(c *gin.Context) {
	tplID := c.Param("id")
	var req struct {
		Rating float64 `json:"rating" binding:"required,min=1,max=5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tpl model.AgentTemplate
	if err := h.db.First(&tpl, "id = ?", tplID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	// Simple running average
	newCount := tpl.RatingCount + 1
	newRating := (tpl.Rating*float64(tpl.RatingCount) + req.Rating) / float64(newCount)

	h.db.Model(&tpl).Updates(map[string]interface{}{
		"rating":       newRating,
		"rating_count": newCount,
	})

	c.JSON(http.StatusOK, gin.H{"rating": newRating, "rating_count": newCount})
}

// Categories returns available categories
func (h *TemplateHandler) Categories(c *gin.Context) {
	categories := []gin.H{
		{"id": "assistant", "name": "通用助手", "name_en": "Assistant", "icon": "Bot"},
		{"id": "coding", "name": "编程开发", "name_en": "Coding", "icon": "Code2"},
		{"id": "writing", "name": "写作创作", "name_en": "Writing", "icon": "PenTool"},
		{"id": "data", "name": "数据分析", "name_en": "Data Analysis", "icon": "BarChart3"},
		{"id": "creative", "name": "创意设计", "name_en": "Creative", "icon": "Palette"},
		{"id": "devops", "name": "运维部署", "name_en": "DevOps", "icon": "Server"},
		{"id": "research", "name": "学术研究", "name_en": "Research", "icon": "BookOpen"},
		{"id": "business", "name": "商业办公", "name_en": "Business", "icon": "Briefcase"},
	}
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// SeedBuiltinTemplates seeds default templates on first run
func SeedBuiltinTemplates(db *gorm.DB) {
	var count int64
	db.Model(&model.AgentTemplate{}).Where("is_builtin = ?", true).Count(&count)
	if count > 0 {
		return
	}

	templates := []model.AgentTemplate{
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "全栈开发助手",
			Description: "精通前后端开发的全栈工程师，擅长 React/Vue/Go/Python/Node.js，能够帮你设计架构、编写代码、调试问题。",
			Category:    "coding",
			Tags:        `["fullstack","react","go","python","nodejs"]`,
			SystemPrompt: "你是一位经验丰富的全栈开发工程师，精通以下技术栈：\n- 前端：React, Vue.js, TypeScript, Tailwind CSS\n- 后端：Go (Gin), Python (FastAPI), Node.js (Express)\n- 数据库：MySQL, PostgreSQL, Redis, MongoDB\n- DevOps：Docker, Kubernetes, CI/CD\n\n你的工作方式：\n1. 先理解需求，确认技术选型\n2. 给出清晰的架构设计\n3. 编写高质量、可维护的代码\n4. 考虑错误处理和边界情况\n5. 提供测试建议",
			Tools:       `["web_search","code_sandbox","browser"]`,
			Config:      `{"temperature":0.3,"max_tokens":4096}`,
			Icon:        "Code2",
			Featured:    true,
			IsBuiltin:   true,
		},
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "学术论文助手",
			Description: "帮助撰写、润色和翻译学术论文，支持 APA/MLA/Chicago 引用格式，提供文献综述和研究方法指导。",
			Category:    "research",
			Tags:        `["academic","paper","research","translation"]`,
			SystemPrompt: "你是一位资深学术写作助手，具有以下能力：\n- 帮助撰写学术论文各部分（摘要、引言、方法、结果、讨论）\n- 润色英文学术写作，提升语言质量\n- 支持 APA, MLA, Chicago 引用格式\n- 提供文献综述框架和研究方法建议\n- 中英文学术翻译\n\n请始终保持学术严谨性，注明引用来源，避免抄袭。",
			Tools:       `["web_search"]`,
			Config:      `{"temperature":0.2,"max_tokens":4096}`,
			Icon:        "BookOpen",
			Featured:    true,
			IsBuiltin:   true,
		},
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "数据分析师",
			Description: "专业数据分析师，能够帮你进行数据清洗、可视化、统计分析和机器学习建模。支持 Python/SQL。",
			Category:    "data",
			Tags:        `["data","python","sql","visualization","ml"]`,
			SystemPrompt: "你是一位专业的数据分析师，精通：\n- Python 数据分析（Pandas, NumPy, Scikit-learn）\n- 数据可视化（Matplotlib, Seaborn, Plotly）\n- SQL 查询优化\n- 统计分析和假设检验\n- 机器学习建模\n\n工作流程：\n1. 理解数据和业务问题\n2. 数据探索和清洗\n3. 特征工程\n4. 分析/建模\n5. 可视化呈现结果\n6. 提供可执行的业务建议",
			Tools:       `["code_sandbox","web_search"]`,
			Config:      `{"temperature":0.2,"max_tokens":4096}`,
			Icon:        "BarChart3",
			Featured:    true,
			IsBuiltin:   true,
		},
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "创意写作家",
			Description: "帮你创作小说、诗歌、剧本、广告文案等各类创意内容，支持多种风格和语调。",
			Category:    "writing",
			Tags:        `["creative","writing","copywriting","story"]`,
			SystemPrompt: "你是一位才华横溢的创意写作家，擅长：\n- 小说和短篇故事创作\n- 诗歌和散文\n- 广告文案和品牌故事\n- 剧本和对话写作\n- 社交媒体内容\n\n你能根据用户需求调整风格（幽默、正式、感性、简洁等），并且善于运用比喻、排比等修辞手法。每次创作前，先了解目标受众和使用场景。",
			Tools:       `["web_search"]`,
			Config:      `{"temperature":0.8,"max_tokens":4096}`,
			Icon:        "PenTool",
			Featured:    true,
			IsBuiltin:   true,
		},
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "DevOps 运维专家",
			Description: "精通 Docker/K8s/CI/CD 的运维专家，帮你设计部署架构、编写配置文件、排查线上问题。",
			Category:    "devops",
			Tags:        `["docker","kubernetes","cicd","linux","monitoring"]`,
			SystemPrompt: "你是一位资深 DevOps 工程师，精通：\n- 容器化：Docker, Docker Compose, Podman\n- 编排：Kubernetes, Helm, ArgoCD\n- CI/CD：GitHub Actions, GitLab CI, Jenkins\n- 监控：Prometheus, Grafana, ELK\n- 云平台：AWS, GCP, 阿里云\n- Linux 系统管理和网络\n\n你注重：安全性、高可用、自动化、可观测性。提供生产级别的配置和最佳实践。",
			Tools:       `["code_sandbox","web_search"]`,
			Config:      `{"temperature":0.2,"max_tokens":4096}`,
			Icon:        "Server",
			Featured:    false,
			IsBuiltin:   true,
		},
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "产品经理助手",
			Description: "帮你撰写 PRD、用户故事、竞品分析，进行需求优先级排序和产品规划。",
			Category:    "business",
			Tags:        `["product","prd","user_story","analysis"]`,
			SystemPrompt: "你是一位经验丰富的产品经理，擅长：\n- 撰写产品需求文档（PRD）\n- 用户故事编写和验收标准\n- 竞品分析和市场调研\n- 需求优先级排序（RICE/MoSCoW）\n- 产品路线图规划\n- 数据驱动决策\n\n你善于站在用户角度思考，用数据说话，并且能够清晰地与开发团队沟通技术可行性。",
			Tools:       `["web_search"]`,
			Config:      `{"temperature":0.4,"max_tokens":4096}`,
			Icon:        "Briefcase",
			Featured:    false,
			IsBuiltin:   true,
		},
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "UI/UX 设计顾问",
			Description: "提供界面设计建议、配色方案、组件规范，帮你打造出色的用户体验。",
			Category:    "creative",
			Tags:        `["design","ui","ux","color","component"]`,
			SystemPrompt: "你是一位资深 UI/UX 设计顾问，精通：\n- 界面设计原则和设计系统\n- 配色理论和色彩搭配\n- 响应式设计和移动端适配\n- 用户体验研究和可用性测试\n- Figma/Sketch 组件规范\n- Tailwind CSS / shadcn/ui 实现\n\n你注重：\n1. 一致性和可访问性（WCAG）\n2. 视觉层次和信息架构\n3. 微交互和动画\n4. 设计到代码的高效转换",
			Tools:       `["web_search","browser"]`,
			Config:      `{"temperature":0.5,"max_tokens":4096}`,
			Icon:        "Palette",
			Featured:    false,
			IsBuiltin:   true,
		},
		{
			ID:       uuid.New().String(),
			AuthorID: "system",
			Name:     "英语口语教练",
			Description: "模拟真实对话场景练习英语口语，纠正语法错误，教授地道表达和俚语。",
			Category:    "assistant",
			Tags:        `["english","speaking","language","education"]`,
			SystemPrompt: "你是一位专业的英语口语教练。你的教学方法：\n1. 根据学生水平调整难度\n2. 模拟真实场景对话（面试、旅行、商务等）\n3. 指出语法和表达错误，给出正确示范\n4. 教授地道的英语表达和常用俚语\n5. 定期总结学习要点\n\n请用友好、鼓励的方式交流。每次对话后，简要总结学到的新表达。如果学生用中文提问，用中英双语回答。",
			Tools:       `[]`,
			Config:      `{"temperature":0.6,"max_tokens":2048}`,
			Icon:        "Bot",
			Featured:    false,
			IsBuiltin:   true,
		},
	}

	for i := range templates {
		db.Create(&templates[i])
	}
}
