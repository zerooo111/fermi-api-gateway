package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto"
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

		// Fetch gRPC GetStatus
		grpcResp, err := p.client.GetStatus(ctx, &pb.GetStatusRequest{})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to fetch gRPC status: %v"}`, err), http.StatusInternalServerError)
			return
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
			http.Error(w, fmt.Sprintf(`{"error":"failed to fetch REST status: %v"}`, err), http.StatusInternalServerError)
			return
		}
		defer httpResp.Body.Close()

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

		// Build unified response
		unified := UnifiedStatusResponse{
			ChainHeight:       restResp.ChainHeight,
			TotalTransactions: grpcResp.TotalTransactions, // Lifetime total from gRPC
			Status:            restResp.Status,
			UptimeSeconds:     grpcResp.UptimeSeconds,
			TxnPerSecond:      txnPerSecond,
			TicksPerSecond:    restResp.Last60Seconds.TicksPerSecond,
			AverageTickTime:   restResp.Last60Seconds.MeanTickTimeMicros,
		}

		// Return merged JSON response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(unified); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to encode response: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}
}
