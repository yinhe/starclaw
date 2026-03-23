package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/nydus/api/internal/config"
	"starclaw.net/nydus/api/internal/middleware"
)

func main() {
	cfgPath := "worm.yaml"
	if v := os.Getenv("NYDUS_WORM_CONFIG"); v != "" {
		cfgPath = v
	}
	config.LoadWorm(cfgPath)

	if config.W.Secret == "" {
		log.Fatal("[worm] secret must be set in config")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "nydus-worm"})
	})

	r.POST("/deploy", middleware.WormAuth(), deploy)
	r.GET("/status", middleware.WormAuth(), status)

	port := config.W.Port
	log.Printf("[worm] Nydus Worm (deploy agent) starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}

type deployResult struct {
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Rev       string `json:"rev"`
	Status    string `json:"status"`
	Output    string `json:"output"`
	Duration  string `json:"duration"`
	Timestamp string `json:"timestamp"`
}

var lastDeploys []deployResult

func deploy(c *gin.Context) {
	var req struct {
		Repo       string `json:"repo" binding:"required"`
		Branch     string `json:"branch"`
		Rev        string `json:"rev"`
		DeployPath string `json:"deploy_path" binding:"required"`
		DeployCmd  string `json:"deploy_cmd" binding:"required"`
		Subdir     string `json:"subdir"`
		RepoURL    string `json:"repo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[worm] deploy request: repo=%s branch=%s rev=%s subdir=%s path=%s",
		req.Repo, req.Branch, req.Rev, req.Subdir, req.DeployPath)

	start := time.Now()
	result := deployResult{
		Repo:      req.Repo,
		Branch:    req.Branch,
		Rev:       req.Rev,
		Timestamp: start.Format(time.RFC3339),
	}

	// Step 1: Ensure deploy path exists
	if err := os.MkdirAll(req.DeployPath, 0755); err != nil {
		result.Status = "failed"
		result.Output = fmt.Sprintf("mkdir failed: %v", err)
		result.Duration = time.Since(start).String()
		saveDeploy(result)
		c.JSON(500, result)
		return
	}

	// Step 2: Sync code from bare repo
	if req.RepoURL != "" {
		cacheDir := fmt.Sprintf("/tmp/nydus-cache/%s", req.Repo)
		gitDir := fmt.Sprintf("%s/.git", cacheDir)

		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			log.Printf("[worm] cloning %s → %s", req.RepoURL, cacheDir)
			os.MkdirAll(cacheDir, 0755)
			cmd := exec.Command("git", "clone", "--branch", req.Branch, "--single-branch", req.RepoURL, cacheDir)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("[worm] git clone failed: %v\n%s", err, out)
				result.Status = "failed"
				result.Output = fmt.Sprintf("git clone failed: %v\n%s", err, lastN(string(out), 2000))
				result.Duration = time.Since(start).String()
				saveDeploy(result)
				c.JSON(500, result)
				return
			}
		} else {
			log.Printf("[worm] pulling latest in %s", cacheDir)
			cmd := exec.Command("git", "-C", cacheDir, "fetch", "origin", req.Branch)
			cmd.CombinedOutput()
			cmd = exec.Command("git", "-C", cacheDir, "reset", "--hard", fmt.Sprintf("origin/%s", req.Branch))
			cmd.CombinedOutput()
		}

		// Step 3: sync subdir to deploy path
		srcDir := cacheDir
		if req.Subdir != "" {
			srcDir = fmt.Sprintf("%s/%s", cacheDir, req.Subdir)
		}
		log.Printf("[worm] syncing %s/ → %s/", srcDir, req.DeployPath)
		syncCmd := exec.Command("sh", "-c", fmt.Sprintf(
			`cd "%s" && find . -not -path './.git/*' -not -name '.git' | while read f; do
				if [ -d "$f" ]; then mkdir -p "%s/$f"; else cp -f "$f" "%s/$f" 2>/dev/null; fi
			done`, srcDir, req.DeployPath, req.DeployPath))
		if out, err := syncCmd.CombinedOutput(); err != nil {
			log.Printf("[worm] sync warning: %v\n%s", err, out)
		}
	}

	// Step 4: Run deploy command
	log.Printf("[worm] running: %s (in %s)", req.DeployCmd, req.DeployPath)
	parts := strings.Fields(req.DeployCmd)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = req.DeployPath
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("NYDUS_REPO=%s", req.Repo),
		fmt.Sprintf("NYDUS_BRANCH=%s", req.Branch),
		fmt.Sprintf("NYDUS_REV=%s", req.Rev),
	)
	out, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result.Duration = duration.String()
	if err != nil {
		result.Status = "failed"
		result.Output = fmt.Sprintf("deploy cmd failed: %v\n%s", err, lastN(string(out), 2000))
		log.Printf("[worm] deploy failed: %v", err)
	} else {
		result.Status = "success"
		result.Output = lastN(string(out), 2000)
		log.Printf("[worm] deploy success in %s", duration)
	}

	saveDeploy(result)
	c.JSON(200, result)
}

func status(c *gin.Context) {
	c.JSON(200, gin.H{"deploys": lastDeploys})
}

func saveDeploy(r deployResult) {
	lastDeploys = append(lastDeploys, r)
	if len(lastDeploys) > 50 {
		lastDeploys = lastDeploys[1:]
	}
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
