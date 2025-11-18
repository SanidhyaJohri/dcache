# DCache Project Setup Guide - macOS

## Table of Contents
1. [Docker vs Non-Docker Development](#docker-vs-non-docker-development)
2. [Prerequisites](#prerequisites)
3. [Installation Steps](#installation-steps)
4. [Project Structure Setup](#project-structure-setup)
5. [Docker Configuration Files](#docker-configuration-files)
6. [Daily Workflow](#daily-workflow)
7. [Troubleshooting](#troubleshooting)

---

## Docker vs Non-Docker Development

### Non-Docker (Native) Development

**Architecture:**
```
Your Mac
├── Go installed directly
├── Redis/other services running locally  
├── Your code accessing localhost ports
└── Dependencies in your system
```

**Pros:**
- ✅ Faster compilation and execution
- ✅ Easier debugging with native tools
- ✅ Direct access to system resources
- ✅ Lower resource overhead
- ✅ IDE integration works seamlessly

**Cons:**
- ❌ Version conflicts between projects
- ❌ "Works on my machine" problems
- ❌ Complex cleanup when switching projects
- ❌ Hard to simulate multi-node clusters locally
- ❌ Dependency pollution in your system

**Best For:**
- Single-node applications
- Quick prototyping
- When performance is critical
- Simple applications with few dependencies

---

### Docker-Based Development

**Architecture:**
```
Your Mac
├── Docker Desktop
└── Containers
    ├── Go development container
    ├── Node 1 container
    ├── Node 2 container
    └── Node 3 container
```

**Pros:**
- ✅ **Isolated environments** - Each project has its own dependencies
- ✅ **Multi-node testing** - Easily run 5+ cache nodes locally
- ✅ **Network simulation** - Test network partitions, latency
- ✅ **Consistent environment** - Same setup as production
- ✅ **Easy cleanup** - Just delete containers
- ✅ **Version management** - Different Go versions per project
- ✅ **Team consistency** - Everyone has identical environment

**Cons:**
- ❌ Slight performance overhead (5-10% on Mac M1/M2)
- ❌ Extra layer of complexity initially
- ❌ More disk space usage (Docker images)
- ❌ Requires Docker Desktop running
- ❌ Some debugging tools need extra configuration

**Best For:**
- Distributed systems (like dcache)
- Microservices development
- Team projects
- Testing failure scenarios
- Production-like development

---

## Why Docker for DCache?

For the dcache project specifically, Docker is ideal because:

1. **Multi-node testing**: You need to test 3-5 nodes communicating
2. **Network failures**: Simulate network partitions between nodes
3. **Failure scenarios**: Kill nodes and test recovery
4. **Consistent hashing**: Test key redistribution with node changes
5. **Resource limits**: Test memory constraints per node
6. **Monitoring stack**: Run Prometheus/Grafana alongside

---

## Prerequisites

- **macOS Version**: 11.0 (Big Sur) or later
- **Available Disk Space**: 10GB minimum
- **RAM**: 8GB minimum (16GB recommended)
- **Internet Connection**: For downloading tools and dependencies

---

## Installation Steps

### Step 1: Install Homebrew
```bash
# Check if Homebrew is already installed
brew --version

# If not installed, run this command:
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# After installation, follow the instructions to add Homebrew to PATH
# For Apple Silicon Macs (M1/M2):
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
eval "$(/opt/homebrew/bin/brew shellenv)"

# Verify installation
brew --version
```

### Step 2: Install Core Development Tools
```bash
# Install Git for version control
brew install git

# Install VS Code (recommended editor for Go)
brew install --cask visual-studio-code

# Install essential command-line tools
brew install wget curl jq make

# Install tree for viewing directory structures
brew install tree
```

### Step 3: Install Docker Desktop
```bash
# Install Docker Desktop for Mac
brew install --cask docker

# After installation:
# 1. Launch Docker Desktop from Applications folder
# 2. Complete the Docker Desktop setup wizard
# 3. Wait for Docker icon in menu bar to show "Docker Desktop is running"

# Verify Docker installation
docker --version
docker compose version

# Test Docker is working
docker run hello-world
```

### Step 4: Install Go (Both Native and for Docker)
```bash
# Install Go locally (provides IDE support even when using Docker)
brew install go

# Verify Go installation
go version

# Configure Go environment
echo 'export GOPATH=$HOME/go' >> ~/.zshrc
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.zshrc
echo 'export GO111MODULE=on' >> ~/.zshrc

# Reload shell configuration
source ~/.zshrc

# Create Go workspace directory
mkdir -p $GOPATH/{bin,src,pkg}
```

### Step 5: Install Additional Development Tools
```bash
# Load testing tool for benchmarking
brew install k6

# Alternative load testing tool
brew install wrk

# Network debugging tools
brew install telnet netcat

# Container inspection tool
brew install dive  # Analyze docker image layers

# Protocol buffer compiler (for future use)
brew install protobuf

# Redis CLI (for comparison/testing)
brew install redis

# JSON processor for API testing
brew install httpie

# gRPC testing tool
brew install grpcurl
```

### Step 6: Install VS Code Extensions
```bash
# Go language support
code --install-extension golang.go

# Docker support
code --install-extension ms-azuretools.vscode-docker

# Makefile support
code --install-extension ms-vscode.makefile-tools

# Protocol Buffers support
code --install-extension zxh404.vscode-proto3

# Go code outline
code --install-extension 766b.go-outliner

# Git history viewer
code --install-extension donjayamanne.githistory

# YAML support for Docker Compose
code --install-extension redhat.vscode-yaml

# Environment file support
code --install-extension mikestead.dotenv
```

---

## Project Structure Setup

### Step 1: Create Project Directory
```bash
# Create project directory
mkdir -p ~/projects/dcache
cd ~/projects/dcache

# Initialize Git repository
git init

# Initialize Go module (replace with your GitHub username)
go mod init github.com/yourusername/dcache

# Create initial directory structure
mkdir -p cmd/{server,cli,benchmark}
mkdir -p internal/{cache,cluster,protocol,replication,storage}
mkdir -p pkg/client
mkdir -p configs
mkdir -p scripts
mkdir -p deployments/{docker,kubernetes}
mkdir -p test/{unit,integration,benchmark}
mkdir -p docs
mkdir -p examples

# Create .gitignore
cat > .gitignore << 'EOF'
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/bin/
/tmp/

# Test binary, built with go test -c
*.test

# Output of go coverage tool
*.out
coverage.html

# Go workspace files
go.work

# Dependency directories
vendor/

# IDE directories
.idea/
.vscode/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# Environment files
.env
.env.local

# Docker volumes
data/

# Log files
*.log
logs/
EOF
```

---

## Docker Configuration Files

### Dockerfile.dev (Development with Hot Reload)
```dockerfile
# Development Dockerfile with hot reload support
FROM golang:1.21-alpine AS dev

# Install system dependencies
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev \
    curl \
    bash

# Install Go development tools
RUN go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install golang.org/x/tools/gopls@latest && \
    go install github.com/cosmtrek/air@latest

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.* ./
RUN go mod download

# Expose ports
EXPOSE 8080 2345 6060

# Use air for hot reload
CMD ["air"]
```

### docker-compose.yml (Multi-Node Development)
```yaml
version: '3.8'

services:
  # Development container with hot reload
  dev:
    build:
      context: .
      dockerfile: Dockerfile.dev
    volumes:
      - .:/app
      - go-cache:/go/pkg/mod
    ports:
      - "8080:8080"      # Application port
      - "2345:2345"      # Delve debugger port
      - "6060:6060"      # pprof port
    environment:
      - CGO_ENABLED=0
      - GOOS=linux
      - GOARCH=amd64
      - ENV=development
    networks:
      - dcache-net

  # Cache Node 1
  node1:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "9001:8080"
      - "9101:9090"  # Metrics
    environment:
      - NODE_ID=node1
      - NODE_PORT=8080
      - CLUSTER_NODES=node1:8080,node2:8080,node3:8080
      - REPLICATION_FACTOR=3
    networks:
      - dcache-net
    depends_on:
      - consul

  # Cache Node 2
  node2:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "9002:8080"
      - "9102:9090"
    environment:
      - NODE_ID=node2
      - NODE_PORT=8080
      - CLUSTER_NODES=node1:8080,node2:8080,node3:8080
      - REPLICATION_FACTOR=3
    networks:
      - dcache-net
    depends_on:
      - consul

  # Cache Node 3
  node3:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "9003:8080"
      - "9103:9090"
    environment:
      - NODE_ID=node3
      - NODE_PORT=8080
      - CLUSTER_NODES=node1:8080,node2:8080,node3:8080
      - REPLICATION_FACTOR=3
    networks:
      - dcache-net
    depends_on:
      - consul

  # Service Discovery (optional)
  consul:
    image: consul:latest
    ports:
      - "8500:8500"
      - "8600:8600/udp"
    command: agent -dev -ui -client=0.0.0.0
    networks:
      - dcache-net

  # Monitoring - Prometheus
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./configs/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    networks:
      - dcache-net

  # Monitoring - Grafana
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_INSTALL_PLUGINS=redis-datasource
    volumes:
      - grafana-data:/var/lib/grafana
      - ./configs/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./configs/grafana/datasources:/etc/grafana/provisioning/datasources
    networks:
      - dcache-net

  # Log Aggregation (optional)
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
    volumes:
      - ./configs/loki.yml:/etc/loki/local-config.yaml
      - loki-data:/loki
    networks:
      - dcache-net

volumes:
  go-cache:
  prometheus-data:
  grafana-data:
  loki-data:

networks:
  dcache-net:
    driver: bridge
    ipam:
      config:
        - subnet: 172.28.0.0/16
```

### .air.toml (Hot Reload Configuration)
```toml
# Air configuration for hot reload
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/server"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "node_modules"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_error = true

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = true

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
```

### Makefile (Build Automation)
```makefile
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
```

### Initial main.go
```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
)

// Version information
var (
    Version   = "dev"
    BuildTime = "unknown"
)

// HealthResponse represents the health check response
type HealthResponse struct {
    Status    string    `json:"status"`
    Version   string    `json:"version"`
    NodeID    string    `json:"node_id"`
    Timestamp time.Time `json:"timestamp"`
}

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    nodeID := os.Getenv("NODE_ID")
    if nodeID == "" {
        nodeID = "dev"
    }

    fmt.Printf("Starting dcache server...\n")
    fmt.Printf("Version: %s\n", Version)
    fmt.Printf("Node ID: %s\n", nodeID)
    fmt.Printf("Port: %s\n", port)

    // Health check endpoint
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        response := HealthResponse{
            Status:    "healthy",
            Version:   Version,
            NodeID:    nodeID,
            Timestamp: time.Now(),
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    })

    // Basic cache endpoints (placeholder)
    http.HandleFunc("/cache/", func(w http.ResponseWriter, r *http.Request) {
        key := r.URL.Path[len("/cache/"):]
        
        switch r.Method {
        case http.MethodGet:
            w.Write([]byte(fmt.Sprintf("GET key: %s from node: %s\n", key, nodeID)))
        case http.MethodPut, http.MethodPost:
            w.Write([]byte(fmt.Sprintf("SET key: %s on node: %s\n", key, nodeID)))
        case http.MethodDelete:
            w.Write([]byte(fmt.Sprintf("DELETE key: %s from node: %s\n", key, nodeID)))
        default:
            w.WriteHeader(http.StatusMethodNotAllowed)
        }
    })

    log.Printf("Server starting on port %s", port)
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatal(err)
    }
}
```

---

## Daily Workflow

### Starting Your Development Session
```bash
# 1. Navigate to project
cd ~/projects/dcache

# 2. Start development container with hot reload
make dev

# Container starts with file watching - any changes auto-reload
```

### Testing Your Code
```bash
# Run tests in another terminal
make test

# Run specific test
docker compose run --rm dev go test -v ./internal/cache

# Run with race detection
docker compose run --rm dev go test -race ./...
```

### Working with Multiple Nodes
```bash
# Start 3-node cluster
make cluster

# Test with curl
curl http://localhost:9001/health
curl http://localhost:9002/health
curl http://localhost:9003/health

# Watch cluster logs
make cluster-logs
```

### Debugging
```bash
# Shell into dev container
make shell

# Inside container, run dlv debugger
dlv debug ./cmd/server

# Or attach to running process
dlv attach <pid>
```

### Monitoring Performance
```bash
# Start monitoring stack
make monitoring

# Access Grafana at http://localhost:3000
# Default credentials: admin/admin
```

### Ending Your Session
```bash
# Stop all containers
docker compose down

# Clean up everything (including volumes)
make clean
```

---

## Troubleshooting

### Common Issues and Solutions

#### 1. Docker Desktop Not Starting
```bash
# Reset Docker Desktop
killall Docker
rm -rf ~/Library/Containers/com.docker.docker
rm -rf ~/Library/Application\ Support/Docker\ Desktop
rm -rf ~/Library/Group\ Containers/group.com.docker
rm -rf ~/.docker

# Reinstall
brew reinstall --cask docker
```

#### 2. Port Already in Use
```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>

# Or change port in docker-compose.yml
```

#### 3. Go Module Issues
```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
go mod download

# In Docker
docker compose run --rm dev go mod tidy
```

#### 4. Container Can't Find Files
```bash
# Ensure Docker has file system access
# Docker Desktop → Preferences → Resources → File Sharing
# Add your project directory
```

#### 5. Slow Performance on Mac
```bash
# Use cached mounts in docker-compose.yml
volumes:
  - .:/app:cached  # Add :cached for better performance
```

#### 6. Memory Issues
```bash
# Increase Docker Desktop memory
# Docker Desktop → Preferences → Resources
# Increase Memory to 4GB minimum
```

---

## Performance Tips

### Docker on macOS
1. **Use named volumes** for dependencies (go modules)
2. **Use :cached or :delegated** mount flags
3. **Exclude unnecessary files** in .dockerignore
4. **Increase Docker resources** in preferences

### Development Speed
1. **Use hot reload** (air) during development
2. **Run tests in parallel**: `go test -parallel 4`
3. **Cache Docker layers**: Order Dockerfile commands properly
4. **Use multi-stage builds** for smaller images

---

## Security Best Practices

1. **Never commit .env files** - Use .env.example
2. **Use specific versions** in Dockerfile, not :latest
3. **Run containers as non-root** user
4. **Scan images for vulnerabilities**: `docker scan dcache:latest`
5. **Use secrets management** for production configs

---

## Next Steps

1. ✅ Complete all installation steps
2. ✅ Verify Docker and Go are working
3. ✅ Run `make init` to set up project
4. ✅ Start with `make dev` and verify health endpoint
5. ✅ Begin Week 1: Implement LRU cache in `internal/cache/`

---

## Useful Resources

- **Go Documentation**: https://go.dev/doc/
- **Docker Documentation**: https://docs.docker.com/
- **Distributed Systems Course**: https://www.youtube.com/playlist?list=PLrw6a1wE39_tb2fErI4-WkMbsvGQk9_UB
- **Consistent Hashing**: https://www.toptal.com/big-data/consistent-hashing
- **Raft Consensus**: https://raft.github.io/
- **Redis Design**: https://redis.io/topics/internals

---

## Support and Questions

If you encounter issues not covered here:

1. Check Docker Desktop logs: `~/Library/Containers/com.docker.docker/Data/log/`
2. Check Go module issues: `go mod why -m <module>`
3. Use `docker compose logs <service>` to debug specific containers
4. Enable verbose logging: `COMPOSE_VERBOSE=true docker compose up`

---

Generated on: $(date)
DCache Project - Distributed In-Memory Cache System
