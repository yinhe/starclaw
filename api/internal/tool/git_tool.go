package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/sandbox"
)

// GitTool provides Git version control operations for coding agents.
// Supports both file:// (local) and http:// (remote) protocols.
type GitTool struct {
	baseDir string // base directory for workspaces
}

// NewGitTool creates a new Git tool
func NewGitTool() *GitTool {
	baseDir := sandbox.WorkspacesDir()
	os.MkdirAll(baseDir, 0755)
	return &GitTool{baseDir: baseDir}
}

func (t *GitTool) Name() string { return "git" }

func (t *GitTool) Description() string {
	return `Git 版本控制工具，用于代码协作和版本管理。
支持操作：
- init: 初始化 Git 仓库
- clone: 克隆仓库到工作目录
- add: 添加文件到暂存区
- commit: 提交暂存区的变更
- push: 推送本地提交到远程
- pull: 拉取远程更新
- branch: 创建或列出分支
- checkout: 切换分支
- merge: 合并分支到当前分支
- status: 查看工作区状态
- log: 查看提交历史
- diff: 查看文件变更

工作流程：clone → checkout branch → 编写代码 → add → commit → push`
}

func (t *GitTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Git action to perform",
				"enum":        []string{"init", "clone", "add", "commit", "push", "pull", "branch", "checkout", "merge", "status", "log", "diff"},
			},
			"workspace_id": map[string]interface{}{
				"type":        "string",
				"description": "Workspace ID (directory name under base dir)",
			},
			"repo_url": map[string]interface{}{
				"type":        "string",
				"description": "Repository URL for clone (file:// or http://)",
			},
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Branch name (for branch/checkout/merge)",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit message (for commit action)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path pattern (for add/diff, default '.' for all)",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of log entries to show (default 10)",
			},
		},
		"required": []string{"action", "workspace_id"},
	}
}

type gitArgs struct {
	Action      string `json:"action"`
	WorkspaceID string `json:"workspace_id"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	Message     string `json:"message"`
	Path        string `json:"path"`
	Count       int    `json:"count"`
}

func (t *GitTool) Execute(ctx context.Context, args string) (string, error) {
	var a gitArgs
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	if a.WorkspaceID == "" {
		if uid, ok := ctx.Value(CtxKeyUserID).(string); ok && uid != "" {
			a.WorkspaceID = uid
		} else {
			a.WorkspaceID = "default"
		}
	}

	// Resolve conversation-isolated workspace
	effectiveWS := a.WorkspaceID
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok && cid != "" {
		effectiveWS = filepath.Join(a.WorkspaceID, cid)
	}

	wsPath := filepath.Join(t.baseDir, effectiveWS)
	os.MkdirAll(wsPath, 0755)

	switch a.Action {
	case "init":
		return t.gitInit(wsPath, a)
	case "clone":
		return t.gitClone(wsPath, a)
	case "add":
		return t.gitAdd(wsPath, a)
	case "commit":
		return t.gitCommit(wsPath, a)
	case "push":
		return t.gitPush(wsPath, a)
	case "pull":
		return t.gitPull(wsPath, a)
	case "branch":
		return t.gitBranch(wsPath, a)
	case "checkout":
		return t.gitCheckout(wsPath, a)
	case "merge":
		return t.gitMerge(wsPath, a)
	case "status":
		return t.gitStatus(wsPath)
	case "log":
		return t.gitLog(wsPath, a)
	case "diff":
		return t.gitDiff(wsPath, a)
	default:
		return toJSON(map[string]interface{}{"error": fmt.Sprintf("unknown git action: %s", a.Action)}), nil
	}
}

func (t *GitTool) gitInit(wsPath string, a gitArgs) (string, error) {
	// Configure git user for this workspace
	t.runGit(wsPath, "init")
	t.runGit(wsPath, "config", "user.email", "agent@starclaw.local")
	t.runGit(wsPath, "config", "user.name", "Claw Agent")

	// Create initial commit so branches work
	readmePath := filepath.Join(wsPath, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		os.WriteFile(readmePath, []byte("# Project\n\nInitialized by Claw Agent.\n"), 0644)
	}
	t.runGit(wsPath, "add", ".")
	t.runGit(wsPath, "commit", "-m", "Initial commit", "--allow-empty")

	return toJSON(map[string]interface{}{
		"action": "init",
		"status": "success",
		"path":   wsPath,
	}), nil
}

func (t *GitTool) gitClone(wsPath string, a gitArgs) (string, error) {
	if a.RepoURL == "" {
		return toJSON(map[string]interface{}{"action": "clone", "error": "repo_url is required"}), nil
	}

	// Clone into workspace path
	out, err := t.runGit(filepath.Dir(wsPath), "clone", a.RepoURL, wsPath)
	if err != nil {
		return toJSON(map[string]interface{}{"action": "clone", "error": err.Error(), "output": out}), nil
	}

	// Configure git user
	t.runGit(wsPath, "config", "user.email", "agent@starclaw.local")
	t.runGit(wsPath, "config", "user.name", "Claw Agent")

	return toJSON(map[string]interface{}{
		"action":   "clone",
		"status":   "success",
		"repo_url": a.RepoURL,
		"path":     wsPath,
	}), nil
}

func (t *GitTool) gitAdd(wsPath string, a gitArgs) (string, error) {
	path := a.Path
	if path == "" {
		path = "."
	}
	out, err := t.runGit(wsPath, "add", path)
	if err != nil {
		return toJSON(map[string]interface{}{"action": "add", "error": err.Error(), "output": out}), nil
	}
	return toJSON(map[string]interface{}{
		"action": "add",
		"status": "success",
		"path":   path,
	}), nil
}

func (t *GitTool) gitCommit(wsPath string, a gitArgs) (string, error) {
	msg := a.Message
	if msg == "" {
		msg = "Update from Claw Agent"
	}
	out, err := t.runGit(wsPath, "commit", "-m", msg)
	if err != nil {
		// Check if it's "nothing to commit"
		if strings.Contains(out, "nothing to commit") {
			return toJSON(map[string]interface{}{
				"action":  "commit",
				"status":  "nothing_to_commit",
				"message": "No changes to commit",
			}), nil
		}
		return toJSON(map[string]interface{}{"action": "commit", "error": err.Error(), "output": out}), nil
	}

	// Extract commit hash
	hashOut, _ := t.runGit(wsPath, "rev-parse", "HEAD")
	hash := strings.TrimSpace(hashOut)

	return toJSON(map[string]interface{}{
		"action":      "commit",
		"status":      "success",
		"message":     msg,
		"commit_hash": hash,
	}), nil
}

func (t *GitTool) gitPush(wsPath string, a gitArgs) (string, error) {
	args := []string{"push"}
	if a.Branch != "" {
		args = append(args, "origin", a.Branch)
	}
	out, err := t.runGit(wsPath, args...)
	if err != nil {
		return toJSON(map[string]interface{}{"action": "push", "error": err.Error(), "output": out}), nil
	}
	return toJSON(map[string]interface{}{
		"action": "push",
		"status": "success",
		"output": out,
	}), nil
}

func (t *GitTool) gitPull(wsPath string, a gitArgs) (string, error) {
	args := []string{"pull"}
	if a.Branch != "" {
		args = append(args, "origin", a.Branch)
	}
	out, err := t.runGit(wsPath, args...)
	if err != nil {
		return toJSON(map[string]interface{}{"action": "pull", "error": err.Error(), "output": out}), nil
	}
	return toJSON(map[string]interface{}{
		"action": "pull",
		"status": "success",
		"output": out,
	}), nil
}

func (t *GitTool) gitBranch(wsPath string, a gitArgs) (string, error) {
	if a.Branch != "" {
		// Create new branch
		out, err := t.runGit(wsPath, "branch", a.Branch)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "branch", "error": err.Error(), "output": out}), nil
		}
		return toJSON(map[string]interface{}{
			"action": "branch",
			"status": "created",
			"branch": a.Branch,
		}), nil
	}

	// List branches
	out, _ := t.runGit(wsPath, "branch", "-a")
	branches := []string{}
	current := ""
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "* ") {
			current = strings.TrimPrefix(line, "* ")
			branches = append(branches, current)
		} else {
			branches = append(branches, line)
		}
	}
	return toJSON(map[string]interface{}{
		"action":         "branch",
		"branches":       branches,
		"current_branch": current,
	}), nil
}

func (t *GitTool) gitCheckout(wsPath string, a gitArgs) (string, error) {
	if a.Branch == "" {
		return toJSON(map[string]interface{}{"action": "checkout", "error": "branch is required"}), nil
	}

	// Try checkout existing, if fails create new branch
	_, err := t.runGit(wsPath, "checkout", a.Branch)
	if err != nil {
		out, err := t.runGit(wsPath, "checkout", "-b", a.Branch)
		if err != nil {
			return toJSON(map[string]interface{}{"action": "checkout", "error": err.Error(), "output": out}), nil
		}
	}

	return toJSON(map[string]interface{}{
		"action": "checkout",
		"status": "success",
		"branch": a.Branch,
	}), nil
}

func (t *GitTool) gitMerge(wsPath string, a gitArgs) (string, error) {
	if a.Branch == "" {
		return toJSON(map[string]interface{}{"action": "merge", "error": "branch is required"}), nil
	}

	out, err := t.runGit(wsPath, "merge", "--no-ff", a.Branch, "-m", fmt.Sprintf("Merge branch '%s'", a.Branch))
	if err != nil {
		// Check for merge conflicts
		if strings.Contains(out, "CONFLICT") || strings.Contains(out, "conflict") {
			statusOut, _ := t.runGit(wsPath, "status", "--porcelain")
			return toJSON(map[string]interface{}{
				"action":    "merge",
				"status":    "conflict",
				"branch":    a.Branch,
				"conflicts": statusOut,
				"message":   "Merge conflicts detected. Resolve conflicts, then add and commit.",
			}), nil
		}
		return toJSON(map[string]interface{}{"action": "merge", "error": err.Error(), "output": out}), nil
	}

	return toJSON(map[string]interface{}{
		"action": "merge",
		"status": "success",
		"branch": a.Branch,
		"output": out,
	}), nil
}

func (t *GitTool) gitStatus(wsPath string) (string, error) {
	out, _ := t.runGit(wsPath, "status", "--porcelain")
	branchOut, _ := t.runGit(wsPath, "rev-parse", "--abbrev-ref", "HEAD")

	changes := []map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if len(line) < 3 {
			continue
		}
		status := strings.TrimSpace(line[:2])
		file := strings.TrimSpace(line[3:])
		changes = append(changes, map[string]string{
			"status": status,
			"file":   file,
		})
	}

	return toJSON(map[string]interface{}{
		"action":  "status",
		"branch":  strings.TrimSpace(branchOut),
		"changes": changes,
		"clean":   len(changes) == 0,
	}), nil
}

func (t *GitTool) gitLog(wsPath string, a gitArgs) (string, error) {
	count := a.Count
	if count <= 0 || count > 50 {
		count = 10
	}
	out, _ := t.runGit(wsPath, "log", fmt.Sprintf("-%d", count), "--oneline", "--decorate")

	commits := []map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, map[string]string{
				"hash":    parts[0],
				"message": parts[1],
			})
		}
	}

	return toJSON(map[string]interface{}{
		"action":  "log",
		"commits": commits,
	}), nil
}

func (t *GitTool) gitDiff(wsPath string, a gitArgs) (string, error) {
	args := []string{"diff"}
	if a.Branch != "" {
		args = append(args, a.Branch)
	}
	if a.Path != "" {
		args = append(args, "--", a.Path)
	}
	out, _ := t.runGit(wsPath, args...)

	// Truncate long diffs
	if len(out) > 10000 {
		out = out[:10000] + "\n... [diff truncated]"
	}

	return toJSON(map[string]interface{}{
		"action": "diff",
		"diff":   out,
	}), nil
}

// runGit executes a git command in the given directory and returns stdout+stderr
func (t *GitTool) runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := hiddenCmdCtx(ctx, "git", args...)
	cmd.Dir = dir
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
		log.Printf("[GitTool] git %s in %s: error=%v output=%s", strings.Join(args, " "), dir, err, out)
	}

	return out, err
}
