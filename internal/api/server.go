package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/navica-dev/nautilus/internal/config"
	"github.com/navica-dev/nautilus/pkg/interfaces"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Server represents the HTTP API server
type Server struct {
	router chi.Router
	server *http.Server
	config *config.APIConfig
	logger zerolog.Logger

	// Health check handlers
	healthCheckers []interfaces.HealthCheck

	// Version info
	version string

	// Start time
	startTime time.Time
}

// // HealthChecker is the interface for components that provide health status
// type HealthChecker interface {
// 	// Name returns a unique identifier for this health check
// 	Name() string

// 	// Check performs the health check
// 	Check(ctx context.Context) error
// }

// HealthResponse represents the health check response format
type HealthResponse struct {
	Status     string                     `json:"status"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
	Timestamp  time.Time                  `json:"timestamp"`
	Version    string                     `json:"version"`
	Uptime     string                     `json:"uptime"`
}

// ComponentHealth represents a single component's health
type ComponentHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// NewServer creates a new API server
func NewServer(cfg *config.APIConfig, version string) *Server {
	r := chi.NewRouter()

	// Base middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Setup logger
	logger := log.With().Str("component", "api-server").Logger()

	s := &Server{
		router:         r,
		config:         cfg,
		logger:         logger,
		healthCheckers: make([]interfaces.HealthCheck, 0),
		version:        version,
		startTime:      time.Now(),
	}

	// Register routes
	s.registerRoutes()

	// Create server
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	return s
}

// RegisterHealthChecker adds a health checker to the server
func (s *Server) RegisterHealthChecker(checker interfaces.HealthCheck) {
	s.healthCheckers = append(s.healthCheckers, checker)
}

// Start starts the server
func (s *Server) Start() error {
	s.logger.Info().Int("port", s.config.Port).Msg("Starting API server")

	if s.config.TLS != nil && s.config.TLS.Enabled {
		return s.server.ListenAndServeTLS(s.config.TLS.CertFile, s.config.TLS.KeyFile)
	}

	return s.server.ListenAndServe()
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Stopping API server")
	return s.server.Shutdown(ctx)
}

// registerRoutes sets up the HTTP routes
func (s *Server) registerRoutes() {
	// Health check endpoint
	s.router.Get("/health", s.handleHealth())

	// Readiness check endpoint
	s.router.Get("/ready", s.handleReady())

	// Liveness check endpoint
	s.router.Get("/live", s.handleLive())

	// Metrics endpoint
	s.router.Handle("/metrics", promhttp.Handler())

	// Version endpoint
	s.router.Get("/version", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": s.version})
	})

	// Debug endpoints
	s.router.Route("/debug", func(r chi.Router) {
		r.Get("/config", s.handleConfig())
	})
}

// handleHealth creates the health check handler
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		response := HealthResponse{
			Status:     "ok",
			Components: make(map[string]ComponentHealth),
			Timestamp:  time.Now(),
			Version:    s.version,
			Uptime:     time.Since(s.startTime).String(),
		}

		// Check all health checkers
		for _, checker := range s.healthCheckers {
			err := checker.HealthCheck(ctx)

			status := "ok"
			message := ""

			if err != nil {
				status = "error"
				message = err.Error()
				response.Status = "error" // Overall status is error if any component fails
			}

			response.Components[checker.Name()] = ComponentHealth{
				Status:  status,
				Message: message,
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if response.Status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// handleReady creates the readiness check handler
func (s *Server) handleReady() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple ready check - if we're serving requests, we're ready
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	}
}

// handleLive creates the liveness check handler
func (s *Server) handleLive() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple liveness check - if we're serving requests, we're alive
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	}
}

// handleConfig creates the config debug handler
func (s *Server) handleConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only show in debug environments
		if s.config.DebugMode {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s.config)
		} else {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Debug endpoints disabled in production"))
		}
	}
}
