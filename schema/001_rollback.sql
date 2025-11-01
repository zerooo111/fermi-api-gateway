-- Rollback Migration: 001_create_tick_tables
-- Description: Drops all tick-related tables, views, and functions
-- Author: Claude Code
-- Date: 2025-01-01

-- ==============================================================================
-- DROP VIEWS
-- ==============================================================================

DROP VIEW IF EXISTS v_recent_ticks CASCADE;
DROP VIEW IF EXISTS v_transaction_details CASCADE;
DROP VIEW IF EXISTS v_ticks_complete CASCADE;

-- ==============================================================================
-- DROP FUNCTIONS
-- ==============================================================================

DROP FUNCTION IF EXISTS get_tick_by_number(BIGINT);
DROP FUNCTION IF EXISTS get_transaction_by_hash(TEXT);

-- ==============================================================================
-- DROP TABLES (in reverse dependency order)
-- ==============================================================================

-- Drop child tables first
DROP TABLE IF EXISTS tick_transactions CASCADE;
DROP TABLE IF EXISTS vdf_proofs CASCADE;

-- Drop parent table last
DROP TABLE IF EXISTS ticks CASCADE;

-- ==============================================================================
-- NOTES
-- ==============================================================================
-- This rollback removes all tick ingestion data and schema.
-- TimescaleDB will automatically clean up hypertable metadata.
-- Compression and retention policies are automatically removed with the tables.
-- ==============================================================================
