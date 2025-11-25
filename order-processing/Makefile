.PHONY: help worker starter greet test clean

help: ## Show this help message
	@echo "Order Processing - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

worker: ## Start the worker
	@echo "Starting worker..."
	go run worker/main.go

starter: ## Start an order workflow (with auto-approve)
	@echo "Starting order workflow..."
	AUTO_APPROVE=true go run starter/main.go

starter-async: ## Start an order workflow asynchronously (manual approval required)
	@echo "Starting order workflow (async - send signals manually)..."
	ASYNC=true go run starter/main.go

test: ## Run tests (placeholder)
	@echo "Running tests..."
	go test -v ./...

tidy: ## Run go mod tidy
	cd .. && go mod tidy

clean: ## Clean build artifacts
	@echo "Cleaning..."
	go clean

build: ## Build worker and starter binaries
	@echo "Building worker..."
	go build -o bin/worker worker/main.go
	@echo "Building starter..."
	go build -o bin/starter starter/main.go
	@echo "Build complete! Binaries in bin/"

.DEFAULT_GOAL := help

