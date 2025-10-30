package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLogging(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		statusCode     int
		expectedFields []string
	}{
		{
			name:       "logs GET request",
			method:     "GET",
			path:       "/api/test",
			statusCode: http.StatusOK,
			expectedFields: []string{
				"method", "GET",
				"path", "/api/test",
				"status", "200",
			},
		},
		{
			name:       "logs POST request",
			method:     "POST",
			path:       "/api/create",
			statusCode: http.StatusCreated,
			expectedFields: []string{
				"method", "POST",
				"path", "/api/create",
				"status", "201",
			},
		},
		{
			name:       "logs error status",
			method:     "GET",
			path:       "/api/notfound",
			statusCode: http.StatusNotFound,
			expectedFields: []string{
				"method", "GET",
				"status", "404",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture log output
			var buf bytes.Buffer

			// Create logger that writes to buffer
			encoderConfig := zapcore.EncoderConfig{
				TimeKey:        "time",
				LevelKey:       "level",
				NameKey:        "logger",
				CallerKey:      "caller",
				MessageKey:     "msg",
				StacktraceKey:  "stacktrace",
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.LowercaseLevelEncoder,
				EncodeTime:     zapcore.ISO8601TimeEncoder,
				EncodeDuration: zapcore.SecondsDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			}

			core := zapcore.NewCore(
				zapcore.NewJSONEncoder(encoderConfig),
				zapcore.AddSync(&buf),
				zapcore.InfoLevel,
			)
			logger := zap.New(core)

			// Create test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("response"))
			})

			// Wrap with logging middleware
			handler := Logging(logger)(testHandler)

			// Create request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("X-Request-ID", "test-request-123")

			// Record response
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Check that log was written
			logOutput := buf.String()
			if logOutput == "" {
				t.Error("Expected log output, got empty string")
			}

			// Verify expected fields are in log
			for _, field := range tt.expectedFields {
				if !strings.Contains(logOutput, field) {
					t.Errorf("Expected log to contain '%s', got: %s", field, logOutput)
				}
			}

			// Verify request ID is logged
			if !strings.Contains(logOutput, "test-request-123") {
				t.Errorf("Expected log to contain request ID, got: %s", logOutput)
			}

			// Verify duration is logged
			if !strings.Contains(logOutput, "duration") {
				t.Errorf("Expected log to contain duration, got: %s", logOutput)
			}
		})
	}
}

func TestLogging_WithoutRequestID(t *testing.T) {
	var buf bytes.Buffer

	encoderConfig := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	logger := zap.New(core)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Logging(logger)(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	// Intentionally not setting X-Request-ID

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Log should still be written even without request ID
	if buf.String() == "" {
		t.Error("Expected log output even without request ID")
	}
}
