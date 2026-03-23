package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"starclaw.net/nydus/api/internal/database"
	"starclaw.net/nydus/api/internal/model"
)

// GrantAccess grants a node access to a repository.
// POST /api/repos/:name/access
//
//	{ "node_id": "claw:xxx", "level": "read|write|owner", "team_id": "..." }
func GrantAccess(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
		TeamID string `json:"team_id"`
		Level  string `json:"level" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.NodeID == "" && req.TeamID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id or team_id required"})
		return
	}
	validLevels := map[string]bool{"owner": true, "write": true, "read": true}
	if !validLevels[req.Level] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level must be owner/write/read"})
		return
	}

	grantedBy, _ := c.Get("node_id")
	grantedByStr := ""
	if grantedBy != nil {
		grantedByStr = grantedBy.(string)
	}

	// Upsert: if access already exists for this node+repo, update level
	if req.NodeID != "" {
		var existing model.RepoAccess
		if err := database.DB.Where("repo_id = ? AND node_id = ?", repo.ID, req.NodeID).First(&existing).Error; err == nil {
			database.DB.Model(&existing).Update("level", req.Level)
			c.JSON(http.StatusOK, gin.H{"access": existing, "message": "access updated"})
			return
		}
	}

	access := model.RepoAccess{
		RepoID:    repo.ID,
		NodeID:    req.NodeID,
		TeamID:    req.TeamID,
		Level:     req.Level,
		GrantedBy: grantedByStr,
	}
	if err := database.DB.Create(&access).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to grant access"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"access": access, "message": fmt.Sprintf("granted %s access to %s", req.Level, repoName)})
}

// ListAccess lists all access grants for a repository.
// GET /api/repos/:name/access
func ListAccess(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var grants []model.RepoAccess
	database.DB.Where("repo_id = ?", repo.ID).Order("created_at DESC").Find(&grants)
	c.JSON(http.StatusOK, gin.H{"access": grants, "total": len(grants)})
}

// RevokeAccess removes a node's access to a repository.
// DELETE /api/repos/:name/access/:node_id
func RevokeAccess(c *gin.Context) {
	repoName := c.Param("name")
	nodeID := c.Param("node_id")

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	result := database.DB.Where("repo_id = ? AND node_id = ?", repo.ID, nodeID).Delete(&model.RepoAccess{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "access not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("revoked access for %s on %s", nodeID, repoName)})
}

// CheckAccess returns the effective access level for a node on a repo.
// GET /node/repos/:name/access  (authenticated via ClawAuth)
func CheckAccess(c *gin.Context) {
	repoName := c.Param("name")
	nodeID, _ := c.Get("node_id")
	if nodeID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	level := resolveAccessLevel(repo, nodeID.(string))
	c.JSON(http.StatusOK, gin.H{
		"repo":   repoName,
		"node":   nodeID,
		"level":  level,
		"can_read":  level != "none",
		"can_write": level == "write" || level == "owner",
		"can_admin": level == "owner",
	})
}

// resolveAccessLevel returns the highest access level for a node on a repo.
// Priority: direct node grant > team grant > public read > none.
func resolveAccessLevel(repo model.NydusRepo, nodeID string) string {
	// Owner of repo always has full access
	if repo.OwnerNodeID == nodeID {
		return "owner"
	}

	// Direct node grant
	var directAccess model.RepoAccess
	if err := database.DB.Where("repo_id = ? AND node_id = ?", repo.ID, nodeID).First(&directAccess).Error; err == nil {
		return directAccess.Level
	}

	// Team-level grant (find node's team, check team access)
	var node model.NydusNode
	if err := database.DB.Where("node_id = ?", nodeID).First(&node).Error; err == nil && node.TeamID != "" {
		var teamAccess model.RepoAccess
		if err := database.DB.Where("repo_id = ? AND team_id = ?", repo.ID, node.TeamID).First(&teamAccess).Error; err == nil {
			return teamAccess.Level
		}
		// Same team as repo owner → read
		if repo.TeamID != "" && repo.TeamID == node.TeamID {
			return "read"
		}
	}

	// Public repos → read for any authenticated node
	if repo.Public {
		return "read"
	}

	return "none"
}
