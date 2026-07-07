# Distributed Key-Value Store (DKV)

A production-grade, horizontally sharded, fault-tolerant key-value store built in Go. This system is architected around clean systems-centric design principles, utilizing Raft consensus for state replication, dynamic shard migration, and Prometheus-based observability.

```
                  ┌─────────────────────────────────┐
                  │      Production Showcase        │
                  └────────────────┬────────────────┘
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         ▼                         ▼                         ▼
┌──────────────────┐      ┌──────────────────┐      ┌──────────────────┐
│   Real Network   │      │ Chaos Injector   │      │  Observability   │
│  gRPC / TCP Go   │      │ Docker partition │      │    Prometheus    │
│  Replacing RPC   │      │ leader kills     │      │   Grafana dashboard
└──────────────────┘      └──────────────────┘      └──────────────────┘
```

## Features

- **Raft Consensus**: Active leader election, log replication, and log compaction (snapshots).
- **Horizontal Sharding**: Dynamically partition keys across multiple replica groups (shards).
- **4-Phase Migration**: Freeze, Install, Delete, and Publish protocol for safe, linearizable shard migrations.
- **Production Enhancements**:
  - **gRPC Transport Layer**: Real TCP network transport instead of in-memory simulated networks.
  - **Durable Disk Persistence**: Crash-safe atomic state persistence.
  - **Chaos Engineering Harness**: CLI tool (`chaos.sh`) for simulating node kills and network partitions.
  - **Full Observability**: Instrumented with Prometheus metrics and dashboard configurations for Grafana.

## Project Structure

```
DistributedKeyValueStore/
├── cmd/                      # Binary entrypoints
│   ├── kvsrvd/               # Standalone single-node KV server
│   ├── kvraftd/              # Replicated KV / Shard Group server
│   ├── shardctrlrd/          # Shard controller daemon
│   └── dkv-client/           # User CLI client
├── deployments/              # Deployment orchestration configurations
│   └── docker/               # Compose file, Dockerfile, Prometheus config
├── internal/                 # Private systems-centric application code
│   ├── core/                 # Innermost dependency-free domain layer (primitives & interfaces)
│   ├── raft/                 # Raft consensus engine (election, replication, persistence)
│   ├── rsm/                  # Replicated State Machine abstraction layer
│   ├── kvsrv/                # Single-node KV server & local lock primitives
│   ├── shardgrp/             # Shard-aware replica group handlers
│   ├── shardctrler/          # Shard migration orchestration logic
│   ├── shardkv/              # Top-level client-side request router
│   ├── transport/            # Concrete networks: local channel-based & gRPC over TCP
│   ├── persist/              # Concrete persistence: memory & atomic disk files
│   └── metrics/              # Prometheus collectors and monitoring setup
├── scripts/                  # Chaos testing and benchmarking tools
│   ├── chaos.sh              # Simulates container failures and network partitions
│   └── bench.sh              # Benchmarks QPS/latency under stress
└── Makefile                  # Compilation and validation workflow commands
```

## Quickstart

Start the cluster (1 controller, 2 shard groups with 3 replica nodes each, Prometheus/Grafana stack):

Using the `Makefile` helper target:
```bash
make docker-up
```

Or directly using `docker compose` (from the repository root):
```bash
docker compose up --build -d
```

Verify the system components using the client. Note that since the cluster nodes register using their container hostnames and internal ports (e.g. `kvraft-0:8000`), the client must run inside the Docker network (`docker_kvnet`) to route requests correctly.

First, compile the client binary:
```bash
CGO_ENABLED=0 GOOS=linux go build -o bin/dkv-client ./cmd/dkv-client
```

Then, run the client inside an Alpine container on the cluster's network (`docker_kvnet`), pointing to `kvsrv:9000` (the configuration store):
```bash
# Put a key-value pair
docker run --rm --network docker_kvnet -v "$PWD"/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 put mykey myvalue

# Get the key-value pair
docker run --rm --network docker_kvnet -v "$PWD"/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 get mykey
```

Run tests:

```bash
make test
```
