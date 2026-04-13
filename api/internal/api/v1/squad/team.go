package squad

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// TeamHandler manages local multi-agent Teams (Hexad collaboration layer).
type TeamHandler struct {
	db *gorm.DB
}

func NewTeamHandler(db *gorm.DB) *TeamHandler {
	return &TeamHandler{db: db}
}

// List returns all teams for the current user.
func (h *TeamHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	var teams []model.Team
	h.db.Preload("Members").Preload("Members.Agent", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, name, description")
	}).Where("user_id = ?", userID).Order("created_at DESC").Find(&teams)
	c.JSON(http.StatusOK, gin.H{"teams": teams})
}

// Create creates a new team with members.
func (h *TeamHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name          string `json:"name" binding:"required"`
		Description   string `json:"description"`
		Icon          string `json:"icon"`
		CoordinatorID string `json:"coordinator_id" binding:"required"`
		Topology      string `json:"topology"`
		TemplateID    string `json:"template_id"`
		Members       []struct {
			AgentID   string `json:"agent_id" binding:"required"`
			Role      string `json:"role"`
			Specialty string `json:"specialty"`
			Order     int    `json:"order"`
		} `json:"members"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	topology := req.Topology
	if topology == "" {
		topology = "sequential"
	}

	team := model.Team{
		Name:          req.Name,
		Description:   req.Description,
		Icon:          req.Icon,
		UserID:        userID,
		CoordinatorID: req.CoordinatorID,
		Topology:      topology,
		TemplateID:    req.TemplateID,
		Status:        "active",
	}

	tx := h.db.Begin()
	if err := tx.Create(&team).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create team"})
		return
	}

	// Add coordinator as first member
	coordMember := model.TeamMember{
		TeamID:    team.ID,
		AgentID:   req.CoordinatorID,
		Role:      "coordinator",
		Specialty: "团长",
		Order:     0,
	}
	tx.Create(&coordMember)

	// Add other members
	for _, m := range req.Members {
		if m.AgentID == req.CoordinatorID {
			continue // skip duplicate coordinator
		}
		role := m.Role
		if role == "" {
			role = "member"
		}
		member := model.TeamMember{
			TeamID:    team.ID,
			AgentID:   m.AgentID,
			Role:      role,
			Specialty: m.Specialty,
			Order:     m.Order,
		}
		tx.Create(&member)
	}

	tx.Commit()

	// Reload with members
	h.db.Preload("Members").Preload("Members.Agent", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, name, description")
	}).First(&team, "id = ?", team.ID)

	c.JSON(http.StatusCreated, gin.H{"team": team})
}

// Get returns a single team with members.
func (h *TeamHandler) Get(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	var team model.Team
	if err := h.db.Preload("Members").Preload("Members.Agent", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, name, description")
	}).Where("id = ? AND user_id = ?", id, userID).First(&team).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"team": team})
}

// Update updates team metadata.
func (h *TeamHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	var team model.Team
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&team).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}
	var req struct {
		Name          *string `json:"name"`
		Description   *string `json:"description"`
		CoordinatorID *string `json:"coordinator_id"`
		Topology      *string `json:"topology"`
		Status        *string `json:"status"`
	}
	c.ShouldBindJSON(&req)
	if req.Name != nil {
		team.Name = *req.Name
	}
	if req.Description != nil {
		team.Description = *req.Description
	}
	if req.CoordinatorID != nil {
		team.CoordinatorID = *req.CoordinatorID
	}
	if req.Topology != nil {
		team.Topology = *req.Topology
	}
	if req.Status != nil {
		team.Status = *req.Status
	}
	h.db.Save(&team)
	c.JSON(http.StatusOK, gin.H{"team": team})
}

// Delete deletes a team and its members.
func (h *TeamHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	var team model.Team
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&team).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}
	h.db.Where("team_id = ?", id).Delete(&model.TeamMember{})
	h.db.Delete(&team)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// AddMember adds an agent to the team.
func (h *TeamHandler) AddMember(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := c.Param("id")
	var team model.Team
	if err := h.db.Where("id = ? AND user_id = ?", teamID, userID).First(&team).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}
	var req struct {
		AgentID   string `json:"agent_id" binding:"required"`
		Specialty string `json:"specialty"`
		Order     int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	member := model.TeamMember{
		TeamID:    teamID,
		AgentID:   req.AgentID,
		Role:      "member",
		Specialty: req.Specialty,
		Order:     req.Order,
	}
	h.db.Create(&member)
	c.JSON(http.StatusCreated, gin.H{"member": member})
}

// RemoveMember removes an agent from the team.
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := c.Param("id")
	memberID := c.Param("member_id")
	var team model.Team
	if err := h.db.Where("id = ? AND user_id = ?", teamID, userID).First(&team).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}
	h.db.Where("id = ? AND team_id = ?", memberID, teamID).Delete(&model.TeamMember{})
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

// TeamTemplate defines a pre-built team configuration.
type TeamTemplate struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Icon        string             `json:"icon"`
	Topology    string             `json:"topology"`
	Roles       []TeamTemplateRole `json:"roles"`
}

type TeamTemplateRole struct {
	Specialty string `json:"specialty"`
	Role      string `json:"role"`
	AgentHint string `json:"agent_hint"` // suggested agent name keyword
}

// ListTemplates returns pre-built team templates.
func (h *TeamHandler) ListTemplates(c *gin.Context) {
	templates := []TeamTemplate{
		{
			ID: "dev", Name: "开发团队", Description: "架构师 + 编码 + 测试 + 代码审查", Icon: "Code2", Topology: "sequential",
			Roles: []TeamTemplateRole{
				{Specialty: "架构设计与任务分解", Role: "coordinator", AgentHint: "编程"},
				{Specialty: "代码编写", Role: "member", AgentHint: "编程"},
				{Specialty: "测试验证与质量检查", Role: "member", AgentHint: "编程"},
			},
		},
		{
			ID: "content", Name: "内容创作团队", Description: "策划 + 写作 + 研究 + 编辑", Icon: "PenTool", Topology: "sequential",
			Roles: []TeamTemplateRole{
				{Specialty: "内容策划与选题", Role: "coordinator", AgentHint: "商业"},
				{Specialty: "内容写作", Role: "member", AgentHint: "写作"},
				{Specialty: "信息搜集与事实核查", Role: "member", AgentHint: "研究"},
			},
		},
		{
			ID: "research", Name: "研究分析团队", Description: "调研 + 数据分析 + 报告撰写", Icon: "Search", Topology: "sequential",
			Roles: []TeamTemplateRole{
				{Specialty: "调研规划与信息搜集", Role: "coordinator", AgentHint: "研究"},
				{Specialty: "数据分析与可视化", Role: "member", AgentHint: "数据"},
				{Specialty: "报告撰写与排版", Role: "member", AgentHint: "写作"},
			},
		},
		{
			ID: "video", Name: "视频制作团队", Description: "导演 + 编剧 + 音乐 + 剪辑", Icon: "Film", Topology: "sequential",
			Roles: []TeamTemplateRole{
				{Specialty: "导演统筹与分镜", Role: "coordinator", AgentHint: "短剧"},
				{Specialty: "音乐创作", Role: "member", AgentHint: "音乐"},
				{Specialty: "视频生成与合成", Role: "member", AgentHint: "视频"},
			},
		},
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}
