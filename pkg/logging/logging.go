package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// DefaultOutput is the default output for logs
var DefaultOutput io.Writer = os.Stdout

// Setup initializes the logging system
func Setup() {
	// Set default time format to ISO8601
	zerolog.TimeFieldFormat = time.RFC3339

	// Get log level from environment
	level, err := zerolog.ParseLevel(os.Getenv("NAUTILUS_LOGGING_LEVEL"))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Add global fields
	hostname, _ := os.Hostname()
	log.Logger = log.With().
		Str("service", os.Getenv("NAUTILUS_OPERATOR_NAME")).
		Str("host", hostname).
		Logger()

	// Add a caller skip to get proper file:line in logs
	zerolog.CallerSkipFrameCount = 2
}

// GetLogger returns a logger with the given component
func GetLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// WithField adds a field to the logger
func WithField(key string, value interface{}) *zerolog.Logger {
	logger := log.With().Interface(key, value).Logger()
	return &logger
}

// WithFields adds multiple fields to the logger
func WithFields(fields map[string]interface{}) *zerolog.Logger {
	ctx := log.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	logger := ctx.Logger()
	return &logger
}

// WithError adds an error to the logger
func WithError(err error) *zerolog.Logger {
	logger := log.With().Err(err).Logger()
	return &logger
}
