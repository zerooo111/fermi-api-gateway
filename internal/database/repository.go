package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Transaction represents a transaction stored in the database
type Transaction struct {
	TxHash             string          `json:"tx_hash"`
	TxID               string          `json:"tx_id"`
	Payload            []byte          `json:"payload"`
	Signature          []byte          `json:"signature"`
	PublicKey          []byte          `json:"public_key"`
	Nonce              uint64          `json:"nonce"`
	ClientTimestamp    uint64          `json:"client_timestamp"`
	SequenceNumber     uint64          `json:"sequence_number"`
	IngestionTimestamp uint64          `json:"ingestion_timestamp"`
	TickNumber         uint64          `json:"tick_number"`
	CreatedAt          time.Time       `json:"created_at"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
}

// Repository handles database operations for transactions
type Repository struct {
	db *DB
}

// NewRepository creates a new repository instance
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// GetTransaction retrieves a transaction by hash
func (r *Repository) GetTransaction(ctx context.Context, txHash string) (*Transaction, error) {
	query := `
		SELECT
			tick_number, sequence_number, tx_hash, tx_id, nonce,
			payload, timestamp_us, public_key, signature, ingestion_timestamp,
			processed_at
		FROM transactions
		WHERE tx_hash = $1
		LIMIT 1
	`

	var tx Transaction
	err := r.db.QueryRowContext(ctx, query, txHash).Scan(
		&tx.TickNumber,
		&tx.SequenceNumber,
		&tx.TxHash,
		&tx.TxID,
		&tx.Nonce,
		&tx.Payload,
		&tx.ClientTimestamp,
		&tx.PublicKey,
		&tx.Signature,
		&tx.IngestionTimestamp,
		&tx.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &tx, nil
}

// GetRecentTransactions retrieves the most recent transactions
func (r *Repository) GetRecentTransactions(ctx context.Context, limit int) ([]Transaction, error) {
	query := `
		SELECT
			tick_number, sequence_number, tx_hash, tx_id, nonce,
			payload, timestamp_us, public_key, signature, ingestion_timestamp,
			processed_at, payload_size, version
		FROM transactions
		ORDER BY processed_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var tx Transaction
		var payloadSize sql.NullInt64
		var version sql.NullInt64

		err := rows.Scan(
			&tx.TickNumber,
			&tx.SequenceNumber,
			&tx.TxHash,
			&tx.TxID,
			&tx.Nonce,
			&tx.Payload,
			&tx.ClientTimestamp,
			&tx.PublicKey,
			&tx.Signature,
			&tx.IngestionTimestamp,
			&tx.CreatedAt,
			&payloadSize,
			&version,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		transactions = append(transactions, tx)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration failed: %w", err)
	}

	return transactions, nil
}
