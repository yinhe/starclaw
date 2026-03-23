package handler

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/nydus/internal/database"
	"github.com/yinhe/starclaw/nydus/internal/model"
)

// ════════════════════════════════════════════════════════════
// CRUD
// ════════════════════════════════════════════════════════════

// CreatePR opens a new pull request.
// POST /api/repos/:name/pulls
func CreatePR(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var req struct {
		Title        string `json:"title" binding:"required"`
		Body         string `json:"body"`
		SourceRepo   string `json:"source_repo"`
		SourceBranch string `json:"source_branch" binding:"required"`
		TargetBranch string `json:"target_branch" binding:"required"`
		Labels       string `json:"labels"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto-increment PR number within this repo
	var maxNum int
	database.DB.Model(&model.PullRequest{}).Where("repo_id = ?", repo.ID).
		Select("COALESCE(MAX(number), 0)").Scan(&maxNum)

	authorNode := ""
	if v, exists := c.Get("node_id"); exists && v != nil {
		authorNode = v.(string)
	}

	pr := model.PullRequest{
		RepoID:       repo.ID,
		Number:       maxNum + 1,
		Title:        req.Title,
		Body:         req.Body,
		AuthorNodeID: authorNode,
		SourceRepo:   req.SourceRepo,
		SourceBranch: req.SourceBranch,
		TargetBranch: req.TargetBranch,
		Status:       "open",
		Labels:       req.Labels,
	}
	if err := database.DB.Create(&pr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create PR"})
		return
	}

	log.Printf("[nydus] PR #%d created on %s: %s → %s by %s", pr.Number, repoName, req.SourceBranch, req.TargetBranch, authorNode)
	c.JSON(http.StatusCreated, gin.H{"pull_request": pr})
}

// ListPRs returns pull requests for a repo.
// GET /api/repos/:name/pulls?status=open
func ListPRs(c *gin.Context) {
	repoName := c.Param("name")
	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	status := c.DefaultQuery("status", "open")
	query := database.DB.Where("repo_id = ?", repo.ID)
	if status != "all" {
		query = query.Where("status = ?", status)
	}

	var prs []model.PullRequest
	query.Order("number DESC").Find(&prs)
	c.JSON(http.StatusOK, gin.H{"pull_requests": prs, "total": len(prs)})
}

// GetPR returns a single pull request by number.
// GET /api/repos/:name/pulls/:number
func GetPR(c *gin.Context) {
	repoName := c.Param("name")
	number, _ := strconv.Atoi(c.Param("number"))

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var pr model.PullRequest
	if err := database.DB.Where("repo_id = ? AND number = ?", repo.ID, number).First(&pr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	// Fetch reviews
	var reviews []model.PRReview
	database.DB.Where("pr_id = ?", pr.ID).Order("created_at ASC").Find(&reviews)

	c.JSON(http.StatusOK, gin.H{"pull_request": pr, "reviews": reviews})
}

// UpdatePR updates PR title/body/labels.
// PUT /api/repos/:name/pulls/:number
func UpdatePR(c *gin.Context) {
	repoName := c.Param("name")
	number, _ := strconv.Atoi(c.Param("number"))

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var pr model.PullRequest
	if err := database.DB.Where("repo_id = ? AND number = ?", repo.ID, number).First(&pr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	var req struct {
		Title  *string `json:"title"`
		Body   *string `json:"body"`
		Labels *string `json:"labels"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Body != nil {
		updates["body"] = *req.Body
	}
	if req.Labels != nil {
		updates["labels"] = *req.Labels
	}
	if len(updates) > 0 {
		database.DB.Model(&pr).Updates(updates)
	}
	c.JSON(http.StatusOK, gin.H{"pull_request": pr})
}

// ClosePR closes a PR without merging.
// POST /api/repos/:name/pulls/:number/close
func ClosePR(c *gin.Context) {
	repoName := c.Param("name")
	number, _ := strconv.Atoi(c.Param("number"))

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var pr model.PullRequest
	if err := database.DB.Where("repo_id = ? AND number = ? AND status = ?", repo.ID, number, "open").First(&pr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "open PR not found"})
		return
	}

	now := time.Now()
	database.DB.Model(&pr).Updates(map[string]interface{}{"status": "closed", "closed_at": &now})
	c.JSON(http.StatusOK, gin.H{"pull_request": pr, "message": "PR closed"})
}

// ════════════════════════════════════════════════════════════
// Diff
// ════════════════════════════════════════════════════════════

// GetPRDiff returns the unified diff for a PR.
// GET /api/repos/:name/pulls/:number/diff
func GetPRDiff(c *gin.Context) {
	repoName := c.Param("name")
	number, _ := strconv.Atoi(c.Param("number"))

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var pr model.PullRequest
	if err := database.DB.Where("repo_id = ? AND number = ?", repo.ID, number).First(&pr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	bareRepo := RepoPath(repoName)
	// git diff target...source (three-dot = merge base diff)
	cmd := exec.Command("git", "--git-dir", bareRepo, "diff", pr.TargetBranch+"..."+pr.SourceBranch)
	out, err := cmd.Output()
	if err != nil {
		// Fallback: two-dot diff
		cmd2 := exec.Command("git", "--git-dir", bareRepo, "diff", pr.TargetBranch+".."+pr.SourceBranch)
		out, err = cmd2.Output()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "diff failed"})
			return
		}
	}

	// Parse stat summary
	statCmd := exec.Command("git", "--git-dir", bareRepo, "diff", "--stat", pr.TargetBranch+"..."+pr.SourceBranch)
	statOut, _ := statCmd.Output()

	// Count files changed
	nameCmd := exec.Command("git", "--git-dir", bareRepo, "diff", "--name-only", pr.TargetBranch+"..."+pr.SourceBranch)
	nameOut, _ := nameCmd.Output()
	files := []string{}
	for _, f := range strings.Split(strings.TrimSpace(string(nameOut)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"diff":          string(out),
		"stat":          string(statOut),
		"files_changed": files,
		"file_count":    len(files),
	})
}

// ════════════════════════════════════════════════════════════
// Review
// ════════════════════════════════════════════════════════════

// AddReview adds a review to a PR.
// POST /api/repos/:name/pulls/:number/reviews
func AddReview(c *gin.Context) {
	repoName := c.Param("name")
	number, _ := strconv.Atoi(c.Param("number"))

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var pr model.PullRequest
	if err := database.DB.Where("repo_id = ? AND number = ?", repo.ID, number).First(&pr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	var req struct {
		Verdict    string `json:"verdict" binding:"required"` // approve / request_changes / comment
		Body       string `json:"body"`
		FilePath   string `json:"file_path"`
		LineNumber int    `json:"line_number"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validVerdicts := map[string]bool{"approve": true, "request_changes": true, "comment": true}
	if !validVerdicts[req.Verdict] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verdict must be approve/request_changes/comment"})
		return
	}

	reviewerNode := ""
	if v, exists := c.Get("node_id"); exists && v != nil {
		reviewerNode = v.(string)
	}

	review := model.PRReview{
		PRID:         pr.ID,
		ReviewerNode: reviewerNode,
		Verdict:      req.Verdict,
		Body:         req.Body,
		FilePath:     req.FilePath,
		LineNumber:   req.LineNumber,
	}
	database.DB.Create(&review)

	log.Printf("[nydus] PR #%d on %s: review by %s → %s", number, repoName, reviewerNode, req.Verdict)
	c.JSON(http.StatusCreated, gin.H{"review": review})
}

// ════════════════════════════════════════════════════════════
// Merge
// ════════════════════════════════════════════════════════════

// MergePR merges a pull request.
// POST /api/repos/:name/pulls/:number/merge
func MergePR(c *gin.Context) {
	repoName := c.Param("name")
	number, _ := strconv.Atoi(c.Param("number"))

	var repo model.NydusRepo
	if err := database.DB.Where("name = ? AND status = ?", repoName, "active").First(&repo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var pr model.PullRequest
	if err := database.DB.Where("repo_id = ? AND number = ? AND status = ?", repo.ID, number, "open").First(&pr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "open PR not found"})
		return
	}

	// Check branch protection
	if bp := GetBranchProtection(repo.ID, pr.TargetBranch); bp != nil {
		if bp.RequireReview {
			var approvalCount int64
			database.DB.Model(&model.PRReview{}).Where("pr_id = ? AND verdict = ?", pr.ID, "approve").Count(&approvalCount)
			minRequired := bp.MinReviewers
			if minRequired < 1 {
				minRequired = 1
			}
			if approvalCount < int64(minRequired) {
				c.JSON(http.StatusPreconditionFailed, gin.H{
					"error":    fmt.Sprintf("branch '%s' requires %d approval(s), got %d", pr.TargetBranch, minRequired, approvalCount),
					"required": minRequired,
					"current":  approvalCount,
				})
				return
			}
		}
		// Check no outstanding request_changes
		var changesRequested int64
		database.DB.Model(&model.PRReview{}).Where("pr_id = ? AND verdict = ?", pr.ID, "request_changes").Count(&changesRequested)
		if changesRequested > 0 {
			c.JSON(http.StatusPreconditionFailed, gin.H{"error": "changes requested — resolve before merging"})
			return
		}
	}

	var req struct {
		Strategy string `json:"strategy"` // merge / squash / rebase (default: merge)
	}
	c.ShouldBindJSON(&req)
	if req.Strategy == "" {
		req.Strategy = "merge"
	}

	mergedBy := ""
	if v, exists := c.Get("node_id"); exists && v != nil {
		mergedBy = v.(string)
	}

	bareRepo := RepoPath(repoName)
	mergeHash, err := gitMerge(bareRepo, pr.SourceBranch, pr.TargetBranch, pr.Title, req.Strategy)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "merge failed: " + err.Error()})
		return
	}

	now := time.Now()
	database.DB.Model(&pr).Updates(map[string]interface{}{
		"status":       "merged",
		"merge_commit": mergeHash,
		"merged_by":    mergedBy,
		"merged_at":    &now,
	})

	log.Printf("[nydus] PR #%d merged on %s: %s → %s (strategy=%s, by=%s)",
		number, repoName, pr.SourceBranch, pr.TargetBranch, req.Strategy, mergedBy)
	c.JSON(http.StatusOK, gin.H{
		"pull_request": pr,
		"merge_commit": mergeHash,
		"strategy":     req.Strategy,
		"message":      fmt.Sprintf("PR #%d merged successfully", number),
	})
}

// gitMerge performs the actual git merge in a bare repo via a temporary worktree.
func gitMerge(bareRepo, sourceBranch, targetBranch, title, strategy string) (string, error) {
	// For bare repos, use git update-ref approach or temporary clone.
	// Simplest: use `git merge-base` + `git merge-tree` for fast-forward detection,
	// then `git update-ref` for the actual merge.

	// Check if fast-forward is possible
	ffCmd := exec.Command("git", "--git-dir", bareRepo, "merge-base", "--is-ancestor", targetBranch, sourceBranch)
	if err := ffCmd.Run(); err == nil {
		// Fast-forward: just update the target ref to source
		sourceHash := resolveRef(bareRepo, sourceBranch)
		if sourceHash == "" {
			return "", fmt.Errorf("cannot resolve source branch %s", sourceBranch)
		}
		updateCmd := exec.Command("git", "--git-dir", bareRepo, "update-ref",
			"refs/heads/"+targetBranch, sourceHash)
		if out, err := updateCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("update-ref failed: %s", out)
		}
		return sourceHash[:8], nil
	}

	// Non-fast-forward: create a merge commit via temporary worktree
	// This is more complex in a bare repo. Use git read-tree + merge approach.
	// For simplicity and reliability, use a temporary clone.
	tmpDir := fmt.Sprintf("/tmp/nydus-merge-%d", time.Now().UnixNano())
	defer exec.Command("rm", "-rf", tmpDir).Run()

	// Clone bare → temp
	cloneCmd := exec.Command("git", "clone", "--no-checkout", bareRepo, tmpDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("temp clone failed: %s", out)
	}

	// Checkout target branch
	checkoutCmd := exec.Command("git", "-C", tmpDir, "checkout", targetBranch)
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("checkout %s failed: %s", targetBranch, out)
	}

	// Merge source branch
	mergeArgs := []string{"-C", tmpDir, "merge"}
	switch strategy {
	case "squash":
		mergeArgs = append(mergeArgs, "--squash", sourceBranch)
	case "rebase":
		// For rebase, do rebase then push
		rebaseCmd := exec.Command("git", "-C", tmpDir, "rebase", sourceBranch)
		if out, err := rebaseCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("rebase failed: %s", out)
		}
		goto pushBack
	default:
		mergeArgs = append(mergeArgs, "--no-ff", "-m", fmt.Sprintf("Merge PR: %s", title), sourceBranch)
	}

	{
		mergeCmd := exec.Command("git", mergeArgs...)
		if out, err := mergeCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("merge failed: %s", out)
		}

		if strategy == "squash" {
			commitCmd := exec.Command("git", "-C", tmpDir, "commit", "-m", fmt.Sprintf("Squash merge PR: %s", title))
			if out, err := commitCmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("squash commit failed: %s", out)
			}
		}
	}

pushBack:
	// Push merged result back to bare repo
	pushCmd := exec.Command("git", "-C", tmpDir, "push", "origin", targetBranch)
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("push failed: %s", out)
	}

	// Get the new HEAD
	hash := resolveRef(bareRepo, targetBranch)
	if len(hash) > 8 {
		hash = hash[:8]
	}
	return hash, nil
}

func resolveRef(bareRepo, ref string) string {
	cmd := exec.Command("git", "--git-dir", bareRepo, "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
