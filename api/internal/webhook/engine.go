package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Event Rule — IF condition THEN action
// ════════════════════════════════════════════════════════════════

// EventRule defines a trigger condition and the actions to execute.
type EventRule struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`
	Description string    `json:"description" gorm:"type:text"`
	EventType   string    `json:"event_type" gorm:"type:varchar(100);index;not null"` // agent.error, agent.complete, chat.message, workflow.fail, alert.fired, system.health
	Condition   string    `json:"condition" gorm:"type:json"`                         // JSON: {"field": "error_rate", "operator": "gt", "value": 0.1}
	Actions     string    `json:"actions" gorm:"type:json;not null"`                  // JSON array of ActionConfig
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	RetryCount  int       `json:"retry_count" gorm:"default:3"`
	RetryDelay  int       `json:"retry_delay" gorm:"default:60"` // seconds between retries
	FiredCount  int64     `json:"fired_count" gorm:"default:0"`
	LastFiredAt *time.Time `json:"last_fired_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r *EventRule) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// ActionConfig defines a single action to execute when a rule fires.
type ActionConfig struct {
	Type    string            `json:"type"`    // webhook, email, notification, fallback_model
	URL     string            `json:"url"`     // for webhook
	Method  string            `json:"method"`  // GET, POST
	Headers map[string]string `json:"headers"` // custom headers
	Body    string            `json:"body"`    // template body (supports {{.event}} placeholders)
	To      string            `json:"to"`      // for email
	Subject string            `json:"subject"` // for email
	Message string            `json:"message"` // for notification
}

// ════════════════════════════════════════════════════════════════
//  Event Log — records every event + action result
// ════════════════════════════════════════════════════════════════

// EventLog records when a rule fired and the result of each action.
type EventLog struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RuleID     string    `json:"rule_id" gorm:"type:varchar(36);index;not null"`
	RuleName   string    `json:"rule_name" gorm:"type:varchar(200)"`
	EventType  string    `json:"event_type" gorm:"type:varchar(100);index"`
	EventData  string    `json:"event_data" gorm:"type:json"`
	Status     string    `json:"status" gorm:"type:varchar(20);index;default:pending"` // pending, success, failed, retrying, dead_letter
	ActionType string    `json:"action_type" gorm:"type:varchar(50)"`
	ActionURL  string    `json:"action_url" gorm:"type:varchar(500)"`
	Response   string    `json:"response" gorm:"type:text"`
	StatusCode int       `json:"status_code" gorm:"default:0"`
	Attempts   int       `json:"attempts" gorm:"default:0"`
	MaxRetries int       `json:"max_retries" gorm:"default:3"`
	NextRetry  *time.Time `json:"next_retry"`
	Error      string    `json:"error" gorm:"type:text"`
	CreatedAt  time.Time `json:"created_at" gorm:"index"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (l *EventLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Event — the payload that triggers rules
// ════════════════════════════════════════════════════════════════

// Event is the in-memory event structure emitted by system components.
type Event struct {
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`    // module that emitted
	Timestamp time.Time             `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// ════════════════════════════════════════════════════════════════
//  Webhook Orchestration Engine
// ════════════════════════════════════════════════════════════════

// Engine processes events, evaluates rules, and dispatches actions.
type Engine struct {
	db     *gorm.DB
	httpC  *http.Client
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Event channel for async processing
	eventCh chan Event
}

// NewEngine creates a new webhook orchestration engine.
func NewEngine(db *gorm.DB) *Engine {
	return &Engine{
		db: db,
		httpC: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh:  make(chan struct{}),
		eventCh: make(chan Event, 1000),
	}
}

// Start begins the event processing loop and retry loop.
func (e *Engine) Start() {
	log.Println("[Webhook] Engine starting...")
	e.wg.Add(2)
	go e.processLoop()
	go e.retryLoop()
}

// Stop gracefully shuts down.
func (e *Engine) Stop() {
	log.Println("[Webhook] Engine stopping...")
	close(e.stopCh)
	e.wg.Wait()
	log.Println("[Webhook] Engine stopped")
}

// Emit sends an event into the processing pipeline.
func (e *Engine) Emit(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	select {
	case e.eventCh <- event:
	default:
		log.Printf("[Webhook] Event channel full, dropping event: %s", event.Type)
	}
}

// processLoop continuously processes incoming events.
func (e *Engine) processLoop() {
	defer e.wg.Done()

	// Wait for DB readiness
	select {
	case <-e.stopCh:
		return
	case <-time.After(10 * time.Second):
	}

	log.Println("[Webhook] Event processing loop started")

	for {
		select {
		case <-e.stopCh:
			return
		case event := <-e.eventCh:
			e.handleEvent(event)
		}
	}
}

// handleEvent evaluates all matching rules for an event.
func (e *Engine) handleEvent(event Event) {
	var rules []EventRule
	e.db.Where("event_type = ? AND enabled = ?", event.Type, true).Find(&rules)

	for _, rule := range rules {
		if !e.evaluateCondition(rule.Condition, event) {
			continue
		}
		e.executeActions(rule, event)
	}
}

// evaluateCondition checks if an event matches a rule's condition.
func (e *Engine) evaluateCondition(conditionJSON string, event Event) bool {
	if conditionJSON == "" || conditionJSON == "{}" || conditionJSON == "null" {
		return true // no condition = always match
	}

	var cond struct {
		Field    string      `json:"field"`
		Operator string      `json:"operator"`
		Value    interface{} `json:"value"`
	}
	if err := json.Unmarshal([]byte(conditionJSON), &cond); err != nil {
		return true // malformed condition = match (fail open)
	}

	if cond.Field == "" {
		return true
	}

	// Get field value from event data
	eventVal, ok := event.Data[cond.Field]
	if !ok {
		return false
	}

	// Compare based on operator
	return compareValues(eventVal, cond.Operator, cond.Value)
}

// compareValues compares two values with an operator.
func compareValues(actual interface{}, operator string, expected interface{}) bool {
	// Convert to float64 for numeric comparison
	actualNum, aOk := toFloat64(actual)
	expectedNum, eOk := toFloat64(expected)

	if aOk && eOk {
		switch operator {
		case "gt":
			return actualNum > expectedNum
		case "gte":
			return actualNum >= expectedNum
		case "lt":
			return actualNum < expectedNum
		case "lte":
			return actualNum <= expectedNum
		case "eq":
			return actualNum == expectedNum
		case "neq":
			return actualNum != expectedNum
		}
	}

	// String comparison
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	switch operator {
	case "eq":
		return actualStr == expectedStr
	case "neq":
		return actualStr != expectedStr
	case "contains":
		return strings.Contains(actualStr, expectedStr)
	case "not_contains":
		return !strings.Contains(actualStr, expectedStr)
	case "starts_with":
		return strings.HasPrefix(actualStr, expectedStr)
	case "ends_with":
		return strings.HasSuffix(actualStr, expectedStr)
	default:
		return false
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// executeActions runs all actions for a fired rule.
func (e *Engine) executeActions(rule EventRule, event Event) {
	var actions []ActionConfig
	if err := json.Unmarshal([]byte(rule.Actions), &actions); err != nil {
		log.Printf("[Webhook] Failed to parse actions for rule %s: %v", rule.ID, err)
		return
	}

	// Update rule stats
	now := time.Now()
	e.db.Model(&EventRule{}).Where("id = ?", rule.ID).Updates(map[string]interface{}{
		"fired_count":  gorm.Expr("fired_count + 1"),
		"last_fired_at": now,
	})

	eventDataJSON, _ := json.Marshal(event)

	for _, action := range actions {
		logEntry := EventLog{
			RuleID:     rule.ID,
			RuleName:   rule.Name,
			EventType:  event.Type,
			EventData:  string(eventDataJSON),
			ActionType: action.Type,
			ActionURL:  action.URL,
			MaxRetries: rule.RetryCount,
			Status:     "pending",
		}

		switch action.Type {
		case "webhook":
			e.executeWebhook(action, event, &logEntry)
		case "notification":
			logEntry.Status = "success"
			logEntry.Response = "notification: " + action.Message
			log.Printf("[Webhook] Notification: %s", action.Message)
		default:
			logEntry.Status = "failed"
			logEntry.Error = "unsupported action type: " + action.Type
		}

		e.db.Create(&logEntry)
	}
}

// executeWebhook sends an HTTP request to a webhook URL.
func (e *Engine) executeWebhook(action ActionConfig, event Event, logEntry *EventLog) {
	method := action.Method
	if method == "" {
		method = "POST"
	}

	// Build body
	body := action.Body
	if body == "" {
		bodyBytes, _ := json.Marshal(event)
		body = string(bodyBytes)
	} else {
		// Simple template substitution
		eventJSON, _ := json.Marshal(event)
		body = strings.ReplaceAll(body, "{{.event}}", string(eventJSON))
		body = strings.ReplaceAll(body, "{{.type}}", event.Type)
		body = strings.ReplaceAll(body, "{{.source}}", event.Source)
	}

	req, err := http.NewRequest(method, action.URL, bytes.NewBufferString(body))
	if err != nil {
		logEntry.Status = "failed"
		logEntry.Error = err.Error()
		logEntry.Attempts = 1
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "StarClaw-Webhook/1.0")
	for k, v := range action.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.httpC.Do(req)
	logEntry.Attempts = 1

	if err != nil {
		logEntry.Status = "retrying"
		logEntry.Error = err.Error()
		nextRetry := time.Now().Add(time.Duration(60) * time.Second)
		logEntry.NextRetry = &nextRetry
		return
	}
	defer resp.Body.Close()

	logEntry.StatusCode = resp.StatusCode

	// Read response (limited)
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	logEntry.Response = string(buf[:n])

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logEntry.Status = "success"
	} else {
		logEntry.Status = "retrying"
		logEntry.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		nextRetry := time.Now().Add(time.Duration(60) * time.Second)
		logEntry.NextRetry = &nextRetry
	}
}

// retryLoop processes failed webhook deliveries.
func (e *Engine) retryLoop() {
	defer e.wg.Done()

	select {
	case <-e.stopCh:
		return
	case <-time.After(30 * time.Second):
	}

	log.Println("[Webhook] Retry loop started (every 30s)")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.processRetries()
		}
	}
}

// processRetries retries failed webhook deliveries.
func (e *Engine) processRetries() {
	var logs []EventLog
	e.db.Where("status = ? AND next_retry <= ? AND attempts < max_retries", "retrying", time.Now()).
		Limit(50).Find(&logs)

	for _, logEntry := range logs {
		// Re-attempt webhook
		var rule EventRule
		if err := e.db.Where("id = ?", logEntry.RuleID).First(&rule).Error; err != nil {
			continue
		}

		var actions []ActionConfig
		if err := json.Unmarshal([]byte(rule.Actions), &actions); err != nil {
			continue
		}

		// Find the matching action
		for _, action := range actions {
			if action.Type == logEntry.ActionType && action.URL == logEntry.ActionURL {
				var event Event
				json.Unmarshal([]byte(logEntry.EventData), &event)

				logEntry.Attempts++
				e.executeWebhook(action, event, &logEntry)

				if logEntry.Status == "retrying" && logEntry.Attempts >= logEntry.MaxRetries {
					logEntry.Status = "dead_letter"
				}

				e.db.Save(&logEntry)
				break
			}
		}
	}
}

// ════════════════════════════════════════════════════════════════
//  Stats
// ════════════════════════════════════════════════════════════════

// Stats returns webhook engine statistics.
func (e *Engine) Stats() map[string]interface{} {
	dayAgo := time.Now().Add(-24 * time.Hour)

	var totalRules, enabledRules int64
	e.db.Model(&EventRule{}).Count(&totalRules)
	e.db.Model(&EventRule{}).Where("enabled = ?", true).Count(&enabledRules)

	var firedToday, successToday, failedToday, deadLetterCount int64
	e.db.Model(&EventLog{}).Where("created_at >= ?", dayAgo).Count(&firedToday)
	e.db.Model(&EventLog{}).Where("created_at >= ? AND status = ?", dayAgo, "success").Count(&successToday)
	e.db.Model(&EventLog{}).Where("created_at >= ? AND status = ?", dayAgo, "failed").Count(&failedToday)
	e.db.Model(&EventLog{}).Where("status = ?", "dead_letter").Count(&deadLetterCount)

	var pendingRetries int64
	e.db.Model(&EventLog{}).Where("status = ?", "retrying").Count(&pendingRetries)

	return map[string]interface{}{
		"total_rules":      totalRules,
		"enabled_rules":    enabledRules,
		"fired_today":      firedToday,
		"success_today":    successToday,
		"failed_today":     failedToday,
		"dead_letters":     deadLetterCount,
		"pending_retries":  pendingRetries,
		"queue_size":       len(e.eventCh),
	}
}
