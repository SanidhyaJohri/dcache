# Makefile for dcache project
.DEFAULT_GOAL := help

# Variables
DOCKER_IMAGE := dcache
VERSION := $(shell git describe --tags --always --dirty)
GO_FILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: help
help: ## Display this help message
	@echo "DCache Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development Commands
.PHONY: dev
dev: ## Start development environment with hot reload
	docker compose up dev

.PHONY: cluster
cluster: ## Start 3-node cluster for testing
	docker compose up -d node1 node2 node3
	@echo "Cluster started. Nodes available at:"
	@echo "  - Node 1: http://localhost:9001"
	@echo "  - Node 2: http://localhost:9002"
	@echo "  - Node 3: http://localhost:9003"

.PHONY: cluster-logs
cluster-logs: ## Follow logs from all cluster nodes
	docker compose logs -f node1 node2 node3

.PHONY: monitoring
monitoring: ## Start monitoring stack (Prometheus + Grafana)
	docker compose up -d prometheus grafana
	@echo "Monitoring started:"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Grafana: http://localhost:3000 (admin/admin)"

.PHONY: full-stack
full-stack: ## Start everything (cluster + monitoring)
	docker compose up -d
	@echo "Full stack started!"

# Build Commands
.PHONY: build
build: ## Build production Docker image
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

.PHONY: build-dev
build-dev: ## Build development Docker image
	docker build -f Dockerfile.dev -t $(DOCKER_IMAGE):dev .

# Testing Commands
.PHONY: test
test: ## Run all tests in Docker
	docker compose run --rm dev go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-unit
test-unit: ## Run unit tests only
	docker compose run --rm dev go test -v -short ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	docker compose run --rm dev go test -v -run Integration ./test/integration

.PHONY: bench
bench: ## Run benchmarks
	docker compose run --rm dev go test -bench=. -benchmem ./...

.PHONY: coverage
coverage: test ## Generate test coverage report
	docker compose run --rm dev go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code Quality Commands
.PHONY: lint
lint: ## Run linters
	docker compose run --rm dev golangci-lint run

.PHONY: fmt
fmt: ## Format Go code
	docker compose run --rm dev go fmt ./...

.PHONY: vet
vet: ## Run go vet
	docker compose run --rm dev go vet ./...

# Utility Commands
.PHONY: shell
shell: ## Open shell in development container
	docker compose exec dev sh

.PHONY: logs
logs: ## Show all container logs
	docker compose logs -f

.PHONY: clean
clean: ## Clean up containers, volumes, and temp files
	docker compose down -v
	rm -rf tmp/ coverage.out coverage.html

.PHONY: reset
reset: clean ## Complete reset (removes images too)
	docker compose down --rmi all

.PHONY: status
status: ## Show status of all containers
	docker compose ps

# Database Commands (for future persistence layer)
.PHONY: migrate
migrate: ## Run database migrations
	docker compose run --rm dev go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Rollback database migrations
	docker compose run --rm dev go run ./cmd/migrate down

# Release Commands
.PHONY: release
release: test lint ## Create a release build
	@echo "Creating release $(VERSION)"
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	@echo "Release $(VERSION) built successfully"

# Load Testing Commands
.PHONY: load-test
load-test: ## Run load tests with k6
	k6 run ./test/load/basic.js

.PHONY: stress-test
stress-test: ## Run stress test (high load)
	k6 run --vus 100 --duration 30s ./test/load/stress.js

# Documentation
.PHONY: docs
docs: ## Generate documentation
	docker compose run --rm dev go doc -all > docs/API.md

# Initialize project
.PHONY: init
init: ## Initialize project (first time setup)
	@echo "Initializing dcache project..."
	go mod tidy
	docker compose build
	@echo "Project initialized! Run 'make dev' to start developing."
