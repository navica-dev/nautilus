package nautilus

import (
	"fmt"
	"time"

	"github.com/navica-dev/nautilus/internal/config"
	"github.com/navica-dev/nautilus/pkg/logging"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Option is a functional option for configuring Nautilus
type Option func(*Nautilus) error

// WithConfigPath loads configuration from the specified path
func WithConfigPath(path string) Option {
	return func(n *Nautilus) error {
		cfg, err := config.LoadConfig(path)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		n.config = cfg
		return nil
	}
}

// WithConfig directly sets the configuration
func WithConfig(cfg *config.Config) Option {
	return func(n *Nautilus) error {
		n.config = cfg
		return nil
	}
}

// WithName sets the robot name
func WithName(name string) Option {
	return func(n *Nautilus) error {
		if n.config.Robot == nil {
			n.config.Robot = &config.RobotConfig{}
		}
		n.config.Robot.Name = name
		return nil
	}
}

// WithDescription sets the robot description
func WithDescription(description string) Option {
	return func(n *Nautilus) error {
		if n.config.Robot == nil {
			n.config.Robot = &config.RobotConfig{}
		}
		n.config.Robot.Description = description
		return nil
	}
}

// WithVersion sets the version information
func WithVersion(version string) Option {
	return func(n *Nautilus) error {
		n.version = version
		return nil
	}
}

// WithRunOnce configures Nautilus to run the robot once and exit
func WithRunOnce(waitAfterCompletion bool) Option {
	return func(n *Nautilus) error {
		if n.config.Execution == nil {
			n.config.Execution = &config.ExecutionConfig{}
		}
		n.config.Execution.RunOnce = true
		n.config.Execution.WaitAfterCompletion = waitAfterCompletion
		return nil
	}
}

// WithSchedule sets a cron schedule for periodic execution
func WithSchedule(cronExpr string) Option {
	return func(n *Nautilus) error {
		if n.config.Execution == nil {
			n.config.Execution = &config.ExecutionConfig{}
		}
		n.config.Execution.Schedule = cronExpr
		return nil
	}
}

// WithInterval sets a time interval for periodic execution
func WithInterval(interval time.Duration) Option {
	return func(n *Nautilus) error {
		if n.config.Execution == nil {
			n.config.Execution = &config.ExecutionConfig{}
		}
		n.config.Execution.Interval = interval
		return nil
	}
}

// WithTimeout sets the maximum execution time for each run
func WithTimeout(timeout time.Duration) Option {
	return func(n *Nautilus) error {
		if n.config.Execution == nil {
			n.config.Execution = &config.ExecutionConfig{}
		}
		n.config.Execution.Timeout = timeout
		return nil
	}
}

// WithParallelism configures the parallel execution settings
func WithParallelism(workers, bufferSize int) Option {
	return func(n *Nautilus) error {
		if n.config.Execution == nil {
			n.config.Execution = &config.ExecutionConfig{}
		}
		if n.config.Execution.Parallel == nil {
			n.config.Execution.Parallel = &config.ParallelConfig{}
		}
		n.config.Execution.Parallel.Workers = workers
		n.config.Execution.Parallel.BufferSize = bufferSize
		return nil
	}
}

// WithAPI enables or disables the API server
func WithAPI(enabled bool, port int) Option {
	return func(n *Nautilus) error {
		if n.config.API == nil {
			n.config.API = &config.APIConfig{}
		}
		n.config.API.Enabled = enabled
		if port > 0 {
			n.config.API.Port = port
		}
		return nil
	}
}

// WithMetrics enables or disables metrics collection
func WithMetrics(enabled bool) Option {
	return func(n *Nautilus) error {
		if n.config.Metrics == nil {
			n.config.Metrics = &config.MetricsConfig{}
		}
		n.config.Metrics.Enabled = enabled
		return nil
	}
}

// WithLogLevel sets the logging level
func WithLogLevel(level string) Option {
	return func(n *Nautilus) error {
		lvl, err := zerolog.ParseLevel(level)
		if err != nil {
			return fmt.Errorf("invalid log level: %w", err)
		}

		if n.config.Logging == nil {
			n.config.Logging = &config.LoggingConfig{}
		}
		n.config.Logging.Level = level

		// Apply the log level
		zerolog.SetGlobalLevel(lvl)
		return nil
	}
}

// WithLogFormat sets the logging format
func WithLogFormat(format string) Option {
	return func(n *Nautilus) error {
		if n.config.Logging == nil {
			n.config.Logging = &config.LoggingConfig{}
		}
		n.config.Logging.Format = format

		// Apply the log format
		switch format {
		case "json":
			// JSON is the default for zerolog
		case "console":
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: logging.DefaultOutput})
		default:
			return fmt.Errorf("unsupported log format: %s", format)
		}

		return nil
	}
}

// WithLogger sets a custom logger
func WithLogger(logger zerolog.Logger) Option {
	return func(n *Nautilus) error {
		n.logger = logger
		return nil
	}
}

// WithMaxConsecutiveFailures sets the maximum allowed consecutive failures
func WithMaxConsecutiveFailures(max int) Option {
	return func(n *Nautilus) error {
		if n.config.Health == nil {
			n.config.Health = &config.HealthConfig{}
		}
		n.config.Health.MaxConsecutiveFailures = max
		return nil
	}
}

// WithHealthcheckDelay sets the delay between health checks
func WithHealthcheckDelay(delay time.Duration) Option {
	return func(n *Nautilus) error {
		if n.config.Health == nil {
			n.config.Health = &config.HealthConfig{}
		}
		n.config.Health.CheckInterval = delay
		return nil
	}
}

// WithPlugin adds a plugin to Nautilus
func WithPlugin(plugin Plugin) Option {
	return func(n *Nautilus) error {
		n.plugins = append(n.plugins, plugin)
		return nil
	}
}

// WithSentry configures Sentry error reporting
func WithSentry(enabled bool, dsn string) Option {
	return func(n *Nautilus) error {
		if n.config.Logging == nil {
			n.config.Logging = &config.LoggingConfig{}
		}
		if n.config.Logging.Sentry == nil {
			n.config.Logging.Sentry = &config.SentryConfig{}
		}

		n.config.Logging.Sentry.Enabled = enabled
		n.config.Logging.Sentry.DSN = dsn

		// Initialize Sentry if enabled
		if enabled && dsn != "" {
			if err := logging.SetupSentry(dsn, n.version, n.config.Robot.Name); err != nil {
				return fmt.Errorf("failed to initialize Sentry: %w", err)
			}
		}

		return nil
	}
}
