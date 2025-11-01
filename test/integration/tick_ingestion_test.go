// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	"github.com/fermilabs/fermi-api-gateway/internal/ingestion"
	"github.com/fermilabs/fermi-api-gateway/internal/parser"
	"github.com/fermilabs/fermi-api-gateway/internal/writer"
	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// TestTickIngestion_EndToEnd tests the complete ingestion pipeline
// Run with: go test -tags=integration ./test/integration -v
//
// Prerequisites:
// 1. TimescaleDB running and accessible
// 2. Schema applied (schema/001_create_tables.sql)
// 3. INTEGRATION_DB_URL environment variable set
func TestTickIngestion_EndToEnd(t *testing.T) {
	t.Skip("Integration test - requires TimescaleDB and gRPC server")

	// This is a template for integration testing
	// Uncomment and configure when ready to run

	/*
	// 1. Connect to test database
	ctx := context.Background()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Fatal("INTEGRATION_DB_URL not set")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// 2. Clean test data
	_, err = pool.Exec(ctx, "TRUNCATE TABLE tick_transactions, vdf_proofs, ticks")
	if err != nil {
		t.Fatalf("Failed to clean test data: %v", err)
	}

	// 3. Create mock stream reader with test data
	testTicks := createTestTicks(100)
	reader := &mockStreamReader{ticks: testTicks}

	// 4. Create parser and writer
	parser := parser.NewProtobufParser()
	writer := writer.NewTimescaleWriter(pool, zap.NewNop())

	// 5. Create and run pipeline
	config := ingestion.PipelineConfig{
		BufferSize:    1000,
		WorkerCount:   4,
		BatchSize:     50,
		FlushInterval: 100 * time.Millisecond,
	}

	pipeline := ingestion.NewPipeline(reader, parser, writer, zap.NewNop(), config)

	pipelineCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err = pipeline.Run(pipelineCtx)
	if err != nil {
		t.Fatalf("Pipeline error: %v", err)
	}

	// 6. Verify data was written
	var count int64
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM ticks").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count ticks: %v", err)
	}

	if count != 100 {
		t.Errorf("Expected 100 ticks, got %d", count)
	}

	// 7. Verify VDF proofs
	var proofCount int64
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM vdf_proofs").Scan(&proofCount)
	if err != nil {
		t.Fatalf("Failed to count proofs: %v", err)
	}

	if proofCount != 100 {
		t.Errorf("Expected 100 proofs, got %d", proofCount)
	}

	t.Logf("✓ Integration test passed: %d ticks ingested", count)
	*/
}

// TestTickIngestion_HighThroughput tests 10k ticks/sec ingestion
func TestTickIngestion_HighThroughput(t *testing.T) {
	t.Skip("Load test - requires TimescaleDB and resources")

	// Template for load testing
	/*
	// Generate 60,000 ticks (6 seconds at 10k/sec)
	tickCount := 60000
	testTicks := createTestTicks(tickCount)

	// ... setup pipeline with production config ...

	start := time.Now()
	// ... run pipeline ...
	duration := time.Since(start)

	throughput := float64(tickCount) / duration.Seconds()
	t.Logf("Throughput: %.0f ticks/sec", throughput)

	if throughput < 10000 {
		t.Errorf("Throughput %.0f below target 10k ticks/sec", throughput)
	}
	*/
}

// Helper: Create test ticks for integration testing
func createTestTicks(count int) []*pb.Tick {
	ticks := make([]*pb.Tick, count)

	for i := 0; i < count; i++ {
		ticks[i] = &pb.Tick{
			TickNumber: uint64(i + 1),
			Timestamp:  uint64(time.Now().Add(time.Duration(i) * time.Second).UnixMicro()),
			VdfProof: &pb.VdfProof{
				Input:      "test_input",
				Output:     "test_output",
				Proof:      "test_proof",
				Iterations: 1000,
			},
			TransactionBatchHash: "test_hash",
			PreviousOutput:       "prev_output",
			Transactions:         []*pb.OrderedTransaction{},
		}
	}

	return ticks
}

// Mock stream reader for testing
type mockStreamReader struct {
	ticks []*pb.Tick
}

func (m *mockStreamReader) Read(ctx context.Context) (<-chan *pb.Tick, <-chan error) {
	tickCh := make(chan *pb.Tick, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(tickCh)
		defer close(errCh)

		for _, tick := range m.ticks {
			select {
			case tickCh <- tick:
			case <-ctx.Done():
				return
			}
		}

		// Keep channel open until context canceled
		<-ctx.Done()
	}()

	return tickCh, errCh
}

func (m *mockStreamReader) Close() error {
	return nil
}
