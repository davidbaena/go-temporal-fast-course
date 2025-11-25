.PHONY: help start stop logs worker starter test clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

start: ## Start Temporal server (using Temporal CLI)
	@echo "Starting Temporal development server..."
	@echo "✅ Temporal UI will be available at: http://localhost:8233"
	@echo "✅ Temporal Server: localhost:7233"
	@echo "✅ Database: ./temporal.db (SQLite)"
	@echo ""
	@echo "Run this command in a separate terminal:"
	@echo "  ./start-temporal.sh"
	@echo ""
	@echo "Or run in background:"
	@echo "  make start-bg"

start-bg: ## Start Temporal server in background
	@echo "Starting Temporal server in background..."
	@./start-temporal.sh > temporal.log 2>&1 &
	@echo $$! > temporal.pid
	@echo "Waiting for server to start..."
	@sleep 3
	@echo "✅ Temporal UI: http://localhost:8233"
	@echo "✅ Temporal Server: localhost:7233"
	@echo "✅ Logs: tail -f temporal.log"
	@echo "✅ PID saved to temporal.pid"

stop: ## Stop Temporal server
	@echo "Stopping Temporal server..."
	@if [ -f temporal.pid ]; then \
		kill $$(cat temporal.pid) 2>/dev/null || true; \
		rm temporal.pid; \
		echo "✅ Temporal server stopped"; \
	else \
		pkill -f "temporal server start-dev" || echo "No running Temporal server found"; \
	fi

restart: stop start-bg ## Restart Temporal server

logs: ## View Temporal server logs (if running in background)
	@if [ -f temporal.log ]; then \
		tail -f temporal.log; \
	else \
		echo "No log file found. Start the server with 'make start-bg'"; \
	fi

status: ## Check if Temporal server is running
	@if pgrep -f "temporal server start-dev" > /dev/null; then \
		echo "✅ Temporal server is running"; \
		echo "UI: http://localhost:8233"; \
		echo "Server: localhost:7233"; \
	else \
		echo "❌ Temporal server is not running"; \
		echo "Start it with: make start or make start-bg"; \
	fi

worker: ## Run the worker
	@echo "Starting worker (Ctrl+C to stop)..."
	go run worker/main.go

starter: ## Start a workflow
	@echo "Starting workflow..."
	go run starter/main.go

test: ## Run tests
	go test ./...

tidy: ## Tidy Go modules
	go mod tidy

download: ## Download Go dependencies
	go mod download

clean: ## Clean up everything (WARNING: removes all data)
	@echo "⚠️  This will remove all workflow data and logs. Press Ctrl+C to cancel, Enter to continue..."
	@read _
	@$(MAKE) stop
	@rm -f temporal.db temporal.db-shm temporal.db-wal temporal.log temporal.pid
	@echo "✅ All data cleaned"

list: ## List all workflows using temporal CLI
	temporal workflow list

describe: ## Describe a workflow (usage: make describe ID=workflow_id)
	temporal workflow describe --workflow-id $(ID)

show: ## Show workflow history (usage: make show ID=workflow_id)
	temporal workflow show --workflow-id $(ID)

terminate: ## Terminate a workflow (usage: make terminate ID=workflow_id)
	temporal workflow terminate --workflow-id $(ID) --reason "Manual termination"

cancel: ## Cancel a workflow (usage: make cancel ID=workflow_id)
	temporal workflow cancel --workflow-id $(ID) --reason "Manual cancellation"

ui: ## Open Temporal UI in browser
	@echo "Opening Temporal UI..."
	@open http://localhost:8233 || xdg-open http://localhost:8233 || echo "Please open http://localhost:8233 in your browser"

dev: start-bg ## Start development environment (server + worker)
	@echo "Starting development environment..."
	@echo "Temporal UI: http://localhost:8233"
	@sleep 3
	@$(MAKE) worker

.DEFAULT_GOAL := help

