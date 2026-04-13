package wiring

import (
	"fmt"
	"log"
	"time"

	"github.com/yinhe/starclaw/internal/autonomy"
	"github.com/yinhe/starclaw/internal/exchange"
	"github.com/yinhe/starclaw/internal/federation"
	"github.com/yinhe/starclaw/internal/nerve"
	"github.com/yinhe/starclaw/internal/swarmctl"
)

// WirePhase5 registers Phase 5 engine workers and cross-engine subscriptions.
// Call after all engines and the Nerve Bus are initialized.
func WirePhase5(bus *nerve.Bus) {
	bus.RegisterWorker("autonomy", autonomyWorker)
	bus.RegisterWorker("exchange", exchangeWorker)
	bus.RegisterWorker("federation", federationWorker)
	bus.RegisterWorker("swarmctl", swarmctlWorker)
	log.Printf("[wiring] Phase 5 civilization workers wired (4 engines)")

	// ── Cross-engine subscriptions ──

	// When swarmctl completes a mission, feed to autonomy for learning
	bus.Subscribe("swarmctl.mission.completed", func(e nerve.Event) {
		success, _ := e.Payload["success"].(bool)
		missionID, _ := e.Payload["mission_id"].(string)
		if !success {
			eng := autonomy.GetEngine()
			if eng == nil {
				return
			}
			eng.ProposeDecision(
				"health_action",
				"mission_failure",
				fmt.Sprintf("Physical mission %s failed", missionID),
				"Mission failure may indicate unit issues or planning flaws",
				"medium",
				autonomy.ProposedAction{
					WorkerType: "sense_claw",
					Action:     "run_health_check",
					Params:     e.Payload,
					Priority:   "P1",
				},
			)
		}
	})

	// When autonomy auto-degrades, log warning
	bus.Subscribe("autonomy.auto_degrade", func(e nerve.Event) {
		log.Printf("[wiring/p5] AUTONOMY DEGRADED: %v → %v", e.Payload["old_level"], e.Payload["new_level"])
	})

	// When federation handshake completes, log for exchange audit
	bus.Subscribe("federation.handshake.completed", func(e nerve.Event) {
		log.Printf("[wiring/p5] federation handshake completed: %v", e.Payload["handshake_id"])
	})

	log.Printf("[wiring] Phase 5 cross-engine subscriptions wired (3 rules)")

	// Start autonomy periodic snapshot goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			eng := autonomy.GetEngine()
			if eng == nil {
				continue
			}
			eng.TakeSnapshot()
		}
	}()
}

// ── Worker Implementations ──

func autonomyWorker(req nerve.TaskRequest) nerve.TaskResult {
	eng := autonomy.GetEngine()
	if eng == nil {
		return nerve.TaskResult{Status: "failed", Error: "autonomy engine not initialized"}
	}

	switch req.Action {
	case "get_level":
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"level": eng.GetLevel().String()},
		}

	case "propose_decision":
		dtype, _ := req.Params["type"].(string)
		trigger, _ := req.Params["trigger"].(string)
		desc, _ := req.Params["description"].(string)
		reasoning, _ := req.Params["reasoning"].(string)
		risk, _ := req.Params["risk"].(string)
		wt, _ := req.Params["worker_type"].(string)
		act, _ := req.Params["action_name"].(string)
		pri, _ := req.Params["priority"].(string)
		d := eng.ProposeDecision(dtype, trigger, desc, reasoning, risk, autonomy.ProposedAction{
			WorkerType: wt, Action: act, Priority: pri,
		})
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"decision_id": d.ID, "status": string(d.Status)},
		}

	case "evaluate_rules":
		metrics := make(map[string]float64)
		for k, v := range req.Params {
			if f, ok := v.(float64); ok {
				metrics[k] = f
			}
		}
		ids := eng.EvaluateRules(metrics)
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"triggered": len(ids)},
		}

	case "take_snapshot":
		snap := eng.TakeSnapshot()
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"level": snap.AutonomyLevel.String(), "success_rate": snap.SuccessRate,
			},
		}

	case "diagnose":
		insights := eng.Diagnose()
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"insights": len(insights)},
		}

	default:
		return nerve.TaskResult{Status: "failed", Error: fmt.Sprintf("unknown autonomy action: %s", req.Action)}
	}
}

func exchangeWorker(req nerve.TaskRequest) nerve.TaskResult {
	eng := exchange.GetEngine()
	if eng == nil {
		return nerve.TaskResult{Status: "failed", Error: "exchange engine not initialized"}
	}

	switch req.Action {
	case "place_order":
		nodeID, _ := req.Params["node_id"].(string)
		side, _ := req.Params["side"].(string)
		otype, _ := req.Params["type"].(string)
		price, _ := req.Params["price"].(float64)
		qty, _ := req.Params["quantity"].(float64)
		order, err := eng.PlaceOrder(nodeID, exchange.OrderSide(side), exchange.OrderType(otype), price, qty)
		if err != nil {
			return nerve.TaskResult{Status: "failed", Error: err.Error()}
		}
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"order_id": order.ID, "status": string(order.Status)},
		}

	case "list_service":
		agentID, _ := req.Params["agent_id"].(string)
		nodeID, _ := req.Params["node_id"].(string)
		name, _ := req.Params["name"].(string)
		desc, _ := req.Params["description"].(string)
		cat, _ := req.Params["category"].(string)
		price, _ := req.Params["base_price"].(float64)
		svc := eng.ListService(agentID, nodeID, name, desc, cat, price, nil)
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"service_id": svc.ID},
		}

	case "get_stats":
		stats := eng.Stats()
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"orders": stats.OrdersPlaced, "trades": stats.TradesExecuted,
				"volume": stats.TotalVolume, "last_price": stats.LastPrice,
			},
		}

	default:
		return nerve.TaskResult{Status: "failed", Error: fmt.Sprintf("unknown exchange action: %s", req.Action)}
	}
}

func federationWorker(req nerve.TaskRequest) nerve.TaskResult {
	eng := federation.GetEngine()
	if eng == nil {
		return nerve.TaskResult{Status: "failed", Error: "federation engine not initialized"}
	}

	switch req.Action {
	case "register_swarm":
		id, _ := req.Params["id"].(string)
		name, _ := req.Params["name"].(string)
		endpoint, _ := req.Params["endpoint"].(string)
		region, _ := req.Params["region"].(string)
		s := eng.RegisterSwarm(id, name, endpoint, region, nil, 1, 0, nil)
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"swarm_id": s.ID, "trust": string(s.Trust)},
		}

	case "init_handshake":
		target, _ := req.Params["target_swarm"].(string)
		hs, err := eng.InitHandshake(target)
		if err != nil {
			return nerve.TaskResult{Status: "failed", Error: err.Error()}
		}
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"handshake_id": hs.ID, "challenge": hs.Challenge},
		}

	case "propose_route":
		target, _ := req.Params["target_swarm"].(string)
		taskType, _ := req.Params["task_type"].(string)
		desc, _ := req.Params["description"].(string)
		pri, _ := req.Params["priority"].(string)
		bid, _ := req.Params["bid"].(float64)
		route, err := eng.ProposeRoute(target, taskType, desc, pri, nil, bid)
		if err != nil {
			return nerve.TaskResult{Status: "failed", Error: err.Error()}
		}
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"route_id": route.ID, "status": string(route.Status)},
		}

	case "find_best_swarm":
		cap, _ := req.Params["capability"].(string)
		best := eng.FindBestSwarm(cap)
		if best == nil {
			return nerve.TaskResult{Status: "completed", Output: map[string]interface{}{"found": false}}
		}
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"found": true, "swarm_id": best.ID, "reputation": best.Reputation},
		}

	default:
		return nerve.TaskResult{Status: "failed", Error: fmt.Sprintf("unknown federation action: %s", req.Action)}
	}
}

func swarmctlWorker(req nerve.TaskRequest) nerve.TaskResult {
	eng := swarmctl.GetEngine()
	if eng == nil {
		return nerve.TaskResult{Status: "failed", Error: "swarmctl engine not initialized"}
	}

	switch req.Action {
	case "register_unit":
		name, _ := req.Params["name"].(string)
		unitType, _ := req.Params["unit_type"].(string)
		domain, _ := req.Params["domain"].(string)
		unit := eng.RegisterUnit(name, unitType, swarmctl.Domain(domain), swarmctl.Position{}, nil, 100, 100, 0, 0, nil)
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"unit_id": unit.ID, "domain": string(unit.Domain)},
		}

	case "create_mission":
		name, _ := req.Params["name"].(string)
		mtype, _ := req.Params["type"].(string)
		pri, _ := req.Params["priority"].(string)
		if pri == "" {
			pri = "P2"
		}
		m, err := eng.CreateMission(name, mtype, pri, nil, nil, nil, nil, swarmctl.MissionConstraints{}, nil)
		if err != nil {
			return nerve.TaskResult{Status: "failed", Error: err.Error()}
		}
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{"mission_id": m.ID, "status": string(m.Status)},
		}

	case "get_stats":
		stats := eng.Stats()
		return nerve.TaskResult{
			Status: "completed",
			Output: map[string]interface{}{
				"units": stats.UnitsTotal, "missions_active": stats.MissionsActive,
			},
		}

	default:
		return nerve.TaskResult{Status: "failed", Error: fmt.Sprintf("unknown swarmctl action: %s", req.Action)}
	}
}
