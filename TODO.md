# Fermi API Gateway - Development Tracker

## Project Overview
Building a low-latency, high-throughput API gateway for a Solana hybrid DEX with monitoring and observability.

**Target Performance**: 50k-200k req/sec
**Language**: Go
**Deployment**: EC2 + Nginx with SSL
**Development Approach**: Test-Driven Development (TDD) ✅

---

## 🧪 Test-Driven Development (TDD) Approach

We are following TDD methodology for all features:

### The TDD Cycle (Red-Green-Refactor)
1. **🔴 RED**: Write a failing test first
2. **🟢 GREEN**: Write minimal code to make the test pass
3. **🔵 REFACTOR**: Clean up code while keeping tests green

### Testing Strategy
- **Unit Tests**: Test individual functions and methods in isolation
- **Integration Tests**: Test HTTP endpoints and middleware chains
- **Table-Driven Tests**: Go's idiomatic way to test multiple scenarios
- **Test Coverage**: Aim for 80%+ coverage on critical paths

### Testing Tools
- **Standard Library**: `testing` package (no external dependencies needed)
- **Test Helpers**: `httptest` for HTTP handler testing
- **Assertions**: Simple `if` checks (Go style) or testify for convenience
- **Coverage**: `go test -cover` to measure coverage

### Test File Organization
- Test files live next to source: `foo.go` → `foo_test.go`
- Use `_test` package suffix for black-box testing when needed
- Table-driven tests for multiple test cases

### Running Tests
```bash
make test              # Run all tests
go test ./...          # Run all tests (verbose)
go test -v ./...       # Verbose output
go test -cover ./...   # With coverage
go test -race ./...    # Race detector (important for concurrent code)
```

---

## Progress Tracker

### ✅ Completed
- [x] TODO.md created
- [x] Step 1: Project Setup & Basic Server
- [x] Tests for Step 1 (100% coverage! 🎉)

### 🚧 In Progress
- [ ] None

### 📋 Pending
- [ ] Step 2: Middleware Layer
- [ ] Step 3: In-Memory Rate Limiting
- [ ] Step 4: Prometheus Metrics
- [ ] Step 5: Reverse Proxy Setup
- [ ] Step 6: EC2 Deployment Scripts
- [ ] Step 7: Nginx Configuration
- [ ] Step 8: Monitoring Setup
- [ ] Step 9: Testing & Documentation

---

## Step-by-Step Implementation

### Step 1: Project Setup & Basic Server ✅
**Goal**: Create Go project with basic HTTP server and health checks

**Tasks**:
- [x] Initialize Go module
- [x] Install Chi router
- [x] Create project structure
- [x] Implement basic HTTP server
- [x] Add /health endpoint
- [x] Environment-based configuration
- [x] Create Makefile for build commands
- [x] Test all endpoints

**Key Files**:
- `go.mod` - Dependency management
- `cmd/gateway/main.go` - Entry point with graceful shutdown
- `internal/config/config.go` - Environment-based configuration
- `internal/health/health.go` - Health and readiness checks
- `Makefile` - Build automation
- `.env.example` - Configuration template
- `README.md` - Project documentation

**Context & Learnings**:

**1. Project Structure**
- Using standard Go project layout: `cmd/` for binaries, `internal/` for private packages
- `internal/` packages cannot be imported by external projects (Go convention)
- This keeps our code organized and prevents accidental API exposure

**2. Chi Router**
- Chose Chi over other routers (Gin, Echo) because:
  - Lightweight and stdlib-compatible (uses standard `net/http`)
  - Excellent middleware support (composable)
  - No external dependencies beyond stdlib
  - Very fast routing performance
- Chi uses `http.HandlerFunc` standard interface, making it easy to learn

**3. Configuration Management**
- Using environment variables (12-factor app methodology)
- All config centralized in `internal/config/config.go`
- Default values provided for local development
- Helper functions (`getEnv`, `getEnvInt`, `getEnvSlice`) make config reading clean

**4. Graceful Shutdown**
- Implemented in `main.go` using signal channels
- Listens for SIGTERM and SIGINT (Ctrl+C)
- Gives in-flight requests 30 seconds to complete before forcing shutdown
- Critical for production deployments (prevents dropped requests)

**5. Health Checks**
- `/health` - Simple liveness check (is the process running?)
- `/ready` - Readiness check (is the service ready to handle traffic?)
- Returns JSON with timestamp and version
- In future steps, we can add backend connectivity checks to `/ready`

**6. HTTP Server Best Practices**
- Set timeouts (Read: 15s, Write: 15s, Idle: 60s) to prevent slowloris attacks
- Used structured responses (JSON)
- Proper HTTP status codes

**Testing Results**:
```bash
$ curl http://localhost:8080/health
{"status":"healthy","timestamp":"2025-10-30T04:20:41.610067+05:30","version":"1.0.0"}

$ curl http://localhost:8080/ready
{"status":"ready","timestamp":"2025-10-30T04:20:48.775725+05:30","version":"1.0.0"}

$ curl http://localhost:8080/
{"service":"fermi-api-gateway","version":"1.0.0","env":"development"}
```

**Key Go Concepts Used**:
- **Goroutines**: Used for concurrent server startup and signal listening
- **Channels**: For inter-goroutine communication (serverErrors, shutdown)
- **Context**: For request cancellation and timeouts
- **Defer**: For cleanup (cancel function)
- **Select**: For multiplexing channel operations

**Next Steps**:
- Middleware layer (CORS, logging, recovery, request IDs)
- These will be "wrapped" around our routes using Chi's middleware stack

---

### Step 1.5: Tests for Step 1 ✅
**Goal**: Add comprehensive tests for existing code (retroactive TDD)

**Tasks**:
- [x] Write unit tests for config package
- [x] Write integration tests for health handlers
- [x] Add table-driven tests for helper functions
- [x] Add benchmark tests for performance
- [x] Run with race detector
- [x] Achieve 100% test coverage
- [x] Update Makefile with test commands

**Key Files**:
- `internal/config/config_test.go` - Config tests (19 test cases)
- `internal/health/health_test.go` - Health handler tests (6 test cases + 2 benchmarks)
- `Makefile` - Enhanced with test-cover, test-race, test-bench, test-all

**Context & Learnings**:

**1. Table-Driven Tests**
- Go's idiomatic way to test multiple scenarios
- Each test case is a struct with inputs and expected outputs
- Use `t.Run()` for subtests (better organization and parallel execution)
- Example pattern:
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"case1", "input1", "output1"},
    {"case2", "input2", "output2"},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := function(tt.input)
        if result != tt.expected {
            t.Errorf("expected %s, got %s", tt.expected, result)
        }
    })
}
```

**2. HTTP Handler Testing with httptest**
- `httptest.NewRequest()` - Create fake HTTP requests
- `httptest.NewRecorder()` - Capture HTTP responses
- No need for actual HTTP server!
- Tests run in milliseconds
- Example:
```go
req := httptest.NewRequest(http.MethodGet, "/health", nil)
rr := httptest.NewRecorder()
handler.ServeHTTP(rr, req)
// Assert on rr.Code, rr.Body, rr.Header()
```

**3. Test Coverage**
- `go test -cover ./...` shows coverage percentage
- `go test -coverprofile=coverage.out` creates detailed report
- `go tool cover -html=coverage.out` for visual report
- We achieved **100% coverage** on config and health packages!

**4. Race Detection**
- `go test -race` detects data races (concurrent access issues)
- Essential for high-concurrency services
- Adds some overhead but worth it
- Our tests pass with no race conditions detected

**5. Benchmark Tests**
- Prefix function with `Benchmark` instead of `Test`
- Use `b.N` as loop counter (Go determines optimal N)
- Use `b.ResetTimer()` to exclude setup time
- Our results:
```
BenchmarkHealthHandler    2,217,540 ops    528.1 ns/op    1153 B/op    11 allocs/op
BenchmarkReadyHandler     2,270,176 ops    529.6 ns/op    1153 B/op    11 allocs/op
```
- This means ~2.2M requests/second per core!
- ~530ns per request is excellent latency

**6. Testing Best Practices**
- ✅ Test files next to source files (`foo.go` → `foo_test.go`)
- ✅ Use descriptive test names that explain intent
- ✅ Test both happy path and edge cases
- ✅ Clean up after tests (use `defer` for cleanup)
- ✅ Use `t.Fatal()` for setup failures, `t.Error()` for test failures
- ✅ Keep tests independent (no shared state)
- ✅ Use `os.Clearenv()` and `defer os.Unsetenv()` for env tests

**7. Test Organization**
- **Unit tests**: Test individual functions (config helpers)
- **Integration tests**: Test HTTP handlers end-to-end
- **Benchmark tests**: Measure performance
- **Race tests**: Detect concurrency issues

**Test Results Summary**:
```bash
$ make test-cover
✓ internal/config: 100.0% coverage (19 tests)
✓ internal/health: 100.0% coverage (6 tests)

$ make test-race
✓ No race conditions detected

$ make test-bench
✓ Health handler: ~2.2M req/sec, 530ns latency
```

**Key Testing Commands**:
```bash
make test              # Run all tests
make test-cover        # With coverage
make test-race         # With race detector
make test-bench        # Benchmarks only
make test-all          # Everything (coverage + race + bench)
```

**Next Steps**:
- For Step 2 onwards, we'll follow TRUE TDD:
  1. Write test first (RED)
  2. Make it pass (GREEN)
  3. Refactor (REFACTOR)
- Write middleware tests BEFORE writing middleware code

---

### Step 2: Middleware Layer
**Goal**: Implement essential middleware for production readiness

**Tasks**:
- [ ] CORS middleware with domain whitelist
- [ ] Structured logging middleware (Zap)
- [ ] Recovery middleware (panic handling)
- [ ] Request ID middleware

**Key Files**:
- `internal/middleware/cors.go`
- `internal/middleware/logging.go`
- `internal/middleware/recovery.go`
- `internal/middleware/requestid.go`

**Context & Learnings**:
- TBD

---

### Step 3: In-Memory IP-Based Rate Limiting
**Goal**: Implement in-memory rate limiting per IP per route

**Tasks**:
- [ ] Install `golang.org/x/time/rate` package
- [ ] IP extraction logic (X-Forwarded-For, X-Real-IP handling)
- [ ] In-memory rate limiter with cleanup (prevent memory leaks)
- [ ] Rate limiter middleware (per route)
- [ ] Configure limits per endpoint
- [ ] Rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining)
- [ ] Tests for rate limiting logic

**Key Files**:
- `internal/ratelimit/limiter.go` - In-memory rate limiter
- `internal/ratelimit/middleware.go` - Rate limiting middleware

**Context & Learnings**:
- Using in-memory rate limiting for single-instance deployment
- **Future scaling**: When deploying multiple instances, consider Redis-backed
  distributed rate limiting to maintain consistent limits across all instances
- Memory cleanup important to prevent leaks from unlimited IP tracking

---

### Step 4: Prometheus Metrics
**Goal**: Add observability with Prometheus metrics

**Tasks**:
- [ ] Metrics middleware
- [ ] Custom metrics (latency, requests, errors)
- [ ] /metrics endpoint
- [ ] Rate limit metrics

**Key Files**:
- `internal/metrics/metrics.go`
- `internal/middleware/metrics.go`

**Context & Learnings**:
- TBD

---

### Step 5: Reverse Proxy Setup
**Goal**: Proxy requests to Rollup service and Continuum backends

**Tasks**:
- [ ] Reverse proxy handler for HTTP (Rollup, Continuum REST)
- [ ] gRPC proxy handler for Continuum gRPC
- [ ] Backend routing configuration
- [ ] Connection pooling
- [ ] Timeout handling

**Key Files**:
- `internal/proxy/proxy.go` - HTTP reverse proxy
- `internal/proxy/grpc.go` - gRPC proxy
- `internal/proxy/router.go` - Backend routing

**Context & Learnings**:
- TBD

---

### Step 6: EC2 Deployment Scripts
**Goal**: Automate deployment on EC2

**Tasks**:
- [ ] setup.sh - Install dependencies
- [ ] deploy.sh - Build and restart
- [ ] Systemd service file
- [ ] Redis configuration

**Key Files**:
- `scripts/setup.sh`
- `scripts/deploy.sh`
- `deployments/gateway.service`
- `deployments/redis.conf`

**Context & Learnings**:
- TBD

---

### Step 7: Nginx Configuration
**Goal**: SSL reverse proxy with Let's Encrypt

**Tasks**:
- [ ] Nginx reverse proxy config
- [ ] SSL setup with certbot
- [ ] Security headers
- [ ] Access logs configuration

**Key Files**:
- `deployments/nginx.conf`
- `scripts/setup-ssl.sh`

**Context & Learnings**:
- TBD

---

### Step 8: Monitoring Setup
**Goal**: Complete observability stack

**Tasks**:
- [ ] Prometheus configuration
- [ ] Grafana dashboard JSON
- [ ] Alert rules

**Key Files**:
- `monitoring/prometheus.yml`
- `monitoring/grafana-dashboard.json`
- `monitoring/alerts.yml`

**Context & Learnings**:
- TBD

---

### Step 9: Testing & Documentation
**Goal**: Ensure reliability and maintainability

**Tasks**:
- [ ] Load testing scripts
- [ ] API documentation
- [ ] README with setup instructions

**Key Files**:
- `scripts/load-test.sh`
- `README.md`
- `docs/API.md`

**Context & Learnings**:
- TBD

---

## Technical Decisions & Architecture

### Rate Limiting Strategy
- **Type**: IP-based per route
- **Storage**: In-memory (single instance)
- **Tiers**:
  - `/api/rollup/*`: 1000 req/min per IP
  - `/api/continuum/grpc/*`: 500 req/min per IP
  - `/api/continuum/rest/*`: 2000 req/min per IP
- **Future**: Redis-backed for multi-instance deployments

### CORS Configuration
- **Allowed Origins**: Whitelisted domains only
- **Credentials**: Allowed
- **Methods**: GET, POST, OPTIONS
- **Headers**: Content-Type, Authorization

### Monitoring Metrics
- Request rate (req/sec)
- Latency percentiles (p50, p95, p99)
- Error rates (4xx, 5xx)
- Rate limit hits
- Backend response times

---

## Environment Variables

```bash
# Server
PORT=8080
ENV=production

# CORS
ALLOWED_ORIGINS=https://yourdex.com,https://app.yourdex.com

# Backends
ROLLUP_URL=http://localhost:3000
CONTINUUM_GRPC_URL=localhost:9090
CONTINUUM_REST_URL=http://localhost:8081

# Rate Limiting
RATE_LIMIT_ROLLUP=1000
RATE_LIMIT_CONTINUUM_GRPC=500
RATE_LIMIT_CONTINUUM_REST=2000
```

---

## Questions & Notes

### Questions
- What specific Rollup and Continuum endpoints do you need to proxy?
- Do you have backend services running or should we mock them initially?
- What domain will be used for the DEX?

### Notes
- Using Chi router for its middleware-friendly design
- In-memory rate limiting for single-instance deployment (simple, no external dependencies)
- For horizontal scaling: implement Redis-backed distributed rate limiting
- Structured logging for easy parsing in production
- Prometheus + Grafana for industry-standard monitoring

---

## Resources & References

### Go Libraries
- Chi Router: https://github.com/go-chi/chi
- Rate Limiting: https://pkg.go.dev/golang.org/x/time/rate
- Zap Logging: https://github.com/uber-go/zap
- Prometheus Go Client: https://github.com/prometheus/client_golang

### Learning Resources
- Effective Go: https://go.dev/doc/effective_go
- Chi Middleware Examples: https://github.com/go-chi/chi/tree/master/_examples
- Rate Limiting Patterns: https://blog.logrocket.com/rate-limiting-go-application/

---

*Last Updated: Step 1 + Tests Complete - 100% Coverage! 🎉*
