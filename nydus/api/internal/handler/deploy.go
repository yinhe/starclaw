package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/nydus/api/internal/config"
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
		Tag    string `json:"tag"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[nydus] push received: repo=%s branch=%s tag=%s rev=%s", req.Repo, req.Branch, req.Tag, req.NewRev)

	// Handle tag push: sync version tags from starclaw.git → claw.git
	if req.Tag != "" && strings.HasPrefix(req.Tag, "v") {
		syncTagToClaw(req.Repo, req.Tag)
		c.JSON(200, gin.H{"message": "tag synced", "tag": req.Tag})
		return
	}

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

// ReportDeploy accepts deploy results from the post-receive bash hook.
// POST /hooks/deploy-report
func ReportDeploy(c *gin.Context) {
	var records []DeployRecord
	if err := c.ShouldBindJSON(&records); err != nil {
		// Try single record
		var single DeployRecord
		if err2 := c.ShouldBindJSON(&single); err2 != nil {
			c.JSON(400, gin.H{"error": "expected array or single deploy record"})
			return
		}
		records = []DeployRecord{single}
	}

	deployMu.Lock()
	for _, r := range records {
		if r.Timestamp == "" {
			r.Timestamp = time.Now().Format(time.RFC3339)
		}
		recentDeploys = append(recentDeploys, r)
	}
	if len(recentDeploys) > 200 {
		recentDeploys = recentDeploys[len(recentDeploys)-200:]
	}
	deployMu.Unlock()

	log.Printf("[nydus] received %d deploy report(s) from hook", len(records))
	c.JSON(200, gin.H{"accepted": len(records)})
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

// syncTagToClaw syncs a version tag from the source repo (starclaw.git) to claw.git
// and regenerates spore-latest.json for Spore update checks.
func syncTagToClaw(sourceRepo, tag string) {
	clawRepo := clawBareRepoPath()
	sourceRepoPath := RepoPath(sourceRepo)

	if _, err := os.Stat(clawRepo); os.IsNotExist(err) {
		log.Printf("[nydus] claw.git not found, skipping tag sync")
		return
	}

	// Check if tag already exists in claw.git
	cmd := exec.Command("git", "--git-dir="+clawRepo, "tag", "-l", tag)
	out, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(out)) == tag {
		log.Printf("[nydus] tag %s already exists in claw.git", tag)
	} else {
		// Get the commit the tag points to in source repo
		cmd = exec.Command("git", "--git-dir="+sourceRepoPath, "rev-parse", tag)
		commitOut, err := cmd.Output()
		if err != nil {
			log.Printf("[nydus] failed to resolve tag %s in %s: %v", tag, sourceRepo, err)
			return
		}
		commitHash := strings.TrimSpace(string(commitOut))

		// Find corresponding commit in claw.git (HEAD is close enough for lightweight tags)
		cmd = exec.Command("git", "--git-dir="+clawRepo, "tag", tag, "HEAD")
		if tagOut, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[nydus] failed to create tag %s in claw.git: %v\n%s", tag, err, tagOut)
			return
		}
		log.Printf("[nydus] synced tag %s to claw.git (source commit: %s)", tag, commitHash[:7])
	}

	// Regenerate spore-latest.json
	regenerateSporeLatest(tag)
}

// regenerateSporeLatest writes/updates spore-latest.json with the given version tag.
func regenerateSporeLatest(tag string) {
	version := strings.TrimPrefix(tag, "v")
	vTag := tag
	if !strings.HasPrefix(vTag, "v") {
		vTag = "v" + version
	}

	base := "/spore/releases"
	data := map[string]interface{}{
		"tag_name":     vTag,
		"name":         "StarClaw " + vTag,
		"body":         "",
		"published_at": time.Now().UTC().Format(time.RFC3339),
		"assets": map[string]interface{}{
			"windows_amd64": map[string]interface{}{
				"url":      fmt.Sprintf("%s/StarClaw-Setup-%s.exe", base, vTag),
				"filename": fmt.Sprintf("StarClaw-Setup-%s.exe", vTag),
			},
			"linux_amd64": map[string]interface{}{
				"url":      fmt.Sprintf("%s/StarClaw-Setup-%s-linux-amd64.tar.gz", base, vTag),
				"filename": fmt.Sprintf("StarClaw-Setup-%s-linux-amd64.tar.gz", vTag),
			},
			"darwin_arm64": map[string]interface{}{
				"url":      fmt.Sprintf("%s/StarClaw-Setup-%s-darwin-arm64.dmg", base, vTag),
				"filename": fmt.Sprintf("StarClaw-Setup-%s-darwin-arm64.dmg", vTag),
			},
			"darwin_amd64": map[string]interface{}{
				"url":      fmt.Sprintf("%s/StarClaw-Setup-%s-darwin-amd64.dmg", base, vTag),
				"filename": fmt.Sprintf("StarClaw-Setup-%s-darwin-amd64.dmg", vTag),
			},
		},
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("[nydus] failed to marshal spore-latest.json: %v", err)
		return
	}

	outPath := filepath.Join(config.C.Server.ReposDir, "releases", "spore-latest.json")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		log.Printf("[nydus] failed to create releases dir: %v", err)
		return
	}
	if err := os.WriteFile(outPath, jsonBytes, 0644); err != nil {
		log.Printf("[nydus] failed to write spore-latest.json: %v", err)
		return
	}
	log.Printf("[nydus] regenerated spore-latest.json for %s", vTag)
}
