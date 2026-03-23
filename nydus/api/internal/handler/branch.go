package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"starclaw.net/nydus/api/internal/database"
	"starclaw.net/nydus/api/internal/model"
)

// SetBranchProtection creates or updates branch protection rules.
// POST /api/repos/:name/branches/protect
//
//	{
//	  "branch": "master",
//	  "require_pr": true,
//	  "require_review": true,
//	  "min_reviewers": 1,
//	  "require_ci": false,
//	  "allow_force_push": false,
//	  "allow_delete": false,
//	  "restrict_push_to": "claw:aaa,claw:bbb"
//	}
func SetBranchProtection(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var req struct {
		Branch         string `json:"branch" binding:"required"`
		RequirePR      *bool  `json:"require_pr"`
		RequireReview  *bool  `json:"require_review"`
		MinReviewers   *int   `json:"min_reviewers"`
		RequireCI      *bool  `json:"require_ci"`
		AllowForcePush *bool  `json:"allow_force_push"`
		AllowDelete    *bool  `json:"allow_delete"`
		RestrictPushTo string `json:"restrict_push_to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdBy := ""
	if v, exists := c.Get("node_id"); exists && v != nil {
		createdBy = v.(string)
	}

	// Upsert: find existing or create new
	var bp model.BranchProtection
	if err := database.DB.Where("repo_id = ? AND branch = ?", repo.ID, req.Branch).First(&bp).Error; err != nil {
		// Create new
		bp = model.BranchProtection{
			RepoID:    repo.ID,
			Branch:    req.Branch,
			CreatedBy: createdBy,
		}
	}

	// Apply fields (only update if provided)
	if req.RequirePR != nil {
		bp.RequirePR = *req.RequirePR
	}
	if req.RequireReview != nil {
		bp.RequireReview = *req.RequireReview
	}
	if req.MinReviewers != nil {
		bp.MinReviewers = *req.MinReviewers
	}
	if req.RequireCI != nil {
		bp.RequireCI = *req.RequireCI
	}
	if req.AllowForcePush != nil {
		bp.AllowForcePush = *req.AllowForcePush
	}
	if req.AllowDelete != nil {
		bp.AllowDelete = *req.AllowDelete
	}
	if req.RestrictPushTo != "" {
		bp.RestrictPushTo = req.RestrictPushTo
	}

	if bp.ID == "" {
		if err := database.DB.Create(&bp).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create protection rule"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"protection": bp, "message": "branch protection created"})
	} else {
		database.DB.Save(&bp)
		c.JSON(http.StatusOK, gin.H{"protection": bp, "message": "branch protection updated"})
	}
}

// ListBranchProtections returns all protection rules for a repo.
// GET /api/repos/:name/branches/protect
func ListBranchProtections(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var rules []model.BranchProtection
	database.DB.Where("repo_id = ?", repo.ID).Order("branch ASC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"protections": rules, "total": len(rules)})
}

// DeleteBranchProtection removes a protection rule.
// DELETE /api/repos/:name/branches/protect/:branch
func DeleteBranchProtection(c *gin.Context) {
	repoName := c.Param("name")
	branch := c.Param("branch")

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	result := database.DB.Where("repo_id = ? AND branch = ?", repo.ID, branch).Delete(&model.BranchProtection{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "protection rule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "branch protection removed for " + branch})
}

// GetBranchProtection returns protection info for a specific branch.
// Used internally by PR merge logic to enforce rules.
func GetBranchProtection(repoID, branch string) *model.BranchProtection {
	var bp model.BranchProtection
	if err := database.DB.Where("repo_id = ? AND branch = ?", repoID, branch).First(&bp).Error; err != nil {
		return nil
	}
	return &bp
}
