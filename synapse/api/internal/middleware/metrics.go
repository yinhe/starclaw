package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starai",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status", "auth_type"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "starai",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"method", "path"},
	)
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "starai",
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)

	// ── Business metrics ──

	InferenceRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starai",
			Name:      "inference_requests_total",
			Help:      "Total inference requests by provider, model, and via (direct/proxy/claw)",
		},
		[]string{"provider", "model", "via", "status"},
	)
	InferenceLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "starai",
			Name:      "inference_latency_seconds",
			Help:      "Inference request latency in seconds",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"provider", "model"},
	)
	BillingDeductionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starai",
			Name:      "billing_deductions_total",
			Help:      "Total billing deductions by type (cny/star_energy)",
		},
		[]string{"type"},
	)
	BillingDeductionAmount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starai",
			Name:      "billing_deduction_amount_cents",
			Help:      "Total billing deduction amount in cents (分)",
		},
		[]string{"type"},
	)
	AuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "starai",
			Name:      "auth_attempts_total",
			Help:      "Authentication attempts by type and result",
		},
		[]string{"type", "result"},
	)
)

// PrometheusMetrics returns a Gin middleware that records HTTP metrics
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		httpRequestsInFlight.Inc()
		start := time.Now()

		c.Next()

		httpRequestsInFlight.Dec()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		authType := c.GetString("auth_type")
		if authType == "" {
			authType = "none"
		}

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status, authType).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// MetricsHandler returns the Prometheus HTTP handler wrapped for Gin
func MetricsHandler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
