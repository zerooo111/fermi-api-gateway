.PHONY: help install build run dev clean test gateway ingester ingester-debug

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
	./bin/gateway

dev: ## Run in development mode (with go run). Usage: make dev PORT=3000
	@if [ -n "$(PORT)" ]; then \
		PORT=$(PORT) go run ./cmd/gateway/main.go; \
	else \
		go run ./cmd/gateway/main.go; \
	fi

clean: ## Clean build artifacts
	rm -rf bin/
	go clean

test: ## Run tests
	go test -v ./...

test-cover: ## Run tests with coverage
	go test -v -cover ./...

test-race: ## Run tests with race detector
	go test -race ./...

test-bench: ## Run benchmark tests
	go test -bench=. -benchmem ./...

test-all: ## Run all tests (coverage + race + bench)
	@echo "Running tests with coverage..."
	@go test -v -cover ./...
	@echo "\nRunning tests with race detector..."
	@go test -race ./...
	@echo "\nRunning benchmarks..."
	@go test -bench=. -benchmem ./...

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
