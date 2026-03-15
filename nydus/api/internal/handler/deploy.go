package handler

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/nydus/internal/config"
)

// DeployRecord holds the result of a single deployment.
type DeployRecord struct {
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Rev       string `json:"rev"`
	Target    string `json:"target"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

var (
	recentDeploys []DeployRecord
	deployMu      sync.Mutex
)

// TriggerDeploy manually triggers deployment for a repo.
func TriggerDeploy(c *gin.Context) {
	name := c.Param("name")
	rc, ok := config.C.Repos[name]
	if !ok {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	branch := c.DefaultQuery("branch", "main")
	rev := GetHead(name)
	results := deployToTargets(name, branch, rev, rc.Targets)
	c.JSON(200, gin.H{"repo": name, "branch": branch, "rev": rev, "results": results})
}

// HookPush handles the post-receive webhook from git.
func HookPush(c *gin.Context) {
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

// ListDeploys returns recent deployment records.
func ListDeploys(c *gin.Context) {
	deployMu.Lock()
	defer deployMu.Unlock()

	pubOnly := isPublicOnly(c)
	result := []DeployRecord{}
	for i := len(recentDeploys) - 1; i >= 0; i-- {
		d := recentDeploys[i]
		if pubOnly {
			rc, ok := config.C.Repos[d.Repo]
			if !ok || !rc.Public {
				continue
			}
		}
		result = append(result, d)
	}
	c.JSON(200, gin.H{"deploys": result})
}

func deployToTargets(repo, branch, rev string, targets []config.TargetConfig) []DeployRecord {
	var results []DeployRecord
	for _, t := range targets {
		log.Printf("[nydus] deploying %s@%s → %s (%s)", repo, branch, t.Name, t.WormURL)
		rec := DeployRecord{
			Repo:      repo,
			Branch:    branch,
			Rev:       rev,
			Target:    t.Name,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		status, msg := callWorm(t, repo, branch, rev)
		rec.Status = status
		rec.Message = msg
		results = append(results, rec)

		deployMu.Lock()
		recentDeploys = append(recentDeploys, rec)
		if len(recentDeploys) > 100 {
			recentDeploys = recentDeploys[1:]
		}
		deployMu.Unlock()
	}
	return results
}

func callWorm(t config.TargetConfig, repo, branch, rev string) (string, string) {
	// Safety guard: customer Claw targets (name starts with "claw-") must NOT deploy queen code
	if strings.HasPrefix(t.Name, "claw-") && (t.Subdir == "" || strings.Contains(t.Subdir, "queen")) {
		log.Printf("[nydus] BLOCKED: customer target %s attempted to deploy queen code (subdir=%s)", t.Name, t.Subdir)
		return "blocked", "customer Claw targets can only deploy claw/ subdir"
	}

	payload := fmt.Sprintf(`{"repo":"%s","branch":"%s","rev":"%s","deploy_path":"%s","deploy_cmd":"%s","subdir":"%s","repo_url":"/data/nydus/repos/%s.git"}`,
		repo, branch, rev, t.DeployPath, t.DeployCmd, t.Subdir, repo)

	// If ssh_host is set, sync code via SCP then call remote Worm via SSH
	if t.SSHHost != "" {
		sshBase := []string{
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "ConnectTimeout=10",
		}
		if t.SSHKey != "" {
			sshBase = append(sshBase, "-i", t.SSHKey)
		}

		// Step 1: Extract subdir from bare repo and sync to remote
		if t.Subdir != "" {
			log.Printf("[nydus] syncing %s/%s → %s:%s", repo, t.Subdir, t.SSHHost, t.DeployPath)
			archiveCmd := fmt.Sprintf(
				`git --git-dir=/data/nydus/repos/%s.git archive HEAD:%s | ssh %s %s 'mkdir -p %s && cd %s && tar xf -'`,
				repo, t.Subdir,
				strings.Join(sshBase, " "), t.SSHHost,
				t.DeployPath, t.DeployPath,
			)
			cmd := exec.Command("sh", "-c", archiveCmd)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("[nydus] code sync to %s failed: %v\n%s", t.Name, err, out)
				return "failed", fmt.Sprintf("code sync failed: %v: %s", err, out)
			}
			log.Printf("[nydus] code synced to %s", t.Name)
		}

		// Step 2: Run deploy command
		if t.WormURL != "" {
			// Call remote Worm agent
			remotePayload := fmt.Sprintf(`{"repo":"%s","branch":"%s","rev":"%s","deploy_path":"%s","deploy_cmd":"%s"}`,
				repo, branch, rev, t.DeployPath, t.DeployCmd)
			curlCmd := fmt.Sprintf(`curl -sf -X POST '%s/deploy' -H 'Content-Type: application/json' -H 'X-Nydus-Secret: %s' -d '%s' --connect-timeout 5 --max-time 300`,
				t.WormURL, config.C.Server.Secret, remotePayload)
			sshArgs := append(sshBase, t.SSHHost, curlCmd)

			cmd := exec.Command("ssh", sshArgs...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("[nydus] SSH deploy to %s via Worm failed: %v\n%s", t.Name, err, out)
				return "failed", fmt.Sprintf("%v: %s", err, out)
			}
			log.Printf("[nydus] SSH deploy to %s via Worm success: %s", t.Name, out)
			return "success", string(out)
		}

		// Direct SSH: run deploy_cmd in deploy_path (no Worm needed)
		log.Printf("[nydus] direct SSH deploy to %s: cd %s && %s", t.Name, t.DeployPath, t.DeployCmd)
		remoteCmd := fmt.Sprintf("cd %s && %s", t.DeployPath, t.DeployCmd)
		sshArgs := append(sshBase, t.SSHHost, remoteCmd)

		cmd := exec.Command("ssh", sshArgs...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[nydus] SSH direct deploy to %s failed: %v\n%s", t.Name, err, out)
			return "failed", fmt.Sprintf("%v: %s", err, out)
		}
		log.Printf("[nydus] SSH direct deploy to %s success: %s", t.Name, out)
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
