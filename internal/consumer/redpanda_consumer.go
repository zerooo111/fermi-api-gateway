package consumer

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/config"
	"github.com/fermilabs/fermi-api-gateway/internal/parser"
	"github.com/fermilabs/fermi-api-gateway/internal/stream"
	pb "github.com/fermilabs/fermi-api-gateway/proto/continuumv1"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// TickConsumer consumes tick messages from Redpanda/Kafka and updates the ring buffer
type TickConsumer struct {
	client     *kgo.Client
	ringBuffer *stream.RingBuffer
	parser     *parser.ProtobufParser
	logger     *zap.Logger
	cfg        *config.RedpandaConfig
}

// NewTickConsumer creates a new Kafka consumer for ticks using franz-go
func NewTickConsumer(cfg *config.RedpandaConfig, ringBuffer *stream.RingBuffer, logger *zap.Logger) (*TickConsumer, error) {
	// Build client options
	// NOTE: We use a unique consumer group per instance + timestamp to ensure
	// we always start from the latest message, not from where we left off.
	// This is important for a real-time explorer that should always show current data.
	consumerGroup := fmt.Sprintf("fermi-api-gateway-explorer-%d", time.Now().Unix())

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumeTopics(cfg.TicksTopic),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()), // Start from latest for real-time explorer
		kgo.FetchMaxBytes(10 * 1024 * 1024),             // 10MB batch for high throughput
		kgo.FetchMaxWait(100 * time.Millisecond),        // Low latency for real-time
		kgo.FetchMinBytes(1),                            // Don't wait for data to accumulate
		kgo.RequestRetries(3),                           // Retry failed requests
		kgo.RequestTimeoutOverhead(5 * time.Second),     // Request timeout
		kgo.SessionTimeout(30 * time.Second),            // Consumer group session timeout
		kgo.RebalanceTimeout(60 * time.Second),          // Rebalance timeout
		kgo.DisableAutoCommit(),                         // Manual offset commits for better control
	}

	// Initialize TLS
	opts = append(opts, kgo.DialTLSConfig(new(tls.Config)))

	// Initialize SASL/SCRAM-SHA-256
	opts = append(opts, kgo.SASL(scram.Auth{
		User: cfg.SASLUsername,
		Pass: cfg.SASLPassword,
	}.AsSha256Mechanism()))

	// Create client
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	return &TickConsumer{
		client:     client,
		ringBuffer: ringBuffer,
		parser:     parser.NewProtobufParser(),
		logger:     logger,
		cfg:        cfg,
	}, nil
}

// Start begins consuming messages from Kafka
func (tc *TickConsumer) Start(ctx context.Context) error {
	tc.logger.Info("Starting Kafka tick consumer (always reading from latest offset)",
		zap.Strings("brokers", tc.cfg.Brokers),
		zap.String("topic", tc.cfg.TicksTopic),
	)

	var (
		messagesProcessed uint64
		batchCount        uint64
		lastLogTime       = time.Now()
		ticksPerSecond    uint64
	)

	for {
		select {
		case <-ctx.Done():
			tc.logger.Info("Shutting down Kafka consumer")
			tc.client.Close()
			return nil
		default:
			// Poll for messages (non-blocking with context)
			fetches := tc.client.PollFetches(ctx)
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					tc.logger.Error("Kafka fetch error",
						zap.Error(err.Err),
						zap.String("topic", err.Topic),
						zap.Int32("partition", err.Partition),
					)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process fetched records in batch
			batchSize := 0
			iter := fetches.RecordIter()

			for !iter.Done() {
				record := iter.Next()
				batchSize++
				messagesProcessed++
				ticksPerSecond++

				// Parse protobuf message
				var pbTick pb.Tick
				if err := proto.Unmarshal(record.Value, &pbTick); err != nil {
					tc.logger.Error("Failed to unmarshal protobuf tick",
						zap.Error(err),
						zap.Int64("offset", record.Offset),
						zap.Int32("partition", record.Partition),
					)
					continue
				}

				// Convert protobuf to domain model
				tick, err := tc.parser.Parse(&pbTick)
				if err != nil {
					tc.logger.Error("Failed to parse tick to domain model",
						zap.Error(err),
						zap.Uint64("tick_number", pbTick.TickNumber),
					)
					continue
				}

				// Add to ring buffer (thread-safe)
				tc.ringBuffer.AddTick(tick)
			}

			// Commit offsets after processing batch
			if batchSize > 0 {
				batchCount++
				if err := tc.client.CommitUncommittedOffsets(ctx); err != nil {
					tc.logger.Warn("Failed to commit offsets", zap.Error(err))
				}
			}

			// Log throughput stats every second
			if time.Since(lastLogTime) >= 1*time.Second {
				tickCount, txCount := tc.ringBuffer.Stats()
				tc.logger.Info("Consumer throughput",
					zap.Uint64("ticks_per_second", ticksPerSecond),
					zap.Uint64("total_ticks_processed", messagesProcessed),
					zap.Uint64("batches_processed", batchCount),
					zap.Int("ring_buffer_ticks", tickCount),
					zap.Int("ring_buffer_txs", txCount),
				)
				ticksPerSecond = 0
				lastLogTime = time.Now()
			}
		}
	}
}

// Close gracefully closes the consumer
func (tc *TickConsumer) Close() error {
	tc.client.Close()
	return nil
}
