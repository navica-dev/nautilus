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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var name = "basic-operator"

// BasicOperator is a simple example operator
type BasicOperator struct {
	// Configuration
	name        string
	description string
	count       int

	// State
	running bool
}

func main() {
	// Initialize logging
	logging.Setup()

	// Parse command line arguments
	configPath := ""
	if len(os.Args) > 1 {
		configPath = os.Args[1]
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

	// Create operator
	op := &BasicOperator{
		name:        name,
		description: "A simple example operator",
	}

	// Create Nautilus instance
	n, err := core.New(
		core.WithConfigPath(configPath),
		core.WithName(op.name),
		core.WithDescription(op.description),
		core.WithVersion("0.0.1"),
		core.WithLogLevel(zerolog.LevelDebugValue),
		core.WithLogFormat(enums.LogFormatConsole),
		core.WithInterval(10*time.Second),
		core.WithAPI(true, 12911),
		core.WithMetrics(true),
		core.WithMaxConsecutiveFailures(3),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Nautilus instance")
	}

	// Run operator
	if err := n.Run(ctx, op); err != nil {
		log.Fatal().Err(err).Msg("error running operator")
	}
}

var _ interfaces.Operator = (*BasicOperator)(nil)

// Initialize prepares the robot for execution
func (r *BasicOperator) Initialize(ctx context.Context) error {
	r.count = 0
	r.running = true
	return nil
}

// Run performs the operator's main task
func (r *BasicOperator) Run(ctx context.Context) error {
	r.count++
	log.Info().Int("run", r.count).Msg("Running BasicOperator")

	// Simulate some work
	select {
	case <-time.After(2 * time.Second):
		log.Info().Int("run", 2*r.count).Msg("BasicOperator completed task")
	case <-ctx.Done():
		return fmt.Errorf("operator execution cancelled: %w", ctx.Err())
	}

	// Simulate occasional failures for testing
	if r.count%5 == 0 {
		return fmt.Errorf("simulated failure on run %d", r.count)
	}

	return nil
}

// Terminate cleans up resources
func (r *BasicOperator) Terminate(ctx context.Context) error {
	log.Info().Msg("Terminating BasicOperator")
	r.running = false
	return nil
}

var _ interfaces.HealthCheck = (*BasicOperator)(nil)

func (r *BasicOperator) Name() string {
	return name
}

func (r *BasicOperator) HealthCheck(ctx context.Context) error {
	if !r.running {
		return fmt.Errorf("operator is not running")
	}
	return nil
}
