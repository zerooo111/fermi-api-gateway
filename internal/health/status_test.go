package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockHealthChecker implements health checking for tests
type MockHealthChecker struct {
	healthy bool
	latency time.Duration
}

func (m *MockHealthChecker) Health(ctx context.Context) error {
	if !m.healthy {
		return context.DeadlineExceeded
	}
	time.Sleep(m.latency)
	return nil
}

func TestStatusHandler_AllServicesHealthy(t *testing.T) {
	// Create mock backends
	rollupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer rollupServer.Close()

	continuumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer continuumServer.Close()

	mockDB := &MockHealthChecker{healthy: true, latency: 5 * time.Millisecond}

	deps := &StatusDependencies{
		RollupURL:        rollupServer.URL,
		ContinuumRestURL: continuumServer.URL,
		DB:               mockDB,
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", resp.Status)
	}

	// Check rollup service
	if resp.Services.Rollup.Status != "healthy" {
		t.Errorf("expected rollup status 'healthy', got '%s'", resp.Services.Rollup.Status)
	}
	if resp.Services.Rollup.LatencyMs <= 0 {
		t.Error("expected positive latency for rollup")
	}

	// Check continuum service
	if resp.Services.Continuum.Status != "healthy" {
		t.Errorf("expected continuum status 'healthy', got '%s'", resp.Services.Continuum.Status)
	}
	if resp.Services.Continuum.LatencyMs <= 0 {
		t.Error("expected positive latency for continuum")
	}

	// Check timescale db
	if resp.Services.TimescaleDB.Status != "healthy" {
		t.Errorf("expected timescale_db status 'healthy', got '%s'", resp.Services.TimescaleDB.Status)
	}
	if resp.Services.TimescaleDB.LatencyMs <= 0 {
		t.Error("expected positive latency for timescale_db")
	}
}

func TestStatusHandler_RollupUnhealthy(t *testing.T) {
	// Rollup returns error
	rollupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer rollupServer.Close()

	continuumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer continuumServer.Close()

	mockDB := &MockHealthChecker{healthy: true, latency: 1 * time.Millisecond}

	deps := &StatusDependencies{
		RollupURL:        rollupServer.URL,
		ContinuumRestURL: continuumServer.URL,
		DB:               mockDB,
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should return 503 when any service is unhealthy
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", resp.Status)
	}

	if resp.Services.Rollup.Status != "unhealthy" {
		t.Errorf("expected rollup status 'unhealthy', got '%s'", resp.Services.Rollup.Status)
	}

	if resp.Services.Rollup.Error == "" {
		t.Error("expected error message for unhealthy rollup")
	}
}

func TestStatusHandler_ContinuumUnhealthy(t *testing.T) {
	rollupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer rollupServer.Close()

	// Continuum returns error
	continuumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer continuumServer.Close()

	mockDB := &MockHealthChecker{healthy: true, latency: 1 * time.Millisecond}

	deps := &StatusDependencies{
		RollupURL:        rollupServer.URL,
		ContinuumRestURL: continuumServer.URL,
		DB:               mockDB,
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Services.Continuum.Status != "unhealthy" {
		t.Errorf("expected continuum status 'unhealthy', got '%s'", resp.Services.Continuum.Status)
	}
}

func TestStatusHandler_TimescaleDBUnhealthy(t *testing.T) {
	rollupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer rollupServer.Close()

	continuumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer continuumServer.Close()

	// DB is unhealthy
	mockDB := &MockHealthChecker{healthy: false}

	deps := &StatusDependencies{
		RollupURL:        rollupServer.URL,
		ContinuumRestURL: continuumServer.URL,
		DB:               mockDB,
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Services.TimescaleDB.Status != "unhealthy" {
		t.Errorf("expected timescale_db status 'unhealthy', got '%s'", resp.Services.TimescaleDB.Status)
	}
}

func TestStatusHandler_DBNotConfigured(t *testing.T) {
	rollupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer rollupServer.Close()

	continuumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer continuumServer.Close()

	// DB is nil (not configured)
	deps := &StatusDependencies{
		RollupURL:        rollupServer.URL,
		ContinuumRestURL: continuumServer.URL,
		DB:               nil,
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should still return 503 because DB is required
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Services.TimescaleDB.Status != "unhealthy" {
		t.Errorf("expected timescale_db status 'unhealthy', got '%s'", resp.Services.TimescaleDB.Status)
	}

	if resp.Services.TimescaleDB.Error != "not configured" {
		t.Errorf("expected error 'not configured', got '%s'", resp.Services.TimescaleDB.Error)
	}
}

func TestStatusHandler_RollupUnreachable(t *testing.T) {
	// Use invalid URL that won't connect
	deps := &StatusDependencies{
		RollupURL:        "http://localhost:99999",
		ContinuumRestURL: "http://localhost:99998",
		DB:               nil,
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Services.Rollup.Status != "unhealthy" {
		t.Errorf("expected rollup status 'unhealthy', got '%s'", resp.Services.Rollup.Status)
	}
}

func TestStatusHandler_ResponseHeaders(t *testing.T) {
	rollupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer rollupServer.Close()

	deps := &StatusDependencies{
		RollupURL:        rollupServer.URL,
		ContinuumRestURL: rollupServer.URL,
		DB:               &MockHealthChecker{healthy: true},
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	cacheControl := rr.Header().Get("Cache-Control")
	if cacheControl != "no-cache, no-store, must-revalidate" {
		t.Errorf("expected no-cache header, got '%s'", cacheControl)
	}
}

func TestStatusHandler_ResponseContainsTimestamp(t *testing.T) {
	rollupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer rollupServer.Close()

	deps := &StatusDependencies{
		RollupURL:        rollupServer.URL,
		ContinuumRestURL: rollupServer.URL,
		DB:               &MockHealthChecker{healthy: true},
	}

	handler := StatusHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	var resp SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	// Timestamp should be recent (within last second)
	if time.Since(resp.Timestamp) > time.Second {
		t.Error("timestamp should be recent")
	}
}
