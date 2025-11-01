package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
)

func TestRecovery(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		shouldPanic    bool
		expectedStatus int
	}{
		{
			name: "normal request without panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			},
			shouldPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name: "recovers from panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("something went wrong")
			},
			shouldPanic:    true,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "recovers from string panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("error message")
			},
			shouldPanic:    true,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "recovers from error panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(http.ErrAbortHandler)
			},
			shouldPanic:    true,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test logger
			logger := zap.NewNop() // No-op logger for tests

			// Wrap handler with Recovery middleware
			handler := Recovery(logger)(tt.handler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			// Execute request (should not panic at test level)
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, rr.Code)
			}

			// If panic was expected, verify error response
			if tt.shouldPanic {
				if rr.Header().Get("Content-Type") != "application/json" {
					t.Error("Expected JSON content type for error response")
				}

				body := rr.Body.String()
				if body == "" {
					t.Error("Expected error response body, got empty")
				}

				// Parse JSON response and validate structure
				var response map[string]interface{}
				if err := json.Unmarshal([]byte(body), &response); err != nil {
					t.Fatalf("Failed to parse JSON response: %v", err)
				}

				// Verify required fields exist
				if _, ok := response["error"]; !ok {
					t.Error("Expected 'error' field in response")
				}
				if _, ok := response["message"]; !ok {
					t.Error("Expected 'message' field in response")
				}

				// Verify error message is generic (not the actual panic message)
				errorMsg, _ := response["error"].(string)
				if errorMsg != "Internal Server Error" {
					t.Errorf("Expected error to be 'Internal Server Error', got '%s'", errorMsg)
				}

				msg, _ := response["message"].(string)
				if msg != "An unexpected error occurred" {
					t.Errorf("Expected message to be 'An unexpected error occurred', got '%s'", msg)
				}
			}
		})
	}
}

func TestRecovery_PreservesRequestID(t *testing.T) {
	// Create test logger
	logger := zap.NewNop()

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with both RequestID and Recovery
	handler := Recovery(logger)(RequestID(panicHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify request ID is in response even after panic
	requestID := rr.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID to be preserved in error response")
	}

	// Verify status is 500
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	// Parse JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify request_id is in JSON body
	if respID, ok := response["request_id"].(string); !ok || respID != requestID {
		t.Errorf("Expected request_id in body to match header: %s", requestID)
	}
}

// TestRecovery_NeverExposePanicMessage is a CRITICAL SECURITY TEST
// Verifies that panic messages containing sensitive data are NEVER exposed to clients
func TestRecovery_NeverExposePanicMessage(t *testing.T) {
	sensitiveData := []string{
		"PASSWORD: secret123",
		"API_KEY: sk-1234567890abcdef",
		"SECRET_TOKEN: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"database connection failed: user=admin password=AdminPass123 host=prod-db",
		"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	for _, sensitiveMsg := range sensitiveData {
		testName := sensitiveMsg
		if len(testName) > 20 {
			testName = testName[:20]
		}
		t.Run("panic with: "+testName, func(t *testing.T) {
			// Create test logger
			logger := zap.NewNop()

			handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(sensitiveMsg)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Verify 500 status
			if rr.Code != http.StatusInternalServerError {
				t.Errorf("Expected status 500, got %d", rr.Code)
			}

			body := rr.Body.String()

			// Parse JSON
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(body), &response); err != nil {
				t.Fatalf("Failed to parse JSON response: %v", err)
			}

			// CRITICAL: Verify sensitive data is NOT in response
			for _, sensitive := range []string{"PASSWORD", "secret123", "API_KEY", "sk-", "SECRET_TOKEN", "password=", "AWS_SECRET"} {
				if strings.Contains(body, sensitive) {
					t.Errorf("SECURITY ISSUE: Response contains sensitive data '%s': %s", sensitive, body)
				}
			}

			// Verify only generic message is returned
			errorMsg, _ := response["error"].(string)
			if errorMsg != "Internal Server Error" {
				t.Errorf("Expected generic error message, got '%s'", errorMsg)
			}

			msg, _ := response["message"].(string)
			if msg != "An unexpected error occurred" {
				t.Errorf("Expected generic message, got '%s'", msg)
			}
		})
	}
}

// TestRecovery_ConcurrentPanics tests that concurrent panics are handled safely
func TestRecovery_ConcurrentPanics(t *testing.T) {
	// Create test logger
	logger := zap.NewNop()

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("concurrent panic")
	}))

	var wg sync.WaitGroup
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Verify each request gets proper error response
			if rr.Code != http.StatusInternalServerError {
				t.Errorf("Request %d: Expected status 500, got %d", id, rr.Code)
			}

			var response map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Errorf("Request %d: Failed to parse JSON: %v", id, err)
				return
			}

			if response["error"] != "Internal Server Error" {
				t.Errorf("Request %d: Expected generic error message", id)
			}
		}(i)
	}

	wg.Wait()
}

// TestRecovery_PanicAfterWriteHeader tests panic after headers are written
func TestRecovery_PanicAfterWriteHeader(t *testing.T) {
	// Create test logger
	logger := zap.NewNop()

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial response"))
		panic("panic after write")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Status should be 200 (already written before panic)
	// But response should still include error handling attempt
	if rr.Code != http.StatusOK {
		t.Logf("Note: Status is %d (headers were already written)", rr.Code)
	}

	// Verify the handler didn't crash the test
	body := rr.Body.String()
	if body == "" {
		t.Error("Expected some response body")
	}
}

// TestRecovery_NilPanic tests recovery from nil pointer panic
func TestRecovery_NilPanic(t *testing.T) {
	// Create test logger
	logger := zap.NewNop()

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p *int
		_ = *p // nil pointer dereference
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify generic error message (not "runtime error: invalid memory address")
	if response["error"] != "Internal Server Error" {
		t.Errorf("Expected generic error, got %v", response["error"])
	}
}

// TestRecovery_JSONStructure validates exact JSON response structure
func TestRecovery_JSONStructure(t *testing.T) {
	// Create test logger
	logger := zap.NewNop()

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify exact structure
	expectedFields := []string{"error", "message", "request_id"}
	for _, field := range expectedFields {
		if _, ok := response[field]; !ok {
			t.Errorf("Expected field '%s' in response", field)
		}
	}

	// Verify no extra fields
	if len(response) != len(expectedFields) {
		t.Errorf("Expected exactly %d fields, got %d: %v", len(expectedFields), len(response), response)
	}

	// Verify field types
	if _, ok := response["error"].(string); !ok {
		t.Error("Expected 'error' to be a string")
	}
	if _, ok := response["message"].(string); !ok {
		t.Error("Expected 'message' to be a string")
	}
	if _, ok := response["request_id"].(string); !ok {
		t.Error("Expected 'request_id' to be a string")
	}
}
