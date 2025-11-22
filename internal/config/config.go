package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	CORS      CORSConfig
	Backend   BackendConfig
	Database  DatabaseConfig
	RateLimit RateLimitConfig
	Redpanda  RedpandaConfig
	Stream    StreamConfig
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

// RedpandaConfig holds Kafka/Redpanda connection configuration
type RedpandaConfig struct {
	Brokers           []string
	SASLUsername      string
	SASLPassword      string
	SASLMechanism     string
	TicksTopic        string
	TransactionsTopic string
	BufferSize        int
	BatchSize         int
	LingerMS          int
	Compression       string
}

// StreamConfig holds SSE streaming configuration
type StreamConfig struct {
	BufferSize int // Number of recent ticks/transactions to keep
	UpdateFPS  int // Frames per second for SSE updates
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
			ContinuumRestURL: getEnv("CONTINUUM_REST_URL", "http://localhost:8081"),
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
		Redpanda: RedpandaConfig{
			Brokers:           getEnvSlice("REDPANDA_BROKERS", []string{"localhost:9092"}),
			SASLUsername:      getEnv("REDPANDA_SASL_USERNAME", ""),
			SASLPassword:      getEnv("REDPANDA_SASL_PASSWORD", ""),
			SASLMechanism:     getEnv("REDPANDA_SASL_MECHANISM", "SCRAM-SHA-256"),
			TicksTopic:        getEnv("REDPANDA_TICKS_TOPIC", "ticks"),
			TransactionsTopic: getEnv("REDPANDA_TRANSACTIONS_TOPIC", "transactions"),
			BufferSize:        getEnvInt("KAFKA_BUFFER_SIZE", 500000),
			BatchSize:         getEnvInt("KAFKA_BATCH_SIZE", 1048576),
			LingerMS:          getEnvInt("KAFKA_LINGER_MS", 0),
			Compression:       getEnv("KAFKA_COMPRESSION", "none"),
		},
		Stream: StreamConfig{
			BufferSize: getEnvInt("STREAM_BUFFER_SIZE", 100),
			UpdateFPS:  getEnvInt("STREAM_UPDATE_FPS", 60),
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
