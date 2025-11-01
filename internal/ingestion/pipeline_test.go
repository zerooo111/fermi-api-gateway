package ingestion

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"go.uber.org/zap"
)

// Mock StreamReader
type mockStreamReader struct {
	ticks     []*pb.Tick
	err       error
	tickDelay time.Duration
}

func (m *mockStreamReader) Read(ctx context.Context) (<-chan *pb.Tick, <-chan error) {
	tickCh := make(chan *pb.Tick, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(tickCh)
		defer close(errCh)

		if m.err != nil {
			errCh <- m.err
			return
		}

		for _, tick := range m.ticks {
			// Add delay between ticks if configured
			if m.tickDelay > 0 {
				time.Sleep(m.tickDelay)
			}

			select {
			case tickCh <- tick:
			case <-ctx.Done():
				return
			}
		}

		// Keep channel open until context is canceled (simulate real stream)
		<-ctx.Done()
	}()

	return tickCh, errCh
}

func (m *mockStreamReader) Close() error {
	return nil
}

// Mock Parser
type mockParser struct {
	parseFunc func(*pb.Tick) (*domain.Tick, error)
}

func (m *mockParser) Parse(tick *pb.Tick) (*domain.Tick, error) {
	if m.parseFunc != nil {
		return m.parseFunc(tick)
	}

	// Default: simple conversion
	return &domain.Tick{
		TickNumber: tick.TickNumber,
		Timestamp:  time.UnixMicro(int64(tick.Timestamp)),
		VDFProof: domain.VDFProof{
			Input:      tick.VdfProof.Input,
			Output:     tick.VdfProof.Output,
			Proof:      tick.VdfProof.Proof,
			Iterations: tick.VdfProof.Iterations,
		},
		BatchHash:    tick.TransactionBatchHash,
		PrevOutput:   tick.PreviousOutput,
		ReceivedAt:   time.Now(),
		Transactions: []domain.Transaction{},
	}, nil
}

// Mock Writer
type mockWriter struct {
	mu           sync.Mutex
	writtenTicks []*domain.Tick
	batchSizes   []int
	writeErr     error
}

func (m *mockWriter) Write(ctx context.Context, tick *domain.Tick) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeErr != nil {
		return m.writeErr
	}

	m.writtenTicks = append(m.writtenTicks, tick)
	m.batchSizes = append(m.batchSizes, 1)
	return nil
}

func (m *mockWriter) WriteBatch(ctx context.Context, ticks []*domain.Tick) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeErr != nil {
		return m.writeErr
	}

	m.writtenTicks = append(m.writtenTicks, ticks...)
	m.batchSizes = append(m.batchSizes, len(ticks))
	return nil
}

func (m *mockWriter) Close() error {
	return nil
}

func (m *mockWriter) GetWrittenTicks() []*domain.Tick {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writtenTicks
}

func (m *mockWriter) GetBatchSizes() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batchSizes
}

func TestDefaultPipelineConfig(t *testing.T) {
	config := DefaultPipelineConfig()

	if config.BufferSize != 10000 {
		t.Errorf("BufferSize = %v, want 10000", config.BufferSize)
	}

	if config.WorkerCount != 8 {
		t.Errorf("WorkerCount = %v, want 8", config.WorkerCount)
	}

	if config.BatchSize != 250 {
		t.Errorf("BatchSize = %v, want 250", config.BatchSize)
	}

	if config.FlushInterval != 100*time.Millisecond {
		t.Errorf("FlushInterval = %v, want 100ms", config.FlushInterval)
	}
}

func TestNewPipeline(t *testing.T) {
	reader := &mockStreamReader{}
	parser := &mockParser{}
	writer := &mockWriter{}
	logger := zap.NewNop()

	config := PipelineConfig{
		BufferSize:    1000,
		WorkerCount:   4,
		BatchSize:     100,
		FlushInterval: 50 * time.Millisecond,
	}

	pipeline := NewPipeline(reader, parser, writer, logger, config)

	if pipeline.bufferSize != 1000 {
		t.Errorf("bufferSize = %v, want 1000", pipeline.bufferSize)
	}

	if pipeline.workerCount != 4 {
		t.Errorf("workerCount = %v, want 4", pipeline.workerCount)
	}

	if pipeline.batchSize != 100 {
		t.Errorf("batchSize = %v, want 100", pipeline.batchSize)
	}
}

func TestPipeline_Run_ProcessesTicks(t *testing.T) {
	// Create test ticks
	pbTicks := []*pb.Tick{
		{
			TickNumber: 1,
			Timestamp:  uint64(time.Now().UnixMicro()),
			VdfProof: &pb.VdfProof{
				Input:      "input1",
				Output:     "output1",
				Proof:      "proof1",
				Iterations: 1000,
			},
			TransactionBatchHash: "hash1",
			PreviousOutput:       "prev1",
		},
		{
			TickNumber: 2,
			Timestamp:  uint64(time.Now().UnixMicro()),
			VdfProof: &pb.VdfProof{
				Input:      "input2",
				Output:     "output2",
				Proof:      "proof2",
				Iterations: 1000,
			},
			TransactionBatchHash: "hash2",
			PreviousOutput:       "prev2",
		},
		{
			TickNumber: 3,
			Timestamp:  uint64(time.Now().UnixMicro()),
			VdfProof: &pb.VdfProof{
				Input:      "input3",
				Output:     "output3",
				Proof:      "proof3",
				Iterations: 1000,
			},
			TransactionBatchHash: "hash3",
			PreviousOutput:       "prev3",
		},
	}

	reader := &mockStreamReader{ticks: pbTicks, tickDelay: 10 * time.Millisecond}
	parser := &mockParser{}
	writer := &mockWriter{}
	logger := zap.NewNop()

	config := PipelineConfig{
		BufferSize:    10,
		WorkerCount:   2,
		BatchSize:     10, // Large batch to ensure we flush on timer
		FlushInterval: 100 * time.Millisecond,
	}

	pipeline := NewPipeline(reader, parser, writer, logger, config)

	ctx, cancel := context.WithCancel(context.Background())

	// Run pipeline in background
	done := make(chan error)
	go func() {
		done <- pipeline.Run(ctx)
	}()

	// Wait for all ticks to be processed
	time.Sleep(300 * time.Millisecond)

	// Cancel and wait for shutdown
	cancel()
	err := <-done
	if err != nil {
		t.Fatalf("Pipeline.Run() error = %v", err)
	}

	writtenTicks := writer.GetWrittenTicks()
	if len(writtenTicks) != 3 {
		t.Errorf("Pipeline processed %d ticks, want 3", len(writtenTicks))
	}

	// Verify all tick numbers are present (order not guaranteed due to parallel processing)
	tickNumbers := make(map[uint64]bool)
	for _, tick := range writtenTicks {
		tickNumbers[tick.TickNumber] = true
	}

	for i := uint64(1); i <= 3; i++ {
		if !tickNumbers[i] {
			t.Errorf("Tick number %d not found in written ticks", i)
		}
	}
}

func TestPipeline_Run_BatchFlushing(t *testing.T) {
	// Create exactly 5 ticks with batch size of 2
	// Should result in: batch(2), batch(2), batch(1)
	var pbTicks []*pb.Tick
	for i := 1; i <= 5; i++ {
		pbTicks = append(pbTicks, &pb.Tick{
			TickNumber: uint64(i),
			Timestamp:  uint64(time.Now().UnixMicro()),
			VdfProof: &pb.VdfProof{
				Input:      "input",
				Output:     "output",
				Proof:      "proof",
				Iterations: 1000,
			},
			TransactionBatchHash: "hash",
			PreviousOutput:       "prev",
		})
	}

	reader := &mockStreamReader{ticks: pbTicks, tickDelay: 10 * time.Millisecond}
	parser := &mockParser{}
	writer := &mockWriter{}
	logger := zap.NewNop()

	config := PipelineConfig{
		BufferSize:    10,
		WorkerCount:   1, // Single worker for predictable batching
		BatchSize:     2, // Batch size of 2
		FlushInterval: 100 * time.Millisecond,
	}

	pipeline := NewPipeline(reader, parser, writer, logger, config)

	ctx, cancel := context.WithCancel(context.Background())

	// Run pipeline in background
	done := make(chan error)
	go func() {
		done <- pipeline.Run(ctx)
	}()

	// Wait for all ticks to be processed
	time.Sleep(300 * time.Millisecond)

	// Cancel and wait for shutdown
	cancel()
	err := <-done
	if err != nil {
		t.Fatalf("Pipeline.Run() error = %v", err)
	}

	writtenTicks := writer.GetWrittenTicks()
	if len(writtenTicks) != 5 {
		t.Errorf("Pipeline wrote %d ticks, want 5", len(writtenTicks))
	}

	batchSizes := writer.GetBatchSizes()
	totalBatches := len(batchSizes)
	if totalBatches < 2 {
		t.Errorf("Pipeline created %d batches, want at least 2", totalBatches)
	}
}

func TestPipeline_Run_ParseError(t *testing.T) {
	pbTicks := []*pb.Tick{
		{TickNumber: 1, Timestamp: uint64(time.Now().UnixMicro()), VdfProof: &pb.VdfProof{Input: "i", Output: "o", Proof: "p", Iterations: 1}, TransactionBatchHash: "h", PreviousOutput: "p"},
		{TickNumber: 2, Timestamp: uint64(time.Now().UnixMicro()), VdfProof: &pb.VdfProof{Input: "i", Output: "o", Proof: "p", Iterations: 1}, TransactionBatchHash: "h", PreviousOutput: "p"},
	}

	reader := &mockStreamReader{ticks: pbTicks}

	// Parser that fails on tick 1
	parser := &mockParser{
		parseFunc: func(tick *pb.Tick) (*domain.Tick, error) {
			if tick.TickNumber == 1 {
				return nil, errors.New("parse error")
			}
			return &domain.Tick{
				TickNumber: tick.TickNumber,
				Timestamp:  time.UnixMicro(int64(tick.Timestamp)),
				VDFProof:   domain.VDFProof{Input: "i", Output: "o", Proof: "p", Iterations: 1},
				BatchHash:  "h",
				PrevOutput: "p",
			}, nil
		},
	}

	writer := &mockWriter{}
	logger := zap.NewNop()

	config := DefaultPipelineConfig()
	pipeline := NewPipeline(reader, parser, writer, logger, config)

	ctx, cancel := context.WithCancel(context.Background())

	// Run pipeline in background
	done := make(chan error)
	go func() {
		done <- pipeline.Run(ctx)
	}()

	// Wait for all ticks to be processed
	time.Sleep(300 * time.Millisecond)

	// Cancel and wait for shutdown
	cancel()
	err := <-done
	if err != nil {
		t.Fatalf("Pipeline.Run() error = %v", err)
	}

	writtenTicks := writer.GetWrittenTicks()
	// Should only process tick 2 (tick 1 failed to parse)
	if len(writtenTicks) != 1 {
		t.Errorf("Pipeline wrote %d ticks, want 1 (tick 1 should have failed)", len(writtenTicks))
	}

	if len(writtenTicks) > 0 && writtenTicks[0].TickNumber != 2 {
		t.Errorf("Written tick has TickNumber = %v, want 2", writtenTicks[0].TickNumber)
	}
}

func TestPipeline_Close(t *testing.T) {
	reader := &mockStreamReader{}
	parser := &mockParser{}
	writer := &mockWriter{}
	logger := zap.NewNop()

	pipeline := NewPipeline(reader, parser, writer, logger, DefaultPipelineConfig())

	err := pipeline.Close()
	if err != nil {
		t.Errorf("Pipeline.Close() error = %v, want nil", err)
	}
}
