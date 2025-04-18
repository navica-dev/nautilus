package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/navica-dev/nautilus/core"
	"github.com/navica-dev/nautilus/pkg/enums"
	"github.com/navica-dev/nautilus/pkg/interfaces"
	"github.com/navica-dev/nautilus/pkg/logging"
	plugin "github.com/navica-dev/nautilus/plugins/database"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var name = "postgres-operator"

// Product represents a product in our database
type Product struct {
	ID        int
	Name      string
	Price     float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PostgresOperator is an operator that interacts with PostgreSQL
type PostgresOperator struct {
	// Configuration
	name        string
	description string
	dbConfig    struct {
		connString string
		tableName  string
	}

	// Dependencies
	pgPlugin *plugin.PostgresPlugin // This would normally be imported from a package

	// State
	runCount int
	running  bool
}

func main() {
	// Initialize logging
	logging.Setup()

	// Parse command line arguments
	configPath := ""
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Get connection string from environment or use default
	connString := os.Getenv("POSTGRES_CONN_STRING")
	if connString == "" {
		connString = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	pgConfig := plugin.PostgresPluginConfig{
		Operator: name,
		ConnString: connString,
		MaxRetries: 5,
		RetryDelay: 5 * time.Second,
		MaxConns: 25,
		MinConns: 5,
		MaxConnIdleTime: 1 * time.Minute,
		MaxConnLifetime: 5 * time.Minute,
	}

	// Create PostgreSQL plugin
	pgPlugin := plugin.NewPostgresPlugin(&pgConfig, "")

	// Create operator
	op := &PostgresOperator{
		name:        name,
		description: "PostgreSQL database operator example",
		pgPlugin:    pgPlugin,
	}
	op.dbConfig.connString = connString
	op.dbConfig.tableName = "products"

	// Create Nautilus instance
	n, err := core.New(
		core.WithConfigPath(configPath),
		core.WithName(op.name),
		core.WithDescription(op.description),
		core.WithVersion("0.1.0"),
		core.WithLogLevel(zerolog.LevelDebugValue),
		core.WithLogFormat(enums.LogFormatConsole),
		core.WithInterval(5*time.Second),
		core.WithAPI(true, 12911),
		core.WithMetrics(true),
		core.WithMaxConsecutiveFailures(3),
		core.WithPlugin(pgPlugin), // Register the PostgreSQL plugin
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Nautilus instance")
	}

	// Run operator
	if err := n.Run(ctx, op); err != nil {
		log.Fatal().Err(err).Msg("error running operator")
	}
}

// Make sure PostgresOperator implements the Operator interface
var _ interfaces.Operator = (*PostgresOperator)(nil)

// Initialize prepares the operator for execution
func (o *PostgresOperator) Initialize(ctx context.Context) error {
	o.runCount = 0
	o.running = true

	// Ensure the database table exists
	err := o.ensureTableExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure table exists: %w", err)
	}

	return nil
}

// Run performs the operator's main task
func (o *PostgresOperator) Run(ctx context.Context) error {
	o.runCount++
	log.Info().Int("run", o.runCount).Msg("Running PostgresOperator")

	// 1. Fetch existing products
	products, err := o.getAllProducts(ctx)
	if err != nil {
		return fmt.Errorf("failed to get products: %w", err)
	}

	log.Info().Int("product_count", len(products)).Msg("Retrieved products")

	// 2. Create a new product (every 3rd run)
	if o.runCount%3 == 0 {
		newProduct := Product{
			Name:  fmt.Sprintf("Product %d", o.runCount),
			Price: float64(10 + (o.runCount % 100)),
		}

		id, err := o.createProduct(ctx, newProduct)
		if err != nil {
			return fmt.Errorf("failed to create product: %w", err)
		}

		log.Info().Int("id", id).Str("name", newProduct.Name).Msg("Created new product")
	}

	// 3. Update a product if we have any (every 5th run)
	if o.runCount%5 == 0 && len(products) > 0 {
		// Update the first product
		productToUpdate := products[0]
		productToUpdate.Name = fmt.Sprintf("%s - Updated", productToUpdate.Name)
		productToUpdate.Price = productToUpdate.Price * 1.1 // 10% price increase

		err := o.updateProduct(ctx, productToUpdate)
		if err != nil {
			return fmt.Errorf("failed to update product: %w", err)
		}

		log.Info().Int("id", productToUpdate.ID).Str("name", productToUpdate.Name).Msg("Updated product")
	}

	// 4. Delete a product if we have more than 10 (every 7th run)
	if o.runCount%7 == 0 && len(products) > 10 {
		// Delete the last product
		productToDelete := products[len(products)-1]

		err := o.deleteProduct(ctx, productToDelete.ID)
		if err != nil {
			return fmt.Errorf("failed to delete product: %w", err)
		}

		log.Info().Int("id", productToDelete.ID).Str("name", productToDelete.Name).Msg("Deleted product")
	}

	return nil
}

// Terminate cleans up resources
func (o *PostgresOperator) Terminate(ctx context.Context) error {
	log.Info().Msg("Terminating PostgresOperator")
	o.running = false
	return nil
}

// Health check implementation
var _ interfaces.HealthCheck = (*PostgresOperator)(nil)

func (o *PostgresOperator) Name() string {
	return name
}

func (o *PostgresOperator) HealthCheck(ctx context.Context) error {
	if !o.running {
		return fmt.Errorf("operator is not running")
	}

	// Check database connection
	if o.pgPlugin != nil {
		if err := o.pgPlugin.Ping(ctx); err != nil {
			return fmt.Errorf("database connection failed: %w", err)
		}
	}

	return nil
}

// Database operations

// ensureTableExists creates the products table if it doesn't exist
func (o *PostgresOperator) ensureTableExists(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			price DECIMAL(10, 2) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`, o.dbConfig.tableName)

	_, err := o.pgPlugin.ExecuteCommand(ctx, query)
	if err != nil {
		return err
	}

	return nil
}

// getAllProducts retrieves all products from the database
func (o *PostgresOperator) getAllProducts(ctx context.Context) ([]Product, error) {
	query := fmt.Sprintf("SELECT id, name, price, created_at, updated_at FROM %s", o.dbConfig.tableName)

	rows, err := o.pgPlugin.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
}

// createProduct inserts a new product
func (o *PostgresOperator) createProduct(ctx context.Context, product Product) (int, error) {
	query := fmt.Sprintf(
		"INSERT INTO %s (name, price, created_at, updated_at) VALUES ($1, $2, NOW(), NOW()) RETURNING id",
		o.dbConfig.tableName,
	)

	var id int
	err := o.pgPlugin.ExecuteQueryRow(ctx, query, product.Name, product.Price).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// updateProduct updates an existing product
func (o *PostgresOperator) updateProduct(ctx context.Context, product Product) error {
	query := fmt.Sprintf(
		"UPDATE %s SET name = $1, price = $2, updated_at = NOW() WHERE id = $3",
		o.dbConfig.tableName,
	)

	result, err := o.pgPlugin.ExecuteCommand(ctx, query, product.Name, product.Price, product.ID)
	if err != nil {
		return err
	}

	if result == 0 {
		return fmt.Errorf("product with ID %d not found", product.ID)
	}

	return nil
}

// deleteProduct removes a product
func (o *PostgresOperator) deleteProduct(ctx context.Context, id int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", o.dbConfig.tableName)

	result, err := o.pgPlugin.ExecuteCommand(ctx, query, id)
	if err != nil {
		return err
	}

	if result == 0 {
		return fmt.Errorf("product with ID %d not found", id)
	}

	return nil
}
