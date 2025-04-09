package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PostgresMetrics holds all metrics for the PostgreSQL operator
type PostgresMetrics struct {
	// Query metrics
	queriesTotal      *prometheus.CounterVec
	queryDuration     *prometheus.HistogramVec
	rowsProcessed     *prometheus.CounterVec
	
	// Database metrics
	connectionErrors  *prometheus.CounterVec
	activeConnections *prometheus.GaugeVec
	
	// Business metrics
	itemsCreated      *prometheus.CounterVec
	itemsUpdated      *prometheus.CounterVec
	itemsDeleted      *prometheus.CounterVec
	
	// Error metrics
	operationErrors   *prometheus.CounterVec
}

// NewPostgresMetrics initializes and registers all PostgreSQL metrics
func NewPostgresMetrics() *PostgresMetrics {
	m := &PostgresMetrics{}
	
	// Query metrics
	m.queriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_queries_total",
			Help: "Total number of PostgreSQL queries executed",
		},
		[]string{"query_type", "status"},
	)
	
	m.queryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "postgres_query_duration_seconds",
			Help:    "Duration of PostgreSQL queries in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // From 1ms to ~1s
		},
		[]string{"query_type"},
	)
	
	m.rowsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_rows_processed_total",
			Help: "Total number of rows processed in PostgreSQL",
		},
		[]string{"operation"},
	)
	
	// Database metrics
	m.connectionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_connection_errors_total",
			Help: "Total number of PostgreSQL connection errors",
		},
		[]string{"error_type"},
	)
	
	m.activeConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "postgres_active_connections",
			Help: "Current number of active PostgreSQL connections",
		},
		[]string{"state"},
	)
	
	// Business metrics
	m.itemsCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_items_created_total",
			Help: "Total number of items created in PostgreSQL",
		},
		[]string{"item_type"},
	)
	
	m.itemsUpdated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_items_updated_total",
			Help: "Total number of items updated in PostgreSQL",
		},
		[]string{"item_type"},
	)
	
	m.itemsDeleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_items_deleted_total",
			Help: "Total number of items deleted in PostgreSQL",
		},
		[]string{"item_type"},
	)
	
	// Error metrics
	m.operationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgres_operation_errors_total",
			Help: "Total number of errors during PostgreSQL operations",
		},
		[]string{"operation", "error_type"},
	)
	
	return m
}