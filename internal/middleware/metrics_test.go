package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fermilabs/fermi-api-gateway/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetrics(t *testing.T) {
	// Create metrics and registry
	m := metrics.NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with metrics middleware
	handler := Metrics(m)(testHandler)

	// Make request
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Verify metrics were recorded
	metricsFound := make(map[string]bool)
	expectedMetrics := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_response_size_bytes",
	}

	for _, mf := range metricFamilies {
		metricsFound[mf.GetName()] = true
	}

	for _, expected := range expectedMetrics {
		if !metricsFound[expected] {
			t.Errorf("Expected metric %s to be present", expected)
		}
	}
}

func TestMetrics_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := metrics.NewMetrics()
			registry := prometheus.NewRegistry()
			m.MustRegister(registry)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			handler := Metrics(m)(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, rr.Code)
			}

			// Verify metrics were recorded
			metricFamilies, err := registry.Gather()
			if err != nil {
				t.Fatalf("Failed to gather metrics: %v", err)
			}

			if len(metricFamilies) == 0 {
				t.Error("Expected metrics to be recorded")
			}
		})
	}
}

func TestMetrics_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			m := metrics.NewMetrics()
			registry := prometheus.NewRegistry()
			m.MustRegister(registry)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := Metrics(m)(testHandler)

			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rr.Code)
			}
		})
	}
}

func TestMetrics_RecordsRequestSize(t *testing.T) {
	m := metrics.NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Metrics(m)(testHandler)

	// Request with body
	req := httptest.NewRequest("POST", "/test", nil)
	req.ContentLength = 1024 // 1KB

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Find request size metric
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_request_size_bytes" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected http_request_size_bytes metric to be present")
	}
}
