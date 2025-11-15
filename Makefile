.PHONY: help start stop logs worker starter test clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

start: ## Start Temporal server (PostgreSQL + Temporal + UI)
	@echo "Starting Temporal services..."
	docker-compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@echo "✅ Temporal UI: http://localhost:8080"
	@echo "✅ Temporal Server: localhost:7233"

stop: ## Stop Temporal server
	@echo "Stopping Temporal services..."
	docker-compose down

restart: stop start ## Restart Temporal server

logs: ## View Temporal server logs
	docker logs temporal -f

logs-ui: ## View Temporal UI logs
	docker logs temporal-ui -f

logs-db: ## View PostgreSQL logs
	docker logs temporal-postgresql -f

status: ## Check status of all services
	@docker-compose ps

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
	@echo "⚠️  This will remove all workflow data. Press Ctrl+C to cancel, Enter to continue..."
	@read _
	docker-compose down -v
	@echo "✅ All data cleaned"

tctl-list: ## List all workflows using tctl
	docker exec temporal-admin-tools tctl workflow list

tctl-describe: ## Describe a workflow (usage: make tctl-describe ID=workflow_id)
	docker exec temporal-admin-tools tctl workflow describe -w $(ID)

tctl-show: ## Show workflow history (usage: make tctl-show ID=workflow_id)
	docker exec temporal-admin-tools tctl workflow show -w $(ID)

tctl-terminate: ## Terminate a workflow (usage: make tctl-terminate ID=workflow_id)
	docker exec temporal-admin-tools tctl workflow terminate -w $(ID) --reason "Manual termination"

ui: ## Open Temporal UI in browser
	@echo "Opening Temporal UI..."
	@open http://localhost:8080 || xdg-open http://localhost:8080 || echo "Please open http://localhost:8080 in your browser"

dev: start ## Start development environment (server + worker)
	@echo "Starting development environment..."
	@echo "Temporal UI: http://localhost:8080"
	@sleep 3
	@$(MAKE) worker

.DEFAULT_GOAL := help

