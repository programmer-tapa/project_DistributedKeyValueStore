# Distributed Key-Value Store: Control Plane Architecture

This document describes the role of the **Config Metadata Server** and the **ShardController** (`shardctrler`) within the sharded key-value store system, explaining how they coordinate cluster topology, manage sharding, and safely execute migrations.

---

## Architecture Overview

```
                      ┌─────────────────────────────────┐
                      │          Client App             │
                      └────────────────┬────────────────┘
                                       │ 1. Query Config
                                       ▼
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │ CONTROL PLANE                                                               │
 │                                                                             │
 │    ┌──────────────────────────┐          ┌───────────────────────────┐      │
 │    │     ShardController      │─────────►│  Config Metadata Server   │      │
 │    │      (shardctrler)       │ 2. Read/ │          (kvsrv)          │      │
 │    │                          │    Write │                           │      │
 │    └────────────┬─────────────┘          └───────────────────────────┘      │
 │                 │                                                           │
 │                 │ 3. Execute Migration (Freeze -> Install -> Delete)        │
 └─────────────────┼───────────────────────────────────────────────────────────┘
                   │
                   ▼
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │ DATA PLANE                                                                  │
 │                                                                             │
 │    ┌──────────────────────────┐          ┌───────────────────────────┐      │
 │    │       ShardGroup 1       │          │       ShardGroup 2        │      │
 │    │      (Replicated)        │          │      (Replicated)         │      │
 │    └──────────────────────────┘          └───────────────────────────┘      │
 └─────────────────────────────────────────────────────────────────────────────┘
```

---

## 1. Config Metadata Server (`kvsrv`)

* **What it is:** A single-node key-value server (backed by the Lab 2 `kvsrv` package) that stores configuration data.
* **Role:** The **Single Source of Truth** for topology state.
* **Key Functions:**
  * Stores the serialized `ShardConfig` struct, which maintains:
    1. The configuration version number (`Num`).
    2. The shard-to-group mapping array (`Shards` array mapping each shard ID to its owning Group ID).
    3. The replica group database (`Groups` map mapping each Group ID to its list of server TCP addresses).
  * Serves configuration updates and retrievals via linearizable versioned `Put` and `Get` requests.

---

## 2. Shard Controller (`shardctrler`)

* **What it is:** The management and coordination engine (control plane process).
* **Role:** The **Orchestrator of Cluster Topology and Migrations**.
* **Key Functions:**

### A. Serving Configuration Updates to Clients
Clients (`shardkv.Clerk`) query the `shardctrler` using the `Query` RPC to locate which replica group currently hosts the target key. If a request routes to the wrong group (due to a shard migrating to another group mid-flight), the clerk queries the controller for the latest config and retries.

### B. Executing the Four-Phase Shard Migration Protocol
When replica groups are added or removed, the controller alters the shard layout. To safely transition a shard without losing writes or allowing split-brain access, it executes a coordinated workflow:
1. **Freeze:** Sends a `FreezeShard` RPC to the source replica group, instructing it to stop accepting client requests for that shard and return its in-memory key-value state.
2. **Install:** Sends an `InstallShard` RPC containing the shard's key-value data to the destination replica group, which begins serving requests for it.
3. **Delete:** Sends a `DeleteShard` RPC to the source replica group to safely delete its local copy of the migrated shard data.
4. **Publish:** Atomically updates and publishes the new `ShardConfig` to the **Config Metadata Server** to let client clerks route future requests to the new owner.

### C. Guaranteeing Shard Exclusivity Safety
The controller ensures that **exactly one replica group owns any given shard at a time**. 
* Every migration command carries a monotonically increasing `ConfigNum`.
* Replica groups reject migration RPCs with older `ConfigNum` arguments. This design constraint guarantees safety even if the controller crashes and a newly elected controller restarts the migration.
