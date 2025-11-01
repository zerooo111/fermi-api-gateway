-- Migration: 001_create_tick_tables
-- Description: Creates TimescaleDB hypertables for high-throughput tick ingestion
-- Author: Claude Code
-- Date: 2025-01-01

-- ==============================================================================
-- TICKS TABLE (Hypertable)
-- ==============================================================================
-- Primary storage for tick metadata
-- Expected write rate: 10,000 ticks/second
-- Retention: 7 days detailed data, then compress/delete
-- ==============================================================================

CREATE TABLE IF NOT EXISTS ticks (
    -- Primary Key & Partitioning
    tick_number BIGINT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,

    -- Tick Metadata
    batch_hash TEXT NOT NULL,

    -- Constraints
    PRIMARY KEY (timestamp, tick_number),
    UNIQUE (tick_number)
);

-- Create hypertable partitioned by timestamp (1 day chunks for 7-day retention)
SELECT create_hypertable(
    'ticks',
    'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_ticks_tick_number ON ticks (tick_number DESC);
CREATE INDEX IF NOT EXISTS idx_ticks_timestamp ON ticks (timestamp DESC);

-- ==============================================================================
-- VDF PROOFS TABLE
-- ==============================================================================
-- 1:1 relationship with ticks
-- Separated for cleaner schema and potential future optimizations
-- ==============================================================================

CREATE TABLE IF NOT EXISTS vdf_proofs (
    tick_number BIGINT NOT NULL,

    -- VDF Proof Data (hex-encoded strings)
    input TEXT NOT NULL,
    output TEXT NOT NULL,
    proof TEXT NOT NULL,
    iterations BIGINT NOT NULL,

    -- Constraints
    PRIMARY KEY (tick_number),
    FOREIGN KEY (tick_number) REFERENCES ticks(tick_number) ON DELETE CASCADE
);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_vdf_proofs_tick_number ON vdf_proofs (tick_number);

-- ==============================================================================
-- TICK TRANSACTIONS TABLE (Hypertable)
-- ==============================================================================
-- Many:1 relationship with ticks
-- Each tick can contain 0-N transactions
-- Hypertable for efficient time-series queries
-- ==============================================================================

CREATE TABLE IF NOT EXISTS tick_transactions (
    -- Transaction Identity
    tx_hash TEXT NOT NULL,
    tx_id TEXT NOT NULL,

    -- Relationship to parent tick
    tick_number BIGINT NOT NULL,
    sequence_number BIGINT NOT NULL,

    -- Transaction Data
    payload BYTEA,
    signature BYTEA NOT NULL,
    public_key BYTEA NOT NULL,
    nonce BIGINT NOT NULL,

    -- Timestamps (consolidated)
    timestamp TIMESTAMPTZ NOT NULL, -- Client timestamp (from transaction)

    -- For hypertable partitioning (denormalized from ticks table)
    tick_timestamp TIMESTAMPTZ NOT NULL,

    -- Constraints
    PRIMARY KEY (tick_timestamp, tx_hash),
    UNIQUE (tx_hash),
    FOREIGN KEY (tick_number) REFERENCES ticks(tick_number) ON DELETE CASCADE
);

-- Create hypertable partitioned by tick_timestamp
SELECT create_hypertable(
    'tick_transactions',
    'tick_timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_tick_transactions_tx_hash ON tick_transactions (tx_hash);
CREATE INDEX IF NOT EXISTS idx_tick_transactions_tick_number ON tick_transactions (tick_number);
CREATE INDEX IF NOT EXISTS idx_tick_transactions_tick_timestamp ON tick_transactions (tick_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_tick_transactions_public_key ON tick_transactions (public_key);

-- ==============================================================================
-- TIMESCALEDB OPTIMIZATIONS
-- ==============================================================================

-- Compression Policy: Compress chunks older than 1 day
-- This significantly reduces storage costs while maintaining query performance
SELECT add_compression_policy(
    'ticks',
    INTERVAL '1 day',
    if_not_exists => TRUE
);

SELECT add_compression_policy(
    'tick_transactions',
    INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Retention Policy: Drop chunks older than 7 days
-- Aligns with your requirement for 7-day detailed data retention
SELECT add_retention_policy(
    'ticks',
    INTERVAL '7 days',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'tick_transactions',
    INTERVAL '7 days',
    if_not_exists => TRUE
);

-- ==============================================================================
-- PERFORMANCE STATISTICS
-- ==============================================================================
-- Enable statistics collection for query optimization
ALTER TABLE ticks SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'tick_number',
    timescaledb.compress_orderby = 'timestamp DESC'
);

ALTER TABLE tick_transactions SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'tick_number',
    timescaledb.compress_orderby = 'tick_timestamp DESC'
);

-- ==============================================================================
-- VIEWS FOR COMMON QUERIES
-- ==============================================================================

-- Complete tick view with VDF proof and transaction count
CREATE OR REPLACE VIEW v_ticks_complete AS
SELECT
    t.tick_number,
    t.timestamp,
    t.batch_hash,
    v.input AS vdf_input,
    v.output AS vdf_output,
    v.proof AS vdf_proof,
    v.iterations AS vdf_iterations,
    COUNT(tx.tx_hash) AS transaction_count
FROM ticks t
LEFT JOIN vdf_proofs v ON t.tick_number = v.tick_number
LEFT JOIN tick_transactions tx ON t.tick_number = tx.tick_number
GROUP BY
    t.tick_number,
    t.timestamp,
    t.batch_hash,
    v.input,
    v.output,
    v.proof,
    v.iterations;

-- Recent ticks view (last 1000)
CREATE OR REPLACE VIEW v_recent_ticks AS
SELECT * FROM v_ticks_complete
ORDER BY tick_number DESC
LIMIT 1000;

-- Transaction lookup view
CREATE OR REPLACE VIEW v_transaction_details AS
SELECT
    tx.tx_hash,
    tx.tx_id,
    tx.tick_number,
    tx.tick_timestamp,
    tx.sequence_number,
    tx.payload,
    tx.signature,
    tx.public_key,
    tx.nonce,
    tx.timestamp
FROM tick_transactions tx;

-- ==============================================================================
-- HELPER FUNCTIONS
-- ==============================================================================

-- Function to get tick by number with all details
CREATE OR REPLACE FUNCTION get_tick_by_number(p_tick_number BIGINT)
RETURNS TABLE (
    tick_number BIGINT,
    timestamp TIMESTAMPTZ,
    batch_hash TEXT,
    vdf_input TEXT,
    vdf_output TEXT,
    vdf_proof TEXT,
    vdf_iterations BIGINT,
    transaction_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT * FROM v_ticks_complete
    WHERE v_ticks_complete.tick_number = p_tick_number;
END;
$$ LANGUAGE plpgsql;

-- Function to get transaction by hash
CREATE OR REPLACE FUNCTION get_transaction_by_hash(p_tx_hash TEXT)
RETURNS TABLE (
    tx_hash TEXT,
    tx_id TEXT,
    tick_number BIGINT,
    tick_timestamp TIMESTAMPTZ,
    sequence_number BIGINT,
    payload BYTEA,
    signature BYTEA,
    public_key BYTEA,
    nonce BIGINT,
    timestamp TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT * FROM v_transaction_details
    WHERE v_transaction_details.tx_hash = p_tx_hash;
END;
$$ LANGUAGE plpgsql;

-- ==============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ==============================================================================

COMMENT ON TABLE ticks IS 'TimescaleDB hypertable storing tick metadata. Partitioned by timestamp with 1-day chunks. Compression after 1 day, retention 7 days.';
COMMENT ON TABLE vdf_proofs IS 'VDF proof data with 1:1 relationship to ticks. Separated for normalization.';
COMMENT ON TABLE tick_transactions IS 'TimescaleDB hypertable storing transactions within ticks. Many:1 relationship with ticks. Partitioned by tick_timestamp.';

COMMENT ON COLUMN ticks.tick_number IS 'Unique sequential tick identifier from Continuum sequencer';
COMMENT ON COLUMN ticks.timestamp IS 'Tick creation timestamp from sequencer (microseconds precision)';
COMMENT ON COLUMN ticks.batch_hash IS 'Hash of all transactions in this tick';

COMMENT ON COLUMN vdf_proofs.output IS 'VDF output for this tick (serves as previous_output for next tick in chain)';

COMMENT ON COLUMN tick_transactions.tx_hash IS 'Unique transaction hash (32-bit)';
COMMENT ON COLUMN tick_transactions.tick_timestamp IS 'Denormalized tick timestamp for hypertable partitioning';
COMMENT ON COLUMN tick_transactions.sequence_number IS 'Transaction order within the tick';
COMMENT ON COLUMN tick_transactions.timestamp IS 'Client-provided transaction timestamp';

-- ==============================================================================
-- PERFORMANCE NOTES
-- ==============================================================================
--
-- Expected Performance:
-- - INSERT: 50k-200k rows/sec using pgx COPY protocol
-- - SELECT by tick_number: <5ms (indexed)
-- - SELECT by tx_hash: <5ms (indexed)
-- - Range queries: <50ms for 1000 ticks
--
-- Storage Estimates (10k ticks/sec, 7-day retention):
-- - Ticks: ~60M rows × ~50 bytes = ~3 GB raw (~1 GB compressed)
-- - VDF Proofs: ~60M rows × ~500 bytes = ~30 GB raw (~10 GB compressed)
-- - Transactions (avg 10 tx/tick): ~600M rows × ~850 bytes = ~510 GB raw (~170 GB compressed)
-- - Total: ~543 GB raw, ~181 GB compressed after 1 day
--
-- Bandwidth Savings (optimizations):
-- - Removed previous_output: -40 bytes/tick = -2.4 GB/7 days
-- - Removed received_at: -8 bytes/tick = -480 MB/7 days
-- - Removed created_at from vdf_proofs: -8 bytes/tick = -480 MB/7 days
-- - Consolidated tx timestamps: -8 bytes/tx = -4.8 GB/7 days
-- - Total savings: ~8.2 GB raw (~2.7 GB compressed)
--
-- Optimization Tips:
-- 1. Use COPY protocol for bulk inserts (10-50x faster than INSERT)
-- 2. Batch writes: 250-500 rows per COPY operation
-- 3. Connection pool: 50-100 connections for write throughput
-- 4. Monitor chunk size: Keep chunks around 100-200 MB for optimal compression
-- 5. Vacuum regularly: VACUUM ANALYZE ticks, tick_transactions
--
-- ==============================================================================
