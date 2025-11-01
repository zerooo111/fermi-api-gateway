package ingestion

import (
	"context"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
)

// StreamReader reads ticks from a source (gRPC stream, file, mock, etc.).
// Implementations must be safe for concurrent use.
//
// The Read method returns two channels:
//   - tick channel: emits ticks as they arrive
//   - error channel: emits errors that occur during streaming
//
// Both channels are closed when the context is canceled or the stream ends.
type StreamReader interface {
	// Read starts reading from the stream and returns channels for ticks and errors.
	// The returned channels will be closed when ctx is canceled or the stream ends.
	Read(ctx context.Context) (<-chan *pb.Tick, <-chan error)

	// Close gracefully shuts down the stream reader and releases resources.
	Close() error
}

// Parser transforms protobuf ticks into domain model ticks.
// Implementations should be stateless and safe for concurrent use.
//
// The Parse method converts a single protobuf tick to a domain tick.
// It should handle all necessary transformations including:
//   - Type conversions (uint64 timestamps to time.Time)
//   - Nested message mapping (VDFProof, Transactions)
//   - Adding metadata (ReceivedAt timestamp)
type Parser interface {
	// Parse converts a protobuf tick to a domain tick.
	// Returns an error if the tick cannot be parsed or is invalid.
	Parse(tick *pb.Tick) (*domain.Tick, error)
}

// Writer persists or outputs processed ticks.
// Implementations must be safe for concurrent use from multiple goroutines.
//
// The Writer interface provides both single-write and batch-write capabilities:
//   - Write: for real-time processing of individual ticks
//   - WriteBatch: for optimized bulk persistence (e.g., database COPY protocol)
//
// Implementations should handle their own connection pooling and error recovery.
type Writer interface {
	// Write persists a single tick.
	// Returns an error if the write operation fails.
	Write(ctx context.Context, tick *domain.Tick) error

	// WriteBatch persists multiple ticks in a single operation.
	// This method should be optimized for bulk writes (e.g., using COPY protocol).
	// Returns an error if any tick in the batch fails to write.
	WriteBatch(ctx context.Context, ticks []*domain.Tick) error

	// Close gracefully shuts down the writer and releases resources.
	// Implementations should flush any pending writes before closing.
	Close() error
}
