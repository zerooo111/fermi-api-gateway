package ingestion

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"go.uber.org/zap"
)

// Pipeline orchestrates the tick ingestion process:
// StreamReader → Parser → Worker Pool → Batch Accumulator → Writer
type Pipeline struct {
	reader      StreamReader
	parser      Parser
	writer      Writer
	logger      *zap.Logger
	metrics     *Metrics
	bufferSize  int
	workerCount int
	batchSize   int
	flushInterval time.Duration

	// Internal state
	wg     sync.WaitGroup
	stopCh chan struct{}
}

// PipelineConfig holds configuration for the pipeline.
type PipelineConfig struct {
	BufferSize    int           // Buffered channel capacity (default: 10000)
	WorkerCount   int           // Number of worker goroutines (default: 8)
	BatchSize     int           // Number of ticks per batch (default: 250)
	FlushInterval time.Duration // Max time before flushing batch (default: 100ms)
}

// DefaultPipelineConfig returns the default configuration.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BufferSize:    10000,
		WorkerCount:   8,
		BatchSize:     250,
		FlushInterval: 100 * time.Millisecond,
	}
}

// NewPipeline creates a new ingestion pipeline.
func NewPipeline(reader StreamReader, parser Parser, writer Writer, logger *zap.Logger, config PipelineConfig) *Pipeline {
	if config.BufferSize == 0 {
		config.BufferSize = DefaultPipelineConfig().BufferSize
	}
	if config.WorkerCount == 0 {
		config.WorkerCount = DefaultPipelineConfig().WorkerCount
	}
	if config.BatchSize == 0 {
		config.BatchSize = DefaultPipelineConfig().BatchSize
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = DefaultPipelineConfig().FlushInterval
	}

	return &Pipeline{
		reader:        reader,
		parser:        parser,
		writer:        writer,
		logger:        logger,
		metrics:       NewMetrics("tick_ingester"),
		bufferSize:    config.BufferSize,
		workerCount:   config.WorkerCount,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		stopCh:        make(chan struct{}),
	}
}

// Run starts the pipeline and blocks until context is canceled.
func (p *Pipeline) Run(ctx context.Context) error {
	p.logger.Info("Starting tick ingestion pipeline",
		zap.Int("buffer_size", p.bufferSize),
		zap.Int("worker_count", p.workerCount),
		zap.Int("batch_size", p.batchSize),
		zap.Duration("flush_interval", p.flushInterval),
	)

	// Create buffered channel for protobuf ticks
	pbTickCh := make(chan *pb.Tick, p.bufferSize)

	// Start reading from stream
	p.wg.Add(1)
	go p.readFromStream(ctx, pbTickCh)

	// Create parsed tick channel
	parsedTickCh := make(chan *domain.Tick, p.bufferSize)

	// Start parser workers
	p.wg.Add(1)
	go p.parseWorkers(ctx, pbTickCh, parsedTickCh)

	// Start batch writers
	p.wg.Add(p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		go p.batchWriter(ctx, i, parsedTickCh)
	}

	// Wait for context cancellation
	<-ctx.Done()
	p.logger.Info("Shutdown signal received, draining pipeline...")

	// Close stop channel to signal graceful shutdown
	close(p.stopCh)

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		p.logger.Info("Pipeline shut down gracefully")
	case <-time.After(30 * time.Second):
		p.logger.Warn("Pipeline shutdown timed out after 30s")
	}

	return nil
}

// readFromStream reads ticks from the stream reader and forwards to the channel.
func (p *Pipeline) readFromStream(ctx context.Context, tickCh chan<- *pb.Tick) {
	defer p.wg.Done()
	defer close(tickCh)

	pbTickCh, errCh := p.reader.Read(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case tick, ok := <-pbTickCh:
			if !ok {
				p.logger.Info("Stream closed")
				return
			}
			tickCh <- tick
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				p.logger.Error("Stream error", zap.Error(err))
			}
		}
	}
}

// parseWorkers runs multiple parser goroutines.
func (p *Pipeline) parseWorkers(ctx context.Context, pbTickCh <-chan *pb.Tick, parsedTickCh chan<- *domain.Tick) {
	defer p.wg.Done()
	defer close(parsedTickCh)

	var wg sync.WaitGroup
	numParsers := p.workerCount // Use same number as batch workers

	for i := 0; i < numParsers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			p.parseWorker(ctx, id, pbTickCh, parsedTickCh)
		}(i)
	}

	wg.Wait()
}

// parseWorker parses individual ticks.
func (p *Pipeline) parseWorker(ctx context.Context, id int, pbTickCh <-chan *pb.Tick, parsedTickCh chan<- *domain.Tick) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case pbTick, ok := <-pbTickCh:
			if !ok {
				return
			}

			tick, err := p.parser.Parse(pbTick)
			if err != nil {
				p.logger.Error("Failed to parse tick",
					zap.Int("worker_id", id),
					zap.Error(err),
				)
				p.metrics.RecordParseError()
				p.metrics.RecordTickError()
				continue
			}

			select {
			case parsedTickCh <- tick:
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			}
		}
	}
}

// batchWriter accumulates ticks and writes them in batches.
func (p *Pipeline) batchWriter(ctx context.Context, id int, tickCh <-chan *domain.Tick) {
	defer p.wg.Done()

	batch := make([]*domain.Tick, 0, p.batchSize)
	timer := time.NewTimer(p.flushInterval)
	defer timer.Stop()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		batchSize := len(batch)
		start := time.Now()
		if err := p.writer.WriteBatch(ctx, batch); err != nil {
			p.logger.Error("Failed to write batch",
				zap.Int("worker_id", id),
				zap.Int("batch_size", batchSize),
				zap.Error(err),
			)
			p.metrics.RecordWriteError()
		} else {
			duration := time.Since(start)
			p.logger.Debug("Wrote batch",
				zap.Int("worker_id", id),
				zap.Int("batch_size", batchSize),
				zap.Duration("duration", duration),
			)
			p.metrics.RecordTickSuccess(batchSize)
			p.metrics.ObserveBatchSize(batchSize)
			p.metrics.ObserveWriteDuration(duration.Seconds())
		}

		// Reset batch
		batch = batch[:0]
		timer.Reset(p.flushInterval)
	}

	for {
		select {
		case <-ctx.Done():
			// Flush remaining batch before exit
			flushBatch()
			return

		case <-p.stopCh:
			// Flush remaining batch before exit
			flushBatch()
			return

		case tick, ok := <-tickCh:
			if !ok {
				// Channel closed, flush and exit
				flushBatch()
				return
			}

			batch = append(batch, tick)

			// Flush if batch is full
			if len(batch) >= p.batchSize {
				flushBatch()
			}

		case <-timer.C:
			// Flush on timer
			flushBatch()
		}
	}
}

// Close gracefully shuts down the pipeline.
func (p *Pipeline) Close() error {
	p.logger.Info("Closing pipeline resources")

	var errs []error

	if err := p.reader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("reader close error: %w", err))
	}

	if err := p.writer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("writer close error: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("pipeline close errors: %v", errs)
	}

	return nil
}
