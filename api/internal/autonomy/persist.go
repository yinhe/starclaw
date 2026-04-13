package autonomy

import (
	"encoding/json"
	"log"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// persistDecision saves a decision to DB (fire-and-forget).
func (e *Engine) persistDecision(d *Decision) {
	if e.db == nil {
		return
	}
	params, _ := json.Marshal(d.Action.Params)
	result, _ := json.Marshal(d.Result)
	rec := model.AutonomyDecision{
		DecisionID:   d.ID,
		NodeID:       e.nodeID,
		Type:         d.Type,
		Trigger:      d.Trigger,
		Description:  d.Description,
		Level:        int(d.Level),
		Status:       string(d.Status),
		Risk:         d.Risk,
		Reasoning:    d.Reasoning,
		WorkerType:   d.Action.WorkerType,
		Action:       d.Action.Action,
		ActionParams: string(params),
		Priority:     d.Action.Priority,
		Result:       string(result),
		ApprovedBy:   d.ApprovedBy,
		ExecutedAt:   d.ExecutedAt,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[autonomy] persist decision %s failed: %v", d.ID, err)
	}
}

// updateDecisionStatus updates an existing decision record in DB.
func (e *Engine) updateDecisionStatus(d *Decision) {
	if e.db == nil {
		return
	}
	result, _ := json.Marshal(d.Result)
	e.db.Model(&model.AutonomyDecision{}).
		Where("decision_id = ?", d.ID).
		Updates(map[string]interface{}{
			"status":      string(d.Status),
			"approved_by": d.ApprovedBy,
			"executed_at": d.ExecutedAt,
			"result":      string(result),
		})
}

// persistRule saves or updates a rule to DB.
func (e *Engine) persistRule(r *DecisionRule) {
	if e.db == nil {
		return
	}
	conds, _ := json.Marshal(r.Conditions)
	params, _ := json.Marshal(r.Action.Params)
	rec := model.AutonomyRule{
		RuleID:       r.ID,
		NodeID:       e.nodeID,
		Name:         r.Name,
		Description:  r.Description,
		Enabled:      r.Enabled,
		MinLevel:     int(r.MinLevel),
		Risk:         r.Risk,
		Conditions:   string(conds),
		WorkerType:   r.Action.WorkerType,
		Action:       r.Action.Action,
		ActionParams: string(params),
		Priority:     r.Action.Priority,
		CooldownSec:  int(r.Cooldown.Seconds()),
		LastFired:    r.LastFired,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[autonomy] persist rule %s failed: %v", r.ID, err)
	}
}

// updateRuleEnabled updates the enabled flag in DB.
func (e *Engine) updateRuleEnabled(id string, enabled bool) {
	if e.db == nil {
		return
	}
	e.db.Model(&model.AutonomyRule{}).Where("rule_id = ?", id).Update("enabled", enabled)
}

// persistInsight saves an insight to DB.
func (e *Engine) persistInsight(ins *EvolutionInsight) {
	if e.db == nil {
		return
	}
	rec := model.AutonomyInsight{
		InsightID:   ins.ID,
		NodeID:      e.nodeID,
		Category:    ins.Category,
		Title:       ins.Title,
		Description: ins.Description,
		Severity:    ins.Severity,
		Suggestion:  ins.Suggestion,
		AutoFix:     ins.AutoFix,
		Status:      ins.Status,
	}
	if err := e.db.Create(&rec).Error; err != nil {
		log.Printf("[autonomy] persist insight %s failed: %v", ins.ID, err)
	}
}

// persistSnapshot saves a performance snapshot to DB.
func (e *Engine) persistSnapshot(snap *PerformanceSnapshot) {
	if e.db == nil {
		return
	}
	ns, _ := json.Marshal(snap.NerveStats)
	rec := model.AutonomySnapshot{
		NodeID:             e.nodeID,
		DecisionCount:      snap.DecisionCount,
		SuccessRate:        snap.SuccessRate,
		AvgResponseMs:      snap.AvgResponseMs,
		ActiveWorkers:      snap.ActiveWorkers,
		AutonomyLevel:      int(snap.AutonomyLevel),
		ConsecutiveSuccess: snap.ConsecutiveSuccess,
		ConsecutiveFail:    snap.ConsecutiveFail,
		NerveStats:         string(ns),
	}
	if err := e.db.Create(&rec).Error; err != nil {
		log.Printf("[autonomy] persist snapshot failed: %v", err)
	}
}

// loadFromDB restores decisions, rules, and insights from DB on startup.
func (e *Engine) loadFromDB() {
	if e.db == nil {
		return
	}

	// Load decisions (latest N)
	var decRecs []model.AutonomyDecision
	e.db.Where("node_id = ?", e.nodeID).Order("created_at desc").Limit(e.config.MaxDecisionHistory).Find(&decRecs)
	for i := len(decRecs) - 1; i >= 0; i-- {
		r := decRecs[i]
		var params map[string]interface{}
		json.Unmarshal([]byte(r.ActionParams), &params)
		var result map[string]interface{}
		json.Unmarshal([]byte(r.Result), &result)
		e.decisions = append(e.decisions, Decision{
			ID:          r.DecisionID,
			Type:        r.Type,
			Trigger:     r.Trigger,
			Description: r.Description,
			Level:       Level(r.Level),
			Status:      DecisionStatus(r.Status),
			Action: ProposedAction{
				WorkerType: r.WorkerType,
				Action:     r.Action,
				Params:     params,
				Priority:   r.Priority,
			},
			Reasoning:  r.Reasoning,
			Risk:       r.Risk,
			Result:     result,
			ApprovedBy: r.ApprovedBy,
			CreatedAt:  r.CreatedAt,
			ExecutedAt: r.ExecutedAt,
		})
	}

	// Load rules
	var ruleRecs []model.AutonomyRule
	e.db.Where("node_id = ?", e.nodeID).Find(&ruleRecs)
	for _, r := range ruleRecs {
		var conds []RuleCondition
		json.Unmarshal([]byte(r.Conditions), &conds)
		var params map[string]interface{}
		json.Unmarshal([]byte(r.ActionParams), &params)
		e.rules[r.RuleID] = &DecisionRule{
			ID:          r.RuleID,
			Name:        r.Name,
			Description: r.Description,
			Enabled:     r.Enabled,
			Conditions:  conds,
			MinLevel:    Level(r.MinLevel),
			Action: ProposedAction{
				WorkerType: r.WorkerType,
				Action:     r.Action,
				Params:     params,
				Priority:   r.Priority,
			},
			Risk:      r.Risk,
			Cooldown:  time.Duration(r.CooldownSec) * time.Second,
			LastFired: r.LastFired,
		}
	}

	// Load insights (latest 100)
	var insRecs []model.AutonomyInsight
	e.db.Where("node_id = ?", e.nodeID).Order("created_at desc").Limit(100).Find(&insRecs)
	for i := len(insRecs) - 1; i >= 0; i-- {
		r := insRecs[i]
		e.insights = append(e.insights, EvolutionInsight{
			ID:          r.InsightID,
			Category:    r.Category,
			Title:       r.Title,
			Description: r.Description,
			Severity:    r.Severity,
			Suggestion:  r.Suggestion,
			AutoFix:     r.AutoFix,
			Status:      r.Status,
			CreatedAt:   r.CreatedAt,
		})
	}

	// Recompute stats from loaded data
	for _, d := range e.decisions {
		e.stats.DecisionsMade++
		switch d.Status {
		case DecisionExecuted:
			e.stats.DecisionsExecuted++
		case DecisionFailed:
			e.stats.DecisionsFailed++
		case DecisionVetoed:
			e.stats.DecisionsVetoed++
		}
	}
	e.stats.InsightsGenerated = len(e.insights)

	if len(decRecs)+len(ruleRecs)+len(insRecs) > 0 {
		log.Printf("[autonomy] restored from DB: %d decisions, %d rules, %d insights",
			len(decRecs), len(ruleRecs), len(insRecs))
	}
}

// SetDB sets the database connection (can be called after init).
func (e *Engine) SetDB(db *gorm.DB) {
	e.db = db
}
