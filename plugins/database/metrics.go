package plugins

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PostgresMetrics holds all metrics for the PostgreSQL plugin
type PostgresMetrics struct {
	// Query metrics
	queriesTotal       *prometheus.CounterVec
	queryDuration      *prometheus.HistogramVec
	rowsProcessed      *prometheus.CounterVec
	preparedQueryUsage *prometheus.CounterVec

	// Connection pool metrics
	poolActiveConnections  *prometheus.GaugeVec
	poolIdleConnections    *prometheus.GaugeVec
	poolTotalConnections   *prometheus.GaugeVec
	poolMaxConnections     *prometheus.GaugeVec
	poolConnectionRequests *prometheus.CounterVec
	poolConnectionTimeouts *prometheus.CounterVec
	poolConnectionWaitTime *prometheus.HistogramVec
	poolConnectionLifetime *prometheus.HistogramVec

	// Database metrics
	connectionErrors      *prometheus.CounterVec
	connectionAcquireTime *prometheus.HistogramVec
	pingDuration          *prometheus.HistogramVec

	// Transaction metrics
	transactionsTotal       *prometheus.CounterVec
	transactionDuration     *prometheus.HistogramVec
	transactionOperations   *prometheus.CounterVec
	transactionRollbacks    *prometheus.CounterVec
	transactionSavepoints   *prometheus.CounterVec

	// Batch metrics
	batchOperationsTotal   *prometheus.CounterVec
	batchOperationDuration *prometheus.HistogramVec
	batchSize              *prometheus.HistogramVec

	// Copy metrics
	copyOperationsTotal *prometheus.CounterVec
	copyRowsProcessed   *prometheus.CounterVec
	copyDuration        *prometheus.HistogramVec

	// Listen/Notify metrics
	notificationsReceived *prometheus.CounterVec
	listenerErrors        *prometheus.CounterVec

	// Error metrics
	operationErrors *prometheus.CounterVec
}

// NewPostgresMetrics initializes and registers all PostgreSQL metrics
func NewPostgresMetrics(namespace string) *PostgresMetrics {
	if namespace != "" {
		namespace = namespace + "_"
	}

	m := &PostgresMetrics{}

	// Query metrics
	m.queriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_queries_total",
			Help: "Total number of PostgreSQL queries executed",
		},
		[]string{"operator", "database", "query_type", "status"},
	)

	m.queryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_query_duration_seconds",
			Help:    "Duration of PostgreSQL queries in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // From 1ms to ~1s
		},
		[]string{"operator", "database", "query_type", "table"},
	)

	m.rowsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_rows_processed_total",
			Help: "Total number of rows processed in PostgreSQL",
		},
		[]string{"operator", "database", "operation", "table"},
	)

	m.preparedQueryUsage = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_prepared_queries_total",
			Help: "Total number of prepared statements used",
		},
		[]string{"operator", "database", "query_name"},
	)

	// Connection pool metrics
	m.poolActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: namespace + "postgres_pool_active_connections",
			Help: "Current number of active connections in the pool",
		},
		[]string{"operator", "database"},
	)

	m.poolIdleConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: namespace + "postgres_pool_idle_connections",
			Help: "Current number of idle connections in the pool",
		},
		[]string{"operator", "database"},
	)

	m.poolTotalConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: namespace + "postgres_pool_total_connections",
			Help: "Total number of connections in the pool",
		},
		[]string{"operator", "database"},
	)

	m.poolMaxConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: namespace + "postgres_pool_max_connections",
			Help: "Maximum number of connections allowed in the pool",
		},
		[]string{"operator", "database"},
	)

	m.poolConnectionRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_pool_connection_requests_total",
			Help: "Total number of connection requests made to the pool",
		},
		[]string{"operator", "database"},
	)

	m.poolConnectionTimeouts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_pool_connection_timeouts_total",
			Help: "Total number of connection timeouts that occurred",
		},
		[]string{"operator", "database"},
	)

	m.poolConnectionWaitTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_pool_connection_wait_seconds",
			Help:    "Time spent waiting for a connection from the pool",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // From 1ms to ~1s
		},
		[]string{"operator", "database"},
	)

	m.poolConnectionLifetime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_pool_connection_lifetime_seconds",
			Help:    "Lifetime of connections in the pool",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10), // From 1s to ~17min
		},
		[]string{"operator", "database"},
	)

	// Database metrics
	m.connectionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_connection_errors_total",
			Help: "Total number of PostgreSQL connection errors",
		},
		[]string{"operator", "database", "error_type"},
	)

	m.connectionAcquireTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_connection_acquire_seconds",
			Help:    "Time taken to acquire a database connection",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // From 1ms to ~1s
		},
		[]string{"operator", "database"},
	)

	m.pingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_ping_duration_seconds",
			Help:    "Duration of PostgreSQL ping operations",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 8), // From 1ms to ~0.25s
		},
		[]string{"operator", "database"},
	)

	// Transaction metrics
	m.transactionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_transactions_total",
			Help: "Total number of PostgreSQL transactions",
		},
		[]string{"operator", "database", "status"}, // status: committed, rolled_back
	)

	m.transactionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_transaction_duration_seconds",
			Help:    "Duration of PostgreSQL transactions",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12), // From 1ms to ~4s
		},
		[]string{"operator", "database"},
	)

	m.transactionOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_transaction_operations_total",
			Help: "Total number of operations within PostgreSQL transactions",
		},
		[]string{"operator", "database", "operation_type"}, // operation_type: query, exec
	)

	m.transactionRollbacks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_transaction_rollbacks_total",
			Help: "Total number of PostgreSQL transaction rollbacks",
		},
		[]string{"operator", "database", "reason"}, // reason: error, explicit
	)

	m.transactionSavepoints = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_transaction_savepoints_total",
			Help: "Total number of PostgreSQL transaction savepoints",
		},
		[]string{"operator", "database", "operation"}, // operation: create, release, rollback_to
	)

	// Batch metrics
	m.batchOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_batch_operations_total",
			Help: "Total number of PostgreSQL batch operations",
		},
		[]string{"operator", "database"},
	)

	m.batchOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_batch_operation_duration_seconds",
			Help:    "Duration of PostgreSQL batch operations",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // From 1ms to ~1s
		},
		[]string{"operator", "database"},
	)

	m.batchSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_batch_size",
			Help:    "Size of PostgreSQL batch operations",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"operator", "database"},
	)

	// Copy metrics
	m.copyOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_copy_operations_total",
			Help: "Total number of PostgreSQL COPY operations",
		},
		[]string{"operator", "database", "table", "status"},
	)

	m.copyRowsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_copy_rows_processed_total",
			Help: "Total number of rows processed in PostgreSQL COPY operations",
		},
		[]string{"operator", "database", "table"},
	)

	m.copyDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    namespace + "postgres_copy_duration_seconds",
			Help:    "Duration of PostgreSQL COPY operations",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // From 10ms to ~10s
		},
		[]string{"operator", "database", "table"},
	)

	// Listen/Notify metrics
	m.notificationsReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_notifications_received_total",
			Help: "Total number of PostgreSQL notifications received",
		},
		[]string{"operator", "database", "channel"},
	)

	m.listenerErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_listener_errors_total",
			Help: "Total number of PostgreSQL listener errors",
		},
		[]string{"operator", "database", "channel", "error_type"},
	)

	// Error metrics
	m.operationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: namespace + "postgres_operation_errors_total",
			Help: "Total number of errors during PostgreSQL operations",
		},
		[]string{"operator", "database", "operation", "error_type"},
	)

	return m
}

// QueryTotal increments the query counter
func (m *PostgresMetrics) QueryTotal(operator, database, queryType, status string) {
	m.queriesTotal.WithLabelValues(operator, database, queryType, status).Inc()
}

// ObserveQueryDuration records the duration of a query
func (m *PostgresMetrics) ObserveQueryDuration(operator, database, queryType, table string, durationSeconds float64) {
	m.queryDuration.WithLabelValues(operator, database, queryType, table).Observe(durationSeconds)
}

// RowsProcessed increments the rows processed counter
func (m *PostgresMetrics) RowsProcessed(operator, database, operation, table string, count int) {
	m.rowsProcessed.WithLabelValues(operator, database, operation, table).Add(float64(count))
}

// PreparedQueryUsage increments the prepared query usage counter
func (m *PostgresMetrics) PreparedQueryUsage(operator, database, queryName string) {
	m.preparedQueryUsage.WithLabelValues(operator, database, queryName).Inc()
}

// SetPoolActiveConnections sets the active connections gauge
func (m *PostgresMetrics) SetPoolActiveConnections(operator, database string, count int) {
	m.poolActiveConnections.WithLabelValues(operator, database).Set(float64(count))
}

// SetPoolIdleConnections sets the idle connections gauge
func (m *PostgresMetrics) SetPoolIdleConnections(operator, database string, count int) {
	m.poolIdleConnections.WithLabelValues(operator, database).Set(float64(count))
}

// SetPoolTotalConnections sets the total connections gauge
func (m *PostgresMetrics) SetPoolTotalConnections(operator, database string, count int) {
	m.poolTotalConnections.WithLabelValues(operator, database).Set(float64(count))
}

// SetPoolMaxConnections sets the max connections gauge
func (m *PostgresMetrics) SetPoolMaxConnections(operator, database string, count int) {
	m.poolMaxConnections.WithLabelValues(operator, database).Set(float64(count))
}

// PoolConnectionRequest increments the connection requests counter
func (m *PostgresMetrics) PoolConnectionRequest(operator, database string) {
	m.poolConnectionRequests.WithLabelValues(operator, database).Inc()
}

// PoolConnectionTimeout increments the connection timeout counter
func (m *PostgresMetrics) PoolConnectionTimeout(operator, database string) {
	m.poolConnectionTimeouts.WithLabelValues(operator, database).Inc()
}

// ObservePoolConnectionWaitTime records the time spent waiting for a connection
func (m *PostgresMetrics) ObservePoolConnectionWaitTime(operator, database string, durationSeconds float64) {
	m.poolConnectionWaitTime.WithLabelValues(operator, database).Observe(durationSeconds)
}

// ObservePoolConnectionLifetime records the lifetime of a connection
func (m *PostgresMetrics) ObservePoolConnectionLifetime(operator, database string, durationSeconds float64) {
	m.poolConnectionLifetime.WithLabelValues(operator, database).Observe(durationSeconds)
}

// ConnectionError increments the connection error counter
func (m *PostgresMetrics) ConnectionError(operator, database, errorType string) {
	m.connectionErrors.WithLabelValues(operator, database, errorType).Inc()
}

// ObserveConnectionAcquireTime records the time taken to acquire a connection
func (m *PostgresMetrics) ObserveConnectionAcquireTime(operator, database string, durationSeconds float64) {
	m.connectionAcquireTime.WithLabelValues(operator, database).Observe(durationSeconds)
}

// ObservePingDuration records the duration of a ping operation
func (m *PostgresMetrics) ObservePingDuration(operator, database string, durationSeconds float64) {
	m.pingDuration.WithLabelValues(operator, database).Observe(durationSeconds)
}

// TransactionTotal increments the transaction counter
func (m *PostgresMetrics) TransactionTotal(operator, database, status string) {
	m.transactionsTotal.WithLabelValues(operator, database, status).Inc()
}

// ObserveTransactionDuration records the duration of a transaction
func (m *PostgresMetrics) ObserveTransactionDuration(operator, database string, durationSeconds float64) {
	m.transactionDuration.WithLabelValues(operator, database).Observe(durationSeconds)
}

// TransactionOperation increments the transaction operation counter
func (m *PostgresMetrics) TransactionOperation(operator, database, operationType string) {
	m.transactionOperations.WithLabelValues(operator, database, operationType).Inc()
}

// TransactionRollback increments the transaction rollback counter
func (m *PostgresMetrics) TransactionRollback(operator, database, reason string) {
	m.transactionRollbacks.WithLabelValues(operator, database, reason).Inc()
}

// TransactionSavepoint increments the transaction savepoint counter
func (m *PostgresMetrics) TransactionSavepoint(operator, database, operation string) {
	m.transactionSavepoints.WithLabelValues(operator, database, operation).Inc()
}

// BatchOperationTotal increments the batch operation counter
func (m *PostgresMetrics) BatchOperationTotal(operator, database string) {
	m.batchOperationsTotal.WithLabelValues(operator, database).Inc()
}

// ObserveBatchOperationDuration records the duration of a batch operation
func (m *PostgresMetrics) ObserveBatchOperationDuration(operator, database string, durationSeconds float64) {
	m.batchOperationDuration.WithLabelValues(operator, database).Observe(durationSeconds)
}

// ObserveBatchSize records the size of a batch operation
func (m *PostgresMetrics) ObserveBatchSize(operator, database string, size int) {
	m.batchSize.WithLabelValues(operator, database).Observe(float64(size))
}

// CopyOperationTotal increments the copy operation counter
func (m *PostgresMetrics) CopyOperationTotal(operator, database, table, status string) {
	m.copyOperationsTotal.WithLabelValues(operator, database, table, status).Inc()
}

// CopyRowsProcessed increments the copy rows processed counter
func (m *PostgresMetrics) CopyRowsProcessed(operator, database, table string, count int64) {
	m.copyRowsProcessed.WithLabelValues(operator, database, table).Add(float64(count))
}

// ObserveCopyDuration records the duration of a copy operation
func (m *PostgresMetrics) ObserveCopyDuration(operator, database, table string, durationSeconds float64) {
	m.copyDuration.WithLabelValues(operator, database, table).Observe(durationSeconds)
}

// NotificationReceived increments the notification received counter
func (m *PostgresMetrics) NotificationReceived(operator, database, channel string) {
	m.notificationsReceived.WithLabelValues(operator, database, channel).Inc()
}

// ListenerError increments the listener error counter
func (m *PostgresMetrics) ListenerError(operator, database, channel, errorType string) {
	m.listenerErrors.WithLabelValues(operator, database, channel, errorType).Inc()
}

// OperationError increments the operation error counter
func (m *PostgresMetrics) OperationError(operator, database, operation, errorType string) {
	m.operationErrors.WithLabelValues(operator, database, operation, errorType).Inc()
}