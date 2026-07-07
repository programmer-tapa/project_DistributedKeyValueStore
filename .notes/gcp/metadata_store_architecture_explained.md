# DKV Architecture: Why a Single Metadata Store Node?

In a production distributed system, having a single metadata coordinator is a **Single Point of Failure (SPOF)**. If the metadata store goes offline, the entire cluster halts because clients and controllers can no longer read or update the shard configuration.

This document explains why the Distributed Key-Value Store (DKV) utilizes a single-node metadata store (`kvsrvd`) by design, the consequences of scaling it without replication, and how real-world enterprise systems solve this challenge.

---

## 1. The Technical Constraint: Lack of Replication in `kvsrvd`

The executable running inside the metadata store is **`kvsrvd`** (Key-Value Server). Unlike the actual data shard replicas (`kvraftd`), which implement the **Raft consensus protocol** to replicate logs and elect leaders:

*   **No Peer Awareness**: `kvsrvd` is a simple, standalone key-value store. It contains no networking code to communicate with other metadata stores, replicate logs, or synchronize memory.
*   **Data Divergence (Split-Brain)**: If we were to spin up 3 replicas of the metadata store under a load balancer or service, they would act as **three completely independent, isolated databases**. 
*   **State Corruption**: If a Shard Controller writes a new shard configuration to replica A, and a client subsequently queries replica B, replica B would return an outdated configuration (or "key not found"). This would immediately break database routing, causing keys to be written to the wrong shard groups and corrupting the entire database state.

---

## 2. The Design Context: Academic & Focused Scope

In the DKV project, the primary educational objective is to implement and demonstrate **sharded Raft consensus at the database shard layer** (the actual storage engines holding user data):

*   **Data Shard Redundancy**: The actual data shards (`kvraft-1` and `kvraft-2`) are fully replicated, fault-tolerant, and run Raft.
*   **Coordinator Simplicity**: The metadata store (`kvsrvd`) is kept intentionally simple (a single-node coordinator) to avoid the immense complexity of running, managing, and bootstrapping **two separate Raft consensus clusters** (one for metadata and one for data) within the same application codebase.

---

## 3. The Real-World Enterprise Solution

In a production-grade sharded database (such as Google Spanner, CockroachDB, or MongoDB), the metadata coordinator is **never a single node**. 

Instead of a custom simple server like `kvsrvd`, enterprise systems delegate configuration management to a highly consistent, replicated consensus store:

| Enterprise Database | Replicated Metadata Store | Consensus Protocol Used |
| :--- | :--- | :--- |
| **Kubernetes (GKE)** | **`etcd`** | **Raft** |
| **Apache Hadoop / HBase** | **`ZooKeeper`** | **Zab** (Paxos-like) |
| **Google Spanner** | **`Chubby`** | **Paxos** |

### How to Upgrade DKV to Production-Grade
To make the DKV metadata layer 100% fault-tolerant:
1.  **Replace `kvsrvd` with `etcd`**: Deploy a 3-node or 5-node `etcd` cluster in GKE.
2.  **Integrate Client & Controller**: Update the Shard Controllers and the client library to read and write the `"config"` key directly to the `etcd` cluster. 
3.  **Achieve Full HA**: Because `etcd` runs Raft internally, it guarantees that metadata is safely replicated across zones, making the coordinator layer immune to single-node failures.
