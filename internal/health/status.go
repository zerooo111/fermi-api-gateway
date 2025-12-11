package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// HealthChecker is an interface for checking service health
type HealthChecker interface {
	Health(ctx context.Context) error
}

// StatusDependencies holds the dependencies needed for status checks
type StatusDependencies struct {
	RollupHealthURL    string // Full URL to rollup health endpoint (e.g., http://host:port/status)
	ContinuumHealthURL string // Full URL to continuum health endpoint (e.g., http://host:port/api/v1/health)
	DB                 HealthChecker
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

// GatewayStats holds runtime statistics for the gateway
type GatewayStats struct {
	Uptime     string  `json:"uptime"`
	UptimeSec  int64   `json:"uptime_seconds"`
	Goroutines int     `json:"goroutines"`
	MemAllocMB float64 `json:"mem_alloc_mb"`
	MemSysMB   float64 `json:"mem_sys_mb"`
	NumGC      uint32  `json:"num_gc"`
	GoVersion  string  `json:"go_version"`
}

// SystemStatus represents the overall system health status
type SystemStatus struct {
	Status    string       `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
	Gateway   GatewayStats `json:"gateway"`
	Services  Services     `json:"services"`
}

// StatusHandler returns an HTTP handler for comprehensive system status checks
func StatusHandler(deps *StatusDependencies) http.HandlerFunc {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	startTime := time.Now()

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
			status := checkHTTPService(ctx, client, deps.RollupHealthURL)
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
			status := checkHTTPService(ctx, client, deps.ContinuumHealthURL)
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

		// Collect gateway stats
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		uptime := time.Since(startTime)

		resp := SystemStatus{
			Status:    overallStatus,
			Timestamp: time.Now(),
			Gateway: GatewayStats{
				Uptime:     formatDuration(uptime),
				UptimeSec:  int64(uptime.Seconds()),
				Goroutines: runtime.NumGoroutine(),
				MemAllocMB: float64(memStats.Alloc) / 1024 / 1024,
				MemSysMB:   float64(memStats.Sys) / 1024 / 1024,
				NumGC:      memStats.NumGC,
				GoVersion:  runtime.Version(),
			},
			Services: services,
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

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
