package writer

import (
	"context"
	"testing"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	"go.uber.org/zap"
)

// Note: These are unit tests for the TimescaleWriter structure.
// Integration tests with a real TimescaleDB instance should be in a separate file.

func createTestTickForDB() *domain.Tick {
	return &domain.Tick{
		TickNumber: 12345,
		Timestamp:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		VDFProof: domain.VDFProof{
			Input:      "input123",
			Output:     "output456",
			Proof:      "proof789",
			Iterations: 1000,
		},
		BatchHash:  "batch_hash_123",
		PrevOutput: "", // Removed from schema
		ReceivedAt: time.Date(2025, 1, 1, 12, 0, 1, 0, time.UTC),
		Transactions: []domain.Transaction{
			{
				TxID:               "tx1",
				TxHash:             "hash1",
				Payload:            []byte("payload1"),
				Signature:          []byte("sig1"),
				PublicKey:          []byte("pubkey1"),
				Nonce:              1,
				ClientTimestamp:    time.Date(2025, 1, 1, 11, 59, 0, 0, time.UTC),
				SequenceNumber:     1,
				IngestionTimestamp: time.Date(2025, 1, 1, 12, 0, 0, 500000, time.UTC),
			},
			{
				TxID:               "tx2",
				TxHash:             "hash2",
				Payload:            []byte("payload2"),
				Signature:          []byte("sig2"),
				PublicKey:          []byte("pubkey2"),
				Nonce:              2,
				ClientTimestamp:    time.Date(2025, 1, 1, 11, 59, 30, 0, time.UTC),
				SequenceNumber:     2,
				IngestionTimestamp: time.Date(2025, 1, 1, 12, 0, 0, 600000, time.UTC),
			},
		},
	}
}

func TestNewTimescaleWriter(t *testing.T) {
	logger := zap.NewNop()
	writer := NewTimescaleWriter(nil, logger)

	if writer == nil {
		t.Fatal("NewTimescaleWriter returned nil")
	}

	if writer.logger != logger {
		t.Error("Logger not set correctly")
	}
}

func TestTimescaleWriter_WriteBatch_EmptyBatch(t *testing.T) {
	logger := zap.NewNop()
	writer := NewTimescaleWriter(nil, logger)

	ctx := context.Background()
	err := writer.WriteBatch(ctx, []*domain.Tick{})

	if err != nil {
		t.Errorf("WriteBatch with empty slice should not error, got: %v", err)
	}
}

// Integration test marker
// These tests require a real TimescaleDB instance and should be run separately
// Run with: go test -tags=integration ./internal/writer

// TestTimescaleWriter_Integration tests the writer against a real database
// This test is skipped in normal test runs
func TestTimescaleWriter_Integration(t *testing.T) {
	t.Skip("Integration test - requires TimescaleDB. Run with -tags=integration")

	// Example integration test structure:
	// 1. Connect to test TimescaleDB
	// 2. Create test tables
	// 3. Write test data
	// 4. Verify data was written correctly
	// 5. Clean up test data
}

func TestTimescaleWriter_Write_CallsWriteBatch(t *testing.T) {
	t.Skip("Requires database connection - use integration tests")

	// This test verifies that Write() delegates to WriteBatch()
	// Run integration tests with a real TimescaleDB instance to test this
}

// Benchmark for COPY performance (requires real DB)
func BenchmarkTimescaleWriter_WriteBatch(b *testing.B) {
	b.Skip("Benchmark requires real TimescaleDB connection")

	// Example benchmark structure:
	// logger := zap.NewNop()
	// pool := connectToTestDB()
	// defer pool.Close()
	// writer := NewTimescaleWriter(pool, logger)
	//
	// ticks := make([]*domain.Tick, 250)
	// for i := range ticks {
	//     ticks[i] = createTestTickForDB()
	//     ticks[i].TickNumber = uint64(i)
	// }
	//
	// b.ResetTimer()
	// for i := 0; i < b.N; i++ {
	//     writer.WriteBatch(context.Background(), ticks)
	// }
}

// Test helper: Validate tick data structure
func TestTickDataStructure(t *testing.T) {
	tick := createTestTickForDB()

	// Verify tick structure matches schema requirements
	if tick.TickNumber == 0 {
		t.Error("TickNumber should not be zero")
	}

	if tick.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if tick.BatchHash == "" {
		t.Error("BatchHash should not be empty")
	}

	if tick.VDFProof.Input == "" {
		t.Error("VDFProof.Input should not be empty")
	}

	if tick.VDFProof.Output == "" {
		t.Error("VDFProof.Output should not be empty")
	}

	if len(tick.Transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(tick.Transactions))
	}

	// Verify transaction structure
	for i, tx := range tick.Transactions {
		if tx.TxHash == "" {
			t.Errorf("Transaction %d: TxHash should not be empty", i)
		}

		if len(tx.Signature) == 0 {
			t.Errorf("Transaction %d: Signature should not be empty", i)
		}

		if len(tx.PublicKey) == 0 {
			t.Errorf("Transaction %d: PublicKey should not be empty", i)
		}

		if tx.ClientTimestamp.IsZero() {
			t.Errorf("Transaction %d: ClientTimestamp should not be zero", i)
		}
	}
}
