package interfaces

import (
	"context"
	"time"

	"github.com/navica-dev/nautilus/pkg/enums"
)

// Core interface for implementing nautilus Operators
type Operator interface {
	// Initialize prepares the nautilus operator for execution
	// It receives a context that may be used for cancellation
	Initialize(ctx context.Context) error

	// Run performs the operator's primary function
	// This method will be called either once or periodically depending on config
	Run(ctx context.Context) error

	// Terminate performs cleanup when the operator is shutting down
	// This allows for graceful release of resources
	Terminate(ctx context.Context) error
}

// Configurable is an optional interface operators can implement for more complex configuraton needs
type Configurable interface {
	// Configure allows a operator to configure itself from structured configuration
	Configure(config map[string]any) error
}

// RunInfo provides informations about a operator's execution
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
