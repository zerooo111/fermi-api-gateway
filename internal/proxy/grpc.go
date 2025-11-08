package proxy

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fermilabs/fermi-api-gateway/internal/database"
	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
)

// GRPCProxy handles gRPC proxying and converts HTTP requests to gRPC calls
type GRPCProxy struct {
	target     string
	conn       *grpc.ClientConn
	client     pb.SequencerServiceClient
	repository *database.Repository
	restURL    string
	logger     *zap.Logger
}

// NewGRPCProxy creates a new gRPC proxy client
func NewGRPCProxy(target string, repository *database.Repository, restURL string, logger *zap.Logger) (*GRPCProxy, error) {
	// Create gRPC connection with connection pooling
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
			grpc.MaxCallSendMsgSize(10*1024*1024), // 10MB
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	client := pb.NewSequencerServiceClient(conn)

	// Use nop logger if none provided
	if logger == nil {
		logger = zap.NewNop()
	}

	return &GRPCProxy{
		target:     target,
		conn:       conn,
		client:     client,
		repository: repository,
		restURL:    restURL,
		logger:     logger,
	}, nil
}

// Close closes the gRPC connection
func (p *GRPCProxy) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// transactionRequest is an intermediate struct that matches the JSON format
// It allows JSON to unmarshal arrays directly into []byte (like GIN does)
type transactionRequest struct {
	TxID      string      `json:"tx_id"`
	Payload   interface{} `json:"payload"`   // Can be array [70,82,77,...] or base64 string
	Signature string      `json:"signature"` // Hex string (needs decoding)
	PublicKey string      `json:"public_key"` // Base58 string (needs decoding)
	Nonce     uint64      `json:"nonce"`
	Timestamp interface{} `json:"timestamp"` // Can be string or number
}

// toProtobuf converts the transactionRequest to a protobuf Transaction
func (tx *transactionRequest) toProtobuf() (*pb.Transaction, error) {
	// Convert payload: handle both array and base64 string
	var payloadBytes []byte
	switch v := tx.Payload.(type) {
	case []interface{}:
		// Array of numbers - convert to []byte
		bytes := make([]byte, 0, len(v))
		for _, num := range v {
			if n, ok := num.(float64); ok {
				bytes = append(bytes, byte(n))
			} else {
				return nil, fmt.Errorf("payload array contains non-numeric values")
			}
		}
		payloadBytes = bytes
	case string:
		// Base64 string - decode it
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid payload (not valid base64): %w", err)
		}
		payloadBytes = decoded
	default:
		return nil, fmt.Errorf("payload must be an array of numbers or base64 string")
	}

	// Convert timestamp: handle both string and number
	var timestamp uint64
	switch v := tx.Timestamp.(type) {
	case string:
		ts, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %w", err)
		}
		timestamp = ts
	case float64:
		timestamp = uint64(v)
	case nil:
		// Timestamp is optional, use 0
	default:
		return nil, fmt.Errorf("timestamp must be a string or number")
	}

	// Decode signature from hex string to bytes
	signatureBytes, err := decodeHex(tx.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	// Decode public key from base58 string to bytes
	publicKeyBytes, err := decodeBase58(tx.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public_key: %w", err)
	}

	return &pb.Transaction{
		TxId:      tx.TxID,
		Payload:   payloadBytes,
		Signature: signatureBytes,
		PublicKey: publicKeyBytes,
		Nonce:     tx.Nonce,
		Timestamp: timestamp,
	}, nil
}

// decodeHex decodes a hex string to bytes
func decodeHex(s string) ([]byte, error) {
	// Remove 0x prefix if present
	if len(s) > 2 && s[0:2] == "0x" {
		s = s[2:]
	}
	// Use standard encoding/hex package
	return hex.DecodeString(s)
}

// decodeBase58 decodes a base58 string to bytes
// Base58 alphabet: 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz (no 0, O, I, l)
// For now, we'll try to decode as hex first (some base58 strings might be hex-compatible)
// If that fails, we'll need a proper base58 library, but for now treat as raw bytes
func decodeBase58(s string) ([]byte, error) {
	// Try hex first (some keys might be hex-encoded)
	if bytes, err := hex.DecodeString(s); err == nil {
		return bytes, nil
	}
	// If not hex, for now just convert string to bytes
	// TODO: Implement proper base58 decoding if needed
	// The public key "CRbNEfDGMKHiWcibuJCbxP6KKvdGEDm2EZE46cBX4kHa" looks like base58
	// For a proper implementation, you'd need a base58 library like github.com/mr-tron/base58
	return []byte(s), nil
}

// HandleSubmitTransaction handles POST /api/continuum/grpc/submit-transaction
func (p *GRPCProxy) HandleSubmitTransaction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			p.logger.Warn("Failed to read request body", zap.Error(err))
			http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
			return
		}

		// Log request body for debugging (limit to first 500 chars to avoid huge logs)
		bodyPreview := string(body)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "... (truncated)"
		}
		p.logger.Debug("Received transaction submission request", 
			zap.String("body_preview", bodyPreview),
			zap.Int("body_length", len(body)),
			zap.String("content_type", r.Header.Get("Content-Type")))

		// Check if body is empty
		if len(body) == 0 {
			p.logger.Warn("Empty request body received")
			http.Error(w, `{"error":"request body is empty"}`, http.StatusBadRequest)
			return
		}

		// Parse JSON into intermediate struct (like GIN does)
		// This allows JSON arrays to unmarshal directly into []byte
		var bodyStruct struct {
			Transaction transactionRequest `json:"transaction"`
		}
		if err := json.Unmarshal(body, &bodyStruct); err != nil {
			p.logger.Warn("Failed to unmarshal JSON request", 
				zap.Error(err),
				zap.String("body_preview", bodyPreview))
			http.Error(w, fmt.Sprintf(`{"error":"invalid request body: %v"}`, err), http.StatusBadRequest)
			return
		}

		// Convert to protobuf transaction (like ToProtobuf() in GIN handler)
		grpcTx, err := bodyStruct.Transaction.toProtobuf()
		if err != nil {
			p.logger.Warn("Failed to convert transaction to protobuf", 
				zap.Error(err),
				zap.String("body_preview", bodyPreview))
			http.Error(w, fmt.Sprintf(`{"error":"invalid transaction data: %v"}`, err), http.StatusBadRequest)
			return
		}

		// Create protobuf request
		req := &pb.SubmitTransactionRequest{
			Transaction: grpcTx,
		}

		// Call gRPC service
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		resp, err := p.client.SubmitTransaction(ctx, req)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"grpc call failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Return JSON response using protojson for consistency
		w.Header().Set("Content-Type", "application/json")
		marshaler := &protojson.MarshalOptions{
			UseProtoNames: true, // Use snake_case field names as in proto
		}
		jsonBytes, err := marshaler.Marshal(resp)
		if err != nil {
			p.logger.Warn("Failed to marshal response", zap.Error(err))
			http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
			return
		}
		w.Write(jsonBytes)
	}
}

// HandleSubmitBatch handles POST /api/continuum/grpc/submit-batch
func (p *GRPCProxy) HandleSubmitBatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
			return
		}

		var req pb.SubmitBatchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid request: %v"}`, err), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		resp, err := p.client.SubmitBatch(ctx, &req)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"grpc call failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleGetStatus handles GET /api/continuum/grpc/status
func (p *GRPCProxy) HandleGetStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		resp, err := p.client.GetStatus(ctx, &pb.GetStatusRequest{})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"grpc call failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleGetTransaction handles GET /api/continuum/grpc/transaction/{hash}
func (p *GRPCProxy) HandleGetTransaction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Extract tx_hash from URL (assuming Chi router extracts it)
		txHash := r.URL.Query().Get("hash")
		if txHash == "" {
			http.Error(w, `{"error":"missing tx_hash parameter"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		resp, err := p.client.GetTransaction(ctx, &pb.GetTransactionRequest{
			TxHash: txHash,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"grpc call failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleGetTick handles GET /api/continuum/grpc/tick/{number}
func (p *GRPCProxy) HandleGetTick() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		tickNumberStr := r.URL.Query().Get("number")
		if tickNumberStr == "" {
			http.Error(w, `{"error":"missing tick_number parameter"}`, http.StatusBadRequest)
			return
		}

		tickNumber, err := strconv.ParseUint(tickNumberStr, 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid tick_number"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		resp, err := p.client.GetTick(ctx, &pb.GetTickRequest{
			TickNumber: tickNumber,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"grpc call failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleGetChainState handles GET /api/continuum/grpc/chain-state
func (p *GRPCProxy) HandleGetChainState() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Optional tick_limit parameter
		tickLimitStr := r.URL.Query().Get("tick_limit")
		var tickLimit uint32 = 10 // default

		if tickLimitStr != "" {
			limit, err := strconv.ParseUint(tickLimitStr, 10, 32)
			if err != nil {
				http.Error(w, `{"error":"invalid tick_limit"}`, http.StatusBadRequest)
				return
			}
			tickLimit = uint32(limit)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		resp, err := p.client.GetChainState(ctx, &pb.GetChainStateRequest{
			TickLimit: tickLimit,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"grpc call failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleStreamTicks handles GET /api/continuum/grpc/stream-ticks (Server-Sent Events)
func (p *GRPCProxy) HandleStreamTicks() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Get start_tick parameter
		startTickStr := r.URL.Query().Get("start_tick")
		var startTick uint64 = 0

		if startTickStr != "" {
			tick, err := strconv.ParseUint(startTickStr, 10, 64)
			if err != nil {
				http.Error(w, `{"error":"invalid start_tick"}`, http.StatusBadRequest)
				return
			}
			startTick = tick
		}

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ctx := r.Context()
		stream, err := p.client.StreamTicks(ctx, &pb.StreamTicksRequest{
			StartTick: startTick,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"grpc stream failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Stream ticks as Server-Sent Events
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
			return
		}

		for {
			tick, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				// Connection closed or error
				break
			}

			// Marshal tick to JSON
			data, err := json.Marshal(tick)
			if err != nil {
				continue
			}

			// Write SSE format: data: {...}\n\n
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Check if client disconnected
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

// HandleGetRecentTransactions handles GET /api/v1/continuum/tx/recent?limit=10
func (p *GRPCProxy) HandleGetRecentTransactions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Get limit parameter (optional)
		limitStr := r.URL.Query().Get("limit")
		limit := 50 // default
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit < 1 || parsedLimit > 1000 {
				http.Error(w, `{"error":"invalid limit (must be 1-1000)"}`, http.StatusBadRequest)
				return
			}
			limit = parsedLimit
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Try database first (if available)
		if p.repository != nil {
			transactions, err := p.repository.GetRecentTransactions(ctx, limit)
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("X-Data-Source", "database")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"transactions": transactions,
					"count":        len(transactions),
				})
				return
			}
			// Log the database error for debugging
			p.logger.Debug("Database query failed", zap.Error(err))
		}

		// Return empty result - database not available or not configured
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("X-Data-Source", "database_unavailable")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"transactions": []interface{}{},
			"count":        0,
			"message":      "Recent transactions unavailable - database not configured or unavailable",
		})
	}
}

// HandleGetTransactionByHash handles GET /api/v1/continuum/tx/:hash
func (p *GRPCProxy) HandleGetTransactionByHash() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Extract hash from URL path - expecting /api/v1/continuum/tx/{hash}
		// After Chi routing, r.URL.Path will be /tx/{hash}
		txHash := strings.TrimPrefix(r.URL.Path, "/tx/")
		txHash = sanitizeInput(txHash)

		if err := validateTransactionHash(txHash); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid transaction hash: %v"}`, err), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Try database first (if available)
		if p.repository != nil {
			tx, err := p.repository.GetTransaction(ctx, txHash)
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "private, max-age=1800")
				w.Header().Set("X-Data-Source", "database")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"source": "db",
					"data":   tx,
				})
				return
			}
		}

		// Fallback to REST API
		resp, err := http.Get(p.restURL + "/tx/" + txHash)
		if err != nil {
			http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			http.Error(w, `{"error":"transaction not found"}`, http.StatusNotFound)
			return
		}

		if resp.StatusCode != http.StatusOK {
			http.Error(w, fmt.Sprintf(`{"error":"upstream returned status %d"}`, resp.StatusCode), http.StatusServiceUnavailable)
			return
		}

		var data interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			http.Error(w, `{"error":"failed to decode response"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "private, max-age=1800")
		w.Header().Set("X-Data-Source", "rest-api")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"source": "continuum",
			"data":   data,
		})
	}
}

// sanitizeInput removes potentially harmful characters
func sanitizeInput(input string) string {
	// Allow only alphanumeric and common safe characters
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	return re.ReplaceAllString(input, "")
}

// validateTransactionHash validates a transaction hash format
func validateTransactionHash(hash string) error {
	if len(hash) == 0 {
		return fmt.Errorf("hash cannot be empty")
	}
	if len(hash) > 128 {
		return fmt.Errorf("hash too long")
	}
	// Check if it's a valid hex string
	matched, _ := regexp.MatchString(`^[a-fA-F0-9]+$`, hash)
	if !matched {
		return fmt.Errorf("hash must be a valid hex string")
	}
	return nil
}
