# DKV Docker Containers & Source File Mappings

This document maps each Docker container configured in `docker-compose.yml` to the specific compiled binaries and Go source files they run under the hood.

---

## Container Overview

| Container Name | Service Name | Build Target / Image | Main Entry Point | Primary Packages & Files |
| :--- | :--- | :--- | :--- | :--- |
| **`dkv-kvsrv`** | `kvsrv` | `kvsrvd` | `cmd/kvsrvd/main.go` | `internal/kvsrv/`, `internal/transport/` |
| **`dkv-kvraft-0`** | `kvraft-0` | `kvraftd` | `cmd/kvraftd/main.go` | `internal/raft/`, `internal/rsm/`, `internal/shardgrp/`, `internal/persist/`, `internal/core/`, `internal/transport/` |
| **`dkv-kvraft-1`** | `kvraft-1` | `kvraftd` | `cmd/kvraftd/main.go` | `internal/raft/`, `internal/rsm/`, `internal/shardgrp/`, `internal/persist/`, `internal/core/`, `internal/transport/` |
| **`dkv-kvraft-2`** | `kvraft-2` | `kvraftd` | `cmd/kvraftd/main.go` | `internal/raft/`, `internal/rsm/`, `internal/shardgrp/`, `internal/persist/`, `internal/core/`, `internal/transport/` |
| **`dkv-shardgrp2-0`** | `shardgrp2-0` | `kvraftd` | `cmd/kvraftd/main.go` | `internal/raft/`, `internal/rsm/`, `internal/shardgrp/`, `internal/persist/`, `internal/core/`, `internal/transport/` |
| **`dkv-shardgrp2-1`** | `shardgrp2-1` | `kvraftd` | `cmd/kvraftd/main.go` | `internal/raft/`, `internal/rsm/`, `internal/shardgrp/`, `internal/persist/`, `internal/core/`, `internal/transport/` |
| **`dkv-shardgrp2-2`** | `shardgrp2-2` | `kvraftd` | `cmd/kvraftd/main.go` | `internal/raft/`, `internal/rsm/`, `internal/shardgrp/`, `internal/persist/`, `internal/core/`, `internal/transport/` |
| **`dkv-shardctrler`** | `shardctrler` | `shardctrlrd` | `cmd/shardctrlrd/main.go` | `internal/shardctrler/`, `internal/metrics/`, `internal/core/`, `internal/transport/` |
| **`dkv-prometheus`** | `prometheus` | `prom/prometheus:latest` | N/A | Exposes Prometheus server using `./prometheus.yml` configuration. |
| **`dkv-grafana`** | `grafana` | `grafana/grafana:latest` | N/A | Runs Grafana visualization dashboard. |

---

## Detailed Mappings

### 1. Central Metadata Config Store (`dkv-kvsrv`)
* **Role:** Runs the single-node key-value server serving as the configurations/metadata database.
* **Compiled Binary:** `kvsrvd`
* **Entry Point File:**
  * [`cmd/kvsrvd/main.go`](../DistributedKeyValueStore/cmd/kvsrvd/main.go)
* **Underlying Source Files:**
  * **Configuration Storage Logic:**
    * [`internal/kvsrv/server.go`](../DistributedKeyValueStore/internal/kvsrv/server.go) - KV server engine.
    * [`internal/kvsrv/client.go`](../DistributedKeyValueStore/internal/kvsrv/client.go) - Client clerk interface.
    * [`internal/kvsrv/lock.go`](../DistributedKeyValueStore/internal/kvsrv/lock.go) - In-memory client lock tracking.
  * **RPC Communication:**
    * [`internal/transport/grpc.go`](../DistributedKeyValueStore/internal/transport/grpc.go) - Network transport layer using gRPC/TCP.
    * [`internal/transport/local.go`](../DistributedKeyValueStore/internal/transport/local.go) - Mock local network for local unit testing.
    * [`internal/transport/doc.go`](../DistributedKeyValueStore/internal/transport/doc.go) - Documentation header.

---

### 2. Shard Replica Group Containers
* **Applicable Containers:**
  * `dkv-kvraft-0`, `dkv-kvraft-1`, `dkv-kvraft-2` (Shard Group GID 1)
  * `dkv-shardgrp2-0`, `dkv-shardgrp2-1`, `dkv-shardgrp2-2` (Shard Group GID 2)
* **Role:** Implements fault-tolerant shard storage clusters using Raft consensus.
* **Compiled Binary:** `kvraftd`
* **Entry Point File:**
  * [`cmd/kvraftd/main.go`](../DistributedKeyValueStore/cmd/kvraftd/main.go)
* **Underlying Source Files:**
  * **Raft Consensus Engine:**
    * [`internal/raft/raft.go`](../DistributedKeyValueStore/internal/raft/raft.go) - Core Raft node structure.
    * [`internal/raft/election.go`](../DistributedKeyValueStore/internal/raft/election.go) - Leader election logic & heartbeat/timeouts.
    * [`internal/raft/replication.go`](../DistributedKeyValueStore/internal/raft/replication.go) - Log replication & commit progression.
    * [`internal/raft/persist.go`](../DistributedKeyValueStore/internal/raft/persist.go) - Metadata state persistence.
  * **Replicated State Machine (RSM) Bridge:**
    * [`internal/rsm/rsm.go`](../DistributedKeyValueStore/internal/rsm/rsm.go) - Translates committed Raft log entries to database operations.
  * **Shard Group Server Logic:**
    * [`internal/shardgrp/server.go`](../DistributedKeyValueStore/internal/shardgrp/server.go) - Shard group state machine handling database lookups/updates.
    * [`internal/shardgrp/client.go`](../DistributedKeyValueStore/internal/shardgrp/client.go) - Clerk client to reach shard group servers.
  * **Persistent Storage Layer:**
    * [`internal/persist/disk.go`](../DistributedKeyValueStore/internal/persist/disk.go) - Disk-backed persister saving state/snapshots.
    * [`internal/persist/memory.go`](../DistributedKeyValueStore/internal/persist/memory.go) - Mock in-memory persister for tests.
  * **Core Definitions:**
    * [`internal/core/interfaces.go`](../DistributedKeyValueStore/internal/core/interfaces.go) - Service contracts.
    * [`internal/core/kv.go`](../DistributedKeyValueStore/internal/core/kv.go) - Core database structures.
    * [`internal/core/raft.go`](../DistributedKeyValueStore/internal/core/raft.go) - Raft-specific structures.
    * [`internal/core/rsm.go`](../DistributedKeyValueStore/internal/core/rsm.go) - State machine commands.
    * [`internal/core/shard.go`](../DistributedKeyValueStore/internal/core/shard.go) - Shard routing helper functions.
  * **RPC Communication:**
    * [`internal/transport/grpc.go`](../DistributedKeyValueStore/internal/transport/grpc.go)

---

### 3. Central Orchestrator / Shard Controller (`dkv-shardctrler`)
* **Role:** Balances shard allocations and manages migrations.
* **Compiled Binary:** `shardctrlrd`
* **Entry Point File:**
  * [`cmd/shardctrlrd/main.go`](../DistributedKeyValueStore/cmd/shardctrlrd/main.go)
* **Underlying Source Files:**
  * **Shard Controller Logic:**
    * [`internal/shardctrler/controller.go`](../DistributedKeyValueStore/internal/shardctrler/controller.go) - Performs rebalancing calculations and manages configuration transitions.
  * **Metrics Instrumentation:**
    * [`internal/metrics/metrics.go`](../DistributedKeyValueStore/internal/metrics/metrics.go) - Exposes prometheus metric counters and gauges.
  * **Core Definitions & RPC:**
    * [`internal/core/shard.go`](../DistributedKeyValueStore/internal/core/shard.go)
    * [`internal/transport/grpc.go`](../DistributedKeyValueStore/internal/transport/grpc.go)

---

### 4. Third-Party Monitoring Stack
* **Prometheus (`dkv-prometheus`)**
  * **Image:** `prom/prometheus:latest`
  * **Configuration:** Exposes metrics scrape targets for the cluster services. Uses configuration details in `prometheus.yml`.
* **Grafana (`dkv-grafana`)**
  * **Image:** `grafana/grafana:latest`
  * **Configuration:** Exposes dashboard visualizations mapping variables and performance indicators.

---

### Note on Client Binary (not a persistent compose container)
* **Client Tool (`dkv-client`)**
  * **Role:** CLI tool to manually issue requests to the sharded store.
  * **Entry Point:** [`cmd/dkv-client/main.go`](../DistributedKeyValueStore/cmd/dkv-client/main.go)
  * **Source Dependency:**
    * [`internal/shardkv/client.go`](../DistributedKeyValueStore/internal/shardkv/client.go) - Coordinates client routing/requests across the `ShardController` and individual `ShardGroup` servers.
