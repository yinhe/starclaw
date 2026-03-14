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
			Namespace: "queen",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "queen",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"method", "path"},
	)
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "queen",
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)

	// ── Swarm metrics ──

	SwarmOnlineNodes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "queen",
			Name:      "swarm_online_nodes",
			Help:      "Number of currently online Claw nodes",
		},
	)
	SwarmHeartbeatsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "swarm_heartbeats_total",
			Help:      "Total heartbeats received from Claw nodes",
		},
	)

	// ── Billing metrics ──

	BillingRechargesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "billing_recharges_total",
			Help:      "Total recharge orders by status",
		},
		[]string{"status", "pay_method"},
	)
	BillingRechargeAmount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "billing_recharge_amount_cents",
			Help:      "Total recharge amount in cents (分)",
		},
	)

	// ── Star Energy (Credit) metrics ──

	CreditGrantsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "credit_grants_total",
			Help:      "Total star energy grants",
		},
	)
	CreditGrantAmount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "credit_grant_amount_units",
			Help:      "Total star energy granted (internal units)",
		},
	)
	CreditConsumesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "credit_consumes_total",
			Help:      "Total star energy consumption events",
		},
	)
	CreditConsumeAmount = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "credit_consume_amount_units",
			Help:      "Total star energy consumed (internal units)",
		},
	)
	CreditTransfersTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "queen",
			Name:      "credit_transfers_total",
			Help:      "Total star energy transfers between claws",
		},
	)
	CreditActiveAccounts = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "queen",
			Name:      "credit_active_accounts",
			Help:      "Number of active star energy accounts",
		},
	)

	// ── Node binding metrics ──

	NodeBindingsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "queen",
			Name:      "node_bindings_active",
			Help:      "Number of active Queen user ↔ Claw node bindings",
		},
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

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// MetricsHandler returns the Prometheus HTTP handler wrapped for Gin
func MetricsHandler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
