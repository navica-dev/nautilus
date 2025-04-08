package nautilus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the overall health status of the system
type HealthStatus struct {
	// Status is the overall status (ok, warning, error)
	Status string `json:"status"`

	// Components contains the health status of individual components
	Components map[string]ComponentStatus `json:"components"`

	// LastChecked is when the health was last checked
	LastChecked time.Time `json:"lastChecked"`

	// Version is the version of the system
	Version string `json:"version"`

	// Uptime is how long the system has been running
	Uptime time.Duration `json:"uptime"`
}

// ComponentStatus represents the health status of a single component
type ComponentStatus struct {
	// Status is the status of this component (ok, warning, error)
	Status string `json:"status"`

	// Message provides additional information about the status
	Message string `json:"message,omitempty"`

	// LastChecked is when the component was last checked
	LastChecked time.Time `json:"lastChecked"`
}

// HealthChecker maintains the health state of the system
type HealthChecker struct {
	// Health status
	status HealthStatus

	// Registered health check components
	components map[string]func(context.Context) error

	// Configuration
	checkInterval time.Duration
	timeout       time.Duration

	// State
	mu            sync.RWMutex
	lastFullCheck time.Time
	stopping      bool

	// Metadata
	version   string
	startTime time.Time
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(version string, checkInterval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		status: HealthStatus{
			Status:     "unknown",
			Components: make(map[string]ComponentStatus),
			Version:    version,
		},
		components:    make(map[string]func(context.Context) error),
		checkInterval: checkInterval,
		timeout:       timeout,
		startTime:     time.Now(),
		version:       version,
	}
}

// RegisterCheck adds a health check function for a component
func (h *HealthChecker) RegisterCheck(name string, check func(context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.components[name] = check
	h.status.Components[name] = ComponentStatus{
		Status: "unknown",
	}
}

// Start begins periodic health checking
func (h *HealthChecker) Start(ctx context.Context) {
	// Perform an initial health check
	h.CheckHealth(ctx)

	// Start periodic checks
	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.CheckHealth(ctx)
		}
	}
}

// Stop ends health checking
func (h *HealthChecker) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.stopping = true
}

// CheckHealth performs a health check of all components
func (h *HealthChecker) CheckHealth(ctx context.Context) HealthStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stopping {
		h.status.Status = "stopping"
		return h.status
	}

	h.lastFullCheck = time.Now()
	h.status.LastChecked = h.lastFullCheck
	h.status.Uptime = time.Since(h.startTime)

	// Default to ok, will be updated if any component fails
	h.status.Status = "ok"

	// Check each component
	for name, checkFn := range h.components {
		componentStatus := ComponentStatus{
			LastChecked: h.lastFullCheck,
		}

		// Create timeout context for this check
		checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
		err := checkFn(checkCtx)
		cancel()

		if err != nil {
			componentStatus.Status = "error"
			componentStatus.Message = err.Error()
			h.status.Status = "error" // Overall status is error if any component fails
		} else {
			componentStatus.Status = "ok"
		}

		h.status.Components[name] = componentStatus
	}

	return h.status
}

// GetHealth returns the current health status
func (h *HealthChecker) GetHealth() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Update uptime
	h.status.Uptime = time.Since(h.startTime)
	return h.status
}

// IsHealthy returns true if the system is healthy
func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.status.Status == "ok"
}

// RequireHealthy blocks until the system is healthy or the context is cancelled
func (h *HealthChecker) RequireHealthy(ctx context.Context, components ...string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			health := h.GetHealth()

			// Check specific components if requested
			if len(components) > 0 {
				allHealthy := true

				for _, component := range components {
					status, ok := health.Components[component]
					if !ok || status.Status != "ok" {
						allHealthy = false
						break
					}
				}

				if allHealthy {
					return nil
				}
			} else if health.Status == "ok" {
				// Check overall health if no specific components
				return nil
			}

			// Check timeout
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for healthy state")
			}
		}
	}
}
