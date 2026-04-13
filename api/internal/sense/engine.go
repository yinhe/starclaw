package sense

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// SenseClaw v1 — 感知引擎
//
// 职责:
//   1. 反馈收集: 用户反馈、Bug报告、功能请求
//   2. 健康巡检: 定期检查服务状态、聚合健康指标
//   3. 告警管理: 告警规则、触发、升级、静默
//   4. 异常检测: 错误率飙升、延迟异常、资源瓶颈
//   5. 事件聚合: 将多源事件汇总为可操作的洞察
// ════════════════════════════════════════════════════════════

// ── Types ──

type FeedbackType string

const (
	FeedbackBug     FeedbackType = "bug"
	FeedbackFeature FeedbackType = "feature_request"
	FeedbackCrash   FeedbackType = "crash"
	FeedbackUX      FeedbackType = "ux_issue"
	FeedbackPraise  FeedbackType = "praise"
	FeedbackOther   FeedbackType = "other"
)

type FeedbackPriority string

const (
	FBPriCritical FeedbackPriority = "critical"
	FBPriHigh     FeedbackPriority = "high"
	FBPriMedium   FeedbackPriority = "medium"
	FBPriLow      FeedbackPriority = "low"
)

type AlertSeverity string

const (
	AlertCritical AlertSeverity = "critical"
	AlertWarning  AlertSeverity = "warning"
	AlertInfo     AlertSeverity = "info"
)

type AlertStatus string

const (
	AlertFiring   AlertStatus = "firing"
	AlertResolved AlertStatus = "resolved"
	AlertSilenced AlertStatus = "silenced"
)

type ServiceStatus string

const (
	ServiceHealthy  ServiceStatus = "healthy"
	ServiceDegraded ServiceStatus = "degraded"
	ServiceDown     ServiceStatus = "down"
	ServiceUnknown  ServiceStatus = "unknown"
)

// ── Data Structures ──

type Feedback struct {
	ID          string           `json:"id"`
	Type        FeedbackType     `json:"type"`
	Priority    FeedbackPriority `json:"priority"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Source      string           `json:"source"`    // user_id, agent_id, or "system"
	NodeID      string           `json:"node_id,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Status      string           `json:"status"` // new, triaged, assigned, resolved, closed
	AssignedTo  string           `json:"assigned_to,omitempty"`
	Resolution  string           `json:"resolution,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	ResolvedAt  *time.Time       `json:"resolved_at,omitempty"`
}

type Alert struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Severity    AlertSeverity `json:"severity"`
	Status      AlertStatus   `json:"status"`
	Service     string        `json:"service"`
	Message     string        `json:"message"`
	Value       float64       `json:"value,omitempty"`
	Threshold   float64       `json:"threshold,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	FiredAt     time.Time     `json:"fired_at"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
	SilencedUntil *time.Time  `json:"silenced_until,omitempty"`
}

type HealthCheck struct {
	Service   string        `json:"service"`
	Status    ServiceStatus `json:"status"`
	Latency   float64       `json:"latency_ms"`
	Message   string        `json:"message,omitempty"`
	CheckedAt time.Time     `json:"checked_at"`
}

type Anomaly struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // error_spike, latency_spike, resource_exhaustion, crash_loop
	Service     string    `json:"service"`
	Description string    `json:"description"`
	Metric      string    `json:"metric,omitempty"`
	Current     float64   `json:"current_value,omitempty"`
	Baseline    float64   `json:"baseline_value,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
}

type Insight struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"` // trend, risk, opportunity, action_needed
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Impact    string    `json:"impact"` // high, medium, low
	Sources   []string  `json:"sources,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Engine ──

type EngineConfig struct {
	HealthCheckInterval int      `json:"health_check_interval_sec"`
	AlertRetention      int      `json:"alert_retention_count"`
	FeedbackRetention   int      `json:"feedback_retention_count"`
	MonitoredServices   []string `json:"monitored_services"`
	ErrorRateThreshold  float64  `json:"error_rate_threshold"`
	LatencyThresholdMs  float64  `json:"latency_threshold_ms"`
}

func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		HealthCheckInterval: 300,
		AlertRetention:      500,
		FeedbackRetention:   1000,
		MonitoredServices: []string{
			"queen", "claw", "synapse", "billing", "overlord",
			"gateway", "hive", "pheromone", "nydus", "forge", "overmind",
		},
		ErrorRateThreshold: 0.05,
		LatencyThresholdMs: 2000,
	}
}

type Engine struct {
	mu        sync.RWMutex
	nodeID    string
	config    *EngineConfig
	feedbacks []Feedback
	alerts    []Alert
	health    map[string]*HealthCheck
	anomalies []Anomaly
	insights  []Insight
	stats     EngineStats
	startAt   time.Time
	nextID    int
}

type EngineStats struct {
	FeedbacksReceived int       `json:"feedbacks_received"`
	FeedbacksResolved int       `json:"feedbacks_resolved"`
	AlertsFired       int       `json:"alerts_fired"`
	AlertsResolved    int       `json:"alerts_resolved"`
	AlertsSilenced    int       `json:"alerts_silenced"`
	HealthChecks      int       `json:"health_checks"`
	AnomaliesDetected int       `json:"anomalies_detected"`
	InsightsGenerated int       `json:"insights_generated"`
	ServicesMonitored int       `json:"services_monitored"`
	Uptime            string    `json:"uptime"`
	LastCheck         time.Time `json:"last_check,omitempty"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(nodeID string, cfg *EngineConfig) *Engine {
	if cfg == nil {
		cfg = DefaultEngineConfig()
	}
	engineOnce.Do(func() {
		health := make(map[string]*HealthCheck)
		for _, svc := range cfg.MonitoredServices {
			health[svc] = &HealthCheck{
				Service:   svc,
				Status:    ServiceUnknown,
				CheckedAt: time.Now(),
			}
		}
		globalEngine = &Engine{
			nodeID:    nodeID,
			config:    cfg,
			feedbacks: make([]Feedback, 0),
			alerts:    make([]Alert, 0),
			health:    health,
			anomalies: make([]Anomaly, 0),
			insights:  make([]Insight, 0),
			startAt:   time.Now(),
		}
		log.Printf("[sense] engine ready (services=%d, error_threshold=%.1f%%, latency_threshold=%.0fms)",
			len(cfg.MonitoredServices), cfg.ErrorRateThreshold*100, cfg.LatencyThresholdMs)
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().Unix(), e.nextID)
}

// ── Feedback ──

func (e *Engine) SubmitFeedback(fbType FeedbackType, priority FeedbackPriority, title, description, source string, tags []string) *Feedback {
	e.mu.Lock()
	defer e.mu.Unlock()

	fb := Feedback{
		ID:          e.genID("fb"),
		Type:        fbType,
		Priority:    priority,
		Title:       title,
		Description: description,
		Source:      source,
		NodeID:      e.nodeID,
		Tags:        tags,
		Status:      "new",
		CreatedAt:   time.Now(),
	}

	e.feedbacks = append(e.feedbacks, fb)
	if len(e.feedbacks) > e.config.FeedbackRetention {
		e.feedbacks = e.feedbacks[1:]
	}
	e.stats.FeedbacksReceived++
	log.Printf("[sense] feedback received: [%s] %s — %s", fbType, title, priority)
	return &fb
}

func (e *Engine) ResolveFeedback(fbID, resolution string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.feedbacks {
		if e.feedbacks[i].ID == fbID {
			now := time.Now()
			e.feedbacks[i].Status = "resolved"
			e.feedbacks[i].Resolution = resolution
			e.feedbacks[i].ResolvedAt = &now
			e.stats.FeedbacksResolved++
			return nil
		}
	}
	return fmt.Errorf("feedback %s not found", fbID)
}

func (e *Engine) ListFeedbacks(status string, limit int) []Feedback {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var filtered []Feedback
	for _, fb := range e.feedbacks {
		if status == "" || fb.Status == status {
			filtered = append(filtered, fb)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

// ── Alerts ──

func (e *Engine) FireAlert(name string, severity AlertSeverity, service, message string, value, threshold float64, labels map[string]string) *Alert {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert := Alert{
		ID:        e.genID("alert"),
		Name:      name,
		Severity:  severity,
		Status:    AlertFiring,
		Service:   service,
		Message:   message,
		Value:     value,
		Threshold: threshold,
		Labels:    labels,
		FiredAt:   time.Now(),
	}

	e.alerts = append(e.alerts, alert)
	if len(e.alerts) > e.config.AlertRetention {
		e.alerts = e.alerts[1:]
	}
	e.stats.AlertsFired++
	log.Printf("[sense] ALERT [%s] %s: %s (service=%s, value=%.2f)", severity, name, message, service, value)
	return &alert
}

func (e *Engine) ResolveAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.alerts {
		if e.alerts[i].ID == alertID {
			now := time.Now()
			e.alerts[i].Status = AlertResolved
			e.alerts[i].ResolvedAt = &now
			e.stats.AlertsResolved++
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

func (e *Engine) SilenceAlert(alertID string, duration time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.alerts {
		if e.alerts[i].ID == alertID {
			until := time.Now().Add(duration)
			e.alerts[i].Status = AlertSilenced
			e.alerts[i].SilencedUntil = &until
			e.stats.AlertsSilenced++
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

func (e *Engine) ListAlerts(status string) []Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []Alert
	for _, a := range e.alerts {
		if status == "" || string(a.Status) == status {
			result = append(result, a)
		}
	}
	return result
}

func (e *Engine) ActiveAlerts() []Alert {
	return e.ListAlerts("firing")
}

// ── Health Check ──

func (e *Engine) UpdateHealth(service string, status ServiceStatus, latencyMs float64, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.health[service] = &HealthCheck{
		Service:   service,
		Status:    status,
		Latency:   latencyMs,
		Message:   message,
		CheckedAt: time.Now(),
	}
	e.stats.HealthChecks++
	e.stats.LastCheck = time.Now()
}

func (e *Engine) GetHealth() map[string]*HealthCheck {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*HealthCheck, len(e.health))
	for k, v := range e.health {
		result[k] = v
	}
	return result
}

func (e *Engine) HealthSummary() map[string]int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	summary := map[string]int{"healthy": 0, "degraded": 0, "down": 0, "unknown": 0}
	for _, h := range e.health {
		summary[string(h.Status)]++
	}
	return summary
}

// ── Anomaly Detection ──

func (e *Engine) ReportAnomaly(anomalyType, service, description, metric string, current, baseline float64) *Anomaly {
	e.mu.Lock()
	defer e.mu.Unlock()

	a := Anomaly{
		ID:          e.genID("anomaly"),
		Type:        anomalyType,
		Service:     service,
		Description: description,
		Metric:      metric,
		Current:     current,
		Baseline:    baseline,
		DetectedAt:  time.Now(),
	}

	e.anomalies = append(e.anomalies, a)
	if len(e.anomalies) > 200 {
		e.anomalies = e.anomalies[1:]
	}
	e.stats.AnomaliesDetected++
	log.Printf("[sense] ANOMALY [%s] %s: %s (current=%.2f, baseline=%.2f)", anomalyType, service, description, current, baseline)
	return &a
}

func (e *Engine) ListAnomalies(limit int) []Anomaly {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.anomalies) {
		limit = len(e.anomalies)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.anomalies) - limit
	result := make([]Anomaly, limit)
	copy(result, e.anomalies[start:])
	return result
}

// ── Insights ──

func (e *Engine) GenerateInsight(category, title, summary, impact string, sources []string) *Insight {
	e.mu.Lock()
	defer e.mu.Unlock()

	insight := Insight{
		ID:        e.genID("insight"),
		Category:  category,
		Title:     title,
		Summary:   summary,
		Impact:    impact,
		Sources:   sources,
		CreatedAt: time.Now(),
	}

	e.insights = append(e.insights, insight)
	if len(e.insights) > 200 {
		e.insights = e.insights[1:]
	}
	e.stats.InsightsGenerated++
	return &insight
}

func (e *Engine) ListInsights(limit int) []Insight {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.insights) {
		limit = len(e.insights)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.insights) - limit
	result := make([]Insight, limit)
	copy(result, e.insights[start:])
	return result
}

// ── Stats ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	s.ServicesMonitored = len(e.config.MonitoredServices)
	return &s
}

func (e *Engine) Config() *EngineConfig {
	return e.config
}
