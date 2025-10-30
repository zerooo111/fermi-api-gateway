package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name              string
		existingRequestID string
		shouldGenerate    bool
	}{
		{
			name:              "generates request ID when none exists",
			existingRequestID: "",
			shouldGenerate:    true,
		},
		{
			name:              "uses existing X-Request-ID header",
			existingRequestID: "existing-request-id-123",
			shouldGenerate:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequestID string
			var capturedContext context.Context

			// Test handler that captures the request ID
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequestID = r.Header.Get("X-Request-ID")
				capturedContext = r.Context()
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with RequestID middleware
			handler := RequestID(testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.existingRequestID != "" {
				req.Header.Set("X-Request-ID", tt.existingRequestID)
			}

			// Record response
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Verify request ID was set
			if capturedRequestID == "" {
				t.Error("Expected request ID to be set in header")
			}

			// Verify response header
			if rr.Header().Get("X-Request-ID") == "" {
				t.Error("Expected X-Request-ID in response header")
			}

			if rr.Header().Get("X-Request-ID") != capturedRequestID {
				t.Errorf("Expected response X-Request-ID to match request, got %s vs %s",
					rr.Header().Get("X-Request-ID"), capturedRequestID)
			}

			// Verify existing ID is preserved
			if tt.existingRequestID != "" {
				if capturedRequestID != tt.existingRequestID {
					t.Errorf("Expected existing request ID to be preserved, got %s, want %s",
						capturedRequestID, tt.existingRequestID)
				}
			}

			// Verify new ID is generated when needed
			if tt.shouldGenerate {
				if len(capturedRequestID) < 16 {
					t.Errorf("Generated request ID seems too short: %s", capturedRequestID)
				}
			}

			// Verify request ID is in context
			ctxRequestID := capturedContext.Value(RequestIDKey)
			if ctxRequestID == nil {
				t.Error("Expected request ID to be in context")
			}

			if ctxRequestID.(string) != capturedRequestID {
				t.Errorf("Expected context request ID to match header, got %s vs %s",
					ctxRequestID, capturedRequestID)
			}
		})
	}
}

func TestRequestID_Uniqueness(t *testing.T) {
	// Test that generated IDs are unique - increased to 1000 iterations for production-grade verification
	ids := make(map[string]bool)
	iterations := 1000

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(testHandler)

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		id := rr.Header().Get("X-Request-ID")
		if id == "" {
			t.Fatal("Generated request ID is empty")
		}

		// Verify ID format (should be 32 hex characters)
		if len(id) != 32 {
			t.Errorf("Expected request ID length 32, got %d: %s", len(id), id)
		}

		// Verify ID is hexadecimal
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Request ID contains non-hex character '%c': %s", c, id)
			}
		}

		if ids[id] {
			t.Fatalf("Duplicate request ID generated at iteration %d: %s", i, id)
		}
		ids[id] = true
	}

	if len(ids) != iterations {
		t.Errorf("Expected %d unique IDs, got %d", iterations, len(ids))
	}
}

// TestRequestID_ConcurrentGeneration tests concurrent request ID generation
func TestRequestID_ConcurrentGeneration(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(testHandler)

	// Use channels to collect IDs from goroutines
	idChan := make(chan string, 100)
	var wg sync.WaitGroup

	// Generate 100 IDs concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			idChan <- rr.Header().Get("X-Request-ID")
		}()
	}

	wg.Wait()
	close(idChan)

	// Collect and verify uniqueness
	ids := make(map[string]bool)
	for id := range idChan {
		if ids[id] {
			t.Errorf("Duplicate request ID in concurrent test: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != 100 {
		t.Errorf("Expected 100 unique IDs, got %d", len(ids))
	}
}

// TestRequestID_Format verifies the format of generated request IDs
func TestRequestID_Format(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-ID")

	// Verify length is 32 characters (16 bytes hex encoded)
	if len(id) != 32 {
		t.Errorf("Expected request ID length 32, got %d", len(id))
	}

	// Verify all characters are hex
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			t.Errorf("Request ID contains invalid character '%c'", c)
		}
	}
}
