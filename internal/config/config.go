package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	CORS      CORSConfig
	Backend   BackendConfig
	Database  DatabaseConfig
	RateLimit RateLimitConfig
	PriceSync PriceSyncConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port string
	Env  string // development, staging, production
}

// CORSConfig holds CORS middleware configuration
type CORSConfig struct {
	AllowedOrigins []string
}

// BackendConfig holds backend service URLs
type BackendConfig struct {
	RollupURL        string
	ContinuumGrpcURL string
	ContinuumRestURL string
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RateLimitConfig holds rate limiting configuration per route
type RateLimitConfig struct {
	RollupRPM        int // Requests per minute
	ContinuumGrpcRPM int
	ContinuumRestRPM int
}

// PriceSyncConfig holds price sync service configuration
type PriceSyncConfig struct {
	Enabled           bool
	MarketEndpoint    string
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	HTTPTimeout       time.Duration
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
			Env:  getEnv("ENV", "development"),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		},
		Backend: BackendConfig{
			RollupURL:        getEnv("ROLLUP_URL", "http://localhost:3000"),
			ContinuumGrpcURL: getEnv("CONTINUUM_GRPC_URL", "localhost:9090"),
			ContinuumRestURL: getEnv("CONTINUUM_REST_URL", "https://18.220.137.190"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "continuum"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		RateLimit: RateLimitConfig{
			RollupRPM:        getEnvInt("RATE_LIMIT_ROLLUP", 1000),
			ContinuumGrpcRPM: getEnvInt("RATE_LIMIT_CONTINUUM_GRPC", 500),
			ContinuumRestRPM: getEnvInt("RATE_LIMIT_CONTINUUM_REST", 2000),
		},
		PriceSync: PriceSyncConfig{
			Enabled:           getEnv("PRICE_SYNC_ENABLED", "false") == "true",
			MarketEndpoint:    getEnv("MARKET_ENDPOINT_FOR_PRICE_SYNC", ""),
			PollInterval:      getEnvDuration("PRICE_SYNC_POLL_INTERVAL", 1*time.Second),
			HeartbeatInterval: getEnvDuration("PRICE_SYNC_HEARTBEAT_INTERVAL", 30*time.Second),
			HTTPTimeout:       getEnvDuration("PRICE_SYNC_HTTP_TIMEOUT", 3*time.Second),
		},
	}
}

// Helper functions to read environment variables with defaults
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

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple split by comma for now
		// In production, you might want to use a proper CSV parser
		result := []string{}
		current := ""
		for _, char := range value {
			if char == ',' {
				if current != "" {
					result = append(result, current)
					current = ""
				}
			} else {
				current += string(char)
			}
		}
		if current != "" {
			result = append(result, current)
		}
		return result
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
