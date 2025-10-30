package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		expectedAllow  bool
		method         string
	}{
		{
			name:           "allowed origin should be permitted",
			allowedOrigins: []string{"http://localhost:3000", "https://example.com"},
			requestOrigin:  "http://localhost:3000",
			expectedAllow:  true,
			method:         "GET",
		},
		{
			name:           "disallowed origin should be rejected",
			allowedOrigins: []string{"http://localhost:3000"},
			requestOrigin:  "http://evil.com",
			expectedAllow:  false,
			method:         "GET",
		},
		{
			name:           "multiple allowed origins",
			allowedOrigins: []string{"http://localhost:3000", "https://app.example.com"},
			requestOrigin:  "https://app.example.com",
			expectedAllow:  true,
			method:         "GET",
		},
		{
			name:           "OPTIONS preflight request",
			allowedOrigins: []string{"http://localhost:3000"},
			requestOrigin:  "http://localhost:3000",
			expectedAllow:  true,
			method:         "OPTIONS",
		},
		{
			name:           "no origin header should not set CORS headers",
			allowedOrigins: []string{"http://localhost:3000"},
			requestOrigin:  "",
			expectedAllow:  false,
			method:         "GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that returns 200
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			// Wrap with CORS middleware
			handler := CORS(tt.allowedOrigins)(testHandler)

			// Create request
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}

			// Record response
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Check CORS headers
			if tt.expectedAllow {
				// Should have CORS headers
				if rr.Header().Get("Access-Control-Allow-Origin") != tt.requestOrigin {
					t.Errorf("Expected Access-Control-Allow-Origin to be %s, got %s",
						tt.requestOrigin, rr.Header().Get("Access-Control-Allow-Origin"))
				}

				if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Errorf("Expected Access-Control-Allow-Credentials to be true, got %s",
						rr.Header().Get("Access-Control-Allow-Credentials"))
				}

				// Check preflight response
				if tt.method == "OPTIONS" {
					if rr.Code != http.StatusNoContent {
						t.Errorf("Expected status code %d for OPTIONS, got %d", http.StatusNoContent, rr.Code)
					}

					if rr.Header().Get("Access-Control-Allow-Methods") == "" {
						t.Error("Expected Access-Control-Allow-Methods header for OPTIONS request")
					}

					if rr.Header().Get("Access-Control-Allow-Headers") == "" {
						t.Error("Expected Access-Control-Allow-Headers header for OPTIONS request")
					}
				} else {
					// Non-preflight should call next handler
					if rr.Code != http.StatusOK {
						t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
					}
				}
			} else {
				// Should not have CORS headers
				if rr.Header().Get("Access-Control-Allow-Origin") != "" {
					t.Errorf("Expected no Access-Control-Allow-Origin header, got %s",
						rr.Header().Get("Access-Control-Allow-Origin"))
				}
			}
		})
	}
}

func TestCORS_EmptyAllowedOrigins(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS([]string{})(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// No allowed origins means no CORS headers should be set
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("Expected no CORS headers with empty allowed origins, got %s",
			rr.Header().Get("Access-Control-Allow-Origin"))
	}
}
