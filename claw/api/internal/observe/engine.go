package observe

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Trace Span — distributed tracing for Agent call chains
// ════════════════════════════════════════════════════════════════

// SpanKind classifies the type of work a span represents.
type SpanKind string

const (
	SpanKindRequest  SpanKind = "request"  // incoming HTTP request
	SpanKindLLM      SpanKind = "llm"      // LLM provider call
	SpanKindTool     SpanKind = "tool"     // tool execution
	SpanKindRAG      SpanKind = "rag"      // RAG retrieval
	SpanKindWorkflow SpanKind = "workflow" // workflow step
	SpanKindAgent    SpanKind = "agent"    // agent reasoning step
	SpanKindInternal SpanKind = "internal" // internal processing
)

// TraceSpan represents a single unit of work in a distributed trace.
type TraceSpan struct {
	ID            string            `json:"id" gorm:"type:varchar(32);primaryKey"`
	TraceID       string            `json:"trace_id" gorm:"type:varchar(32);index;not null"`
	ParentSpanID  string            `json:"parent_span_id" gorm:"type:varchar(32);index"`
	Name          string            `json:"name" gorm:"type:varchar(200);not null"`
	Kind          SpanKind          `json:"kind" gorm:"type:varchar(20);index"`
	Status        string            `json:"status" gorm:"type:varchar(20);default:ok"` // ok, error
	StatusMessage string            `json:"status_message" gorm:"type:text"`
	AgentID       string            `json:"agent_id" gorm:"type:varchar(36);index"`
	UserID        string            `json:"user_id" gorm:"type:varchar(36);index"`
	StartTime     time.Time         `json:"start_time" gorm:"index;not null"`
	EndTime       time.Time         `json:"end_time"`
	DurationMs    int64             `json:"duration_ms" gorm:"index"`
	Attributes    string            `json:"attributes" gorm:"type:json"` // JSON key-value pairs
	Events        string            `json:"events" gorm:"type:json"`     // JSON array of timed events
	tags          map[string]string // in-memory only, serialized to Attributes
}

// ════════════════════════════════════════════════════════════════
//  Alert Rule — configurable alerting
// ════════════════════════════════════════════════════════════════

// AlertSeverity defines how critical an alert is.
type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

// AlertRule defines when an alert should fire.
type AlertRule struct {
	ID          string        `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string        `json:"name" gorm:"type:varchar(200);not null"`
	Description string        `json:"description" gorm:"type:text"`
	Metric      string        `json:"metric" gorm:"type:varchar(100);not null"`  // e.g. error_rate, p99_latency, agent_failures
	Operator    string        `json:"operator" gorm:"type:varchar(10);not null"` // gt, lt, gte, lte, eq
	Threshold   float64       `json:"threshold" gorm:"not null"`
	WindowSec   int           `json:"window_sec" gorm:"default:300"` // evaluation window in seconds
	Severity    AlertSeverity `json:"severity" gorm:"type:varchar(20);default:warning"`
	Enabled     bool          `json:"enabled" gorm:"default:true"`
	Actions     string        `json:"actions" gorm:"type:json"` // JSON: [{type: "webhook", url: "..."}, {type: "email", to: "..."}]
	UserID      string        `json:"user_id" gorm:"type:varchar(36);index;not null"`
	CooldownSec int           `json:"cooldown_sec" gorm:"default:3600"` // min seconds between firings
	LastFiredAt *time.Time    `json:"last_fired_at"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// AlertHistory records when an alert rule fired.
type AlertHistory struct {
	ID         string        `json:"id" gorm:"type:varchar(36);primaryKey"`
	RuleID     string        `json:"rule_id" gorm:"type:varchar(36);index;not null"`
	RuleName   string        `json:"rule_name" gorm:"type:varchar(200)"`
	Severity   AlertSeverity `json:"severity" gorm:"type:varchar(20)"`
	Metric     string        `json:"metric" gorm:"type:varchar(100)"`
	Value      float64       `json:"value"`
	Threshold  float64       `json:"threshold"`
	Message    string        `json:"message" gorm:"type:text"`
	Resolved   bool          `json:"resolved" gorm:"default:false"`
	ResolvedAt *time.Time    `json:"resolved_at"`
	CreatedAt  time.Time     `json:"created_at" gorm:"index"`
}

// ════════════════════════════════════════════════════════════════
//  Structured Log Entry
// ════════════════════════════════════════════════════════════════

// LogLevel defines severity for structured logs.
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// LogEntry is a structured log record that can be queried.
type LogEntry struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TraceID   string    `json:"trace_id" gorm:"type:varchar(32);index"`
	SpanID    string    `json:"span_id" gorm:"type:varchar(32);index"`
	Level     LogLevel  `json:"level" gorm:"type:varchar(10);index;not null"`
	Message   string    `json:"message" gorm:"type:text;not null"`
	Source    string    `json:"source" gorm:"type:varchar(100);index"` // module/package name
	AgentID   string    `json:"agent_id" gorm:"type:varchar(36);index"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index"`
	Fields    string    `json:"fields" gorm:"type:json"` // arbitrary structured fields
	CreatedAt time.Time `json:"created_at" gorm:"index;not null"`
}

// ════════════════════════════════════════════════════════════════
//  Observe Engine — manages traces, alerts, and logs
// ════════════════════════════════════════════════════════════════

// Engine is the observability engine.
type Engine struct {
	db     *gorm.DB
	stopCh chan struct{}
	wg     sync.WaitGroup

	// In-memory active spans (not yet finished)
	mu          sync.RWMutex
	activeSpans map[string]*TraceSpan
}

// NewEngine creates a new observability engine.
func NewEngine(db *gorm.DB) *Engine {
	return &Engine{
		db:          db,
		stopCh:      make(chan struct{}),
		activeSpans: make(map[string]*TraceSpan),
	}
}

// Start begins the background alert evaluation loop.
func (e *Engine) Start() {
	log.Println("[Observe] Engine starting...")
	e.wg.Add(1)
	go e.alertLoop()
}

// Stop gracefully shuts down.
func (e *Engine) Stop() {
	log.Println("[Observe] Engine stopping...")
	close(e.stopCh)
	e.wg.Wait()
	log.Println("[Observe] Engine stopped")
}

// ── Tracing ──

// generateID creates a random hex ID.
func generateID(bytes int) string {
	b := make([]byte, bytes)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// NewTraceID generates a new 128-bit trace ID.
func NewTraceID() string { return generateID(16) }

// NewSpanID generates a new 64-bit span ID.
func NewSpanID() string { return generateID(8) }

// StartSpan begins a new trace span.
func (e *Engine) StartSpan(traceID, parentSpanID, name string, kind SpanKind, userID, agentID string) *TraceSpan {
	span := &TraceSpan{
		ID:           NewSpanID(),
		TraceID:      traceID,
		ParentSpanID: parentSpanID,
		Name:         name,
		Kind:         kind,
		Status:       "ok",
		UserID:       userID,
		AgentID:      agentID,
		StartTime:    time.Now(),
		tags:         make(map[string]string),
	}
	if traceID == "" {
		span.TraceID = NewTraceID()
	}

	e.mu.Lock()
	e.activeSpans[span.ID] = span
	e.mu.Unlock()

	return span
}

// EndSpan finishes and persists a span.
func (e *Engine) EndSpan(span *TraceSpan, status, statusMessage string) {
	if span == nil {
		return
	}
	span.EndTime = time.Now()
	span.DurationMs = span.EndTime.Sub(span.StartTime).Milliseconds()
	if status != "" {
		span.Status = status
	}
	span.StatusMessage = statusMessage

	e.mu.Lock()
	delete(e.activeSpans, span.ID)
	e.mu.Unlock()

	// Persist async
	go e.db.Create(span)
}

// GetTrace retrieves all spans for a trace.
func (e *Engine) GetTrace(traceID string) ([]TraceSpan, error) {
	var spans []TraceSpan
	err := e.db.Where("trace_id = ?", traceID).Order("start_time ASC").Find(&spans).Error
	return spans, err
}

// QuerySpans searches spans with filters.
func (e *Engine) QuerySpans(userID, agentID string, kind SpanKind, minDurationMs int64, status string, since time.Time, limit int) ([]TraceSpan, error) {
	q := e.db.Model(&TraceSpan{})
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if minDurationMs > 0 {
		q = q.Where("duration_ms >= ?", minDurationMs)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if !since.IsZero() {
		q = q.Where("start_time >= ?", since)
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var spans []TraceSpan
	err := q.Order("start_time DESC").Limit(limit).Find(&spans).Error
	return spans, err
}

// ── Alerts ──

// alertLoop evaluates alert rules periodically.
func (e *Engine) alertLoop() {
	defer e.wg.Done()

	// Wait for DB to be ready
	select {
	case <-e.stopCh:
		return
	case <-time.After(20 * time.Second):
	}

	log.Println("[Observe] Alert evaluation loop started (every 5m)")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.evaluateAlerts()
		}
	}
}

// evaluateAlerts checks all enabled rules.
func (e *Engine) evaluateAlerts() {
	var rules []AlertRule
	e.db.Where("enabled = ?", true).Find(&rules)

	for _, rule := range rules {
		// Skip if within cooldown
		if rule.LastFiredAt != nil {
			cooldown := time.Duration(rule.CooldownSec) * time.Second
			if time.Since(*rule.LastFiredAt) < cooldown {
				continue
			}
		}

		value, err := e.computeMetric(rule.Metric, rule.WindowSec)
		if err != nil {
			continue
		}

		if e.thresholdBreached(value, rule.Operator, rule.Threshold) {
			e.fireAlert(rule, value)
		}
	}
}

// computeMetric calculates a metric value from recent data.
func (e *Engine) computeMetric(metric string, windowSec int) (float64, error) {
	since := time.Now().Add(-time.Duration(windowSec) * time.Second)

	switch metric {
	case "error_rate":
		var total, errors int64
		e.db.Model(&TraceSpan{}).Where("start_time >= ?", since).Count(&total)
		e.db.Model(&TraceSpan{}).Where("start_time >= ? AND status = ?", since, "error").Count(&errors)
		if total == 0 {
			return 0, nil
		}
		return float64(errors) / float64(total), nil

	case "p99_latency":
		var spans []TraceSpan
		e.db.Where("start_time >= ? AND kind = ?", since, SpanKindLLM).
			Order("duration_ms DESC").Limit(100).Find(&spans)
		if len(spans) == 0 {
			return 0, nil
		}
		idx := int(float64(len(spans)) * 0.01)
		if idx >= len(spans) {
			idx = len(spans) - 1
		}
		return float64(spans[idx].DurationMs), nil

	case "p95_latency":
		var spans []TraceSpan
		e.db.Where("start_time >= ? AND kind = ?", since, SpanKindLLM).
			Order("duration_ms DESC").Limit(100).Find(&spans)
		if len(spans) == 0 {
			return 0, nil
		}
		idx := int(float64(len(spans)) * 0.05)
		if idx >= len(spans) {
			idx = len(spans) - 1
		}
		return float64(spans[idx].DurationMs), nil

	case "agent_failures":
		var count int64
		e.db.Model(&TraceSpan{}).Where("start_time >= ? AND kind = ? AND status = ?",
			since, SpanKindAgent, "error").Count(&count)
		return float64(count), nil

	case "error_count":
		var count int64
		e.db.Model(&LogEntry{}).Where("created_at >= ? AND level = ?", since, LogError).Count(&count)
		return float64(count), nil

	case "avg_latency":
		var result struct{ Avg float64 }
		e.db.Model(&TraceSpan{}).Where("start_time >= ? AND kind = ?", since, SpanKindLLM).
			Select("COALESCE(AVG(duration_ms), 0) as avg").Scan(&result)
		return result.Avg, nil

	default:
		return 0, fmt.Errorf("unknown metric: %s", metric)
	}
}

// thresholdBreached checks if a value crosses the threshold.
func (e *Engine) thresholdBreached(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

// fireAlert creates an alert history record and triggers actions.
func (e *Engine) fireAlert(rule AlertRule, value float64) {
	now := time.Now()
	history := AlertHistory{
		ID:        generateID(18),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Metric:    rule.Metric,
		Value:     value,
		Threshold: rule.Threshold,
		Message:   fmt.Sprintf("[%s] %s: %.4f %s %.4f", rule.Severity, rule.Metric, value, rule.Operator, rule.Threshold),
	}
	e.db.Create(&history)

	// Update last fired time
	e.db.Model(&AlertRule{}).Where("id = ?", rule.ID).Update("last_fired_at", now)

	log.Printf("[Observe] Alert fired: %s (value=%.4f, threshold=%.4f)", rule.Name, value, rule.Threshold)

	// TODO: dispatch webhook/email actions from rule.Actions JSON
}

// ── Structured Logging ──

// Log writes a structured log entry.
func (e *Engine) Log(level LogLevel, source, message, traceID, spanID, agentID, userID, fieldsJSON string) {
	entry := LogEntry{
		ID:        generateID(18),
		TraceID:   traceID,
		SpanID:    spanID,
		Level:     level,
		Message:   message,
		Source:    source,
		AgentID:   agentID,
		UserID:    userID,
		Fields:    fieldsJSON,
		CreatedAt: time.Now(),
	}
	go e.db.Create(&entry)
}

// QueryLogs searches log entries.
func (e *Engine) QueryLogs(traceID, spanID, agentID, userID string, level LogLevel, source string, since time.Time, limit int) ([]LogEntry, error) {
	q := e.db.Model(&LogEntry{})
	if traceID != "" {
		q = q.Where("trace_id = ?", traceID)
	}
	if spanID != "" {
		q = q.Where("span_id = ?", spanID)
	}
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if source != "" {
		q = q.Where("source = ?", source)
	}
	if !since.IsZero() {
		q = q.Where("created_at >= ?", since)
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	var entries []LogEntry
	err := q.Order("created_at DESC").Limit(limit).Find(&entries).Error
	return entries, err
}

// ── Stats ──

// Stats returns observability overview stats.
func (e *Engine) Stats() map[string]interface{} {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	var spansHour, spansDay int64
	e.db.Model(&TraceSpan{}).Where("start_time >= ?", hourAgo).Count(&spansHour)
	e.db.Model(&TraceSpan{}).Where("start_time >= ?", dayAgo).Count(&spansDay)

	var errorsHour, errorsDay int64
	e.db.Model(&TraceSpan{}).Where("start_time >= ? AND status = ?", hourAgo, "error").Count(&errorsHour)
	e.db.Model(&TraceSpan{}).Where("start_time >= ? AND status = ?", dayAgo, "error").Count(&errorsDay)

	var logsHour, logsDay int64
	e.db.Model(&LogEntry{}).Where("created_at >= ?", hourAgo).Count(&logsHour)
	e.db.Model(&LogEntry{}).Where("created_at >= ?", dayAgo).Count(&logsDay)

	var alertsDay int64
	e.db.Model(&AlertHistory{}).Where("created_at >= ?", dayAgo).Count(&alertsDay)

	var activeRules int64
	e.db.Model(&AlertRule{}).Where("enabled = ?", true).Count(&activeRules)

	var avgLatency struct{ Avg float64 }
	e.db.Model(&TraceSpan{}).Where("start_time >= ? AND kind = ?", hourAgo, SpanKindLLM).
		Select("COALESCE(AVG(duration_ms), 0) as avg").Scan(&avgLatency)

	e.mu.RLock()
	activeSpanCount := len(e.activeSpans)
	e.mu.RUnlock()

	return map[string]interface{}{
		"spans_last_hour":    spansHour,
		"spans_last_24h":     spansDay,
		"errors_last_hour":   errorsHour,
		"errors_last_24h":    errorsDay,
		"error_rate_hour":    safeDiv(float64(errorsHour), float64(spansHour)),
		"logs_last_hour":     logsHour,
		"logs_last_24h":      logsDay,
		"alerts_last_24h":    alertsDay,
		"active_alert_rules": activeRules,
		"avg_llm_latency_ms": avgLatency.Avg,
		"active_spans":       activeSpanCount,
	}
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
