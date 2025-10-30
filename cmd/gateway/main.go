package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/health"
	"github.com/fermilabs/fermi-api-gateway/internal/metrics"
	"github.com/fermilabs/fermi-api-gateway/internal/middleware"
	"github.com/fermilabs/fermi-api-gateway/internal/ratelimit"
)

func main() {
	// Load configuration from environment
	cfg := config.Load()

	// Initialize logger
	var logger *zap.Logger
	var err error
	if cfg.Server.Env == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Initialize metrics
	m := metrics.NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Create router
	r := chi.NewRouter()

	// Apply global middleware (order matters!)
	r.Use(middleware.RequestID)           // Generate request IDs first
	r.Use(middleware.Recovery)            // Recover from panics
	r.Use(middleware.Logging(logger))     // Log all requests
	r.Use(middleware.Metrics(m))          // Record metrics
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins)) // Handle CORS

	// Metrics endpoint (no auth for now)
	r.Get("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP)

	// Health check endpoints (no rate limiting)
	r.Get("/health", health.Handler())
	r.Get("/ready", health.ReadyHandler())

	// API routes with rate limiting
	r.Route("/api", func(r chi.Router) {
		// Rollup API - 1000 req/min = ~16.67 req/sec
		rollupLimiter := ratelimit.NewIPRateLimiter(float64(cfg.RateLimit.RollupRPM)/60, cfg.RateLimit.RollupRPM)
		r.Route("/rollup", func(r chi.Router) {
			r.Use(ratelimit.Middleware(rollupLimiter))
			r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
				w.Write([]byte(`{"message":"Rollup proxy not yet implemented"}`))
			})
		})

		// Continuum gRPC - 500 req/min = ~8.33 req/sec
		grpcLimiter := ratelimit.NewIPRateLimiter(float64(cfg.RateLimit.ContinuumGrpcRPM)/60, cfg.RateLimit.ContinuumGrpcRPM)
		r.Route("/continuum/grpc", func(r chi.Router) {
			r.Use(ratelimit.Middleware(grpcLimiter))
			r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
				w.Write([]byte(`{"message":"Continuum gRPC proxy not yet implemented"}`))
			})
		})

		// Continuum REST - 2000 req/min = ~33.33 req/sec
		restLimiter := ratelimit.NewIPRateLimiter(float64(cfg.RateLimit.ContinuumRestRPM)/60, cfg.RateLimit.ContinuumRestRPM)
		r.Route("/continuum/rest", func(r chi.Router) {
			r.Use(ratelimit.Middleware(restLimiter))
			r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
				w.Write([]byte(`{"message":"Continuum REST proxy not yet implemented"}`))
			})
		})
	})

	// Basic info endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"service":"fermi-api-gateway","version":"1.0.0","env":"%s"}`, cfg.Server.Env)
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for errors coming from the listener.
	serverErrors := make(chan error, 1)

	// Start the server
	go func() {
		log.Printf("Starting API Gateway on port %s (env: %s)", cfg.Server.Port, cfg.Server.Env)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for interrupt or terminate signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		log.Fatalf("Error starting server: %v", err)

	case sig := <-shutdown:
		log.Printf("Received signal %v, starting graceful shutdown", sig)

		// Give outstanding requests a deadline for completion
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := srv.Shutdown(ctx); err != nil {
			srv.Close()
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}

		log.Println("Server stopped gracefully")
	}
}
