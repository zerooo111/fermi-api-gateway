package pricesync

import (
	"fmt"
	"os"
	"time"
)

// Config holds price sync service configuration
type Config struct {
	MarketEndpoint    string        // MARKET_ENDPOINT_FOR_PRICE_SYNC
	PollInterval      time.Duration // PRICE_SYNC_POLL_INTERVAL (default: 1s)
	HeartbeatInterval time.Duration // PRICE_SYNC_HEARTBEAT_INTERVAL (default: 30s)
	HTTPTimeout       time.Duration // PRICE_SYNC_HTTP_TIMEOUT (default: 3s)
}

// LoadConfig reads configuration from environment variables
func LoadConfig() (*Config, error) {
	marketEndpoint := os.Getenv("MARKET_ENDPOINT_FOR_PRICE_SYNC")
	if marketEndpoint == "" {
		return nil, fmt.Errorf("MARKET_ENDPOINT_FOR_PRICE_SYNC is required")
	}

	cfg := &Config{
		MarketEndpoint:    marketEndpoint,
		PollInterval:      getDuration("PRICE_SYNC_POLL_INTERVAL", 1*time.Second),
		HeartbeatInterval: getDuration("PRICE_SYNC_HEARTBEAT_INTERVAL", 30*time.Second),
		HTTPTimeout:       getDuration("PRICE_SYNC_HTTP_TIMEOUT", 3*time.Second),
	}

	return cfg, nil
}

// getDuration reads a duration from environment variable with a default value
func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
