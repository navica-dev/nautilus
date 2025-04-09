package nautilus

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/navica-dev/nautilus/internal/api"
	"github.com/navica-dev/nautilus/internal/config"
	"github.com/navica-dev/nautilus/internal/metrics"
	"github.com/navica-dev/nautilus/internal/parallel"
	"github.com/navica-dev/nautilus/pkg/enums"
	"github.com/navica-dev/nautilus/pkg/interfaces"
)

// Nautilus is the main orchestrator for operator execution
type Nautilus struct {
	// Configuration
	config *config.Config

	// Components
	apiServer     *api.Server
	metricsClient *metrics.Client

	// Execution
	cron        *cron.Cron
	workerPool  *parallel.Pool
	runCount    int
	lastRunTime time.Time

	// Run state tracking
	currentRun       *interfaces.RunInfo
	consecutiveFails int

	// Metadata
	version   string
	startTime time.Time

	// State
	mu           sync.RWMutex
	shutdownOnce sync.Once
	stopping     bool

	// Plugins
	plugins []Plugin

	// Logging
	logger zerolog.Logger
}

// New creates a new Nautilus instance with the provided options
func New(options ...Option) (*Nautilus, error) {
	// Create with defaults
	n := &Nautilus{
		config:    &config.Config{},
		startTime: time.Now(),
		version:   "dev",
		plugins:   make([]Plugin, 0),
	}

	// Apply options
	for _, option := range options {
		if err := option(n); err != nil {
			return nil, err
		}
	}

	n.logger = log.With().Str("component", "nautilus-core").Str("operator", n.config.Operator.Name).Logger()

	// Validate and initialize components
	if err := n.initialize(); err != nil {
		return nil, err
	}

	return n, nil
}

// initialize performs internal initialization for Nautilus
func (n *Nautilus) initialize() error {
	// Set up worker pool for parallel execution
	n.workerPool = parallel.NewPool(
		n.config.Execution.Parallel.Workers,
		n.config.Execution.Parallel.BufferSize,
	)

	// Set up scheduler for periodic execution
	n.cron = cron.New(cron.WithSeconds())

	// Set up metrics client
	if n.config.Metrics.Enabled {
		n.metricsClient = metrics.NewClient(n.config.Metrics)
		n.metricsClient.RegisterBasicMetrics(n.config.Operator.Name)
	}

	// Set up API server for health checks
	if n.config.API.Enabled {
		n.apiServer = api.NewServer(n.config.API, n.version)

		// Add Nautilus itself as a health checker
		n.apiServer.RegisterHealthChecker(n)
	}

	return nil
}

func (n *Nautilus) RegisterPlugin(plugin Plugin) {
	n.plugins = append(n.plugins, plugin)
}

func (n *Nautilus) Run(ctx context.Context, operator interfaces.Operator) error {
	n.logger.Info().
		Str("version", n.version).
		Msg("Starting operator")

	// Create a cancellation context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle signals in a separate goroutine
	go func() {
		select {
		case <-sigChan:
			n.logger.Info().Msg("Received shutdown signal, shutting down ...")
			n.Shutdown(ctx)
		case <-ctx.Done():
			// Context cancelled, clean shutdown
		}
	}()

	// Initialize plugins
	if err := n.initializePlugins(ctx); err != nil {
		return fmt.Errorf("plugin initialization failed: %w", err)
	}
	defer n.terminatePlugins(ctx)

	// Start API server if enabled
	if n.apiServer != nil {
		go func() {
			if err := n.apiServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				n.logger.Error().Err(err).Msg("API server error")
			}
		}()
	}

	// Initialize the operator
	n.logger.Info().Msg("Initializing operator")
	if err := operator.Initialize(ctx); err != nil {
		return fmt.Errorf("operator initialization failed: %w", err)
	}

	// Register health check if supported
	if healthChecker, ok := operator.(api.HealthCheck); ok && n.apiServer != nil {
		n.apiServer.RegisterHealthChecker(healthChecker)
	}

	// Execute based on configuration
	if n.config.Execution.RunOnce {
		// Run once and exit (unless WaitAfterCompletion is true)
		err := n.executeRun(ctx, operator)

		if !n.config.Execution.WaitAfterCompletion {
			n.Shutdown(ctx)
			return err
		}

		// Wait for shutdown signal
		<-ctx.Done()
		return err
	} else if n.config.Execution.Schedule != "" {
		// Use cron scheduler
		_, err := n.cron.AddFunc(n.config.Execution.Schedule, func() {
			if err := n.executeRun(ctx, operator); err != nil {
				n.logger.Error().Err(err).Msg("Scheduled run failed")
			}
		})
		if err != nil {
			return fmt.Errorf("failed to schedule run: %w", err)
		}

		n.cron.Start()
	} else if n.config.Execution.Interval > 0 {
		// Use interval-based execution
		go func() {
			ticker := time.NewTicker(n.config.Execution.Interval)
			defer ticker.Stop()

			// Execute immediately on start
			if err := n.executeRun(ctx, operator); err != nil {
				n.logger.Error().Err(err).Msg("Initial run failed")
			}

			for {
				select {
				case <-ticker.C:
					if err := n.executeRun(ctx, operator); err != nil {
						n.logger.Error().Err(err).Msg("Periodic run failed")
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	} else {
		return fmt.Errorf("invalid execution configuration: must specify RunOnce, Schedule, or Interval")
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Clean shutdown
	return n.performTermination(operator)
}

// executeRun performs a single execution of the operator
func (n *Nautilus) executeRun(ctx context.Context, operator interfaces.Operator) error {
	startTime := time.Now()
	n.mu.Lock()
	n.runCount++
	currentRun := n.runCount

	// Create run info
	runID := uuid.New().String()
	n.currentRun = &interfaces.RunInfo{
		RunID:     runID,
		StartTime: startTime,
		Status:    enums.RunStatusRunning,
	}
	n.mu.Unlock()

	// Create run context with timeout if configured
	runCtx := ctx
	if n.config.Execution.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, n.config.Execution.Timeout)
		defer cancel()
	}

	// Enrich context with run info
	runCtx = context.WithValue(runCtx, "nautilus_run_id", runID)
	runCtx = context.WithValue(runCtx, "nautilus_run_count", currentRun)

	// Track metrics
	n.logger.Info().
		Int("run", currentRun).
		Str("run_id", runID).
		Time("start_time", startTime).
		Msg("Starting operator run")

	if n.metricsClient != nil {
		n.metricsClient.RecordRunStart()
	}

	// Execute the operator
	err := operator.Run(runCtx)

	// Update metrics and run info
	duration := time.Since(startTime)

	n.mu.Lock()
	if n.currentRun != nil && n.currentRun.RunID == runID {
		n.currentRun.Duration = duration
		n.lastRunTime = startTime

		if err != nil {
			n.currentRun.Status = enums.RunStatusFailed
			n.currentRun.Error = err
			n.consecutiveFails++
		} else {
			n.currentRun.Status = enums.RunStatusCompleted
			n.consecutiveFails = 0
		}
	}
	n.mu.Unlock()

	if n.metricsClient != nil {
		if err != nil {
			n.metricsClient.RecordRunFailure(duration)
		} else {
			n.metricsClient.RecordRunSuccess(duration)
		}
	}

	// Log completion
	logEvent := n.logger.Info()
	if err != nil {
		logEvent = n.logger.Error().Err(err)
	}

	logEvent.
		Int("run", currentRun).
		Str("run_id", runID).
		Dur("duration", duration).
		Time("end_time", time.Now()).
		Msg("Operator run completed")

	return err
}

// initializePlugins initializes all registered plugins
func (n *Nautilus) initializePlugins(ctx context.Context) error {
	for _, plugin := range n.plugins {
		n.logger.Debug().Str("plugin", plugin.Name()).Msg("Initializing plugin")
		if err := plugin.Initialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize plugin %s: %w", plugin.Name(), err)
		}

		// Register health check if plugin implements HealthCheck interface
		if healthChecker, ok := plugin.(api.HealthCheck); ok && n.apiServer != nil {
			n.apiServer.RegisterHealthChecker(healthChecker)
		}
	}

	return nil
}

// terminatePlugins terminates all registered plugins
func (n *Nautilus) terminatePlugins(ctx context.Context) {
	for _, plugin := range n.plugins {
		n.logger.Debug().Str("plugin", plugin.Name()).Msg("Terminating plugin")
		if err := plugin.Terminate(ctx); err != nil {
			n.logger.Error().Err(err).Str("plugin", plugin.Name()).Msg("Failed to terminate plugin")
		}
	}
}

// Shutdown initiates a graceful shutdown of Nautilus
func (n *Nautilus) Shutdown(ctx context.Context) {
	n.shutdownOnce.Do(func() {
		n.mu.Lock()
		n.stopping = true
		n.mu.Unlock()

		n.logger.Info().Msg("Shutting down Nautilus")

		// Stop the scheduler if running
		if n.cron != nil {
			n.cron.Stop()
		}

		// Stop the API server
		if n.apiServer != nil {
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if err := n.apiServer.Stop(shutdownCtx); err != nil {
				n.logger.Error().Err(err).Msg("Failed to stop API server gracefully")
			}
		}

		// Clean up worker pool
		if n.workerPool != nil {
			n.workerPool.Stop()
		}
	})
}

// performTermination handles operator termination
func (n *Nautilus) performTermination(operator interfaces.Operator) error {
	terminateCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	n.logger.Info().Msgf("Terminating operator")
	return operator.Terminate(terminateCtx)
}

// Execute Parallel runs a function in the worker pool
func (n *Nautilus) ExecuteParallel(ctx context.Context, fn func(ctx context.Context) error) error {
	return n.workerPool.Execute(ctx, fn)
}

// GetCurrentRun return information about the current run
func (n *Nautilus) GetCurrentRun() *interfaces.RunInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.currentRun == nil {
		return nil
	}

	// Return a copy to avoid race conditions
	run := *n.currentRun
	return &run
}

// GetRunCount returns the total number of runs executed
func (n *Nautilus) GetRunCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.runCount
}

// GetLastRunTime returns the time of the last run
func (n *Nautilus) GetLastRunTime() time.Time {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.lastRunTime
}

// GetUptime returns the uptime of Nautilus
func (n *Nautilus) GetUptime() time.Duration {
	return time.Since(n.startTime)
}

// --- Heath Checker ---
var _ api.HealthCheck = (*Nautilus)(nil)

func (n *Nautilus) Name() string {
	return "nautilus-core"
}

func (n *Nautilus) HealthCheck(ctx context.Context) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Report unhealthy if we're shutting down
	if n.stopping {
		return fmt.Errorf("shutting down")
	}

	// Report unhealthy if too many consecutive failures
	maxFails := n.config.Health.MaxConsecutiveFailures
	if maxFails > 0 && n.consecutiveFails >= maxFails {
		return fmt.Errorf("too many consecutive failures: %d", n.consecutiveFails)
	}

	// Include basic health info
	return nil
}
