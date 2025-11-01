# Testing Guide for Tick Ingestion Service

## Test Types

### 1. Unit Tests

Test individual components in isolation.

```bash
# Run all unit tests
make test-ingester

# Run specific package tests
go test -v ./internal/domain
go test -v ./internal/parser
go test -v ./internal/writer
go test -v ./internal/ingestion

# Run with coverage
go test -cover ./internal/...

# Generate coverage report
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out
```

**Coverage Targets:**
- Domain models: 100%
- Parser: 100%
- Console writer: 100%
- Pipeline: 90%+

### 2. Integration Tests

Test complete pipeline with real database.

**Prerequisites:**
- TimescaleDB running
- Schema applied
- `INTEGRATION_DB_URL` environment variable set

```bash
# Run integration tests
go test -tags=integration ./test/integration -v

# With specific database
INTEGRATION_DB_URL="postgres://user:pass@localhost/test_db" \
  go test -tags=integration ./test/integration -v
```

**What's tested:**
- End-to-end tick ingestion
- Database writes via COPY protocol
- Data integrity verification
- Performance under load

### 3. Load Tests

Validate 10k ticks/sec performance target.

```bash
# Start the service first
OUTPUT_MODE=timescale ./bin/tick-ingester &

# Run load test
./test/loadtest/loadtest.sh
```

**Load Test Metrics:**
- Duration: 60 seconds
- Target: 10,000 ticks/sec
- Acceptable error rate: <1%
- Results saved to `loadtest-results-*.txt`

### 4. Benchmark Tests

Measure component performance.

```bash
# Run all benchmarks
go test -bench=. ./internal/...

# Specific benchmarks
go test -bench=BenchmarkPipeline ./internal/ingestion
go test -bench=BenchmarkParser ./internal/parser

# With memory profiling
go test -bench=. -benchmem ./internal/...

# CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./internal/ingestion
go tool pprof cpu.prof
```

## Test Environments

### Local Development

```bash
# Use console output for debugging
OUTPUT_MODE=console OUTPUT_FORMAT=table ./bin/tick-ingester
```

### Staging

```bash
# Connect to staging database
DATABASE_URL="postgres://staging..." ./bin/tick-ingester
```

### Production

```bash
# Full configuration
source .env.ingester.production
./bin/tick-ingester
```

## Integration Test Setup

### 1. Start TimescaleDB (Docker)

```bash
docker run -d \
  --name timescale-test \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=testpass \
  timescale/timescaledb:latest-pg16
```

### 2. Create Test Database

```bash
docker exec -it timescale-test psql -U postgres -c "CREATE DATABASE test_ingestion;"
```

### 3. Apply Schema

```bash
docker exec -i timescale-test psql -U postgres -d test_ingestion < schema/001_create_tables.sql
```

### 4. Run Tests

```bash
export INTEGRATION_DB_URL="postgres://postgres:testpass@localhost/test_ingestion?sslmode=disable"
go test -tags=integration ./test/integration -v
```

### 5. Cleanup

```bash
docker stop timescale-test
docker rm timescale-test
```

## Load Testing Scenarios

### Scenario 1: Sustained Throughput

Test steady 10k ticks/sec for 60 seconds.

```bash
./test/loadtest/loadtest.sh
```

**Expected Results:**
- Average TPS: ≥10,000
- Error rate: <0.01%
- p95 latency: <100ms
- Buffer utilization: <50%

### Scenario 2: Burst Load

Test ability to handle burst traffic.

```bash
# Modify DURATION_SECONDS=10 and TARGET_TPS=20000 in loadtest.sh
./test/loadtest/loadtest.sh
```

**Expected Results:**
- Buffer absorbs burst
- No errors
- Recovery to steady state

### Scenario 3: Long Duration

Test stability over extended period.

```bash
# Modify DURATION_SECONDS=3600 (1 hour)
./test/loadtest/loadtest.sh
```

**Expected Results:**
- Stable throughput
- No memory leaks
- Consistent latency

## Manual Testing

### Test Console Output

```bash
# JSON format
OUTPUT_MODE=console OUTPUT_FORMAT=json ./bin/tick-ingester

# Compact format
OUTPUT_MODE=console OUTPUT_FORMAT=compact ./bin/tick-ingester

# Table format (human-readable)
OUTPUT_MODE=console OUTPUT_FORMAT=table ./bin/tick-ingester
```

### Test Health Endpoints

```bash
# Health check
curl http://localhost:8081/health
# Expected: {"status":"ok"}

# Readiness check
curl http://localhost:8081/ready
# Expected: {"status":"ready"}

# Metrics
curl http://localhost:8081/metrics
# Expected: Prometheus metrics
```

### Test Graceful Shutdown

```bash
# Start service
./bin/tick-ingester &
PID=$!

# Send shutdown signal
kill -SIGTERM $PID

# Verify logs show graceful shutdown
# Expected: "Pipeline shut down successfully"
```

### Test Database Writes

```bash
# Start service
OUTPUT_MODE=timescale ./bin/tick-ingester &

# Wait a few seconds
sleep 10

# Check database
psql $DATABASE_URL -c "SELECT COUNT(*) FROM ticks;"
psql $DATABASE_URL -c "SELECT * FROM v_recent_ticks LIMIT 5;"
```

## Performance Testing

### Baseline Performance

```bash
# Measure baseline (no load)
go test -bench=. -benchtime=10s ./internal/ingestion
```

### With Different Configurations

```bash
# Small batches
BATCH_SIZE=100 go test -bench=BenchmarkPipeline ./internal/ingestion

# Large batches
BATCH_SIZE=500 go test -bench=BenchmarkPipeline ./internal/ingestion

# More workers
WORKER_COUNT=16 go test -bench=BenchmarkPipeline ./internal/ingestion
```

## Test Data Generation

### Generate Test Ticks

```go
func generateTestTicks(count int) []*domain.Tick {
    ticks := make([]*domain.Tick, count)
    for i := 0; i < count; i++ {
        ticks[i] = &domain.Tick{
            TickNumber: uint64(i + 1),
            Timestamp:  time.Now(),
            VDFProof:   domain.VDFProof{ /* ... */ },
            // ...
        }
    }
    return ticks
}
```

### Mock gRPC Server

For testing without real Continuum gRPC server:

```go
type mockGRPCServer struct {
    tickRate int // ticks per second
}

func (s *mockGRPCServer) StreamTicks(req *pb.StreamTicksRequest, stream pb.SequencerService_StreamTicksServer) error {
    ticker := time.NewTicker(time.Second / time.Duration(s.tickRate))
    defer ticker.Stop()

    tickNum := req.StartTick
    for range ticker.C {
        tick := generateTick(tickNum)
        if err := stream.Send(tick); err != nil {
            return err
        }
        tickNum++
    }
    return nil
}
```

## Troubleshooting Tests

### Test Failures

```bash
# Verbose output
go test -v ./...

# Show all logs
go test -v -args -test.v

# Run specific test
go test -run TestPipeline_Run_ProcessesTicks ./internal/ingestion
```

### Integration Test Issues

```bash
# Check database connection
psql $INTEGRATION_DB_URL -c "SELECT 1;"

# Verify schema
psql $INTEGRATION_DB_URL -c "\dt"

# Check for test data
psql $INTEGRATION_DB_URL -c "SELECT COUNT(*) FROM ticks;"
```

### Load Test Issues

```bash
# Check service logs
journalctl -u tick-ingester -f

# Monitor metrics in real-time
watch -n 1 'curl -s http://localhost:8081/metrics | grep tick_ingester'

# Check database performance
psql $DATABASE_URL -c "SELECT * FROM pg_stat_activity WHERE datname = 'tsdb';"
```

## Continuous Integration

### GitHub Actions Example

```yaml
name: Test Tick Ingestion

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      timescaledb:
        image: timescale/timescaledb:latest-pg16
        env:
          POSTGRES_PASSWORD: testpass
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'

      - name: Run Unit Tests
        run: make test-ingester

      - name: Apply Schema
        run: psql postgres://postgres:testpass@localhost/postgres < schema/001_create_tables.sql

      - name: Run Integration Tests
        env:
          INTEGRATION_DB_URL: postgres://postgres:testpass@localhost/postgres
        run: go test -tags=integration ./test/integration -v
```

## Test Metrics

Track test quality over time:

- Test coverage: ≥90%
- Test execution time: <30s (unit tests)
- Integration test success rate: 100%
- Load test pass rate: 100%
