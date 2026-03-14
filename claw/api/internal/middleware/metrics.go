package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starclaw",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "starclaw",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"method", "path"},
	)
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "starclaw",
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)
	activeWebSockets = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "starclaw",
			Name:      "websocket_connections_active",
			Help:      "Number of active WebSocket/SSE connections",
		},
	)

	// ── Business metrics: Agent tasks ──

	AgentTasksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starclaw",
			Name:      "agent_tasks_total",
			Help:      "Total agent tasks by status (ok/error)",
		},
		[]string{"status"},
	)

	// ── Business metrics: Chat / LLM ──

	ChatRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starclaw",
			Name:      "chat_requests_total",
			Help:      "Total chat requests by provider and model",
		},
		[]string{"provider", "model", "status"},
	)
	ChatLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "starclaw",
			Name:      "chat_latency_seconds",
			Help:      "Chat request latency (time to first token or full response)",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model"},
	)
	TokensUsedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starclaw",
			Name:      "tokens_used_total",
			Help:      "Total tokens consumed by type (prompt/completion)",
		},
		[]string{"provider", "model", "type"},
	)

	// ── Business metrics: Tool calls ──

	ToolCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starclaw",
			Name:      "tool_calls_total",
			Help:      "Total tool calls by tool name and status",
		},
		[]string{"tool", "status"},
	)
	ToolCallLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "starclaw",
			Name:      "tool_call_latency_seconds",
			Help:      "Tool call execution latency",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"tool"},
	)

	// ── Business metrics: Star Energy ──

	StarEnergyBalance = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "starclaw",
			Name:      "star_energy_balance",
			Help:      "Current star energy balance (in stars, 1 star = 10000 units)",
		},
	)
	StarEnergyHP = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "starclaw",
			Name:      "star_energy_hp_percent",
			Help:      "Current HP percentage (0-100)",
		},
	)

	// ── Business metrics: Inference (contributor) ──

	InferenceTasksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starclaw",
			Name:      "inference_tasks_total",
			Help:      "Total inference tasks processed as a contributor",
		},
		[]string{"model", "status"},
	)
)

// PrometheusMetrics returns a Gin middleware that records HTTP metrics
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := normalizePath(c.FullPath())
		if path == "" {
			path = "unknown"
		}

		httpRequestsInFlight.Inc()
		start := time.Now()

		c.Next()

		httpRequestsInFlight.Dec()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// IncrSSE increments active SSE connection count (call on connect)
func IncrSSE() { activeWebSockets.Inc() }

// DecrSSE decrements active SSE connection count (call on disconnect)
func DecrSSE() { activeWebSockets.Dec() }

// normalizePath collapses path params to reduce cardinality
func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	return path
}
