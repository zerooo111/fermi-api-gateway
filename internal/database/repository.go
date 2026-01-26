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

// VDFProof represents VDF proof data for a tick
type VDFProof struct {
	Input      string `json:"input"`
	Output     string `json:"output"`
	Proof      string `json:"proof"`
	Iterations uint64 `json:"iterations"`
}

// Tick represents a tick from the Continuum sequencer
type Tick struct {
	TickNumber           uint64        `json:"tick_number"`
	Timestamp            uint64        `json:"timestamp"`
	Time                 time.Time     `json:"time"`
	VDFProof             *VDFProof     `json:"vdf_proof,omitempty"`
	TransactionCount     int           `json:"transaction_count"`
	TransactionBatchHash string        `json:"transaction_batch_hash"`
	IngestedAt           time.Time     `json:"ingested_at"`
	Transactions         []Transaction `json:"transactions,omitempty"`
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

// Health checks database connectivity (implements health.HealthChecker)
func (r *Repository) Health(ctx context.Context) error {
	if r.db == nil {
		return fmt.Errorf("database not configured")
	}
	return r.db.Health(ctx)
}

// GetTransaction retrieves a transaction by hash
// Supports both full hash and 8-character short hash (prefix search)
func (r *Repository) GetTransaction(ctx context.Context, txHash string) (*Transaction, error) {
	var query string
	var arg interface{}

	if len(txHash) == 8 {
		// Short hash: search for transactions starting with this prefix
		query = `
			SELECT
				tick_number, sequence_number, tx_hash, tx_id, nonce,
				payload, timestamp, public_key, signature, ingestion_timestamp,
				ingested_at
			FROM transactions
			WHERE tx_hash LIKE $1
			LIMIT 1
		`
		arg = txHash + "%"
	} else {
		// Full hash: exact match
		query = `
			SELECT
				tick_number, sequence_number, tx_hash, tx_id, nonce,
				payload, timestamp, public_key, signature, ingestion_timestamp,
				ingested_at
			FROM transactions
			WHERE tx_hash = $1
			LIMIT 1
		`
		arg = txHash
	}

	var tx Transaction
	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&tx.TickNumber,
		&tx.SequenceNumber,
		&tx.TxHash,
		&tx.TxID,
		&tx.Nonce,
		&tx.Payload,
		&tx.ClientTimestamp, // timestamp stored as int64 (microseconds)
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
			payload, timestamp, public_key, signature, ingestion_timestamp,
			ingested_at
		FROM transactions
		ORDER BY sequence_number DESC
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
			&tx.TickNumber,
			&tx.SequenceNumber,
			&tx.TxHash,
			&tx.TxID,
			&tx.Nonce,
			&tx.Payload,
			&tx.ClientTimestamp, // timestamp stored as int64 (microseconds)
			&tx.PublicKey,
			&tx.Signature,
			&tx.IngestionTimestamp,
			&tx.CreatedAt,
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

// GetTickByNumber retrieves a tick by its tick number, including VDF proof data
// Actual database schema: ticks(tick_number, timestamp_us, vdf_input, vdf_output, vdf_proof, vdf_iterations, transaction_batch_hash, tx_count, processed_at)
func (r *Repository) GetTickByNumber(ctx context.Context, tickNumber uint64) (*Tick, error) {
	query := `
		SELECT
			tick_number, timestamp_us,
			vdf_input, vdf_output, vdf_proof, vdf_iterations,
			tx_count, transaction_batch_hash,
			processed_at
		FROM ticks
		WHERE tick_number = $1
		LIMIT 1
	`

	var tick Tick
	var vdfInput, vdfOutput, vdfProof sql.NullString
	var vdfIterations sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, tickNumber).Scan(
		&tick.TickNumber,
		&tick.Timestamp, // timestamp_us is bigint, directly into uint64
		&vdfInput,
		&vdfOutput,
		&vdfProof,
		&vdfIterations,
		&tick.TransactionCount,
		&tick.TransactionBatchHash,
		&tick.IngestedAt, // processed_at -> IngestedAt
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tick not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Set Time from timestamp_us (microseconds to time.Time)
	tick.Time = time.UnixMicro(int64(tick.Timestamp))

	// Populate VDF proof if available
	if vdfInput.Valid && vdfOutput.Valid && vdfProof.Valid {
		tick.VDFProof = &VDFProof{
			Input:      vdfInput.String,
			Output:     vdfOutput.String,
			Proof:      vdfProof.String,
			Iterations: uint64(vdfIterations.Int64),
		}
	}

	// Fetch transactions for this tick
	if tick.TransactionCount > 0 {
		txns, err := r.GetTransactionsByTickNumber(ctx, tickNumber)
		if err == nil {
			tick.Transactions = txns
		}
		// If fetching transactions fails, we still return the tick without transactions
	}

	return &tick, nil
}

// GetRecentTicks retrieves the most recent ticks in descending order by tick number
func (r *Repository) GetRecentTicks(ctx context.Context, limit int) ([]Tick, error) {
	query := `
		SELECT
			tick_number, timestamp_us,
			vdf_input, vdf_output, vdf_proof, vdf_iterations,
			tx_count, transaction_batch_hash,
			processed_at
		FROM ticks
		ORDER BY tick_number DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var ticks []Tick
	for rows.Next() {
		var tick Tick
		var vdfInput, vdfOutput, vdfProof sql.NullString
		var vdfIterations sql.NullInt64

		err := rows.Scan(
			&tick.TickNumber,
			&tick.Timestamp,
			&vdfInput,
			&vdfOutput,
			&vdfProof,
			&vdfIterations,
			&tick.TransactionCount,
			&tick.TransactionBatchHash,
			&tick.IngestedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		// Set Time from timestamp_us
		tick.Time = time.UnixMicro(int64(tick.Timestamp))

		// Populate VDF proof if available
		if vdfInput.Valid && vdfOutput.Valid && vdfProof.Valid {
			tick.VDFProof = &VDFProof{
				Input:      vdfInput.String,
				Output:     vdfOutput.String,
				Proof:      vdfProof.String,
				Iterations: uint64(vdfIterations.Int64),
			}
		}

		ticks = append(ticks, tick)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration failed: %w", err)
	}

	return ticks, nil
}

// GetTransactionsByTickNumber retrieves all transactions for a given tick number
func (r *Repository) GetTransactionsByTickNumber(ctx context.Context, tickNumber uint64) ([]Transaction, error) {
	query := `
		SELECT
			tick_number, sequence_number, tx_hash, tx_id, nonce,
			payload, timestamp, public_key, signature, ingestion_timestamp,
			ingested_at
		FROM transactions
		WHERE tick_number = $1
		ORDER BY sequence_number ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tickNumber)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var tx Transaction
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

// GetTransactionByTxID retrieves a transaction by its tx_id field
func (r *Repository) GetTransactionByTxID(ctx context.Context, txID string) (*Transaction, error) {
	query := `
		SELECT
			tick_number, sequence_number, tx_hash, tx_id, nonce,
			payload, timestamp, public_key, signature, ingestion_timestamp,
			ingested_at
		FROM transactions
		WHERE tx_id = $1
		LIMIT 1
	`

	var tx Transaction
	err := r.db.QueryRowContext(ctx, query, txID).Scan(
		&tx.TickNumber,
		&tx.SequenceNumber,
		&tx.TxHash,
		&tx.TxID,
		&tx.Nonce,
		&tx.Payload,
		&tx.ClientTimestamp, // timestamp stored as int64 (microseconds)
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

	// Optimized single-pass query using window functions
	// Previous query used 3 CTEs with DISTINCT ON which caused multiple table scans
	//
	// Performance optimizations:
	// 1. Single pass over data with window functions (FIRST_VALUE/LAST_VALUE)
	// 2. All aggregations (open, high, low, close) computed in one GROUP BY
	// 3. Uses covering index for index-only scans
	//
	// Expected performance: <500ms for 30-day 1h query (vs 6-10s before)
	query := `
		SELECT
			time_bucket($1::interval, ts) AS bucket,
			(array_agg(price ORDER BY ts ASC))[1] AS open_price,
			MAX(price) AS high_price,
			MIN(price) AS low_price,
			(array_agg(price ORDER BY ts DESC))[1] AS close_price
		FROM market_prices
		WHERE market_id = $2::uuid AND ts >= $3 AND ts <= $4
		GROUP BY bucket
		ORDER BY bucket DESC
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

// InsertMarketPrice inserts a market price into the database
// Uses ON CONFLICT DO NOTHING to handle duplicate timestamps
func (r *Repository) InsertMarketPrice(ctx context.Context, marketID string, ts time.Time, price float64) error {
	query := `
		INSERT INTO market_prices (market_id, ts, price)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, marketID, ts, price)
	if err != nil {
		return fmt.Errorf("failed to insert market price: %w", err)
	}

	return nil
}
