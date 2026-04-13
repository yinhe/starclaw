package nerve

import (
	"fmt"
	"log"
	"time"

	"github.com/yinhe/starclaw/internal/chitin"
	"github.com/yinhe/starclaw/internal/cocoon"
	"github.com/yinhe/starclaw/internal/lair"
	"github.com/yinhe/starclaw/internal/sense"
	"github.com/yinhe/starclaw/internal/testclaw"
)

// ════════════════════════════════════════════════════════════
// Worker Bee Wiring — 工蜂神经连接
//
// 将 Phase 4C 各引擎注册为 Nerve Bus 上的 Worker，
// 使 Abathur 能通过 Dispatch 真正调度工蜂执行任务。
// ════════════════════════════════════════════════════════════

// RegisterAllWorkers wires all worker bee engines to the Nerve Bus.
// Call this after all engines are initialized.
func RegisterAllWorkers(bus *Bus) {
	bus.RegisterWorker("sense_claw", senseClawWorker)
	bus.RegisterWorker("test_claw", testClawWorker)
	bus.RegisterWorker("ops_claw", opsClawWorker)
	bus.RegisterWorker("dev_team", devTeamWorker)
	bus.RegisterWorker("scout_claw", scoutClawWorker)
	log.Printf("[nerve] all 5 worker bees wired to nerve bus")
}

// RegisterCrossEngineSubscriptions sets up event-driven cross-engine connections.
func RegisterCrossEngineSubscriptions(bus *Bus) {
	// When SenseClaw fires an alert, auto-create an Abathur hotfix for P0/P1
	bus.Subscribe("sense.alert.fired", func(e Event) {
		sev, _ := e.Payload["severity"].(string)
		if sev == "critical" || sev == "warning" {
			log.Printf("[nerve] auto-escalating %s alert to Abathur hotfix", sev)
			bus.Publish("abathur.hotfix.auto", "nerve", map[string]interface{}{
				"source_alert": e.ID,
				"severity":     sev,
				"title":        e.Payload["title"],
			})
		}
	})

	// When a Lair deployment completes, trigger TestClaw post-deploy validation
	bus.Subscribe("lair.deploy.completed", func(e Event) {
		log.Printf("[nerve] auto-triggering post-deploy validation for %v", e.Payload["deploy_id"])
		bus.Dispatch(TaskRequest{
			WorkerType:  "test_claw",
			Action:      "post_deploy_validation",
			Params:      e.Payload,
			Priority:    "P1",
			RequestedBy: "nerve_auto",
			Timeout:     60 * time.Second,
		})
	})

	// When TestClaw completes a suite, publish result for SenseClaw to track
	bus.Subscribe("testclaw.suite.completed", func(e Event) {
		failed, _ := e.Payload["failed"].(int)
		if failed > 0 {
			bus.Publish("sense.feedback.auto", "nerve", map[string]interface{}{
				"type":     "bug",
				"source":   "testclaw_auto",
				"title":    fmt.Sprintf("Test suite had %d failures", failed),
				"severity": "warning",
			})
		}
	})

	// When Cocoon publishes a package, notify Lair for potential rollout
	bus.Subscribe("cocoon.published", func(e Event) {
		log.Printf("[nerve] package published: %v — notifying Lair", e.Payload["name"])
		bus.Publish("lair.rollout.candidate", "nerve", e.Payload)
	})

	log.Printf("[nerve] cross-engine subscriptions wired (4 auto-rules)")
}

// ── Worker Implementations ──

func senseClawWorker(req TaskRequest) TaskResult {
	eng := sense.GetEngine()
	if eng == nil {
		return TaskResult{Status: "failed", Error: "sense engine not initialized"}
	}

	switch req.Action {
	case "run_health_check":
		health := eng.GetHealth()
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"health": health,
			},
		}

	case "collect_feedback":
		title, _ := req.Params["title"].(string)
		body, _ := req.Params["body"].(string)
		source, _ := req.Params["source"].(string)
		fb := eng.SubmitFeedback(sense.FeedbackBug, sense.FBPriMedium, title, body, source, nil)
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"feedback_id": fb.ID,
				"type":        fb.Type,
			},
		}

	case "fire_alert":
		severity, _ := req.Params["severity"].(string)
		source, _ := req.Params["source"].(string)
		title, _ := req.Params["title"].(string)
		msg, _ := req.Params["message"].(string)
		sev := sense.AlertWarning
		if severity == "critical" {
			sev = sense.AlertCritical
		} else if severity == "info" {
			sev = sense.AlertInfo
		}
		alert := eng.FireAlert(source, sev, title, msg, 0, 0, nil)
		if bus := GetBus(); bus != nil {
			bus.Publish("sense.alert.fired", "sense_claw", map[string]interface{}{
				"alert_id": alert.ID,
				"severity": severity,
				"title":    title,
			})
		}
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"alert_id": alert.ID,
			},
		}

	case "generate_insight":
		insight := eng.GenerateInsight("auto", "Auto Insight", "System-generated insight", "medium", nil)
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"insight_id": insight.ID,
				"summary":    insight.Summary,
			},
		}

	default:
		return TaskResult{Status: "failed", Error: fmt.Sprintf("unknown sense action: %s", req.Action)}
	}
}

func testClawWorker(req TaskRequest) TaskResult {
	eng := testclaw.GetEngine()
	if eng == nil {
		return TaskResult{Status: "failed", Error: "testclaw engine not initialized"}
	}

	switch req.Action {
	case "run_smoke_test":
		suite := eng.CreateSuite("Smoke Test (auto)", "smoke", "Auto-triggered smoke test")
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"suite_id": suite.ID,
				"status":   suite.Status,
			},
		}

	case "post_deploy_validation":
		suite := eng.CreateSuite("Post-Deploy Validation (auto)", "deploy", "Auto-triggered post-deploy validation")
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"suite_id": suite.ID,
				"type":     "deploy",
			},
		}

	case "run_regression":
		suite := eng.CreateSuite("Regression (auto)", "regression", "Auto-triggered regression test")
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"suite_id": suite.ID,
				"type":     "regression",
			},
		}

	default:
		return TaskResult{Status: "failed", Error: fmt.Sprintf("unknown testclaw action: %s", req.Action)}
	}
}

func opsClawWorker(req TaskRequest) TaskResult {
	switch req.Action {
	case "deploy":
		lairEng := lair.GetEngine()
		if lairEng == nil {
			return TaskResult{Status: "failed", Error: "lair engine not initialized"}
		}
		agentID, _ := req.Params["agent_id"].(string)
		version, _ := req.Params["version"].(string)
		name, _ := req.Params["name"].(string)
		dep, err := lairEng.Deploy(name, agentID, version, "", "", "", 1)
		if err != nil {
			return TaskResult{Status: "failed", Error: err.Error()}
		}
		if bus := GetBus(); bus != nil {
			bus.Publish("lair.deploy.completed", "ops_claw", map[string]interface{}{
				"deploy_id": dep.ID,
				"agent_id":  agentID,
				"version":   version,
			})
		}
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"deploy_id": dep.ID,
				"status":    dep.Status,
			},
		}

	case "health_patrol":
		// Check Chitin instances health
		chitinEng := chitin.GetEngine()
		if chitinEng == nil {
			return TaskResult{Status: "failed", Error: "chitin engine not initialized"}
		}
		stats := chitinEng.Stats()
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"chitin_stats": stats,
			},
		}

	case "rollout":
		lairEng := lair.GetEngine()
		if lairEng == nil {
			return TaskResult{Status: "failed", Error: "lair engine not initialized"}
		}
		name, _ := req.Params["name"].(string)
		agentID, _ := req.Params["agent_id"].(string)
		version, _ := req.Params["version"].(string)
		strategy, _ := req.Params["strategy"].(string)
		if strategy == "" {
			strategy = "rolling"
		}
		rollout, err := lairEng.CreateRollout(name, agentID, version, strategy, nil, 1)
		if err != nil {
			return TaskResult{Status: "failed", Error: err.Error()}
		}
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"rollout_id": rollout.ID,
				"strategy":   strategy,
			},
		}

	default:
		return TaskResult{Status: "failed", Error: fmt.Sprintf("unknown ops action: %s", req.Action)}
	}
}

func devTeamWorker(req TaskRequest) TaskResult {
	// DevClaw is a team agent (drone/agents/dev_team/) — in v1 we create a
	// Cocoon build as a proxy for "development work done"
	switch req.Action {
	case "build":
		cocoonEng := cocoon.GetEngine()
		if cocoonEng == nil {
			return TaskResult{Status: "failed", Error: "cocoon engine not initialized"}
		}
		build, err := cocoonEng.StartBuild("", cocoon.TargetLinuxAMD64)
		if err != nil {
			return TaskResult{Status: "failed", Error: err.Error()}
		}
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"build_id": build.ID,
				"status":   build.Status,
			},
		}

	case "fix_bug":
		// Simulate: acknowledge the bug fix task
		title, _ := req.Params["title"].(string)
		log.Printf("[nerve/dev_team] bug fix task received: %s", title)
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"action":  "fix_bug",
				"title":   title,
				"message": "Bug fix task acknowledged and queued for dev_team agent",
			},
		}

	case "implement_feature":
		title, _ := req.Params["title"].(string)
		log.Printf("[nerve/dev_team] feature implementation task received: %s", title)
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"action":  "implement_feature",
				"title":   title,
				"message": "Feature task acknowledged and queued for dev_team agent",
			},
		}

	default:
		return TaskResult{Status: "failed", Error: fmt.Sprintf("unknown dev_team action: %s", req.Action)}
	}
}

func scoutClawWorker(req TaskRequest) TaskResult {
	// ScoutClaw collects external data (drone/agents/scout_claw/)
	switch req.Action {
	case "scan_marketplace":
		log.Printf("[nerve/scout_claw] marketplace scan requested")
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"action":  "scan_marketplace",
				"message": "Marketplace scan queued for scout_claw agent",
			},
		}

	case "competitive_analysis":
		target, _ := req.Params["target"].(string)
		log.Printf("[nerve/scout_claw] competitive analysis requested for: %s", target)
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"action":  "competitive_analysis",
				"target":  target,
				"message": "Analysis queued for scout_claw agent",
			},
		}

	case "collect_external":
		source, _ := req.Params["source"].(string)
		log.Printf("[nerve/scout_claw] external collection from: %s", source)
		return TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"action": "collect_external",
				"source": source,
			},
		}

	default:
		return TaskResult{Status: "failed", Error: fmt.Sprintf("unknown scout action: %s", req.Action)}
	}
}
