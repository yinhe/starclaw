package nerve

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ════════════════════════════════════════════════════════════
// Evolution Loop Demo — 端到端进化闭环演示
//
// 完整链路:
//   1. Bug 上报 → SenseClaw 收集反馈
//   2. SenseClaw 触发告警 → Nerve Bus 自动升级到 Abathur
//   3. Abathur 分析 → 分配任务给 DevClaw (修复) + TestClaw (验证)
//   4. DevClaw 执行修复 (Cocoon build)
//   5. TestClaw 执行回归测试
//   6. OpsClaw 部署上线 (Lair deploy)
//   7. SenseClaw 验证修复 (健康检查)
//
// 每一步都通过 Nerve Bus Dispatch 真正调用对应引擎。
// ════════════════════════════════════════════════════════════

type DemoStep struct {
	Step     int                    `json:"step"`
	Name     string                 `json:"name"`
	Worker   string                 `json:"worker"`
	Action   string                 `json:"action"`
	Status   string                 `json:"status"`
	Duration string                 `json:"duration"`
	Output   map[string]interface{} `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

type DemoResult struct {
	Title     string     `json:"title"`
	Status    string     `json:"status"` // completed, partial_failure
	Steps     []DemoStep `json:"steps"`
	TotalTime string     `json:"total_time"`
	Summary   string     `json:"summary"`
}

// HandleEvolutionDemo handles POST /nerve/demo/evolution
func HandleEvolutionDemo(w http.ResponseWriter, r *http.Request) {
	bus := GetBus()
	if bus == nil {
		http.Error(w, `{"error":"nerve bus not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		BugTitle       string `json:"bug_title"`
		BugDescription string `json:"bug_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.BugTitle == "" {
		req.BugTitle = "Demo Bug: API response time degradation"
	}
	if req.BugDescription == "" {
		req.BugDescription = "P95 latency increased 3x on /v1/chat endpoint after recent deploy"
	}

	log.Printf("[nerve/demo] === Evolution Loop Demo Started ===")
	log.Printf("[nerve/demo] Bug: %s", req.BugTitle)

	start := time.Now()
	var steps []DemoStep
	allOK := true

	// ── Step 1: SenseClaw collects feedback ──
	step1 := runDemoStep(bus, 1, "SenseClaw 收集反馈", "sense_claw", "collect_feedback", map[string]interface{}{
		"title":  req.BugTitle,
		"body":   req.BugDescription,
		"source": "demo_user",
	})
	steps = append(steps, step1)
	if step1.Status != "completed" {
		allOK = false
	}

	// ── Step 2: SenseClaw fires alert ──
	step2 := runDemoStep(bus, 2, "SenseClaw 触发告警", "sense_claw", "fire_alert", map[string]interface{}{
		"severity": "warning",
		"source":   "sense_claw",
		"title":    fmt.Sprintf("Bug detected: %s", req.BugTitle),
		"message":  req.BugDescription,
	})
	steps = append(steps, step2)
	if step2.Status != "completed" {
		allOK = false
	}

	// ── Step 3: DevClaw fixes the bug ──
	step3 := runDemoStep(bus, 3, "DevClaw 修复Bug", "dev_team", "fix_bug", map[string]interface{}{
		"title":       req.BugTitle,
		"description": req.BugDescription,
	})
	steps = append(steps, step3)
	if step3.Status != "completed" {
		allOK = false
	}

	// ── Step 4: DevClaw builds the fix ──
	step4 := runDemoStep(bus, 4, "DevClaw 构建修复包", "dev_team", "build", map[string]interface{}{
		"name":    "hotfix-build",
		"version": "0.0.1-hotfix",
	})
	steps = append(steps, step4)
	if step4.Status != "completed" {
		allOK = false
	}

	// ── Step 5: TestClaw runs regression tests ──
	step5 := runDemoStep(bus, 5, "TestClaw 回归测试", "test_claw", "run_regression", map[string]interface{}{
		"trigger": "hotfix_validation",
	})
	steps = append(steps, step5)
	if step5.Status != "completed" {
		allOK = false
	}

	// ── Step 6: TestClaw runs smoke test ──
	step6 := runDemoStep(bus, 6, "TestClaw 冒烟测试", "test_claw", "run_smoke_test", nil)
	steps = append(steps, step6)
	if step6.Status != "completed" {
		allOK = false
	}

	// ── Step 7: OpsClaw deploys the fix ──
	step7 := runDemoStep(bus, 7, "OpsClaw 部署修复", "ops_claw", "deploy", map[string]interface{}{
		"name":     "hotfix-deploy",
		"agent_id": "demo-agent",
		"version":  "0.0.1-hotfix",
	})
	steps = append(steps, step7)
	if step7.Status != "completed" {
		allOK = false
	}

	// ── Step 8: SenseClaw verifies the fix ──
	step8 := runDemoStep(bus, 8, "SenseClaw 验证修复", "sense_claw", "run_health_check", nil)
	steps = append(steps, step8)
	if step8.Status != "completed" {
		allOK = false
	}

	// ── Step 9: SenseClaw generates insight ──
	step9 := runDemoStep(bus, 9, "SenseClaw 生成洞察", "sense_claw", "generate_insight", nil)
	steps = append(steps, step9)
	if step9.Status != "completed" {
		allOK = false
	}

	totalDuration := time.Since(start)

	status := "completed"
	if !allOK {
		status = "partial_failure"
	}

	completedCount := 0
	for _, s := range steps {
		if s.Status == "completed" {
			completedCount++
		}
	}

	result := DemoResult{
		Title:     fmt.Sprintf("进化闭环演示: %s", req.BugTitle),
		Status:    status,
		Steps:     steps,
		TotalTime: totalDuration.Round(time.Millisecond).String(),
		Summary: fmt.Sprintf("9步进化闭环完成 (%d/%d 成功) · Bug→SenseClaw→DevClaw→TestClaw→OpsClaw→SenseClaw验证 · 总耗时 %s",
			completedCount, len(steps), totalDuration.Round(time.Millisecond)),
	}

	// Publish completion event
	bus.Publish("nerve.demo.completed", "demo", map[string]interface{}{
		"status":    status,
		"steps":     len(steps),
		"completed": completedCount,
		"duration":  totalDuration.String(),
	})

	log.Printf("[nerve/demo] === Evolution Loop Demo %s (%d/%d steps) in %s ===",
		status, completedCount, len(steps), totalDuration.Round(time.Millisecond))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func runDemoStep(bus *Bus, step int, name, workerType, action string, params map[string]interface{}) DemoStep {
	start := time.Now()
	log.Printf("[nerve/demo] Step %d: %s (%s.%s)", step, name, workerType, action)

	result, err := bus.Dispatch(TaskRequest{
		WorkerType:  workerType,
		Action:      action,
		Params:      params,
		Priority:    "P1",
		RequestedBy: "evolution_demo",
		Timeout:     15 * time.Second,
	})

	duration := time.Since(start)
	ds := DemoStep{
		Step:     step,
		Name:     name,
		Worker:   workerType,
		Action:   action,
		Duration: duration.Round(time.Millisecond).String(),
	}

	if err != nil {
		ds.Status = "failed"
		ds.Error = err.Error()
		log.Printf("[nerve/demo] Step %d FAILED: %v", step, err)
	} else {
		ds.Status = result.Status
		ds.Output = result.Output
		if result.Error != "" {
			ds.Error = result.Error
		}
		log.Printf("[nerve/demo] Step %d %s in %s", step, result.Status, duration.Round(time.Millisecond))
	}

	return ds
}
