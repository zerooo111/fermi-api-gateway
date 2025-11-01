package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/ingestion"
	"github.com/fermilabs/fermi-api-gateway/internal/parser"
	"github.com/fermilabs/fermi-api-gateway/internal/stream"
	"github.com/fermilabs/fermi-api-gateway/internal/writer"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Load configuration
	cfg, err := ingestion.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := initLogger(cfg.Environment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting Tick Ingestion Service",
		zap.String("service", cfg.ServiceName),
		zap.String("environment", cfg.Environment),
		zap.String("grpc_url", cfg.ContinuumGRPCURL),
		zap.String("output_mode", cfg.OutputMode),
		zap.Uint64("start_tick", cfg.StartTick),
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	reader := stream.NewGRPCReader(
		cfg.ContinuumGRPCURL,
		stream.WithStartTick(cfg.StartTick),
	)

	parserInstance := parser.NewProtobufParser()

	var writerInstance ingestion.Writer
	if cfg.OutputMode == "console" {
		// Console writer for debugging
		format := writer.FormatJSON
		switch cfg.OutputFormat {
		case "compact":
			format = writer.FormatCompact
		case "table":
			format = writer.FormatTable
		}
		writerInstance = writer.NewConsoleWriter(writer.WithFormat(format))
		logger.Info("Using console writer", zap.String("format", cfg.OutputFormat))
	} else {
		// TimescaleDB writer for production
		pool, err := connectDatabase(ctx, cfg, logger)
		if err != nil {
			logger.Fatal("Failed to connect to database", zap.Error(err))
		}
		defer pool.Close()

		writerInstance = writer.NewTimescaleWriter(pool, logger)
		logger.Info("Using TimescaleDB writer",
			zap.Int("max_connections", cfg.MaxConnections),
			zap.Int("batch_size", cfg.BatchSize),
		)
	}

	// Create pipeline
	pipelineConfig := ingestion.PipelineConfig{
		BufferSize:    cfg.BufferSize,
		WorkerCount:   cfg.WorkerCount,
		BatchSize:     cfg.BatchSize,
		FlushInterval: cfg.FlushInterval,
	}

	pipeline := ingestion.NewPipeline(reader, parserInstance, writerInstance, logger, pipelineConfig)
	defer pipeline.Close()

	// Start health check server
	healthServer := startHealthServer(cfg.HealthCheckPort, logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := healthServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("Health server shutdown error", zap.Error(err))
		}
	}()

	// Start pipeline in background
	pipelineDone := make(chan error, 1)
	go func() {
		pipelineDone <- pipeline.Run(ctx)
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		cancel() // Trigger graceful shutdown
	case err := <-pipelineDone:
		if err != nil {
			logger.Error("Pipeline error", zap.Error(err))
		}
	}

	// Wait for pipeline to finish (with timeout)
	shutdownTimer := time.NewTimer(30 * time.Second)
	defer shutdownTimer.Stop()

	select {
	case err := <-pipelineDone:
		if err != nil {
			logger.Error("Pipeline shutdown with error", zap.Error(err))
			os.Exit(1)
		}
		logger.Info("Pipeline shut down successfully")
	case <-shutdownTimer.C:
		logger.Warn("Pipeline shutdown timed out after 30 seconds")
		os.Exit(1)
	}

	logger.Info("Tick Ingestion Service stopped")
}

// initLogger creates a zap logger based on environment.
func initLogger(environment string) (*zap.Logger, error) {
	var config zap.Config

	if environment == "production" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	return config.Build()
}

// connectDatabase creates a pgx connection pool.
func connectDatabase(ctx context.Context, cfg *ingestion.Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = int32(cfg.MaxConnections)
	poolConfig.MinConns = int32(cfg.MinConnections)
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	// Create pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Connected to TimescaleDB",
		zap.Int32("max_connections", poolConfig.MaxConns),
		zap.Int32("min_connections", poolConfig.MinConns),
	)

	return pool, nil
}

// startHealthServer starts an HTTP server for health checks.
func startHealthServer(port int, logger *zap.Logger) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Readiness endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.Info("Health check server started", zap.Int("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Health server error", zap.Error(err))
		}
	}()

	return server
}
