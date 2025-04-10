package plugin

import (
	"context"
)

// Plugin is the interface that all Nautilus plugins must implement
type Plugin interface {
	// Name returns the name of the plugin
	Name() string

	// Initialize is called during Nautilus initialization
	Initialize(ctx context.Context) error

	// Terminate is called during Nautilus shutdown
	Terminate(ctx context.Context) error
}

// DatabasePlugin is an interface for database plugins
type DatabasePlugin interface {
	Plugin

	// Connect establishes a connection to the database
	Connect(ctx context.Context) error

	// Disconnect closes the connection to the database
	Disconnect(ctx context.Context) error

	// Ping verifies the database connection is alive
	Ping(ctx context.Context) error
}

// MessagingPlugin is an interface for messaging plugins
type MessagingPlugin interface {
	Plugin

	// Connect establishes a connection to the messaging service
	Connect(ctx context.Context) error

	// Disconnect closes the connection to the messaging service
	Disconnect(ctx context.Context) error

	// Publish sends a message to the messaging service
	Publish(ctx context.Context, topic string, message []byte) error

	// Subscribe listens for messages from the messaging service
	Subscribe(ctx context.Context, topic string, handler func(message []byte) error) error
}

// StoragePlugin is an interface for storage plugins
type StoragePlugin interface {
	Plugin

	// Put writes data to storage
	Put(ctx context.Context, key string, value []byte) error

	// Get reads data from storage
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes data from storage
	Delete(ctx context.Context, key string) error

	// List returns a list of keys in storage
	List(ctx context.Context, prefix string) ([]string, error)
}

// SecretsPlugin is an interface for secrets plugins
type SecretsPlugin interface {
	Plugin

	// GetSecret retrieves a secret by key
	GetSecret(ctx context.Context, key string) (string, error)

	// SetSecret stores a secret
	SetSecret(ctx context.Context, key, value string) error
}

// PluginRegistry provides access to registered plugins
type PluginRegistry struct {
	plugins map[string]Plugin
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry
func (r *PluginRegistry) Register(plugin Plugin) {
	r.plugins[plugin.Name()] = plugin
}

// Get retrieves a plugin by name
func (r *PluginRegistry) Get(name string) (Plugin, bool) {
	plugin, ok := r.plugins[name]
	return plugin, ok
}

// GetByType retrieves all plugins of a specific type
func (r *PluginRegistry) GetByType(pluginType any) []Plugin {
	var result []Plugin

	for _, plugin := range r.plugins {
		switch pluginType.(type) {
		case DatabasePlugin:
			if _, ok := plugin.(DatabasePlugin); ok {
				result = append(result, plugin)
			}
		case MessagingPlugin:
			if _, ok := plugin.(MessagingPlugin); ok {
				result = append(result, plugin)
			}
		case StoragePlugin:
			if _, ok := plugin.(StoragePlugin); ok {
				result = append(result, plugin)
			}
		case SecretsPlugin:
			if _, ok := plugin.(SecretsPlugin); ok {
				result = append(result, plugin)
			}
		}
	}

	return result
}

func (r *PluginRegistry) GetAll() []Plugin {
	plugins := make([]Plugin, 0, len(r.plugins))

	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// InitializeAll initializes all registered plugins
func (r *PluginRegistry) InitializeAll(ctx context.Context) error {
	for _, plugin := range r.plugins {
		if err := plugin.Initialize(ctx); err != nil {
			return err
		}
	}

	return nil
}

// TerminateAll terminates all registered plugins
func (r *PluginRegistry) TerminateAll(ctx context.Context) {
	for _, plugin := range r.plugins {
		_ = plugin.Terminate(ctx)
	}
}
