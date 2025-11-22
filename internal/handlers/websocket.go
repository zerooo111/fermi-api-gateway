package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	"github.com/fermilabs/fermi-api-gateway/internal/stream"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now - CORS middleware handles this
		return true
	},
}

// WebSocketHandler handles WebSocket connections for live tick/transaction streaming
type WebSocketHandler struct {
	ringBuffer *stream.RingBuffer
	cfg        *config.StreamConfig
	logger     *zap.Logger

	// Connection management
	clients   map[*websocket.Conn]*wsClient
	clientsMu sync.RWMutex

	// Broadcast channel
	broadcast chan *domain.Tick
}

// wsClient represents a connected WebSocket client
type wsClient struct {
	conn     *websocket.Conn
	send     chan interface{}
	done     chan struct{}
	writeMu  sync.Mutex
	lastPing time.Time
}

// WSMessage represents a WebSocket message structure
type WSMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// TickUpdate represents a single tick update message
type TickUpdate struct {
	Tick *domain.Tick `json:"tick"`
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(ringBuffer *stream.RingBuffer, cfg *config.StreamConfig, logger *zap.Logger) *WebSocketHandler {
	handler := &WebSocketHandler{
		ringBuffer: ringBuffer,
		cfg:        cfg,
		logger:     logger,
		clients:    make(map[*websocket.Conn]*wsClient),
		broadcast:  make(chan *domain.Tick, 100), // Not used anymore but keep for now
	}

	// Subscribe to ring buffer updates and broadcast every 1 second
	go handler.subscribeToUpdates()

	return handler
}

// HandleLiveStream handles WebSocket upgrade and streaming
func (h *WebSocketHandler) HandleLiveStream(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade to WebSocket", zap.Error(err))
		return
	}

	// Create client
	client := &wsClient{
		conn:     conn,
		send:     make(chan interface{}, 10),
		done:     make(chan struct{}),
		lastPing: time.Now(),
	}

	// Register client
	h.clientsMu.Lock()
	h.clients[conn] = client
	h.clientsMu.Unlock()

	h.logger.Info("WebSocket client connected",
		zap.String("remote_addr", r.RemoteAddr),
		zap.Int("total_clients", h.ClientCount()),
	)

	// Send initial snapshot (may be empty if we just started)
	ticks, transactions := h.ringBuffer.GetSnapshot()
	snapshot := StreamSnapshot{
		Ticks:        ticks,
		Transactions: transactions,
		Timestamp:    time.Now().Format(time.RFC3339Nano),
	}

	msg := WSMessage{
		Type:      "snapshot",
		Data:      snapshot,
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}

	if err := h.writeJSON(client, msg); err != nil {
		h.logger.Error("Failed to send initial snapshot", zap.Error(err))
		h.removeClient(conn)
		return
	}

	if len(ticks) == 0 {
		h.logger.Info("Sent empty initial snapshot to client (waiting for first tick)",
			zap.String("remote_addr", r.RemoteAddr),
		)
	} else {
		h.logger.Info("Sent initial snapshot to client",
			zap.String("remote_addr", r.RemoteAddr),
			zap.Int("ticks", len(ticks)),
			zap.Int("transactions", len(transactions)),
		)
	}

	// Start goroutines for this client
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Write pump
	go h.writePump(client, ctx)

	// Read pump (handles pings/pongs and disconnect detection)
	h.readPump(client, ctx, cancel)

	// Client disconnected
	h.removeClient(conn)
	h.logger.Info("WebSocket client disconnected",
		zap.String("remote_addr", r.RemoteAddr),
		zap.Int("remaining_clients", h.ClientCount()),
	)
}

// subscribeToUpdates subscribes to ring buffer updates and broadcasts snapshots every second
func (h *WebSocketHandler) subscribeToUpdates() {
	updateChan := h.ringBuffer.Subscribe()
	defer h.ringBuffer.Unsubscribe(updateChan)

	// Ticker to send updates every 1 second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastTickNumber uint64
	var ticksReceived uint64
	var lastBroadcastTime time.Time

	for {
		select {
		case <-updateChan:
			// Just count ticks, don't broadcast yet
			ticksReceived++

		case tickerTime := <-ticker.C:
			broadcastStart := time.Now()

			// Measure ticker accuracy
			var tickerDelay time.Duration
			if !lastBroadcastTime.IsZero() {
				tickerDelay = broadcastStart.Sub(lastBroadcastTime)
			}

			// Measure snapshot retrieval
			snapshotStart := time.Now()
			ticks, transactions := h.ringBuffer.GetSnapshot()
			snapshotDuration := time.Since(snapshotStart)

			if len(ticks) == 0 {
				h.logger.Debug("Empty buffer, skipping broadcast",
					zap.Duration("since_last_broadcast", tickerDelay),
				)
				lastBroadcastTime = broadcastStart
				continue
			}

			latestTick := ticks[len(ticks)-1]

			// Only broadcast if we have new data
			if latestTick.TickNumber > lastTickNumber {
				// Measure message creation
				msgCreateStart := time.Now()
				msg := WSMessage{
					Type: "snapshot",
					Data: StreamSnapshot{
						Ticks:        ticks,
						Transactions: transactions,
						Timestamp:    time.Now().Format(time.RFC3339Nano),
					},
					Timestamp: time.Now().Format(time.RFC3339Nano),
				}
				msgCreateDuration := time.Since(msgCreateStart)

				// Measure broadcast to clients
				broadcastClientsStart := time.Now()
				h.clientsMu.RLock()
				clientCount := len(h.clients)
				droppedCount := 0
				for _, client := range h.clients {
					select {
					case client.send <- msg:
					default:
						droppedCount++
						h.logger.Warn("Client send buffer full, dropping snapshot")
					}
				}
				h.clientsMu.RUnlock()
				broadcastClientsDuration := time.Since(broadcastClientsStart)

				// Total duration
				totalDuration := time.Since(broadcastStart)

				// Log performance metrics
				h.logger.Info("Broadcast performance",
					zap.Uint64("latest_tick", latestTick.TickNumber),
					zap.Int("ticks_in_buffer", len(ticks)),
					zap.Int("txs_in_buffer", len(transactions)),
					zap.Int("connected_clients", clientCount),
					zap.Int("dropped_clients", droppedCount),
					zap.Uint64("ticks_received_since_last", ticksReceived),
					// Timing metrics
					zap.Duration("interval_since_last", tickerDelay),
					zap.Duration("snapshot_duration", snapshotDuration),
					zap.Duration("msg_create_duration", msgCreateDuration),
					zap.Duration("broadcast_clients_duration", broadcastClientsDuration),
					zap.Duration("total_duration", totalDuration),
					// Ticker accuracy
					zap.Time("ticker_time", tickerTime),
					zap.Time("actual_time", broadcastStart),
					zap.Duration("ticker_drift", broadcastStart.Sub(tickerTime)),
				)

				lastTickNumber = latestTick.TickNumber
				ticksReceived = 0
			} else {
				// No new data
				h.logger.Debug("No new data, skipping broadcast",
					zap.Uint64("latest_tick", latestTick.TickNumber),
					zap.Duration("since_last_broadcast", tickerDelay),
				)
			}

			lastBroadcastTime = broadcastStart
		}
	}
}

// broadcastLoop broadcasts tick updates to all connected clients
func (h *WebSocketHandler) broadcastLoop() {
	for tick := range h.broadcast {
		msg := WSMessage{
			Type: "tick_update",
			Data: TickUpdate{
				Tick: tick,
			},
			Timestamp: time.Now().Format(time.RFC3339Nano),
		}

		// Send to all clients
		h.clientsMu.RLock()
		for _, client := range h.clients {
			select {
			case client.send <- msg:
			default:
				// Client's send buffer is full, skip
				h.logger.Warn("Client send buffer full, dropping message",
					zap.Uint64("tick_number", tick.TickNumber),
				)
			}
		}
		h.clientsMu.RUnlock()

		// Log stats periodically
		if tick.TickNumber%100 == 0 {
			h.logger.Info("Broadcast tick update",
				zap.Uint64("tick_number", tick.TickNumber),
				zap.Int("connected_clients", h.ClientCount()),
				zap.Int("transactions", len(tick.Transactions)),
			)
		}
	}
}

// writePump pumps messages from the send channel to the WebSocket connection
func (h *WebSocketHandler) writePump(client *wsClient, ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-client.send:
			if !ok {
				// Channel closed
				return
			}

			if err := h.writeJSON(client, msg); err != nil {
				h.logger.Error("Failed to write message", zap.Error(err))
				return
			}

		case <-ticker.C:
			// Send ping
			if err := h.writePing(client); err != nil {
				h.logger.Error("Failed to send ping", zap.Error(err))
				return
			}
		}
	}
}

// readPump pumps messages from the WebSocket connection (handles pings/pongs)
func (h *WebSocketHandler) readPump(client *wsClient, ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	// Set read deadline
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	// Set pong handler
	client.conn.SetPongHandler(func(string) error {
		client.lastPing = time.Now()
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read message (we don't expect clients to send anything, but this detects disconnect)
			_, _, err := client.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					h.logger.Error("WebSocket read error", zap.Error(err))
				}
				return
			}
			// Update read deadline
			client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
	}
}

// writeJSON writes a JSON message to the client (thread-safe)
func (h *WebSocketHandler) writeJSON(client *wsClient, msg interface{}) error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()

	client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return client.conn.WriteJSON(msg)
}

// writePing writes a ping message to the client (thread-safe)
func (h *WebSocketHandler) writePing(client *wsClient) error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()

	client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return client.conn.WriteMessage(websocket.PingMessage, []byte{})
}

// removeClient removes a client from the connected clients map
func (h *WebSocketHandler) removeClient(conn *websocket.Conn) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if client, ok := h.clients[conn]; ok {
		close(client.send)
		close(client.done)
		conn.Close()
		delete(h.clients, conn)
	}
}

// ClientCount returns the number of connected clients
func (h *WebSocketHandler) ClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// Close gracefully closes all WebSocket connections
func (h *WebSocketHandler) Close() {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	for conn, client := range h.clients {
		close(client.send)
		close(client.done)
		conn.Close()
	}

	h.clients = make(map[*websocket.Conn]*wsClient)
	close(h.broadcast)
}
