package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

func TestMetrics_MustRegister_PanicsOnDuplicate(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := NewMetrics()
	m.MustRegister(registry)

	// Should panic on duplicate registration
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected MustRegister to panic on duplicate registration")
		}
	}()

	m.MustRegister(registry)
}

func TestMetrics_RecordRequest(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record 3 requests with the same labels
	m.RequestsTotal.WithLabelValues("GET", "/api/test", "200").Inc()
	m.RequestsTotal.WithLabelValues("GET", "/api/test", "200").Inc()
	m.RequestsTotal.WithLabelValues("GET", "/api/test", "200").Inc()

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Find and verify http_requests_total
	var found bool
	var counterValue float64
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_requests_total" {
			found = true
			if len(mf.GetMetric()) == 0 {
				t.Fatal("Expected at least one metric")
			}

			// Verify the counter value is 3
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				if hasLabels(labels, map[string]string{"method": "GET", "path": "/api/test", "status": "200"}) {
					counterValue = m.GetCounter().GetValue()
					break
				}
			}
		}
	}

	if !found {
		t.Error("Expected http_requests_total metric to be present")
	}

	if counterValue != 3.0 {
		t.Errorf("Expected counter value to be 3, got %f", counterValue)
	}
}

func TestMetrics_RecordRequest_DifferentLabels(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record requests with different labels
	m.RequestsTotal.WithLabelValues("GET", "/api/test", "200").Inc()
	m.RequestsTotal.WithLabelValues("POST", "/api/test", "201").Inc()
	m.RequestsTotal.WithLabelValues("GET", "/api/users", "200").Inc()

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Find http_requests_total
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_requests_total" {
			// Should have 3 different metric series
			if len(mf.GetMetric()) != 3 {
				t.Errorf("Expected 3 metric series, got %d", len(mf.GetMetric()))
			}

			// Verify each has value of 1
			for _, m := range mf.GetMetric() {
				value := m.GetCounter().GetValue()
				if value != 1.0 {
					t.Errorf("Expected counter value to be 1, got %f", value)
				}
			}
		}
	}
}

func TestMetrics_RecordDuration(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record durations
	m.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.123)
	m.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.456)
	m.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.789)

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Find and verify histogram
	var found bool
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_request_duration_seconds" {
			found = true
			if len(mf.GetMetric()) == 0 {
				t.Fatal("Expected at least one metric")
			}

			// Verify histogram has the right labels
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				if hasLabels(labels, map[string]string{"method": "GET", "path": "/api/test", "status": "200"}) {
					histogram := m.GetHistogram()

					// Verify sample count is 3
					if histogram.GetSampleCount() != 3 {
						t.Errorf("Expected sample count to be 3, got %d", histogram.GetSampleCount())
					}

					// Verify sum is approximately 0.123 + 0.456 + 0.789 = 1.368
					expectedSum := 1.368
					actualSum := histogram.GetSampleSum()
					if actualSum < expectedSum-0.001 || actualSum > expectedSum+0.001 {
						t.Errorf("Expected sum to be approximately %f, got %f", expectedSum, actualSum)
					}

					// Verify histogram has buckets
					buckets := histogram.GetBucket()
					if len(buckets) == 0 {
						t.Error("Expected histogram to have buckets")
					}

					// Verify cumulative counts are correct
					// All 3 samples should be in buckets >= 1.0
					for _, bucket := range buckets {
						if bucket.GetUpperBound() >= 1.0 {
							if bucket.GetCumulativeCount() < 3 {
								t.Errorf("Expected bucket %f to have cumulative count >= 3, got %d",
									bucket.GetUpperBound(), bucket.GetCumulativeCount())
							}
							break
						}
					}
				}
			}
		}
	}

	if !found {
		t.Error("Expected http_request_duration_seconds metric to be present")
	}
}

func TestMetrics_HistogramBuckets(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record samples in different buckets
	m.RequestDuration.WithLabelValues("GET", "/fast", "200").Observe(0.001)  // < 0.005
	m.RequestDuration.WithLabelValues("GET", "/medium", "200").Observe(0.1)  // 0.1 bucket
	m.RequestDuration.WithLabelValues("GET", "/slow", "200").Observe(5.0)    // 5.0 bucket

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range metricFamilies {
		if mf.GetName() == "http_request_duration_seconds" {
			// Verify we have 3 different label sets
			if len(mf.GetMetric()) != 3 {
				t.Errorf("Expected 3 histogram series, got %d", len(mf.GetMetric()))
			}

			// Each should have sample count of 1
			for _, m := range mf.GetMetric() {
				histogram := m.GetHistogram()
				if histogram.GetSampleCount() != 1 {
					t.Errorf("Expected sample count to be 1, got %d", histogram.GetSampleCount())
				}
			}
		}
	}
}

func TestMetrics_RecordRateLimitHit(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record rate limit hits
	m.RateLimitHits.WithLabelValues("/api/test").Inc()
	m.RateLimitHits.WithLabelValues("/api/test").Inc()

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Find and verify counter
	var found bool
	var counterValue float64
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_rate_limit_hits_total" {
			found = true
			if len(mf.GetMetric()) == 0 {
				t.Fatal("Expected at least one metric")
			}

			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				if hasLabels(labels, map[string]string{"path": "/api/test"}) {
					counterValue = m.GetCounter().GetValue()
					break
				}
			}
		}
	}

	if !found {
		t.Error("Expected http_rate_limit_hits_total metric to be present")
	}

	if counterValue != 2.0 {
		t.Errorf("Expected counter value to be 2, got %f", counterValue)
	}
}

func TestMetrics_ConcurrentRecording(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	var wg sync.WaitGroup
	concurrency := 100
	incrementsPerGoroutine := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				m.RequestsTotal.WithLabelValues("GET", "/concurrent", "200").Inc()
			}
		}()
	}

	wg.Wait()

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Verify counter value is exactly concurrency * incrementsPerGoroutine
	expectedValue := float64(concurrency * incrementsPerGoroutine)
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_requests_total" {
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				if hasLabels(labels, map[string]string{"method": "GET", "path": "/concurrent", "status": "200"}) {
					actualValue := m.GetCounter().GetValue()
					if actualValue != expectedValue {
						t.Errorf("Expected counter value to be %f, got %f (possible race condition)",
							expectedValue, actualValue)
					}
					return
				}
			}
		}
	}

	t.Error("Expected to find concurrent metric")
}

func TestMetrics_VerifyAllLabels(t *testing.T) {
	m := NewMetrics()
	registry := prometheus.NewRegistry()
	m.MustRegister(registry)

	// Record with specific labels
	m.RequestsTotal.WithLabelValues("POST", "/api/users", "201").Inc()

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range metricFamilies {
		if mf.GetName() == "http_requests_total" {
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()

				// Verify all expected labels are present
				expectedLabels := map[string]string{
					"method": "POST",
					"path":   "/api/users",
					"status": "201",
				}

				if !hasLabels(labels, expectedLabels) {
					t.Error("Labels do not match expected values")
				}

				// Verify exact label count (no extra labels)
				if len(labels) != len(expectedLabels) {
					t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
				}
			}
		}
	}
}

// hasLabels checks if the metric labels contain all expected key-value pairs
func hasLabels(labels []*dto.LabelPair, expected map[string]string) bool {
	labelMap := make(map[string]string)
	for _, label := range labels {
		labelMap[label.GetName()] = label.GetValue()
	}

	for key, value := range expected {
		if labelMap[key] != value {
			return false
		}
	}

	return true
}
