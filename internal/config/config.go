package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete configuration for Nautilus
type Config struct {
	Operator  *OperatorConfig  `mapstructure:"operator"`
	Execution *ExecutionConfig `mapstructure:"execution"`
	API       *APIConfig       `mapstructure:"api"`
	Logging   *LoggingConfig   `mapstructure:"logging"`
	Metrics   *MetricsConfig   `mapstructure:"metrics"`
	Secrets   *SecretsConfig   `mapstructure:"secrets"`
	Health    *HealthConfig    `mapstructure:"health"`
}

// OperatorConfig contains operator-specific configuration
type OperatorConfig struct {
	Name        string                 `mapstructure:"name"`
	Description string                 `mapstructure:"description"`
	Parameters  map[string]interface{} `mapstructure:"parameters"`
}

// ExecutionConfig controls how the operator is executed
type ExecutionConfig struct {
	// Schedule using cron expression (takes precedence over Interval if set)
	Schedule string `mapstructure:"schedule"`

	// Simple interval for periodic execution
	Interval time.Duration `mapstructure:"interval"`

	// RunOnce executes the operator once and then stops
	RunOnce bool `mapstructure:"runOnce"`

	// WaitAfterCompletion keeps the process running after completion
	WaitAfterCompletion bool `mapstructure:"waitAfterCompletion"`

	// Parallel execution settings
	Parallel *ParallelConfig `mapstructure:"parallel"`

	// Timeout for each execution of the Run method
	Timeout time.Duration `mapstructure:"timeout"`
}

// ParallelConfig controls parallel execution settings
type ParallelConfig struct {
	Workers    int `mapstructure:"workers"`
	BufferSize int `mapstructure:"bufferSize"`
}

// APIConfig controls the HTTP API server
type APIConfig struct {
	Enabled   bool       `mapstructure:"enabled"`
	Port      int        `mapstructure:"port"`
	Path      string     `mapstructure:"path"`
	DebugMode bool       `mapstructure:"debugMode"`
	TLS       *TLSConfig `mapstructure:"tls"`
}

// TLSConfig contains TLS configuration for the API server
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
}

// LoggingConfig controls logging behavior
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json, console, gelf

	Sentry *SentryConfig `mapstructure:"sentry"`
	GELF   *GELFConfig   `mapstructure:"gelf"`
}

// SentryConfig controls Sentry error reporting
type SentryConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	DSN     string `mapstructure:"dsn"`
}

// GELFConfig controls GELF logging
type GELFConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
}

// MetricsConfig controls metrics collection
type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`

	Prometheus *PrometheusConfig `mapstructure:"prometheus"`
}

// PrometheusConfig controls Prometheus metrics
type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// SecretsConfig controls how secrets are managed
type SecretsConfig struct {
	Provider string `mapstructure:"provider"` // vault, aws, env, etc.

	Vault *VaultConfig `mapstructure:"vault"`
}

// VaultConfig controls HashiCorp Vault integration
type VaultConfig struct {
	Address   string `mapstructure:"address"`
	TokenPath string `mapstructure:"tokenPath"`
}

// HealthConfig controls health checking behavior
type HealthConfig struct {
	// CheckInterval is how frequently to run health checks
	CheckInterval time.Duration `mapstructure:"checkInterval"`

	// MaxConsecutiveFailures is the maximum number of consecutive failures allowed
	MaxConsecutiveFailures int `mapstructure:"maxConsecutiveFailures"`
}

// LoadConfig loads configuration from file, environment, and flags
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Use config file if specified
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Search for config in standard locations
		v.SetConfigName("nautilus")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/nautilus")
	}

	// Read environment variables prefixed with NAUTILUS_
	v.SetEnvPrefix("NAUTILUS")
	v.AutomaticEnv()

	// Attempt to read config file (non-fatal if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Parse config into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Ensure required structures exist
	if config.Operator == nil {
		config.Operator = &OperatorConfig{
			Name: "nautilus-operator",
		}
	}

	if config.Execution == nil {
		config.Execution = &ExecutionConfig{
			RunOnce: true,
		}
	}

	if config.Execution.Parallel == nil {
		config.Execution.Parallel = &ParallelConfig{
			Workers:    10,
			BufferSize: 100,
		}
	}

	if config.API == nil {
		config.API = &APIConfig{
			Enabled:   true,
			Port:      12911,
			Path:      "/health",
			DebugMode: false,
		}
	}

	if config.Logging == nil {
		config.Logging = &LoggingConfig{
			Level:  "info",
			Format: "console",
		}
	}

	if config.Metrics == nil {
		config.Metrics = &MetricsConfig{
			Enabled: true,
		}
	}

	if config.Health == nil {
		config.Health = &HealthConfig{
			CheckInterval:          30 * time.Second,
			MaxConsecutiveFailures: 3,
		}
	}

	return &config, nil
}

// setDefaults sets sensible default values for configuration
func setDefaults(v *viper.Viper) {
	// Operator defaults
	v.SetDefault("operator.name", "nautilus-operator")

	// Execution defaults
	v.SetDefault("execution.runOnce", true)
	v.SetDefault("execution.waitAfterCompletion", false)
	v.SetDefault("execution.parallel.workers", 10)
	v.SetDefault("execution.parallel.bufferSize", 100)
	v.SetDefault("execution.timeout", "10m")

	// API defaults
	v.SetDefault("api.enabled", true)
	v.SetDefault("api.port", 12911)
	v.SetDefault("api.path", "/health")
	v.SetDefault("api.tls.enabled", false)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")
	v.SetDefault("logging.sentry.enabled", false)
	v.SetDefault("logging.gelf.enabled", false)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.prometheus.enabled", true)
	v.SetDefault("metrics.prometheus.path", "/metrics")

	// Secrets defaults
	v.SetDefault("secrets.provider", "env")

	// Health defaults
	v.SetDefault("health.checkInterval", "30s")
	v.SetDefault("health.maxConsecutiveFailures", 3)
}
