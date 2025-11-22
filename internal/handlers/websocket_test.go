package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	"github.com/fermilabs/fermi-api-gateway/internal/stream"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func TestNewWebSocketHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	if handler == nil {
		t.Fatal("NewWebSocketHandler returned nil")
	}
	if handler.ringBuffer != ringBuffer {
		t.Error("handler.ringBuffer not set correctly")
	}
	if handler.cfg != cfg {
		t.Error("handler.cfg not set correctly")
	}
	if handler.logger != logger {
		t.Error("handler.logger not set correctly")
	}
}

func TestWebSocketUpgrade(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleLiveStream))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect to WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Set read deadline
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Should receive initial snapshot
	var msg WSMessage
	err = ws.ReadJSON(&msg)
	if err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	if msg.Type != "snapshot" {
		t.Errorf("Expected message type 'snapshot', got '%s'", msg.Type)
	}

	// Verify snapshot structure (data will be map[string]interface{})
	dataMap, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be map[string]interface{}, got %T", msg.Data)
	}

	// Check ticks array exists
	ticks, ok := dataMap["ticks"].([]interface{})
	if !ok {
		t.Fatalf("Expected ticks to be []interface{}, got %T", dataMap["ticks"])
	}

	// Initial snapshot should be empty
	if len(ticks) != 0 {
		t.Errorf("Expected 0 ticks in empty buffer, got %d", len(ticks))
	}
}

func TestWebSocketTickUpdates(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleLiveStream))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect to WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Read initial snapshot
	var initialMsg WSMessage
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.ReadJSON(&initialMsg); err != nil {
		t.Fatalf("Failed to read initial snapshot: %v", err)
	}

	// Add a tick to ring buffer
	tick := &domain.Tick{
		TickNumber: 1,
		Timestamp:  time.Now(),
		VDFProof: domain.VDFProof{
			Input:      "input1",
			Output:     "output1",
			Proof:      "proof1",
			Iterations: 1000,
		},
		Transactions: []domain.Transaction{
			{
				TxHash:    "tx1",
				Payload:   []byte("payload1"),
				Signature: []byte("sig1"),
				PublicKey: []byte("pk1"),
			},
		},
		BatchHash:  "batch1",
		PrevOutput: "prev1",
		ReceivedAt: time.Now(),
	}

	ringBuffer.AddTick(tick)

	// Should receive tick update
	var updateMsg WSMessage
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.ReadJSON(&updateMsg); err != nil {
		t.Fatalf("Failed to read tick update: %v", err)
	}

	if updateMsg.Type != "tick_update" {
		t.Errorf("Expected message type 'tick_update', got '%s'", updateMsg.Type)
	}

	// Verify update structure
	dataMap, ok := updateMsg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be map[string]interface{}, got %T", updateMsg.Data)
	}

	tickMap, ok := dataMap["tick"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tick to be map[string]interface{}, got %T", dataMap["tick"])
	}

	tickNumber := uint64(tickMap["tick_number"].(float64))
	if tickNumber != 1 {
		t.Errorf("Expected tick number 1, got %d", tickNumber)
	}

	transactions := tickMap["transactions"].([]interface{})
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}
}

func TestWebSocketMultipleClients(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleLiveStream))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect two clients
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 1: %v", err)
	}
	defer ws1.Close()

	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 2: %v", err)
	}
	defer ws2.Close()

	// Read initial snapshots for both clients
	var msg1, msg2 WSMessage
	ws1.SetReadDeadline(time.Now().Add(5 * time.Second))
	ws2.SetReadDeadline(time.Now().Add(5 * time.Second))

	if err := ws1.ReadJSON(&msg1); err != nil {
		t.Fatalf("Client 1 failed to read snapshot: %v", err)
	}
	if err := ws2.ReadJSON(&msg2); err != nil {
		t.Fatalf("Client 2 failed to read snapshot: %v", err)
	}

	// Verify client count
	if handler.ClientCount() != 2 {
		t.Errorf("Expected 2 connected clients, got %d", handler.ClientCount())
	}

	// Add a tick
	tick := &domain.Tick{
		TickNumber: 100,
		Timestamp:  time.Now(),
		VDFProof: domain.VDFProof{
			Input:      "input100",
			Output:     "output100",
			Proof:      "proof100",
			Iterations: 1000,
		},
		Transactions: []domain.Transaction{},
		BatchHash:    "batch100",
		PrevOutput:   "prev100",
		ReceivedAt:   time.Now(),
	}

	ringBuffer.AddTick(tick)

	// Both clients should receive the update
	var update1, update2 WSMessage
	ws1.SetReadDeadline(time.Now().Add(5 * time.Second))
	ws2.SetReadDeadline(time.Now().Add(5 * time.Second))

	if err := ws1.ReadJSON(&update1); err != nil {
		t.Fatalf("Client 1 failed to read update: %v", err)
	}
	if err := ws2.ReadJSON(&update2); err != nil {
		t.Fatalf("Client 2 failed to read update: %v", err)
	}

	if update1.Type != "tick_update" || update2.Type != "tick_update" {
		t.Error("Both clients should receive tick_update")
	}
}

func TestWebSocketClientDisconnect(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleLiveStream))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Read initial snapshot
	var msg WSMessage
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read snapshot: %v", err)
	}

	// Verify client is connected
	if handler.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", handler.ClientCount())
	}

	// Close connection
	ws.Close()

	// Give server time to detect disconnect
	time.Sleep(100 * time.Millisecond)

	// Verify client is disconnected
	if handler.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", handler.ClientCount())
	}
}

func TestWebSocketPingPong(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleLiveStream))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect to WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	// Read initial snapshot
	var msg WSMessage
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read snapshot: %v", err)
	}

	// Set up pong handler
	pongReceived := make(chan bool, 1)
	ws.SetPongHandler(func(appData string) error {
		pongReceived <- true
		return nil
	})

	// Send ping
	if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Server should respond with pong (gorilla/websocket does this automatically)
	// We need to read to trigger the pong handler
	go func() {
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-pongReceived:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Did not receive pong response")
	}
}

func TestWebSocketConcurrentWrites(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleLiveStream))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	// Read initial snapshot
	var msg WSMessage
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := ws.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read snapshot: %v", err)
	}

	// Add multiple ticks concurrently to test thread safety
	done := make(chan bool)
	go func() {
		for i := 0; i < 10; i++ {
			tick := &domain.Tick{
				TickNumber: uint64(i + 1),
				Timestamp:  time.Now(),
				VDFProof: domain.VDFProof{
					Input:      "input",
					Output:     "output",
					Proof:      "proof",
					Iterations: 1000,
				},
				Transactions: []domain.Transaction{},
				BatchHash:    "batch",
				PrevOutput:   "prev",
				ReceivedAt:   time.Now(),
			}
			ringBuffer.AddTick(tick)
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Read all updates
	receivedCount := 0
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))

	go func() {
		for receivedCount < 10 {
			var updateMsg WSMessage
			if err := ws.ReadJSON(&updateMsg); err != nil {
				break
			}
			if updateMsg.Type == "tick_update" {
				receivedCount++
			}
		}
	}()

	<-done
	time.Sleep(500 * time.Millisecond)

	if receivedCount != 10 {
		t.Errorf("Expected to receive 10 tick updates, got %d", receivedCount)
	}
}

func BenchmarkWebSocketBroadcast(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	ringBuffer := stream.NewRingBuffer(100, 100)
	cfg := &config.StreamConfig{
		BufferSize: 100,
		UpdateFPS:  60,
	}

	handler := NewWebSocketHandler(ringBuffer, cfg, logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleLiveStream))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect 10 clients
	clients := make([]*websocket.Conn, 10)
	for i := 0; i < 10; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			b.Fatalf("Failed to connect client %d: %v", i, err)
		}
		defer ws.Close()
		clients[i] = ws

		// Read initial snapshot
		var msg WSMessage
		ws.ReadJSON(&msg)
	}

	// Create tick
	tick := &domain.Tick{
		TickNumber: 1,
		Timestamp:  time.Now(),
		VDFProof: domain.VDFProof{
			Input:      "input",
			Output:     "output",
			Proof:      "proof",
			Iterations: 1000,
		},
		Transactions: []domain.Transaction{},
		BatchHash:    "batch",
		PrevOutput:   "prev",
		ReceivedAt:   time.Now(),
	}

	b.ResetTimer()

	// Benchmark broadcasting to all clients
	for i := 0; i < b.N; i++ {
		tick.TickNumber = uint64(i + 1)
		ringBuffer.AddTick(tick)
	}
}
