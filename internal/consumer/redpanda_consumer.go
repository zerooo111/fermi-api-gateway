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
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumeTopics(cfg.TicksTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()), // Start from beginning
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
	tc.logger.Info("Starting Kafka tick consumer",
		zap.Strings("brokers", tc.cfg.Brokers),
		zap.String("topic", tc.cfg.TicksTopic),
	)

	messagesProcessed := 0

	for {
		select {
		case <-ctx.Done():
			tc.logger.Info("Shutting down Kafka consumer")
			tc.client.Close()
			return nil
		default:
			// Poll for messages
			fetches := tc.client.PollFetches(ctx)
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					tc.logger.Error("Kafka fetch error", zap.Error(err.Err))
				}
				time.Sleep(1 * time.Second)
				continue
			}

			// Process fetched records
			iter := fetches.RecordIter()
			for !iter.Done() {
				record := iter.Next()
				messagesProcessed++

				// Parse protobuf message
				var pbTick pb.Tick
				if err := proto.Unmarshal(record.Value, &pbTick); err != nil {
					tc.logger.Error("Failed to unmarshal protobuf tick",
						zap.Error(err),
						zap.Int64("offset", record.Offset),
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

				// Add to ring buffer
				tc.ringBuffer.AddTick(tick)

				// Log stats every 1000 ticks
				if tick.TickNumber%1000 == 0 {
					tickCount, txCount := tc.ringBuffer.Stats()
					tc.logger.Info("Ring buffer stats",
						zap.Uint64("latest_tick", tick.TickNumber),
						zap.Int("buffer_ticks", tickCount),
						zap.Int("buffer_txs", txCount),
						zap.Int("total_processed", messagesProcessed),
					)
				}
			}
		}
	}
}

// Close gracefully closes the consumer
func (tc *TickConsumer) Close() error {
	tc.client.Close()
	return nil
}
