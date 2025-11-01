# Tick Ingestion Service

High-performance service for ingesting 10,000 ticks/second from Continuum gRPC stream into TimescaleDB.

## Architecture

```
Continuum gRPC → Stream Reader → Parser (8 workers) → Batch Writer (8 workers) → TimescaleDB
                      ↓              ↓                      ↓
                  10k buffer    pb.Tick → domain.Tick   250 ticks/batch
                                                        100ms timeout
```

## Quick Start

### 1. Apply Database Schema

```bash
# Set your database URL
export DATABASE_URL="postgres://user:pass@host:port/db?sslmode=require"

# Apply schema
psql $DATABASE_URL -f schema/001_create_tables.sql
```

### 2. Configure Environment

```bash
# Copy example config
cp .env.ingester.example .env.ingester

# Edit configuration
nano .env.ingester
```

### 3. Run Service

```bash
# Production mode (TimescaleDB)
make build-ingester
source .env.ingester
./bin/tick-ingester

# Debug mode (Console output)
OUTPUT_MODE=console OUTPUT_FORMAT=table ./bin/tick-ingester
```

## Configuration

### Required

- `CONTINUUM_GRPC_URL` - Continuum gRPC endpoint (e.g., `localhost:50051`)
- `DATABASE_URL` - TimescaleDB connection string (if `OUTPUT_MODE=timescale`)

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE_NAME` | `tick-ingester` | Service identifier |
| `ENV` | `development` | Environment: development/staging/production |
| `START_TICK` | `0` | Starting tick (0 = latest) |
| `DB_MAX_CONNECTIONS` | `100` | Max database connections |
| `DB_MIN_CONNECTIONS` | `10` | Min idle connections |
| `BUFFER_SIZE` | `10000` | Tick buffer capacity |
| `WORKER_COUNT` | `8` | Number of worker goroutines |
| `BATCH_SIZE` | `250` | Ticks per batch write |
| `FLUSH_INTERVAL` | `100ms` | Max time before flushing |
| `OUTPUT_MODE` | `timescale` | Output: `timescale` or `console` |
| `OUTPUT_FORMAT` | `json` | Console format: `json`, `compact`, or `table` |
| `HEALTH_CHECK_PORT` | `8081` | Health check HTTP port |

## Output Modes

### 1. TimescaleDB (Production)

Writes ticks to TimescaleDB using high-performance COPY protocol.

```bash
OUTPUT_MODE=timescale DATABASE_URL=postgres://... ./bin/tick-ingester
```

**Performance:**
- 50k-200k rows/sec write throughput
- <100ms latency (stream → DB)
- 3-table transaction (ticks, vdf_proofs, tick_transactions)

### 2. Console (Debugging)

Outputs ticks to stdout for local development.

```bash
# JSON format (pretty-printed)
OUTPUT_MODE=console OUTPUT_FORMAT=json ./bin/tick-ingester

# Compact JSON (one line per tick)
OUTPUT_MODE=console OUTPUT_FORMAT=compact ./bin/tick-ingester

# Table format (human-readable)
OUTPUT_MODE=console OUTPUT_FORMAT=table ./bin/tick-ingester
```

## Health Checks

The service exposes health endpoints on port 8081 (configurable):

```bash
# Health check
curl http://localhost:8081/health
# {"status":"ok"}

# Readiness check
curl http://localhost:8081/ready
# {"status":"ready"}
```

## Graceful Shutdown

The service handles `SIGINT` and `SIGTERM` gracefully:

1. Stops accepting new ticks from stream
2. Drains buffered ticks (up to 30 seconds)
3. Flushes final batch to database
4. Closes database connections
5. Exits

```bash
# Send shutdown signal
kill -SIGTERM <pid>

# Or use Ctrl+C
```

## Monitoring

### Logs

Structured JSON logs (production) or pretty console logs (development):

```json
{
  "level": "info",
  "timestamp": "2025-01-01T12:00:00Z",
  "msg": "Wrote batch to TimescaleDB",
  "tick_count": 250,
  "first_tick": 12345,
  "last_tick": 12594
}
```

### Metrics (Coming Soon)

Prometheus metrics endpoint (`:8081/metrics`):
- `tick_ingester_ticks_total{status="success|error"}`
- `tick_ingester_buffer_size`
- `tick_ingester_write_duration_seconds`
- `tick_ingester_grpc_reconnects_total`

## Performance Tuning

### For Maximum Throughput

```bash
WORKER_COUNT=16              # More workers (if CPU available)
BATCH_SIZE=500               # Larger batches
BUFFER_SIZE=20000            # Larger buffer
DB_MAX_CONNECTIONS=200       # More DB connections
FLUSH_INTERVAL=200ms         # Longer flush interval
```

### For Low Latency

```bash
WORKER_COUNT=8               # Standard workers
BATCH_SIZE=100               # Smaller batches
FLUSH_INTERVAL=50ms          # Faster flush
```

### For Resource Constrained

```bash
WORKER_COUNT=4               # Fewer workers
BATCH_SIZE=100               # Smaller batches
BUFFER_SIZE=5000             # Smaller buffer
DB_MAX_CONNECTIONS=50        # Fewer DB connections
```

## Troubleshooting

### "Failed to connect to database"

Check DATABASE_URL format and network connectivity:

```bash
psql $DATABASE_URL -c "SELECT 1"
```

### "Stream error (will reconnect)"

The service auto-reconnects to gRPC. Check Continuum gRPC server status.

### "Pipeline shutdown timed out"

Buffer may be too large or database too slow. Reduce `BUFFER_SIZE` or increase `DB_MAX_CONNECTIONS`.

### High Memory Usage

Reduce `BUFFER_SIZE` and `BATCH_SIZE`:

```bash
BUFFER_SIZE=5000
BATCH_SIZE=100
```

## Development

### Build

```bash
go build -o bin/tick-ingester ./cmd/tick-ingester
```

### Test

```bash
# Unit tests
make test-ingester

# Specific package
go test -v ./internal/ingestion

# With coverage
go test -cover ./internal/...
```

### Debug Mode

```bash
# Console output with table format
OUTPUT_MODE=console \
OUTPUT_FORMAT=table \
CONTINUUM_GRPC_URL=localhost:50051 \
./bin/tick-ingester
```

## Production Deployment

### systemd Service

Create `/etc/systemd/system/tick-ingester.service`:

```ini
[Unit]
Description=Tick Ingestion Service
After=network.target

[Service]
Type=simple
User=fermi
WorkingDirectory=/opt/fermi
EnvironmentFile=/opt/fermi/.env.ingester
ExecStart=/opt/fermi/bin/tick-ingester
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable tick-ingester
sudo systemctl start tick-ingester
sudo systemctl status tick-ingester
```

### Docker

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN go build -o tick-ingester ./cmd/tick-ingester

FROM debian:12-slim
COPY --from=builder /app/tick-ingester /usr/local/bin/
CMD ["tick-ingester"]
```

## Architecture Details

### Stream → Parse → Write Pipeline

1. **Stream Reader**: Reads ticks from gRPC, auto-reconnects on failure
2. **Parser Workers**: Convert protobuf to domain models (8 parallel workers)
3. **Batch Writers**: Accumulate and write batches (8 parallel workers)

### Database Write Strategy

Uses PostgreSQL COPY protocol (10-50x faster than INSERT):

```sql
COPY ticks (tick_number, timestamp, batch_hash) FROM STDIN
COPY vdf_proofs (tick_number, input, output, proof, iterations) FROM STDIN
COPY tick_transactions (...) FROM STDIN
```

All three COPYs wrapped in a single transaction for atomicity.

## License

Proprietary - Fermi Labs
