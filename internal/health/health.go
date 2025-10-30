package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// Status represents the health status of the service
type Status struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// Handler returns an HTTP handler for health checks
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := Status{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	}
}

// ReadyHandler returns an HTTP handler for readiness checks
// This is useful in Kubernetes/container environments
func ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// In the future, we can add checks for:
		// - Redis connection
		// - Backend service availability
		// For now, if the server is running, it's ready
		status := Status{
			Status:    "ready",
			Timestamp: time.Now(),
			Version:   "1.0.0",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	}
}
