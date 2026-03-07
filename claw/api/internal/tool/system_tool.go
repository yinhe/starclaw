package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"gorm.io/gorm"
)

// DelegateResult holds the result of delegating a task to another agent
type DelegateResult struct {
	Content string
	Error   string
}

// DelegateFunc is a callback that runs a sub-agent and returns its response.
// This avoids circular imports between tool and agent packages.
type DelegateFunc func(ctx context.Context, agent model.Agent, modelCfg model.ModelConfig, message string) (*DelegateResult, error)

// SystemTool provides system management actions: create agents, workflows, schedules, delegate to agents, etc.
type SystemTool struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	delegateFunc     DelegateFunc
}

func NewSystemTool(db *gorm.DB, pr *provider.Registry, df DelegateFunc) *SystemTool {
	return &SystemTool{db: db, providerRegistry: pr, delegateFunc: df}
}

func (t *SystemTool) Name() string { return "system" }

func (t *SystemTool) Description() string {
	return "系统管理：创建查看 Agent、创建工作流、调度任务、管理模型、发送通知。"
}

func (t *SystemTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action to perform",
				Enum:        []string{"create_agent", "list_agents", "list_models", "create_workflow", "schedule_task", "list_schedules", "delegate_to_agent", "create_task", "update_task", "list_tasks", "notify_user"},
			},
			"name":           {Type: "string", Description: "Name for agent/workflow/schedule"},
			"description":    {Type: "string", Description: "Description text"},
			"system_prompt":  {Type: "string", Description: "System prompt for agent creation"},
			"model_id":       {Type: "string", Description: "Model ID for agent/workflow"},
			"tools":          {Type: "string", Description: "JSON array of tool names for agent, e.g. [\"code\",\"web_search\"]"},
			"definition":     {Type: "string", Description: "Workflow JSON definition with nodes and edges (advanced, usually auto-generated)"},
			"steps":          {Type: "string", Description: "For create_workflow: JSON array of workflow steps, e.g. [{\"name\":\"编写脚本\",\"type\":\"llm\",\"description\":\"生成分场景脚本\"},{\"name\":\"生成视频\",\"type\":\"tool\",\"description\":\"逐场景调用视频生成\"}]. Types: llm, tool, condition. Auto-generates visual workflow nodes."},
			"workflow_id":    {Type: "string", Description: "Workflow ID for scheduling"},
			"cron_expr":      {Type: "string", Description: "Cron expression for scheduling, e.g. '0 9 * * *' for daily 9am"},
			"user_id":        {Type: "string", Description: "User ID (auto-filled from context if empty)"},
			"agent_id":       {Type: "string", Description: "Target agent ID (for delegate_to_agent, create_task)"},
			"message":        {Type: "string", Description: "Message/task to send to the target agent (for delegate_to_agent, notify_user content)"},
			"title":          {Type: "string", Description: "Title for task or notification"},
			"goal":           {Type: "string", Description: "Goal/instructions for create_task - what the agent should do"},
			"priority":       {Type: "string", Description: "Task priority: urgent, high, normal, low"},
			"task_id":        {Type: "string", Description: "Task ID (for update_task)"},
			"progress":       {Type: "string", Description: "Progress 0-100 (for update_task)"},
			"progress_note":  {Type: "string", Description: "Progress note (for update_task)"},
			"scheduled_at":   {Type: "string", Description: "ISO8601 datetime for delayed task execution, e.g. 2026-03-03T09:00:00Z"},
			"parent_task_id": {Type: "string", Description: "Parent task ID for sub-tasks"},
		},
		Required: []string{"action"},
	}
}

type systemArgs struct {
	Action         string `json:"action"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	SystemPrompt   string `json:"system_prompt"`
	ModelID        string `json:"model_id"`
	Tools          string `json:"tools"`
	Definition     string `json:"definition"`
	WorkflowID     string `json:"workflow_id"`
	CronExpr       string `json:"cron_expr"`
	UserID         string `json:"user_id"`
	AgentID        string `json:"agent_id"`
	Message        string `json:"message"`
	Title          string `json:"title"`
	Goal           string `json:"goal"`
	Priority       string `json:"priority"`
	TaskID         string `json:"task_id"`
	Progress       string `json:"progress"`
	ProgressNote   string `json:"progress_note"`
	ScheduledAt    string `json:"scheduled_at"`
	ParentTaskID   string `json:"parent_task_id"`
	ConversationID string `json:"conversation_id"`
	Steps          string `json:"steps"`
}

func (t *SystemTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args systemArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	// Auto-inject user_id and conversation_id from context (set by chat handler)
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok && uid != "" {
		args.UserID = uid
	}
	if cid, ok := ctx.Value(CtxKeyConversationID).(string); ok && cid != "" {
		args.ConversationID = cid
	}

	switch args.Action {
	case "create_agent":
		return t.createAgent(args)
	case "list_agents":
		return t.listAgents(args)
	case "list_models":
		return t.listModels()
	case "create_workflow":
		return t.createWorkflow(args)
	case "schedule_task":
		return t.scheduleTask(args)
	case "list_schedules":
		return t.listSchedules(args)
	case "delegate_to_agent":
		return t.delegateToAgent(ctx, args)
	case "create_task":
		return t.createTask(args)
	case "update_task":
		return t.updateTask(args)
	case "list_tasks":
		return t.listTasks(args)
	case "notify_user":
		return t.notifyUser(args)
	default:
		return "", fmt.Errorf("unknown action: %s", args.Action)
	}
}

func (t *SystemTool) createAgent(args systemArgs) (string, error) {
	if args.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if args.UserID == "" {
		args.UserID = "system"
	}

	// Dedup: reuse existing agent with same name for this user
	var existing model.Agent
	if err := t.db.Where("name = ? AND user_id = ?", args.Name, args.UserID).First(&existing).Error; err == nil {
		// Update fields if provided
		updates := map[string]interface{}{}
		if args.Description != "" {
			updates["description"] = args.Description
		}
		if args.SystemPrompt != "" {
			updates["system_prompt"] = args.SystemPrompt
		}
		if args.ModelID != "" {
			updates["model_id"] = args.ModelID
		}
		if args.Tools != "" {
			updates["tools"] = sanitizeToolsJSON(args.Tools)
		}
		if len(updates) > 0 {
			t.db.Model(&existing).Updates(updates)
		}

		result, _ := json.Marshal(map[string]interface{}{
			"status":  "success",
			"action":  "create_agent",
			"reused":  true,
			"message": "Agent with this name already exists, reusing",
			"agent": map[string]string{
				"id":          existing.ID,
				"name":        existing.Name,
				"description": existing.Description,
				"model_id":    existing.ModelID,
				"tools":       existing.Tools,
			},
		})
		return string(result), nil
	}

	tools := "[]"
	if args.Tools != "" {
		tools = sanitizeToolsJSON(args.Tools)
	}

	agent := model.Agent{
		ID:           uuid.New().String(),
		UserID:       args.UserID,
		Name:         args.Name,
		Description:  args.Description,
		SystemPrompt: args.SystemPrompt,
		ModelID:      args.ModelID,
		Tools:        tools,
		Config:       `{"temperature":0.7,"max_tokens":4096}`,
	}

	if err := t.db.Create(&agent).Error; err != nil {
		return "", fmt.Errorf("failed to create agent: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"action": "create_agent",
		"agent": map[string]string{
			"id":          agent.ID,
			"name":        agent.Name,
			"description": agent.Description,
			"model_id":    agent.ModelID,
			"tools":       agent.Tools,
		},
	})
	return string(result), nil
}

func (t *SystemTool) listAgents(args systemArgs) (string, error) {
	var agents []model.Agent
	q := t.db.Select("id, name, description, tools, model_id, is_public, created_at").Order("created_at DESC").Limit(20)
	if args.UserID != "" {
		q = q.Where("user_id = ?", args.UserID)
	}
	if err := q.Find(&agents).Error; err != nil {
		return "", fmt.Errorf("failed to list agents: %v", err)
	}

	type agentSummary struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Tools       string `json:"tools"`
		ModelID     string `json:"model_id"`
		IsPublic    bool   `json:"is_public"`
	}
	var summaries []agentSummary
	for _, a := range agents {
		summaries = append(summaries, agentSummary{
			ID: a.ID, Name: a.Name, Description: a.Description,
			Tools: a.Tools, ModelID: a.ModelID, IsPublic: a.IsPublic,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"action": "list_agents",
		"count":  len(summaries),
		"agents": summaries,
	})
	return string(result), nil
}

func (t *SystemTool) listModels() (string, error) {
	var models []model.ModelConfig
	if err := t.db.Select("id, provider, model_name, display_name").Find(&models).Error; err != nil {
		return "", fmt.Errorf("failed to list models: %v", err)
	}

	type modelSummary struct {
		ID          string `json:"id"`
		Provider    string `json:"provider"`
		ModelName   string `json:"model_name"`
		DisplayName string `json:"display_name"`
	}
	var summaries []modelSummary
	for _, m := range models {
		summaries = append(summaries, modelSummary{
			ID: m.ID, Provider: m.Provider, ModelName: m.ModelName, DisplayName: m.DisplayName,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"action": "list_models",
		"count":  len(summaries),
		"models": summaries,
	})
	return string(result), nil
}

// workflowStep represents a simple step definition for auto-generating workflows
type workflowStep struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // llm, tool, condition
	Description string `json:"description"`
	Tool        string `json:"tool,omitempty"`  // tool name for tool nodes
	Model       string `json:"model,omitempty"` // model for llm nodes
}

func (t *SystemTool) createWorkflow(args systemArgs) (string, error) {
	if args.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if args.UserID == "" {
		args.UserID = "system"
	}

	definition := args.Definition

	// Auto-generate definition from steps if provided
	if definition == "" && args.Steps != "" {
		var steps []workflowStep
		if err := json.Unmarshal([]byte(args.Steps), &steps); err != nil {
			return "", fmt.Errorf("invalid steps JSON: %v", err)
		}
		definition = generateWorkflowDefinition(steps)
	}
	if definition == "" {
		definition = `{"nodes":[],"edges":[]}`
	}

	wf := model.Workflow{
		ID:             uuid.New().String(),
		UserID:         args.UserID,
		ConversationID: args.ConversationID,
		Name:           args.Name,
		Description:    args.Description,
		Definition:     definition,
	}

	if err := t.db.Create(&wf).Error; err != nil {
		return "", fmt.Errorf("failed to create workflow: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":   "success",
		"action":   "create_workflow",
		"workflow": map[string]string{"id": wf.ID, "name": wf.Name},
		"message":  fmt.Sprintf("工作流 '%s' 已创建，包含 %d 个步骤。可在「工作流」页面查看和编辑。", wf.Name, countSteps(args.Steps)),
	})
	return string(result), nil
}

// generateWorkflowDefinition creates a React Flow compatible definition from simple steps
func generateWorkflowDefinition(steps []workflowStep) string {
	type nodeDef struct {
		ID       string                 `json:"id"`
		Type     string                 `json:"type"`
		Position map[string]float64     `json:"position"`
		Data     map[string]interface{} `json:"data"`
	}
	type edgeDef struct {
		ID     string `json:"id"`
		Source string `json:"source"`
		Target string `json:"target"`
	}

	var nodes []nodeDef
	var edges []edgeDef

	// Start node
	startID := "start-1"
	nodes = append(nodes, nodeDef{
		ID:       startID,
		Type:     "start",
		Position: map[string]float64{"x": 300, "y": 50},
		Data:     map[string]interface{}{"label": "开始"},
	})

	// Step nodes
	prevID := startID
	yPos := 150.0
	for i, step := range steps {
		nodeID := fmt.Sprintf("step-%d", i+1)
		nodeType := step.Type
		if nodeType == "" {
			nodeType = "tool"
		}
		// Validate type
		switch nodeType {
		case "llm", "tool", "condition":
		default:
			nodeType = "tool"
		}

		data := map[string]interface{}{
			"label":       step.Name,
			"description": step.Description,
		}
		if step.Tool != "" {
			data["toolName"] = step.Tool
		}
		if step.Model != "" {
			data["model"] = step.Model
		}

		nodes = append(nodes, nodeDef{
			ID:       nodeID,
			Type:     nodeType,
			Position: map[string]float64{"x": 300, "y": yPos},
			Data:     data,
		})

		edges = append(edges, edgeDef{
			ID:     fmt.Sprintf("edge-%s-%s", prevID, nodeID),
			Source: prevID,
			Target: nodeID,
		})

		prevID = nodeID
		yPos += 120
	}

	// End node
	endID := "end-1"
	nodes = append(nodes, nodeDef{
		ID:       endID,
		Type:     "end",
		Position: map[string]float64{"x": 300, "y": yPos},
		Data:     map[string]interface{}{"label": "结束"},
	})
	edges = append(edges, edgeDef{
		ID:     fmt.Sprintf("edge-%s-%s", prevID, endID),
		Source: prevID,
		Target: endID,
	})

	def := map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
	}
	result, _ := json.Marshal(def)
	return string(result)
}

// countSteps counts the number of steps in a JSON steps string
func countSteps(stepsJSON string) int {
	if stepsJSON == "" {
		return 0
	}
	var steps []workflowStep
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		return 0
	}
	return len(steps)
}

func (t *SystemTool) scheduleTask(args systemArgs) (string, error) {
	if args.CronExpr == "" {
		return "", fmt.Errorf("cron_expr is required")
	}
	if args.Goal == "" && args.Title == "" && args.Name == "" {
		return "", fmt.Errorf("goal or title is required")
	}
	if args.UserID == "" {
		args.UserID = "system"
	}

	title := args.Title
	if title == "" {
		title = args.Name
	}
	if title == "" {
		title = truncateStr(args.Goal, 100)
	}
	goal := args.Goal
	if goal == "" {
		goal = title
	}

	schedule := model.Schedule{
		ID:             uuid.New().String(),
		UserID:         args.UserID,
		WorkflowID:     args.WorkflowID,
		AgentID:        args.AgentID,
		ConversationID: args.ConversationID,
		Title:          title,
		Goal:           goal,
		Description:    args.Description,
		CronExpr:       args.CronExpr,
		Input:          title,
		Enabled:        true,
		MaxInstances:   1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := t.db.Create(&schedule).Error; err != nil {
		return "", fmt.Errorf("failed to create schedule: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"action": "schedule_task",
		"schedule": map[string]string{
			"id":       schedule.ID,
			"title":    title,
			"cron":     schedule.CronExpr,
			"agent_id": schedule.AgentID,
			"goal":     truncateStr(goal, 200),
		},
		"message": "定时任务已创建，将按 cron 表达式周期性执行。",
	})
	return string(result), nil
}

func (t *SystemTool) listSchedules(args systemArgs) (string, error) {
	var schedules []model.Schedule
	q := t.db.Select("id, input, cron_expr, workflow_id, enabled, created_at").Order("created_at DESC").Limit(20)
	if args.UserID != "" {
		q = q.Where("user_id = ?", args.UserID)
	}
	if err := q.Find(&schedules).Error; err != nil {
		return "", fmt.Errorf("failed to list schedules: %v", err)
	}

	type scheduleSummary struct {
		ID         string `json:"id"`
		Label      string `json:"label"`
		CronExpr   string `json:"cron_expr"`
		WorkflowID string `json:"workflow_id"`
		Enabled    bool   `json:"enabled"`
	}
	var summaries []scheduleSummary
	for _, s := range schedules {
		summaries = append(summaries, scheduleSummary{
			ID: s.ID, Label: s.Input, CronExpr: s.CronExpr, WorkflowID: s.WorkflowID, Enabled: s.Enabled,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":    "success",
		"action":    "list_schedules",
		"count":     len(summaries),
		"schedules": summaries,
	})
	return string(result), nil
}

func (t *SystemTool) delegateToAgent(ctx context.Context, args systemArgs) (string, error) {
	if args.AgentID == "" {
		return "", fmt.Errorf("agent_id is required")
	}
	if args.Message == "" {
		return "", fmt.Errorf("message is required")
	}
	if t.delegateFunc == nil {
		return "", fmt.Errorf("agent delegation not configured")
	}

	// Look up the target agent
	var agent model.Agent
	if err := t.db.Where("id = ?", args.AgentID).First(&agent).Error; err != nil {
		return "", fmt.Errorf("agent not found: %s", args.AgentID)
	}

	// Look up the model config
	var modelCfg model.ModelConfig
	if err := t.db.Where("id = ?", agent.ModelID).First(&modelCfg).Error; err != nil {
		return "", fmt.Errorf("model config not found for agent %s", agent.Name)
	}

	log.Printf("[SystemTool] Delegating to agent %s (%s): %s", agent.Name, agent.ID, truncateStr(args.Message, 100))

	result, err := t.delegateFunc(ctx, agent, modelCfg, args.Message)
	if err != nil {
		return toJSON(map[string]interface{}{
			"action":     "delegate_to_agent",
			"status":     "error",
			"agent_id":   args.AgentID,
			"agent_name": agent.Name,
			"error":      err.Error(),
		}), nil
	}

	if result.Error != "" {
		return toJSON(map[string]interface{}{
			"action":     "delegate_to_agent",
			"status":     "error",
			"agent_id":   args.AgentID,
			"agent_name": agent.Name,
			"error":      result.Error,
		}), nil
	}

	log.Printf("[SystemTool] Agent %s responded: %d chars", agent.Name, len(result.Content))

	return toJSON(map[string]interface{}{
		"action":     "delegate_to_agent",
		"status":     "success",
		"agent_id":   args.AgentID,
		"agent_name": agent.Name,
		"response":   truncateStr(result.Content, 8000),
	}), nil
}

func (t *SystemTool) createTask(args systemArgs) (string, error) {
	if args.Title == "" && args.Goal == "" {
		return "", fmt.Errorf("title or goal is required")
	}
	if args.Goal == "" {
		args.Goal = args.Title
	}
	if args.Title == "" {
		args.Title = truncateStr(args.Goal, 100)
	}
	if args.UserID == "" {
		args.UserID = "system"
	}

	task := model.Task{
		UserID:         args.UserID,
		AgentID:        args.AgentID,
		ConversationID: args.ConversationID,
		ParentTaskID:   args.ParentTaskID,
		Title:          args.Title,
		Description:    args.Description,
		Goal:           args.Goal,
		Status:         model.TaskStatusPending,
		Priority:       model.TaskPriority(args.Priority),
	}
	if task.Priority == "" {
		task.Priority = model.TaskPriorityNormal
	}

	// Handle scheduled execution
	if args.ScheduledAt != "" {
		if scheduled, err := time.Parse(time.RFC3339, args.ScheduledAt); err == nil {
			task.ScheduledAt = &scheduled
			task.Status = model.TaskStatusWaiting
		}
	}

	if err := t.db.Create(&task).Error; err != nil {
		return "", fmt.Errorf("failed to create task: %v", err)
	}

	return toJSON(map[string]interface{}{
		"action":  "create_task",
		"status":  "success",
		"task_id": task.ID,
		"title":   task.Title,
		"state":   string(task.Status),
		"message": "Task created and queued for background execution",
	}), nil
}

func (t *SystemTool) updateTask(args systemArgs) (string, error) {
	if args.TaskID == "" {
		return "", fmt.Errorf("task_id is required")
	}

	updates := map[string]interface{}{"updated_at": time.Now()}

	if args.Progress != "" {
		var p int
		fmt.Sscanf(args.Progress, "%d", &p)
		if p >= 0 && p <= 100 {
			updates["progress"] = p
		}
	}
	if args.ProgressNote != "" {
		updates["progress_note"] = args.ProgressNote
	}

	if err := t.db.Model(&model.Task{}).Where("id = ?", args.TaskID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("failed to update task: %v", err)
	}

	return toJSON(map[string]interface{}{
		"action":  "update_task",
		"status":  "success",
		"task_id": args.TaskID,
	}), nil
}

func (t *SystemTool) listTasks(args systemArgs) (string, error) {
	var tasks []model.Task
	q := t.db.Select("id, title, status, priority, progress, progress_note, agent_id, parent_task_id, created_at, completed_at").
		Order("created_at DESC").Limit(20)
	if args.UserID != "" {
		q = q.Where("user_id = ?", args.UserID)
	}
	if err := q.Find(&tasks).Error; err != nil {
		return "", fmt.Errorf("failed to list tasks: %v", err)
	}

	type taskSummary struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Status       string `json:"status"`
		Priority     string `json:"priority"`
		Progress     int    `json:"progress"`
		ProgressNote string `json:"progress_note"`
		AgentID      string `json:"agent_id"`
		ParentTaskID string `json:"parent_task_id"`
	}
	var summaries []taskSummary
	for _, tk := range tasks {
		summaries = append(summaries, taskSummary{
			ID: tk.ID, Title: tk.Title, Status: string(tk.Status),
			Priority: string(tk.Priority), Progress: tk.Progress,
			ProgressNote: tk.ProgressNote, AgentID: tk.AgentID,
			ParentTaskID: tk.ParentTaskID,
		})
	}

	return toJSON(map[string]interface{}{
		"action": "list_tasks",
		"status": "success",
		"count":  len(summaries),
		"tasks":  summaries,
	}), nil
}

func (t *SystemTool) notifyUser(args systemArgs) (string, error) {
	if args.Title == "" && args.Message == "" {
		return "", fmt.Errorf("title or message is required")
	}
	if args.Title == "" {
		args.Title = truncateStr(args.Message, 100)
	}
	if args.UserID == "" {
		args.UserID = "system"
	}

	notifType := model.NotifyInfo
	if args.Priority == "urgent" || args.Priority == "high" {
		notifType = model.NotifyWarning
	}

	notif := model.Notification{
		UserID:  args.UserID,
		TaskID:  args.TaskID,
		Type:    notifType,
		Title:   args.Title,
		Content: args.Message,
	}
	if err := t.db.Create(&notif).Error; err != nil {
		return "", fmt.Errorf("failed to create notification: %v", err)
	}

	return toJSON(map[string]interface{}{
		"action":          "notify_user",
		"status":          "success",
		"notification_id": notif.ID,
		"message":         "Notification sent to user",
	}), nil
}

// sanitizeToolsJSON ensures the tools value is valid JSON for MySQL JSON column.
// If input is already valid JSON array, return as-is. Otherwise try to parse as
// comma-separated tool names and convert to JSON array. Falls back to "[]".
func sanitizeToolsJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "[]"
	}
	// Already valid JSON array?
	var arr []interface{}
	if json.Unmarshal([]byte(s), &arr) == nil {
		return s
	}
	// Already valid JSON string?
	var obj interface{}
	if json.Unmarshal([]byte(s), &obj) == nil {
		return s
	}
	// Try comma/space separated tool names ↀJSON array
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
	var names []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			names = append(names, p)
		}
	}
	if len(names) > 0 {
		b, _ := json.Marshal(names)
		return string(b)
	}
	return "[]"
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "... [truncated]"
	}
	return s
}
