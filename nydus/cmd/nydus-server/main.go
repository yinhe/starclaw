package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/nydus/internal/config"
)

func main() {
	cfgPath := "nydus.yaml"
	if v := os.Getenv("NYDUS_CONFIG"); v != "" {
		cfgPath = v
	}
	config.LoadServer(cfgPath)

	// Ensure repos dir exists
	os.MkdirAll(config.C.Server.ReposDir, 0755)

	// Init bare repos from config
	for name := range config.C.Repos {
		initBareRepo(name)
	}

	if config.C.Server.Secret == "" {
		log.Fatal("[nydus] server.secret must be set in config")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "nydus-server"})
	})

	api := r.Group("/api")
	api.Use(authMiddleware())
	{
		api.GET("/repos", listRepos)
		api.POST("/repos", createRepo)
		api.GET("/repos/:name", getRepo)
		api.DELETE("/repos/:name", deleteRepo)
		api.POST("/repos/:name/deploy", triggerDeploy)
		api.GET("/deploys", listDeploys)
	}

	// Hook endpoint (called by post-receive, uses same secret)
	r.POST("/hooks/push", authMiddleware(), hookPush)

	port := config.C.Server.Port
	log.Printf("[nydus] Nydus Server starting on :%s", port)
	log.Printf("[nydus] SSH clone: git clone %s@<host>:<repo>.git", config.C.Server.SSHUser)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}

// ==================== Auth ====================

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Nydus-Secret")
		if token == "" {
			token = c.Query("secret")
		}
		if token != config.C.Server.Secret {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// ==================== Repo Management ====================

func repoPath(name string) string {
	return filepath.Join(config.C.Server.ReposDir, name+".git")
}

func initBareRepo(name string) {
	path := repoPath(name)
	if _, err := os.Stat(filepath.Join(path, "HEAD")); err == nil {
		log.Printf("[nydus] repo %s already exists", name)
		installHook(name)
		return
	}
	cmd := exec.Command("git", "init", "--bare", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[nydus] failed to init repo %s: %v\n%s", name, err, out)
		return
	}
	installHook(name)
	log.Printf("[nydus] initialized bare repo: %s", name)
}

func installHook(name string) {
	hookPath := filepath.Join(repoPath(name), "hooks", "post-receive")
	script := fmt.Sprintf(`#!/bin/sh
# Nydus post-receive hook — notify server on push
while read oldrev newrev refname; do
  branch=$(echo "$refname" | sed 's|refs/heads/||')
  curl -sf -X POST "http://127.0.0.1:%s/hooks/push?secret=%s" \
    -H "Content-Type: application/json" \
    -d "{\"repo\":\"%s\",\"branch\":\"$branch\",\"newrev\":\"$newrev\"}" || true
done
`, config.C.Server.Port, config.C.Server.Secret, name)
	os.WriteFile(hookPath, []byte(script), 0755)
}

func listRepos(c *gin.Context) {
	repos := []gin.H{}
	for name, rc := range config.C.Repos {
		path := repoPath(name)
		exists := false
		if _, err := os.Stat(filepath.Join(path, "HEAD")); err == nil {
			exists = true
		}
		repos = append(repos, gin.H{
			"name":        name,
			"description": rc.Description,
			"targets":     len(rc.Targets),
			"initialized": exists,
			"ssh_url":     fmt.Sprintf("%s@nydus.starclaw.net:%s.git", config.C.Server.SSHUser, name),
		})
	}
	c.JSON(200, gin.H{"repos": repos})
}

func createRepo(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if _, exists := config.C.Repos[req.Name]; exists {
		c.JSON(409, gin.H{"error": "repo already exists in config"})
		return
	}
	initBareRepo(req.Name)
	c.JSON(201, gin.H{
		"name":    req.Name,
		"ssh_url": fmt.Sprintf("%s@nydus.starclaw.net:%s.git", config.C.Server.SSHUser, req.Name),
		"message": "repo created (add targets to nydus.yaml for auto-deploy)",
	})
}

func getRepo(c *gin.Context) {
	name := c.Param("name")
	rc, ok := config.C.Repos[name]
	if !ok {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	// Get latest commit
	head := getHead(name)
	c.JSON(200, gin.H{
		"name":        name,
		"description": rc.Description,
		"head":        head,
		"targets":     rc.Targets,
		"ssh_url":     fmt.Sprintf("%s@nydus.starclaw.net:%s.git", config.C.Server.SSHUser, name),
	})
}

func deleteRepo(c *gin.Context) {
	name := c.Param("name")
	path := repoPath(name)
	if err := os.RemoveAll(path); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": fmt.Sprintf("repo %s deleted", name)})
}

func getHead(name string) string {
	cmd := exec.Command("git", "--git-dir", repoPath(name), "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "(empty)"
	}
	return string(out[:len(out)-1]) // trim newline
}

// ==================== Deploy ====================

type deployRecord struct {
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Rev       string `json:"rev"`
	Target    string `json:"target"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

var recentDeploys []deployRecord

func triggerDeploy(c *gin.Context) {
	name := c.Param("name")
	rc, ok := config.C.Repos[name]
	if !ok {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	branch := c.DefaultQuery("branch", "main")
	rev := getHead(name)
	results := deployToTargets(name, branch, rev, rc.Targets)
	c.JSON(200, gin.H{"repo": name, "branch": branch, "rev": rev, "results": results})
}

func hookPush(c *gin.Context) {
	var req struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
		NewRev string `json:"newrev"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[nydus] push received: repo=%s branch=%s rev=%s", req.Repo, req.Branch, req.NewRev)

	rc, ok := config.C.Repos[req.Repo]
	if !ok {
		log.Printf("[nydus] repo %s has no deploy targets configured", req.Repo)
		c.JSON(200, gin.H{"message": "no targets configured"})
		return
	}

	// Filter targets by branch
	var targets []config.TargetConfig
	for _, t := range rc.Targets {
		if t.Branch == "" || t.Branch == req.Branch {
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		log.Printf("[nydus] no targets match branch %s", req.Branch)
		c.JSON(200, gin.H{"message": "no targets for this branch"})
		return
	}

	rev := req.NewRev
	if len(rev) > 7 {
		rev = rev[:7]
	}
	results := deployToTargets(req.Repo, req.Branch, rev, targets)
	c.JSON(200, gin.H{"repo": req.Repo, "results": results})
}

func deployToTargets(repo, branch, rev string, targets []config.TargetConfig) []deployRecord {
	var results []deployRecord
	for _, t := range targets {
		log.Printf("[nydus] deploying %s@%s → %s (%s)", repo, branch, t.Name, t.WormURL)
		rec := deployRecord{
			Repo:      repo,
			Branch:    branch,
			Rev:       rev,
			Target:    t.Name,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Call worm agent
		status, msg := callWorm(t, repo, branch, rev)
		rec.Status = status
		rec.Message = msg
		results = append(results, rec)

		// Keep last 100 deploys
		recentDeploys = append(recentDeploys, rec)
		if len(recentDeploys) > 100 {
			recentDeploys = recentDeploys[1:]
		}
	}
	return results
}

func callWorm(t config.TargetConfig, repo, branch, rev string) (string, string) {
	payload := fmt.Sprintf(`{"repo":"%s","branch":"%s","rev":"%s","deploy_path":"%s","deploy_cmd":"%s","subdir":"%s","repo_url":"/data/nydus/repos/%s.git"}`,
		repo, branch, rev, t.DeployPath, t.DeployCmd, t.Subdir, repo)

	// If ssh_host is set, call the remote Worm via SSH tunnel (avoids opening ports)
	if t.SSHHost != "" {
		wormURL := t.WormURL
		if wormURL == "" {
			wormURL = "http://127.0.0.1:8097"
		}
		sshArgs := []string{
			"-o", "StrictHostKeyChecking=no",
			"-o", "ConnectTimeout=10",
		}
		if t.SSHKey != "" {
			sshArgs = append(sshArgs, "-i", t.SSHKey)
		}
		curlCmd := fmt.Sprintf(`curl -sf -X POST '%s/deploy' -H 'Content-Type: application/json' -H 'X-Nydus-Secret: %s' -d '%s' --connect-timeout 5 --max-time 300`,
			wormURL, config.C.Server.Secret, payload)
		sshArgs = append(sshArgs, t.SSHHost, curlCmd)

		cmd := exec.Command("ssh", sshArgs...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[nydus] SSH deploy to %s failed: %v\n%s", t.Name, err, out)
			return "failed", fmt.Sprintf("%v: %s", err, out)
		}
		log.Printf("[nydus] SSH deploy to %s success: %s", t.Name, out)
		return "success", string(out)
	}

	// Local Worm: direct HTTP call
	cmd := exec.Command("curl", "-sf", "-X", "POST",
		fmt.Sprintf("%s/deploy", t.WormURL),
		"-H", "Content-Type: application/json",
		"-H", fmt.Sprintf("X-Nydus-Secret: %s", config.C.Server.Secret),
		"-d", payload,
		"--connect-timeout", "5",
		"--max-time", "300",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[nydus] deploy to %s failed: %v\n%s", t.Name, err, out)
		return "failed", fmt.Sprintf("%v: %s", err, out)
	}
	log.Printf("[nydus] deploy to %s success: %s", t.Name, out)
	return "success", string(out)
}

func listDeploys(c *gin.Context) {
	// Return in reverse order (newest first)
	n := len(recentDeploys)
	result := make([]deployRecord, n)
	for i, d := range recentDeploys {
		result[n-1-i] = d
	}
	c.JSON(200, gin.H{"deploys": result})
}
