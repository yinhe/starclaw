package squad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// Engine orchestrates multi-node squad missions.
// It runs on the Captain node and handles:
// 1. Planning: decomposing a mission goal into steps via the orchestrator agent
// 2. Dispatching: sending steps to member nodes via Nydus/HTTP
// 3. Tracking: monitoring callbacks and advancing the mission
// 4. Sprint lifecycle: iterative development with Git collaboration
type Engine struct {
	db           *gorm.DB
	identity     *node.Identity
	nydus        *node.NydusManager
	hivemind     *node.HivemindEngine
	providerReg  *provider.Registry
	toolRegistry *tool.Registry
	gitMgr       *GitManager
	httpC        *http.Client
	selfAddress  string

	mu      sync.Mutex
	stopCh  chan struct{}
	started bool
}

// NewEngine creates a new Squad engine.
func NewEngine(db *gorm.DB, identity *node.Identity, nydus *node.NydusManager, hivemind *node.HivemindEngine, providerReg *provider.Registry, toolRegistry *tool.Registry, selfAddress string) *Engine {
	gm := NewGitManager()
	gm.SetSelfAddress(selfAddress)

	e := &Engine{
		db:           db,
		identity:     identity,
		nydus:        nydus,
		hivemind:     hivemind,
		providerReg:  providerReg,
		toolRegistry: toolRegistry,
		gitMgr:       gm,
		httpC:        &http.Client{Timeout: 60 * time.Second},
		selfAddress:  selfAddress,
		stopCh:       make(chan struct{}),
	}

	// Wire LLM-based conflict resolver into GitManager
	gm.SetConflictResolver(e.llmResolveConflict)

	return e
}

// Start begins the engine's background loop that watches for missions to execute.
func (e *Engine) Start() {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return
	}
	e.started = true
	e.mu.Unlock()

	log.Println("[squad] engine started")
	go e.loop()

	// Start Hive Mind real-time broadcaster
	hiveBroadcaster := NewHiveBroadcaster(e.db)
	go hiveBroadcaster.Start()
}

// Stop shuts down the engine.
func (e *Engine) Stop() {
	close(e.stopCh)
	log.Println("[squad] engine stopped")
}

// loop checks for missions that need planning or have pending dispatches.
func (e *Engine) loop() {
	for {
		select {
		case <-e.stopCh:
			return
		case <-time.After(5 * time.Second):
		}

		// Find missions that just started (status = executing, no steps yet)
		var missions []model.Mission
		e.db.Where("status = ? AND captain_node = ? AND total_steps = 0",
			"executing", e.identity.NodeID).Find(&missions)

		for _, m := range missions {
			e.planAndDispatch(m)
		}

		// Find missions in executing state with steps ready to dispatch
		e.advanceAllMissions()
	}
}

// ── Planning ──

// MissionPlan is the output of the orchestrator agent's planning phase.
type MissionPlan struct {
	Steps []PlannedStep `json:"steps"`
}

// PlannedStep is a single step in the plan.
type PlannedStep struct {
	Title      string `json:"title"`
	Task       string `json:"task"`
	Specialty  string `json:"specialty"`             // which specialty should handle this
	AgentName  string `json:"agent_name"`            // preferred agent name (optional)
	DependsOn  []int  `json:"depends_on"`            // indices of prerequisite steps (0-based)
	TargetNode string `json:"target_node,omitempty"` // filled in during dispatch
}

// planAndDispatch uses a local LLM to decompose the mission and create steps.
// It initializes a Git repo, creates a Sprint, assigns branches, and dispatches.
func (e *Engine) planAndDispatch(mission model.Mission) {
	log.Printf("[squad] planning mission %s: %s", mission.ID, mission.Title)

	// Immediately mark as planning to prevent re-entry from the loop
	e.db.Model(&model.Mission{}).Where("id = ? AND total_steps = 0", mission.ID).
		Update("total_steps", -1)

	// Initialize Git bare repo for this mission
	repoPath, err := e.gitMgr.InitMissionRepo(mission.ID)
	if err != nil {
		log.Printf("[squad] git init failed for mission %s: %v", mission.ID, err)
		// Continue without git — fallback to text-only mode
	} else {
		e.db.Model(&model.Mission{}).Where("id = ?", mission.ID).Updates(map[string]interface{}{
			"repo_path":      repoPath,
			"workspace_path": e.gitMgr.GetMissionWorkspace(mission.ID),
		})
		mission.RepoPath = repoPath
	}

	// Get squad members
	var members []model.SquadMember
	e.db.Where("squad_id = ?", mission.SquadID).Find(&members)

	if len(members) == 0 {
		e.failMission(mission.ID, "no squad members found")
		return
	}

	// Build member capability description for the planner
	var memberDescs []string
	for _, m := range members {
		desc := fmt.Sprintf("- 节点 %s (角色: %s, 特长: %s)", m.NodeID[:min(16, len(m.NodeID))], m.Role, m.Specialty)
		if m.AgentExport != "" {
			desc += fmt.Sprintf(" Agent: %s", m.AgentExport)
		}
		memberDescs = append(memberDescs, desc)
	}

	// Use LLM to plan the mission
	plan, err := e.generatePlan(mission, memberDescs)
	if err != nil {
		log.Printf("[squad] planning failed for mission %s: %v", mission.ID, err)
		e.failMission(mission.ID, fmt.Sprintf("planning failed: %v", err))
		return
	}

	if len(plan.Steps) == 0 {
		e.failMission(mission.ID, "plan generated 0 steps")
		return
	}

	// Assign steps to nodes based on specialty matching
	e.assignStepsToNodes(plan, members)

	// Create Sprint 0 record
	sprintNum := mission.CurrentSprint
	sprint := model.Sprint{
		MissionID:  mission.ID,
		Number:     sprintNum,
		Goal:       fmt.Sprintf("Sprint %d: %s", sprintNum, mission.Title),
		Status:     "executing",
		TotalSteps: len(plan.Steps),
	}
	now := time.Now()
	sprint.StartedAt = &now
	e.db.Create(&sprint)

	// Create MissionStep records with branch assignments
	stepIDs := make([]string, len(plan.Steps))
	var branches []string
	for i, ps := range plan.Steps {
		stepID := uuid.New().String()
		stepIDs[i] = stepID

		// Convert depends_on from indices to step IDs
		var deps []string
		for _, depIdx := range ps.DependsOn {
			if depIdx >= 0 && depIdx < len(stepIDs) {
				deps = append(deps, stepIDs[depIdx])
			}
		}
		depsJSON, _ := json.Marshal(deps)

		// Assign a Git branch for this step
		branch := fmt.Sprintf("sprint-%d/step-%d-%s", sprintNum, i, sanitizeBranch(ps.Specialty))
		branches = append(branches, branch)

		step := model.MissionStep{
			ID:          stepID,
			MissionID:   mission.ID,
			SprintID:    sprint.ID,
			TargetNode:  ps.TargetNode,
			TargetAgent: ps.AgentName,
			Task:        ps.Task,
			Status:      "pending",
			DependsOn:   string(depsJSON),
			Sequence:    i,
			Branch:      branch,
		}
		e.db.Create(&step)
	}

	// Save plan and update mission
	planJSON, _ := json.Marshal(plan)
	e.db.Model(&model.Mission{}).Where("id = ?", mission.ID).Updates(map[string]interface{}{
		"plan":        string(planJSON),
		"total_steps": len(plan.Steps),
	})

	log.Printf("[squad] mission %s planned: %d steps, sprint %d, repo=%s",
		mission.ID, len(plan.Steps), sprintNum, repoPath)

	// Dispatch ready steps (those with no dependencies)
	e.advanceMission(mission.ID)
}

// sanitizeBranch makes a string safe for use as a git branch name
func sanitizeBranch(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	// Keep only alphanumeric and hyphens
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteRune(c)
		}
	}
	result := b.String()
	if result == "" {
		result = "task"
	}
	return result
}

// buildPlanPrompt constructs the LLM prompt for mission planning.
// For Overlord Team Agent missions (user_id starts with "overlord:"), it uses
// a specialized Architect Agent prompt with role-aware topology.
func (e *Engine) buildPlanPrompt(mission model.Mission, memberDescs []string) string {
	isOverlordMission := strings.HasPrefix(mission.UserID, "overlord:")

	if isOverlordMission {
		// Fetch squad tags (which contain template role codes from Overlord)
		var squad model.Squad
		e.db.First(&squad, "id = ?", mission.SquadID)
		roleTags := squad.Tags // JSON array of role codes, e.g. ["architect","coder","reviewer","tester"]

		return fmt.Sprintf(`你是 StarClaw Architect Agent — 一个专业的 AI 团队编排架构师。
你负责将企业任务分解为结构化的执行计划，由 AI 团队成员协作完成。

## 你的职责
1. 分析任务目标，拆解为可执行的原子步骤
2. 根据团队角色分配最合适的执行者
3. 设计步骤间的依赖关系和并行策略
4. 确保每个步骤都有明确的交付物

## 任务目标
%s

## AI 团队角色
%s

## 可用节点
%s

## 执行环境
- 每步骤独立 Git 分支 + 工作目录
- Agent 工具: code (读写文件/执行命令/启动服务) + git (add/commit/push)
- 完成后 Captain 自动 merge 所有分支
- 每步骤完成后触发 Code Review (最多 3 次重试)
- 所有步骤完成后进入 CI Gate (质量评分 + 自动预览)

## 输出格式
仅输出 JSON:
{
  "steps": [
    {
      "title": "步骤标题",
      "task": "详细任务描述。必须产出实际代码文件。描述包含：目标文件、技术栈、功能需求。完成后 git add + commit + push。",
      "specialty": "coding|design|video|writing|testing|sales|general",
      "agent_name": "推荐 Agent（可选）",
      "depends_on": []
    }
  ]
}

## 编排规则
1. 每步骤必须产出实际代码/配置文件
2. 最大化并行 — 无依赖的步骤同时执行
3. 步骤数 2-8 个，粒度适中
4. depends_on 用 0-based 索引
5. 先架构后实现：基础设施/数据模型 → 核心逻辑 → UI/测试
6. 只输出 JSON`, mission.Goal, roleTags, strings.Join(memberDescs, "\n"))
	}

	// Default prompt for regular squad missions
	return fmt.Sprintf(`你是一个敏捷开发编排专家。请将以下任务目标分解为多个可并行或串行执行的步骤。

## 任务目标
%s

## 可用团队成员
%s

## 执行环境
每个步骤会分配一个独立的 Git 分支和工作目录。Agent 拥有以下工具：
- code 工具：读写文件、执行代码、运行命令、启动 Web 服务
- git 工具：git add / commit / push 提交代码到分支

所有步骤完成后，Captain 会自动 merge 所有分支到 master。

请输出 JSON 格式的执行计划，格式如下：
{
  "steps": [
    {
      "title": "步骤标题",
      "task": "详细的任务描述。必须明确要求 Agent 编写实际可运行的代码文件，而不是只写文档或计划。描述应包含：要创建的文件、使用的技术栈、功能需求。完成后必须用 git 工具 add + commit + push 提交代码。",
      "specialty": "适合的特长类型 (coding/design/video/writing/testing/sales/general)",
      "agent_name": "推荐的 Agent 名称（可选）",
      "depends_on": []
    }
  ]
}

规则：
1. 每个步骤必须产出实际的代码文件，不能只输出文字说明
2. 任务描述中必须明确要求：写代码 → git add → git commit → git push
3. 尽可能并行化，减少串行依赖
4. 步骤数量 2-8 个，不要过多
5. depends_on 使用步骤在数组中的索引（0-based）
6. 只输出 JSON，不要其他文字`, mission.Goal, strings.Join(memberDescs, "\n"))
}

// generatePlan calls a local LLM to decompose the mission goal into steps.
func (e *Engine) generatePlan(mission model.Mission, memberDescs []string) (*MissionPlan, error) {
	prompt := e.buildPlanPrompt(mission, memberDescs)

	// Find a model to use for planning
	var modelCfg model.ModelConfig
	if err := e.db.Where("is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
		return nil, fmt.Errorf("no model available for planning")
	}

	p := provider.CreateFromConfig(e.providerReg, modelCfg)
	if p == nil {
		return nil, fmt.Errorf("failed to create provider for model %s", modelCfg.ModelName)
	}

	resp, err := p.ChatSync(context.Background(), &provider.ChatRequest{
		Model:       modelCfg.ModelName,
		Messages:    []provider.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   2000,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Extract JSON from response
	content := resp.Content
	// Try to find JSON in the response
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON found in LLM response: %.200s", content)
	}
	jsonStr := content[jsonStart : jsonEnd+1]

	var plan MissionPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w (raw: %.200s)", err, jsonStr)
	}

	return &plan, nil
}

// assignStepsToNodes matches each step's specialty to the best member node.
// It tries Hivemind agent-capability matching first, then falls back to member specialty.
func (e *Engine) assignStepsToNodes(plan *MissionPlan, members []model.SquadMember) {
	// Build a set of squad member node IDs for filtering
	memberSet := make(map[string]bool, len(members))
	for _, m := range members {
		memberSet[m.NodeID] = true
	}

	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.TargetNode != "" {
			continue // already assigned
		}

		// Strategy 1: Use Hivemind agent capability matching (best quality)
		if e.hivemind != nil && step.Specialty != "" {
			nodeID, agent := e.hivemind.FindBestNodeForSpecialty(step.Specialty, nil)
			if nodeID != "" && memberSet[nodeID] {
				step.TargetNode = nodeID
				if agent != nil && step.AgentName == "" {
					step.AgentName = agent.Name
				}
				log.Printf("[squad] step %d assigned to %s via Hivemind (agent: %s, specialty: %s)",
					i, nodeID[:min(16, len(nodeID))], step.AgentName, step.Specialty)
				continue
			}
		}

		// Strategy 2: Match by member specialty
		var bestMember *model.SquadMember
		for j := range members {
			m := &members[j]
			if m.Specialty == step.Specialty {
				bestMember = m
				break
			}
		}

		// Strategy 3: Round-robin fallback
		if bestMember == nil {
			bestMember = &members[i%len(members)]
		}

		step.TargetNode = bestMember.NodeID
		log.Printf("[squad] step %d assigned to %s via member-match (specialty: %s)",
			i, step.TargetNode[:min(16, len(step.TargetNode))], step.Specialty)
	}
}

// ── Dispatching ──

// advanceAllMissions checks all executing missions for steps ready to dispatch.
func (e *Engine) advanceAllMissions() {
	var missions []model.Mission
	e.db.Where("status = ? AND captain_node = ? AND total_steps > 0",
		"executing", e.identity.NodeID).Find(&missions)

	for _, m := range missions {
		e.advanceMission(m.ID)
	}
}

// advanceMission dispatches all steps whose dependencies are satisfied.
func (e *Engine) advanceMission(missionID string) {
	var steps []model.MissionStep
	e.db.Where("mission_id = ?", missionID).Order("sequence ASC").Find(&steps)

	// Build a set of completed step IDs
	doneSet := make(map[string]bool)
	for _, s := range steps {
		if s.Status == "done" {
			doneSet[s.ID] = true
		}
	}

	// Build a map of step outputs for context passing
	outputMap := make(map[string]string)
	for _, s := range steps {
		if s.Status == "done" && s.Output != "" {
			outputMap[s.ID] = s.Output
		}
	}

	for _, step := range steps {
		if step.Status != "pending" {
			continue
		}

		// Check if all dependencies are done
		allDepsDone := true
		var deps []string
		if step.DependsOn != "" && step.DependsOn != "null" {
			json.Unmarshal([]byte(step.DependsOn), &deps)
		}
		var contextParts []string
		for _, depID := range deps {
			if !doneSet[depID] {
				allDepsDone = false
				break
			}
			if output, ok := outputMap[depID]; ok {
				contextParts = append(contextParts, output)
			}
		}

		if !allDepsDone {
			continue
		}

		// All deps satisfied — dispatch this step
		contextStr := strings.Join(contextParts, "\n\n---\n\n")
		go e.dispatchStep(step, contextStr)
	}
}

// dispatchStep sends a step to the target node for execution.
func (e *Engine) dispatchStep(step model.MissionStep, contextStr string) {
	log.Printf("[squad] dispatching step %s to %s (branch=%s): %.80s",
		step.ID, step.TargetNode[:min(16, len(step.TargetNode))], step.Branch, step.Task)

	// Look up mission for repo info
	var mission model.Mission
	e.db.Where("id = ?", step.MissionID).First(&mission)

	// Mark as dispatched
	now := time.Now()
	e.db.Model(&step).Updates(map[string]interface{}{
		"status":        "dispatched",
		"input":         contextStr,
		"dispatched_at": &now,
	})

	// If target is self, execute locally
	if step.TargetNode == e.identity.NodeID {
		e.executeLocal(step, contextStr, mission)
		return
	}

	// Resolve target node address
	address := e.resolveNodeAddress(step.TargetNode)
	if address == "" {
		e.failStep(step.ID, step.MissionID, "cannot resolve node address: "+step.TargetNode)
		return
	}

	// Build callback URL (our own address)
	callbackURL := ""
	if e.selfAddress != "" {
		callbackURL = e.selfAddress + "/v1/peer/squad/callback"
	}

	// For remote nodes, provide HTTP URL so they can clone via Smart HTTP protocol.
	// Local nodes use the file:// path directly.
	repoURL := e.gitMgr.RepoHTTPURL(mission.ID)
	if repoURL == "" {
		repoURL = mission.RepoPath // fallback to bare path
	}

	// Send execute request (includes repo URL and branch for Git collaboration)
	reqBody := map[string]string{
		"mission_id":   step.MissionID,
		"step_id":      step.ID,
		"task":         step.Task,
		"context":      contextStr,
		"agent_name":   step.TargetAgent,
		"callback_url": callbackURL,
		"repo_path":    repoURL,
		"branch":       step.Branch,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := address + "/v1/peer/squad/execute"
	resp, err := e.httpC.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		e.failStep(step.ID, step.MissionID, fmt.Sprintf("dispatch failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		e.failStep(step.ID, step.MissionID, fmt.Sprintf("remote returned %d", resp.StatusCode))
		return
	}

	e.db.Model(&model.MissionStep{}).Where("id = ?", step.ID).Update("status", "running")
	log.Printf("[squad] step %s dispatched to %s successfully (branch=%s)",
		step.ID, step.TargetNode[:min(16, len(step.TargetNode))], step.Branch)
}

// executeLocal runs a step on the local node (captain handling its own step).
func (e *Engine) executeLocal(step model.MissionStep, contextStr string, mission model.Mission) {
	log.Printf("[squad] executing step %s locally (branch=%s)", step.ID, step.Branch)

	e.db.Model(&model.MissionStep{}).Where("id = ?", step.ID).Update("status", "running")

	// If Git repo exists, clone workspace for this step's branch
	var wsPath string
	if mission.RepoPath != "" && step.Branch != "" {
		var err error
		wsPath, err = e.gitMgr.CloneForStep(mission.RepoPath, mission.ID, step.Branch)
		if err != nil {
			log.Printf("[squad] git clone for step %s failed: %v", step.ID, err)
		}
	}

	// Find local agent
	var agent model.Agent
	if step.TargetAgent != "" {
		e.db.Where("name LIKE ?", "%"+step.TargetAgent+"%").First(&agent)
	}
	if agent.ID == "" {
		e.db.First(&agent)
	}
	if agent.ID == "" {
		e.failStep(step.ID, step.MissionID, "no local agent available")
		return
	}

	// Build goal with Git context
	var goalParts []string
	if contextStr != "" {
		goalParts = append(goalParts, "## 上下文\n"+contextStr)
	}
	if wsPath != "" {
		goalParts = append(goalParts, fmt.Sprintf(`## Git 工作区
- 工作目录: %s
- 分支: %s
- 仓库: %s

请使用 code 工具在工作目录中编写代码，使用 git 工具提交和推送代码。
工作流程: 编写代码 → git add → git commit → git push`, wsPath, step.Branch, mission.RepoPath))
	}
	goalParts = append(goalParts, "## 任务\n"+step.Task)
	goal := strings.Join(goalParts, "\n\n")

	task := model.Task{
		UserID:   agent.UserID,
		AgentID:  agent.ID,
		Title:    "🤝 " + step.Task[:min(100, len(step.Task))],
		Goal:     goal,
		Status:   model.TaskStatusPending,
		Priority: model.TaskPriorityNormal,
	}
	e.db.Create(&task)

	// Poll for task completion (simple approach — future: use event system)
	go func() {
		for i := 0; i < 120; i++ { // up to 10 minutes
			time.Sleep(5 * time.Second)

			var t model.Task
			if err := e.db.Where("id = ?", task.ID).First(&t).Error; err != nil {
				continue
			}

			if t.Status == model.TaskStatusCompleted {
				// Store output on step but don't finalize yet — route through review
				e.db.Model(&model.MissionStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
					"output": t.Result,
				})
				log.Printf("[squad] local step %s execution completed, triggering review", step.ID)

				// Reload step with output
				var updatedStep model.MissionStep
				e.db.Where("id = ?", step.ID).First(&updatedStep)
				e.triggerAutoReview(updatedStep, mission)
				return
			}

			if t.Status == model.TaskStatusFailed {
				e.failStep(step.ID, step.MissionID, t.ErrorMsg)
				return
			}
		}

		e.failStep(step.ID, step.MissionID, "local execution timed out")
	}()
}

// ── LLM Conflict Resolution ──

// llmResolveConflict uses an LLM to resolve a Git merge conflict between ours and theirs.
func (e *Engine) llmResolveConflict(filePath, ours, theirs, base string) (string, error) {
	// Truncate large files
	maxLen := 2000
	if len(ours) > maxLen {
		ours = ours[:maxLen] + "\n...(truncated)"
	}
	if len(theirs) > maxLen {
		theirs = theirs[:maxLen] + "\n...(truncated)"
	}
	if len(base) > maxLen {
		base = base[:maxLen] + "\n...(truncated)"
	}

	prompt := fmt.Sprintf(`你是代码合并专家。请解决以下 Git 合并冲突。

文件: %s

## BASE 版本（共同祖先）
%s

## OURS 版本（当前分支）
%s

## THEIRS 版本（要合并的分支）
%s

请输出合并后的完整文件内容。规则：
1. 保留两边的有用改动，智能合并
2. 如果是不同区域的修改，都保留
3. 如果是同一区域的修改，选择更完整或更新的版本
4. 只输出最终文件内容，不要解释
5. 不要输出 markdown 代码块标记`, filePath, base, ours, theirs)

	var modelCfg model.ModelConfig
	if err := e.db.Where("is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
		return "", fmt.Errorf("no model available for conflict resolution")
	}

	p := provider.CreateFromConfig(e.providerReg, modelCfg)
	if p == nil {
		return "", fmt.Errorf("failed to create provider")
	}

	resp, err := p.ChatSync(context.Background(), &provider.ChatRequest{
		Model:       modelCfg.ModelName,
		Messages:    []provider.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   4000,
	})
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	// Strip markdown code fences if present
	content := resp.Content
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			lines = lines[1 : len(lines)-1]
			if strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
		}
		content = strings.Join(lines, "\n")
	}

	log.Printf("[conflict] LLM resolved conflict in %s (%d bytes)", filePath, len(content))
	return content, nil
}

// ── Helpers ──

// resolveNodeAddress returns the HTTP address for a node ID.
func (e *Engine) resolveNodeAddress(nodeID string) string {
	var peer model.Peer
	if err := e.db.Where("node_id = ?", nodeID).First(&peer).Error; err == nil {
		if peer.Address != "" {
			addr := peer.Address
			if !strings.HasPrefix(addr, "http") {
				addr = "https://" + addr
			}
			return addr
		}
	}
	return ""
}

// failMission marks a mission as failed.
func (e *Engine) failMission(missionID, reason string) {
	log.Printf("[squad] mission %s failed: %s", missionID, reason)
	e.db.Model(&model.Mission{}).Where("id = ?", missionID).Updates(map[string]interface{}{
		"status":       "failed",
		"final_result": "Failed: " + reason,
	})
}

// failStep marks a step as failed and checks mission state.
func (e *Engine) failStep(stepID, missionID, reason string) {
	log.Printf("[squad] step %s failed: %s", stepID, reason)
	now := time.Now()
	e.db.Model(&model.MissionStep{}).Where("id = ?", stepID).Updates(map[string]interface{}{
		"status":       "failed",
		"error_msg":    reason,
		"completed_at": &now,
	})
}
