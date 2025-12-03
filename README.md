# DCache - Distributed In-Memory Cache

High-performance distributed in-memory cache system built with Go. Features consistent hashing, automatic node discovery via gossip protocol, and fault tolerance.

## Quick Start

    # Clone repository
    git clone https://github.com/sjohri/dcache.git
    cd dcache

    # Build and run single node
    docker compose up dev

    # Run 3-node cluster
    docker compose up node1 node2 node3

## Basic Usage

    # Store value
    curl -X POST http://localhost:8001/cache/key -d "value"

    # Retrieve value  
    curl http://localhost:8001/cache/key

    # Check cluster status
    curl http://localhost:8001/cluster/nodes | jq

    # Find key owner
    curl http://localhost:8001/cluster/locate/key | jq

## Documentation

For complete implementation details, architecture, performance benchmarks, and detailed run instructions, see [DCache Implementation Report.md](DCache%20Implementation%20Report.md).

## Project Structure

    dcache/
    ├── cmd/server/         # Main server application
    ├── internal/cache/     # Core cache implementation
    ├── internal/cluster/   # Distributed system components
    ├── pkg/client/         # Smart client library
    └── docker-compose.yml  # Multi-node orchestration

## Requirements

- Docker Desktop
- Go 1.22+ (optional, for client development)

## License

MIT

---

Built as a learning project for distributed systems concepts. December 2024.
