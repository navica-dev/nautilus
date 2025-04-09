package metrics

import (
	"time"

	"github.com/navica-dev/nautilus/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
)

// Client manages metrics collection and reporting
type Client struct {
	// Configuration
	config *config.MetricsConfig

	// Prometheus metrics
	operatorRuns       *prometheus.CounterVec
	operatorRunsFailed *prometheus.CounterVec
	operatorRunLatency *prometheus.HistogramVec
	workerPoolSize     *prometheus.GaugeVec
	taskQueueSize      *prometheus.GaugeVec

	// Labels
	labels map[string]string
}

// NewClient creates a new metrics client
func NewClient(cfg *config.MetricsConfig) *Client {
	c := &Client{
		config: cfg,
		labels: make(map[string]string),
	}

	// Initialize Prometheus if enabled
	if cfg.Prometheus != nil && cfg.Prometheus.Enabled {
		c.initPrometheus()
	}

	return c
}

// initPrometheus initializes Prometheus metrics
func (c *Client) initPrometheus() {
	c.operatorRuns = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nautilus_operator_runs_total",
			Help: "Total number of operator executions",
		},
		[]string{"operator", "status"},
	)

	c.operatorRunsFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nautilus_operator_runs_failed_total",
			Help: "Total number of failed operator executions",
		},
		[]string{"operator"},
	)

	c.operatorRunLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nautilus_operator_run_duration_seconds",
			Help:    "Duration of operator executions in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~102s
		},
		[]string{"operator", "status"},
	)

	c.workerPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nautilus_worker_pool_size",
			Help: "Current size of the worker pool",
		},
		[]string{"operator"},
	)

	c.taskQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nautilus_task_queue_size",
			Help: "Current size of the task queue",
		},
		[]string{"operator"},
	)

	log.Debug().Msg("Prometheus metrics initialized")
}

// RegisterBasicMetrics sets up basic metrics for a operator
func (c *Client) RegisterBasicMetrics(operatorName string) {
	c.labels["operator"] = operatorName

	// Initialize counters with 0 values to ensure they appear in Prometheus
	if c.operatorRuns != nil {
		c.operatorRuns.WithLabelValues(operatorName, "success").Add(0)
		c.operatorRuns.WithLabelValues(operatorName, "failure").Add(0)
	}

	if c.operatorRunsFailed != nil {
		c.operatorRunsFailed.WithLabelValues(operatorName).Add(0)
	}

	log.Debug().Str("operator", operatorName).Msg("Basic metrics registered")
}

// RecordRunStart records the start of a operator run
func (c *Client) RecordRunStart() {
	// Nothing to record at start time for Prometheus
	// This is a hook for other metrics systems that might need it
}

// RecordRunSuccess records a successful operator run
func (c *Client) RecordRunSuccess(duration time.Duration) {
	if !c.config.Enabled {
		return
	}

	operatorName := c.labels["operator"]

	if c.operatorRuns != nil {
		c.operatorRuns.WithLabelValues(operatorName, "success").Inc()
	}

	if c.operatorRunLatency != nil {
		c.operatorRunLatency.WithLabelValues(operatorName, "success").Observe(duration.Seconds())
	}
}

// RecordRunFailure records a failed operator run
func (c *Client) RecordRunFailure(duration time.Duration) {
	if !c.config.Enabled {
		return
	}

	operatorName := c.labels["operator"]

	if c.operatorRuns != nil {
		c.operatorRuns.WithLabelValues(operatorName, "failure").Inc()
	}

	if c.operatorRunsFailed != nil {
		c.operatorRunsFailed.WithLabelValues(operatorName).Inc()
	}

	if c.operatorRunLatency != nil {
		c.operatorRunLatency.WithLabelValues(operatorName, "failure").Observe(duration.Seconds())
	}
}

// RecordWorkerPoolSize records the current size of the worker pool
func (c *Client) RecordWorkerPoolSize(size int) {
	if !c.config.Enabled {
		return
	}

	operatorName := c.labels["operator"]

	if c.workerPoolSize != nil {
		c.workerPoolSize.WithLabelValues(operatorName).Set(float64(size))
	}
}

// RecordTaskQueueSize records the current size of the task queue
func (c *Client) RecordTaskQueueSize(size int) {
	if !c.config.Enabled {
		return
	}

	operatorName := c.labels["operator"]

	if c.taskQueueSize != nil {
		c.taskQueueSize.WithLabelValues(operatorName).Set(float64(size))
	}
}

// CreateCustomCounter creates a custom counter metric
func (c *Client) CreateCustomCounter(name, help string, labelNames ...string) *prometheus.CounterVec {
	if !c.config.Enabled || c.config.Prometheus == nil || !c.config.Prometheus.Enabled {
		return nil
	}

	return promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		labelNames,
	)
}

// CreateCustomGauge creates a custom gauge metric
func (c *Client) CreateCustomGauge(name, help string, labelNames ...string) *prometheus.GaugeVec {
	if !c.config.Enabled || c.config.Prometheus == nil || !c.config.Prometheus.Enabled {
		return nil
	}

	return promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		},
		labelNames,
	)
}

// CreateCustomHistogram creates a custom histogram metric
func (c *Client) CreateCustomHistogram(name, help string, buckets []float64, labelNames ...string) *prometheus.HistogramVec {
	if !c.config.Enabled || c.config.Prometheus == nil || !c.config.Prometheus.Enabled {
		return nil
	}

	return promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		},
		labelNames,
	)
}
