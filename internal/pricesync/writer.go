package pricesync

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Writer handles writing market prices to the database
type Writer struct {
	db         *sql.DB
	insertStmt *sql.Stmt

	// Metrics
	successCount int64
	errorCount   int64
	skipCount    int64
}

// NewWriter creates a new Writer instance with prepared statement
func NewWriter(db *sql.DB) (*Writer, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}

	stmt, err := db.Prepare("INSERT INTO market_prices (market_id, ts, price) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING")
	if err != nil {
		return nil, fmt.Errorf("prepare insert failed: %w", err)
	}

	return &Writer{
		db:         db,
		insertStmt: stmt,
	}, nil
}

// WritePrice writes a single market price to the database
func (w *Writer) WritePrice(ctx context.Context, marketID string, ts time.Time, price float64) error {
	ctxDB, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	_, err := w.insertStmt.ExecContext(ctxDB, marketID, ts, price)
	if err != nil {
		w.errorCount++
		return fmt.Errorf("database insert failed | Market=%s | Price=%.8f | TS=%s | Error: %w",
			marketID, price, ts.Format(time.RFC3339), err)
	}

	w.successCount++
	return nil
}

// WritePrices writes multiple market prices in a batch
func (w *Writer) WritePrices(ctx context.Context, prices []MarketPrice) (inserted int, errors int) {
	for _, p := range prices {
		if err := w.WritePrice(ctx, p.MarketID, p.Timestamp, p.Price); err != nil {
			errors++
		} else {
			inserted++
		}
	}
	return inserted, errors
}

// GetMetrics returns current metrics
func (w *Writer) GetMetrics() (success, errors, skipped int64) {
	return w.successCount, w.errorCount, w.skipCount
}

// Close closes the prepared statement
func (w *Writer) Close() error {
	if w.insertStmt != nil {
		return w.insertStmt.Close()
	}
	return nil
}

// Reconnect attempts to reconnect and re-prepare the statement
func (w *Writer) Reconnect(ctx context.Context) error {
	// Test database connection
	pingTimeout := 2 * time.Second
	ctxPing, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := w.db.PingContext(ctxPing); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Re-prepare the statement
	stmt, err := w.db.Prepare("INSERT INTO market_prices (market_id, ts, price) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING")
	if err != nil {
		return fmt.Errorf("failed to re-prepare statement: %w", err)
	}

	// Close old statement and replace
	if w.insertStmt != nil {
		_ = w.insertStmt.Close()
	}
	w.insertStmt = stmt

	return nil
}
