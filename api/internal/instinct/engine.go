package instinct

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/worker"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Engine is the Instinct engine — it evaluates activities and creates tasks when conditions are met.
// Runs as a background goroutine alongside the TaskWorker.
type Engine struct {
	db     *gorm.DB
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewEngine creates a new Instinct engine.
func NewEngine(db *gorm.DB) *Engine {
	return &Engine{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start begins the instinct evaluation loop.
func (e *Engine) Start() {
	log.Println("[Instinct] Engine starting...")
	e.wg.Add(1)
	go e.loop()
}

// Stop gracefully stops the engine.
func (e *Engine) Stop() {
	log.Println("[Instinct] Engine stopping...")
	close(e.stopCh)
	e.wg.Wait()
	log.Println("[Instinct] Engine stopped")
}

// loop evaluates all enabled activities every 60 seconds.
func (e *Engine) loop() {
	defer e.wg.Done()

	// Wait on startup for DB migration
	select {
	case <-e.stopCh:
		return
	case <-time.After(15 * time.Second):
	}

	log.Println("[Instinct] Engine started — evaluating activities every 60s")

	for {
		select {
		case <-e.stopCh:
			return
		case <-time.After(60 * time.Second):
		}

		e.evaluate()
	}
}

// evaluate scans all enabled activities and fires those whose conditions are met.
func (e *Engine) evaluate() {
	silent := e.db.Session(&gorm.Session{Logger: logger.Discard})

	var activities []model.Activity
	silent.Where("enabled = ?", true).Find(&activities)

	now := time.Now()
	for _, act := range activities {
		if e.shouldFire(act, now) {
			e.fire(act, now)
		}
	}
}

// shouldFire checks if an activity should fire at the given time.
func (e *Engine) shouldFire(act model.Activity, now time.Time) bool {
	trigger := strings.TrimSpace(act.Trigger)

	// Special triggers
	if strings.HasPrefix(trigger, "@") {
		return e.evaluateSpecialTrigger(act, trigger, now)
	}

	// Standard cron trigger
	cron, err := worker.ParseCron(trigger)
	if err != nil {
		return false
	}

	// Check if next_run_at is set and has arrived
	if act.NextRunAt != nil && !act.NextRunAt.IsZero() {
		if act.NextRunAt.After(now) {
			return false // not yet
		}
	} else {
		// Initialize next_run_at
		next := cron.NextAfter(now.Add(-61 * time.Second))
		if !next.IsZero() {
			e.db.Model(&act).Update("next_run_at", &next)
		}
		return false
	}

	// Check cooldown
	if act.LastRunAt != nil {
		cooldown := parseDuration(act.Cooldown, 24*time.Hour)
		if now.Sub(*act.LastRunAt) < cooldown {
			return false
		}
	}

	// Evaluate condition (if any)
	if act.Condition != "" {
		if !e.evaluateCondition(act, now) {
			// Condition not met — advance next_run_at but don't fire
			next := cron.NextAfter(now)
			if !next.IsZero() {
				e.db.Model(&act).Update("next_run_at", &next)
			}
			return false
		}
	}

	return true
}

// evaluateSpecialTrigger handles @idle, @event:xxx triggers.
func (e *Engine) evaluateSpecialTrigger(act model.Activity, trigger string, now time.Time) bool {
	switch {
	case trigger == "@idle":
		// Fire when no tasks are running and cooldown has passed
		var running int64
		e.db.Model(&model.Task{}).Where("status = ?", model.TaskStatusRunning).Count(&running)
		if running > 0 {
			return false
		}
		if act.LastRunAt != nil {
			cooldown := parseDuration(act.Cooldown, 24*time.Hour)
			if now.Sub(*act.LastRunAt) < cooldown {
				return false
			}
		}
		return true

	case strings.HasPrefix(trigger, "@event:"):
		// Event triggers are handled externally via FireEvent()
		return false

	default:
		return false
	}
}

// evaluateCondition checks simple conditions like "user.birthday == today".
// For v1, we support a small set of built-in conditions.
func (e *Engine) evaluateCondition(act model.Activity, now time.Time) bool {
	cond := strings.TrimSpace(act.Condition)

	switch {
	case cond == "user.birthday == today":
		// Check if today matches user's birthday (stored in user profile)
		return e.isBirthday(act.UserID, now)

	case strings.HasPrefix(cond, "weekday =="):
		// "weekday == monday"
		dayStr := strings.TrimSpace(strings.TrimPrefix(cond, "weekday =="))
		return strings.EqualFold(now.Weekday().String(), dayStr)

	case strings.HasPrefix(cond, "day =="):
		// "day == 1" (first of month)
		dayStr := strings.TrimSpace(strings.TrimPrefix(cond, "day =="))
		return fmt.Sprintf("%d", now.Day()) == dayStr

	case cond == "true" || cond == "always":
		return true

	default:
		// Unknown condition — default to true (let the agent handle it)
		return true
	}
}

// isBirthday checks if today matches the user's birthday.
func (e *Engine) isBirthday(userID string, now time.Time) bool {
	// Look up user's birthday from Cerebrate memory or profile
	var mem model.Memory
	err := e.db.Where("user_id = ? AND category = ? AND key LIKE ?",
		userID, "fact", "%birthday%").First(&mem).Error
	if err != nil {
		return false
	}
	// Try to match month-day in the memory content
	todayMMDD := now.Format("01-02")
	return strings.Contains(mem.Content, todayMMDD) ||
		strings.Contains(mem.Content, now.Format("1月2日"))
}

// fire creates a task from an activity and updates its state.
func (e *Engine) fire(act model.Activity, now time.Time) {
	log.Printf("[Instinct] Firing activity %s (%s): %s", act.Name, act.Type, act.Title)

	// Create a task for this activity
	task := model.Task{
		UserID:   act.UserID,
		AgentID:  act.AgentID,
		Title:    fmt.Sprintf("🧬 %s", act.Title),
		Goal:     buildActivityGoal(act),
		Status:   model.TaskStatusPending,
		Priority: model.TaskPriorityLow,
	}

	// If the builtin template specifies ToolsOnly, lock down the task's tool access
	if act.Template != "" {
		for _, tmpl := range BuiltinTemplates() {
			if tmpl.Name == act.Template && len(tmpl.ToolsOnly) > 0 {
				if b, err := json.Marshal(tmpl.ToolsOnly); err == nil {
					task.ToolsOverride = string(b)
					log.Printf("[Instinct] Activity %s locked to tools: %v", act.Name, tmpl.ToolsOnly)
				}
				break
			}
		}
	}

	if err := e.db.Create(&task).Error; err != nil {
		log.Printf("[Instinct] Failed to create task for activity %s: %v", act.ID, err)
		e.db.Model(&act).Updates(map[string]interface{}{
			"consec_fails": act.ConsecFails + 1,
			"updated_at":   now,
		})
		// Auto-disable after 5 consecutive failures
		if act.ConsecFails+1 >= 5 {
			e.db.Model(&act).Update("enabled", false)
			log.Printf("[Instinct] Activity %s auto-disabled after 5 consecutive failures", act.ID)
		}
		return
	}

	// Record activity log
	e.db.Create(&model.ActivityLog{
		ActivityID: act.ID,
		UserID:     act.UserID,
		TaskID:     task.ID,
		Status:     "ok",
	})

	// Advance next_run_at
	trigger := strings.TrimSpace(act.Trigger)
	var nextRun *time.Time
	if !strings.HasPrefix(trigger, "@") {
		if cron, err := worker.ParseCron(trigger); err == nil {
			next := cron.NextAfter(now)
			if !next.IsZero() {
				nextRun = &next
			}
		}
	}

	e.db.Model(&act).Updates(map[string]interface{}{
		"last_run_at":  &now,
		"next_run_at":  nextRun,
		"total_runs":   act.TotalRuns + 1,
		"success_runs": act.SuccessRuns + 1,
		"consec_fails": 0,
		"updated_at":   now,
	})

	log.Printf("[Instinct] Activity %s fired → task %s", act.Name, task.ID)
}

// FireEvent fires all activities matching an event trigger.
// Called externally when an event occurs (e.g. GitHub webhook).
func (e *Engine) FireEvent(eventType string) {
	trigger := "@event:" + eventType
	var activities []model.Activity
	e.db.Where("enabled = ? AND trigger = ?", true, trigger).Find(&activities)

	now := time.Now()
	for _, act := range activities {
		// Check cooldown
		if act.LastRunAt != nil {
			cooldown := parseDuration(act.Cooldown, 24*time.Hour)
			if now.Sub(*act.LastRunAt) < cooldown {
				continue
			}
		}
		e.fire(act, now)
	}
}

// buildActivityGoal constructs the task goal from the activity config.
func buildActivityGoal(act model.Activity) string {
	goal := act.Action
	if act.Channel != "" {
		goal += fmt.Sprintf("\n\n完成后，请通过 %s 渠道发送结果给用户。", act.Channel)
	}
	if act.Config != "" {
		goal += fmt.Sprintf("\n\n活动配置: %s", act.Config)
	}
	return goal
}

// parseDuration parses a Go duration string with a fallback default.
func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}
