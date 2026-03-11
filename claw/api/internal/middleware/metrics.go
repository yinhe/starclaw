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
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
	activeWebSockets = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "websocket_connections_active",
			Help: "Number of active WebSocket/SSE connections",
		},
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
