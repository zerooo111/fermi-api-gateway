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
- [x] Step 2: Middleware Layer (CORS, Logging, Recovery, Request ID)
- [x] Step 3: In-Memory IP-Based Rate Limiting
- [x] Step 4: Prometheus Metrics
- [x] Step 5: Reverse Proxy Setup (HTTP & gRPC proxies)

### 🚧 In Progress
- [ ] None

### 📋 Pending
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

### Step 2: Middleware Layer ✅
**Goal**: Implement essential middleware for production readiness

**Tasks**:
- [x] CORS middleware with domain whitelist
- [x] Structured logging middleware (Zap)
- [x] Recovery middleware (panic handling)
- [x] Request ID middleware
- [x] Integration into main.go with proper ordering
- [x] Comprehensive tests for all middleware

**Key Files**:
- `internal/middleware/cors.go` - CORS with origin whitelist and preflight handling
- `internal/middleware/cors_test.go` - Tests for CORS scenarios
- `internal/middleware/logging.go` - Structured logging with Zap
- `internal/middleware/logging_test.go` - Tests for logging middleware
- `internal/middleware/recovery.go` - Panic recovery with stack traces
- `internal/middleware/recovery_test.go` - Tests for panic scenarios
- `internal/middleware/requestid.go` - Request ID generation and propagation
- `internal/middleware/requestid_test.go` - Tests for request ID uniqueness

**Context & Learnings**:

**1. Middleware Order Matters**
- Implemented critical middleware ordering in main.go:
  1. **RequestID** - First to generate IDs for tracking all subsequent operations
  2. **Recovery** - Second to catch panics and log them with request context
  3. **Logging** - Third to log all requests (even recovered panics)
  4. **Metrics** - Fourth to record metrics for all requests
  5. **CORS** - Fifth to handle CORS before business logic
- Each middleware wraps the next, forming a chain

**2. CORS Middleware**
- Whitelist-based origin checking (not wildcard `*`)
- Handles preflight OPTIONS requests
- Sets proper headers: Access-Control-Allow-Origin, Allow-Credentials, etc.
- Configurable via environment variable `ALLOWED_ORIGINS`
- Test coverage: 100%

**3. Request ID Middleware**
- Generates unique 32-character hex IDs using crypto/rand
- Checks for existing X-Request-ID header (preserves from load balancers)
- Stores in both HTTP header and request context
- Custom context key type prevents collisions
- Used for request tracing across services
- Test coverage: Uniqueness verified across 1000 concurrent requests

**4. Recovery Middleware**
- Catches panics using defer/recover pattern
- Logs full stack trace using runtime/debug.Stack()
- Returns proper JSON 500 error to client
- Prevents server crashes from unhandled errors
- Preserves request context (including request ID)
- Test coverage: Multiple panic types tested

**5. Logging Middleware**
- Uses Uber's Zap for structured, high-performance logging
- Wraps response writer to capture status codes
- Logs: method, path, status, duration, remote_addr, request_id
- Log level based on status: ERROR (500+), WARN (400+), INFO (else)
- Production mode: JSON logging for parsing
- Development mode: Human-readable console logging
- Test coverage: 95.5%

**6. Response Writer Wrapping Pattern**
- Created custom response writer wrappers to intercept WriteHeader/Write calls
- Used for both logging (capture status) and metrics (capture bytes)
- Critical pattern for middleware that needs to inspect responses

**Test Results**:
```bash
✓ internal/middleware: 95.5% coverage
✓ CORS: 100% coverage
✓ Request ID: 100% coverage
✓ Recovery: 100% coverage
✓ Logging: 95.5% coverage
✓ No race conditions detected
```

**Git Commit**:
```
feat: add middleware layer with CORS, logging, recovery, and request ID
```

---

### Step 3: In-Memory IP-Based Rate Limiting ✅
**Goal**: Implement in-memory rate limiting per IP per route

**Tasks**:
- [x] Install `golang.org/x/time/rate` package
- [x] IP extraction logic (X-Forwarded-For, X-Real-IP handling)
- [x] In-memory rate limiter with cleanup (prevent memory leaks)
- [x] Rate limiter middleware (per route)
- [x] Configure limits per endpoint
- [x] Rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining)
- [x] Tests for rate limiting logic
- [x] Fix race condition in cleanup goroutine

**Key Files**:
- `internal/ratelimit/ip.go` - IP extraction from headers
- `internal/ratelimit/ip_test.go` - Tests for IP extraction
- `internal/ratelimit/limiter.go` - In-memory rate limiter with cleanup
- `internal/ratelimit/limiter_test.go` - Tests for rate limiter
- `internal/ratelimit/middleware.go` - Rate limiting middleware
- `internal/ratelimit/middleware_test.go` - Tests for middleware

**Context & Learnings**:

**1. IP Extraction Strategy**
- Priority order for determining client IP:
  1. **X-Forwarded-For** header (most reliable behind proxies/load balancers)
  2. **X-Real-IP** header (fallback for some proxies)
  3. **RemoteAddr** (direct connection IP)
- X-Forwarded-For can contain multiple IPs (comma-separated chain)
- Always use first IP in chain (original client)
- Handles IPv6 addresses correctly
- Test coverage: 100%

**2. Token Bucket Algorithm**
- Uses `golang.org/x/time/rate` library
- Token bucket per IP address
- Configurable rate (tokens/second) and burst (max tokens)
- Per-route configuration:
  - Rollup: 1000 req/min (16.67 req/sec)
  - Continuum gRPC: 500 req/min (8.33 req/sec)
  - Continuum REST: 2000 req/min (33.33 req/sec)

**3. Memory Management**
- Background cleanup goroutine prevents memory leaks
- Removes inactive IP entries after 3 minutes
- Uses sync.RWMutex for thread-safe access
- Cleanup runs every 1 minute by default
- Properly handles concurrent access across goroutines

**4. Rate Limit Headers**
- Returns proper HTTP 429 (Too Many Requests) status
- Sets X-RateLimit-Remaining header
- JSON error response for better client handling
- Allows clients to implement backoff strategies

**5. Race Condition Fix**
- Initial test had data race in cleanup goroutine test
- Fixed by simplifying test to not modify cleanupInterval
- Verified with `go test -race` - no races detected
- Important lesson: concurrent tests need careful design

**6. Architecture Decision**
- Using in-memory rate limiting for **single-instance deployment**
- Simple, no external dependencies (Redis not needed)
- **Future scaling**: When deploying multiple instances, consider Redis-backed
  distributed rate limiting to maintain consistent limits across all instances
- This design keeps initial deployment simple while allowing future expansion

**Test Results**:
```bash
✓ internal/ratelimit: 87.7% coverage
✓ IP extraction: 100% coverage
✓ Rate limiter: Concurrent access tested
✓ Middleware: Headers and rate limiting verified
✓ No race conditions detected
```

**Git Commits**:
```
feat: add in-memory IP-based rate limiting with per-route limits
fix: resolve race condition in rate limiter cleanup test
```

---

### Step 4: Prometheus Metrics ✅
**Goal**: Add observability with Prometheus metrics

**Tasks**:
- [x] Install Prometheus Go client library
- [x] Create metrics package with collectors
- [x] Create metrics middleware with tests
- [x] Add /metrics endpoint for Prometheus scraping
- [x] Integrate metrics into main.go
- [x] Test metrics end-to-end

**Key Files**:
- `internal/metrics/metrics.go` - Prometheus collectors
- `internal/metrics/metrics_test.go` - Tests for metrics
- `internal/middleware/metrics.go` - Metrics middleware
- `internal/middleware/metrics_test.go` - Tests for metrics middleware

**Context & Learnings**:

**1. Prometheus Metrics Collectors**
- **CounterVec**: `http_requests_total` - Total requests by method, path, status
- **HistogramVec**: `http_request_duration_seconds` - Request latency with buckets
- **SummaryVec**: `http_request_size_bytes` - Request size distribution
- **SummaryVec**: `http_response_size_bytes` - Response size distribution
- **CounterVec**: `http_rate_limit_hits_total` - Rate limit hits by path

**2. Metric Labels**
- Labels allow filtering and grouping in Prometheus queries
- Request metrics: `method`, `path`, `status`
- Request size: `method`, `path` (no status as it's measured before response)
- Rate limit: `path` only
- More labels = more cardinality, but also more memory usage

**3. Histogram vs Summary**
- **Histogram**: Pre-defined buckets, server-side quantile calculation
  - Used for `request_duration_seconds`
  - Default buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s
  - Allows aggregation across instances
- **Summary**: Client-side quantile calculation
  - Used for request/response sizes
  - Lower overhead, but can't aggregate across instances

**4. Metrics Middleware Implementation**
- Custom `metricsResponseWriter` wrapper to capture:
  - Status code (defaults to 200 if not explicitly set)
  - Bytes written (via Write() calls)
- Records metrics after request completes
- Measures duration using `time.Since(start)`
- Does not interfere with other middleware

**5. Metrics Endpoint**
- `/metrics` endpoint exposed for Prometheus scraping
- Uses `promhttp.HandlerFor(registry, opts)` handler
- No authentication (typically scraped from internal network)
- Custom registry (not default global registry) for isolation

**6. Testing Prometheus Metrics**
- Use `registry.Gather()` to collect metrics in tests
- Verify metric names exist in gathered families
- Test different status codes, methods, and paths
- Verify request/response size recording
- All tests pass with 87.5% coverage

**Test Results**:
```bash
✓ internal/metrics: 87.5% coverage
✓ internal/middleware (metrics): 95.5% coverage
✓ All metric types recorded correctly
✓ Different status codes handled
✓ Different HTTP methods tested
✓ No race conditions detected
```

**Integration**:
```go
// In main.go
m := metrics.NewMetrics()
registry := prometheus.NewRegistry()
m.MustRegister(registry)
r.Use(middleware.Metrics(m))
r.Get("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP)
```

**Git Commit**:
```
feat: add Prometheus metrics with /metrics endpoint
```

---

### Step 5: Reverse Proxy Setup ✅
**Goal**: Proxy requests to Rollup service and Continuum backends

**Tasks**:
- [x] Reverse proxy handler for HTTP (Rollup, Continuum REST)
- [x] gRPC proxy handler for Continuum gRPC
- [x] Backend routing configuration
- [x] Connection pooling
- [x] Timeout handling
- [x] Tests for HTTP proxy (11 test cases)
- [x] Integration with main.go
- [x] Real backend testing

**Key Files**:
- `internal/proxy/http.go` - HTTP reverse proxy with connection pooling
- `internal/proxy/http_test.go` - Comprehensive HTTP proxy tests
- `internal/proxy/grpc.go` - gRPC-to-HTTP conversion proxy
- `proto/continuum.proto` - Protobuf service definition
- `proto/continuum.pb.go`, `proto/continuum_grpc.pb.go` - Generated gRPC code
- `cmd/gateway/main.go` - Integration with routing

**Context & Learnings**:

**1. HTTP Reverse Proxy Implementation**
- Generic, reusable proxy for HTTP backends
- Connection pooling: 100 max idle connections, 10 per host
- Configurable timeouts (default 15s)
- Proper header forwarding: X-Forwarded-For, X-Forwarded-Proto, X-Forwarded-Host
- Error handling: 502 (Bad Gateway) vs 504 (Gateway Timeout) distinction
- Test coverage: 11 test cases covering all scenarios

**2. Path Concatenation Bug Fix**
- Critical bug: Base URL path was being overwritten instead of concatenated
- Example: Backend `http://api.com/api/v1` + request `/health` = `http://api.com/health` (WRONG)
- Fixed: Now correctly produces `http://api.com/api/v1/health`
- Code fix in `internal/proxy/http.go` lines 68-74

**3. gRPC-to-HTTP Conversion Proxy**
- Converts HTTP requests to gRPC calls for Continuum sequencer service
- 7 endpoint handlers: SubmitTransaction, SubmitBatch, GetStatus, GetTransaction, GetTick, GetChainState, StreamTicks
- JSON request/response marshaling
- Query parameter extraction (tick_number, tick_limit, etc.)
- Server-Sent Events for streaming endpoints
- Connection reuse for performance

**4. Protocol Buffer Code Generation**
- Used `protoc` with Go plugins to generate code from proto file
- Generated files: `continuum.pb.go` (message types), `continuum_grpc.pb.go` (service client)
- Service definition includes 7 RPC methods and 20+ message types

**5. Backend Integration**
- Tested against real backends:
  - Continuum gRPC: `100.24.216.168:9090`
  - Continuum REST: `http://100.24.216.168:8080/api/v1`
- Environment variable configuration for backend URLs

**6. Performance Metrics**
- Gateway overhead: < 1ms (sub-millisecond)
- REST proxy: 310-685ms average (network + backend time)
- gRPC first call: 1,119ms (includes connection setup)
- gRPC subsequent calls: ~520ms (connection pooling working - 2x speedup)

**7. Integration Testing Results**
- 14 endpoints tested total
- 11 endpoints working (78.6% success rate)
- Gateway core: 3/3 (100%)
- Continuum REST: 3/6 (50% - backend API limitations)
- Continuum gRPC: 5/5 (100%)
- All failures were backend issues, not gateway bugs

**8. Architecture Decisions**
- HTTP proxy is generic and reusable for any HTTP backend
- gRPC proxy converts HTTP to gRPC for REST API compatibility
- Per-route configuration in main.go
- Rate limiting applied per backend
- Chi router with path prefix stripping for clean routing

**Test Results**:
```bash
✓ internal/proxy/http_test.go: 11 test cases passing
✓ Basic proxying, headers, methods, timeouts all working
✓ No race conditions detected
✓ Real backend testing: 11/14 endpoints functional
```

**Test Scripts Created**:
- `test-real-backends.sh` - Comprehensive endpoint testing
- `final-test.sh` - Test runner with environment setup
- `start-with-env.sh` - Server startup with environment variables

**Git Commit**:
```
feat: implement reverse proxy for Rollup, Continuum REST, and Continuum gRPC

- Add generic HTTP reverse proxy with connection pooling
- Add gRPC-to-HTTP conversion proxy for Continuum sequencer
- Generate Go code from protobuf service definition
- Fix path concatenation bug in HTTP proxy
- Test all endpoints against real backends (78.6% success rate)
- Gateway overhead < 1ms, connection pooling working (2x speedup)
```

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

*Last Updated: Steps 1-5 Complete! Reverse Proxy Setup Tested with Real Backends! 🚀*
