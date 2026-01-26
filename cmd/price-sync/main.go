package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/pricesync"
)

func main() {
	// Initialize logger
	logger, err := initLogger(getEnv("ENV", "development"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting Price Sync Service")

	// Load configuration
	priceSyncCfg, err := pricesync.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load price sync configuration", zap.Error(err))
	}

	logger.Info("Configuration loaded",
		zap.String("market_endpoint", priceSyncCfg.MarketEndpoint),
		zap.Duration("poll_interval", priceSyncCfg.PollInterval),
		zap.Duration("heartbeat_interval", priceSyncCfg.HeartbeatInterval),
		zap.Duration("http_timeout", priceSyncCfg.HTTPTimeout),
	)

	// Load database configuration from gateway config
	gatewayCfg := config.Load()

	// Connect to database
	db, err := connectDatabase(gatewayCfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Error closing database connection", zap.Error(err))
		} else {
			logger.Info("Database connection closed")
		}
	}()

	logger.Info("Connected to TimescaleDB",
		zap.String("host", gatewayCfg.Database.Host),
		zap.String("port", gatewayCfg.Database.Port),
		zap.String("database", gatewayCfg.Database.DBName),
	)

	// Initialize components
	fetcher := pricesync.NewFetcher(priceSyncCfg.MarketEndpoint, priceSyncCfg.HTTPTimeout)

	writer, err := pricesync.NewWriter(db)
	if err != nil {
		logger.Fatal("Failed to initialize writer", zap.Error(err))
	}
	defer func() {
		if err := writer.Close(); err != nil {
			logger.Error("Error closing writer", zap.Error(err))
		}
	}()

	service := pricesync.NewService(fetcher, writer, logger, priceSyncCfg)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start service in background with panic recovery
	serviceDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("PANIC in price sync service",
					zap.Any("panic", r),
				)
				logger.Info("Attempting to restart service after panic...")
				time.Sleep(5 * time.Second)
				// Restart the service
				go func() {
					serviceDone <- service.Start(ctx)
				}()
			}
		}()
		serviceDone <- service.Start(ctx)
	}()

	logger.Info("Price sync service started successfully. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		cancel() // Trigger graceful shutdown
	case err := <-serviceDone:
		if err != nil && err != context.Canceled {
			logger.Error("Service error", zap.Error(err))
		}
	}

	// Wait for service to finish with timeout
	shutdownTimer := time.NewTimer(30 * time.Second)
	defer shutdownTimer.Stop()

	select {
	case err := <-serviceDone:
		if err != nil && err != context.Canceled {
			logger.Error("Service shutdown with error", zap.Error(err))
			os.Exit(1)
		}
		logger.Info("Service shut down successfully")
	case <-shutdownTimer.C:
		logger.Warn("Service shutdown timed out after 30 seconds")
		os.Exit(1)
	}

	logger.Info("Price Sync Service stopped")
}

// initLogger creates a zap logger based on environment
func initLogger(environment string) (*zap.Logger, error) {
	var config zap.Config

	if environment == "production" || environment == "staging" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	return config.Build()
}

// connectDatabase creates a database connection pool
func connectDatabase(cfg config.DatabaseConfig, logger *zap.Logger) (*sql.DB, error) {
	// Build connection string
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	// Open database connection
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection pool configured",
		zap.Int("max_open_conns", 25),
		zap.Int("max_idle_conns", 5),
	)

	return db, nil
}

// getEnv reads environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
