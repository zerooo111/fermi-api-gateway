# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Fermi API Gateway - A high-performance Go API gateway for a Solana hybrid DEX, targeting 50k-200k req/sec with comprehensive monitoring. Built following Test-Driven Development (TDD) principles.

## Development Commands

### Core Development
```bash
make dev              # Run in development mode (fastest for iteration)
make build            # Build binary to bin/gateway
make run              # Build and run
make fmt              # Format code (run before committing)
```

### Testing (Critical - TDD Project)
```bash
make test             # Run all tests
make test-cover       # Run tests with coverage report
make test-race        # Run with race detector (detects concurrency issues)
make test-bench       # Run benchmark tests
make test-all         # Run everything (coverage + race + benchmarks)
```

**Testing Philosophy**: This project follows strict TDD. For any new feature:
1. Write tests FIRST (RED phase)
2. Write minimal code to pass tests (GREEN phase)
3. Refactor while keeping tests green (REFACTOR phase)

Current test coverage: 100% on all packages (config, health). Maintain this standard.

### Running a Single Test
```bash
# Run specific test function
go test -v ./internal/config -run TestLoad

# Run specific package
go test -v ./internal/health

# Run with race detector on specific package
go test -race ./internal/middleware
```

## Code Architecture

### Request Flow
```
Client → Nginx (SSL, caching) → HTTP Server (main.go) → Chi Router → [Middleware Stack] → Handler → Backend/Response
                                                                            ↓
                                                                    CORS → Logging → Recovery → RateLimit (in-memory) → Metrics
```

### Key Architecture Decisions

**1. Standard Go Project Layout**
- `cmd/gateway/` - Application entry point with graceful shutdown
- `internal/` - Private packages (cannot be imported externally)
  - `config/` - Environment-based configuration (12-factor methodology)
  - `health/` - Health check handlers
  - `middleware/` - HTTP middleware (CORS, logging, etc.) [Coming soon]
  - `ratelimit/` - In-memory rate limiting [Coming soon]
  - `proxy/` - Reverse proxy to backends [Coming soon]
  - `metrics/` - Prometheus metrics [Coming soon]

**2. Chi Router**
- Chosen for stdlib compatibility and composable middleware
- All routes use standard `http.HandlerFunc` interface
- Middleware is applied using `r.Use()` or per-route

**3. Configuration Management**
- All config via environment variables (`.env.example` template)
- Centralized in `internal/config/config.go`
- Helper functions: `getEnv`, `getEnvInt`, `getEnvSlice`
- No config files - follows 12-factor app principles

**4. Graceful Shutdown Pattern**
- Main goroutine blocks on signal channels
- SIGTERM/SIGINT trigger 30-second graceful shutdown
- In-flight requests complete before shutdown
- Critical for zero-downtime deployments

**5. HTTP Server Configuration**
```go
ReadTimeout:  15s   // Prevent slow client DoS
WriteTimeout: 15s   // Prevent slow write DoS
IdleTimeout:  60s   // Keep-alive connection timeout
```

## Important Patterns

### Test-Driven Development
- **Test files**: Next to source (`foo.go` → `foo_test.go`)
- **Table-driven tests**: Use `t.Run()` subtests for multiple cases
- **HTTP testing**: Use `httptest.NewRequest()` and `httptest.NewRecorder()`
- **Coverage target**: 80%+ on critical paths (currently at 100%)
- **Benchmark naming**: Prefix with `Benchmark` (e.g., `BenchmarkHealthHandler`)

### Error Handling
```go
// In tests: t.Fatal() for setup failures, t.Error() for test failures
if err != nil {
    t.Fatalf("setup failed: %v", err)
}

// In production code: Return errors, don't panic
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### Middleware Pattern (Chi)
```go
// Middleware wraps handlers
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Before handler
        next.ServeHTTP(w, r)
        // After handler
    })
}

// Apply globally
r.Use(MyMiddleware)

// Apply to specific routes
r.With(MyMiddleware).Get("/path", handler)
```

## Environment Configuration

Key variables (see `.env.example` for full list):
- `PORT` - Server port (default: 8080)
- `ENV` - Environment: development/staging/production
- `ALLOWED_ORIGINS` - Comma-separated CORS whitelist
- `ROLLUP_URL` - Rollup service endpoint
- `CONTINUUM_GRPC_URL` - Continuum gRPC endpoint
- `CONTINUUM_REST_URL` - Continuum REST API endpoint
- `RATE_LIMIT_*` - Per-route rate limits (requests/min)

## Development Context from TODO.md

**Current Status**: Step 1 Complete (Basic HTTP server + tests)
- ✅ HTTP server with Chi router
- ✅ Health endpoints (`/health`, `/ready`)
- ✅ Environment-based configuration
- ✅ Graceful shutdown
- ✅ 100% test coverage with benchmarks (~2.2M req/sec achieved)

**Next Steps** (refer to TODO.md for details):
- Step 2: Middleware Layer (CORS, logging, recovery, request IDs)
- Step 3: In-memory IP-based rate limiting (golang.org/x/time/rate)
- Step 4: Prometheus metrics
- Step 5: Reverse proxy to backends
- Step 6-9: Deployment, monitoring, documentation

**Performance Benchmarks** (Apple M4):
- Health endpoint: 2.2M requests/second per core
- Latency: 530ns per request
- Memory: 1153 bytes per request

## Code Style Guidelines

1. **Follow Go conventions**: Run `make fmt` before committing
2. **Write tests first**: New features must include tests before implementation
3. **Document exported functions**: Use Go doc comments
4. **Keep handlers thin**: Business logic in separate packages
5. **Use context for cancellation**: Pass `context.Context` for operations that can timeout
6. **Structured errors**: Wrap errors with context using `fmt.Errorf("%w", err)`

## Common Development Tasks

### Adding a New HTTP Endpoint
1. Write tests in appropriate `_test.go` file (TDD!)
2. Create handler function returning `http.HandlerFunc`
3. Register route in `cmd/gateway/main.go`
4. Run `make test` to verify
5. Run `make test-race` to check for concurrency issues

### Adding Middleware
1. Write tests in `internal/middleware/foo_test.go`
2. Create middleware following Chi pattern
3. Apply using `r.Use()` or `r.With()`
4. Verify with `make test-cover`

### Modifying Configuration
1. Update structs in `internal/config/config.go`
2. Add to `Load()` function with `getEnv*` helper
3. Write tests (table-driven for edge cases)
4. Update `.env.example`
5. Update README.md configuration section

## Dependencies

- **Chi Router** (`github.com/go-chi/chi/v5`) - HTTP routing and middleware
- **Standard library only** - No external logging/testing frameworks yet
- **Future**: golang.org/x/time/rate (rate limiting), zap (structured logging), prometheus client

## Rate Limiting Strategy (Planned)

**Current approach**: In-memory rate limiting for single-instance deployment
- IP-based per route
- Uses `golang.org/x/time/rate` package
- Route-specific limits:
  - `/api/rollup/*`: 1000 req/min
  - `/api/continuum/grpc/*`: 500 req/min
  - `/api/continuum/rest/*`: 2000 req/min

**Future scaling**: When deploying multiple gateway instances, consider implementing Redis-backed distributed rate limiting to maintain consistent limits across all instances.

## Testing Best Practices

1. **Environment tests**: Use `os.Clearenv()` and defer cleanup
2. **HTTP tests**: Use `httptest` package, not real HTTP server
3. **Table-driven**: Multiple test cases in slice of structs
4. **Descriptive names**: Test names should explain what's being tested
5. **Race detector**: Always run `make test-race` for concurrent code
6. **Benchmarks**: Add for performance-critical paths

## Performance Considerations

- **Target throughput**: 50k-200k req/sec
- **Target latency**: <10ms p95 (excluding backend)
- **Connection pooling**: For Redis and backend services
- **Keep-alive**: Use HTTP keep-alive for backend connections
- **Goroutine safety**: All handlers must be concurrent-safe

## Deployment

- Target platform: AWS EC2
- Reverse proxy: Nginx (SSL termination, security headers)
- Process manager: systemd service
- Monitoring: Prometheus + Grafana (planned)

See `TODO.md` for detailed implementation steps and learnings from each development phase.
- Commit after each task with conventional git message under 100 characters