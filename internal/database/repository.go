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

// OHLCCandle represents an OHLC (Open, High, Low, Close) candle
type OHLCCandle struct {
	Timestamp time.Time `json:"t"` // timestamp
	Open      float64   `json:"o"` // open price
	High      float64   `json:"h"` // high price
	Low       float64   `json:"l"` // low price
	Close     float64   `json:"c"` // close price
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

// GetMarketCandles retrieves OHLC candles for a market within a time range
// This queries the market_prices table (or equivalent) using TimescaleDB's time_bucket function
// limit: maximum number of candles to return (Binance-style: default 500, max 1000)
func (r *Repository) GetMarketCandles(ctx context.Context, marketID string, timeframe string, from, to time.Time, limit int) ([]OHLCCandle, error) {
	// Map timeframe to PostgreSQL interval
	intervalMap := map[string]string{
		"1m":  "1 minute",
		"5m":  "5 minutes",
		"15m": "15 minutes",
		"1h":  "1 hour",
		"4h":  "4 hours",
		"1d":  "1 day",
	}

	interval, ok := intervalMap[timeframe]
	if !ok {
		return nil, fmt.Errorf("invalid timeframe: %s", timeframe)
	}

	// Highly optimized query using TimescaleDB's time_bucket
	// Optimized for performance with proper indexes (see schema/002_add_market_prices_indexes.sql)
	// Uses single CTE with efficient window functions for first/last values
	// 
	// Performance optimizations:
	// 1. Single CTE to minimize passes over data
	// 2. Window functions with proper partitioning (efficient with indexes)
	// 3. Direct aggregation in single GROUP BY
	// 4. Early filtering with WHERE clause
	//
	// Requires indexes:
	// - idx_market_prices_market_ts (market_id, ts DESC)
	// - idx_market_prices_market_ts_price (covering index for index-only scans)
	//
	// Note: Includes incomplete buckets (latest candle) so users can see current price.
	query := `
		WITH bucketed AS (
			SELECT time_bucket($1::interval, ts) AS bucket, price, ts
			FROM market_prices
			WHERE market_id = $2::uuid AND ts >= $3 AND ts <= $4
		),
		first_prices AS (
			SELECT DISTINCT ON (bucket) bucket, price AS open_price
			FROM bucketed
			ORDER BY bucket, ts ASC
		),
		last_prices AS (
			SELECT DISTINCT ON (bucket) bucket, price AS close_price
			FROM bucketed
			ORDER BY bucket, ts DESC
		)
		SELECT
			b.bucket,
			fp.open_price,
			MAX(b.price) AS high_price,
			MIN(b.price) AS low_price,
			lp.close_price
		FROM bucketed b
		INNER JOIN first_prices fp ON b.bucket = fp.bucket
		INNER JOIN last_prices lp ON b.bucket = lp.bucket
		GROUP BY b.bucket, fp.open_price, lp.close_price
		ORDER BY b.bucket DESC
		LIMIT $5
	`

	rows, err := r.db.QueryContext(ctx, query, interval, marketID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var candles []OHLCCandle
	for rows.Next() {
		var candle OHLCCandle
		err := rows.Scan(
			&candle.Timestamp,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		candles = append(candles, candle)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration failed: %w", err)
	}

	// Reverse to return chronological order (oldest to newest)
	// Query returns newest first (DESC), but API should return oldest first (ASC)
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	return candles, nil
}
