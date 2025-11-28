package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/fermilabs/fermi-api-gateway/internal/database"
)

// ContinuumHandler handles Continuum API endpoints that query TimescaleDB directly
type ContinuumHandler struct {
	repo   *database.Repository
	logger *zap.Logger
}

// NewContinuumHandler creates a new Continuum handler
func NewContinuumHandler(repo *database.Repository, logger *zap.Logger) *ContinuumHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ContinuumHandler{
		repo:   repo,
		logger: logger,
	}
}

// HandleGetTickByNumber handles GET /api/v1/continuum/tick/{tickNumber}
// Returns tick data along with VDF proof
func (h *ContinuumHandler) HandleGetTickByNumber() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract tick number from URL using Chi router
		tickNumberStr := chi.URLParam(r, "tickNumber")

		if tickNumberStr == "" {
			h.writeError(w, "missing tick number in path", http.StatusBadRequest)
			return
		}

		tickNumber, err := strconv.ParseUint(tickNumberStr, 10, 64)
		if err != nil {
			h.writeError(w, "invalid tick number: must be a positive integer", http.StatusBadRequest)
			return
		}

		// Check if repository is available
		if h.repo == nil {
			h.writeError(w, "database not configured", http.StatusServiceUnavailable)
			return
		}

		ctx := r.Context()
		ctx, cancel := withTimeout(ctx, 10*time.Second)
		defer cancel()

		tick, err := h.repo.GetTickByNumber(ctx, tickNumber)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				h.writeError(w, fmt.Sprintf("tick %d not found", tickNumber), http.StatusNotFound)
				return
			}
			h.logger.Error("Failed to get tick", zap.Uint64("tick_number", tickNumber), zap.Error(err))
			h.writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600") // Ticks are immutable, cache for 1 hour
		json.NewEncoder(w).Encode(tick)
	}
}

// HandleGetRecentTicks handles GET /api/v1/continuum/tick/recent?limit=N
// Returns the most recent ticks in descending order by tick number
func (h *ContinuumHandler) HandleGetRecentTicks() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse limit parameter
		limitStr := r.URL.Query().Get("limit")
		limit := 100 // default
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit < 1 {
				h.writeError(w, "invalid limit: must be a positive integer", http.StatusBadRequest)
				return
			}
			if parsedLimit > 1000 {
				parsedLimit = 1000 // cap at 1000
			}
			limit = parsedLimit
		}

		// Check if repository is available
		if h.repo == nil {
			h.writeError(w, "database not configured", http.StatusServiceUnavailable)
			return
		}

		ctx := r.Context()
		ctx, cancel := withTimeout(ctx, 10*time.Second)
		defer cancel()

		ticks, err := h.repo.GetRecentTicks(ctx, limit)
		if err != nil {
			h.logger.Error("Failed to get recent ticks", zap.Int("limit", limit), zap.Error(err))
			h.writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // Recent data shouldn't be cached
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ticks": ticks,
			"count": len(ticks),
		})
	}
}

// HandleGetRecentTransactions handles GET /api/v1/continuum/txn/recent?limit=N
// Returns the most recent transactions
func (h *ContinuumHandler) HandleGetRecentTransactions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse limit parameter
		limitStr := r.URL.Query().Get("limit")
		limit := 50 // default
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit < 1 {
				h.writeError(w, "invalid limit: must be a positive integer", http.StatusBadRequest)
				return
			}
			if parsedLimit > 1000 {
				parsedLimit = 1000 // cap at 1000
			}
			limit = parsedLimit
		}

		// Check if repository is available
		if h.repo == nil {
			h.writeError(w, "database not configured", http.StatusServiceUnavailable)
			return
		}

		ctx := r.Context()
		ctx, cancel := withTimeout(ctx, 10*time.Second)
		defer cancel()

		transactions, err := h.repo.GetRecentTransactions(ctx, limit)
		if err != nil {
			h.logger.Error("Failed to get recent transactions", zap.Int("limit", limit), zap.Error(err))
			h.writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // Recent data shouldn't be cached
		json.NewEncoder(w).Encode(map[string]interface{}{
			"transactions": transactions,
			"count":        len(transactions),
		})
	}
}

// HandleGetTransactionByID handles GET /api/v1/continuum/txn/{txnId}
// Returns a specific transaction by its tx_id
func (h *ContinuumHandler) HandleGetTransactionByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract txn ID from URL using Chi router
		txnID := chi.URLParam(r, "txnId")

		if txnID == "" {
			h.writeError(w, "missing transaction ID in path", http.StatusBadRequest)
			return
		}

		// Basic validation of txnID
		if len(txnID) > 256 {
			h.writeError(w, "transaction ID too long", http.StatusBadRequest)
			return
		}

		// Check if repository is available
		if h.repo == nil {
			h.writeError(w, "database not configured", http.StatusServiceUnavailable)
			return
		}

		ctx := r.Context()
		ctx, cancel := withTimeout(ctx, 10*time.Second)
		defer cancel()

		// Try to get by tx_id first
		tx, err := h.repo.GetTransactionByTxID(ctx, txnID)
		if err != nil {
			// If not found by tx_id, try by tx_hash (for flexibility)
			if strings.Contains(err.Error(), "not found") {
				tx, err = h.repo.GetTransaction(ctx, txnID)
			}
		}

		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				h.writeError(w, fmt.Sprintf("transaction %s not found", txnID), http.StatusNotFound)
				return
			}
			h.logger.Error("Failed to get transaction", zap.String("txn_id", txnID), zap.Error(err))
			h.writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600") // Transactions are immutable, cache for 1 hour
		json.NewEncoder(w).Encode(tx)
	}
}

// writeError writes a JSON error response
func (h *ContinuumHandler) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// withTimeout creates a context with timeout if not already set
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}
