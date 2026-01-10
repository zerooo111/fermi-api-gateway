package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
)

// httpClientWithTLSSkip is an HTTP client that skips TLS verification
// for internal services with self-signed certificates
var httpClientWithTLSSkip = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

// ExplorerHealthResponse represents the Explorer API /health endpoint response
type ExplorerHealthResponse struct {
	Status     string `json:"status"`
	DBHealthy  bool   `json:"db_healthy"`
	LatestTick uint64 `json:"latest_tick"`
}

// ExplorerStatsResponse represents the Explorer API /api/v1/stats endpoint response
type ExplorerStatsResponse struct {
	TicksIndexed        uint64  `json:"ticks_indexed"`
	TransactionsIndexed uint64  `json:"transactions_indexed"`
	EmptyTicksSkipped   uint64  `json:"empty_ticks_skipped"`
	LatestTickNumber    uint64  `json:"latest_tick_number"`
	MemoryTicksCount    uint64  `json:"memory_ticks_count"`
	MemoryTxsCount      uint64  `json:"memory_txs_count"`
	TickHitRate         float64 `json:"tick_hit_rate"`
	TxHitRate           float64 `json:"tx_hit_rate"`
	TicksWithTxRatio    float64 `json:"ticks_with_tx_ratio"`
	DBSizeMB            uint64  `json:"db_size_mb"`
}

// UnifiedStatusResponse represents the merged status from Explorer API + gRPC
type UnifiedStatusResponse struct {
	Status              string  `json:"status"`
	DBHealthy           bool    `json:"db_healthy"`
	LatestTick          uint64  `json:"latest_tick"`
	TicksIndexed        uint64  `json:"ticks_indexed"`
	TransactionsIndexed uint64  `json:"transactions_indexed"`
	TotalTransactions   uint64  `json:"total_transactions"` // From gRPC (lifetime)
	UptimeSeconds       uint64  `json:"uptime_seconds"`     // From gRPC
	DBSizeMB            uint64  `json:"db_size_mb"`
}

// HandleUnifiedStatus creates a unified status endpoint that merges Explorer API health/stats and gRPC GetStatus
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

		// Fetch Explorer API /health
		healthURL := fmt.Sprintf("%s/health", restURL)
		healthResp, err := fetchJSON[ExplorerHealthResponse](ctx, healthURL)
		if err != nil {
			if grpcErr != nil {
				http.Error(w, fmt.Sprintf(`{"error":"both backends unavailable: gRPC: %v, REST: %v"}`, grpcErr, err), http.StatusServiceUnavailable)
			} else {
				http.Error(w, fmt.Sprintf(`{"error":"failed to fetch health: %v"}`, err), http.StatusServiceUnavailable)
			}
			return
		}

		// Fetch Explorer API /api/v1/stats
		statsURL := fmt.Sprintf("%s/api/v1/stats", restURL)
		statsResp, err := fetchJSON[ExplorerStatsResponse](ctx, statsURL)
		if err != nil {
			// Stats is optional, continue with just health data
			statsResp = &ExplorerStatsResponse{}
		}

		// Build unified response
		unified := UnifiedStatusResponse{
			Status:              healthResp.Status,
			DBHealthy:           healthResp.DBHealthy,
			LatestTick:          healthResp.LatestTick,
			TicksIndexed:        statsResp.TicksIndexed,
			TransactionsIndexed: statsResp.TransactionsIndexed,
			TotalTransactions:   statsResp.TransactionsIndexed, // Fallback
			UptimeSeconds:       0,
			DBSizeMB:            statsResp.DBSizeMB,
		}

		// If gRPC available, use its values
		if grpcResp != nil {
			unified.TotalTransactions = grpcResp.TotalTransactions
			unified.UptimeSeconds = grpcResp.UptimeSeconds
		}

		// Return merged JSON response (include warning if gRPC unavailable)
		if grpcErr != nil {
			unifiedJson, _ := json.Marshal(unified)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"partial","warnings":["gRPC backend unavailable, using Explorer API data only"],"data":%s}`, string(unifiedJson))
			return
		}

		if err := json.NewEncoder(w).Encode(unified); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to encode response: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}
}

// fetchJSON fetches a URL and unmarshals the JSON response
func fetchJSON[T any](ctx context.Context, url string) (*T, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpResp, err := httpClientWithTLSSkip.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("returned %d: %s", httpResp.StatusCode, string(body))
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
