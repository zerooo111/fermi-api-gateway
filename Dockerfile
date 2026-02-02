# Multi-stage Dockerfile for Fermi API Gateway
# Optimized for Cloud Run deployment

# ============================================
# Stage 1: Builder
# ============================================
FROM golang:1.24.5-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY proto/ ./proto/

# Build the gateway binary
# CGO_ENABLED=0 for static binary (no libc dependencies)
# -ldflags="-w -s" strips debug info (smaller binary)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o gateway \
    ./cmd/gateway

# ============================================
# Stage 2: Runtime
# ============================================
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /build/gateway /app/gateway

# Set working directory
WORKDIR /app

# Use non-root user (distroless default: uid=65532)
USER nonroot:nonroot

# Cloud Run expects PORT env var (default 8080)
ENV PORT=8080

# Expose port (documentation only for Cloud Run)
EXPOSE 8080

# Health check (optional, Cloud Run uses /health endpoint)
# Uncomment if running locally with docker
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#   CMD ["/app/gateway", "health"] || exit 1

# Run the gateway
ENTRYPOINT ["/app/gateway"]
