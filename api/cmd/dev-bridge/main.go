package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ───────────────────────────────────────────────
// StarClaw Dev Bridge — Development MCP Server
//
// Bridges DevClaw (product layer) ↔ Windsurf (code layer).
// Exposes MCP JSON-RPC 2.0 tools for:
//   - Git branch management (parallel development)
//   - Task handoff (DevClaw → Windsurf and back)
//   - Service build/deploy status
//   - Agent sandbox test & publish (via Claw API)
//
// Usage:
//   go run ./cmd/dev-bridge                  # default :9102
//   go run ./cmd/dev-bridge -port 9102 -repo /path/to/starclaw
//
// Then add in Claw → Settings → MCP 工具:
//   Name:    dev
//   BaseURL: http://localhost:9102
// ───────────────────────────────────────────────

var version = "dev"

// --- JSON-RPC types (same as mcp-bridge) ---

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// --- Config ---

var (
	repoDir  string // StarClaw monorepo root
	clawAPI  string // Claw API base URL for agent sandbox/publish
	taskFile string // JSON file for task persistence
)

// --- Tool definitions ---

type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

func getTools() []toolDef {
	return []toolDef{
		// ── Git 分支管理 ──
		{
			Name:        "git_status",
			Description: "查看 StarClaw 仓库当前分支和工作区状态",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "git_branches",
			Description: "列出所有功能分支（feat/fix/hotfix），显示最后提交信息",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": prop("string", "筛选模式（如 feat/claw/*），留空列出全部"),
				},
			},
		},
		{
			Name:        "git_create_branch",
			Description: "从 master 创建新的功能分支。分支名需遵循 feat/{service}/{name} 或 fix/{service}/{name} 格式",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"branch": prop("string", "分支名，如 feat/claw/memory-v2"),
				},
				"required": []string{"branch"},
			},
		},
		{
			Name:        "git_diff",
			Description: "查看某分支与 master 之间的文件变更摘要",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"branch": prop("string", "分支名（留空则用当前分支）"),
					"full":   propBool("显示完整 diff（默认只显示 --stat 摘要）"),
				},
			},
		},
		{
			Name:        "git_merge",
			Description: "将指定分支合并到 master（--no-ff 模式）。合并前自动 fetch + rebase",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"branch": prop("string", "要合并的分支名"),
				},
				"required": []string{"branch"},
			},
		},
		{
			Name:        "git_log",
			Description: "查看指定分支最近的提交记录",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"branch": prop("string", "分支名（留空则用当前分支）"),
					"count":  propInt("显示条数（默认10）"),
				},
			},
		},
		// ── 任务管理 (DevClaw ↔ Windsurf 桥接) ──
		{
			Name:        "task_create",
			Description: "创建开发任务。DevClaw 可以用此请求 Windsurf 实现代码需求，或反向通知 DevClaw 代码就绪",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":    prop("string", "任务标题"),
					"desc":     prop("string", "详细描述"),
					"from":     prop("string", "来源: devclaw / windsurf / user"),
					"to":       prop("string", "目标: devclaw / windsurf / user"),
					"service":  prop("string", "涉及的服务目录（如 claw/api）"),
					"priority": prop("string", "优先级: high / medium / low"),
				},
				"required": []string{"title", "from", "to"},
			},
		},
		{
			Name:        "task_list",
			Description: "查看所有开发任务，支持按状态和目标筛选",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": prop("string", "筛选状态: pending / in_progress / done / all（默认 all）"),
					"to":     prop("string", "筛选目标: devclaw / windsurf / user"),
				},
			},
		},
		{
			Name:        "task_update",
			Description: "更新任务状态或添加备注",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":     prop("string", "任务 ID"),
					"status": prop("string", "新状态: pending / in_progress / done"),
					"note":   prop("string", "备注信息"),
				},
				"required": []string{"id"},
			},
		},
		// ── 服务管理 ──
		{
			Name:        "service_list",
			Description: "列出 StarClaw monorepo 中所有服务及其状态（go.mod 是否存在、最后修改时间）",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "service_build",
			Description: "构建指定服务（go build / npm run build），验证代码是否可编译",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"service": prop("string", "服务名: claw-api / claw-web / hive / queen-api / synapse-api / spore / overlord-api / nydus / carapace"),
				},
				"required": []string{"service"},
			},
		},
		{
			Name:        "deploy_push",
			Description: "推送当前 master 到 nydus 触发自动部署（git push nydus master）",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		// ── Agent 沙箱/发布 (调用 Claw API) ──
		{
			Name:        "agent_test",
			Description: "在 Claw 沙箱中测试 Agent 配置。传入 system_prompt + model + tools + 测试问题，返回测试结果和评分",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"system_prompt": prop("string", "Agent 的系统提示词"),
					"model":         prop("string", "模型名（如 deepseek-chat）"),
					"tools":         prop("string", "工具列表 JSON 数组（如 [\"web_search\"]）"),
					"test_messages":  prop("string", "测试消息 JSON 数组（如 [\"你好\",\"帮我开处方\"]）"),
					"claw_url":      prop("string", "Claw API 地址（默认用启动参数）"),
					"token":         prop("string", "Claw API Token"),
				},
				"required": []string{"system_prompt", "test_messages"},
			},
		},
		{
			Name:        "agent_publish",
			Description: "将 Agent 配置发布到 Claw 市场",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":          prop("string", "Agent 名称"),
					"description":   prop("string", "一句话描述"),
					"system_prompt": prop("string", "系统提示词"),
					"model":         prop("string", "模型名"),
					"tools":         prop("string", "工具列表 JSON"),
					"category":      prop("string", "分类: assistant/medical/coding/writing/finance/legal/education"),
					"tags":          prop("string", "标签 JSON 数组"),
					"icon":          prop("string", "图标 emoji"),
					"claw_url":      prop("string", "Claw API 地址"),
					"token":         prop("string", "Claw API Token"),
				},
				"required": []string{"name", "system_prompt", "token"},
			},
		},
	}
}

// --- Schema helpers ---

func prop(typ, desc string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": desc}
}
func propBool(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "boolean", "description": desc}
}
func propInt(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "integer", "description": desc}
}

// ═══════════════════════════════════════════════
// Git Tool Implementations
// ═══════════════════════════════════════════════

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func execGitStatus(_ map[string]interface{}) mcpToolResult {
	branch, _ := git("branch", "--show-current")
	status, _ := git("status", "--short")
	lastCommit, _ := git("log", "-1", "--oneline")

	result := fmt.Sprintf("Branch:  %s\nLast:    %s\n", branch, lastCommit)
	if status != "" {
		result += fmt.Sprintf("Changes:\n%s\n", status)
	} else {
		result += "Changes: (clean)\n"
	}
	return textResult(result)
}

func execGitBranches(args map[string]interface{}) mcpToolResult {
	pattern, _ := args["pattern"].(string)

	gitArgs := []string{"branch", "-a", "--format=%(refname:short) %(upstream:short) %(committerdate:relative) %(subject)"}
	out, err := git(gitArgs...)
	if err != nil {
		return errResult("git branch: " + out)
	}

	lines := strings.Split(out, "\n")
	var filtered []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Filter to feature branches
		if strings.HasPrefix(line, "feat/") || strings.HasPrefix(line, "fix/") ||
			strings.HasPrefix(line, "hotfix/") || strings.HasPrefix(line, "refactor/") {
			if pattern == "" || strings.Contains(line, pattern) {
				filtered = append(filtered, line)
			}
		}
	}

	if len(filtered) == 0 {
		return textResult("No feature branches found. All work is on master.")
	}
	return textResult(strings.Join(filtered, "\n"))
}

func execGitCreateBranch(args map[string]interface{}) mcpToolResult {
	branch, _ := args["branch"].(string)
	if branch == "" {
		return errResult("branch name is required")
	}

	// Validate naming convention
	validPrefixes := []string{"feat/", "fix/", "hotfix/", "refactor/"}
	valid := false
	for _, p := range validPrefixes {
		if strings.HasPrefix(branch, p) {
			valid = true
			break
		}
	}
	if !valid {
		return errResult("branch name must start with feat/, fix/, hotfix/, or refactor/. Example: feat/claw/memory-v2")
	}

	// Fetch latest master
	if out, err := git("fetch", "nydus", "master"); err != nil {
		return errResult("fetch failed: " + out)
	}

	// Create branch from latest master
	if out, err := git("checkout", "-b", branch, "nydus/master"); err != nil {
		return errResult("create branch failed: " + out)
	}

	return textResult(fmt.Sprintf("Created branch '%s' from latest master.\nYou are now on this branch.", branch))
}

func execGitDiff(args map[string]interface{}) mcpToolResult {
	branch, _ := args["branch"].(string)
	full, _ := args["full"].(bool)

	if branch == "" {
		b, _ := git("branch", "--show-current")
		branch = b
	}

	diffArgs := []string{"diff", "master..." + branch}
	if !full {
		diffArgs = append(diffArgs, "--stat")
	}

	out, _ := git(diffArgs...)
	if out == "" {
		return textResult(fmt.Sprintf("No differences between master and %s", branch))
	}

	if len(out) > 50000 {
		out = out[:50000] + "\n...[truncated, use full=false for summary]"
	}
	return textResult(out)
}

func execGitMerge(args map[string]interface{}) mcpToolResult {
	branch, _ := args["branch"].(string)
	if branch == "" {
		return errResult("branch name is required")
	}

	// Fetch latest
	if out, err := git("fetch", "nydus"); err != nil {
		return errResult("fetch failed: " + out)
	}

	// Switch to master
	if out, err := git("checkout", "master"); err != nil {
		return errResult("checkout master failed: " + out)
	}

	// Pull latest master
	if out, err := git("pull", "nydus", "master"); err != nil {
		return errResult("pull master failed: " + out)
	}

	// Merge with --no-ff
	mergeMsg := fmt.Sprintf("Merge branch '%s' into master", branch)
	if out, err := git("merge", "--no-ff", "-m", mergeMsg, branch); err != nil {
		return errResult("merge failed (resolve conflicts manually):\n" + out)
	}

	return textResult(fmt.Sprintf("Merged '%s' into master.\nRun deploy_push to deploy, or review with git_log first.", branch))
}

func execGitLog(args map[string]interface{}) mcpToolResult {
	branch, _ := args["branch"].(string)
	countF, _ := args["count"].(float64)
	count := 10
	if countF > 0 {
		count = int(countF)
	}

	gitArgs := []string{"log", "--oneline", fmt.Sprintf("-n%d", count)}
	if branch != "" {
		gitArgs = append(gitArgs, branch)
	}

	out, err := git(gitArgs...)
	if err != nil {
		return errResult("git log: " + out)
	}
	return textResult(out)
}

// ═══════════════════════════════════════════════
// Task Management
// ═══════════════════════════════════════════════

type Task struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Desc      string `json:"desc,omitempty"`
	From      string `json:"from"`      // devclaw / windsurf / user
	To        string `json:"to"`        // devclaw / windsurf / user
	Service   string `json:"service,omitempty"`
	Priority  string `json:"priority,omitempty"`
	Status    string `json:"status"` // pending / in_progress / done
	Notes     string `json:"notes,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

var (
	tasks   []Task
	tasksMu sync.Mutex
	taskSeq int
)

func loadTasks() {
	data, err := os.ReadFile(taskFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &tasks)
	for _, t := range tasks {
		var id int
		fmt.Sscanf(t.ID, "T%d", &id)
		if id > taskSeq {
			taskSeq = id
		}
	}
}

func saveTasks() {
	data, _ := json.MarshalIndent(tasks, "", "  ")
	os.WriteFile(taskFile, data, 0644)
}

func execTaskCreate(args map[string]interface{}) mcpToolResult {
	tasksMu.Lock()
	defer tasksMu.Unlock()

	taskSeq++
	now := time.Now().Format("2006-01-02 15:04:05")
	t := Task{
		ID:        fmt.Sprintf("T%03d", taskSeq),
		Title:     strVal(args, "title"),
		Desc:      strVal(args, "desc"),
		From:      strVal(args, "from"),
		To:        strVal(args, "to"),
		Service:   strVal(args, "service"),
		Priority:  orDefault(strVal(args, "priority"), "medium"),
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	tasks = append(tasks, t)
	saveTasks()

	return textResult(fmt.Sprintf("Task created: %s\n  Title:    %s\n  From:     %s → To: %s\n  Service:  %s\n  Priority: %s",
		t.ID, t.Title, t.From, t.To, t.Service, t.Priority))
}

func execTaskList(args map[string]interface{}) mcpToolResult {
	tasksMu.Lock()
	defer tasksMu.Unlock()

	statusFilter := orDefault(strVal(args, "status"), "all")
	toFilter := strVal(args, "to")

	var lines []string
	for _, t := range tasks {
		if statusFilter != "all" && t.Status != statusFilter {
			continue
		}
		if toFilter != "" && t.To != toFilter {
			continue
		}
		icon := "⏳"
		switch t.Status {
		case "in_progress":
			icon = "🔨"
		case "done":
			icon = "✅"
		}
		line := fmt.Sprintf("%s %s [%s] %s→%s: %s", icon, t.ID, t.Priority, t.From, t.To, t.Title)
		if t.Service != "" {
			line += fmt.Sprintf(" (%s)", t.Service)
		}
		if t.Notes != "" {
			line += fmt.Sprintf("\n     Note: %s", t.Notes)
		}
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return textResult("No tasks found.")
	}
	return textResult(strings.Join(lines, "\n"))
}

func execTaskUpdate(args map[string]interface{}) mcpToolResult {
	tasksMu.Lock()
	defer tasksMu.Unlock()

	id := strVal(args, "id")
	for i := range tasks {
		if tasks[i].ID == id {
			if s := strVal(args, "status"); s != "" {
				tasks[i].Status = s
			}
			if n := strVal(args, "note"); n != "" {
				if tasks[i].Notes != "" {
					tasks[i].Notes += "\n" + n
				} else {
					tasks[i].Notes = n
				}
			}
			tasks[i].UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
			saveTasks()
			return textResult(fmt.Sprintf("Updated %s: status=%s", id, tasks[i].Status))
		}
	}
	return errResult("task not found: " + id)
}

// ═══════════════════════════════════════════════
// Service Management
// ═══════════════════════════════════════════════

type serviceInfo struct {
	Name    string
	Dir     string
	Type    string // go / node / library
	BuildCmd string
}

var services = []serviceInfo{
	{"claw-api", "claw/api", "go", "go build -o /dev/null ./cmd/server"},
	{"claw-web", "claw/web", "node", "npm ci && npx vite build"},
	{"hive", "hive/api", "go", "go build -o /dev/null ./cmd/"},
	{"queen-api", "queen/api", "go", "go build -o /dev/null ./cmd/server"},
	{"queen-swarm", "queen/swarm", "go", "go build -o /dev/null ./cmd/server"},
	{"queen-arena", "queen/arena", "go", "go build -o /dev/null ./cmd/server"},
	{"synapse-api", "synapse/api", "go", "go build -o /dev/null ./cmd/server"},
	{"synapse-core", "synapse/core", "node", "npm ci && npx vite build"},
	{"overlord-api", "overlord/api", "go", "go build -o /dev/null ./cmd/server"},
	{"overlord-web", "overlord/web", "node", "npm ci && npx vite build"},
	{"nydus", "nydus/api", "go", "go build -o /dev/null ./cmd/server"},
	{"spore", "spore", "go", "go build -o /dev/null ./cmd/spore/"},
	{"carapace", "carapace", "go", "go test ./..."},
}

func execServiceList(_ map[string]interface{}) mcpToolResult {
	var lines []string
	for _, s := range services {
		dir := filepath.Join(repoDir, s.Dir)
		status := "✅"
		if _, err := os.Stat(dir); err != nil {
			status = "❌ missing"
		}

		// Get last commit date for this dir
		lastCommit, _ := git("log", "-1", "--format=%cr", "--", s.Dir)
		if lastCommit == "" {
			lastCommit = "unknown"
		}

		lines = append(lines, fmt.Sprintf("%s %-16s %-6s %s  (last: %s)", status, s.Name, s.Type, s.Dir, lastCommit))
	}
	return textResult(strings.Join(lines, "\n"))
}

func execServiceBuild(args map[string]interface{}) mcpToolResult {
	name := strVal(args, "service")
	var svc *serviceInfo
	for _, s := range services {
		if s.Name == name {
			svc = &s
			break
		}
	}
	if svc == nil {
		names := make([]string, len(services))
		for i, s := range services {
			names[i] = s.Name
		}
		return errResult(fmt.Sprintf("Unknown service '%s'. Available: %s", name, strings.Join(names, ", ")))
	}

	dir := filepath.Join(repoDir, svc.Dir)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-Command", svc.BuildCmd)
	} else {
		cmd = exec.Command("bash", "-c", svc.BuildCmd)
	}
	cmd.Dir = dir

	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	result := fmt.Sprintf("Service: %s\nDir:     %s\nCmd:     %s\nTime:    %s\n", svc.Name, svc.Dir, svc.BuildCmd, elapsed.Round(time.Millisecond))
	if err != nil {
		result += fmt.Sprintf("Status:  FAILED\nOutput:\n%s", string(out))
		return mcpToolResult{Content: []mcpContent{{Type: "text", Text: result}}, IsError: true}
	}
	result += "Status:  OK"
	if len(out) > 0 {
		result += "\nOutput:\n" + string(out)
	}
	return textResult(result)
}

func execDeployPush(_ map[string]interface{}) mcpToolResult {
	// Check we're on master
	branch, _ := git("branch", "--show-current")
	if branch != "master" {
		return errResult(fmt.Sprintf("Not on master (current: %s). Switch to master first.", branch))
	}

	out, err := git("push", "nydus", "master")
	if err != nil {
		return errResult("push failed: " + out)
	}
	return textResult("Pushed master to nydus. Auto-deploy triggered.\n" + out)
}

// ═══════════════════════════════════════════════
// Agent Sandbox Test & Publish (via Claw API)
// ═══════════════════════════════════════════════

func execAgentTest(args map[string]interface{}) mcpToolResult {
	url := orDefault(strVal(args, "claw_url"), clawAPI)
	token := strVal(args, "token")
	prompt := strVal(args, "system_prompt")
	model := orDefault(strVal(args, "model"), "deepseek-chat")
	tools := orDefault(strVal(args, "tools"), "[]")
	testMsgs := strVal(args, "test_messages")

	if url == "" {
		return errResult("claw_url is required (or set -claw flag)")
	}
	if prompt == "" {
		return errResult("system_prompt is required")
	}

	// Parse test messages
	var messages []string
	json.Unmarshal([]byte(testMsgs), &messages)
	if len(messages) == 0 {
		return errResult("test_messages must be a JSON array of strings")
	}

	// Build sandbox request
	reqBody := map[string]interface{}{
		"system_prompt": prompt,
		"model":         model,
		"tools":         tools,
		"test_messages":  messages,
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", url+"/v1/internal/agent-sandbox", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult("Claw API error: " + err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return errResult(fmt.Sprintf("Claw API %d: %s", resp.StatusCode, string(respBody)))
	}

	return textResult(string(respBody))
}

func execAgentPublish(args map[string]interface{}) mcpToolResult {
	url := orDefault(strVal(args, "claw_url"), clawAPI)
	token := strVal(args, "token")

	if url == "" {
		return errResult("claw_url is required")
	}
	if token == "" {
		return errResult("token is required")
	}

	// Step 1: Create agent
	agentBody := map[string]interface{}{
		"name":          strVal(args, "name"),
		"description":   strVal(args, "description"),
		"system_prompt": strVal(args, "system_prompt"),
		"model_name":    orDefault(strVal(args, "model"), "deepseek-chat"),
		"tools":         orDefault(strVal(args, "tools"), "[]"),
		"icon":          orDefault(strVal(args, "icon"), "🤖"),
		"is_public":     true,
	}
	bodyJSON, _ := json.Marshal(agentBody)

	req, _ := http.NewRequest("POST", url+"/v1/agents", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult("Create agent error: " + err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return errResult(fmt.Sprintf("Create agent %d: %s", resp.StatusCode, string(respBody)))
	}

	// Parse agent ID from response
	var agentResp map[string]interface{}
	json.Unmarshal(respBody, &agentResp)

	agentData, _ := agentResp["agent"].(map[string]interface{})
	agentID := ""
	if agentData != nil {
		if id, ok := agentData["id"].(float64); ok {
			agentID = fmt.Sprintf("%.0f", id)
		}
	}

	// Step 2: Create template for marketplace
	category := orDefault(strVal(args, "category"), "assistant")
	tags := orDefault(strVal(args, "tags"), "[]")

	tplBody := map[string]interface{}{
		"agent_id": agentID,
		"category": category,
		"tags":     tags,
	}
	tplJSON, _ := json.Marshal(tplBody)

	tplReq, _ := http.NewRequest("POST", url+"/v1/templates", strings.NewReader(string(tplJSON)))
	tplReq.Header.Set("Content-Type", "application/json")
	tplReq.Header.Set("Authorization", "Bearer "+token)

	tplResp, err := client.Do(tplReq)
	if err != nil {
		return textResult(fmt.Sprintf("Agent created (ID: %s) but template publish failed: %s", agentID, err.Error()))
	}
	defer tplResp.Body.Close()
	tplRespBody, _ := io.ReadAll(tplResp.Body)

	return textResult(fmt.Sprintf("Agent published!\n  Agent ID: %s\n  Category: %s\n  Response: %s", agentID, category, string(tplRespBody)))
}

// ═══════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════

func strVal(args map[string]interface{}, key string) string {
	v, _ := args[key].(string)
	return v
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func textResult(text string) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: text}}}
}

func errResult(msg string) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: msg}}, IsError: true}
}

// ═══════════════════════════════════════════════
// RPC Dispatcher + HTTP Handler
// ═══════════════════════════════════════════════

func callTool(name string, args map[string]interface{}) mcpToolResult {
	switch name {
	// Git
	case "git_status":
		return execGitStatus(args)
	case "git_branches":
		return execGitBranches(args)
	case "git_create_branch":
		return execGitCreateBranch(args)
	case "git_diff":
		return execGitDiff(args)
	case "git_merge":
		return execGitMerge(args)
	case "git_log":
		return execGitLog(args)
	// Tasks
	case "task_create":
		return execTaskCreate(args)
	case "task_list":
		return execTaskList(args)
	case "task_update":
		return execTaskUpdate(args)
	// Services
	case "service_list":
		return execServiceList(args)
	case "service_build":
		return execServiceBuild(args)
	case "deploy_push":
		return execDeployPush(args)
	// Agent
	case "agent_test":
		return execAgentTest(args)
	case "agent_publish":
		return execAgentPublish(args)
	default:
		return errResult("unknown tool: " + name)
	}
}

func dispatch(method string, params json.RawMessage) (interface{}, *rpcError) {
	switch method {
	case "tools/list":
		return map[string]interface{}{"tools": getTools()}, nil

	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
		}
		log.Printf("[dev-bridge] tool call: %s", p.Name)
		result := callTool(p.Name, p.Arguments)
		return result, nil

	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]string{"name": "starclaw-dev-bridge", "version": version},
			"capabilities":    map[string]interface{}{"tools": map[string]bool{"listChanged": false}},
		}, nil

	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(200)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/health" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok", "service": "dev-bridge", "version": version,
			"repo": repoDir,
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0", ID: nil,
			Error: &rpcError{Code: -32700, Message: "parse error"},
		})
		return
	}

	result, rpcErr := dispatch(req.Method, req.Params)

	resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	port := "9102"
	repoDir, _ = os.Getwd()
	clawAPI = "http://localhost:8080"

	for i, arg := range os.Args {
		if (arg == "-port" || arg == "--port") && i+1 < len(os.Args) {
			port = os.Args[i+1]
		}
		if (arg == "-repo" || arg == "--repo") && i+1 < len(os.Args) {
			repoDir = os.Args[i+1]
		}
		if (arg == "-claw" || arg == "--claw") && i+1 < len(os.Args) {
			clawAPI = os.Args[i+1]
		}
	}

	// Task persistence
	taskFile = filepath.Join(repoDir, ".dev-bridge-tasks.json")
	loadTasks()

	fmt.Printf(`
  ╔══════════════════════════════════════════════════╗
  ║   StarClaw Dev Bridge v%-10s                ║
  ║   Development MCP Server                        ║
  ╠══════════════════════════════════════════════════╣
  ║   Port:     :%-5s                              ║
  ║   Repo:     %-36s  ║
  ║   Claw API: %-36s  ║
  ╠══════════════════════════════════════════════════╣
  ║   Git:   git_status, git_branches,              ║
  ║          git_create_branch, git_diff,            ║
  ║          git_merge, git_log                      ║
  ║   Tasks: task_create, task_list, task_update     ║
  ║   Build: service_list, service_build,            ║
  ║          deploy_push                             ║
  ║   Agent: agent_test, agent_publish               ║
  ╠══════════════════════════════════════════════════╣
  ║   Add in Claw → Settings → MCP 工具:            ║
  ║   Name: dev    URL: http://localhost:%s        ║
  ╚══════════════════════════════════════════════════╝
`, version, port, repoDir, clawAPI, port)

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
