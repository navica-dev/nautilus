package api

import "context"

// HealthCheck is an optional interface operators can implement
// to provide custom health status information
type HealthCheck interface {
	// Name returns the name of this health check component
	Name() string

	// HealthCheck reports on the health status of the operator
	// Returns nil if healthy, or an error describing the issue
	HealthCheck(ctx context.Context) error
}
