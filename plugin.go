package nautilus

import "context"

// Plugin is the interface that all Nautilus plugins must implement
type Plugin interface {
	// Name returns the name of the plugin
	Name() string

	// Initialize is called during Nautilus initialization
	Initialize(ctx context.Context) error

	// Terminate is called during Nautilus shutdown
	Terminate(ctx context.Context) error
}

// [TODO]