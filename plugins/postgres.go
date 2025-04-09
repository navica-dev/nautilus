package plugins

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/rs/zerolog/log"
)

// PostgresPlugin implements the nautilus.DatabasePlugin interface for PostgreSQL
type PostgresPlugin struct {
	// Configuration
	connString string
	db         *sql.DB
	maxRetries int
	retryDelay time.Duration

	// State
	connected bool
}

// NewPostgresPlugin creates a new PostgreSQL plugin
func NewPostgresPlugin(connString string, maxRetries int, retryDelay time.Duration) *PostgresPlugin {
	if maxRetries <= 0 {
		maxRetries = 5
	}
	if retryDelay <= 0 {
		retryDelay = 5 * time.Second
	}

	return &PostgresPlugin{
		connString: connString,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
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
	if p.connected && p.db != nil {
		return nil // Already connected
	}

	var err error
	var db *sql.DB

	// Try to connect with retries
	for i := 0; i < p.maxRetries; i++ {
		db, err = sql.Open("postgres", p.connString)
		if err != nil {
			log.Error().Err(err).Int("attempt", i+1).Msg("Failed to open PostgreSQL connection")
			select {
			case <-time.After(p.retryDelay):
				// Retry after delay
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while connecting to PostgreSQL: %w", ctx.Err())
			}
			continue
		}

		// Test the connection
		err = db.PingContext(ctx)
		if err == nil {
			break // Successfully connected
		}

		log.Error().Err(err).Int("attempt", i+1).Msg("Failed to ping PostgreSQL")
		db.Close()

		select {
		case <-time.After(p.retryDelay):
			// Retry after delay
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while testing PostgreSQL connection: %w", ctx.Err())
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL after %d attempts: %w", p.maxRetries, err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	p.db = db
	p.connected = true
	log.Info().Msg("Successfully connected to PostgreSQL")
	return nil
}

// Disconnect closes the database connection
func (p *PostgresPlugin) Disconnect(ctx context.Context) error {
	if p.db != nil {
		err := p.db.Close()
		p.connected = false
		p.db = nil
		if err != nil {
			return fmt.Errorf("error closing PostgreSQL connection: %w", err)
		}
	}
	return nil
}

// Ping verifies the database connection is alive
func (p *PostgresPlugin) Ping(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("not connected to PostgreSQL")
	}
	return p.db.PingContext(ctx)
}

// GetDB returns the underlying database connection
func (p *PostgresPlugin) GetDB() *sql.DB {
	return p.db
}

// Terminate cleans up resources
func (p *PostgresPlugin) Terminate(ctx context.Context) error {
	log.Info().Msg("Terminating PostgreSQL plugin")
	return p.Disconnect(ctx)
}

// ExecuteQuery executes a query and returns rows
func (p *PostgresPlugin) ExecuteQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}
	return p.db.QueryContext(ctx, query, args...)
}

// ExecuteQueryRow executes a query and returns a single row
func (p *PostgresPlugin) ExecuteQueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if p.db == nil {
		return nil
	}
	return p.db.QueryRowContext(ctx, query, args...)
}

// ExecuteCommand executes a command and returns the number of affected rows
func (p *PostgresPlugin) ExecuteCommand(ctx context.Context, command string, args ...interface{}) (int64, error) {
	if p.db == nil {
		return 0, fmt.Errorf("not connected to PostgreSQL")
	}

	result, err := p.db.ExecContext(ctx, command, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// CreateTransaction starts a new transaction
func (p *PostgresPlugin) CreateTransaction(ctx context.Context) (*sql.Tx, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to PostgreSQL")
	}
	return p.db.BeginTx(ctx, nil)
}

var _ DatabasePlugin = (*PostgresPlugin)(nil)
