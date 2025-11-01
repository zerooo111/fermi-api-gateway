# TimescaleDB Schema for Tick Ingestion

## Quick Start

```bash
# Apply schema
psql $DATABASE_URL -f schema/001_create_tables.sql

# Rollback (if needed)
psql $DATABASE_URL -f schema/001_rollback.sql
```

## Database Connection

Use your TimescaleDB cloud credentials:

```bash
export DATABASE_URL="postgres://tsdbadmin:fermilabstsdb@q3qz8ken0n.uwi84b59od.tsdb.cloud.timescale.com:36864/tsdb?sslmode=require"
```

## Schema Overview

### Tables Created

1. **`ticks`** (Hypertable)
   - Primary tick metadata
   - Partitioned by timestamp (1-day chunks)
   - Compression after 1 day, retention 7 days

2. **`vdf_proofs`**
   - VDF proof data (1:1 with ticks)
   - `output` field serves as `previous_output` for next tick

3. **`tick_transactions`** (Hypertable)
   - Transaction data (many:1 with ticks)
   - Partitioned by tick_timestamp
   - Compression after 1 day, retention 7 days

### Views Created

- **`v_ticks_complete`** - Full tick view with VDF proof and tx count
- **`v_recent_ticks`** - Last 1000 ticks
- **`v_transaction_details`** - Transaction lookup with context

### Helper Functions

- **`get_tick_by_number(tick_number)`** - Fetch complete tick
- **`get_transaction_by_hash(tx_hash)`** - Fetch transaction details

## Performance Settings

- **Write throughput**: 50k-200k rows/sec (with pgx COPY protocol)
- **Storage**: ~543 GB raw → ~181 GB compressed (7-day retention @ 10k ticks/sec)
- **Query latency**: <5ms for point lookups, <50ms for range queries

## Monitoring Queries

```sql
-- Check hypertable chunks
SELECT * FROM timescaledb_information.chunks
WHERE hypertable_name IN ('ticks', 'tick_transactions');

-- Check compression stats
SELECT * FROM timescaledb_information.compression_settings;

-- Check recent ticks
SELECT * FROM v_recent_ticks LIMIT 10;

-- Check storage size
SELECT
    hypertable_name,
    pg_size_pretty(total_bytes) AS total_size,
    pg_size_pretty(compressed_total_bytes) AS compressed_size
FROM timescaledb_information.hypertables;
```

## Notes

- Previous tick's VDF output: Query `vdf_proofs` with `tick_number - 1`
- Transaction timestamp is client-provided (single consolidated field)
- No ingestion timestamps stored (tracked in metrics instead)
