package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/stream"
	"go.uber.org/zap"
)

// SSEHandler handles Server-Sent Events streaming for live tick updates
type SSEHandler struct {
	ringBuffer *stream.RingBuffer
	cfg        *config.StreamConfig
	logger     *zap.Logger
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(ringBuffer *stream.RingBuffer, cfg *config.StreamConfig, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		ringBuffer: ringBuffer,
		cfg:        cfg,
		logger:     logger,
	}
}

// StreamSnapshot represents the JSON structure sent to clients
type StreamSnapshot struct {
	Ticks        interface{} `json:"ticks"`
	Transactions interface{} `json:"transactions"`
	Timestamp    string      `json:"timestamp"`
}

// HandleLiveStream handles SSE connections for live tick/transaction streaming
func (h *SSEHandler) HandleLiveStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // TODO: Use CORS middleware instead
	w.Header().Set("X-Accel-Buffering", "no")         // Disable nginx buffering

	// Get flusher for SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("Streaming not supported by response writer")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create context that cancels when client disconnects
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Send initial snapshot
	ticks, transactions := h.ringBuffer.GetSnapshot()
	snapshot := StreamSnapshot{
		Ticks:        ticks,
		Transactions: transactions,
		Timestamp:    time.Now().Format(time.RFC3339Nano),
	}

	if err := h.sendEvent(w, "snapshot", snapshot); err != nil {
		h.logger.Error("Failed to send initial snapshot", zap.Error(err))
		return
	}
	flusher.Flush()

	h.logger.Info("New SSE client connected",
		zap.String("remote_addr", r.RemoteAddr),
		zap.Int("initial_ticks", len(ticks)),
		zap.Int("initial_txs", len(transactions)),
	)

	// Subscribe to ring buffer updates
	updateChan := h.ringBuffer.Subscribe()
	defer h.ringBuffer.Unsubscribe(updateChan)

	// Calculate update interval based on FPS
	updateInterval := time.Duration(1000/h.cfg.UpdateFPS) * time.Millisecond
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	// Keep-alive ticker (send comment every 30 seconds)
	keepAliveTicker := time.NewTicker(30 * time.Second)
	defer keepAliveTicker.Stop()

	// Track last tick number to detect changes
	lastTickNumber := uint64(0)
	if len(ticks) > 0 {
		lastTickNumber = ticks[len(ticks)-1].TickNumber
	}

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("SSE client disconnected", zap.String("remote_addr", r.RemoteAddr))
			return

		case <-ticker.C:
			// Get current snapshot
			currentTicks, transactions := h.ringBuffer.GetSnapshot()

			// Check if we have new data by comparing latest tick number
			currentTickNumber := uint64(0)
			if len(currentTicks) > 0 {
				currentTickNumber = currentTicks[len(currentTicks)-1].TickNumber
			}

			// Send update if tick number changed (new data)
			if currentTickNumber != lastTickNumber {
				snapshot := StreamSnapshot{
					Ticks:        currentTicks,
					Transactions: transactions,
					Timestamp:    time.Now().Format(time.RFC3339Nano),
				}

				if err := h.sendEvent(w, "update", snapshot); err != nil {
					h.logger.Error("Failed to send update", zap.Error(err))
					return
				}
				flusher.Flush()

				lastTickNumber = currentTickNumber
			}

		case <-keepAliveTicker.C:
			// Send keep-alive comment
			if _, err := fmt.Fprintf(w, ": keep-alive\n\n"); err != nil {
				h.logger.Error("Failed to send keep-alive", zap.Error(err))
				return
			}
			flusher.Flush()

		case <-updateChan:
			// Ring buffer updated, will be picked up on next ticker interval
			// This ensures we rate-limit to configured FPS
			continue
		}
	}
}

// sendEvent sends an SSE event with the given name and data
func (h *SSEHandler) sendEvent(w http.ResponseWriter, eventName string, data interface{}) error {
	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write SSE event
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, jsonData); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}
