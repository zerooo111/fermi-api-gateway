package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
			// Wrap handler with Recovery middleware
			handler := Recovery(tt.handler)

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

				// Check for error message in response
				if len(body) < 10 {
					t.Errorf("Error response seems too short: %s", body)
				}
			}
		})
	}
}

func TestRecovery_PreservesRequestID(t *testing.T) {
	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with both RequestID and Recovery
	handler := Recovery(RequestID(panicHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify request ID is in response even after panic
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID to be preserved in error response")
	}

	// Verify status is 500
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}
