package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fermilabs/fermi-api-gateway/internal/metrics"
	"go.uber.org/zap"
)

// TestResponseWriterImplementsFlusher tests that responseWriter implements http.Flusher
func TestResponseWriterImplementsFlusher(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the wrapped writer implements http.Flusher
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("responseWriter does not implement http.Flusher")
			return
		}

		// Call Flush to ensure it doesn't panic
		flusher.Flush()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	logger, _ := zap.NewDevelopment()
	middleware := Logging(logger)(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// TestMetricsResponseWriterImplementsFlusher tests that metricsResponseWriter implements http.Flusher
func TestMetricsResponseWriterImplementsFlusher(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the wrapped writer implements http.Flusher
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("metricsResponseWriter does not implement http.Flusher")
			return
		}

		// Call Flush to ensure it doesn't panic
		flusher.Flush()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	m := metrics.NewMetrics()
	middleware := Metrics(m)(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// TestBothMiddlewaresFlusher tests that both middleware stacked together still support Flusher
func TestBothMiddlewaresFlusher(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the wrapped writer implements http.Flusher
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("stacked middleware does not implement http.Flusher")
			return
		}

		// Simulate SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for i := 0; i < 3; i++ {
			w.Write([]byte("data: test\n\n"))
			flusher.Flush()
		}
	})

	logger, _ := zap.NewDevelopment()
	m := metrics.NewMetrics()

	// Stack both middleware
	middleware := Logging(logger)(Metrics(m)(handler))

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body != "data: test\n\ndata: test\n\ndata: test\n\n" {
		t.Errorf("unexpected body: %s", body)
	}
}
