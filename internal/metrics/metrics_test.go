package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()

	if m == nil {
		t.Fatal("Expected metrics to be created, got nil")
	}

	if m.RequestsTotal == nil {
		t.Error("Expected RequestsTotal counter to be initialized")
	}

	if m.RequestDuration == nil {
		t.Error("Expected RequestDuration histogram to be initialized")
	}

	if m.RequestSize == nil {
		t.Error("Expected RequestSize summary to be initialized")
	}

	if m.ResponseSize == nil {
		t.Error("Expected ResponseSize summary to be initialized")
	}

	if m.RateLimitHits == nil {
		t.Error("Expected RateLimitHits counter to be initialized")
	}
}

func TestMetrics_Register(t *testing.T) {
	// Create a new registry for testing
	registry := prometheus.NewRegistry()

	m := NewMetrics()

	// Register metrics
	err := m.Register(registry)
	if err != nil {
		t.Fatalf("Failed to register metrics: %v", err)
	}

	// Try to register again (should fail with duplicate error)
	err = m.Register(registry)
	if err == nil {
		t.Error("Expected error when registering metrics twice")
	}
}

func TestMetrics_MustRegister(t *testing.T) {
	// Create a new registry for testing
	registry := prometheus.NewRegistry()

	m := NewMetrics()

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustRegister panicked: %v", r)
		}
	}()

	m.MustRegister(registry)
}

func TestMetrics_RecordRequest(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record a request
	m.RequestsTotal.WithLabelValues("GET", "/api/test", "200").Inc()

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Verify metrics were recorded
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_requests_total" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected http_requests_total metric to be present")
	}
}

func TestMetrics_RecordDuration(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record duration
	m.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.123)

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Verify metrics were recorded
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_request_duration_seconds" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected http_request_duration_seconds metric to be present")
	}
}

func TestMetrics_RecordRateLimitHit(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record rate limit hit
	m.RateLimitHits.WithLabelValues("/api/test").Inc()

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Verify metrics were recorded
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_rate_limit_hits_total" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected http_rate_limit_hits_total metric to be present")
	}
}
