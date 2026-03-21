package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/middleware"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

type TeamHandler struct {
	db *gorm.DB
}

func NewTeamHandler(db *gorm.DB) *TeamHandler {
	return &TeamHandler{db: db}
}

// ---------- POST /brood/teams ----------

func (h *TeamHandler) CreateTeam(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		DisplayName string `json:"display_name"`
		MaxNodes    int    `json:"max_nodes"`
		MaxTokens   int64  `json:"max_tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team := model.Team{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		MaxNodes:    req.MaxNodes,
		MaxTokens:   req.MaxTokens,
	}
	if err := h.db.Create(&team).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "team name already exists"})
		return
	}

	audit(h.db, c, "create_team", team.ID, "team created: "+req.Name)
	c.JSON(http.StatusCreated, gin.H{"team": team})
}

// ---------- GET /brood/teams ----------

func (h *TeamHandler) ListTeams(c *gin.Context) {
	var teams []model.Team
	q := middleware.TeamScope(c, h.db)
	q.Order("name ASC").Find(&teams)

	// Enrich with node counts
	type result struct {
		Team  string
		Count int64
	}
	var counts []result
	h.db.Model(&model.ClawNode{}).Select("team, COUNT(*) as count").Where("team != ''").Group("team").Scan(&counts)
	countMap := make(map[string]int64)
	for _, r := range counts {
		countMap[r.Team] = r.Count
	}

	type teamWithCount struct {
		model.Team
		NodeCount int64 `json:"node_count"`
	}
	out := make([]teamWithCount, len(teams))
	for i, t := range teams {
		out[i] = teamWithCount{Team: t, NodeCount: countMap[t.Name]}
	}

	c.JSON(http.StatusOK, gin.H{"teams": out, "total": len(out)})
}

// ---------- GET /brood/teams/:id ----------

func (h *TeamHandler) GetTeam(c *gin.Context) {
	id := c.Param("id")
	var team model.Team
	if err := h.db.First(&team, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	var nodeCount int64
	h.db.Model(&model.ClawNode{}).Where("team = ?", team.Name).Count(&nodeCount)

	c.JSON(http.StatusOK, gin.H{"team": team, "node_count": nodeCount})
}

// ---------- PUT /brood/teams/:id ----------

func (h *TeamHandler) UpdateTeam(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		DisplayName *string `json:"display_name"`
		MaxNodes    *int    `json:"max_nodes"`
		MaxTokens   *int64  `json:"max_tokens"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.MaxNodes != nil {
		updates["max_nodes"] = *req.MaxNodes
	}
	if req.MaxTokens != nil {
		updates["max_tokens"] = *req.MaxTokens
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if err := h.db.Model(&model.Team{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update team"})
		return
	}

	audit(h.db, c, "update_team", id, "team updated")
	c.JSON(http.StatusOK, gin.H{"message": "team updated"})
}

// ---------- DELETE /brood/teams/:id ----------

func (h *TeamHandler) DeleteTeam(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.Team{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete team"})
		return
	}
	audit(h.db, c, "delete_team", id, "team deleted")
	c.JSON(http.StatusOK, gin.H{"message": "team deleted"})
}

// ---------- Admin user management ----------

func (h *TeamHandler) CreateAdmin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role"`
		TeamID   string `json:"team_id"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Role == "" {
		req.Role = "viewer"
	}
	if _, ok := model.RolePermissions[req.Role]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, must be: superadmin, admin, operator, viewer"})
		return
	}

	user := model.AdminUser{
		Username:     req.Username,
		PasswordHash: hashPassword(req.Password),
		Role:         req.Role,
		TeamID:       req.TeamID,
		Email:        req.Email,
	}
	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	audit(h.db, c, "create_admin", user.ID, "admin user created: "+req.Username+" ("+req.Role+")")
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func (h *TeamHandler) ListAdmins(c *gin.Context) {
	var users []model.AdminUser
	h.db.Order("username ASC").Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users, "total": len(users)})
}

func (h *TeamHandler) DeleteAdmin(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.AdminUser{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete admin"})
		return
	}
	audit(h.db, c, "delete_admin", id, "admin user deleted")
	c.JSON(http.StatusOK, gin.H{"message": "admin deleted"})
}

// ---------- POST /brood/auth/login ----------

func (h *TeamHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash := hashPassword(req.Password)
	var user model.AdminUser
	if err := h.db.Where("username = ? AND password_hash = ?", req.Username, hash).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate API token — stored in token_hash (password_hash stays intact for re-login)
	token := generateToken(32)
	now := time.Now()
	h.db.Model(&user).Updates(map[string]interface{}{
		"token_hash":    hashPassword(token),
		"last_login_at": &now,
	})

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user":    user,
		"message": "login successful",
	})
}

// helpers
func audit(db *gorm.DB, c *gin.Context, action, targetID, detail string) {
	actor := middleware.GetAdminActor(c)
	db.Create(&model.AuditLog{
		Actor:    actor,
		Action:   action,
		TargetID: targetID,
		Detail:   detail,
	})
}

func hashPassword(raw string) string {
	return middleware.HashTokenExported(raw)
}
