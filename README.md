# Nautilus

Nautilus is a Go framework for building and running reliable, observable operator services. It provides a structured way to create long-running services with built-in support for health checks, metrics, API endpoints, and database connections.

[![Go Report Card](https://goreportcard.com/badge/github.com/navica-dev/nautilus)](https://goreportcard.com/report/github.com/navica-dev/nautilus)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![GoDoc](https://godoc.org/github.com/navica-dev/nautilus?status.svg)](https://godoc.org/github.com/navica-dev/nautilus)

## Features

- **Flexible Execution Models**: Run operators once, on a schedule, or at regular intervals
- **Built-in Observability**:
  - Health checks API endpoints
  - Prometheus metrics integration
  - Structured logging with zerolog
  - Grafana dashboard templates
- **Plugin System**: Extend functionality with database connectors and more
- **Database Integration**: Ready-to-use PostgreSQL plugin
- **Developer Experience**:
  - Simple, expressive API
  - Functional options pattern for configuration
  - Comprehensive examples
- **Runtime Management**:
  - Graceful shutdown
  - Resource cleanup
  - Error handling
  - Signal handling

## Installation

```bash
go get github.com/navica-dev/nautilus
```

## Quick Start

Here's a simple example of how to create a basic operator:

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/navica-dev/nautilus"
	"github.com/navica-dev/nautilus/pkg/interfaces"
)

// SimpleOperator implements the interfaces.Operator interface
var _ interfaces.Operator = (*SimpleOperator)(nil)

type SimpleOperator struct {
	counter int
}

func (op *SimpleOperator) Initialize(ctx context.Context) error {
	op.counter = 0
	return nil
}

func (op *SimpleOperator) Run(ctx context.Context) error {
	op.counter++
	log.Printf("Running operator for the %dth time", op.counter)
	return nil
}

func (op *SimpleOperator) Terminate(ctx context.Context) error {
	log.Printf("Terminating operator after %d runs", op.counter)
	return nil
}

func main() {
	ctx := context.Background()

	// Create Nautilus instance with options
	n, err := nautilus.New(
		nautilus.WithName("simple-operator"),
		nautilus.WithDescription("A simple example operator"),
		nautilus.WithVersion("0.1.0"),
		nautilus.WithInterval(5*time.Second),
		nautilus.WithAPI(true, 12911),
		nautilus.WithMetrics(true),
	)
	if err != nil {
		log.Fatalf("Failed to create Nautilus instance: %v", err)
	}

	// Create and run the operator
	op := &SimpleOperator{}
	if err := n.Run(ctx, op); err != nil {
		log.Fatalf("Error running operator: %v", err)
	}
}
```

## Configuration

Nautilus can be configured through functional options:

```go
n, err := nautilus.New(
	// Basic configuration
	nautilus.WithName("my-operator"),
	nautilus.WithDescription("My custom operator"),
	nautilus.WithVersion("1.0.0"),
	
	// Execution configuration
	nautilus.WithInterval(30*time.Second),
	// OR
	nautilus.WithSchedule("*/5 * * * *"), // Cron expression
	// OR
	nautilus.WithRunOnce(false),
	
	// API configuration
	nautilus.WithAPI(true, 12911),
	
	// Metrics configuration
	nautilus.WithMetrics(true),
	
	// Logging configuration
	nautilus.WithLogLevel(zerolog.LevelInfoValue),
	nautilus.WithLogFormat(enums.LogFormatConsole),
	
	// Advanced configuration
	nautilus.WithMaxConsecutiveFailures(3),
	nautilus.WithTimeout(5*time.Minute),
)
```

You can also use a configuration file:

```go
nautilus.WithConfigPath("./config.yaml")
```

## Examples

The repository includes several examples:

### Basic Example

A simple operator that runs periodically and simulates work.

```bash
go run examples/basic/main.go
```

### PostgreSQL Example

An operator that interacts with a PostgreSQL database.

```bash
go run examples/postgres/main.go
```

## Advanced Usage

### Creating Custom Plugins

Nautilus supports a plugin system to extend functionality. Here's how to create a custom plugin:

```go
type MyPlugin struct {
	// Your plugin fields
}

var _ plugin.Plugin = (*MyPlugin)(nil)

func (p *MyPlugin) Name() string {
	return "my-plugin"
}

func (p *MyPlugin) Initialize(ctx context.Context) error {
	// Initialize your plugin
	return nil
}

func (p *MyPlugin) Terminate(ctx context.Context) error {
	// Clean up resources
	return nil
}

// Then register it with Nautilus
n, err := nautilus.New(
	// ... other options
	nautilus.WithPlugin(&MyPlugin{}),
)
```

### Health Checks

Implement the `interfaces.HealthCheck` interface to provide custom health information:

```go
var _ interfaces.HealthCheck = (*MyOperator)(nil)

func (op *MyOperator) Name() string {
	return "my-operator"
}

func (op *MyOperator) HealthCheck(ctx context.Context) error {
	// If healthy, return nil
	if op.isHealthy() {
		return nil
	}
	// If unhealthy, return an error
	return errors.New("operator is not healthy")
}
```

### Parallel Execution

Nautilus provides support for parallel execution:

```go
func (op *MyOperator) Run(ctx context.Context) error {
	// Execute multiple tasks in parallel
	return n.ExecuteParallel(ctx, func(ctx context.Context) error {
		// Task logic here
		return nil
	})
}
```

## Development Setup

### Prerequisites

- Go 1.24+
- Docker and Docker Compose (for running PostgreSQL examples)

### Setting Up the Development Environment

1. Clone the repository:

   ```bash
   git clone https://github.com/navica-dev/nautilus.git
   cd nautilus
   ```

2. Start the development environment:

   ```bash
   cd .devenv
   docker-compose up -d
   ```

   This starts:
   - PostgreSQL on port 5432
   - PgAdmin on port 5050
   - Prometheus on port 9090
   - Grafana on port 3000

3. Run examples:

   ```bash
   go run examples/basic/main.go
   # or
   go run examples/postgres/main.go
   ```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
