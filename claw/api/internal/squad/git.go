package squad

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/sandbox"
)

// ConflictResolver is a callback that resolves merge conflicts using an LLM.
// It receives the file path, ours content, theirs content, and base content.
// Returns the resolved content or an error.
type ConflictResolver func(filePath, ours, theirs, base string) (string, error)

// GitManager handles Git repository operations for Squad missions.
// Phase 1: local bare repos with file:// protocol.
// Phase 2: HTTP Git server for cross-network collaboration.
type GitManager struct {
	reposDir         string           // base directory for bare repos
	workspacesDir    string           // base directory for working copies
	selfAddress      string           // node's HTTP address (for generating repo URLs)
	conflictResolver ConflictResolver // LLM-based conflict resolution callback
}

// NewGitManager creates a new Git manager
func NewGitManager() *GitManager {
	reposDir := filepath.Join(sandbox.WorkspacesDir(), "..", "repos")
	workspacesDir := sandbox.WorkspacesDir()
	os.MkdirAll(reposDir, 0755)
	return &GitManager{
		reposDir:      reposDir,
		workspacesDir: workspacesDir,
	}
}

// SetSelfAddress sets the node's HTTP address for generating repo URLs.
func (g *GitManager) SetSelfAddress(addr string) {
	g.selfAddress = addr
}

// SetConflictResolver sets the LLM-based conflict resolution callback.
func (g *GitManager) SetConflictResolver(resolver ConflictResolver) {
	g.conflictResolver = resolver
}

// ReposDir returns the base directory for bare repos.
func (g *GitManager) ReposDir() string {
	return g.reposDir
}

// InitMissionRepo creates a bare Git repository for a mission.
// Returns the repo path (file:// URL compatible).
func (g *GitManager) InitMissionRepo(missionID string) (string, error) {
	repoPath := filepath.Join(g.reposDir, fmt.Sprintf("mission-%s.git", missionID))

	// Skip if already exists
	if _, err := os.Stat(repoPath); err == nil {
		return repoPath, nil
	}

	// Create bare repo
	if _, err := g.runGit("", "init", "--bare", repoPath); err != nil {
		return "", fmt.Errorf("failed to init bare repo: %w", err)
	}

	// Create a temp working copy to make initial commit (bare repos need this)
	tmpDir := filepath.Join(g.workspacesDir, "_init_"+missionID)
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	g.runGit(tmpDir, "init")
	g.runGit(tmpDir, "config", "user.email", "captain@starclaw.local")
	g.runGit(tmpDir, "config", "user.name", "Captain")

	// Create README
	readmePath := filepath.Join(tmpDir, "README.md")
	os.WriteFile(readmePath, []byte(fmt.Sprintf("# Mission %s\n\nInitialized by Squad Captain.\n", missionID[:8])), 0644)

	g.runGit(tmpDir, "add", ".")
	g.runGit(tmpDir, "commit", "-m", "Initial commit")
	g.runGit(tmpDir, "remote", "add", "origin", repoPath)
	g.runGit(tmpDir, "push", "-u", "origin", "master")

	// Enable HTTP receive-pack (required for Smart HTTP push)
	EnableReceivePack(repoPath)
	UpdateServerInfo(repoPath)

	log.Printf("[GitManager] Created bare repo: %s", repoPath)
	return repoPath, nil
}

// CloneForStep creates a working copy for a mission step, checks out the designated branch.
// Returns the workspace path.
func (g *GitManager) CloneForStep(repoPath, missionID, branch string) (string, error) {
	// Workspace: /app/workspaces/mission-{id}/{branch}
	safeBranch := strings.ReplaceAll(branch, "/", "_")
	wsPath := filepath.Join(g.workspacesDir, fmt.Sprintf("mission-%s", missionID), safeBranch)

	// Remove existing workspace if present (clean start)
	os.RemoveAll(wsPath)
	os.MkdirAll(filepath.Dir(wsPath), 0755)

	// Clone from bare repo
	if _, err := g.runGit("", "clone", repoPath, wsPath); err != nil {
		return "", fmt.Errorf("failed to clone repo: %w", err)
	}

	// Configure user
	g.runGit(wsPath, "config", "user.email", "agent@starclaw.local")
	g.runGit(wsPath, "config", "user.name", "Claw Agent")

	// Create and checkout the branch
	g.runGit(wsPath, "checkout", "-b", branch)

	log.Printf("[GitManager] Cloned workspace for branch %s: %s", branch, wsPath)
	return wsPath, nil
}

// GetMissionWorkspace returns the main working directory for a mission (for merge/build/preview).
func (g *GitManager) GetMissionWorkspace(missionID string) string {
	return filepath.Join(g.workspacesDir, fmt.Sprintf("mission-%s", missionID), "_main")
}

// MergeBranches merges all specified branches into master in the main workspace.
// Returns merge output and any error.
func (g *GitManager) MergeBranches(repoPath, missionID string, branches []string) (string, error) {
	mainWS := g.GetMissionWorkspace(missionID)

	// Clone or reset main workspace
	os.RemoveAll(mainWS)
	os.MkdirAll(filepath.Dir(mainWS), 0755)

	if _, err := g.runGit("", "clone", repoPath, mainWS); err != nil {
		return "", fmt.Errorf("failed to clone main workspace: %w", err)
	}

	g.runGit(mainWS, "config", "user.email", "captain@starclaw.local")
	g.runGit(mainWS, "config", "user.name", "Captain")

	// Fetch all branches
	g.runGit(mainWS, "fetch", "--all")

	var mergeOutput strings.Builder
	mergeOutput.WriteString("Merge results:\n")

	for _, branch := range branches {
		// Check if the branch exists on the remote
		out, err := g.runGit(mainWS, "branch", "-r", "--list", fmt.Sprintf("origin/%s", branch))
		if err != nil || strings.TrimSpace(out) == "" {
			mergeOutput.WriteString(fmt.Sprintf("  %s: SKIP (branch not found on remote)\n", branch))
			continue
		}

		out, err = g.runGit(mainWS, "merge", "--no-ff", fmt.Sprintf("origin/%s", branch), "-m", fmt.Sprintf("Merge branch '%s'", branch))
		if err != nil {
			if strings.Contains(out, "CONFLICT") {
				resolved := g.resolveConflicts(mainWS, branch)
				if resolved {
					mergeOutput.WriteString(fmt.Sprintf("  %s: MERGED (LLM-resolved conflicts)\n", branch))
				} else {
					// Fallback: accept theirs
					g.runGit(mainWS, "checkout", "--theirs", ".")
					g.runGit(mainWS, "add", ".")
					g.runGit(mainWS, "commit", "-m", fmt.Sprintf("Auto-resolve conflicts from '%s' (accept theirs)", branch))
					mergeOutput.WriteString(fmt.Sprintf("  %s: MERGED (fallback: accept theirs)\n", branch))
				}
			} else {
				mergeOutput.WriteString(fmt.Sprintf("  %s: FAILED (%s)\n", branch, out))
			}
		} else {
			mergeOutput.WriteString(fmt.Sprintf("  %s: MERGED OK\n", branch))
		}
	}

	// Push merge result back to bare repo
	g.runGit(mainWS, "push", "origin", "master")

	result := mergeOutput.String()
	log.Printf("[GitManager] %s", result)
	return result, nil
}

// RepoURL returns the file:// URL for a bare repo path (local mode).
func (g *GitManager) RepoURL(repoPath string) string {
	absPath, _ := filepath.Abs(repoPath)
	return "file://" + filepath.ToSlash(absPath)
}

// RepoHTTPURL returns the HTTP URL for a mission repo.
// Format: http://{selfAddress}/v1/git/mission-{missionID}
// Remote nodes use this URL to clone/push via Git Smart HTTP protocol.
func (g *GitManager) RepoHTTPURL(missionID string) string {
	if g.selfAddress == "" {
		return ""
	}
	addr := g.selfAddress
	if !strings.HasPrefix(addr, "http") {
		addr = "http://" + addr
	}
	return fmt.Sprintf("%s/v1/git/mission-%s", addr, missionID)
}

// IsLocalNode checks if the given address matches our own address.
func (g *GitManager) IsLocalNode(addr string) bool {
	return addr == g.selfAddress
}

// ListBranches lists all branches in a repository.
func (g *GitManager) ListBranches(repoPath string) ([]string, error) {
	out, err := g.runGit(repoPath, "branch", "-a")
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// GetLatestCommit returns the latest commit hash on a branch in the repo.
func (g *GitManager) GetLatestCommit(repoPath, branch string) string {
	out, err := g.runGit(repoPath, "log", "-1", "--format=%H", branch)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// resolveConflicts attempts to resolve merge conflicts using the LLM resolver.
// Returns true if all conflicts were resolved successfully.
func (g *GitManager) resolveConflicts(wsPath, branch string) bool {
	if g.conflictResolver == nil {
		return false
	}

	// List conflicting files
	out, err := g.runGit(wsPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil || strings.TrimSpace(out) == "" {
		return false
	}

	conflictFiles := strings.Split(strings.TrimSpace(out), "\n")
	allResolved := true

	for _, filePath := range conflictFiles {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}

		absPath := filepath.Join(wsPath, filePath)

		// Get ours version (HEAD)
		ours, _ := g.runGit(wsPath, "show", "HEAD:"+filePath)
		// Get theirs version (MERGE_HEAD)
		theirs, _ := g.runGit(wsPath, "show", "MERGE_HEAD:"+filePath)
		// Get base version (common ancestor)
		base, _ := g.runGit(wsPath, "show", ":1:"+filePath)

		// Call LLM resolver
		resolved, err := g.conflictResolver(filePath, ours, theirs, base)
		if err != nil {
			log.Printf("[GitManager] LLM conflict resolution failed for %s: %v", filePath, err)
			allResolved = false
			continue
		}

		// Write resolved content
		if err := os.WriteFile(absPath, []byte(resolved), 0644); err != nil {
			log.Printf("[GitManager] failed to write resolved file %s: %v", filePath, err)
			allResolved = false
			continue
		}

		g.runGit(wsPath, "add", filePath)
		log.Printf("[GitManager] LLM resolved conflict in %s", filePath)
	}

	if !allResolved {
		return false
	}

	// Commit the resolution
	_, err = g.runGit(wsPath, "commit", "-m", fmt.Sprintf("LLM-resolved merge conflicts from '%s'", branch))
	return err == nil
}

// runGit executes a git command and returns combined output.
func (g *GitManager) runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := hiddenCmdCtx(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Claw Agent",
		"GIT_AUTHOR_EMAIL=agent@starclaw.local",
		"GIT_COMMITTER_NAME=Claw Agent",
		"GIT_COMMITTER_EMAIL=agent@starclaw.local",
	)

	output, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(output))

	if err != nil {
		log.Printf("[GitManager] git %s: error=%v output=%s", strings.Join(args, " "), err, out)
		return out, fmt.Errorf("%s: %s", err.Error(), out)
	}

	return out, nil
}
