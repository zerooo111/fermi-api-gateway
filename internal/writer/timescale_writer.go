package writer

import (
	"context"
	"fmt"

	"github.com/fermilabs/fermi-api-gateway/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// TimescaleWriter writes ticks to TimescaleDB using pgx COPY protocol.
// This is 10-50x faster than individual INSERT statements.
type TimescaleWriter struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewTimescaleWriter creates a new TimescaleDB writer.
func NewTimescaleWriter(pool *pgxpool.Pool, logger *zap.Logger) *TimescaleWriter {
	return &TimescaleWriter{
		pool:   pool,
		logger: logger,
	}
}

// Write writes a single tick (uses WriteBatch internally).
func (w *TimescaleWriter) Write(ctx context.Context, tick *domain.Tick) error {
	return w.WriteBatch(ctx, []*domain.Tick{tick})
}

// WriteBatch writes multiple ticks using the COPY protocol for maximum performance.
// This method writes to 3 tables in a transaction:
// 1. ticks (tick metadata)
// 2. vdf_proofs (VDF proof data)
// 3. tick_transactions (transaction data)
func (w *TimescaleWriter) WriteBatch(ctx context.Context, ticks []*domain.Tick) error {
	if len(ticks) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if not committed

	// 1. Insert ticks using COPY
	if err := w.copyTicks(ctx, tx, ticks); err != nil {
		return fmt.Errorf("failed to copy ticks: %w", err)
	}

	// 2. Insert VDF proofs using COPY
	if err := w.copyVDFProofs(ctx, tx, ticks); err != nil {
		return fmt.Errorf("failed to copy vdf_proofs: %w", err)
	}

	// 3. Insert transactions using COPY
	if err := w.copyTransactions(ctx, tx, ticks); err != nil {
		return fmt.Errorf("failed to copy transactions: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	w.logger.Debug("Wrote batch to TimescaleDB",
		zap.Int("tick_count", len(ticks)),
		zap.Uint64("first_tick", ticks[0].TickNumber),
		zap.Uint64("last_tick", ticks[len(ticks)-1].TickNumber),
	)

	return nil
}

// copyTicks uses pgx COPY to bulk insert ticks.
func (w *TimescaleWriter) copyTicks(ctx context.Context, tx pgx.Tx, ticks []*domain.Tick) error {
	// COPY ticks (tick_number, timestamp, batch_hash) FROM STDIN
	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"ticks"},
		[]string{"tick_number", "timestamp", "batch_hash"},
		pgx.CopyFromSlice(len(ticks), func(i int) ([]any, error) {
			tick := ticks[i]
			return []any{
				tick.TickNumber,
				tick.Timestamp,
				tick.BatchHash,
			}, nil
		}),
	)

	return err
}

// copyVDFProofs uses pgx COPY to bulk insert VDF proofs.
func (w *TimescaleWriter) copyVDFProofs(ctx context.Context, tx pgx.Tx, ticks []*domain.Tick) error {
	// COPY vdf_proofs (tick_number, input, output, proof, iterations) FROM STDIN
	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"vdf_proofs"},
		[]string{"tick_number", "input", "output", "proof", "iterations"},
		pgx.CopyFromSlice(len(ticks), func(i int) ([]any, error) {
			tick := ticks[i]
			return []any{
				tick.TickNumber,
				tick.VDFProof.Input,
				tick.VDFProof.Output,
				tick.VDFProof.Proof,
				tick.VDFProof.Iterations,
			}, nil
		}),
	)

	return err
}

// copyTransactions uses pgx COPY to bulk insert all transactions from all ticks.
func (w *TimescaleWriter) copyTransactions(ctx context.Context, tx pgx.Tx, ticks []*domain.Tick) error {
	// Count total transactions
	totalTxs := 0
	for _, tick := range ticks {
		totalTxs += len(tick.Transactions)
	}

	if totalTxs == 0 {
		return nil // No transactions to insert
	}

	// Flatten transactions from all ticks
	// COPY tick_transactions (tx_hash, tx_id, tick_number, sequence_number, payload, signature, public_key, nonce, timestamp, tick_timestamp) FROM STDIN
	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"tick_transactions"},
		[]string{"tx_hash", "tx_id", "tick_number", "sequence_number", "payload", "signature", "public_key", "nonce", "timestamp", "tick_timestamp"},
		pgx.CopyFromSlice(totalTxs, func(i int) ([]any, error) {
			// Find which tick and transaction this index corresponds to
			currentIndex := 0
			for _, tick := range ticks {
				if currentIndex+len(tick.Transactions) > i {
					// Transaction is in this tick
					txInTick := i - currentIndex
					tx := tick.Transactions[txInTick]
					return []any{
						tx.TxHash,
						tx.TxID,
						tick.TickNumber,
						tx.SequenceNumber,
						tx.Payload,
						tx.Signature,
						tx.PublicKey,
						tx.Nonce,
						tx.ClientTimestamp,
						tick.Timestamp, // tick_timestamp (denormalized)
					}, nil
				}
				currentIndex += len(tick.Transactions)
			}
			return nil, fmt.Errorf("transaction index out of range: %d", i)
		}),
	)

	return err
}

// Close closes the database connection pool.
func (w *TimescaleWriter) Close() error {
	w.pool.Close()
	w.logger.Info("TimescaleDB connection pool closed")
	return nil
}
