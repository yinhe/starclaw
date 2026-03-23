package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/nydus/internal/config"
	"github.com/yinhe/starclaw/nydus/internal/database"
	"github.com/yinhe/starclaw/nydus/internal/model"
)

// isPublicOnly returns true if the request is on a public (unauthenticated) route.
func isPublicOnly(c *gin.Context) bool {
	v, exists := c.Get("public_only")
	return exists && v.(bool)
}

// repoAccessible checks if a repo can be accessed in the current context.
// Checks both DB and YAML config. On public routes, only public repos are accessible.
func repoAccessible(c *gin.Context, name string) bool {
	// Check DB first
	if database.DB != nil {
		var repo model.NydusRepo
		if err := database.DB.Where("name = ? AND status = ?", name, "active").First(&repo).Error; err == nil {
			if isPublicOnly(c) && !repo.Public {
				return false
			}
			return true
		}
	}
	// Fallback to YAML config
	rc, ok := config.C.Repos[name]
	if !ok {
		return false
	}
	if isPublicOnly(c) && !rc.Public {
		return false
	}
	return true
}

// RepoPath returns the filesystem path for a bare repo.
func RepoPath(name string) string {
	return filepath.Join(config.C.Server.ReposDir, name+".git")
}

// InitBareRepo creates a bare git repo and installs the post-receive hook.
func InitBareRepo(name string) {
	path := RepoPath(name)
	if _, err := os.Stat(filepath.Join(path, "HEAD")); err == nil {
		log.Printf("[nydus] repo %s already exists", name)
		InstallHook(name)
		return
	}
	cmd := exec.Command("git", "init", "--bare", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[nydus] failed to init repo %s: %v\n%s", name, err, out)
		return
	}
	InstallHook(name)
	log.Printf("[nydus] initialized bare repo: %s", name)
}

// InstallHook writes the post-receive hook script.
func InstallHook(name string) {
	hookPath := filepath.Join(RepoPath(name), "hooks", "post-receive")
	script := fmt.Sprintf(`#!/bin/sh
# Nydus post-receive hook — notify server on push (branches + tags)
while read oldrev newrev refname; do
  if echo "$refname" | grep -q '^refs/tags/'; then
    tag=$(echo "$refname" | sed 's|refs/tags/||')
    curl -sf -X POST "http://127.0.0.1:%s/hooks/push?secret=%s" \
      -H "Content-Type: application/json" \
      -d "{\"repo\":\"%s\",\"branch\":\"\",\"newrev\":\"$newrev\",\"tag\":\"$tag\"}" || true
  else
    branch=$(echo "$refname" | sed 's|refs/heads/||')
    curl -sf -X POST "http://127.0.0.1:%s/hooks/push?secret=%s" \
      -H "Content-Type: application/json" \
      -d "{\"repo\":\"%s\",\"branch\":\"$branch\",\"newrev\":\"$newrev\",\"tag\":\"\"}" || true
  fi
done
`, config.C.Server.Port, config.C.Server.Secret, name,
		config.C.Server.Port, config.C.Server.Secret, name)
	os.WriteFile(hookPath, []byte(script), 0755)
}

// GetHead returns the short HEAD commit hash for a repo.
func GetHead(name string) string {
	cmd := exec.Command("git", "--git-dir", RepoPath(name), "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "(empty)"
	}
	return strings.TrimSpace(string(out))
}

// ListRepos returns all repos from DB with git stats.
func ListRepos(c *gin.Context) {
	pubOnly := isPublicOnly(c)

	var dbRepos []model.NydusRepo
	query := database.DB.Where("status = ?", "active")
	if pubOnly {
		query = query.Where("public = ?", true)
	}
	if teamID := c.Query("team_id"); teamID != "" {
		query = query.Where("team_id = ?", teamID)
	}
	query.Order("name ASC").Find(&dbRepos)

	repos := []gin.H{}
	for _, r := range dbRepos {
		path := RepoPath(r.Name)
		exists := false
		if _, err := os.Stat(filepath.Join(path, "HEAD")); err == nil {
			exists = true
		}
		// Count deploy targets from YAML config (if any)
		targetCount := 0
		if rc, ok := config.C.Repos[r.Name]; ok {
			targetCount = len(rc.Targets)
		}
		entry := gin.H{
			"id":          r.ID,
			"name":        r.Name,
			"description": r.Description,
			"owner":       r.OwnerNodeID,
			"team_id":     r.TeamID,
			"public":      r.Public,
			"source":      r.Source,
			"forked_from": r.ForkedFrom,
			"targets":     targetCount,
			"initialized": exists,
			"ssh_url":     fmt.Sprintf("%s@nydus.starclaw.net:%s.git", config.C.Server.SSHUser, r.Name),
			"https_url":   fmt.Sprintf("https://nydus.starclaw.net/%s.git", r.Name),
			"created_at":  r.CreatedAt,
		}
		if exists {
			entry["head"] = GetHead(r.Name)
			entry["branches"] = countGitItems(r.Name, "refs/heads")
			entry["tags"] = countGitItems(r.Name, "refs/tags")
			entry["commit_count"] = countCommits(r.Name)
			entry["last_commit"] = getLastCommitInfo(r.Name)
		}
		repos = append(repos, entry)
	}
	c.JSON(200, gin.H{"repos": repos, "total": len(repos)})
}

// CreateRepo creates a new bare repo and persists to DB.
func CreateRepo(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Public      bool   `json:"public"`
		TeamID      string `json:"team_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check for duplicates in DB
	var existing model.NydusRepo
	if err := database.DB.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "repo already exists"})
		return
	}

	// Determine owner from auth context
	ownerNodeID, _ := c.Get("node_id")
	ownerStr := ""
	if ownerNodeID != nil {
		ownerStr = ownerNodeID.(string)
	}

	repo := model.NydusRepo{
		Name:        req.Name,
		Description: req.Description,
		Public:      req.Public,
		OwnerNodeID: ownerStr,
		TeamID:      req.TeamID,
		Source:      "dynamic",
		Status:      "active",
	}
	if err := database.DB.Create(&repo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create repo: " + err.Error()})
		return
	}

	InitBareRepo(req.Name)

	c.JSON(http.StatusCreated, gin.H{
		"repo":      repo,
		"ssh_url":   fmt.Sprintf("%s@nydus.starclaw.net:%s.git", config.C.Server.SSHUser, req.Name),
		"https_url": fmt.Sprintf("https://nydus.starclaw.net/%s.git", req.Name),
		"message":   "repo created",
	})
}

// GetRepo returns details about a single repo.
func GetRepo(c *gin.Context) {
	name := c.Param("name")
	if !repoAccessible(c, name) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	rc := config.C.Repos[name]
	head := GetHead(name)
	c.JSON(200, gin.H{
		"name":        name,
		"description": rc.Description,
		"head":        head,
		"targets":     rc.Targets,
		"ssh_url":     fmt.Sprintf("%s@nydus.starclaw.net:%s.git", config.C.Server.SSHUser, name),
		"https_url":   fmt.Sprintf("https://nydus.starclaw.net/%s.git", name),
	})
}

// DeleteRepo removes a bare repo from disk.
func DeleteRepo(c *gin.Context) {
	name := c.Param("name")
	path := RepoPath(name)
	if err := os.RemoveAll(path); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": fmt.Sprintf("repo %s deleted", name)})
}

// countGitItems counts branches or tags in a bare repo.
func countGitItems(name, refPrefix string) int {
	cmd := exec.Command("git", "--git-dir", RepoPath(name), "for-each-ref", "--format=%(refname)", refPrefix)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}

// countCommits returns total commit count for the default branch.
func countCommits(name string) int {
	cmd := exec.Command("git", "--git-dir", RepoPath(name), "rev-list", "--count", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// getLastCommitInfo returns the last commit's message, author, and time.
func getLastCommitInfo(name string) gin.H {
	cmd := exec.Command("git", "--git-dir", RepoPath(name), "log", "-1", "--format=%H|%h|%s|%an|%cr|%ci")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 6)
	if len(parts) < 5 {
		return nil
	}
	result := gin.H{
		"hash":       parts[0],
		"short_hash": parts[1],
		"message":    parts[2],
		"author":     parts[3],
		"time_ago":   parts[4],
	}
	if len(parts) == 6 {
		result["date"] = parts[5]
	}
	return result
}

// GetRepoTree returns the file/directory listing of a repo (like GitHub file browser).
func GetRepoTree(c *gin.Context) {
	name := c.Param("name")
	if !repoAccessible(c, name) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	path := c.DefaultQuery("path", "")
	ref := c.DefaultQuery("ref", "HEAD")

	bareRepo := RepoPath(name)
	if _, err := os.Stat(filepath.Join(bareRepo, "HEAD")); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}

	treeRef := ref
	if path != "" {
		treeRef = ref + ":" + path
	}

	cmd := exec.Command("git", "--git-dir", bareRepo, "ls-tree", "-l", treeRef)
	out, err := cmd.Output()
	if err != nil {
		c.JSON(200, gin.H{"items": []gin.H{}, "path": path})
		return
	}

	items := []gin.H{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// format: <mode> <type> <hash> <size>\t<name>
		tabIdx := strings.IndexByte(line, '\t')
		if tabIdx < 0 {
			continue
		}
		meta := strings.Fields(line[:tabIdx])
		itemName := line[tabIdx+1:]
		if len(meta) < 4 {
			continue
		}

		itemType := meta[1] // "blob" or "tree"
		size := meta[3]     // "-" for tree

		// Get last commit for this file
		filePath := itemName
		if path != "" {
			filePath = path + "/" + itemName
		}
		lastMsg, lastTime := getFileLastCommit(bareRepo, ref, filePath)

		entry := gin.H{
			"name":     itemName,
			"type":     itemType,
			"message":  lastMsg,
			"time_ago": lastTime,
		}
		if itemType == "blob" {
			entry["size"] = size
		}
		items = append(items, entry)
	}

	// Sort: directories first, then files
	sortTreeItems(items)

	c.JSON(200, gin.H{"items": items, "path": path, "ref": ref})
}

func getFileLastCommit(bareRepo, ref, filePath string) (string, string) {
	cmd := exec.Command("git", "--git-dir", bareRepo, "log", "-1", "--format=%s|%cr", ref, "--", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return strings.TrimSpace(string(out)), ""
}

func sortTreeItems(items []gin.H) {
	// Simple bubble sort — small list
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			ti := items[i]["type"].(string)
			tj := items[j]["type"].(string)
			ni := items[i]["name"].(string)
			nj := items[j]["name"].(string)
			// tree before blob, then alphabetical
			if tj == "tree" && ti == "blob" {
				items[i], items[j] = items[j], items[i]
			} else if ti == tj && ni > nj {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// GetRepoReadme returns the README content from a repo.
func GetRepoReadme(c *gin.Context) {
	name := c.Param("name")
	if !repoAccessible(c, name) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	ref := c.DefaultQuery("ref", "HEAD")

	bareRepo := RepoPath(name)
	if _, err := os.Stat(filepath.Join(bareRepo, "HEAD")); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}

	// Try common README names
	for _, readme := range []string{"README.md", "readme.md", "README", "README.txt"} {
		cmd := exec.Command("git", "--git-dir", bareRepo, "show", ref+":"+readme)
		out, err := cmd.Output()
		if err == nil {
			c.JSON(200, gin.H{"name": readme, "content": string(out)})
			return
		}
	}
	c.JSON(200, gin.H{"name": "", "content": ""})
}

// GetRepoBranches returns branches for a repo.
func GetRepoBranches(c *gin.Context) {
	name := c.Param("name")
	if !repoAccessible(c, name) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	bareRepo := RepoPath(name)
	cmd := exec.Command("git", "--git-dir", bareRepo, "for-each-ref", "--format=%(refname:short)|%(objectname:short)|%(creatordate:relative)", "refs/heads/")
	out, err := cmd.Output()
	if err != nil {
		c.JSON(200, gin.H{"branches": []gin.H{}})
		return
	}
	branches := []gin.H{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		entry := gin.H{"name": parts[0]}
		if len(parts) >= 2 {
			entry["head"] = parts[1]
		}
		if len(parts) >= 3 {
			entry["updated"] = parts[2]
		}
		branches = append(branches, entry)
	}
	c.JSON(200, gin.H{"branches": branches})
}

// GetRepoTags returns tags for a repo.
func GetRepoTags(c *gin.Context) {
	name := c.Param("name")
	if !repoAccessible(c, name) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	bareRepo := RepoPath(name)
	cmd := exec.Command("git", "--git-dir", bareRepo, "for-each-ref", "--sort=-version:refname", "--format=%(refname:short)|%(objectname:short)|%(creatordate:relative)", "refs/tags/")
	out, err := cmd.Output()
	if err != nil {
		c.JSON(200, gin.H{"tags": []gin.H{}})
		return
	}
	tags := []gin.H{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		entry := gin.H{"name": parts[0]}
		if len(parts) >= 2 {
			entry["hash"] = parts[1]
		}
		if len(parts) >= 3 {
			entry["date"] = parts[2]
		}
		tags = append(tags, entry)
	}
	c.JSON(200, gin.H{"tags": tags, "count": len(tags)})
}

// GetServerStats returns overall server statistics.
func GetServerStats(c *gin.Context) {
	pubOnly := isPublicOnly(c)
	totalRepos := 0
	totalTargets := 0
	totalCommits := 0
	totalTags := 0
	for name, rc := range config.C.Repos {
		if pubOnly && !rc.Public {
			continue
		}
		totalRepos++
		totalTargets += len(rc.Targets)
		path := RepoPath(name)
		if _, err := os.Stat(filepath.Join(path, "HEAD")); err == nil {
			totalCommits += countCommits(name)
			totalTags += countGitItems(name, "refs/tags")
		}
	}

	c.JSON(200, gin.H{
		"repos":         totalRepos,
		"targets":       totalTargets,
		"total_commits": totalCommits,
		"total_tags":    totalTags,
	})
}

// GetRecentCommits returns the last N commits for a repo (for web UI).
func GetRecentCommits(c *gin.Context) {
	name := c.DefaultQuery("repo", "claw")
	if !repoAccessible(c, name) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	limit := c.DefaultQuery("limit", "20")
	bareRepo := RepoPath(name)
	if _, err := os.Stat(filepath.Join(bareRepo, "HEAD")); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}

	cmd := exec.Command("git", "--git-dir", bareRepo, "log", "--oneline", "-"+limit, "--format=%h|%s|%cr|%an")
	out, err := cmd.Output()
	if err != nil {
		c.JSON(200, gin.H{"commits": []gin.H{}})
		return
	}

	commits := []gin.H{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 3 {
			entry := gin.H{
				"hash":    parts[0],
				"message": parts[1],
				"time":    parts[2],
			}
			if len(parts) == 4 {
				entry["author"] = parts[3]
			}
			commits = append(commits, entry)
		}
	}
	c.JSON(200, gin.H{"commits": commits})
}
