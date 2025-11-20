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
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// TickConsumer consumes tick messages from Redpanda/Kafka and updates the ring buffer
type TickConsumer struct {
	reader     *kafka.Reader
	ringBuffer *stream.RingBuffer
	parser     *parser.ProtobufParser
	logger     *zap.Logger
	cfg        *config.RedpandaConfig
}

// NewTickConsumer creates a new Kafka consumer for ticks
func NewTickConsumer(cfg *config.RedpandaConfig, ringBuffer *stream.RingBuffer, logger *zap.Logger) (*TickConsumer, error) {
	// Setup SASL mechanism
	mechanism, err := scram.Mechanism(scram.SHA256, cfg.SASLUsername, cfg.SASLPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create SASL mechanism: %w", err)
	}

	// Create Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: cfg.Brokers,
		Topic:   cfg.TicksTopic,
		GroupID: "api-gateway-sse",
		Dialer: &kafka.Dialer{
			Timeout:       10 * time.Second,
			DualStack:     true,
			SASLMechanism: mechanism,
			TLS:           &tls.Config{},
		},
		MinBytes:        1,           // Fetch immediately when any data is available
		MaxBytes:        10e6,        // 10MB max per fetch
		MaxWait:         100 * time.Millisecond,
		ReadBackoffMin:  100 * time.Millisecond,
		ReadBackoffMax:  1 * time.Second,
		StartOffset:     kafka.LastOffset, // Start from latest (don't replay history)
		CommitInterval:  1 * time.Second,  // Auto-commit offsets every second
	})

	return &TickConsumer{
		reader:     reader,
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

	for {
		select {
		case <-ctx.Done():
			tc.logger.Info("Shutting down Kafka consumer")
			return tc.reader.Close()
		default:
			// Read next message
			msg, err := tc.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context cancelled, exit gracefully
					return nil
				}
				tc.logger.Error("Failed to read Kafka message", zap.Error(err))
				time.Sleep(1 * time.Second) // Backoff on error
				continue
			}

			// Parse protobuf message
			var pbTick pb.Tick
			if err := proto.Unmarshal(msg.Value, &pbTick); err != nil {
				tc.logger.Error("Failed to unmarshal protobuf tick",
					zap.Error(err),
					zap.Int64("offset", msg.Offset),
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
				)
			}
		}
	}
}

// Close gracefully closes the consumer
func (tc *TickConsumer) Close() error {
	return tc.reader.Close()
}
