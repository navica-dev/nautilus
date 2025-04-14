package plugins

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/navica-dev/nautilus/pkg/plugin"
	"github.com/rs/zerolog/log"
)

// PostgresPlugin implements the nautilus.DatabasePlugin interface for PostgreSQL
type PostgresPlugin struct {
	// Configuration
	config *PostgresPluginConfig
	pool   *pgxpool.Pool

	// State
	connected bool

	// Metrics
	metrics *PostgresMetrics
}

type PostgresPluginConfig struct {
	Operator        string        `json:"operator"`
	ConnString      string        `json:"connString"`
	MaxRetries      int           `json:"maxRetries"`
	RetryDelay      time.Duration `json:"retryDelay"`
	MaxConns        int           `json:"maxConns"`
	MinConns        int           `json:"minConns"`
	MaxConnIdleTime time.Duration `json:"maxConnIdleTime"`
	MaxConnLifetime time.Duration `json:"maxConnLifetime"`
	dbName          string
}

func (c *PostgresPluginConfig) validate() error {
	if c.Operator == "" {
		return fmt.Errorf("operator is required")
	}

	if c.ConnString == "" {
		return fmt.Errorf("connString is required")
	}

	if c.MaxRetries <= 0 {
		return fmt.Errorf("maxRetries must be greater than 0")
	}

	if c.RetryDelay <= 0 {
		return fmt.Errorf("retryDelay must be greater than 0")
	}

	if 0 < c.MaxConns && c.MaxConns < c.MinConns {
		return fmt.Errorf("maxConns must be greater than or equal to minConns")
	}

	if c.MaxConnIdleTime <= 0 {
		return fmt.Errorf("maxIdleTime must be greater than 0")
	}

	if c.MaxConnLifetime <= 0 {
		return fmt.Errorf("maxLifetime must be greater than 0")
	}

	return nil
}

// NewPostgresPlugin creates a new PostgreSQL plugin
func NewPostgresPlugin(config *PostgresPluginConfig, namespace string) *PostgresPlugin {
	if err := config.validate(); err != nil {
		log.Fatal().Err(err).Msg("Invalid PostgreSQL plugin configuration")
	}

	config.dbName = extractDBName(config.ConnString)

	return &PostgresPlugin{
		config:  config,
		metrics: NewPostgresMetrics(namespace),
	}
}

func extractDBName(connString string) string {
	// Look for dbname parameter in the connection string
	if parts := strings.Split(connString, " "); len(parts) > 0 {
		for _, part := range parts {
			if strings.HasPrefix(part, "dbname=") {
				return strings.TrimPrefix(part, "dbname=")
			}
		}
	}
	return "unknown"
}

// Name returns the plugin name
func (p *PostgresPlugin) Name() string {
	return "postgres-plugin"
}

// Initialize sets up the plugin
func (p *PostgresPlugin) Initialize(ctx context.Context) error {
	log.Info().Msg("Initializing PostgreSQL plugin")
	return p.Connect(ctx)
}

// Connect establishes a connection to the database
func (p *PostgresPlugin) Connect(ctx context.Context) error {
	if p.connected && p.pool != nil {
		return nil // Already connected
	}

	var err error
	startTime := time.Now()

	// Parse the connection config
	config, err := pgxpool.ParseConfig(p.config.ConnString)
	if err != nil {
		p.metrics.ConnectionError(p.config.Operator, p.config.dbName, "parse_config")
		return fmt.Errorf("failed to parse PostgreSQL connection string: %w", err)
	}

	config.MaxConns = int32(p.config.MaxConns)
	config.MinConns = int32(p.config.MinConns)
	config.MaxConnLifetime = p.config.MaxConnLifetime
	config.MaxConnIdleTime = p.config.MaxConnIdleTime

	config.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
		p.metrics.PoolConnectionRequest(p.config.Operator, p.config.dbName)
		return true
	}

	var pool *pgxpool.Pool
	// Try to connect with retries
	for i := range p.config.MaxRetries {
		acquireStartTime := time.Now()
		pool, err = pgxpool.NewWithConfig(ctx, config)
		acquireDuration := time.Since(acquireStartTime).Seconds()
		p.metrics.ObserveConnectionAcquireTime(p.config.Operator, p.config.dbName, acquireDuration)

		if err != nil {
			p.metrics.ConnectionError(p.config.Operator, p.config.dbName, "create_pool")
			log.Error().Err(err).Int("attempt", i+1).Msg("Failed to create PostgreSQL connection pool")
			select {
			case <-time.After(p.config.RetryDelay):
				// Retry after delay
			case <-ctx.Done():
				p.metrics.ConnectionError(p.config.Operator, p.config.dbName, "context_cancelled")
				return fmt.Errorf("context cancelled while connecting to PostgreSQL: %w", ctx.Err())
			}
			continue
		}

		// Test the connection
		pingStartTime := time.Now()
		err = pool.Ping(ctx)
		pingDuration := time.Since(pingStartTime).Seconds()
		p.metrics.ObservePingDuration(p.config.Operator, p.config.dbName, pingDuration)

		if err == nil {
			break // Successfully connected
		}

		log.Error().Err(err).Int("attempt", i+1).Msg("Failed to ping PostgreSQL")
		pool.Close()

		p.metrics.ConnectionError(p.config.Operator, p.config.dbName, "ping_failed")
		log.Error().Err(err).Int("attempt", i+1).Msg("Failed to ping PostgreSQL")
		pool.Close()

		select {
		case <-time.After(p.config.RetryDelay):
			// Retry after delay
		case <-ctx.Done():
			p.metrics.ConnectionError(p.config.Operator, p.config.dbName, "context_cancelled")
			return fmt.Errorf("context cancelled while testing PostgreSQL connection: %w", ctx.Err())
		}
	}

	if err != nil {
		p.metrics.ConnectionError(p.config.Operator, p.config.dbName, "max_retries_exceeded")
		return fmt.Errorf("failed to connect to PostgreSQL after %d attempts: %w", p.config.MaxRetries, err)
	}

	p.pool = pool
	p.connected = true

	stats := pool.Stat()
	p.metrics.SetPoolTotalConnections(p.config.Operator, p.config.dbName, int(stats.TotalConns()))
	p.metrics.SetPoolActiveConnections(p.config.Operator, p.config.dbName, int(stats.AcquiredConns()))
	p.metrics.SetPoolIdleConnections(p.config.Operator, p.config.dbName, int(stats.IdleConns()))
	p.metrics.SetPoolMaxConnections(p.config.Operator, p.config.dbName, int(config.MaxConns))

	connectDuration := time.Since(startTime).Seconds()
	log.Info().Float64("duration_seconds", connectDuration).Msg("Connected to PostgreSQL")
	return nil
}

// Disconnect closes the database connection
func (p *PostgresPlugin) Disconnect(ctx context.Context) error {
	if p.pool != nil {
		p.pool.Close()
		p.connected = false
		p.pool = nil

		// Reset connection metrics
		p.metrics.SetPoolTotalConnections(p.config.Operator, p.config.dbName, 0)
		p.metrics.SetPoolActiveConnections(p.config.Operator, p.config.dbName, 0)
		p.metrics.SetPoolIdleConnections(p.config.Operator, p.config.dbName, 0)
	}
	return nil
}

// Ping verifies the database connection is alive
func (p *PostgresPlugin) Ping(ctx context.Context) error {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "ping", "not_connected")
		return fmt.Errorf("not connected to PostgreSQL")
	}

	startTime := time.Now()
	err := p.pool.Ping(ctx)
	duration := time.Since(startTime).Seconds()
	p.metrics.ObservePingDuration(p.config.Operator, p.config.dbName, duration)

	if err != nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "ping", "failed")
	}

	return err
}

// GetPool returns the underlying connection pool
func (p *PostgresPlugin) GetPool() *pgxpool.Pool {
	return p.pool
}

// UpdatePoolMetrics updates connection pool metrics
func (p *PostgresPlugin) UpdatePoolMetrics() {
	if p.pool != nil {
		stats := p.pool.Stat()
		p.metrics.SetPoolTotalConnections(p.config.Operator, p.config.dbName, int(stats.TotalConns()))
		p.metrics.SetPoolActiveConnections(p.config.Operator, p.config.dbName, int(stats.AcquiredConns()))
		p.metrics.SetPoolIdleConnections(p.config.Operator, p.config.dbName, int(stats.IdleConns()))
	}
}

// Terminate cleans up resources
func (p *PostgresPlugin) Terminate(ctx context.Context) error {
	log.Info().Msg("Terminating PostgreSQL plugin")
	return p.Disconnect(ctx)
}

// extractTableFromQuery attempts to extract the table name from a SQL query
func extractTableFromQuery(query string) string {
	// Simplified implementation - in real scenarios, you'd want to use a SQL parser
	query = strings.ToLower(query)

	// Try to find table name after FROM or UPDATE or INSERT INTO
	fromIndex := strings.Index(query, " from ")
	if fromIndex >= 0 {
		afterFrom := query[fromIndex+6:]
		// Take until next space or where clause
		endIndex := strings.IndexAny(afterFrom, " \t\n\r,;")
		if endIndex >= 0 {
			return strings.TrimSpace(afterFrom[:endIndex])
		}
		return strings.TrimSpace(afterFrom)
	}

	updateIndex := strings.Index(query, "update ")
	if updateIndex >= 0 {
		afterUpdate := query[updateIndex+7:]
		endIndex := strings.IndexAny(afterUpdate, " \t\n\r,;")
		if endIndex >= 0 {
			return strings.TrimSpace(afterUpdate[:endIndex])
		}
		return strings.TrimSpace(afterUpdate)
	}

	insertIndex := strings.Index(query, "insert into ")
	if insertIndex >= 0 {
		afterInsert := query[insertIndex+12:]
		endIndex := strings.IndexAny(afterInsert, " \t\n\r(,;")
		if endIndex >= 0 {
			return strings.TrimSpace(afterInsert[:endIndex])
		}
		return strings.TrimSpace(afterInsert)
	}

	// Default if we can't parse it
	return "unknown"
}

// determineQueryType determines the query type from a SQL statement
func determineQueryType(query string) string {
	query = strings.ToLower(strings.TrimSpace(query))

	if strings.HasPrefix(query, "select") {
		return "select"
	} else if strings.HasPrefix(query, "insert") {
		return "insert"
	} else if strings.HasPrefix(query, "update") {
		return "update"
	} else if strings.HasPrefix(query, "delete") {
		return "delete"
	} else if strings.HasPrefix(query, "create") {
		return "create"
	} else if strings.HasPrefix(query, "alter") {
		return "alter"
	} else if strings.HasPrefix(query, "drop") {
		return "drop"
	} else if strings.HasPrefix(query, "truncate") {
		return "truncate"
	} else {
		return "unknown"
	}
}

// ExecuteQuery executes a query and returns rows
func (p *PostgresPlugin) ExecuteQuery(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "query", "not_connected")
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	queryType := determineQueryType(query)
	tableName := extractTableFromQuery(query)

	startTime := time.Now()
	rows, err := p.pool.Query(ctx, query, args...)
	duration := time.Since(startTime).Seconds()

	// Update metrics
	p.metrics.ObserveQueryDuration(p.config.Operator, p.config.dbName, queryType, tableName, duration)

	if err != nil {
		p.metrics.QueryTotal(p.config.Operator, p.config.dbName, queryType, "error")
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "query", "execution_failed")
	} else {
		p.metrics.QueryTotal(p.config.Operator, p.config.dbName, queryType, "success")
	}

	p.UpdatePoolMetrics()

	return rows, err
}

// ExecuteQueryRow executes a query and returns a single row
func (p *PostgresPlugin) ExecuteQueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "query_row", "not_connected")
		return nil
	}

	queryType := determineQueryType(query)
	tableName := extractTableFromQuery(query)

	startTime := time.Now()
	row := p.pool.QueryRow(ctx, query, args...)
	duration := time.Since(startTime).Seconds()

	// Update metrics
	p.metrics.ObserveQueryDuration(p.config.Operator, p.config.dbName, queryType, tableName, duration)
	p.metrics.QueryTotal(p.config.Operator, p.config.dbName, queryType, "success")

	p.UpdatePoolMetrics()

	return row
}

// ExecuteCommand executes a command and returns the number of affected rows
func (p *PostgresPlugin) ExecuteCommand(ctx context.Context, command string, args ...any) (int64, error) {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "exec", "not_connected")
		return 0, fmt.Errorf("not connected to PostgreSQL")
	}

	queryType := determineQueryType(command)
	tableName := extractTableFromQuery(command)

	startTime := time.Now()
	result, err := p.pool.Exec(ctx, command, args...)
	duration := time.Since(startTime).Seconds()

	// Update metrics
	p.metrics.ObserveQueryDuration(p.config.Operator, p.config.dbName, queryType, tableName, duration)

	if err != nil {
		p.metrics.QueryTotal(p.config.Operator, p.config.dbName, queryType, "error")
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "exec", "execution_failed")
		return 0, err
	}

	p.metrics.QueryTotal(p.config.Operator, p.config.dbName, queryType, "success")
	rowsAffected := result.RowsAffected()
	if rowsAffected > 0 {
		p.metrics.RowsProcessed(p.config.Operator, p.config.dbName, queryType, tableName, int(rowsAffected))
	}

	p.UpdatePoolMetrics()

	return rowsAffected, nil
}

// CreateTransaction starts a new transaction
func (p *PostgresPlugin) CreateTransaction(ctx context.Context) (pgx.Tx, error) {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "transaction", "not_connected")
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	startTime := time.Now()
	tx, err := p.pool.Begin(ctx)
	duration := time.Since(startTime).Seconds()

	// Update metrics
	p.metrics.ObserveTransactionDuration(p.config.Operator, p.config.dbName, duration)

	if err != nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "transaction_begin", "failed")
		return nil, err
	}

	p.metrics.TransactionTotal(p.config.Operator, p.config.dbName, "started")

	// Update connection pool metrics
	p.UpdatePoolMetrics()

	// Return a wrapped transaction to track metrics on commit/rollback
	return &instrumentedTx{
		Tx:        tx,
		metrics:   p.metrics,
		dbName:    p.config.dbName,
		operator:  p.config.Operator,
		plugin:    p,
		startTime: startTime,
	}, nil
}

// instrumentedTx wraps pgx.Tx to add metrics
type instrumentedTx struct {
	pgx.Tx
	metrics   *PostgresMetrics
	dbName    string
	operator  string
	plugin    *PostgresPlugin
	startTime time.Time
}

// Commit wraps the transaction commit with metrics
func (tx *instrumentedTx) Commit(ctx context.Context) error {
	err := tx.Tx.Commit(ctx)
	duration := time.Since(tx.startTime).Seconds()

	if err != nil {
		tx.metrics.TransactionTotal(tx.operator, tx.dbName, "commit_error")
		tx.metrics.OperationError(tx.operator, tx.dbName, "transaction_commit", "failed")
	} else {
		tx.metrics.TransactionTotal(tx.operator, tx.dbName, "committed")
		tx.metrics.ObserveTransactionDuration(tx.operator, tx.dbName, duration)
	}

	// Update connection pool metrics
	tx.plugin.UpdatePoolMetrics()

	return err
}

// Rollback wraps the transaction rollback with metrics
func (tx *instrumentedTx) Rollback(ctx context.Context) error {
	err := tx.Tx.Rollback(ctx)
	duration := time.Since(tx.startTime).Seconds()

	if err != nil {
		tx.metrics.OperationError(tx.operator, tx.dbName, "transaction_rollback", "failed")
	} else {
		tx.metrics.TransactionTotal(tx.operator, tx.dbName, "rolled_back")
		tx.metrics.TransactionRollback(tx.operator, tx.dbName, "explicit")
		tx.metrics.ObserveTransactionDuration(tx.operator, tx.dbName, duration)
	}

	// Update connection pool metrics
	tx.plugin.UpdatePoolMetrics()

	return err
}

// BatchOperations executes multiple operations in a batch
func (p *PostgresPlugin) BatchOperations(ctx context.Context) (*pgx.Batch, error) {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "batch", "not_connected")
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	batch := &pgx.Batch{}
	p.metrics.BatchOperationTotal(p.config.Operator, p.config.dbName)

	return batch, nil
}

// ExecuteBatch executes a batch of operations
func (p *PostgresPlugin) ExecuteBatch(ctx context.Context, batch *pgx.Batch) (pgx.BatchResults, error) {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "batch_exec", "not_connected")
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	// Record batch size
	batchSize := batch.Len()
	p.metrics.ObserveBatchSize(p.config.Operator, p.config.dbName, batchSize)

	startTime := time.Now()
	batchResults := p.pool.SendBatch(ctx, batch)
	duration := time.Since(startTime).Seconds()

	// Update metrics
	p.metrics.ObserveBatchOperationDuration(p.config.Operator, p.config.dbName, duration)

	p.UpdatePoolMetrics()

	return batchResults, nil
}

// CopyFrom efficiently copies data from a source to a table
func (p *PostgresPlugin) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	if p.pool == nil {
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "copy_from", "not_connected")
		return 0, fmt.Errorf("not connected to PostgreSQL")
	}

	table := string(tableName[0])

	startTime := time.Now()
	rowsAffected, err := p.pool.CopyFrom(ctx, tableName, columnNames, rowSrc)
	duration := time.Since(startTime).Seconds()

	p.metrics.ObserveCopyDuration(p.config.Operator, p.config.dbName, table, duration)

	if err != nil {
		p.metrics.CopyOperationTotal(p.config.Operator, p.config.dbName, table, "error")
		p.metrics.OperationError(p.config.Operator, p.config.dbName, "copy_from", "failed")
		return 0, err
	}

	p.metrics.CopyOperationTotal(p.config.Operator, p.config.dbName, table, "success")
	p.metrics.CopyRowsProcessed(p.config.Operator, p.config.dbName, table, rowsAffected)

	p.UpdatePoolMetrics()

	return rowsAffected, nil
}

// Listen establishes a LISTEN/NOTIFY connection
func (p *PostgresPlugin) Listen(ctx context.Context, channel string) (*pgx.Conn, error) {
	if p.pool == nil {
		p.metrics.ListenerError(p.config.Operator, p.config.dbName, channel, "not_connected")
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}

	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		p.metrics.ListenerError(p.config.Operator, p.config.dbName, channel, "acquire_connection")
		return nil, fmt.Errorf("failed to acquire connection for LISTEN: %w", err)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("LISTEN %s", channel))
	if err != nil {
		conn.Release()
		p.metrics.ListenerError(p.config.Operator, p.config.dbName, channel, "listen_command")
		return nil, fmt.Errorf("failed to LISTEN on channel %s: %w", channel, err)
	}

	p.UpdatePoolMetrics()

	return conn.Conn(), nil
}

// HandleNotification should be called when a notification is received
func (p *PostgresPlugin) HandleNotification(channel string) {
	p.metrics.NotificationReceived(p.config.Operator, p.config.dbName, channel)
}

var _ plugin.DatabasePlugin = (*PostgresPlugin)(nil)
