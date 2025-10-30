package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	// Create a request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handler := Handler()
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check content type
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Handler returned wrong content type: got %v want %v", contentType, expectedContentType)
	}

	// Parse response body
	var status Status
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Check status field
	if status.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", status.Status)
	}

	// Check version field
	if status.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", status.Version)
	}

	// Check timestamp is recent (within last 5 seconds)
	now := time.Now()
	timeDiff := now.Sub(status.Timestamp)
	if timeDiff < 0 || timeDiff > 5*time.Second {
		t.Errorf("Timestamp seems off: %v (diff: %v)", status.Timestamp, timeDiff)
	}
}

func TestReadyHandler(t *testing.T) {
	// Create a request
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handler := ReadyHandler()
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check content type
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Handler returned wrong content type: got %v want %v", contentType, expectedContentType)
	}

	// Parse response body
	var status Status
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Check status field
	if status.Status != "ready" {
		t.Errorf("Expected status 'ready', got '%s'", status.Status)
	}

	// Check version field
	if status.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", status.Version)
	}

	// Check timestamp is recent (within last 5 seconds)
	now := time.Now()
	timeDiff := now.Sub(status.Timestamp)
	if timeDiff < 0 || timeDiff > 5*time.Second {
		t.Errorf("Timestamp seems off: %v (diff: %v)", status.Timestamp, timeDiff)
	}
}

func TestHealthHandler_MultipleRequests(t *testing.T) {
	handler := Handler()

	// Make multiple requests to ensure handler is stateless
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Request %d: Handler returned wrong status code: got %v want %v", i, status, http.StatusOK)
		}

		var status Status
		if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
			t.Fatalf("Request %d: Failed to decode response body: %v", i, err)
		}

		if status.Status != "healthy" {
			t.Errorf("Request %d: Expected status 'healthy', got '%s'", i, status.Status)
		}
	}
}

func TestReadyHandler_MultipleRequests(t *testing.T) {
	handler := ReadyHandler()

	// Make multiple requests to ensure handler is stateless
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Request %d: Handler returned wrong status code: got %v want %v", i, status, http.StatusOK)
		}

		var status Status
		if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
			t.Fatalf("Request %d: Failed to decode response body: %v", i, err)
		}

		if status.Status != "ready" {
			t.Errorf("Request %d: Expected status 'ready', got '%s'", i, status.Status)
		}
	}
}

// Benchmark tests
func BenchmarkHealthHandler(b *testing.B) {
	handler := Handler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkReadyHandler(b *testing.B) {
	handler := ReadyHandler()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
