package proxy

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/fermilabs/fermi-api-gateway/internal/database"
)

// roundTo2Decimals rounds a float64 to 2 decimal places
func roundTo2Decimals(val float64) float64 {
	return math.Round(val*100) / 100
}

// CandlesHandler handles market candles endpoint requests
type CandlesHandler struct {
	repository *database.Repository
	logger     *zap.Logger
}

// NewCandlesHandler creates a new candles handler
func NewCandlesHandler(repository *database.Repository, logger *zap.Logger) *CandlesHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CandlesHandler{
		repository: repository,
		logger:     logger,
	}
}

// GetMarketCandles handles GET /api/v1/rollup/markets/:marketId/candles
func (h *CandlesHandler) GetMarketCandles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Extract marketId from URL path parameter
		marketID := chi.URLParam(r, "marketId")
		if marketID == "" {
			h.writeErrorResponse(w, http.StatusBadRequest, "Market ID is required")
			return
		}

		// Validate timeframe
		tf := r.URL.Query().Get("tf")
		if tf == "" {
			tf = "1h" // default
		}

		allowedTimeframes := map[string]bool{
			"1m":  true,
			"5m":  true,
			"15m": true,
			"1h":  true,
			"4h":  true,
			"1d":  true,
		}

		if !allowedTimeframes[tf] {
			h.writeErrorResponse(w, http.StatusBadRequest, "Invalid timeframe. Allowed values: 1m, 5m, 15m, 1h, 4h, 1d")
			return
		}

		// Parse date range
		// Support for incremental updates: use 'since' parameter to fetch only new candles
		// This allows frontend to fetch historical data once, then poll for updates only
		now := time.Now().UTC()
		fromStr := r.URL.Query().Get("from")
		sinceStr := r.URL.Query().Get("since") // New parameter for incremental updates
		toStr := r.URL.Query().Get("to")

		var from, to time.Time
		var err error

		// If 'since' is provided, use it instead of 'from' for incremental updates
		// 'since' should be a timestamp in milliseconds (Unix epoch)
		if sinceStr != "" {
			sinceMs, err := strconv.ParseInt(sinceStr, 10, 64)
			if err != nil {
				h.writeErrorResponse(w, http.StatusBadRequest, "Invalid 'since' format. Use Unix timestamp in milliseconds (e.g., 1704067200000)")
				return
			}
			// Convert milliseconds to time.Time, add 1ms to exclude the last candle (get only new ones)
			from = time.Unix(0, sinceMs*int64(time.Millisecond)).UTC().Add(1 * time.Millisecond)
		} else if fromStr == "" {
			from = now.Add(-24 * time.Hour) // default: 24 hours ago
		} else {
			from, err = time.Parse(time.RFC3339, fromStr)
			if err != nil {
				h.writeErrorResponse(w, http.StatusBadRequest, "Invalid 'from' date format. Use RFC3339 format (e.g., 2023-01-01T00:00:00Z)")
				return
			}
		}

		if toStr == "" {
			to = now // default: current time
		} else {
			to, err = time.Parse(time.RFC3339, toStr)
			if err != nil {
				h.writeErrorResponse(w, http.StatusBadRequest, "Invalid 'to' date format. Use RFC3339 format (e.g., 2023-01-01T23:59:59Z)")
				return
			}
		}

		// Validate date range
		if from.After(to) {
			h.writeErrorResponse(w, http.StatusBadRequest, "'from' date must be before 'to' date")
			return
		}

		// Limit query range to 30 days
		maxRange := 30 * 24 * time.Hour
		if to.Sub(from) > maxRange {
			h.writeErrorResponse(w, http.StatusBadRequest, "Date range cannot exceed 30 days")
			return
		}

		// Parse limit parameter (Binance-style: default 500, max 1000)
		limit := 500 // default
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit < 1 || parsedLimit > 1000 {
				h.writeErrorResponse(w, http.StatusBadRequest, "Invalid limit (must be 1-1000)")
				return
			}
			limit = parsedLimit
		}

		// Query database
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if h.repository == nil {
			h.writeErrorResponse(w, http.StatusInternalServerError, "Database not available")
			return
		}

		candles, err := h.repository.GetMarketCandles(ctx, marketID, tf, from, to, limit)
		if err != nil {
			h.logger.Warn("Failed to get market candles", zap.Error(err))
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get market candles")
			return
		}

		// Binance-style: Return array directly (no wrapping object) for efficiency
		// Format: [[timestamp_ms, open, high, low, close], ...]
		// Using compact array format reduces payload size by ~40% vs objects
		// Prices are divided by 1M and rounded to 2 decimals to reduce response size (~20-33% smaller)
		// This converts from USDC micro-units (e.g., 163885020) to USDC (e.g., 163.89)
		candleArrays := make([][]interface{}, len(candles))
		for i, candle := range candles {
			// Convert timestamp to milliseconds (Unix epoch) for compactness
			// Binance uses milliseconds since epoch (not RFC3339 strings)
			timestampMs := candle.Timestamp.UnixMilli()
			// Divide prices by 1M and round to 2 decimals for smaller response size
			// This reduces JSON payload by ~20-33% and improves network transfer time
			candleArrays[i] = []interface{}{
				timestampMs,                                    // Open time (ms)
				roundTo2Decimals(candle.Open / 1000000.0),   // Open price (USDC)
				roundTo2Decimals(candle.High / 1000000.0),   // High price (USDC)
				roundTo2Decimals(candle.Low / 1000000.0),    // Low price (USDC)
				roundTo2Decimals(candle.Close / 1000000.0),  // Close price (USDC)
			}
		}

		// Set response headers (Binance-style optimizations)
		w.Header().Set("Content-Type", "application/json")
		if sinceStr == "" {
			// Only cache full historical data, not incremental updates
			w.Header().Set("Cache-Control", "public, max-age=5")
		} else {
			// Don't cache incremental updates
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("X-Data-Source", "database")
		
		// Add header with latest candle timestamp for frontend to use in next 'since' request
		if len(candles) > 0 {
			lastCandleTimestamp := candles[len(candles)-1].Timestamp.UnixMilli()
			w.Header().Set("X-Last-Candle-Timestamp", strconv.FormatInt(lastCandleTimestamp, 10))
		}
		
		// Note: gzip compression should be handled by middleware or reverse proxy
		// Setting it here without actual compression would break the response

		// Use compact JSON encoding (no indentation) for minimal payload size
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false) // Don't escape HTML characters for better performance
		if err := enc.Encode(candleArrays); err != nil {
			// Encoding errors are rare and usually indicate connection issues
			// Log silently as response may have already been partially written
		}
	}
}

// writeErrorResponse writes an error response in the standard format
func (h *CandlesHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"data":       nil,
		"statusCode": statusCode,
		"error":      message,
	}
	json.NewEncoder(w).Encode(response)
}

