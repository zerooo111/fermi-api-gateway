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
			tx_hash, tx_id, payload, signature, public_key, nonce,
			client_timestamp, sequence_number, ingestion_timestamp,
			tick_number, created_at, metadata
		FROM transactions
		WHERE tx_hash = $1
		LIMIT 1
	`

	var tx Transaction
	err := r.db.QueryRowContext(ctx, query, txHash).Scan(
		&tx.TxHash,
		&tx.TxID,
		&tx.Payload,
		&tx.Signature,
		&tx.PublicKey,
		&tx.Nonce,
		&tx.ClientTimestamp,
		&tx.SequenceNumber,
		&tx.IngestionTimestamp,
		&tx.TickNumber,
		&tx.CreatedAt,
		&tx.Metadata,
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
			tx_hash, tx_id, payload, signature, public_key, nonce,
			client_timestamp, sequence_number, ingestion_timestamp,
			tick_number, created_at, metadata
		FROM transactions
		ORDER BY created_at DESC
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
		err := rows.Scan(
			&tx.TxHash,
			&tx.TxID,
			&tx.Payload,
			&tx.Signature,
			&tx.PublicKey,
			&tx.Nonce,
			&tx.ClientTimestamp,
			&tx.SequenceNumber,
			&tx.IngestionTimestamp,
			&tx.TickNumber,
			&tx.CreatedAt,
			&tx.Metadata,
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
