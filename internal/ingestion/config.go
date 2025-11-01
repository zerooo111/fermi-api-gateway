package ingestion

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the tick ingestion service.
type Config struct {
	// Service
	ServiceName string
	Environment string

	// gRPC Stream
	ContinuumGRPCURL string
	StartTick        uint64

	// Database
	DatabaseURL      string
	MaxConnections   int
	MinConnections   int
	MaxConnLifetime  time.Duration
	MaxConnIdleTime  time.Duration

	// Pipeline
	BufferSize    int
	WorkerCount   int
	BatchSize     int
	FlushInterval time.Duration

	// Output Mode
	OutputMode   string // "console" or "timescale"
	OutputFormat string // "json", "compact", or "table" (for console mode)

	// Health Check
	HealthCheckPort int
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Defaults
		ServiceName:      getEnv("SERVICE_NAME", "tick-ingester"),
		Environment:      getEnv("ENV", "development"),
		ContinuumGRPCURL: getEnv("CONTINUUM_GRPC_URL", "localhost:50051"),
		StartTick:        getEnvUint64("START_TICK", 0),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		MaxConnections:   getEnvInt("DB_MAX_CONNECTIONS", 100),
		MinConnections:   getEnvInt("DB_MIN_CONNECTIONS", 10),
		MaxConnLifetime:  getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
		MaxConnIdleTime:  getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
		BufferSize:       getEnvInt("BUFFER_SIZE", 10000),
		WorkerCount:      getEnvInt("WORKER_COUNT", 8),
		BatchSize:        getEnvInt("BATCH_SIZE", 250),
		FlushInterval:    getEnvDuration("FLUSH_INTERVAL", 100*time.Millisecond),
		OutputMode:       getEnv("OUTPUT_MODE", "timescale"),
		OutputFormat:     getEnv("OUTPUT_FORMAT", "json"),
		HealthCheckPort:  getEnvInt("HEALTH_CHECK_PORT", 8081),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.ContinuumGRPCURL == "" {
		return fmt.Errorf("CONTINUUM_GRPC_URL is required")
	}

	if c.OutputMode == "timescale" && c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required when OUTPUT_MODE=timescale")
	}

	if c.OutputMode != "console" && c.OutputMode != "timescale" {
		return fmt.Errorf("OUTPUT_MODE must be 'console' or 'timescale', got: %s", c.OutputMode)
	}

	if c.OutputFormat != "json" && c.OutputFormat != "compact" && c.OutputFormat != "table" {
		return fmt.Errorf("OUTPUT_FORMAT must be 'json', 'compact', or 'table', got: %s", c.OutputFormat)
	}

	if c.BufferSize <= 0 {
		return fmt.Errorf("BUFFER_SIZE must be positive, got: %d", c.BufferSize)
	}

	if c.WorkerCount <= 0 {
		return fmt.Errorf("WORKER_COUNT must be positive, got: %d", c.WorkerCount)
	}

	if c.BatchSize <= 0 {
		return fmt.Errorf("BATCH_SIZE must be positive, got: %d", c.BatchSize)
	}

	return nil
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvUint64(key string, defaultValue uint64) uint64 {
	if value := os.Getenv(key); value != "" {
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return uintValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
