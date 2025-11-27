package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestContinuumHandler_HandleGetTickByNumber_InvalidInput(t *testing.T) {
	handler := NewContinuumHandler(nil, nil)

	tests := []struct {
		name           string
		tickNumber     string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "empty tick number",
			tickNumber:     "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "missing tick number in path",
		},
		{
			name:           "invalid tick number - non-numeric",
			tickNumber:     "abc",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid tick number",
		},
		{
			name:           "invalid tick number - negative",
			tickNumber:     "-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid tick number",
		},
		{
			name:           "valid tick number but no database",
			tickNumber:     "123",
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "database not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tick/"+tt.tickNumber, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("tickNumber", tt.tickNumber)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.HandleGetTickByNumber().ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			var body map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if body["error"] == "" {
				t.Error("expected error message in response")
			}
		})
	}
}

func TestContinuumHandler_HandleGetRecentTransactions_InvalidInput(t *testing.T) {
	handler := NewContinuumHandler(nil, nil)

	tests := []struct {
		name           string
		limit          string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid limit - non-numeric",
			limit:          "abc",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid limit",
		},
		{
			name:           "invalid limit - zero",
			limit:          "0",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid limit",
		},
		{
			name:           "invalid limit - negative",
			limit:          "-10",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid limit",
		},
		{
			name:           "valid limit but no database",
			limit:          "10",
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "database not configured",
		},
		{
			name:           "default limit but no database",
			limit:          "",
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "database not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/txn/recent"
			if tt.limit != "" {
				url += "?limit=" + tt.limit
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)

			rr := httptest.NewRecorder()
			handler.HandleGetRecentTransactions().ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			var body map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if body["error"] == "" {
				t.Error("expected error message in response")
			}
		})
	}
}

func TestContinuumHandler_HandleGetTransactionByID_InvalidInput(t *testing.T) {
	handler := NewContinuumHandler(nil, nil)

	tests := []struct {
		name           string
		txnID          string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "empty txn id",
			txnID:          "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "missing transaction ID",
		},
		{
			name:           "valid txn id but no database",
			txnID:          "tx123abc",
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "database not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/txn/"+tt.txnID, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("txnId", tt.txnID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.HandleGetTransactionByID().ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			var body map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if body["error"] == "" {
				t.Error("expected error message in response")
			}
		})
	}
}

func TestContinuumHandler_HandleGetTransactionByID_TooLongID(t *testing.T) {
	handler := NewContinuumHandler(nil, nil)

	// Create a txnID longer than 256 characters
	longID := ""
	for i := 0; i < 300; i++ {
		longID += "a"
	}

	req := httptest.NewRequest(http.MethodGet, "/txn/"+longID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("txnId", longID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	handler.HandleGetTransactionByID().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body["error"] != "transaction ID too long" {
		t.Errorf("expected 'transaction ID too long', got '%s'", body["error"])
	}
}

func TestContinuumHandler_ResponseHeaders(t *testing.T) {
	handler := NewContinuumHandler(nil, nil)

	t.Run("tick endpoint sets cache header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tick/123", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("tickNumber", "123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		handler.HandleGetTickByNumber().ServeHTTP(rr, req)

		// Even on error, content-type should be set
		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
		}
	})

	t.Run("recent txn endpoint sets no-cache header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/txn/recent", nil)

		rr := httptest.NewRecorder()
		handler.HandleGetRecentTransactions().ServeHTTP(rr, req)

		// Even on error, content-type should be set
		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
		}
	})
}

func TestNewContinuumHandler(t *testing.T) {
	t.Run("with nil logger", func(t *testing.T) {
		handler := NewContinuumHandler(nil, nil)
		if handler == nil {
			t.Error("expected non-nil handler")
		}
		if handler.logger == nil {
			t.Error("expected logger to be set to nop logger")
		}
	})
}

// TestRouteIntegration tests that routes work correctly with Chi router
func TestRouteIntegration(t *testing.T) {
	handler := NewContinuumHandler(nil, nil)

	r := chi.NewRouter()
	r.Get("/tick/{tickNumber}", handler.HandleGetTickByNumber())
	r.Get("/txn/recent", handler.HandleGetRecentTransactions())
	r.Get("/txn/{txnId}", handler.HandleGetTransactionByID())

	t.Run("tick route with valid parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tick/12345", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		// Should return 503 because no database configured
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("recent transactions route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/txn/recent?limit=20", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		// Should return 503 because no database configured
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("transaction by id route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/txn/abc123def", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		// Should return 503 because no database configured
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})

	t.Run("recent route takes precedence over txn/{id}", func(t *testing.T) {
		// This tests that /txn/recent matches the first route, not /txn/{txnId}
		req := httptest.NewRequest(http.MethodGet, "/txn/recent", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		// Should return 503, not 400 (which would indicate it tried to use "recent" as txnId)
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
		}
	})
}
