package interfaces

import (
	"context"
	"time"

	"github.com/navica-dev/nautilus/pkg/enums"
)

// Core interface for implementing nautilus Robots
type Robot interface {
	// Initialize prepares the nautilus Robot for execution
	// It receives a context that may be used for cancellation
	Initialize(ctx context.Context) error

	// Run performs the robot's primary function
	// This method will be called either once or periodically depending on config
	Run(ctx context.Context) error

	// Terminate performs cleanup when the robot is shutting down
	// This allows for graceful release of resources
	Terminate(ctx context.Context) error
}

// HealthCheck is an optional interface robots can implement
// to provide custom health status information
type HealthCheck interface {
	// Name returns the name of this health check component
	Name() string

	// HealthCheck reports on the health status of the robot
	// Returns nil if healthy, or an error describing the issue
	HealthCheck(ctx context.Context) error
}

// Configurable is an optional interface robots can implement for more complex configuraton needs
type Configurable interface {
	// Configure allows a robot to configure itself from structured configuration
	Configure(config map[string]any) error
}

// RunInfo provides informations about a robot's execution
type RunInfo struct {
	// RunID is a unique identifier for this execution
	RunID string

	// StartTime is when the execution began
	StartTime time.Time

	// Duration is how long the execution took (or has been running)
	Duration time.Duration

	// Status indicates the current state of the execution
	Status enums.RunStatusEnum

	Error error
}
