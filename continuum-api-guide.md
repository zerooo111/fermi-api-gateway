# Continuum Explorer API Guide

## Overview

The Continuum Explorer API provides read-only access to indexed blockchain data from the Continuum network. It exposes endpoints for querying ticks (blocks), transactions, and system statistics.

**Base URL:** `http://http://18.220.137.190`

**Content-Type:** All responses are `application/json`

**Authentication:** None (internal service)

---

## Response Format

### Success Response

All successful responses return HTTP `200 OK` with the data directly or wrapped in an `ApiResponse`:

```json
{
  "success": true,
  "data": { ... }
}
```

### Error Response

Error responses use appropriate HTTP status codes with a consistent error structure:

```json
{
  "success": false,
  "error": {
    "error": "Human-readable error message",
    "code": "MACHINE_READABLE_CODE",
    "details": "Optional additional context"
  }
}
```

### Error Codes

| Code | Description |
|------|-------------|
| `NOT_FOUND` | Requested resource does not exist |
| `BAD_REQUEST` | Invalid request parameters |
| `INTERNAL_ERROR` | Server-side error |

### HTTP Status Codes

| Status | Meaning |
|--------|---------|
| `200` | Success |
| `404` | Resource not found |
| `503` | Service unavailable (health check only) |

---

## Endpoints

### Health Check

Check if the service is healthy and operational.

```
GET /health
```

**Response (200 - Healthy):**
```json
{
  "status": "healthy",
  "db_healthy": true,
  "latest_tick": 51548118616
}
```

**Response (503 - Degraded):**
```json
{
  "status": "degraded",
  "db_healthy": false,
  "latest_tick": 51548118616
}
```

**Use Cases:**
- Load balancer health checks
- Monitoring and alerting
- API Gateway health probes

---

### Root / Service Info

Get service information and available endpoints.

```
GET /
```

**Response (200):**
```json
{
  "service": "Continuum Explorer API",
  "version": "1.0.0",
  "endpoints": {
    "recent_ticks": "/api/v1/ticks/recent?limit=20",
    "get_tick": "/api/v1/ticks/:tick_number",
    "recent_transactions": "/api/v1/transactions/recent?limit=20",
    "get_transaction": "/api/v1/transactions/:tx_hash",
    "stats": "/api/v1/stats",
    "health": "/health"
  }
}
```

---

### Get Recent Ticks

Retrieve the most recent ticks (blocks) that contain transactions.

```
GET /api/v1/ticks/recent
GET /api/v1/ticks/recent?limit=50
```

**Query Parameters:**

| Parameter | Type | Default | Range | Description |
|-----------|------|---------|-------|-------------|
| `limit` | integer | 20 | 1-100 | Number of ticks to return |

**Response (200):**
```json
{
  "ticks": [
    {
      "tick_number": 51548050841,
      "timestamp": 1768028416704159,
      "transaction_count": 1,
      "transaction_batch_hash": "17c7906c4da365b5b7031d56e5df83221095fd8251684aab6371503966517217"
    },
    {
      "tick_number": 51548050502,
      "timestamp": 1768028416365127,
      "transaction_count": 2,
      "transaction_batch_hash": "a1b2c3d4e5f6..."
    }
  ],
  "count": 2,
  "latest_tick_number": 51548050889
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `ticks` | array | List of tick summaries (most recent first) |
| `ticks[].tick_number` | u64 | Unique tick identifier |
| `ticks[].timestamp` | u64 | Unix timestamp in microseconds |
| `ticks[].transaction_count` | integer | Number of transactions in this tick |
| `ticks[].transaction_batch_hash` | string | Hash of the transaction batch (hex) |
| `count` | integer | Number of ticks returned |
| `latest_tick_number` | u64 | Most recent tick number in the system |

---

### Get Tick by Number

Retrieve detailed information about a specific tick.

```
GET /api/v1/ticks/:tick_number
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `tick_number` | u64 | The tick number to retrieve |

**Response (200):**
```json
{
  "success": true,
  "data": {
    "tick_number": 51548050841,
    "timestamp": 1768028416704159,
    "transaction_count": 1,
    "transaction_batch_hash": "17c7906c4da365b5b7031d56e5df83221095fd8251684aab6371503966517217",
    "previous_output": "a1b2c3d4e5f6...",
    "vdf_proof": {
      "input": "0x...",
      "output": "0x...",
      "proof": "0x...",
      "iterations": 1000000
    },
    "transactions": [
      {
        "tx_hash": "abc123...",
        "tick_number": 51548050841,
        "sequence_number": 0,
        "tx_id": "user_tx_001",
        "timestamp": 1768028416704000,
        "payload_size": 256
      }
    ]
  }
}
```

**Response (404 - Not Found):**
```json
{
  "success": false,
  "error": {
    "error": "Tick not found",
    "code": "NOT_FOUND",
    "details": "No Tick found with identifier: 999999999999"
  }
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `tick_number` | u64 | Unique tick identifier |
| `timestamp` | u64 | Unix timestamp in microseconds |
| `transaction_count` | integer | Number of transactions |
| `transaction_batch_hash` | string | Hash of transaction batch (hex) |
| `previous_output` | string | Previous VDF output (hex) |
| `vdf_proof` | object \| null | VDF proof details (if available) |
| `vdf_proof.input` | string | VDF input (hex) |
| `vdf_proof.output` | string | VDF output (hex) |
| `vdf_proof.proof` | string | VDF proof data (hex) |
| `vdf_proof.iterations` | u64 | Number of VDF iterations |
| `transactions` | array | List of transaction summaries |

---

### Get Recent Transactions

Retrieve the most recent transactions across all ticks.

```
GET /api/v1/transactions/recent
GET /api/v1/transactions/recent?limit=50
```

**Query Parameters:**

| Parameter | Type | Default | Range | Description |
|-----------|------|---------|-------|-------------|
| `limit` | integer | 20 | 1-100 | Number of transactions to return |

**Response (200):**
```json
{
  "transactions": [
    {
      "tx_hash": "abc123def456...",
      "tick_number": 51548050841,
      "sequence_number": 0,
      "tx_id": "user_tx_001",
      "timestamp": 1768028416704000,
      "payload_size": 256
    }
  ],
  "count": 1
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `transactions` | array | List of transaction summaries |
| `transactions[].tx_hash` | string | Unique transaction hash (hex) |
| `transactions[].tick_number` | u64 | Tick containing this transaction |
| `transactions[].sequence_number` | u64 | Order within the tick |
| `transactions[].tx_id` | string | User-provided transaction ID |
| `transactions[].timestamp` | u64 | Transaction timestamp (microseconds) |
| `transactions[].payload_size` | integer | Size of payload in bytes |
| `count` | integer | Number of transactions returned |

---

### Get Transaction by Hash

Retrieve detailed information about a specific transaction.

```
GET /api/v1/transactions/:tx_hash
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `tx_hash` | string | The transaction hash (hex) |

**Response (200):**
```json
{
  "success": true,
  "data": {
    "tx_hash": "abc123def456...",
    "tick_number": 51548050841,
    "sequence_number": 0,
    "ingestion_timestamp": 1768028416800000,
    "tx_id": "user_tx_001",
    "nonce": 12345,
    "timestamp": 1768028416704000,
    "payload": "SGVsbG8gV29ybGQ=",
    "payload_text": "Hello World",
    "payload_size": 11,
    "signature": "304402...",
    "public_key": "04a1b2c3..."
  }
}
```

**Response (404 - Not Found):**
```json
{
  "success": false,
  "error": {
    "error": "Transaction not found",
    "code": "NOT_FOUND",
    "details": "No Transaction found with identifier: nonexistent_hash"
  }
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `tx_hash` | string | Unique transaction hash (hex) |
| `tick_number` | u64 | Tick containing this transaction |
| `sequence_number` | u64 | Order within the tick |
| `ingestion_timestamp` | u64 | When the explorer indexed this (microseconds) |
| `tx_id` | string | User-provided transaction ID |
| `nonce` | u64 | Transaction nonce |
| `timestamp` | u64 | Transaction creation timestamp (microseconds) |
| `payload` | string | Transaction payload (base64 encoded) |
| `payload_text` | string \| null | Payload as UTF-8 text (if valid UTF-8) |
| `payload_size` | integer | Size of payload in bytes |
| `signature` | string | Transaction signature (hex) |
| `public_key` | string | Signer's public key (hex) |

---

### Get Statistics

Retrieve indexing statistics and system metrics.

```
GET /api/v1/stats
```

**Response (200):**
```json
{
  "ticks_indexed": 870432,
  "transactions_indexed": 1523456,
  "empty_ticks_skipped": 576543210,
  "latest_tick_number": 51548202863,
  "memory_ticks_count": 1000,
  "memory_txs_count": 10000,
  "tick_hit_rate": 0.85,
  "tx_hit_rate": 0.92,
  "ticks_with_tx_ratio": 0.0015,
  "prune_runs": 5,
  "ticks_pruned": 50000,
  "transactions_pruned": 125000,
  "oldest_tick_number": 51500000000,
  "db_size_mb": 2286
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `ticks_indexed` | u64 | Total ticks with transactions stored |
| `transactions_indexed` | u64 | Total transactions stored |
| `empty_ticks_skipped` | u64 | Ticks without transactions (not stored) |
| `latest_tick_number` | u64 | Most recent tick number |
| `memory_ticks_count` | u64 | Ticks in memory ring buffer |
| `memory_txs_count` | u64 | Transactions in memory ring buffer |
| `tick_hit_rate` | f64 | Cache hit rate for tick queries (0.0-1.0) |
| `tx_hit_rate` | f64 | Cache hit rate for transaction queries (0.0-1.0) |
| `ticks_with_tx_ratio` | f64 | Ratio of ticks containing transactions |
| `prune_runs` | u64 | Number of pruning operations executed |
| `ticks_pruned` | u64 | Total ticks removed by pruning |
| `transactions_pruned` | u64 | Total transactions removed by pruning |
| `oldest_tick_number` | u64 | Oldest tick in database |
| `db_size_mb` | u64 | RocksDB size in megabytes |

---

## CORS Configuration

CORS is configurable via the `CORS_ALLOWED_ORIGINS` environment variable:

- **Not set / empty:** Permissive CORS (allows all origins) - development only
- **Set:** Comma-separated list of allowed origins

Example:
```
CORS_ALLOWED_ORIGINS=https://explorer.example.com,https://app.example.com
```

---

## Rate Limiting

No rate limiting is implemented at the application level. Rate limiting should be configured at the API Gateway.

**Recommended limits:**
- `/api/v1/ticks/recent`: 100 req/min
- `/api/v1/transactions/recent`: 100 req/min
- `/api/v1/ticks/:tick_number`: 300 req/min
- `/api/v1/transactions/:tx_hash`: 300 req/min
- `/api/v1/stats`: 60 req/min
- `/health`: No limit (for health checks)

---

## Frontend Integration Examples

### JavaScript/Fetch

```javascript
const API_BASE = 'http://localhost:3000';

// Get recent ticks
async function getRecentTicks(limit = 20) {
  const response = await fetch(`${API_BASE}/api/v1/ticks/recent?limit=${limit}`);
  return response.json();
}

// Get specific tick
async function getTick(tickNumber) {
  const response = await fetch(`${API_BASE}/api/v1/ticks/${tickNumber}`);
  if (response.status === 404) {
    const error = await response.json();
    throw new Error(error.error.details);
  }
  return response.json();
}

// Get transaction
async function getTransaction(txHash) {
  const response = await fetch(`${API_BASE}/api/v1/transactions/${txHash}`);
  if (response.status === 404) {
    const error = await response.json();
    throw new Error(error.error.details);
  }
  const result = await response.json();
  return result.data;
}

// Poll for updates
function pollRecentTicks(callback, intervalMs = 1000) {
  setInterval(async () => {
    const data = await getRecentTicks(20);
    callback(data);
  }, intervalMs);
}
```

### React Hook Example

```javascript
import { useState, useEffect } from 'react';

function useRecentTicks(limit = 20, pollInterval = 1000) {
  const [ticks, setTicks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchTicks = async () => {
      try {
        const response = await fetch(
          `${API_BASE}/api/v1/ticks/recent?limit=${limit}`
        );
        const data = await response.json();
        setTicks(data.ticks);
        setLoading(false);
      } catch (err) {
        setError(err.message);
        setLoading(false);
      }
    };

    fetchTicks();
    const interval = setInterval(fetchTicks, pollInterval);
    return () => clearInterval(interval);
  }, [limit, pollInterval]);

  return { ticks, loading, error };
}
```

---

## Timestamp Handling

All timestamps are in **microseconds** since Unix epoch. To convert to JavaScript Date:

```javascript
function microsecondsToDate(microseconds) {
  return new Date(microseconds / 1000);
}

// Example
const tick = await getTick(51548050841);
const date = microsecondsToDate(tick.data.timestamp);
console.log(date.toISOString()); // "2026-01-10T06:40:16.704Z"
```

---

## Data Retention

- Only ticks containing transactions are indexed
- Database is pruned when size exceeds configured limit (default: 40GB)
- Oldest ticks are removed first during pruning
- In-memory ring buffers hold most recent 1000 ticks and 10000 transactions

---

## Contact

For API issues or feature requests, contact the platform team.
