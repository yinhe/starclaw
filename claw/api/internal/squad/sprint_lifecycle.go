package squad

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/ws"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  20.7.6 — Auto Code Review (Agent cross-review)
// ════════════════════════════════════════════════════════════════

// ReviewMatrix maps producer roles to reviewer roles and review focus areas.
var ReviewMatrix = map[string]struct {
	Reviewer string
	Focus    string
}{
	"Backend":  {Reviewer: "QA", Focus: "安全、错误处理、测试性、输入验证"},
	"Frontend": {Reviewer: "Design", Focus: "UI 一致性、可访问性、响应式设计"},
	"QA":       {Reviewer: "Backend", Focus: "测试覆盖率、边界条件、mock 质量"},
	"PM":       {Reviewer: "Captain", Focus: "需求完整性、可行性、验收标准"},
	"Design":   {Reviewer: "Frontend", Focus: "设计规范一致性、交互合理性"},
	"DevOps":   {Reviewer: "Backend", Focus: "配置正确性、安全性、幂等性"},
}

const maxReviewRetries = 3

// triggerAutoReview creates a StepReview for a completed step and dispatches
// the review to the appropriate reviewer node based on the review matrix.
func (e *Engine) triggerAutoReview(step model.MissionStep, mission model.Mission) {
	// Determine the producer's role from the step's target config
	producerRole := guessRoleFromStep(step)
	reviewSpec, ok := ReviewMatrix[producerRole]
	if !ok {
		// No review matrix entry — auto-approve
		log.Printf("[review] no reviewer for role %s, auto-approving step %s", producerRole, step.ID)
		e.finalizeStepDone(step, mission)
		return
	}

	// Find a reviewer node with the required role
	reviewerNode := e.findNodeByRole(mission.SquadID, reviewSpec.Reviewer)
	if reviewerNode == "" {
		// No reviewer available — auto-approve
		log.Printf("[review] no reviewer node found for role %s, auto-approving step %s", reviewSpec.Reviewer, step.ID)
		e.finalizeStepDone(step, mission)
		return
	}

	// Count existing reviews for this step
	var existingReviews int64
	e.db.Model(&model.StepReview{}).Where("step_id = ?", step.ID).Count(&existingReviews)
	if existingReviews >= maxReviewRetries {
		log.Printf("[review] step %s exceeded max review retries (%d), auto-approving", step.ID, maxReviewRetries)
		e.finalizeStepDone(step, mission)
		return
	}

	// Create a StepReview record
	review := model.StepReview{
		StepID:       step.ID,
		ReviewerNode: reviewerNode,
		Status:       "pending",
	}
	e.db.Create(&review)

	// Mark step as "reviewing"
	e.db.Model(&model.MissionStep{}).Where("id = ?", step.ID).Update("status", "reviewing")

	log.Printf("[review] step %s → reviewer %s (role=%s, focus=%s)",
		step.ID, reviewerNode[:min(16, len(reviewerNode))], reviewSpec.Reviewer, reviewSpec.Focus)

	// Dispatch review task
	go e.executeReview(review, step, mission, reviewSpec.Focus)
}

// executeReview runs the code review via LLM on the reviewer node.
func (e *Engine) executeReview(review model.StepReview, step model.MissionStep, mission model.Mission, focus string) {
	// Build review prompt with the step's output/diff
	diffContent := step.Output
	if len(diffContent) > 3000 {
		diffContent = diffContent[:3000] + "\n...(truncated)"
	}

	prompt := fmt.Sprintf(`你是一个代码审查专家。请审查以下步骤的产出。

## 审查重点
%s

## 步骤任务
%s

## 步骤产出
%s

## Git 分支
%s

请输出 JSON 格式的审查结果：
{
  "verdict": "approved" 或 "changes_requested",
  "summary": "变更摘要（1-2句）",
  "comments": "详细审查意见",
  "issues": ["问题1", "问题2"]
}

规则：
- 如果代码基本可用、无重大安全或逻辑问题 → approved
- 如果有明显 bug、安全漏洞或严重设计问题 → changes_requested
- 审查应该务实，不纠结细枝末节`, focus, step.Task, diffContent, step.Branch)

	// Find a model
	var modelCfg model.ModelConfig
	if err := e.db.Where("is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
		log.Printf("[review] no model available, auto-approving step %s", step.ID)
		e.completeReview(review.ID, step, mission, "approved", "No model available for review", "")
		return
	}

	p := provider.CreateFromConfig(e.providerReg, modelCfg)
	if p == nil {
		e.completeReview(review.ID, step, mission, "approved", "Failed to create provider", "")
		return
	}

	resp, err := p.ChatSync(context.Background(), &provider.ChatRequest{
		Model:       modelCfg.ModelName,
		Messages:    []provider.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   1000,
	})
	if err != nil {
		log.Printf("[review] LLM review failed for step %s: %v", step.ID, err)
		e.completeReview(review.ID, step, mission, "approved", "Review LLM failed: "+err.Error(), "")
		return
	}

	// Parse review result
	content := resp.Content
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	verdict := "approved"
	comments := content
	diffSummary := ""

	if jsonStart >= 0 && jsonEnd > jsonStart {
		var result struct {
			Verdict  string   `json:"verdict"`
			Summary  string   `json:"summary"`
			Comments string   `json:"comments"`
			Issues   []string `json:"issues"`
		}
		if err := json.Unmarshal([]byte(content[jsonStart:jsonEnd+1]), &result); err == nil {
			verdict = result.Verdict
			comments = result.Comments
			diffSummary = result.Summary
			if len(result.Issues) > 0 {
				comments += "\n\nIssues:\n- " + strings.Join(result.Issues, "\n- ")
			}
		}
	}

	if verdict != "approved" && verdict != "changes_requested" {
		verdict = "approved"
	}

	e.completeReview(review.ID, step, mission, verdict, comments, diffSummary)
}

// completeReview finalizes a review and either approves the step or requests changes.
func (e *Engine) completeReview(reviewID string, step model.MissionStep, mission model.Mission, verdict, comments, diffSummary string) {
	e.db.Model(&model.StepReview{}).Where("id = ?", reviewID).Updates(map[string]interface{}{
		"status":       verdict,
		"comments":     comments,
		"diff_summary": diffSummary,
	})

	if verdict == "approved" {
		log.Printf("[review] step %s approved ✅", step.ID)
		e.finalizeStepDone(step, mission)
	} else {
		log.Printf("[review] step %s changes_requested ❌: %.100s", step.ID, comments)
		e.retryStepWithFeedback(step, mission, comments)
	}
}

// retryStepWithFeedback re-dispatches a step after a review rejection.
// It appends the reviewer's feedback to the task so the agent can address the issues,
// then resets the step status and dispatches again. If max retries are exceeded,
// triggerAutoReview will auto-approve on the next round.
func (e *Engine) retryStepWithFeedback(step model.MissionStep, mission model.Mission, reviewComments string) {
	// Count how many reviews this step already has
	var reviewCount int64
	e.db.Model(&model.StepReview{}).Where("step_id = ?", step.ID).Count(&reviewCount)

	if reviewCount >= maxReviewRetries {
		log.Printf("[review-retry] step %s hit max retries (%d), force-approving", step.ID, maxReviewRetries)
		e.finalizeStepDone(step, mission)
		return
	}

	log.Printf("[review-retry] step %s retry %d/%d — re-dispatching with feedback", step.ID, reviewCount, maxReviewRetries)

	// Append review feedback to task
	feedbackSuffix := fmt.Sprintf("\n\n## ⚠️ 审查反馈 (第 %d 次修改)\n审查结果: changes_requested\n\n%s\n\n请根据以上审查意见修改代码，修改完成后 git add + commit + push。",
		reviewCount, reviewComments)

	updatedTask := step.Task + feedbackSuffix
	if len(updatedTask) > 8000 {
		// Truncate old task to keep total reasonable
		updatedTask = step.Task[:4000] + "...(truncated)" + feedbackSuffix
	}

	// Reset step for re-execution
	e.db.Model(&model.MissionStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
		"status": "pending",
		"task":   updatedTask,
		"output": "", // clear previous output
	})

	// Reload step with updated task
	var updatedStep model.MissionStep
	e.db.Where("id = ?", step.ID).First(&updatedStep)

	// Push WS notification about the retry
	hub := ws.GetHub()
	if mission.UserID != "" {
		hub.SendToUser(mission.UserID, ws.EventHiveStepUpdate, map[string]interface{}{
			"step_id":    step.ID,
			"status":     "retry",
			"retry_num":  reviewCount,
			"max_retry":  maxReviewRetries,
			"mission_id": mission.ID,
		})
	}

	// Re-dispatch — use previous output as context for the retry
	retryContext := fmt.Sprintf("你之前的产出被审查拒绝，请根据反馈修改。\n\n## 上次产出\n%s", step.Output)
	go e.dispatchStep(updatedStep, retryContext)
}

// finalizeStepDone marks a step as fully done (post-review) and advances the mission.
func (e *Engine) finalizeStepDone(step model.MissionStep, mission model.Mission) {
	now := time.Now()
	e.db.Model(&model.MissionStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
		"status":       "done",
		"completed_at": &now,
	})
	e.db.Model(&model.Mission{}).Where("id = ?", step.MissionID).
		Update("done_steps", gorm.Expr("done_steps + 1"))

	log.Printf("[squad] step %s finalized as done", step.ID)

	// Push real-time update
	hub := ws.GetHub()
	if mission.UserID != "" {
		hub.SendToUser(mission.UserID, ws.EventHiveStepUpdate, map[string]interface{}{
			"step_id": step.ID,
			"status":  "done",
			"branch":  step.Branch,
		})
	}

	e.checkSprintComplete(step.MissionID)
}

// ════════════════════════════════════════════════════════════════
//  20.7.7 — CI Gate (quality gate after Sprint)
// ════════════════════════════════════════════════════════════════

// CIGateResult holds the outcome of a CI gate check.
type CIGateResult struct {
	Passed  bool   `json:"passed"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Details string `json:"details"`
}

// checkSprintComplete checks if all steps in the current sprint are done,
// then triggers the CI gate pipeline.
func (e *Engine) checkSprintComplete(missionID string) {
	var mission model.Mission
	if err := e.db.Where("id = ?", missionID).First(&mission).Error; err != nil {
		return
	}

	var totalDone int64
	e.db.Model(&model.MissionStep{}).Where("mission_id = ? AND status = ?", missionID, "done").Count(&totalDone)
	var totalFailed int64
	e.db.Model(&model.MissionStep{}).Where("mission_id = ? AND status = ?", missionID, "failed").Count(&totalFailed)
	var totalReviewing int64
	e.db.Model(&model.MissionStep{}).Where("mission_id = ? AND status = ?", missionID, "reviewing").Count(&totalReviewing)

	// If there are still steps being reviewed or pending, wait
	if totalReviewing > 0 {
		return
	}

	total := totalDone + totalFailed
	if int(total) < mission.TotalSteps {
		return // still running
	}

	if totalFailed > 0 {
		// Some steps failed — fail the sprint
		e.failSprint(missionID, fmt.Sprintf("%d/%d steps failed", totalFailed, mission.TotalSteps))
		return
	}

	// All steps done — run CI gate
	go e.runCIGate(mission)
}

// runCIGate executes the CI pipeline stages for a completed sprint.
func (e *Engine) runCIGate(mission model.Mission) {
	log.Printf("[ci-gate] running CI gate for mission %s", mission.ID)

	var sprint model.Sprint
	if err := e.db.Where("mission_id = ? AND status = ?", mission.ID, "executing").
		Order("number DESC").First(&sprint).Error; err != nil {
		log.Printf("[ci-gate] no executing sprint found for mission %s", mission.ID)
		e.completeMission(mission)
		return
	}

	// Update sprint status to reviewing
	e.db.Model(&sprint).Update("status", "reviewing")

	var results []CIGateResult

	// Stage 1: Git Merge
	if mission.RepoPath != "" {
		var steps []model.MissionStep
		e.db.Where("mission_id = ? AND status = ?", mission.ID, "done").Find(&steps)
		var branches []string
		for _, s := range steps {
			if s.Branch != "" {
				branches = append(branches, s.Branch)
			}
		}
		if len(branches) > 0 {
			mergeOutput, err := e.gitMgr.MergeBranches(mission.RepoPath, mission.ID, branches)
			if err != nil {
				results = append(results, CIGateResult{
					Passed: false, Stage: "merge",
					Message: "Git merge failed", Details: err.Error(),
				})
				log.Printf("[ci-gate] merge failed: %v", err)
			} else {
				results = append(results, CIGateResult{
					Passed: true, Stage: "merge",
					Message: fmt.Sprintf("Merged %d branches into master", len(branches)),
					Details: mergeOutput,
				})
				log.Printf("[ci-gate] merge succeeded: %d branches", len(branches))
			}
		}
	}

	// Stage 2-4: LLM-based quality analysis (lint + test + build estimation)
	qualityResult := e.runLLMQualityCheck(mission)
	results = append(results, qualityResult)

	// Check if all stages passed
	allPassed := true
	for _, r := range results {
		if !r.Passed {
			allPassed = false
			break
		}
	}

	resultsJSON, _ := json.Marshal(results)

	if allPassed {
		log.Printf("[ci-gate] mission %s CI gate PASSED ✅", mission.ID)

		// Stage 5: Auto Preview — try to launch a preview server
		previewURL := e.launchPreview(mission)
		if previewURL != "" {
			results = append(results, CIGateResult{
				Passed: true, Stage: "preview",
				Message: "Preview available at " + previewURL,
			})
			e.db.Model(&sprint).Update("preview_url", previewURL)
			e.db.Model(&mission).Update("preview_url", previewURL)
			log.Printf("[ci-gate] preview launched: %s", previewURL)
		}

		resultsJSON, _ = json.Marshal(results) // re-marshal with preview result

		// Update sprint with review notes
		now := time.Now()
		e.db.Model(&sprint).Updates(map[string]interface{}{
			"status":       "done",
			"review_notes": string(resultsJSON),
			"completed_at": &now,
		})

		// Run retrospective before completing
		go e.runRetrospective(mission, sprint, results)
	} else {
		log.Printf("[ci-gate] mission %s CI gate FAILED ❌", mission.ID)
		e.db.Model(&sprint).Updates(map[string]interface{}{
			"status":       "failed",
			"review_notes": string(resultsJSON),
		})
		e.failSprint(mission.ID, "CI gate failed: "+string(resultsJSON))
	}
}

// runLLMQualityCheck uses an LLM to analyze code quality of the sprint output.
func (e *Engine) runLLMQualityCheck(mission model.Mission) CIGateResult {
	var steps []model.MissionStep
	e.db.Where("mission_id = ? AND status = ?", mission.ID, "done").Order("sequence ASC").Find(&steps)

	// Collect step outputs for analysis
	var outputSummaries []string
	for _, s := range steps {
		output := s.Output
		if len(output) > 500 {
			output = output[:500] + "..."
		}
		outputSummaries = append(outputSummaries,
			fmt.Sprintf("Step %d [%s]: %s\nOutput: %s", s.Sequence, s.Branch, s.Task[:min(100, len(s.Task))], output))
	}

	prompt := fmt.Sprintf(`请分析以下 Sprint 的代码产出质量。

## 任务目标
%s

## 步骤产出
%s

请输出 JSON：
{
  "quality_score": 1-10,
  "passed": true/false,
  "issues": ["问题1"],
  "suggestions": ["建议1"],
  "summary": "总体评价"
}

评分标准：
- 8-10 分：代码质量好，可以通过
- 5-7 分：有一些小问题，但可以通过
- 1-4 分：严重问题，不能通过

只要没有严重逻辑错误或安全漏洞，建议通过(passed=true)。`, mission.Goal, strings.Join(outputSummaries, "\n\n"))

	var modelCfg model.ModelConfig
	if err := e.db.Where("is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
		return CIGateResult{Passed: true, Stage: "quality", Message: "No model for quality check, auto-pass"}
	}

	p := provider.CreateFromConfig(e.providerReg, modelCfg)
	if p == nil {
		return CIGateResult{Passed: true, Stage: "quality", Message: "Provider creation failed, auto-pass"}
	}

	resp, err := p.ChatSync(context.Background(), &provider.ChatRequest{
		Model:       modelCfg.ModelName,
		Messages:    []provider.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   800,
	})
	if err != nil {
		return CIGateResult{Passed: true, Stage: "quality", Message: "Quality check LLM failed: " + err.Error()}
	}

	content := resp.Content
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		var result struct {
			QualityScore int      `json:"quality_score"`
			Passed       bool     `json:"passed"`
			Issues       []string `json:"issues"`
			Suggestions  []string `json:"suggestions"`
			Summary      string   `json:"summary"`
		}
		if err := json.Unmarshal([]byte(content[jsonStart:jsonEnd+1]), &result); err == nil {
			return CIGateResult{
				Passed:  result.Passed,
				Stage:   "quality",
				Message: fmt.Sprintf("Score: %d/10 — %s", result.QualityScore, result.Summary),
				Details: content[jsonStart : jsonEnd+1],
			}
		}
	}

	return CIGateResult{Passed: true, Stage: "quality", Message: "Could not parse quality result, auto-pass", Details: content}
}

// ════════════════════════════════════════════════════════════════
//  20.7.8 — Sprint Retrospective (LLM self-optimization)
// ════════════════════════════════════════════════════════════════

// RetroResult is the output of a sprint retrospective.
type RetroResult struct {
	SuccessRate     string   `json:"success_rate"`
	FailureAnalysis []string `json:"failure_analysis"`
	QualityIssues   []string `json:"quality_issues"`
	Improvements    []string `json:"improvements"`
	NextSprintHints []string `json:"next_sprint_hints"`
}

// runRetrospective analyzes the completed sprint and produces optimization hints.
// After analysis, decides whether to start a new Sprint or complete the mission.
func (e *Engine) runRetrospective(mission model.Mission, sprint model.Sprint, ciResults []CIGateResult) {
	log.Printf("[retro] running retrospective for mission %s sprint %d", mission.ID, sprint.Number)

	var steps []model.MissionStep
	e.db.Where("mission_id = ? AND sprint_id = ?", mission.ID, sprint.ID).Order("sequence ASC").Find(&steps)

	// Collect execution data
	var stepSummaries []string
	doneCount := 0
	failedCount := 0
	for _, s := range steps {
		status := s.Status
		if status == "done" {
			doneCount++
		} else if status == "failed" {
			failedCount++
		}
		errInfo := ""
		if s.ErrorMsg != "" {
			errInfo = " ERROR: " + s.ErrorMsg
		}
		stepSummaries = append(stepSummaries,
			fmt.Sprintf("Step %d [%s → %s]: %s (status=%s)%s",
				s.Sequence, s.TargetNode, s.Branch, s.Task[:min(80, len(s.Task))], status, errInfo))
	}

	// Collect reviews
	var reviews []model.StepReview
	for _, s := range steps {
		var stepReviews []model.StepReview
		e.db.Where("step_id = ?", s.ID).Find(&stepReviews)
		reviews = append(reviews, stepReviews...)
	}
	var reviewSummaries []string
	for _, r := range reviews {
		reviewSummaries = append(reviewSummaries,
			fmt.Sprintf("Review %s: %s — %s", r.Status, r.DiffSummary, r.Comments[:min(100, len(r.Comments))]))
	}

	// CI results
	var ciSummaries []string
	for _, r := range ciResults {
		passStr := "PASS"
		if !r.Passed {
			passStr = "FAIL"
		}
		ciSummaries = append(ciSummaries, fmt.Sprintf("[%s] %s: %s", passStr, r.Stage, r.Message))
	}

	// User feedback
	feedback := sprint.UserFeedback

	prompt := fmt.Sprintf(`你是敏捷教练。请对以下 Sprint 进行回顾分析。

## Sprint 信息
- 任务: %s
- Sprint %d, 目标: %s
- 成功率: %d/%d

## 步骤执行
%s

## Code Review
%s

## CI 结果
%s

## 用户反馈
%s

请输出 JSON：
{
  "success_rate": "x/y (百分比)",
  "failure_analysis": ["失败原因1"],
  "quality_issues": ["质量问题1"],
  "improvements": ["改进建议1"],
  "next_sprint_hints": ["下轮提示1"]
}`,
		mission.Goal,
		sprint.Number, sprint.Goal,
		doneCount, len(steps),
		strings.Join(stepSummaries, "\n"),
		strings.Join(reviewSummaries, "\n"),
		strings.Join(ciSummaries, "\n"),
		feedback,
	)

	var modelCfg model.ModelConfig
	if err := e.db.Where("is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
		log.Printf("[retro] no model available, skipping retrospective")
		e.completeMission(mission)
		return
	}

	p := provider.CreateFromConfig(e.providerReg, modelCfg)
	if p == nil {
		e.completeMission(mission)
		return
	}

	resp, err := p.ChatSync(context.Background(), &provider.ChatRequest{
		Model:       modelCfg.ModelName,
		Messages:    []provider.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   800,
	})

	retroNotes := ""
	if err == nil {
		retroNotes = resp.Content
		log.Printf("[retro] retrospective complete for mission %s sprint %d", mission.ID, sprint.Number)
	} else {
		retroNotes = "Retrospective failed: " + err.Error()
		log.Printf("[retro] retrospective LLM failed: %v", err)
	}

	// Append retro notes to sprint review_notes
	existingNotes := sprint.ReviewNotes
	if existingNotes != "" {
		retroNotes = existingNotes + "\n\n## Retrospective\n" + retroNotes
	}
	e.db.Model(&sprint).Update("review_notes", retroNotes)

	// Decide: start next Sprint or complete mission?
	// Re-read mission to get latest state (user may have submitted feedback)
	var freshMission model.Mission
	e.db.Where("id = ?", mission.ID).First(&freshMission)

	var freshSprint model.Sprint
	e.db.Where("id = ?", sprint.ID).First(&freshSprint)

	if e.shouldStartNextSprint(freshMission, freshSprint, retroNotes) {
		e.startNextSprint(freshMission, freshSprint, retroNotes)
	} else {
		e.completeMission(freshMission)
	}
}

// ════════════════════════════════════════════════════════════════
//  20.7.9 — User Feedback
// ════════════════════════════════════════════════════════════════

// SubmitFeedback stores user feedback on a mission's current sprint.
// This is called from the API handler.
func (e *Engine) SubmitFeedback(missionID, feedback string) error {
	var sprint model.Sprint
	if err := e.db.Where("mission_id = ?", missionID).
		Order("number DESC").First(&sprint).Error; err != nil {
		return fmt.Errorf("no sprint found for mission %s", missionID)
	}

	existing := sprint.UserFeedback
	if existing != "" {
		feedback = existing + "\n---\n" + feedback
	}

	e.db.Model(&sprint).Update("user_feedback", feedback)
	log.Printf("[feedback] stored feedback for mission %s sprint %d", missionID, sprint.Number)
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Helpers
// ════════════════════════════════════════════════════════════════

// ════════════════════════════════════════════════════════════════
//  Auto Preview (20.7.7 Stage 5)
// ════════════════════════════════════════════════════════════════

// launchPreview attempts to start a preview server for the merged workspace.
// Returns the preview URL if successful, empty string otherwise.
func (e *Engine) launchPreview(mission model.Mission) string {
	wsPath := mission.WorkspacePath
	if wsPath == "" {
		wsPath = e.gitMgr.GetMissionWorkspace(mission.ID)
	}
	if wsPath == "" {
		return ""
	}

	// Detect project type and choose preview command
	previewPort := 9000 + (int(time.Now().UnixNano()) % 1000) // semi-random port 9000-9999
	var cmd *exec.Cmd

	switch {
	case fileExists(wsPath + "/package.json"):
		// Node.js project — try npm run dev or npm start
		if fileContains(wsPath+"/package.json", "\"dev\"") {
			cmd = exec.Command("npm", "run", "dev", "--", "--port", fmt.Sprintf("%d", previewPort))
		} else {
			cmd = exec.Command("npm", "start")
		}
	case fileExists(wsPath + "/main.go"):
		// Go project
		cmd = exec.Command("go", "run", "main.go")
	case fileExists(wsPath + "/requirements.txt"):
		// Python project
		cmd = exec.Command("python", "app.py")
	case fileExists(wsPath + "/index.html"):
		// Static site — use python http.server
		cmd = exec.Command("python", "-m", "http.server", fmt.Sprintf("%d", previewPort))
	default:
		log.Printf("[preview] no recognized project type in %s", wsPath)
		return ""
	}

	cmd.Dir = wsPath
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		log.Printf("[preview] failed to start preview: %v", err)
		return ""
	}

	// Give server a moment to start
	time.Sleep(2 * time.Second)

	// Build preview URL
	host := "localhost"
	if e.selfAddress != "" {
		// Extract host from self address
		h := strings.TrimPrefix(e.selfAddress, "https://")
		h = strings.TrimPrefix(h, "http://")
		if idx := strings.Index(h, ":"); idx > 0 {
			h = h[:idx]
		}
		if h != "" {
			host = h
		}
	}

	previewURL := fmt.Sprintf("http://%s:%d", host, previewPort)
	log.Printf("[preview] started preview server at %s (pid=%d)", previewURL, cmd.Process.Pid)

	return previewURL
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fileContains checks if a file contains a substring.
func fileContains(path, substr string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substr)
}

// shouldStartNextSprint decides if another Sprint iteration is needed.
func (e *Engine) shouldStartNextSprint(mission model.Mission, sprint model.Sprint, retroNotes string) bool {
	// Don't exceed max sprints
	if sprint.Number+1 >= mission.MaxSprints {
		log.Printf("[sprint] mission %s reached max sprints (%d), completing", mission.ID, mission.MaxSprints)
		return false
	}

	// If user submitted feedback, always iterate
	if sprint.UserFeedback != "" {
		log.Printf("[sprint] mission %s has user feedback, starting next sprint", mission.ID)
		return true
	}

	// If retro suggests improvements and we have room, iterate
	if strings.Contains(retroNotes, "next_sprint_hints") &&
		!strings.Contains(retroNotes, "\"next_sprint_hints\": []") {
		// Parse retro to check if hints are meaningful
		jsonStart := strings.Index(retroNotes, "{")
		jsonEnd := strings.LastIndex(retroNotes, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			var retro RetroResult
			if err := json.Unmarshal([]byte(retroNotes[jsonStart:jsonEnd+1]), &retro); err == nil {
				if len(retro.NextSprintHints) > 0 && len(retro.QualityIssues) > 0 {
					log.Printf("[sprint] mission %s has %d quality issues and %d hints, starting next sprint",
						mission.ID, len(retro.QualityIssues), len(retro.NextSprintHints))
					return true
				}
			}
		}
	}

	return false
}

// startNextSprint creates and dispatches a new Sprint incorporating retro + feedback.
func (e *Engine) startNextSprint(mission model.Mission, prevSprint model.Sprint, retroNotes string) {
	nextNum := prevSprint.Number + 1
	log.Printf("[sprint] starting sprint %d for mission %s", nextNum, mission.ID)

	// Update mission current sprint
	e.db.Model(&mission).Update("current_sprint", nextNum)

	// Build sprint goal from feedback + retro
	var goalParts []string
	goalParts = append(goalParts, fmt.Sprintf("Sprint %d 改进迭代: %s", nextNum, mission.Title))

	if prevSprint.UserFeedback != "" {
		goalParts = append(goalParts, "用户反馈: "+prevSprint.UserFeedback)
	}

	// Extract retro hints
	jsonStart := strings.Index(retroNotes, "{")
	jsonEnd := strings.LastIndex(retroNotes, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		var retro RetroResult
		if err := json.Unmarshal([]byte(retroNotes[jsonStart:jsonEnd+1]), &retro); err == nil {
			if len(retro.Improvements) > 0 {
				goalParts = append(goalParts, "改进建议: "+strings.Join(retro.Improvements, "; "))
			}
			if len(retro.NextSprintHints) > 0 {
				goalParts = append(goalParts, "下轮提示: "+strings.Join(retro.NextSprintHints, "; "))
			}
		}
	}

	sprintGoal := strings.Join(goalParts, "\n")

	// Get squad members
	var members []model.SquadMember
	e.db.Where("squad_id = ?", mission.SquadID).Find(&members)
	if len(members) == 0 {
		e.completeMission(mission)
		return
	}

	// Build member descriptions
	var memberDescs []string
	for _, m := range members {
		desc := fmt.Sprintf("- 节点 %s (角色: %s, 特长: %s)", m.NodeID[:min(16, len(m.NodeID))], m.Role, m.Specialty)
		memberDescs = append(memberDescs, desc)
	}

	// Generate new plan incorporating previous sprint context
	plan, err := e.generateSprintPlan(mission, nextNum, sprintGoal, memberDescs)
	if err != nil {
		log.Printf("[sprint] planning failed for sprint %d: %v", nextNum, err)
		e.completeMission(mission)
		return
	}

	if len(plan.Steps) == 0 {
		log.Printf("[sprint] sprint %d generated 0 steps, completing mission", nextNum)
		e.completeMission(mission)
		return
	}

	// Assign steps
	e.assignStepsToNodes(plan, members)

	// Create Sprint record
	sprint := model.Sprint{
		MissionID:  mission.ID,
		Number:     nextNum,
		Goal:       sprintGoal,
		Status:     "executing",
		TotalSteps: len(plan.Steps),
	}
	now := time.Now()
	sprint.StartedAt = &now
	e.db.Create(&sprint)

	// Create steps
	for i, ps := range plan.Steps {
		stepID := fmt.Sprintf("%s-s%d-%d", mission.ID[:8], nextNum, i)
		branch := fmt.Sprintf("sprint-%d/step-%d-%s", nextNum, i, sanitizeBranch(ps.Specialty))

		var deps []string
		depsJSON, _ := json.Marshal(deps)

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

	// Update mission step counts
	var totalSteps int64
	e.db.Model(&model.MissionStep{}).Where("mission_id = ?", mission.ID).Count(&totalSteps)
	e.db.Model(&mission).Updates(map[string]interface{}{
		"total_steps": totalSteps,
		"status":      "executing",
	})

	log.Printf("[sprint] sprint %d started: %d steps", nextNum, len(plan.Steps))

	// Push real-time update
	hub := ws.GetHub()
	if mission.UserID != "" {
		hub.SendToUser(mission.UserID, ws.EventHiveSprint, map[string]interface{}{
			"mission_id":    mission.ID,
			"sprint_number": nextNum,
			"sprint_status": "executing",
			"total_steps":   len(plan.Steps),
		})
	}

	// Dispatch ready steps
	e.advanceMission(mission.ID)
}

// generateSprintPlan generates a plan for a subsequent Sprint with retro context.
func (e *Engine) generateSprintPlan(mission model.Mission, sprintNum int, sprintGoal string, memberDescs []string) (*MissionPlan, error) {
	prompt := fmt.Sprintf(`你是一个敏捷开发编排专家。请为 Sprint %d 生成改进计划。

## 原始任务目标
%s

## Sprint %d 目标（含改进要求）
%s

## 可用团队成员
%s

## 执行环境
每个步骤会分配一个独立的 Git 分支。Agent 拥有 code 和 git 工具。
所有步骤完成后自动 merge。

请输出 JSON 格式的执行计划：
{
  "steps": [
    {
      "title": "步骤标题",
      "task": "详细任务描述，必须产出实际代码文件，完成后 git add + commit + push",
      "specialty": "coding/design/testing/general",
      "agent_name": "",
      "depends_on": []
    }
  ]
}

规则：
1. 聚焦于改进和修复，不要重复上一轮已完成的工作
2. 每个步骤必须产出实际代码
3. 步骤数量 1-5 个
4. 只输出 JSON`,
		sprintNum, mission.Goal, sprintNum, sprintGoal, strings.Join(memberDescs, "\n"))

	var modelCfg model.ModelConfig
	if err := e.db.Where("is_enabled = ?", true).Order("created_at ASC").First(&modelCfg).Error; err != nil {
		return nil, fmt.Errorf("no model available")
	}

	p := provider.CreateFromConfig(e.providerReg, modelCfg)
	if p == nil {
		return nil, fmt.Errorf("failed to create provider")
	}

	resp, err := p.ChatSync(context.Background(), &provider.ChatRequest{
		Model:       modelCfg.ModelName,
		Messages:    []provider.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   1500,
	})
	if err != nil {
		return nil, err
	}

	content := resp.Content
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON in response")
	}

	var plan MissionPlan
	if err := json.Unmarshal([]byte(content[jsonStart:jsonEnd+1]), &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// completeMission marks a mission as completed with all results collected.
func (e *Engine) completeMission(mission model.Mission) {
	var steps []model.MissionStep
	e.db.Where("mission_id = ? AND status = ?", mission.ID, "done").Order("sequence ASC").Find(&steps)

	var results []string
	for _, s := range steps {
		results = append(results, fmt.Sprintf("## %s\n%s", s.Task[:min(80, len(s.Task))], s.Output))
	}

	now := time.Now()
	var totalDone int64
	e.db.Model(&model.MissionStep{}).Where("mission_id = ? AND status = ?", mission.ID, "done").Count(&totalDone)

	e.db.Model(&mission).Updates(map[string]interface{}{
		"status":       "completed",
		"done_steps":   totalDone,
		"final_result": strings.Join(results, "\n\n---\n\n"),
		"completed_at": &now,
	})

	// Push completion event
	hub := ws.GetHub()
	if mission.UserID != "" {
		hub.SendToUser(mission.UserID, ws.EventHiveSprint, map[string]interface{}{
			"mission_id":    mission.ID,
			"sprint_status": "done",
			"done_steps":    totalDone,
		})
	}

	log.Printf("[squad] mission %s completed! (%d steps)", mission.ID, totalDone)
}

// failSprint marks the current sprint as failed.
func (e *Engine) failSprint(missionID, reason string) {
	var sprint model.Sprint
	if err := e.db.Where("mission_id = ? AND status IN ?", missionID, []string{"executing", "reviewing"}).
		Order("number DESC").First(&sprint).Error; err == nil {
		now := time.Now()
		e.db.Model(&sprint).Updates(map[string]interface{}{
			"status":       "failed",
			"completed_at": &now,
		})
	}
	e.failMission(missionID, reason)
}

// guessRoleFromStep infers the producer role from step metadata.
func guessRoleFromStep(step model.MissionStep) string {
	task := strings.ToLower(step.Task)
	agent := strings.ToLower(step.TargetAgent)
	combined := task + " " + agent + " " + strings.ToLower(step.TargetNode)

	switch {
	case strings.Contains(combined, "backend") || strings.Contains(combined, "api") || strings.Contains(combined, "server"):
		return "Backend"
	case strings.Contains(combined, "frontend") || strings.Contains(combined, "ui") || strings.Contains(combined, "react"):
		return "Frontend"
	case strings.Contains(combined, "test") || strings.Contains(combined, "qa"):
		return "QA"
	case strings.Contains(combined, "design") || strings.Contains(combined, "css") || strings.Contains(combined, "style"):
		return "Design"
	case strings.Contains(combined, "devops") || strings.Contains(combined, "deploy") || strings.Contains(combined, "docker"):
		return "DevOps"
	case strings.Contains(combined, "pm") || strings.Contains(combined, "product") || strings.Contains(combined, "plan"):
		return "PM"
	default:
		return ""
	}
}

// findNodeByRole finds a squad member node with the given role.
func (e *Engine) findNodeByRole(squadID, role string) string {
	roleLower := strings.ToLower(role)
	var members []model.SquadMember
	e.db.Where("squad_id = ?", squadID).Find(&members)

	for _, m := range members {
		if strings.EqualFold(m.Specialty, roleLower) || strings.Contains(strings.ToLower(m.Specialty), roleLower) {
			return m.NodeID
		}
	}

	// Fallback: try Hivemind capability match
	if e.hivemind != nil {
		nodeID, _ := e.hivemind.FindBestNodeForSpecialty(roleLower, nil)
		if nodeID != "" {
			return nodeID
		}
	}

	return ""
}
