package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
)

// RESTStatusResponse represents the REST /status endpoint response
type RESTStatusResponse struct {
	ChainHeight       uint64 `json:"chain_height"`
	TotalTransactions uint64 `json:"total_transactions"` // This is last 60 seconds
	LatestTick        uint64 `json:"latest_tick"`
	Status            string `json:"status"`
	Last60Seconds     struct {
		TickCount           uint64  `json:"tick_count"`
		MeanTickTimeMicros  float64 `json:"mean_tick_time_micros"`
		TicksPerSecond      float64 `json:"ticks_per_second"`
	} `json:"last_60_seconds"`
}

// UnifiedStatusResponse represents the merged status from REST + gRPC
type UnifiedStatusResponse struct {
	ChainHeight       uint64  `json:"chain_height"`        // From REST
	TotalTransactions uint64  `json:"total_transactions"`  // From gRPC (lifetime)
	Status            string  `json:"status"`              // From REST ("running")
	UptimeSeconds     uint64  `json:"uptime_seconds"`      // From gRPC
	TxnPerSecond      float64 `json:"txn_per_second"`      // Calculated: REST.total_transactions / 60
	TicksPerSecond    float64 `json:"ticks_per_second"`    // From REST.last_60_seconds
	AverageTickTime   float64 `json:"average_tick_time"`   // From REST.last_60_seconds (microseconds)
}

// HandleUnifiedStatus creates a unified status endpoint that merges REST status and gRPC GetStatus
func (p *GRPCProxy) HandleUnifiedStatus(restURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		// Fetch gRPC GetStatus (optional - don't fail if unavailable)
		var grpcResp *pb.GetStatusResponse
		var grpcErr error
		grpcResp, grpcErr = p.client.GetStatus(ctx, &pb.GetStatusRequest{})
		if grpcErr != nil {
			// Log but don't fail - we'll use REST data only
			grpcResp = nil
		}

		// Fetch REST /status
		restStatusURL := fmt.Sprintf("%s/status", restURL)
		httpReq, err := http.NewRequestWithContext(ctx, "GET", restStatusURL, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to create REST request: %v"}`, err), http.StatusInternalServerError)
			return
		}

		httpResp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			// If REST also fails, return error
			if grpcErr != nil {
				http.Error(w, fmt.Sprintf(`{"error":"both backends unavailable: gRPC: %v, REST: %v"}`, grpcErr, err), http.StatusServiceUnavailable)
			} else {
				http.Error(w, fmt.Sprintf(`{"error":"failed to fetch REST status: %v"}`, err), http.StatusServiceUnavailable)
			}
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(httpResp.Body)
			if grpcErr != nil {
				http.Error(w, fmt.Sprintf(`{"error":"REST returned %d: %s, gRPC unavailable: %v"}`, httpResp.StatusCode, string(body), grpcErr), http.StatusServiceUnavailable)
			} else {
				http.Error(w, fmt.Sprintf(`{"error":"REST returned %d: %s"}`, httpResp.StatusCode, string(body)), http.StatusServiceUnavailable)
			}
			return
		}

		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to read REST response: %v"}`, err), http.StatusInternalServerError)
			return
		}

		var restResp RESTStatusResponse
		if err := json.Unmarshal(body, &restResp); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to parse REST response: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Calculate txn_per_second: REST total_transactions is for last 60 seconds
		txnPerSecond := float64(restResp.TotalTransactions) / 60.0

		// Build unified response with fallback values if gRPC unavailable
		unified := UnifiedStatusResponse{
			ChainHeight:       restResp.ChainHeight,
			TotalTransactions: restResp.TotalTransactions, // Fallback to REST value if gRPC unavailable
			Status:            restResp.Status,
			UptimeSeconds:     0, // Only available from gRPC
			TxnPerSecond:      txnPerSecond,
			TicksPerSecond:    restResp.Last60Seconds.TicksPerSecond,
			AverageTickTime:   restResp.Last60Seconds.MeanTickTimeMicros,
		}

		// If gRPC available, use its values
		if grpcResp != nil {
			unified.TotalTransactions = grpcResp.TotalTransactions // Lifetime total from gRPC
			unified.UptimeSeconds = grpcResp.UptimeSeconds
		}

		// Return merged JSON response (include warning if gRPC unavailable)
		if grpcErr != nil {
			// Add a note that gRPC data is partial
			unifiedJson, _ := json.Marshal(unified)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"partial","warnings":["gRPC backend unavailable, using REST data only"],"data":%s}`, string(unifiedJson))
			return
		}

		if err := json.NewEncoder(w).Encode(unified); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to encode response: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}
}
