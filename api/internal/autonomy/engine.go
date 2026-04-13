package autonomy

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/nerve"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════
// Autonomy Engine — 虫群自治引擎 (Phase 5A)
//
// 三大子系统:
//   1. Autonomy Controller — 4级自治等级控制
//   2. Decision Engine — 基于规则的自主决策循环
//   3. Self-Evolution — 性能自审计 + 瓶颈自诊断 + 改进提案
//
// 自治等级:
//   L0 Manual     — 所有操作需人工确认
//   L1 Advisory   — 系统建议，人工批准
//   L2 Supervised — 自动执行，人工可否决 (30s窗口)
//   L3 Autonomous — 完全自主，仅事后通知
//
// 安全护栏:
//   - 每级都有操作白名单和黑名单
//   - L3 不允许执行 dangerous 类操作
//   - 降级触发器: 连续3次失败 → 自动降一级
//   - 升级需要: 连续成功 + 最低运行时间 + 人工审批(L2→L3)
// ════════════════════════════════════════════════════════════

// ── Autonomy Level ──

type Level int

const (
	LevelManual     Level = 0 // 人工操作
	LevelAdvisory   Level = 1 // 建议模式
	LevelSupervised Level = 2 // 监督执行
	LevelAutonomous Level = 3 // 完全自治
)

func (l Level) String() string {
	switch l {
	case LevelManual:
		return "manual"
	case LevelAdvisory:
		return "advisory"
	case LevelSupervised:
		return "supervised"
	case LevelAutonomous:
		return "autonomous"
	default:
		return "unknown"
	}
}

// ── Decision ──

type DecisionStatus string

const (
	DecisionPending  DecisionStatus = "pending"
	DecisionApproved DecisionStatus = "approved"
	DecisionRejected DecisionStatus = "rejected"
	DecisionExecuted DecisionStatus = "executed"
	DecisionFailed   DecisionStatus = "failed"
	DecisionVetoed   DecisionStatus = "vetoed"
)

type Decision struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`    // health_action, scale_action, hotfix_action, evolution_action
	Trigger     string                 `json:"trigger"` // what triggered this decision
	Description string                 `json:"description"`
	Level       Level                  `json:"level"` // autonomy level when decision was made
	Status      DecisionStatus         `json:"status"`
	Action      ProposedAction         `json:"action"`
	Reasoning   string                 `json:"reasoning"`   // why this decision was made
	Risk        string                 `json:"risk"`        // low, medium, high, critical
	VetoWindow  time.Duration          `json:"veto_window"` // time for human to veto (L2)
	Result      map[string]interface{} `json:"result,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	ExecutedAt  *time.Time             `json:"executed_at,omitempty"`
	ApprovedBy  string                 `json:"approved_by,omitempty"` // "auto" or human ID
}

type ProposedAction struct {
	WorkerType string                 `json:"worker_type"`
	Action     string                 `json:"action"`
	Params     map[string]interface{} `json:"params"`
	Priority   string                 `json:"priority"`
}

// ── Decision Rule ──

type RuleCondition struct {
	Metric    string  `json:"metric"`   // e.g. "health.degraded_count", "alerts.critical_count"
	Operator  string  `json:"operator"` // gt, lt, eq, gte, lte
	Threshold float64 `json:"threshold"`
}

type DecisionRule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
	Conditions  []RuleCondition `json:"conditions"` // all must match (AND)
	MinLevel    Level           `json:"min_level"`  // minimum autonomy level to fire
	Action      ProposedAction  `json:"action"`
	Risk        string          `json:"risk"`
	Cooldown    time.Duration   `json:"cooldown"` // min time between firings
	LastFired   *time.Time      `json:"last_fired,omitempty"`
}

// ── Self-Evolution ──

type EvolutionInsight struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"` // performance, reliability, efficiency, capability
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"` // info, warning, critical
	Suggestion  string    `json:"suggestion"`
	AutoFix     bool      `json:"auto_fix"` // can be auto-resolved
	Status      string    `json:"status"`   // detected, proposed, accepted, applied, dismissed
	CreatedAt   time.Time `json:"created_at"`
}

type PerformanceSnapshot struct {
	Timestamp          time.Time              `json:"timestamp"`
	NerveStats         map[string]interface{} `json:"nerve_stats"`
	DecisionCount      int                    `json:"decision_count"`
	SuccessRate        float64                `json:"success_rate"`
	AvgResponseMs      float64                `json:"avg_response_ms"`
	ActiveWorkers      int                    `json:"active_workers"`
	AutonomyLevel      Level                  `json:"autonomy_level"`
	ConsecutiveSuccess int                    `json:"consecutive_success"`
	ConsecutiveFail    int                    `json:"consecutive_fail"`
}

// ── Engine ──

type EngineConfig struct {
	InitialLevel       Level         `json:"initial_level"`
	VetoWindow         time.Duration `json:"veto_window"`         // L2 veto window
	DegradeThreshold   int           `json:"degrade_threshold"`   // consecutive fails to auto-degrade
	PromoteMinSuccess  int           `json:"promote_min_success"` // consecutive successes to propose upgrade
	PromoteMinUptime   time.Duration `json:"promote_min_uptime"`  // min time at current level before upgrade
	SnapshotInterval   time.Duration `json:"snapshot_interval"`   // performance snapshot interval
	MaxDecisionHistory int           `json:"max_decision_history"`
}

func DefaultConfig() *EngineConfig {
	return &EngineConfig{
		InitialLevel:       LevelAdvisory,
		VetoWindow:         30 * time.Second,
		DegradeThreshold:   3,
		PromoteMinSuccess:  10,
		PromoteMinUptime:   1 * time.Hour,
		SnapshotInterval:   5 * time.Minute,
		MaxDecisionHistory: 500,
	}
}

type Engine struct {
	mu              sync.RWMutex
	db              *gorm.DB
	nodeID          string
	config          *EngineConfig
	level           Level
	levelChangedAt  time.Time
	decisions       []Decision
	rules           map[string]*DecisionRule
	insights        []EvolutionInsight
	snapshots       []PerformanceSnapshot
	stats           EngineStats
	consecutiveOK   int
	consecutiveFail int
	startAt         time.Time
	nextID          int

	// Domain-specific allow/deny per level
	levelAllowActions map[Level]map[string]bool // level → "worker:action" → allowed
	levelDenyActions  map[Level]map[string]bool
}

type EngineStats struct {
	CurrentLevel       string    `json:"current_level"`
	DecisionsMade      int       `json:"decisions_made"`
	DecisionsExecuted  int       `json:"decisions_executed"`
	DecisionsFailed    int       `json:"decisions_failed"`
	DecisionsVetoed    int       `json:"decisions_vetoed"`
	AutoUpgrades       int       `json:"auto_upgrades"`
	AutoDegrades       int       `json:"auto_degrades"`
	RulesActive        int       `json:"rules_active"`
	InsightsGenerated  int       `json:"insights_generated"`
	ConsecutiveSuccess int       `json:"consecutive_success"`
	ConsecutiveFail    int       `json:"consecutive_fail"`
	Uptime             string    `json:"uptime"`
	LevelSince         time.Time `json:"level_since"`
	LastDecision       time.Time `json:"last_decision,omitempty"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(nodeID string, cfg *EngineConfig, db *gorm.DB) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	engineOnce.Do(func() {
		globalEngine = &Engine{
			db:                db,
			nodeID:            nodeID,
			config:            cfg,
			level:             cfg.InitialLevel,
			levelChangedAt:    time.Now(),
			decisions:         make([]Decision, 0, cfg.MaxDecisionHistory),
			rules:             make(map[string]*DecisionRule),
			insights:          make([]EvolutionInsight, 0),
			snapshots:         make([]PerformanceSnapshot, 0),
			startAt:           time.Now(),
			levelAllowActions: buildDefaultAllowActions(),
			levelDenyActions:  buildDefaultDenyActions(),
		}
		globalEngine.loadFromDB()
		if len(globalEngine.rules) == 0 {
			globalEngine.seedDefaultRules()
		}
		log.Printf("[autonomy] engine initialized (level=%s, node=%s, db=%v)", cfg.InitialLevel, nodeID, db != nil)
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), e.nextID)
}

// ── Level Management ──

func (e *Engine) GetLevel() Level {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.level
}

func (e *Engine) SetLevel(level Level, approvedBy string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	old := e.level
	e.level = level
	e.levelChangedAt = time.Now()
	e.consecutiveOK = 0
	e.consecutiveFail = 0
	log.Printf("[autonomy] level changed: %s → %s (by %s)", old, level, approvedBy)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("autonomy.level.changed", "autonomy", map[string]interface{}{
			"old_level": old.String(),
			"new_level": level.String(),
			"by":        approvedBy,
		})
	}
}

func (e *Engine) checkAutoDegrade() {
	if e.consecutiveFail >= e.config.DegradeThreshold && e.level > LevelManual {
		old := e.level
		e.level--
		e.levelChangedAt = time.Now()
		e.consecutiveFail = 0
		e.stats.AutoDegrades++
		log.Printf("[autonomy] AUTO-DEGRADE: %s → %s (consecutive failures=%d)", old, e.level, e.config.DegradeThreshold)

		if bus := nerve.GetBus(); bus != nil {
			bus.Publish("autonomy.auto_degrade", "autonomy", map[string]interface{}{
				"old_level": old.String(),
				"new_level": e.level.String(),
				"reason":    "consecutive_failures",
			})
		}
	}
}

func (e *Engine) checkAutoPromote() {
	if e.consecutiveOK >= e.config.PromoteMinSuccess &&
		e.level < LevelAutonomous &&
		time.Since(e.levelChangedAt) >= e.config.PromoteMinUptime {

		if e.level == LevelSupervised {
			// L2→L3 requires human approval — generate insight instead
			e.insights = append(e.insights, EvolutionInsight{
				ID:          e.genID("insight"),
				Category:    "capability",
				Title:       "Autonomy Level Upgrade Candidate",
				Description: fmt.Sprintf("System has %d consecutive successes over %v at L2. Ready for L3 autonomous.", e.consecutiveOK, time.Since(e.levelChangedAt).Round(time.Minute)),
				Severity:    "info",
				Suggestion:  "Consider upgrading to L3 Autonomous via POST /v1/autonomy/level",
				Status:      "proposed",
				CreatedAt:   time.Now(),
			})
			e.stats.InsightsGenerated++
			return
		}

		old := e.level
		e.level++
		e.levelChangedAt = time.Now()
		e.consecutiveOK = 0
		e.stats.AutoUpgrades++
		log.Printf("[autonomy] AUTO-PROMOTE: %s → %s (consecutive_ok=%d)", old, e.level, e.config.PromoteMinSuccess)

		if bus := nerve.GetBus(); bus != nil {
			bus.Publish("autonomy.auto_promote", "autonomy", map[string]interface{}{
				"old_level": old.String(),
				"new_level": e.level.String(),
				"reason":    "consecutive_success",
			})
		}
	}
}

// ── Decision Making ──

func (e *Engine) ProposeDecision(dtype, trigger, description, reasoning, risk string, action ProposedAction) *Decision {
	e.mu.Lock()
	defer e.mu.Unlock()

	d := Decision{
		ID:          e.genID("dec"),
		Type:        dtype,
		Trigger:     trigger,
		Description: description,
		Level:       e.level,
		Status:      DecisionPending,
		Action:      action,
		Reasoning:   reasoning,
		Risk:        risk,
		VetoWindow:  e.config.VetoWindow,
		CreatedAt:   time.Now(),
	}

	// Check if action is allowed at current level
	actionKey := action.WorkerType + ":" + action.Action
	if denied, ok := e.levelDenyActions[e.level]; ok && denied[actionKey] {
		d.Status = DecisionRejected
		d.Reasoning += " [BLOCKED: action denied at current autonomy level]"
		e.appendDecision(d)
		return &d
	}

	switch e.level {
	case LevelManual:
		d.Status = DecisionPending
		d.Reasoning += " [MANUAL: awaiting human approval]"

	case LevelAdvisory:
		d.Status = DecisionPending
		d.Reasoning += " [ADVISORY: recommendation generated, awaiting approval]"

	case LevelSupervised:
		if risk == "critical" || risk == "high" {
			d.Status = DecisionPending
			d.Reasoning += " [SUPERVISED: high-risk, awaiting approval]"
		} else {
			d.Status = DecisionApproved
			d.ApprovedBy = "auto_supervised"
		}

	case LevelAutonomous:
		if risk == "critical" {
			d.Status = DecisionPending
			d.Reasoning += " [AUTONOMOUS: critical risk still requires approval]"
		} else {
			d.Status = DecisionApproved
			d.ApprovedBy = "auto_autonomous"
		}
	}

	e.appendDecision(d)
	e.stats.DecisionsMade++
	e.stats.LastDecision = d.CreatedAt

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("autonomy.decision.proposed", "autonomy", map[string]interface{}{
			"decision_id": d.ID,
			"type":        d.Type,
			"status":      string(d.Status),
			"risk":        d.Risk,
			"level":       d.Level.String(),
		})
	}

	// Auto-execute if approved
	if d.Status == DecisionApproved {
		go e.executeDecision(d.ID)
	}

	return &d
}

func (e *Engine) ApproveDecision(id, approvedBy string) (*Decision, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.decisions {
		if e.decisions[i].ID == id {
			if e.decisions[i].Status != DecisionPending {
				return nil, fmt.Errorf("decision %s is not pending (status=%s)", id, e.decisions[i].Status)
			}
			e.decisions[i].Status = DecisionApproved
			e.decisions[i].ApprovedBy = approvedBy
			d := e.decisions[i]
			go e.updateDecisionStatus(&d)
			go e.executeDecision(id)
			return &d, nil
		}
	}
	return nil, fmt.Errorf("decision %s not found", id)
}

func (e *Engine) VetoDecision(id, vetoedBy string) (*Decision, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.decisions {
		if e.decisions[i].ID == id {
			if e.decisions[i].Status != DecisionPending && e.decisions[i].Status != DecisionApproved {
				return nil, fmt.Errorf("decision %s cannot be vetoed (status=%s)", id, e.decisions[i].Status)
			}
			e.decisions[i].Status = DecisionVetoed
			e.decisions[i].ApprovedBy = "vetoed_by:" + vetoedBy
			e.stats.DecisionsVetoed++
			d := e.decisions[i]
			go e.updateDecisionStatus(&d)
			return &d, nil
		}
	}
	return nil, fmt.Errorf("decision %s not found", id)
}

func (e *Engine) executeDecision(id string) {
	bus := nerve.GetBus()
	if bus == nil {
		return
	}

	e.mu.RLock()
	var d *Decision
	for i := range e.decisions {
		if e.decisions[i].ID == id {
			d = &e.decisions[i]
			break
		}
	}
	if d == nil || d.Status != DecisionApproved {
		e.mu.RUnlock()
		return
	}
	action := d.Action
	e.mu.RUnlock()

	result, err := bus.Dispatch(nerve.TaskRequest{
		WorkerType:  action.WorkerType,
		Action:      action.Action,
		Params:      action.Params,
		Priority:    action.Priority,
		RequestedBy: "autonomy_engine",
		Timeout:     30 * time.Second,
	})

	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.decisions {
		if e.decisions[i].ID == id {
			now := time.Now()
			e.decisions[i].ExecutedAt = &now
			if err != nil || (result != nil && result.Status != "completed") {
				e.decisions[i].Status = DecisionFailed
				if err != nil {
					e.decisions[i].Result = map[string]interface{}{"error": err.Error()}
				} else {
					e.decisions[i].Result = result.Output
				}
				e.stats.DecisionsFailed++
				e.consecutiveFail++
				e.consecutiveOK = 0
				e.checkAutoDegrade()
			} else {
				e.decisions[i].Status = DecisionExecuted
				e.decisions[i].Result = result.Output
				e.stats.DecisionsExecuted++
				e.consecutiveOK++
				e.consecutiveFail = 0
				e.checkAutoPromote()
			}
			d := e.decisions[i]
			go e.updateDecisionStatus(&d)
			break
		}
	}
}

// ── Rules ──

func (e *Engine) AddRule(name, description string, conditions []RuleCondition, minLevel Level, action ProposedAction, risk string, cooldown time.Duration) *DecisionRule {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule := &DecisionRule{
		ID:          e.genID("rule"),
		Name:        name,
		Description: description,
		Enabled:     true,
		Conditions:  conditions,
		MinLevel:    minLevel,
		Action:      action,
		Risk:        risk,
		Cooldown:    cooldown,
	}
	e.rules[rule.ID] = rule
	e.stats.RulesActive++
	go e.persistRule(rule)
	log.Printf("[autonomy] rule added: %s — %s", rule.ID, name)
	return rule
}

func (e *Engine) ListRules() []*DecisionRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rules := make([]*DecisionRule, 0, len(e.rules))
	for _, r := range e.rules {
		rules = append(rules, r)
	}
	return rules
}

func (e *Engine) ToggleRule(id string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("rule %s not found", id)
	}
	r.Enabled = enabled
	go e.updateRuleEnabled(id, enabled)
	return nil
}

// EvaluateRules checks all rules against current metrics and proposes decisions.
// Called periodically or on metric change.
func (e *Engine) EvaluateRules(metrics map[string]float64) []string {
	e.mu.RLock()
	currentLevel := e.level
	var toFire []*DecisionRule
	for _, r := range e.rules {
		if !r.Enabled || r.MinLevel > currentLevel {
			continue
		}
		if r.LastFired != nil && time.Since(*r.LastFired) < r.Cooldown {
			continue
		}
		if matchConditions(r.Conditions, metrics) {
			toFire = append(toFire, r)
		}
	}
	e.mu.RUnlock()

	var decisionIDs []string
	for _, r := range toFire {
		d := e.ProposeDecision(
			"rule_triggered",
			r.Name,
			r.Description,
			fmt.Sprintf("Rule %q conditions met against current metrics", r.Name),
			r.Risk,
			r.Action,
		)
		decisionIDs = append(decisionIDs, d.ID)
		e.mu.Lock()
		now := time.Now()
		if rule, ok := e.rules[r.ID]; ok {
			rule.LastFired = &now
		}
		e.mu.Unlock()
	}
	return decisionIDs
}

func matchConditions(conditions []RuleCondition, metrics map[string]float64) bool {
	for _, c := range conditions {
		val, ok := metrics[c.Metric]
		if !ok {
			return false
		}
		switch c.Operator {
		case "gt":
			if val <= c.Threshold {
				return false
			}
		case "lt":
			if val >= c.Threshold {
				return false
			}
		case "eq":
			if val != c.Threshold {
				return false
			}
		case "gte":
			if val < c.Threshold {
				return false
			}
		case "lte":
			if val > c.Threshold {
				return false
			}
		}
	}
	return true
}

// ── Self-Evolution ──

func (e *Engine) TakeSnapshot() *PerformanceSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	var nerveStats map[string]interface{}
	if bus := nerve.GetBus(); bus != nil {
		s := bus.Stats()
		nerveStats = map[string]interface{}{
			"events_published": s.EventsPublished,
			"tasks_dispatched": s.TasksDispatched,
			"tasks_completed":  s.TasksCompleted,
			"tasks_failed":     s.TasksFailed,
		}
	}

	successRate := float64(0)
	if e.stats.DecisionsExecuted+e.stats.DecisionsFailed > 0 {
		successRate = float64(e.stats.DecisionsExecuted) / float64(e.stats.DecisionsExecuted+e.stats.DecisionsFailed) * 100
	}

	snap := PerformanceSnapshot{
		Timestamp:          time.Now(),
		NerveStats:         nerveStats,
		DecisionCount:      e.stats.DecisionsMade,
		SuccessRate:        successRate,
		AutonomyLevel:      e.level,
		ConsecutiveSuccess: e.consecutiveOK,
		ConsecutiveFail:    e.consecutiveFail,
	}

	if bus := nerve.GetBus(); bus != nil {
		snap.ActiveWorkers = len(bus.RegisteredWorkers())
	}

	e.snapshots = append(e.snapshots, snap)
	if len(e.snapshots) > 200 {
		e.snapshots = e.snapshots[1:]
	}
	go e.persistSnapshot(&snap)

	return &snap
}

func (e *Engine) Diagnose() []EvolutionInsight {
	e.mu.Lock()
	defer e.mu.Unlock()

	var newInsights []EvolutionInsight

	// Check success rate
	total := e.stats.DecisionsExecuted + e.stats.DecisionsFailed
	if total > 5 {
		rate := float64(e.stats.DecisionsExecuted) / float64(total) * 100
		if rate < 70 {
			newInsights = append(newInsights, EvolutionInsight{
				ID:          e.genID("insight"),
				Category:    "reliability",
				Title:       "Low Decision Success Rate",
				Description: fmt.Sprintf("Success rate is %.1f%% (%d/%d), below 70%% threshold", rate, e.stats.DecisionsExecuted, total),
				Severity:    "warning",
				Suggestion:  "Review failed decisions, consider tightening rule conditions or reducing autonomy level",
				Status:      "detected",
				CreatedAt:   time.Now(),
			})
		}
	}

	// Check for stale rules (never fired)
	staleCnt := 0
	for _, r := range e.rules {
		if r.Enabled && r.LastFired == nil {
			staleCnt++
		}
	}
	if staleCnt > 3 {
		newInsights = append(newInsights, EvolutionInsight{
			ID:          e.genID("insight"),
			Category:    "efficiency",
			Title:       "Stale Decision Rules",
			Description: fmt.Sprintf("%d enabled rules have never fired — conditions may be too strict or metrics unavailable", staleCnt),
			Severity:    "info",
			Suggestion:  "Review rule conditions or disable unused rules",
			Status:      "detected",
			CreatedAt:   time.Now(),
		})
	}

	// Check if ready for upgrade
	if e.level < LevelAutonomous && e.consecutiveOK >= e.config.PromoteMinSuccess/2 {
		newInsights = append(newInsights, EvolutionInsight{
			ID:          e.genID("insight"),
			Category:    "capability",
			Title:       "Trending Toward Level Upgrade",
			Description: fmt.Sprintf("Currently at %s with %d consecutive successes (need %d for upgrade)", e.level, e.consecutiveOK, e.config.PromoteMinSuccess),
			Severity:    "info",
			Suggestion:  fmt.Sprintf("Continue stable operation to auto-promote to %s", Level(e.level+1)),
			Status:      "detected",
			CreatedAt:   time.Now(),
		})
	}

	for i := range newInsights {
		e.insights = append(e.insights, newInsights[i])
		e.stats.InsightsGenerated++
		ins := newInsights[i]
		go e.persistInsight(&ins)
	}

	return newInsights
}

func (e *Engine) ListInsights(limit int) []EvolutionInsight {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.insights) {
		limit = len(e.insights)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.insights) - limit
	result := make([]EvolutionInsight, limit)
	copy(result, e.insights[start:])
	return result
}

// ── Query ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.CurrentLevel = e.level.String()
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	s.LevelSince = e.levelChangedAt
	s.ConsecutiveSuccess = e.consecutiveOK
	s.ConsecutiveFail = e.consecutiveFail
	activeRules := 0
	for _, r := range e.rules {
		if r.Enabled {
			activeRules++
		}
	}
	s.RulesActive = activeRules
	return &s
}

func (e *Engine) ListDecisions(limit int) []Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.decisions) {
		limit = len(e.decisions)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.decisions) - limit
	result := make([]Decision, limit)
	copy(result, e.decisions[start:])
	return result
}

func (e *Engine) GetDecision(id string) *Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for i := range e.decisions {
		if e.decisions[i].ID == id {
			d := e.decisions[i]
			return &d
		}
	}
	return nil
}

func (e *Engine) ListSnapshots(limit int) []PerformanceSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.snapshots) {
		limit = len(e.snapshots)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.snapshots) - limit
	result := make([]PerformanceSnapshot, limit)
	copy(result, e.snapshots[start:])
	return result
}

// ── Internal ──

func (e *Engine) appendDecision(d Decision) {
	e.decisions = append(e.decisions, d)
	if len(e.decisions) > e.config.MaxDecisionHistory {
		e.decisions = e.decisions[1:]
	}
	go e.persistDecision(&d)
}

func buildDefaultAllowActions() map[Level]map[string]bool {
	return map[Level]map[string]bool{
		LevelManual:     {},
		LevelAdvisory:   {},
		LevelSupervised: {"sense_claw:run_health_check": true, "test_claw:run_smoke_test": true, "sense_claw:generate_insight": true},
		LevelAutonomous: {"sense_claw:run_health_check": true, "test_claw:run_smoke_test": true, "test_claw:run_regression": true, "ops_claw:deploy": true, "dev_team:build": true},
	}
}

func buildDefaultDenyActions() map[Level]map[string]bool {
	return map[Level]map[string]bool{
		LevelManual:     {},
		LevelAdvisory:   {},
		LevelSupervised: {},
		LevelAutonomous: {}, // even L3 can't do certain things in the future (e.g. delete_production_data)
	}
}

func (e *Engine) seedDefaultRules() {
	e.rules["rule-health-degrade"] = &DecisionRule{
		ID:          "rule-health-degrade",
		Name:        "Health Degradation Response",
		Description: "Auto health-check when degraded services detected",
		Enabled:     true,
		Conditions: []RuleCondition{
			{Metric: "health.degraded_count", Operator: "gt", Threshold: 0},
		},
		MinLevel: LevelAdvisory,
		Action: ProposedAction{
			WorkerType: "sense_claw",
			Action:     "run_health_check",
			Priority:   "P1",
		},
		Risk:     "low",
		Cooldown: 5 * time.Minute,
	}

	e.rules["rule-critical-alert"] = &DecisionRule{
		ID:          "rule-critical-alert",
		Name:        "Critical Alert Triage",
		Description: "Auto-triage critical alerts by running smoke tests",
		Enabled:     true,
		Conditions: []RuleCondition{
			{Metric: "alerts.critical_count", Operator: "gt", Threshold: 0},
		},
		MinLevel: LevelSupervised,
		Action: ProposedAction{
			WorkerType: "test_claw",
			Action:     "run_smoke_test",
			Priority:   "P0",
		},
		Risk:     "medium",
		Cooldown: 10 * time.Minute,
	}

	e.rules["rule-high-failure"] = &DecisionRule{
		ID:          "rule-high-failure",
		Name:        "High Failure Rate Diagnosis",
		Description: "Generate insight when task failure rate is high",
		Enabled:     true,
		Conditions: []RuleCondition{
			{Metric: "nerve.failure_rate", Operator: "gt", Threshold: 30},
		},
		MinLevel: LevelAdvisory,
		Action: ProposedAction{
			WorkerType: "sense_claw",
			Action:     "generate_insight",
			Priority:   "P2",
		},
		Risk:     "low",
		Cooldown: 15 * time.Minute,
	}

	log.Printf("[autonomy] seeded %d default decision rules", len(e.rules))
}
