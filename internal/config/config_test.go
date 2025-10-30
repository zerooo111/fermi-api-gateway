package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing environment variables
	os.Clearenv()

	cfg := Load()

	// Test Server defaults
	if cfg.Server.Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", cfg.Server.Port)
	}
	if cfg.Server.Env != "development" {
		t.Errorf("Expected default env development, got %s", cfg.Server.Env)
	}

	// Test CORS defaults
	if len(cfg.CORS.AllowedOrigins) != 1 || cfg.CORS.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("Expected default CORS origin [http://localhost:3000], got %v", cfg.CORS.AllowedOrigins)
	}

	// Test Backend defaults
	if cfg.Backend.RollupURL != "http://localhost:3000" {
		t.Errorf("Expected default Rollup URL, got %s", cfg.Backend.RollupURL)
	}
	if cfg.Backend.ContinuumGrpcURL != "localhost:9090" {
		t.Errorf("Expected default Continuum gRPC URL, got %s", cfg.Backend.ContinuumGrpcURL)
	}
	if cfg.Backend.ContinuumRestURL != "http://localhost:8081" {
		t.Errorf("Expected default Continuum REST URL, got %s", cfg.Backend.ContinuumRestURL)
	}

	// Test RateLimit defaults
	if cfg.RateLimit.RollupRPM != 1000 {
		t.Errorf("Expected default Rollup rate limit 1000, got %d", cfg.RateLimit.RollupRPM)
	}
	if cfg.RateLimit.ContinuumGrpcRPM != 500 {
		t.Errorf("Expected default Continuum gRPC rate limit 500, got %d", cfg.RateLimit.ContinuumGrpcRPM)
	}
	if cfg.RateLimit.ContinuumRestRPM != 2000 {
		t.Errorf("Expected default Continuum REST rate limit 2000, got %d", cfg.RateLimit.ContinuumRestRPM)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	// Set custom environment variables
	os.Setenv("PORT", "9000")
	os.Setenv("ENV", "production")
	os.Setenv("ALLOWED_ORIGINS", "https://example.com,https://app.example.com")
	os.Setenv("ROLLUP_URL", "https://rollup.example.com")
	os.Setenv("CONTINUUM_GRPC_URL", "grpc.example.com:9090")
	os.Setenv("CONTINUUM_REST_URL", "https://continuum.example.com")
	os.Setenv("RATE_LIMIT_ROLLUP", "5000")
	os.Setenv("RATE_LIMIT_CONTINUUM_GRPC", "2500")
	os.Setenv("RATE_LIMIT_CONTINUUM_REST", "10000")

	// Clean up after test
	defer os.Clearenv()

	cfg := Load()

	// Test Server custom values
	if cfg.Server.Port != "9000" {
		t.Errorf("Expected port 9000, got %s", cfg.Server.Port)
	}
	if cfg.Server.Env != "production" {
		t.Errorf("Expected env production, got %s", cfg.Server.Env)
	}

	// Test CORS custom values
	expectedOrigins := []string{"https://example.com", "https://app.example.com"}
	if len(cfg.CORS.AllowedOrigins) != len(expectedOrigins) {
		t.Errorf("Expected %d origins, got %d", len(expectedOrigins), len(cfg.CORS.AllowedOrigins))
	}
	for i, origin := range expectedOrigins {
		if cfg.CORS.AllowedOrigins[i] != origin {
			t.Errorf("Expected origin %s at index %d, got %s", origin, i, cfg.CORS.AllowedOrigins[i])
		}
	}

	// Test Backend custom values
	if cfg.Backend.RollupURL != "https://rollup.example.com" {
		t.Errorf("Expected custom Rollup URL, got %s", cfg.Backend.RollupURL)
	}
	if cfg.Backend.ContinuumGrpcURL != "grpc.example.com:9090" {
		t.Errorf("Expected custom Continuum gRPC URL, got %s", cfg.Backend.ContinuumGrpcURL)
	}
	if cfg.Backend.ContinuumRestURL != "https://continuum.example.com" {
		t.Errorf("Expected custom Continuum REST URL, got %s", cfg.Backend.ContinuumRestURL)
	}

	// Test RateLimit custom values
	if cfg.RateLimit.RollupRPM != 5000 {
		t.Errorf("Expected Rollup rate limit 5000, got %d", cfg.RateLimit.RollupRPM)
	}
	if cfg.RateLimit.ContinuumGrpcRPM != 2500 {
		t.Errorf("Expected Continuum gRPC rate limit 2500, got %d", cfg.RateLimit.ContinuumGrpcRPM)
	}
	if cfg.RateLimit.ContinuumRestRPM != 10000 {
		t.Errorf("Expected Continuum REST rate limit 10000, got %d", cfg.RateLimit.ContinuumRestRPM)
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "returns default when env not set",
			key:          "TEST_KEY_1",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "returns env value when set",
			key:          "TEST_KEY_2",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "returns empty string from env",
			key:          "TEST_KEY_3",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			// Execute
			result := getEnv(tt.key, tt.defaultValue)

			// Assert
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		expected     int
	}{
		{
			name:         "returns default when env not set",
			key:          "TEST_INT_1",
			defaultValue: 100,
			envValue:     "",
			expected:     100,
		},
		{
			name:         "returns env value when valid int",
			key:          "TEST_INT_2",
			defaultValue: 100,
			envValue:     "500",
			expected:     500,
		},
		{
			name:         "returns default when env value is not valid int",
			key:          "TEST_INT_3",
			defaultValue: 100,
			envValue:     "invalid",
			expected:     100,
		},
		{
			name:         "handles negative numbers",
			key:          "TEST_INT_4",
			defaultValue: 100,
			envValue:     "-50",
			expected:     -50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			// Execute
			result := getEnvInt(tt.key, tt.defaultValue)

			// Assert
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetEnvSlice(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue []string
		envValue     string
		expected     []string
	}{
		{
			name:         "returns default when env not set",
			key:          "TEST_SLICE_1",
			defaultValue: []string{"default1", "default2"},
			envValue:     "",
			expected:     []string{"default1", "default2"},
		},
		{
			name:         "parses single value",
			key:          "TEST_SLICE_2",
			defaultValue: []string{"default"},
			envValue:     "value1",
			expected:     []string{"value1"},
		},
		{
			name:         "parses multiple values",
			key:          "TEST_SLICE_3",
			defaultValue: []string{"default"},
			envValue:     "value1,value2,value3",
			expected:     []string{"value1", "value2", "value3"},
		},
		{
			name:         "handles trailing comma",
			key:          "TEST_SLICE_4",
			defaultValue: []string{"default"},
			envValue:     "value1,value2,",
			expected:     []string{"value1", "value2"},
		},
		{
			name:         "handles leading comma",
			key:          "TEST_SLICE_5",
			defaultValue: []string{"default"},
			envValue:     ",value1,value2",
			expected:     []string{"value1", "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			// Execute
			result := getEnvSlice(tt.key, tt.defaultValue)

			// Assert
			if len(result) != len(tt.expected) {
				t.Errorf("Expected slice length %d, got %d", len(tt.expected), len(result))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("At index %d: expected %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}
