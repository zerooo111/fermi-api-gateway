# Running Fermi API Gateway Services

This project has two main services that can be run independently:

## 1. API Gateway Server

Runs the HTTP API gateway that routes requests to backend services.

```bash
make gateway
```

**Configuration**: Uses `.env` file (gateway section)
**Port**: 3000 (default)
**Endpoints**: 
- `/health` - Health check
- `/ready` - Readiness check
- API routes for rollup and continuum services

---

## 2. Tick Ingester Service

### Production Mode (with TimescaleDB)

Ingests ticks from Continuum gRPC stream and persists to TimescaleDB.

```bash
make ingester
```

**Configuration**: Uses `.env` file (ingester section)
**Features**:
- Connects to real Continuum gRPC stream
- Writes to TimescaleDB via COPY protocol
- 10k ticks/sec throughput
- Automatic batching and buffering
- Prometheus metrics at `:8081/metrics`

**Endpoints**:
- `http://localhost:8081/health` - Health check
- `http://localhost:8081/ready` - Readiness check
- `http://localhost:8081/metrics` - Prometheus metrics

### Debug Mode (console output only)

Ingests ticks but only outputs to console (no database writes).

```bash
make ingester-debug
```

**Configuration**: Uses `.env` for gRPC URL, overrides OUTPUT_MODE
**Features**:
- Console table output
- No database required
- Perfect for testing and debugging
- Same gRPC stream as production

---

## Configuration

All services automatically load configuration from the `.env` file in the project root when using `make` commands.

### Key Variables:

**Gateway:**
- `PORT` - HTTP server port
- `CONTINUUM_GRPC_URL` - Continuum gRPC endpoint
- `CONTINUUM_REST_URL` - Continuum REST API endpoint

**Ingester:**
- `CONTINUUM_GRPC_URL` - Continuum gRPC endpoint (100.24.216.168:9090)
- `DATABASE_URL` - TimescaleDB connection string
- `BUFFER_SIZE` - Tick buffer capacity (10000)
- `WORKER_COUNT` - Worker goroutines (8)
- `BATCH_SIZE` - Ticks per batch (250)
- `OUTPUT_MODE` - timescale or console

---

## Quick Start

1. **Start API Gateway:**
   ```bash
   make gateway
   ```

2. **Start Tick Ingester (Production):**
   ```bash
   make ingester
   ```

3. **Test Tick Ingester (Debug):**
   ```bash
   make ingester-debug
   ```

---

## Development

- **Format code**: `make fmt`
- **Run tests**: `make test`
- **Run all tests**: `make test-all`
- **Build binaries**: `make build`

For more details, run `make help`
