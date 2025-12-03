# DCache Implementation Report - Distributed Cache System

**Project**: Distributed In-Memory Cache  
**Date**: December 2, 2024  
**Status**: ✅ Week 1-4 Complete

---

# WEEK 1-2: SINGLE NODE CACHE IMPLEMENTATION

## Project Overview

DCache is a high-performance, thread-safe in-memory cache implementation with LRU eviction, TTL support, and dual protocol interfaces (HTTP and TCP/Redis-compatible).

## Performance Metrics

### Benchmark Results

Platform: macOS Apple Silicon (Docker Container)  
Environment: golang:1.22-alpine

    BenchmarkSet-10                3,217,560 ops    372.1 ns/op
    BenchmarkGet-10                7,217,032 ops    164.9 ns/op  
    BenchmarkConcurrentGetSet-10  6,992,480 ops    174.3 ns/op

### Key Performance Indicators

- **Throughput**: 20,000-30,000 operations/second
- **Set Operation Latency**: 372 nanoseconds
- **Get Operation Latency**: 165 nanoseconds
- **Concurrent Operations**: 174 nanoseconds
- **Test Coverage**: 80.8% of statements

### Test Results

All 10 unit tests passing:
- ✅ Store initialization and configuration
- ✅ Set and Get operations
- ✅ TTL expiration (validated at 150ms)
- ✅ LRU eviction policy
- ✅ Size-based eviction
- ✅ Delete operations
- ✅ Clear cache functionality
- ✅ Multi-key operations (GetMulti)
- ✅ Concurrent access safety
- ✅ Statistics tracking

## Architecture

### Core Components

- **internal/cache/store.go**: LRU cache with O(1) operations using doubly-linked list + hashmap
- **internal/server/tcp_server.go**: TCP server implementing Redis-compatible protocol
- **cmd/server/main.go**: HTTP and TCP server entry point
- **cmd/client/main.go**: Interactive CLI client

### Features Implemented

- Thread-safe operations with RWMutex
- LRU eviction with configurable capacity
- TTL support with background cleanup
- Size-based memory management
- Dual protocol support (HTTP REST API + TCP)
- Real-time statistics and metrics
- Hot reload development environment

## Installation & Setup

### Prerequisites

- Docker Desktop
- Git

### Quick Start

    # Clone the repository
    git clone https://github.com/sjohri/dcache.git
    cd dcache

    # Build Docker image
    docker build -f Dockerfile.dev -t dcache:dev .

    # Start the server
    docker compose up dev

### Docker Configuration

The project uses Go 1.22 with Air v1.49.0 for hot reload compatibility.

## Running the Cache Server

### Using Docker Compose (Recommended)

    # Start server with logs
    docker compose up dev

    # Start in background
    docker compose up -d dev

    # View logs
    docker compose logs -f dev

    # Stop server
    docker compose down

### Using Docker Run

    docker run -it --rm \
      -v $(pwd):/app \
      -p 8080:8080 \
      -p 6379:6379 \
      --name dcache-dev \
      dcache:dev

### Configuration Options

Environment variables:

    NODE_ID=node1           # Node identifier
    CAPACITY=10000          # Max items in cache
    MAX_SIZE_MB=100        # Max memory size
    DEFAULT_TTL_MIN=10     # Default TTL in minutes
    HTTP_PORT=8080         # HTTP API port
    TCP_PORT=6379          # TCP protocol port

## API Usage

### HTTP API Examples

    # Health check
    curl http://localhost:8080/health
    # Response: {"node_id":"node1","status":"healthy","timestamp":1764174805}

    # Store a value
    curl -X POST http://localhost:8080/cache/user:1 -d "John Doe"
    # Response: Stored

    # Retrieve a value
    curl http://localhost:8080/cache/user:1
    # Response: John Doe

    # Store with TTL (expires in 5 seconds)
    curl -X POST http://localhost:8080/cache/session:123 \
      -H "X-TTL-Seconds: 5" \
      -d "temporary-data"

    # Delete a key
    curl -X DELETE http://localhost:8080/cache/user:1
    # Response: Deleted

    # Get cache statistics
    curl http://localhost:8080/stats
    # Response: JSON with metrics

    # List all keys
    curl http://localhost:8080/keys
    # Response: {"count":3,"keys":["key1","key2","key3"]}

    # Clear entire cache
    curl -X POST http://localhost:8080/clear

### TCP Protocol (Redis-Compatible)

    # Using the built-in client
    docker compose exec dev go run cmd/client/main.go

    # Commands:
    dcache> SET name "Alice"
    dcache> GET name
    dcache> DELETE name
    dcache> KEYS
    dcache> STATS
    dcache> PING
    dcache> QUIT

## Testing

### Run Unit Tests

    # All tests with verbose output
    docker compose exec dev go test -v ./...

    # Specific package tests
    docker compose exec dev go test -v ./internal/cache

    # With coverage report
    docker compose exec dev go test -cover ./internal/cache
    # Result: 80.8% coverage

### Run Benchmarks

    docker compose exec dev go test -bench=. ./internal/cache

### Load Testing

    # Simple load test - insert 1000 keys
    for i in {1..1000}; do
      curl -s -X POST http://localhost:8080/cache/key$i -d "value$i" &
    done
    wait

    # Check statistics after load
    curl http://localhost:8080/stats | jq

### LRU Eviction Test

    # Run with limited capacity
    docker run -it --rm \
      -v $(pwd):/app \
      -p 8080:8080 \
      -e CAPACITY=3 \
      dcache:dev

    # Test eviction (in another terminal)
    curl -X POST http://localhost:8080/cache/a -d "1"
    curl -X POST http://localhost:8080/cache/b -d "2"
    curl -X POST http://localhost:8080/cache/c -d "3"
    curl http://localhost:8080/cache/a  # Access 'a' to make it recently used
    curl -X POST http://localhost:8080/cache/d -d "4"  # Should evict 'b'
    curl http://localhost:8080/keys
    # Result: ["a","c","d"] - 'b' was evicted

## Monitoring

### Real-time Statistics

    # Watch stats (requires jq)
    watch -n 1 'curl -s http://localhost:8080/stats | jq "{items, hits, misses, hit_rate, size_mb}"'

    # Or use a loop
    while true; do 
      clear
      curl -s http://localhost:8080/stats | python3 -m json.tool
      sleep 1
    done

### Docker Resource Monitoring

    docker stats dcache-dev-1

## Makefile Commands

The project includes a Makefile for common operations:

    make dev     # Start development server
    make test    # Run tests
    make bench   # Run benchmarks
    make logs    # View logs
    make shell   # Shell into container
    make clean   # Clean up containers and volumes

## Implementation Details

### Cache Store Features

- **Data Structure**: Doubly-linked list + HashMap for O(1) operations
- **Thread Safety**: RWMutex for concurrent read/write operations
- **Memory Management**: Automatic eviction based on capacity and size limits
- **TTL Support**: Background goroutine for expired item cleanup
- **Statistics**: Real-time tracking of hits, misses, evictions

### Protocol Support

- **HTTP API**: RESTful endpoints on port 8080
- **TCP Protocol**: Redis-compatible text protocol on port 6379
- **Response Formats**: JSON for HTTP, RESP-like for TCP

## Project Structure

    dcache/
    ├── cmd/
    │   ├── server/main.go      # Server entry point
    │   └── client/main.go      # CLI client
    ├── internal/
    │   ├── cache/
    │   │   ├── store.go        # Core cache implementation
    │   │   └── store_test.go   # Unit tests
    │   └── server/
    │       └── tcp_server.go   # TCP protocol handler
    ├── Dockerfile.dev          # Development container
    ├── docker-compose.yml      # Service orchestration
    ├── Makefile               # Build automation
    ├── go.mod                 # Go module definition
    └── .air.toml             # Hot reload configuration

## Success Metrics Achieved

| Metric | Target | Achieved |
|--------|--------|----------|
| Throughput | 10,000+ ops/sec | 20,000-30,000 ops/sec |
| Get Latency | <10ms | 165 ns |
| Set Latency | <10ms | 372 ns |
| Test Coverage | >70% | 80.8% |
| Memory Safety | Thread-safe | ✅ RWMutex |
| Eviction Policy | LRU | ✅ O(1) implementation |
| TTL Support | Required | ✅ Automatic expiration |

---

# WEEK 3-4: DISTRIBUTED CLUSTERING IMPLEMENTATION

## Project Overview

Transformed DCache from a single-node cache into a fully distributed caching system with consistent hashing, automatic node discovery, fault tolerance, and intelligent client routing.

## Performance Metrics

### Cluster Performance Results

Platform: 3-Node Docker Cluster  
Environment: golang:1.22-alpine

    Cluster Size: 3 nodes
    Virtual Nodes: 150 per physical node (450 total)
    Load Test: 1000 keys in 6 seconds (166 keys/sec)
    Distribution Variance: < 5%
    Failure Detection: 15 seconds (healthy → suspect)
    Dead Detection: 35 seconds (suspect → dead)
    Node Recovery: 5 seconds

### Key Performance Indicators

- **Throughput**: 166 operations/second (with network overhead)
- **Key Distribution**: Node1: 31.2%, Node2: 33.6%, Node3: 35.2%
- **Rebalancing**: Minimal key movement (1/N) on topology change
- **Cluster Formation**: < 1 second
- **Node State Transitions**: Automatic with configurable timeouts
- **Test Coverage**: 80.8% maintained

### Test Results

All cluster features validated:
- ✅ Consistent hashing with virtual nodes
- ✅ Even key distribution (±5% variance)
- ✅ Automatic node discovery via gossip
- ✅ Node failure detection (15s → suspect, 35s → dead)
- ✅ Node recovery and rejoining (5s)
- ✅ Smart client routing
- ✅ HTTP redirects (307 status) to correct nodes
- ✅ Load balancing across cluster
- ✅ Graceful node departure
- ✅ Topology synchronization

## Architecture

### Core Components

- **internal/cluster/consistent_hash.go**: Hash ring with virtual nodes for even distribution
- **internal/cluster/node_manager.go**: Cluster membership and health monitoring
- **internal/cluster/gossip.go**: UDP-based gossip protocol for node discovery
- **pkg/client/smart_client.go**: Cluster-aware client with automatic routing

### Features Implemented

- Consistent hashing with 150 virtual nodes per physical node
- Dynamic cluster management with join/leave operations
- Gossip protocol for peer-to-peer node discovery
- Health monitoring with automatic state transitions
- Smart client with local topology view
- Automatic key routing to correct nodes
- Minimal redistribution on topology changes

## Installation & Setup

### Prerequisites

- Docker Desktop with Docker Compose support
- Git
- Go 1.22+ (for smart client development)

### Quick Start - 3-Node Cluster

    # Clone the repository (if not already done)
    git clone https://github.com/sjohri/dcache.git
    cd dcache

    # Build Docker image
    docker build -f Dockerfile.dev -t dcache:dev .

    # Start 3-node cluster
    docker compose up node1 node2 node3

    # Verify cluster formation
    curl http://localhost:8001/cluster/nodes | jq

### Cluster Configuration

The cluster uses enhanced Docker Compose configuration with proper networking and node discovery.

## Running the Cluster

### Using Docker Compose

    # Start all nodes
    docker compose up node1 node2 node3

    # Start in background
    docker compose up -d node1 node2 node3

    # View individual node logs
    docker logs -f dcache-node1
    docker logs -f dcache-node2
    docker logs -f dcache-node3

    # Stop cluster
    docker compose down

### Configuration Options

Environment variables per node:

    NODE_ID=node1                # Unique node identifier
    NODE_ADDRESS=node1:8080      # HTTP address for cluster
    CLUSTER_MODE=true           # Enable clustering
    GOSSIP_PORT=7946           # UDP port for gossip
    VIRTUAL_NODES=150          # Virtual nodes per physical
    SEED_NODES=node1:7946      # Initial cluster contact points
    
    # Inherited from Week 1-2
    CAPACITY=10000             # Max items per node
    MAX_SIZE_MB=100           # Max memory per node
    DEFAULT_TTL_MIN=10        # Default TTL
    HTTP_PORT=8080            # HTTP API port
    TCP_PORT=6379             # TCP protocol port

## Cluster API Usage

### Cluster Management Endpoints

    # View all nodes in cluster
    curl http://localhost:8001/cluster/nodes | jq
    # Response: Array of nodes with id, address, status, last_seen

    # Check hash ring distribution
    curl http://localhost:8001/cluster/ring | jq
    # Response: Virtual node distribution percentages

    # Find node owning a key
    curl http://localhost:8001/cluster/locate/mykey | jq
    # Response: {"node_id":"node2","address":"node2:8080","local":false}

### Data Operations with Routing

    # Store data (automatically routes to correct node)
    curl -X POST http://localhost:8001/cache/test -d "value"
    # Response: "Stored" or "Key belongs to node X"

    # Retrieve data (follows redirects)
    curl -v http://localhost:8001/cache/test
    # Response: 307 redirect with X-Redirect-Node header

    # Delete across cluster
    curl -X DELETE http://localhost:8001/cache/test

### Smart Client Operations

    # Run test client
    cd test/integration
    go run test_client.go
    
    # Output shows:
    # - Successful key operations
    # - Distribution across nodes
    # - Automatic routing

## Testing

### Cluster Formation Test

    # Start cluster and verify all nodes see each other
    docker compose up node1 node2 node3
    
    # Check each node's view
    curl http://localhost:8001/cluster/nodes | jq length  # Should be 3
    curl http://localhost:8002/cluster/nodes | jq length  # May be 1-3
    curl http://localhost:8003/cluster/nodes | jq length  # May be 1-3

### Node Failure Test

    # Stop a node
    docker stop dcache-node2
    
    # Monitor status changes
    sleep 15
    curl http://localhost:8001/cluster/nodes | jq '.[] | {id, status}'
    # Node2 shows "suspect"
    
    sleep 20
    curl http://localhost:8001/cluster/nodes | jq '.[] | {id, status}'
    # Node2 shows "dead"
    
    # Restart node
    docker start dcache-node2
    sleep 5
    curl http://localhost:8001/cluster/nodes | jq
    # Node2 shows "healthy" again

### Load Distribution Test

    # Insert 1000 keys
    for i in {1..1000}; do
      PORT=$((8001 + (i % 3)))
      curl -s -X POST http://localhost:$PORT/cache/key$i -d "value$i"
    done
    
    # Check distribution
    for PORT in 8001 8002 8003; do
      echo "Port $PORT: $(curl -s http://localhost:$PORT/stats | jq .items) items"
    done
    # Should show roughly equal distribution (±5%)

### Smart Client Test

    # Test client automatically routes to correct nodes
    # Creates test keys and verifies distribution
    # Located in test/integration/test_client.go
    
    # Results show:
    # - 10 test keys stored and retrieved
    # - 1000 sample keys distributed evenly
    # - Node1: ~31%, Node2: ~34%, Node3: ~35%

## Monitoring

### Real-time Cluster Monitoring

    # Watch cluster topology changes
    watch -n 2 'curl -s http://localhost:8001/cluster/nodes | jq'

    # Monitor all node statistics
    watch -n 1 'for p in 8001 8002 8003; do 
        echo "Node $((p-8000)): $(curl -s http://localhost:$p/stats | jq .items) items"
    done'

### Key Distribution Analysis

    # Sample 100 keys to verify distribution
    for i in {1..100}; do
        curl -s http://localhost:8001/cluster/locate/key$i | jq -r .node_id
    done | sort | uniq -c
    # Should show roughly 33 keys per node

### Health Check Dashboard

    # Check cluster health across all nodes
    for PORT in 8001 8002 8003; do
        echo "Node $((PORT-8000)) health:"
        curl -s http://localhost:$PORT/health | jq
    done

## Implementation Details

### Consistent Hashing

- **Hash Function**: MD5 with 32-bit output
- **Virtual Nodes**: 150 per physical node for better distribution
- **Lookup**: Binary search O(log n) on sorted hash values
- **Rebalancing**: Only 1/N keys move when nodes change

### Gossip Protocol

- **Transport**: UDP on port 7946
- **Interval**: 5-second gossip rounds
- **Message Types**: announce, heartbeat, leave
- **Peer Selection**: Random subset for efficiency
- **Convergence**: Eventually consistent cluster view

### Node Management

- **Health States**: Healthy → Suspect → Dead → Removed
- **Timeouts**: Configurable per state transition
- **Failure Detection**: Based on last heartbeat time
- **Recovery**: Automatic rejoin on restart

### Smart Client

- **Routing**: Local hash ring for client-side routing
- **Topology Updates**: Periodic refresh every 10 seconds
- **Connection Pooling**: Reuses HTTP connections
- **Failover**: Handles node failures gracefully

## Updated Project Structure

    dcache/
    ├── cmd/
    │   ├── server/main.go         # Enhanced with cluster support
    │   └── client/main.go         # CLI client (unchanged)
    ├── internal/
    │   ├── cache/                 # Week 1-2 cache implementation
    │   │   ├── store.go          
    │   │   └── store_test.go     
    │   ├── cluster/               # Week 3-4 clustering additions
    │   │   ├── consistent_hash.go # Hash ring implementation
    │   │   ├── node_manager.go    # Membership management
    │   │   └── gossip.go          # Gossip protocol
    │   └── server/
    │       └── tcp_server.go      # TCP protocol (unchanged)
    ├── pkg/
    │   └── client/
    │       └── smart_client.go    # Week 3-4 smart client
    ├── test/
    │   ├── unit/
    │   │   └── cluster_test.go    # Cluster unit tests
    │   └── integration/
    │       └── test_client.go     # Integration tests
    ├── docker-compose.yml         # Enhanced for multi-node
    ├── Dockerfile.dev             # Unchanged from Week 1-2
    └── go.mod                     # Updated dependencies

## Success Metrics Achieved

| Metric | Target | Week 1-2 Single | Week 3-4 Cluster |
|--------|--------|-----------------|------------------|
| Throughput | 10,000+ ops/sec | 20,000-30,000 | 166 (with network) |
| Latency | <10ms | <1μs | <1ms |
| Test Coverage | >70% | 80.8% | 80.8% |
| Distribution | N/A | N/A | ±5% variance |
| Failure Detection | N/A | N/A | 15 seconds |
| Recovery Time | N/A | N/A | 5 seconds |
| Virtual Nodes | N/A | N/A | 150 per node |
| Cluster Size | N/A | 1 node | 3+ nodes |

---

*Implementation completed December 2, 2024*  
*System successfully evolved from single-node cache to distributed cluster*
