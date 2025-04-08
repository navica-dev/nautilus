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
	robotRuns       *prometheus.CounterVec
	robotRunsFailed *prometheus.CounterVec
	robotRunLatency *prometheus.HistogramVec
	workerPoolSize  *prometheus.GaugeVec
	taskQueueSize   *prometheus.GaugeVec

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
	c.robotRuns = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nautilus_robot_runs_total",
			Help: "Total number of robot executions",
		},
		[]string{"robot", "status"},
	)

	c.robotRunsFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nautilus_robot_runs_failed_total",
			Help: "Total number of failed robot executions",
		},
		[]string{"robot"},
	)

	c.robotRunLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nautilus_robot_run_duration_seconds",
			Help:    "Duration of robot executions in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~102s
		},
		[]string{"robot", "status"},
	)

	c.workerPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nautilus_worker_pool_size",
			Help: "Current size of the worker pool",
		},
		[]string{"robot"},
	)

	c.taskQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nautilus_task_queue_size",
			Help: "Current size of the task queue",
		},
		[]string{"robot"},
	)

	log.Debug().Msg("Prometheus metrics initialized")
}

// RegisterBasicMetrics sets up basic metrics for a robot
func (c *Client) RegisterBasicMetrics(robotName string) {
	c.labels["robot"] = robotName

	// Initialize counters with 0 values to ensure they appear in Prometheus
	if c.robotRuns != nil {
		c.robotRuns.WithLabelValues(robotName, "success").Add(0)
		c.robotRuns.WithLabelValues(robotName, "failure").Add(0)
	}

	if c.robotRunsFailed != nil {
		c.robotRunsFailed.WithLabelValues(robotName).Add(0)
	}

	log.Debug().Str("robot", robotName).Msg("Basic metrics registered")
}

// RecordRunStart records the start of a robot run
func (c *Client) RecordRunStart() {
	// Nothing to record at start time for Prometheus
	// This is a hook for other metrics systems that might need it
}

// RecordRunSuccess records a successful robot run
func (c *Client) RecordRunSuccess(duration time.Duration) {
	if !c.config.Enabled {
		return
	}

	robotName := c.labels["robot"]

	if c.robotRuns != nil {
		c.robotRuns.WithLabelValues(robotName, "success").Inc()
	}

	if c.robotRunLatency != nil {
		c.robotRunLatency.WithLabelValues(robotName, "success").Observe(duration.Seconds())
	}
}

// RecordRunFailure records a failed robot run
func (c *Client) RecordRunFailure(duration time.Duration) {
	if !c.config.Enabled {
		return
	}

	robotName := c.labels["robot"]

	if c.robotRuns != nil {
		c.robotRuns.WithLabelValues(robotName, "failure").Inc()
	}

	if c.robotRunsFailed != nil {
		c.robotRunsFailed.WithLabelValues(robotName).Inc()
	}

	if c.robotRunLatency != nil {
		c.robotRunLatency.WithLabelValues(robotName, "failure").Observe(duration.Seconds())
	}
}

// RecordWorkerPoolSize records the current size of the worker pool
func (c *Client) RecordWorkerPoolSize(size int) {
	if !c.config.Enabled {
		return
	}

	robotName := c.labels["robot"]

	if c.workerPoolSize != nil {
		c.workerPoolSize.WithLabelValues(robotName).Set(float64(size))
	}
}

// RecordTaskQueueSize records the current size of the task queue
func (c *Client) RecordTaskQueueSize(size int) {
	if !c.config.Enabled {
		return
	}

	robotName := c.labels["robot"]

	if c.taskQueueSize != nil {
		c.taskQueueSize.WithLabelValues(robotName).Set(float64(size))
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
