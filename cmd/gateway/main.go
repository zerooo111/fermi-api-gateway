package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/database"
	"github.com/fermilabs/fermi-api-gateway/internal/health"
	"github.com/fermilabs/fermi-api-gateway/internal/metrics"
	"github.com/fermilabs/fermi-api-gateway/internal/middleware"
	"github.com/fermilabs/fermi-api-gateway/internal/proxy"
	"github.com/fermilabs/fermi-api-gateway/internal/ratelimit"
)

func main() {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

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
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Initialize metrics
	m := metrics.NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Initialize database connection (optional - gracefully handle if not configured)
	var repo *database.Repository
	if cfg.Database.Host != "" && cfg.Database.DBName != "" {
		db, err := database.NewDB(cfg.Database)
		if err != nil {
			logger.Warn("Database connection failed - transaction endpoints will have limited functionality", zap.Error(err))
		} else {
			defer db.Close()
			repo = database.NewRepository(db)
			logger.Info("Database connected successfully")
		}
	} else {
		logger.Info("Database not configured - transaction endpoints will have limited functionality")
	}

	// Initialize proxies
	rollupProxy := proxy.NewHTTPProxy(cfg.Backend.RollupURL, 15*time.Second)
	continuumRestProxy := proxy.NewHTTPProxy(cfg.Backend.ContinuumRestURL, 15*time.Second)

	continuumGrpcProxy, err := proxy.NewGRPCProxy(cfg.Backend.ContinuumGrpcURL, repo, cfg.Backend.ContinuumRestURL, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Continuum gRPC proxy", zap.Error(err))
	}
	defer continuumGrpcProxy.Close()

	// Create router
	r := chi.NewRouter()

	// Apply global middleware (order matters!)
	r.Use(middleware.RequestID)                     // Generate request IDs first
	r.Use(middleware.Recovery(logger))              // Recover from panics
	r.Use(middleware.Logging(logger))               // Log all requests
	r.Use(middleware.Metrics(m))                    // Record metrics
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins)) // Handle CORS

	// Metrics endpoint (no auth for now)
	r.Get("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP)

	// Health check endpoints (no rate limiting)
	r.Get("/health", health.Handler())
	r.Get("/ready", health.ReadyHandler())

	// System status endpoint - checks health of all backend services
	statusDeps := &health.StatusDependencies{
		RollupHealthURL:    "http://44.194.22.128:8080/status",
		ContinuumHealthURL: cfg.Backend.ContinuumRestURL + "/health",
		DB:                 repo,
	}
	r.Get("/status", health.StatusHandler(statusDeps))
	r.Get("/statuspage", health.StatusPageHandler(statusDeps))

	// API v1 routes - clean, versioned endpoints
	r.Route("/api/v1", func(r chi.Router) {
		// Rollup API - 1000 req/min = ~16.67 req/sec
		rollupLimiter := ratelimit.NewIPRateLimiter(float64(cfg.RateLimit.RollupRPM)/60, cfg.RateLimit.RollupRPM)
		r.Route("/rollup", func(r chi.Router) {
			r.Use(ratelimit.Middleware(rollupLimiter))

			// Candles endpoint - queries database directly
			candlesHandler := proxy.NewCandlesHandler(repo, logger)
			r.Get("/markets/{marketId}/candles", candlesHandler.GetMarketCandles())

			// Catch-all proxy handler for other rollup routes
			r.Handle("/*", rollupProxy.Handler())
		})

		// Continuum Explorer API - proxies to the new read-only Explorer API
		// Use higher rate limit (2000 req/min) since this is a read-heavy API
		continuumLimiter := ratelimit.NewIPRateLimiter(float64(cfg.RateLimit.ContinuumRestRPM)/60, cfg.RateLimit.ContinuumRestRPM)
		r.Route("/continuum", func(r chi.Router) {
			r.Use(ratelimit.Middleware(continuumLimiter))

			// === Continuum Explorer API Endpoints ===
			// See continuum-api-guide.md for full API documentation

			// Health check - GET /health
			r.Get("/health", continuumRestProxy.HandlerWithPath("/health"))

			// Service info - GET /
			r.Get("/info", continuumRestProxy.HandlerWithPath("/"))

			// Statistics - GET /api/v1/stats
			r.Get("/stats", continuumRestProxy.HandlerWithPath("/api/v1/stats"))

			// Ticks endpoints
			// GET /tick/recent?limit=N - returns recent ticks with transactions
			r.Get("/tick/recent", continuumRestProxy.HandlerWithPath("/api/v1/ticks/recent"))
			// GET /tick/{tickNumber} - returns tick details with VDF proof and transactions
			r.Get("/tick/{tickNumber}", continuumRestProxy.HandlerWithPathTemplate("/api/v1/ticks/{tickNumber}", "tickNumber"))

			// Transactions endpoints
			// GET /txn/recent?limit=N - returns recent transactions
			r.Get("/txn/recent", continuumRestProxy.HandlerWithPath("/api/v1/transactions/recent"))
			// GET /txn/{txnId} - returns transaction by hash
			r.Get("/txn/{txnId}", continuumRestProxy.HandlerWithPathTemplate("/api/v1/transactions/{txnId}", "txnId"))

			// === gRPC Write Endpoints (for submitting transactions) ===
			// Transaction submission endpoints
			r.Get("/tx/recent", continuumGrpcProxy.HandleGetRecentTransactions())
			r.Handle("/tx/*", continuumGrpcProxy.HandleGetTransactionByHash())
			r.Post("/tx", continuumGrpcProxy.HandleSubmitTransaction())
			r.Post("/tx/batch", continuumGrpcProxy.HandleSubmitBatch())

			// Legacy gRPC endpoints (backward compatibility)
			r.Post("/submit-transaction", continuumGrpcProxy.HandleSubmitTransaction())
			r.Post("/submit-batch", continuumGrpcProxy.HandleSubmitBatch())
			r.Get("/stream-ticks", continuumGrpcProxy.HandleStreamTicks())

			// Status endpoint - proxy to Explorer API health
			r.Get("/status", continuumRestProxy.HandlerWithPath("/health"))

			// Other gRPC endpoints
			r.Get("/transaction", continuumGrpcProxy.HandleGetTransaction())
			r.Get("/tick", continuumGrpcProxy.HandleGetTick())
			r.Get("/chain-state", continuumGrpcProxy.HandleGetChainState())

			// Catch-all proxy to Explorer API for any unmatched routes
			r.Handle("/*", continuumRestProxy.Handler())
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
		logger.Info("Starting API Gateway",
			zap.String("port", cfg.Server.Port),
			zap.String("env", cfg.Server.Env),
		)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for interrupt or terminate signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		logger.Fatal("Error starting server", zap.Error(err))

	case sig := <-shutdown:
		logger.Info("Received shutdown signal, starting graceful shutdown",
			zap.String("signal", sig.String()),
		)

		// Give outstanding requests a deadline for completion
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := srv.Shutdown(ctx); err != nil {
			srv.Close()
			logger.Fatal("Could not gracefully shutdown the server", zap.Error(err))
		}

		logger.Info("Server stopped gracefully")
	}
}
