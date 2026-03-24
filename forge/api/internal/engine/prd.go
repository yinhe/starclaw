package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"starclaw.net/forge/internal/config"
	"starclaw.net/forge/internal/model"
)

// PRDEngine generates and manages PRDs using LLM.
type PRDEngine struct {
	DB     *gorm.DB
	Cfg    *config.Config
	client *http.Client // shared, keep-alive enabled
}

// NewPRDEngine creates a PRDEngine with a persistent HTTP client.
func NewPRDEngine(db *gorm.DB, cfg *config.Config) *PRDEngine {
	return &PRDEngine{
		DB:  db,
		Cfg: cfg,
		client: &http.Client{
			Timeout: 300 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     600 * time.Second,
			},
		},
	}
}

// GeneratePRD calls LLM to produce a structured PRD from a natural language prompt.
func (e *PRDEngine) GeneratePRD(projectID, prompt string) (*model.ForgePRD, error) {
	systemPrompt := `你是 StarClaw Forge 的产品经理 AI。根据用户的需求描述，生成结构化的 PRD（产品需求文档）。

输出严格 JSON 格式:
{
  "title": "需求标题",
  "objective": "一句话目标",
  "features": [
    {"id": "F1", "title": "功能名", "desc": "描述", "service": "claw/api"}
  ],
  "non_functional": ["非功能需求1", "非功能需求2"],
  "acceptance_criteria": ["验收标准1", "验收标准2"],
  "services": ["claw/api", "claw/web"],
  "estimated_sprints": 2
}

注意:
- service 必须是 StarClaw monorepo 中的目录: claw/api, claw/web, queen/api, queen/swarm, queen/arena, synapse/api, synapse/core, overlord/api, overlord/web, hive/api, nydus/api, spore, carapace, forge/api, forge/web
- 合理估算 Sprint 数量 (每个 Sprint 3-5天)
- 每个 feature 标注涉及的 service`

	rawResp, err := e.callLLM(systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse LLM response — extract JSON
	jsonStr := extractJSON(rawResp)

	prd := &model.ForgePRD{
		ProjectID:      projectID,
		Prompt:         prompt,
		Status:         "draft",
		RawLLMResponse: rawResp,
	}

	// Try to parse structured fields
	var parsed struct {
		Title              string        `json:"title"`
		Objective          string        `json:"objective"`
		Features           []interface{} `json:"features"`
		NonFunctional      []string      `json:"non_functional"`
		AcceptanceCriteria []string      `json:"acceptance_criteria"`
		Services           []string      `json:"services"`
		EstimatedSprints   int           `json:"estimated_sprints"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
		prd.Title = parsed.Title
		prd.Objective = parsed.Objective
		prd.EstimatedSprints = parsed.EstimatedSprints
		if b, err := json.Marshal(parsed.Features); err == nil {
			prd.Features = string(b)
		}
		if b, err := json.Marshal(parsed.NonFunctional); err == nil {
			prd.NonFunctional = string(b)
		}
		if b, err := json.Marshal(parsed.AcceptanceCriteria); err == nil {
			prd.AcceptanceCriteria = string(b)
		}
		if b, err := json.Marshal(parsed.Services); err == nil {
			prd.Services = string(b)
		}
	}

	e.DB.Create(prd)
	return prd, nil
}

// PlanSprints generates Sprint + Issue breakdown from a confirmed PRD.
func (e *PRDEngine) PlanSprints(prd *model.ForgePRD) ([]model.ForgeSprint, []model.ForgeIssue, error) {
	planPrompt := fmt.Sprintf(`基于以下 PRD，拆分为具体的 Sprint 和 Issue。

PRD:
标题: %s
目标: %s
功能: %s
预估 Sprint 数: %d

输出严格 JSON 格式:
{
  "sprints": [
    {
      "name": "Sprint 名称",
      "goal": "Sprint 目标",
      "issues": [
        {
          "title": "Issue 标题",
          "body": "详细描述",
          "type": "task",
          "priority": "high",
          "service": "claw/api",
          "task_type": "code",
          "story_points": 3,
          "depends_on_indices": []
        }
      ]
    }
  ]
}

注意:
- depends_on_indices 是同一 Sprint 内的 Issue 索引 (0-based)，表示依赖关系
- task_type: code (代码任务→Windsurf) / agent (Agent开发→DevClaw) / config / doc / design / review
- story_points: 1-5 (1=很小, 3=中等, 5=很大)
- 合理安排并行和依赖`, prd.Title, prd.Objective, prd.Features, prd.EstimatedSprints)

	rawResp, err := e.callLLM("你是 StarClaw Forge 的 Sprint 规划 AI。将 PRD 拆分为可执行的 Sprint 和 Issue。输出严格 JSON。", planPrompt)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM plan failed: %w", err)
	}

	jsonStr := extractJSON(rawResp)

	var parsed struct {
		Sprints []struct {
			Name   string `json:"name"`
			Goal   string `json:"goal"`
			Issues []struct {
				Title            string `json:"title"`
				Body             string `json:"body"`
				Type             string `json:"type"`
				Priority         string `json:"priority"`
				Service          string `json:"service"`
				TaskType         string `json:"task_type"`
				StoryPoints      int    `json:"story_points"`
				DependsOnIndices []int  `json:"depends_on_indices"`
			} `json:"issues"`
		} `json:"sprints"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, nil, fmt.Errorf("parse plan JSON: %w (raw: %s)", err, jsonStr[:min(len(jsonStr), 500)])
	}

	// Get project for key prefix
	var project model.ForgeProject
	e.DB.First(&project, "id = ?", prd.ProjectID)

	var sprints []model.ForgeSprint
	var allIssues []model.ForgeIssue

	for si, sp := range parsed.Sprints {
		sprint := model.ForgeSprint{
			ProjectID: prd.ProjectID,
			PRDID:     prd.ID,
			Name:      sp.Name,
			Goal:      sp.Goal,
			Status:    "planned",
			SeqNum:    si + 1,
		}
		e.DB.Create(&sprint)
		sprints = append(sprints, sprint)

		// Track issue IDs within sprint for dependency resolution
		sprintIssueIDs := make([]string, len(sp.Issues))

		for ii, iss := range sp.Issues {
			// Auto-increment issue number
			e.DB.Model(&project).Update("issue_seq", gorm.Expr("issue_seq + 1"))
			e.DB.First(&project, "id = ?", prd.ProjectID)
			num := project.IssueSeq

			// Build depends_on JSON
			var deps []string
			for _, di := range iss.DependsOnIndices {
				if di >= 0 && di < len(sprintIssueIDs) && sprintIssueIDs[di] != "" {
					deps = append(deps, sprintIssueIDs[di])
				}
			}
			depsJSON := "[]"
			if len(deps) > 0 {
				b, _ := json.Marshal(deps)
				depsJSON = string(b)
			}

			issue := model.ForgeIssue{
				ProjectID:   prd.ProjectID,
				Number:      num,
				Key:         fmt.Sprintf("%s-%d", project.Key, num),
				Title:       iss.Title,
				Body:        iss.Body,
				Type:        orDefault(iss.Type, "task"),
				Priority:    orDefault(iss.Priority, "medium"),
				Status:      "backlog",
				Service:     iss.Service,
				TaskType:    orDefault(iss.TaskType, "code"),
				SprintID:    sprint.ID,
				PRDID:       prd.ID,
				StoryPoints: iss.StoryPoints,
				DependsOn:   depsJSON,
			}
			e.DB.Create(&issue)
			sprintIssueIDs[ii] = issue.ID
			allIssues = append(allIssues, issue)
		}
	}

	// Update PRD status
	e.DB.Model(prd).Update("status", "planned")

	return sprints, allIssues, nil
}

// callLLM sends a chat completion request to the configured LLM.
func (e *PRDEngine) callLLM(systemPrompt, userMessage string) (string, error) {
	if e.Cfg.LLMAPIKey == "" {
		return "", fmt.Errorf("FORGE_LLM_API_KEY not configured")
	}

	reqBody := map[string]interface{}{
		"model": e.Cfg.LLMModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"temperature": 0.3,
		"max_tokens":  4096,
		"keep_alive":  "30m",
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", e.Cfg.LLMBaseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.Cfg.LLMAPIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("LLM API %d: %s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(body, &result)
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}
	return result.Choices[0].Message.Content, nil
}

// StreamChunk represents a piece of streaming LLM output.
type StreamChunk struct {
	Type string `json:"type"` // "thinking", "content", "done", "error"
	Text string `json:"text"`
}

// CallLLMStream sends a streaming chat completion request, calling onChunk for each piece.
// Returns the full accumulated content (non-thinking) text.
func (e *PRDEngine) CallLLMStream(systemPrompt, userMessage string, onChunk func(StreamChunk)) (string, error) {
	if e.Cfg.LLMAPIKey == "" {
		return "", fmt.Errorf("FORGE_LLM_API_KEY not configured")
	}

	reqBody := map[string]interface{}{
		"model": e.Cfg.LLMModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"temperature": 0.3,
		"max_tokens":  8192,
		"stream":      true,
		"keep_alive":  "30m",
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", e.Cfg.LLMBaseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.Cfg.LLMAPIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API %d: %s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}

	var rawAll strings.Builder // accumulate ALL raw content
	var inThinkTag bool
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// Method 1: explicit reasoning_content field (newer Ollama / OpenAI)
		if delta.ReasoningContent != "" {
			onChunk(StreamChunk{Type: "thinking", Text: delta.ReasoningContent})
		}

		// Method 2: detect <think>...</think> tags in content (qwen3 via Ollama)
		// Simple approach: no cross-chunk buffering, just track state and emit directly
		if delta.Content != "" {
			rawAll.WriteString(delta.Content)
			text := delta.Content

			// Handle tag transitions within this single chunk
			for len(text) > 0 {
				if inThinkTag {
					if idx := strings.Index(text, "</think>"); idx >= 0 {
						if idx > 0 {
							onChunk(StreamChunk{Type: "thinking", Text: text[:idx]})
						}
						text = text[idx+8:]
						inThinkTag = false
					} else {
						// Entire chunk is thinking content — emit as-is (no byte slicing)
						onChunk(StreamChunk{Type: "thinking", Text: text})
						text = ""
					}
				} else {
					if idx := strings.Index(text, "<think>"); idx >= 0 {
						if idx > 0 {
							onChunk(StreamChunk{Type: "content", Text: text[:idx]})
						}
						text = text[idx+7:]
						inThinkTag = true
					} else {
						// Entire chunk is regular content — emit as-is (no byte slicing)
						onChunk(StreamChunk{Type: "content", Text: text})
						text = ""
					}
				}
			}
		}
	}

	onChunk(StreamChunk{Type: "done", Text: ""})

	// Final extraction: strip <think> blocks from complete raw text
	clean := stripThinkTags(rawAll.String())
	return strings.TrimSpace(clean), nil
}

// GetGenerateSystemPrompt returns the system prompt for PRD generation.
func (e *PRDEngine) GetGenerateSystemPrompt() string {
	return `你是 StarClaw Forge 的产品经理 AI。根据用户的需求描述，生成结构化的 PRD（产品需求文档）。

输出严格 JSON 格式:
{
  "title": "需求标题",
  "objective": "一句话目标",
  "features": [
    {"id": "F1", "title": "功能名", "desc": "描述", "service": "claw/api"}
  ],
  "non_functional": ["非功能需求1", "非功能需求2"],
  "acceptance_criteria": ["验收标准1", "验收标准2"],
  "services": ["claw/api", "claw/web"],
  "estimated_sprints": 2
}

注意:
- service 必须是 StarClaw monorepo 中的目录: claw/api, claw/web, queen/api, queen/swarm, queen/arena, synapse/api, synapse/core, overlord/api, overlord/web, hive/api, nydus/api, spore, carapace, forge/api, forge/web
- 合理估算 Sprint 数量 (每个 Sprint 3-5天)
- 每个 feature 标注涉及的 service`
}

// GetPlanSystemPrompt returns the system prompt for sprint planning.
func (e *PRDEngine) GetPlanSystemPrompt() string {
	return "你是 StarClaw Forge 的 Sprint 规划 AI。将 PRD 拆分为可执行的 Sprint 和 Issue。输出严格 JSON。"
}

// GetPlanUserPrompt returns the user prompt for sprint planning from a PRD.
func (e *PRDEngine) GetPlanUserPrompt(prd *model.ForgePRD) string {
	return fmt.Sprintf(`基于以下 PRD，拆分为具体的 Sprint 和 Issue。

PRD:
标题: %s
目标: %s
功能: %s
预估 Sprint 数: %d

输出严格 JSON 格式:
{
  "sprints": [
    {
      "name": "Sprint 名称",
      "goal": "Sprint 目标",
      "issues": [
        {
          "title": "Issue 标题",
          "body": "详细描述",
          "type": "task",
          "priority": "high",
          "service": "claw/api",
          "task_type": "code",
          "story_points": 3,
          "depends_on_indices": []
        }
      ]
    }
  ]
}

注意:
- depends_on_indices 是同一 Sprint 内的 Issue 索引 (0-based)，表示依赖关系
- task_type: code (代码任务→Windsurf) / agent (Agent开发→DevClaw) / config / doc / design / review
- story_points: 1-5 (1=很小, 3=中等, 5=很大)
- 合理安排并行和依赖`, prd.Title, prd.Objective, prd.Features, prd.EstimatedSprints)
}

// SaveGeneratedPRD parses raw LLM output and saves it as a PRD.
func (e *PRDEngine) SaveGeneratedPRD(projectID, prompt, rawContent string) (*model.ForgePRD, error) {
	jsonStr := extractJSON(rawContent)
	prd := &model.ForgePRD{
		ProjectID:      projectID,
		Prompt:         prompt,
		Status:         "draft",
		RawLLMResponse: rawContent,
	}
	var parsed struct {
		Title              string        `json:"title"`
		Objective          string        `json:"objective"`
		Features           []interface{} `json:"features"`
		NonFunctional      []string      `json:"non_functional"`
		AcceptanceCriteria []string      `json:"acceptance_criteria"`
		Services           []string      `json:"services"`
		EstimatedSprints   int           `json:"estimated_sprints"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
		prd.Title = parsed.Title
		prd.Objective = parsed.Objective
		prd.EstimatedSprints = parsed.EstimatedSprints
		if b, err := json.Marshal(parsed.Features); err == nil {
			prd.Features = string(b)
		}
		if b, err := json.Marshal(parsed.NonFunctional); err == nil {
			prd.NonFunctional = string(b)
		}
		if b, err := json.Marshal(parsed.AcceptanceCriteria); err == nil {
			prd.AcceptanceCriteria = string(b)
		}
		if b, err := json.Marshal(parsed.Services); err == nil {
			prd.Services = string(b)
		}
	}
	e.DB.Create(prd)
	return prd, nil
}

// SavePlanResult parses raw LLM plan output and saves sprints + issues to DB.
func (e *PRDEngine) SavePlanResult(prd *model.ForgePRD, rawContent string) ([]model.ForgeSprint, []model.ForgeIssue, error) {
	jsonStr := extractJSON(rawContent)

	var parsed struct {
		Sprints []struct {
			Name   string `json:"name"`
			Goal   string `json:"goal"`
			Issues []struct {
				Title            string `json:"title"`
				Body             string `json:"body"`
				Type             string `json:"type"`
				Priority         string `json:"priority"`
				Service          string `json:"service"`
				TaskType         string `json:"task_type"`
				StoryPoints      int    `json:"story_points"`
				DependsOnIndices []int  `json:"depends_on_indices"`
			} `json:"issues"`
		} `json:"sprints"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, nil, fmt.Errorf("parse plan JSON: %w (raw: %s)", err, jsonStr[:min(len(jsonStr), 500)])
	}

	var project model.ForgeProject
	e.DB.First(&project, "id = ?", prd.ProjectID)

	var sprints []model.ForgeSprint
	var allIssues []model.ForgeIssue

	for si, sp := range parsed.Sprints {
		sprint := model.ForgeSprint{
			ProjectID: prd.ProjectID,
			PRDID:     prd.ID,
			Name:      sp.Name,
			Goal:      sp.Goal,
			Status:    "planned",
			SeqNum:    si + 1,
		}
		e.DB.Create(&sprint)
		sprints = append(sprints, sprint)

		sprintIssueIDs := make([]string, len(sp.Issues))

		for ii, iss := range sp.Issues {
			e.DB.Model(&project).Update("issue_seq", gorm.Expr("issue_seq + 1"))
			e.DB.First(&project, "id = ?", prd.ProjectID)
			num := project.IssueSeq

			var deps []string
			for _, di := range iss.DependsOnIndices {
				if di >= 0 && di < len(sprintIssueIDs) && sprintIssueIDs[di] != "" {
					deps = append(deps, sprintIssueIDs[di])
				}
			}
			depsJSON := "[]"
			if len(deps) > 0 {
				b, _ := json.Marshal(deps)
				depsJSON = string(b)
			}

			issue := model.ForgeIssue{
				ProjectID:   prd.ProjectID,
				Number:      num,
				Key:         fmt.Sprintf("%s-%d", project.Key, num),
				Title:       iss.Title,
				Body:        iss.Body,
				Type:        orDefault(iss.Type, "task"),
				Priority:    orDefault(iss.Priority, "medium"),
				Status:      "backlog",
				Service:     iss.Service,
				TaskType:    orDefault(iss.TaskType, "code"),
				SprintID:    sprint.ID,
				PRDID:       prd.ID,
				StoryPoints: iss.StoryPoints,
				DependsOn:   depsJSON,
			}
			e.DB.Create(&issue)
			sprintIssueIDs[ii] = issue.ID
			allIssues = append(allIssues, issue)
		}
	}

	e.DB.Model(prd).Update("status", "planned")
	return sprints, allIssues, nil
}

// stripThinkTags removes all <think>...</think> blocks from text, returning only the non-thinking content.
func stripThinkTags(s string) string {
	var result strings.Builder
	for {
		idx := strings.Index(s, "<think>")
		if idx < 0 {
			result.WriteString(s)
			break
		}
		result.WriteString(s[:idx])
		s = s[idx+7:]
		end := strings.Index(s, "</think>")
		if end >= 0 {
			s = s[end+8:]
		} else {
			// unclosed <think> — discard rest
			break
		}
	}
	return result.String()
}

// extractJSON finds the first JSON object in a string (handles markdown code blocks).
func extractJSON(s string) string {
	// Try to find ```json ... ``` block
	start := -1
	for i := 0; i < len(s)-6; i++ {
		if s[i:i+7] == "```json" {
			start = i + 7
			for start < len(s) && s[start] != '{' && s[start] != '[' {
				start++
			}
			break
		}
	}
	if start >= 0 {
		end := len(s)
		for i := start; i < len(s)-2; i++ {
			if s[i:i+3] == "```" {
				end = i
				break
			}
		}
		return s[start:end]
	}
	// Try raw JSON
	for i, c := range s {
		if c == '{' || c == '[' {
			return s[i:]
		}
	}
	return s
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
