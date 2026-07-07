# DKV Internal Packages Directory

This directory houses the core internal components, domain models, consensus engines, and communication protocols of the Distributed Key-Value Store. 

Below is an overview of the packages and folders in `internal/`:

---

### Folder Directory & Package Roles

#### 1. [`core/`](./core)
* **Role:** Shared domain definitions and core abstractions.
* **Key Contents:** 
  * Exposes shared interfaces for storage persistence (`Persister`) and RPC networking (`Network`).
  * Defines core request/reply payloads, log entries, and the state-machine operations (`Op`).
  * Contains the global sharding config structures (`ShardConfig`) and the deterministic partitioning hash function `Key2Shard(key)`.

#### 2. [`kvsrv/`](./kvsrv)
* **Role:** Simple single-node key-value database engines.
* **Key Contents:**
  * Implements linearizable, locked memory map store operations.
  * In the sharded database deployment, this package runs inside the `dkv-kvsrv` container, acting as the authoritative central configuration and metadata store holding the global group configuration.

#### 3. [`metrics/`](./metrics)
* **Role:** Observability and system metrics.
* **Key Contents:**
  * Defines Prometheus-compatible counters, gauges, and histograms to track:
    * Current configuration version numbers.
    * Shard migration states and transition durations.
    * RPC invocation rates, errors, and latencies.

#### 4. [`persist/`](./persist)
* **Role:** State persistence layer.
* **Key Contents:**
  * Implements `DiskPersister` which writes state data and DB snapshots directly to durable filesystem storage inside the container volume.
  * Implements a mocked in-memory persister (`MemoryPersister`) used to execute fast, repeatable unit tests.

#### 5. [`raft/`](./raft)
* **Role:** Raft Consensus Engine.
* **Key Contents:**
  * Fully implements the Raft consensus algorithm: leader election, randomized timeouts, periodic heartbeats, log replication, commit index progression, and state snapshot compaction.

#### 6. [`rsm/`](./rsm)
* **Role:** Replicated State Machine (RSM) Adaptor.
* **Key Contents:**
  * Serves as the bridge between the Raft consensus layer and the database engine.
  * Serializes client submissions, starts Raft proposals, consumes committed messages sequentially from the `applyCh` channel, executes them on the local database, and notifies waiting client threads.

#### 7. [`shardctrler/`](./shardctrler)
* **Role:** Shard Controller (Cluster Orchestrator).
* **Key Contents:**
  * Tracks group memberships and balances shard allocations evenly across groups.
  * Orchestrates the multi-phase shard migration protocol (Freeze ➔ Fetch ➔ Install ➔ Delete ➔ Publish) when topology changes.

#### 8. [`shardgrp/`](./shardgrp)
* **Role:** Shard replica group database engine.
* **Key Contents:**
  * Implements the local state machine for replica group nodes, handling shard data mutation checks (`DoOp`) and state machine snapshot loads.
  * Implements a replica group client (`shardgrp.Clerk`) that iterates through a group's servers to find the active Raft leader.

#### 9. [`shardkv/`](./shardkv)
* **Role:** Top-level sharded client clerk.
* **Key Contents:**
  * Exposes the main database client clerk (`shardkv.Clerk`) that users interface with.
  * Coordinates config querying, parses target keys to shards, finds the active group owner, and routes client reads and writes.

#### 10. [`transport/`](./transport)
* **Role:** Network communication layer.
* **Key Contents:**
  * Implements `GRPCNetwork` using standard Go RPC and gRPC style connection management over TCP to handle inter-node RPCs.
  * Implements `LocalNetwork`, simulating network failures, latency spikes, and partition simulations for testing consensus resilience.
