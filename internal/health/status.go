package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthChecker is an interface for checking service health
type HealthChecker interface {
	Health(ctx context.Context) error
}

// StatusDependencies holds the dependencies needed for status checks
type StatusDependencies struct {
	RollupURL        string
	ContinuumRestURL string
	DB               HealthChecker
}

// ServiceStatus represents the health status of a single service
type ServiceStatus struct {
	Status    string  `json:"status"`
	LatencyMs float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

// Services holds the status of all backend services
type Services struct {
	Rollup      ServiceStatus `json:"rollup"`
	Continuum   ServiceStatus `json:"continuum"`
	TimescaleDB ServiceStatus `json:"timescale_db"`
}

// SystemStatus represents the overall system health status
type SystemStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Services  Services  `json:"services"`
}

// StatusHandler returns an HTTP handler for comprehensive system status checks
func StatusHandler(deps *StatusDependencies) http.HandlerFunc {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var mu sync.Mutex

		services := Services{}
		allHealthy := true

		// Check Rollup
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := checkHTTPService(ctx, client, deps.RollupURL+"/health")
			mu.Lock()
			services.Rollup = status
			if status.Status != "healthy" {
				allHealthy = false
			}
			mu.Unlock()
		}()

		// Check Continuum REST
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := checkHTTPService(ctx, client, deps.ContinuumRestURL+"/health")
			mu.Lock()
			services.Continuum = status
			if status.Status != "healthy" {
				allHealthy = false
			}
			mu.Unlock()
		}()

		// Check TimescaleDB
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := checkDatabase(ctx, deps.DB)
			mu.Lock()
			services.TimescaleDB = status
			if status.Status != "healthy" {
				allHealthy = false
			}
			mu.Unlock()
		}()

		wg.Wait()

		overallStatus := "healthy"
		httpStatus := http.StatusOK
		if !allHealthy {
			overallStatus = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}

		resp := SystemStatus{
			Status:    overallStatus,
			Timestamp: time.Now(),
			Services:  services,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(resp)
	}
}

// checkHTTPService checks the health of an HTTP service
func checkHTTPService(ctx context.Context, client *http.Client, url string) ServiceStatus {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ServiceStatus{
			Status:    "unhealthy",
			LatencyMs: float64(time.Since(start).Milliseconds()),
			Error:     err.Error(),
		}
	}

	resp, err := client.Do(req)
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return ServiceStatus{
			Status:    "unhealthy",
			LatencyMs: latencyMs,
			Error:     err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return ServiceStatus{
			Status:    "healthy",
			LatencyMs: latencyMs,
		}
	}

	return ServiceStatus{
		Status:    "unhealthy",
		LatencyMs: latencyMs,
		Error:     "unexpected status code: " + resp.Status,
	}
}

// checkDatabase checks the health of the database
func checkDatabase(ctx context.Context, db HealthChecker) ServiceStatus {
	if db == nil {
		return ServiceStatus{
			Status:    "unhealthy",
			LatencyMs: 0,
			Error:     "not configured",
		}
	}

	start := time.Now()
	err := db.Health(ctx)
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return ServiceStatus{
			Status:    "unhealthy",
			LatencyMs: latencyMs,
			Error:     err.Error(),
		}
	}

	return ServiceStatus{
		Status:    "healthy",
		LatencyMs: latencyMs,
	}
}
