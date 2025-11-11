-- Migration: 002_add_market_prices_indexes
-- Description: Adds indexes for efficient candle queries on market_prices table
-- Author: Performance Optimization
-- Date: 2025-01-11

-- ==============================================================================
-- MARKET PRICES TABLE INDEXES
-- ==============================================================================
-- Critical indexes for candle query performance
-- These indexes enable fast time-range queries with market_id filtering
-- ==============================================================================

-- Composite index for market_id + timestamp range queries (most common pattern)
-- This index is optimized for: WHERE market_id = X AND ts >= Y AND ts <= Z
CREATE INDEX IF NOT EXISTS idx_market_prices_market_ts 
ON market_prices (market_id, ts DESC);

-- Covering index for time_bucket queries (includes price to avoid table lookups)
-- This enables index-only scans for candle aggregation queries
CREATE INDEX IF NOT EXISTS idx_market_prices_market_ts_price 
ON market_prices (market_id, ts DESC) 
INCLUDE (price);

-- ==============================================================================
-- PERFORMANCE NOTES
-- ==============================================================================
--
-- Query Pattern Optimized:
--   SELECT time_bucket(interval, ts), price
--   FROM market_prices
--   WHERE market_id = $1 AND ts >= $2 AND ts <= $3
--
-- Expected Performance Improvement:
--   - Before: 1-2 seconds (full table scan or inefficient index usage)
--   - After: <100ms (index-only scan with covering index)
--
-- Index Selection:
--   - idx_market_prices_market_ts: General purpose, smaller index
--   - idx_market_prices_market_ts_price: Covering index, faster but larger
--     PostgreSQL will use the covering index when available for index-only scans
--
-- Maintenance:
--   - These indexes will be automatically maintained by PostgreSQL
--   - Consider REINDEX if query performance degrades over time
--   - Monitor index size: SELECT pg_size_pretty(pg_relation_size('idx_market_prices_market_ts_price'));
--

