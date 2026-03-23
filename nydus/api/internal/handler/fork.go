package handler

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
	"starclaw.net/nydus/api/internal/config"
	"starclaw.net/nydus/api/internal/database"
	"starclaw.net/nydus/api/internal/model"
)

// ForkRepo creates a server-side fork of an existing repository.
// POST /api/repos/:name/fork
//
//	{ "new_name": "my-fork", "team_id": "..." }
//
// The fork is a bare clone of the original repo on disk,
// and a new NydusRepo row with forked_from set to the parent's ID.
func ForkRepo(c *gin.Context) {
	parentName := c.Param("name")

	// Verify parent exists in DB
	var parent model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", parentName, "active").First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source repo not found"})
		return
	}

	var req struct {
		NewName string `json:"new_name" binding:"required"`
		TeamID  string `json:"team_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check fork name not taken
	var existing model.NydusRepo
	if err := database.DB.Where("name = ?", req.NewName).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("repo '%s' already exists", req.NewName)})
		return
	}

	// Determine owner
	ownerNodeID := ""
	if v, exists := c.Get("node_id"); exists && v != nil {
		ownerNodeID = v.(string)
	}

	// Git: clone --bare the parent repo on disk
	srcPath := RepoPath(parentName)
	dstPath := RepoPath(req.NewName)
	cmd := exec.Command("git", "clone", "--bare", srcPath, dstPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[nydus] fork git clone failed: %v\n%s", err, out)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "git clone --bare failed"})
		return
	}

	// Install post-receive hook on fork
	InstallHook(req.NewName)

	// Create DB record
	fork := model.NydusRepo{
		Name:          req.NewName,
		Description:   fmt.Sprintf("Fork of %s", parentName),
		OwnerNodeID:   ownerNodeID,
		TeamID:        req.TeamID,
		Public:        parent.Public, // inherit visibility
		DefaultBranch: parent.DefaultBranch,
		ForkedFrom:    parent.ID,
		Source:        "dynamic",
		Status:        "active",
	}
	if err := database.DB.Create(&fork).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create fork record: " + err.Error()})
		return
	}

	// Auto-grant owner access
	if ownerNodeID != "" {
		database.DB.Create(&model.RepoAccess{
			RepoID:    fork.ID,
			NodeID:    ownerNodeID,
			Level:     "owner",
			GrantedBy: ownerNodeID,
		})
	}

	log.Printf("[nydus] forked %s → %s (owner=%s)", parentName, req.NewName, ownerNodeID)
	c.JSON(http.StatusCreated, gin.H{
		"repo":      fork,
		"parent":    parentName,
		"ssh_url":   fmt.Sprintf("%s@nydus.starclaw.net:%s.git", config.C.Server.SSHUser, req.NewName),
		"https_url": fmt.Sprintf("https://nydus.starclaw.net/%s.git", req.NewName),
		"message":   fmt.Sprintf("forked %s → %s", parentName, req.NewName),
	})
}

// ListForks returns all forks of a repository.
// GET /api/repos/:name/forks
func ListForks(c *gin.Context) {
	parentName := c.Param("name")

	var parent model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", parentName, "active").First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var forks []model.NydusRepo
	database.DB.Where("forked_from = ? AND status = ?", parent.ID, "active").Order("created_at DESC").Find(&forks)

	c.JSON(http.StatusOK, gin.H{"forks": forks, "total": len(forks)})
}
