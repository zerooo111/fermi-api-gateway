.PHONY: help install build run dev clean test gateway ingester ingester-debug test-grpc

# Load .env file if it exists (for shell environment)
ifneq (,$(wildcard .env))
    include .env
    export
endif

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install: ## Install Go dependencies
	go mod download
	go mod tidy

build: ## Build the gateway binary
	go build -o bin/gateway ./cmd/gateway

run: build ## Build and run the gateway
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && ./bin/gateway

dev: ## Run in development mode (with go run)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && go run ./cmd/gateway/main.go

clean: ## Clean build artifacts
	rm -rf bin/
	go clean

test: ## Run tests
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && go test -v ./...

test-cover: ## Run tests with coverage
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && go test -v -cover ./...

test-race: ## Run tests with race detector
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && go test -race ./...

test-bench: ## Run benchmark tests
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && go test -bench=. -benchmem ./...

test-all: ## Run all tests (coverage + race + bench)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && echo "Running tests with coverage..." && go test -v -cover ./... && echo "\nRunning tests with race detector..." && go test -race ./... && echo "\nRunning benchmarks..." && go test -bench=. -benchmem ./...

fmt: ## Format code
	go fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

deps: ## Show dependency graph
	go mod graph

# === Main Services ===

gateway: ## Run API gateway server (from .env)
	@echo "Building and starting API gateway..."
	@go build -o bin/gateway ./cmd/gateway
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && ./bin/gateway

ingester: ## Run tick ingester with TimescaleDB (from .env)
	@echo "Building and starting tick ingester (TimescaleDB mode)..."
	@go build -o bin/tick-ingester ./cmd/tick-ingester
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && ./bin/tick-ingester

ingester-debug: ## Run tick ingester with console output (no database)
	@echo "Building and starting tick ingester (debug mode - console output)..."
	@go build -o bin/tick-ingester ./cmd/tick-ingester
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && OUTPUT_MODE=console OUTPUT_FORMAT=table ./bin/tick-ingester

test-grpc: ## Test gRPC stream connection (diagnostic tool)
	@echo "Building gRPC stream test tool..."
	@go build -o bin/test-grpc-stream ./cmd/test-grpc-stream
	@echo "Running test (use Ctrl+C to stop)..."
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi && ./bin/test-grpc-stream -server "$${CONTINUUM_GRPC_URL:-100.24.216.168:9090}"
