package stream

import (
	"context"
	"fmt"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// GRPCReader reads ticks from a Continuum gRPC StreamTicks endpoint.
// It implements automatic reconnection with exponential backoff.
type GRPCReader struct {
	serverAddr string
	startTick  uint64
	conn       *grpc.ClientConn
	client     pb.SequencerServiceClient
	logger     *zap.Logger

	// Reconnection config
	maxRetries     int
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	backoffFactor  float64
	reconnectDelay time.Duration
}

// GRPCReaderOption is a functional option for configuring GRPCReader.
type GRPCReaderOption func(*GRPCReader)

// WithStartTick sets the starting tick number (0 = latest).
func WithStartTick(tick uint64) GRPCReaderOption {
	return func(r *GRPCReader) {
		r.startTick = tick
	}
}

// WithMaxRetries sets the maximum number of reconnection attempts (0 = infinite).
func WithMaxRetries(max int) GRPCReaderOption {
	return func(r *GRPCReader) {
		r.maxRetries = max
	}
}

// WithBackoffConfig sets the exponential backoff configuration.
func WithBackoffConfig(base, max time.Duration, factor float64) GRPCReaderOption {
	return func(r *GRPCReader) {
		r.baseBackoff = base
		r.maxBackoff = max
		r.backoffFactor = factor
	}
}

// WithLogger sets the logger for the reader.
func WithLogger(logger *zap.Logger) GRPCReaderOption {
	return func(r *GRPCReader) {
		r.logger = logger
	}
}

// NewGRPCReader creates a new gRPC stream reader.
func NewGRPCReader(serverAddr string, opts ...GRPCReaderOption) *GRPCReader {
	r := &GRPCReader{
		serverAddr:     serverAddr,
		startTick:      0, // Default: start from latest
		maxRetries:     0, // Default: infinite retries
		baseBackoff:    1 * time.Second,
		maxBackoff:     30 * time.Second,
		backoffFactor:  2.0,
		reconnectDelay: 500 * time.Millisecond,
		logger:         zap.NewNop(), // Default: no-op logger
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Read starts reading ticks from the gRPC stream.
// Returns two channels: one for ticks and one for errors.
// Both channels are closed when the context is canceled.
func (r *GRPCReader) Read(ctx context.Context) (<-chan *pb.Tick, <-chan error) {
	tickCh := make(chan *pb.Tick, 100) // Buffer for smooth flow
	errCh := make(chan error, 10)

	go r.readLoop(ctx, tickCh, errCh)

	return tickCh, errCh
}

// Close closes the gRPC connection.
func (r *GRPCReader) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// readLoop is the main reading loop with reconnection logic.
func (r *GRPCReader) readLoop(ctx context.Context, tickCh chan<- *pb.Tick, errCh chan<- error) {
	defer close(tickCh)
	defer close(errCh)

	attempts := 0
	backoff := r.baseBackoff

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Connect to gRPC server
		if err := r.connect(ctx); err != nil {
			errCh <- fmt.Errorf("failed to connect: %w", err)

			// Check if we should retry
			if r.maxRetries > 0 && attempts >= r.maxRetries {
				errCh <- fmt.Errorf("max retries (%d) exceeded", r.maxRetries)
				return
			}

			attempts++
			r.sleep(ctx, backoff)
			backoff = r.nextBackoff(backoff)
			continue
		}

		// Start streaming
		stream, err := r.client.StreamTicks(ctx, &pb.StreamTicksRequest{
			StartTick: r.startTick,
		})
		if err != nil {
			errCh <- fmt.Errorf("failed to start stream: %w", err)
			r.sleep(ctx, r.reconnectDelay)
			continue
		}

		// Reset backoff on successful connection
		backoff = r.baseBackoff
		attempts = 0

		// Read from stream
		if shouldReconnect := r.readStream(ctx, stream, tickCh, errCh); !shouldReconnect {
			return
		}

		// Wait before reconnecting
		r.sleep(ctx, r.reconnectDelay)
	}
}

// connect establishes a gRPC connection.
func (r *GRPCReader) connect(ctx context.Context) error {
	// Close existing connection if any
	if r.conn != nil {
		_ = r.conn.Close()
	}

	conn, err := grpc.NewClient(
		r.serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB max message size
		),
	)
	if err != nil {
		return err
	}

	r.conn = conn
	r.client = pb.NewSequencerServiceClient(conn)

	return nil
}

// readStream reads ticks from the stream until an error occurs.
// Returns true if we should reconnect, false if we should stop.
func (r *GRPCReader) readStream(ctx context.Context, stream pb.SequencerService_StreamTicksClient, tickCh chan<- *pb.Tick, errCh chan<- error) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		tick, err := stream.Recv()
		if err != nil {
			// Check if error is recoverable
			if r.isRecoverableError(err) {
				errCh <- fmt.Errorf("stream error (will reconnect): %w", err)
				return true
			}

			// Non-recoverable error or context canceled
			errCh <- fmt.Errorf("stream error (fatal): %w", err)
			return false
		}

		// Send tick to channel
		select {
		case tickCh <- tick:
		case <-ctx.Done():
			return false
		}
	}
}

// sleep sleeps for the given duration or until context is canceled.
func (r *GRPCReader) sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// nextBackoff calculates the next backoff duration with exponential growth.
func (r *GRPCReader) nextBackoff(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * r.backoffFactor)
	if next > r.maxBackoff {
		return r.maxBackoff
	}
	return next
}

// isRecoverableError determines if a gRPC error is recoverable.
func (r *GRPCReader) isRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation (not recoverable)
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Check gRPC status codes
	st, ok := status.FromError(err)
	if !ok {
		// Unknown error type, treat as recoverable
		r.logger.Debug("Unknown error type, treating as recoverable", zap.Error(err))
		return true
	}

	// Recoverable status codes
	switch st.Code() {
	case codes.Unavailable,    // Server unavailable
		codes.DeadlineExceeded, // Request timeout
		codes.Canceled,         // Request canceled
		codes.Aborted,          // Operation aborted
		codes.Internal,         // Internal server error
		codes.Unknown:          // Unknown error
		return true
	default:
		return false
	}
}
