package stream

import (
	"context"
	"testing"
	"time"

	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestNewGRPCReader(t *testing.T) {
	reader := NewGRPCReader("localhost:50051")

	if reader.serverAddr != "localhost:50051" {
		t.Errorf("serverAddr = %v, want localhost:50051", reader.serverAddr)
	}

	if reader.startTick != 0 {
		t.Errorf("startTick = %v, want 0", reader.startTick)
	}

	if reader.maxRetries != 0 {
		t.Errorf("maxRetries = %v, want 0 (infinite)", reader.maxRetries)
	}

	if reader.baseBackoff != 1*time.Second {
		t.Errorf("baseBackoff = %v, want 1s", reader.baseBackoff)
	}
}

func TestNewGRPCReader_WithOptions(t *testing.T) {
	reader := NewGRPCReader(
		"localhost:50051",
		WithStartTick(1000),
		WithMaxRetries(5),
		WithBackoffConfig(500*time.Millisecond, 10*time.Second, 1.5),
	)

	if reader.startTick != 1000 {
		t.Errorf("startTick = %v, want 1000", reader.startTick)
	}

	if reader.maxRetries != 5 {
		t.Errorf("maxRetries = %v, want 5", reader.maxRetries)
	}

	if reader.baseBackoff != 500*time.Millisecond {
		t.Errorf("baseBackoff = %v, want 500ms", reader.baseBackoff)
	}

	if reader.maxBackoff != 10*time.Second {
		t.Errorf("maxBackoff = %v, want 10s", reader.maxBackoff)
	}

	if reader.backoffFactor != 1.5 {
		t.Errorf("backoffFactor = %v, want 1.5", reader.backoffFactor)
	}
}

func TestGRPCReader_Close(t *testing.T) {
	reader := NewGRPCReader("localhost:50051")

	// Close without connection should not error
	err := reader.Close()
	if err != nil {
		t.Errorf("Close() on nil connection error = %v, want nil", err)
	}
}

func TestGRPCReader_NextBackoff(t *testing.T) {
	reader := NewGRPCReader(
		"localhost:50051",
		WithBackoffConfig(1*time.Second, 10*time.Second, 2.0),
	)

	tests := []struct {
		name    string
		current time.Duration
		want    time.Duration
	}{
		{
			name:    "first backoff",
			current: 1 * time.Second,
			want:    2 * time.Second,
		},
		{
			name:    "second backoff",
			current: 2 * time.Second,
			want:    4 * time.Second,
		},
		{
			name:    "third backoff",
			current: 4 * time.Second,
			want:    8 * time.Second,
		},
		{
			name:    "capped at max",
			current: 8 * time.Second,
			want:    10 * time.Second, // Capped at maxBackoff
		},
		{
			name:    "already at max",
			current: 10 * time.Second,
			want:    10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reader.nextBackoff(tt.current)
			if got != tt.want {
				t.Errorf("nextBackoff(%v) = %v, want %v", tt.current, got, tt.want)
			}
		})
	}
}

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context canceled",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "unavailable",
			err:  status.Error(codes.Unavailable, "service unavailable"),
			want: true,
		},
		{
			name: "deadline exceeded",
			err:  status.Error(codes.DeadlineExceeded, "deadline exceeded"),
			want: true,
		},
		{
			name: "internal error",
			err:  status.Error(codes.Internal, "internal error"),
			want: true,
		},
		{
			name: "invalid argument (not recoverable)",
			err:  status.Error(codes.InvalidArgument, "invalid argument"),
			want: false,
		},
		{
			name: "permission denied (not recoverable)",
			err:  status.Error(codes.PermissionDenied, "permission denied"),
			want: false,
		},
		{
			name: "not found (not recoverable)",
			err:  status.Error(codes.NotFound, "not found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewGRPCReader("localhost:50051")
			got := reader.isRecoverableError(tt.err)
			if got != tt.want {
				t.Errorf("isRecoverableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGRPCReader_Sleep(t *testing.T) {
	reader := NewGRPCReader("localhost:50051")

	t.Run("sleep completes", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now()
		reader.sleep(ctx, 100*time.Millisecond)
		elapsed := time.Since(start)

		if elapsed < 100*time.Millisecond {
			t.Errorf("sleep() duration = %v, want >= 100ms", elapsed)
		}
		if elapsed > 200*time.Millisecond {
			t.Errorf("sleep() duration = %v, want < 200ms", elapsed)
		}
	})

	t.Run("sleep canceled by context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Cancel context after 50ms
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		reader.sleep(ctx, 1*time.Second) // Try to sleep for 1 second
		elapsed := time.Since(start)

		// Should wake up early due to context cancellation
		if elapsed > 200*time.Millisecond {
			t.Errorf("sleep() with canceled context took %v, want < 200ms", elapsed)
		}
	})
}

// TestGRPCReader_Read_ContextCancellation tests that Read stops when context is canceled.
func TestGRPCReader_Read_ContextCancellation(t *testing.T) {
	reader := NewGRPCReader("invalid:99999") // Invalid address

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tickCh, errCh := reader.Read(ctx)

	// Collect all errors (should get connection errors)
	var errors []error
	timeout := time.After(200 * time.Millisecond)

readLoop:
	for {
		select {
		case err, ok := <-errCh:
			if !ok {
				// Channel closed, exit
				break readLoop
			}
			errors = append(errors, err)
		case <-timeout:
			t.Fatal("Test timed out waiting for error channel to close")
		}
	}

	// Verify tick channel is also closed
	select {
	case _, ok := <-tickCh:
		if ok {
			t.Error("Tick channel should be closed")
		}
	default:
		t.Error("Tick channel should be closed")
	}

	// Should have received at least one connection error
	if len(errors) == 0 {
		t.Error("Expected at least one connection error")
	}
}

// Mock stream for testing (we'll create a simple version)
type mockTickStream struct {
	ticks []*pb.Tick
	err   error
	index int
}

func (m *mockTickStream) Recv() (*pb.Tick, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.index >= len(m.ticks) {
		return nil, status.Error(codes.Unavailable, "stream ended")
	}

	tick := m.ticks[m.index]
	m.index++
	return tick, nil
}

// CloseSend implements the gRPC stream interface
func (m *mockTickStream) CloseSend() error {
	return nil
}

// Header implements the gRPC stream interface
func (m *mockTickStream) Header() (metadata.MD, error) {
	return nil, nil
}

// Trailer implements the gRPC stream interface
func (m *mockTickStream) Trailer() metadata.MD {
	return nil
}

// Context implements the gRPC stream interface
func (m *mockTickStream) Context() context.Context {
	return context.Background()
}

// SendMsg implements the gRPC stream interface
func (m *mockTickStream) SendMsg(msg any) error {
	return nil
}

// RecvMsg implements the gRPC stream interface
func (m *mockTickStream) RecvMsg(msg any) error {
	return nil
}

func TestGRPCReader_ReadStream(t *testing.T) {
	reader := NewGRPCReader("localhost:50051")

	t.Run("read successful ticks", func(t *testing.T) {
		ticks := []*pb.Tick{
			{TickNumber: 1},
			{TickNumber: 2},
			{TickNumber: 3},
		}

		mockStream := &mockTickStream{ticks: ticks}
		tickCh := make(chan *pb.Tick, 10)
		errCh := make(chan error, 10)

		ctx := context.Background()

		go func() {
			shouldReconnect := reader.readStream(ctx, mockStream, tickCh, errCh)
			if !shouldReconnect {
				t.Error("Expected shouldReconnect = true for recoverable error")
			}
			close(tickCh)
			close(errCh)
		}()

		// Collect ticks
		var received []*pb.Tick
		for tick := range tickCh {
			received = append(received, tick)
		}

		if len(received) != 3 {
			t.Errorf("Received %d ticks, want 3", len(received))
		}
	})

	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		mockStream := &mockTickStream{ticks: []*pb.Tick{{TickNumber: 1}}}
		tickCh := make(chan *pb.Tick, 10)
		errCh := make(chan error, 10)

		shouldReconnect := reader.readStream(ctx, mockStream, tickCh, errCh)
		if shouldReconnect {
			t.Error("Expected shouldReconnect = false for canceled context")
		}
	})
}
