# API Migration Guide

This document outlines the API changes for the Continuum Explorer integration. Frontend developers should review this guide to update their applications accordingly.

## Migration Date
**January 2026**

## Overview

The Continuum API endpoints have been migrated from the legacy database-backed implementation to the new **Continuum Explorer API**. This provides better performance, richer data, and new endpoints.

---

## Breaking Changes

### 1. Response Format Changes for Ticks

#### `GET /api/v1/continuum/tick/recent`

**Old Response:**
```json
{
  "ticks": [
    {
      "tick_number": 12345,
      "timestamp": "2026-01-10T12:00:00Z",
      "transaction_count": 5
    }
  ]
}
```

**New Response:**
```json
{
  "ticks": [
    {
      "tick_number": 51587251390,
      "timestamp": 1768032065977135,
      "transaction_count": 1,
      "transaction_batch_hash": "4dc64e7bcf83859ce4e4c6f595294586ae433ed4dcc762a3d7a5df2853532598"
    }
  ],
  "count": 20,
  "latest_tick_number": 51587251685
}
```

**Changes:**
| Field | Change |
|-------|--------|
| `timestamp` | Now returns **Unix microseconds** (integer) instead of ISO 8601 string |
| `transaction_batch_hash` | **New field** - Hash of the transaction batch |
| `count` | **New field** - Number of ticks returned |
| `latest_tick_number` | **New field** - Latest tick number in the system |

---

### 2. Response Format Changes for Transactions

#### `GET /api/v1/continuum/txn/recent`

**Old Response:**
```json
{
  "transactions": [
    {
      "tx_id": "frm_order_123",
      "tick_number": 12345,
      "timestamp": "2026-01-10T12:00:00Z"
    }
  ]
}
```

**New Response:**
```json
{
  "transactions": [
    {
      "tx_hash": "7ab375cb7454de32835eb523a2bc5bc83507850565bc3930cd21501e79df83a9",
      "tick_number": 51587321692,
      "sequence_number": 41164830,
      "tx_id": "frm_order_41817751_1768032072395949",
      "timestamp": 1768032072395949,
      "payload_size": 625
    }
  ],
  "count": 20,
  "latest_tick_number": 51587321700
}
```

**Changes:**
| Field | Change |
|-------|--------|
| `timestamp` | Now returns **Unix microseconds** (integer) instead of ISO 8601 string |
| `tx_hash` | **New field** - Transaction hash (use this as primary identifier) |
| `sequence_number` | **New field** - Global sequence number |
| `payload_size` | **New field** - Size of transaction payload in bytes |
| `count` | **New field** - Number of transactions returned |
| `latest_tick_number` | **New field** - Latest tick number |

---

### 3. Transaction Detail Response

#### `GET /api/v1/continuum/txn/{txHash}`

**Old Response:**
```json
{
  "tx_id": "frm_order_123",
  "tick_number": 12345,
  "payload": "base64_encoded_data"
}
```

**New Response:**
```json
{
  "success": true,
  "data": {
    "tx_hash": "7ab375cb7454de32835eb523a2bc5bc83507850565bc3930cd21501e79df83a9",
    "tick_number": 51587321692,
    "sequence_number": 41164830,
    "ingestion_timestamp": 1768032072504735,
    "tx_id": "frm_order_41817751_1768032072395949",
    "nonce": 41817751,
    "timestamp": 1768032072395949,
    "payload": "RlJNX3YxLjA6eyJpbnRlbnQiOi...",
    "payload_text": "FRM_v1.0:{\"intent\":{...}}",
    "payload_size": 625,
    "signature": "1b49a8e4fb928152...",
    "public_key": "35bdfe06777c5b93..."
  }
}
```

**Changes:**
| Field | Change |
|-------|--------|
| Response wrapped in `success` and `data` | **Structure change** |
| `tx_hash` | **New field** - Transaction hash |
| `ingestion_timestamp` | **New field** - When the transaction was indexed |
| `nonce` | **New field** - Transaction nonce |
| `payload_text` | **New field** - Decoded payload as text |
| `signature` | **New field** - Transaction signature |
| `public_key` | **New field** - Signer's public key |

---

### 4. Tick Detail Response

#### `GET /api/v1/continuum/tick/{tickNumber}`

**New Response:**
```json
{
  "success": true,
  "data": {
    "tick_number": 51587251390,
    "timestamp": 1768032065977135,
    "transaction_count": 1,
    "transaction_batch_hash": "4dc64e7bcf...",
    "vdf_proof": {
      "output": "base64_encoded...",
      "proof": "base64_encoded..."
    },
    "transactions": [
      {
        "tx_hash": "abc123...",
        "tx_id": "frm_order_123",
        "sequence_number": 12345
      }
    ]
  }
}
```

**Changes:**
| Field | Change |
|-------|--------|
| Response wrapped in `success` and `data` | **Structure change** |
| `vdf_proof` | **New field** - VDF proof object with `output` and `proof` |
| `transactions` | **New field** - Array of transactions in this tick |

---

## New Endpoints

### 1. Health Check
```
GET /api/v1/continuum/health
```

**Response:**
```json
{
  "status": "healthy",
  "db_healthy": true,
  "latest_tick": 51582539683
}
```

### 2. Service Info
```
GET /api/v1/continuum/info
```

**Response:**
```json
{
  "service": "Continuum Explorer API",
  "version": "1.0.0",
  "endpoints": {
    "health": "/health",
    "stats": "/api/v1/stats",
    "recent_ticks": "/api/v1/ticks/recent?limit=20",
    "get_tick": "/api/v1/ticks/:tick_number",
    "recent_transactions": "/api/v1/transactions/recent?limit=20",
    "get_transaction": "/api/v1/transactions/:tx_hash"
  }
}
```

### 3. Statistics
```
GET /api/v1/continuum/stats
```

**Response:**
```json
{
  "ticks_indexed": 54019,
  "transactions_indexed": 54133,
  "empty_ticks_skipped": 35693588,
  "latest_tick_number": 51583193962,
  "memory_ticks_count": 1000,
  "memory_txs_count": 10000,
  "tick_hit_rate": 0.0,
  "tx_hit_rate": 0.0,
  "ticks_with_tx_ratio": 0.0015,
  "db_size_mb": 2502
}
```

---

## Timestamp Handling

All timestamps are now returned as **Unix microseconds** (not milliseconds or ISO strings).

### Conversion Examples

**JavaScript:**
```javascript
// Convert microseconds to Date
const timestampMicros = 1768032065977135;
const date = new Date(timestampMicros / 1000);
console.log(date.toISOString()); // "2026-01-10T07:54:25.977Z"

// Helper function
function microsToDate(micros) {
  return new Date(micros / 1000);
}
```

**TypeScript:**
```typescript
// Type definition
interface Tick {
  tick_number: number;
  timestamp: number; // Unix microseconds
  transaction_count: number;
  transaction_batch_hash: string;
}

// Conversion utility
const formatTimestamp = (micros: number): string => {
  return new Date(micros / 1000).toISOString();
};
```

---

## Endpoint Mapping

| Old Endpoint | New Endpoint | Notes |
|--------------|--------------|-------|
| `GET /api/v1/continuum/tick/recent` | `GET /api/v1/continuum/tick/recent` | Same path, new response format |
| `GET /api/v1/continuum/tick/{tickNumber}` | `GET /api/v1/continuum/tick/{tickNumber}` | Same path, new response format |
| `GET /api/v1/continuum/txn/recent` | `GET /api/v1/continuum/txn/recent` | Same path, new response format |
| `GET /api/v1/continuum/txn/{txnId}` | `GET /api/v1/continuum/txn/{txHash}` | Now uses tx_hash instead of tx_id |
| N/A | `GET /api/v1/continuum/health` | **New endpoint** |
| N/A | `GET /api/v1/continuum/info` | **New endpoint** |
| N/A | `GET /api/v1/continuum/stats` | **New endpoint** |

---

## Query Parameters

### Pagination

| Parameter | Default | Max | Description |
|-----------|---------|-----|-------------|
| `limit` | 20 | 100 | Number of items to return |

**Example:**
```
GET /api/v1/continuum/tick/recent?limit=50
GET /api/v1/continuum/txn/recent?limit=100
```

---

## Error Responses

All errors now follow a consistent format:

```json
{
  "success": false,
  "error": "Error message here"
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Bad Request - Invalid parameters |
| 404 | Not Found - Resource doesn't exist |
| 429 | Rate Limited - Too many requests |
| 500 | Internal Server Error |
| 502 | Bad Gateway - Explorer API unavailable |

---

## Rate Limits

| Endpoint Group | Limit |
|----------------|-------|
| `/api/v1/continuum/*` | 2000 requests/minute |

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 2000
X-RateLimit-Remaining: 1999
X-RateLimit-Reset: 1704891600
```

---

## Migration Checklist

- [ ] Update timestamp parsing to handle Unix microseconds
- [ ] Update type definitions for new response formats
- [ ] Handle wrapped responses (`success` + `data` structure)
- [ ] Update transaction lookups to use `tx_hash` instead of `tx_id`
- [ ] Add support for new fields (`transaction_batch_hash`, `vdf_proof`, etc.)
- [ ] Implement new endpoints (`/health`, `/info`, `/stats`) if needed
- [ ] Update error handling for new error format
- [ ] Test all Continuum-related features

---

## Support

For questions or issues with the migration, contact the backend team or open an issue in the repository.
