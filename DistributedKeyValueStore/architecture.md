# Distributed Key/Value Store — Architectural Plan

> A production-grade, sharded, fault-tolerant key/value store (Key/Value Server → Raft → Fault-tolerant KV → Sharded KV)

---

## 1. System Overview

This project builds a **production-grade, sharded, fault-tolerant key/value store** in four progressive layers:

| Phase | Component | Purpose |
|-----|-----------|---------|
| 1 | `kvsrv` | Single-node linearizable KV server with versioned puts |
| 2 | `raft` | Consensus engine: leader election, log replication, snapshots |
| 3 | `kvraft` / `rsm` | Replicated KV service backed by Raft via the RSM abstraction |
| 4 | `shardkv` | Horizontally sharded KV service with dynamic shard migration |

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Clients (Clerks)                              │
│          Put(key,value,version) / Get(key) via RPC                   │
└──────────────┬──────────────────────────────┬───────────────────────┘
               │                              │
     ┌─────────▼─────────┐        ┌──────────▼──────────┐
     │   ShardCtrler     │        │   ShardKV Clerk      │
     │  (config store)   │        │  (routes by shard)   │
     └─────────┬─────────┘        └──────────┬──────────┘
               │ kvsrv (Phase 1)             │ Key2Shard()
               │                   ┌─────────▼──────────────────┐
               │                   │    ShardGroup 1..N          │
               │                   │  (Raft RSM replicated KV)   │
               │                   └─────────────────────────────┘
               │
        ┌──────▼───────┐
        │  kvsrv1d     │  ← single-node, stores ShardConfig as string
        └──────────────┘
```

---

## 2. Layer-by-Layer Architecture

### 2A. Phase 1 — `kvsrv`: Single-Node KV Server

**Purpose:** Linearizable key/value store; foundation for config storage in Phase 4.

**Data Model:**
```
map[string] → {value string, version uint64}
```

**RPC Interface:**
```go
// Put installs value only if versions match; increments version on success
Put(key, value string, version uint64) → (ErrOK | ErrVersion | ErrNoKey | ErrMaybe)

// Get returns current value and version
Get(key string) → (value string, version uint64, ErrOK | ErrNoKey)
```

**Key Design Decisions:**
- **Versioned puts** → at-most-once semantics without server-side dedup tables
- **`ErrMaybe`** returned by Clerk when a retransmitted Put gets `ErrVersion` (ambiguous outcome)
- **Retry loop in Clerk** with `100ms` backoff; tracks whether RPC is a retransmit
- **Distributed Lock** built on top: `Acquire` spins on `Put(lockKey, clientID, currentVersion)`, `Release` calls `Put(lockKey, "", currentVersion)`

**File Layout:**
```
kvsrv/
├── server.go       # KVServer struct, Put/Get handlers, in-memory map
├── client.go       # Clerk struct, Put/Get with retry + ErrMaybe logic
└── rpc/rpc.go      # PutArgs, GetArgs, PutReply, GetReply, error constants
kvsrv/lock/
└── lock.go         # Lock struct, Acquire/Release using Clerk
```

---

### 2B. Phase 2 — `raft`: Consensus Engine

**Purpose:** Replicated log consensus; powers all fault-tolerant layers above.

**Raft Peer API:**
```go
rf := Make(peers []labrpc.ClientEnd, me int, persister *Persister, applyCh chan ApplyMsg)
rf.Start(command interface{}) → (index int, term int, isLeader bool)
rf.GetState() → (term int, isLeader bool)
rf.Snapshot(index int, snapshot []byte)
```

**Parts Implemented:**

| Part | Feature |
|------|---------|
| 3A | Leader election via `RequestVote` RPC; heartbeats via `AppendEntries` (empty) |
| 3B | Log replication: full `AppendEntries` with log consistency check; commitment |
| 3C | Persistence: `currentTerm`, `votedFor`, `log[]` encoded via `labgob` to Persister |
| 3D | Log compaction: `Snapshot(index, data)` trims log; `InstallSnapshot` RPC for lagging peers |

**Key Timings:**
- Heartbeat interval: ≤ 100ms (tester cap: 10/sec)
- Election timeout: 300–500ms (randomized to avoid split votes)
- Leader election must complete within 5s of failure

**State Machine (Figure 2):**
```
Follower  ──election timeout──►  Candidate  ──majority votes──►  Leader
   ▲                                  │                              │
   └─────────── receives heartbeat ◄──┘   ─── sends heartbeats ─────┘
```

**Persistent State** (must survive crashes):
- `currentTerm` — latest term seen
- `votedFor` — candidate voted for in current term
- `log[]` — log entries (index, term, command)
- `snapshot` + `snapshotIndex`, `snapshotTerm` — for 3D

**`ApplyMsg` sent on `applyCh`:**
```go
type ApplyMsg struct {
    CommandValid  bool
    Command       interface{}
    CommandIndex  int
    SnapshotValid bool
    Snapshot      []byte
    SnapshotTerm  int
    SnapshotIndex int
}
```

**File Layout:**
```
raft1/
└── raft.go         # Raft struct, Make(), Start(), ticker goroutine,
                    # RequestVote, AppendEntries, InstallSnapshot RPCs,
                    # Snapshot(), persist(), readPersist()
raftapi/
└── raftapi.go      # ApplyMsg type, Raft interface
```

---

### 2C. Phase 3 — `kvraft` + `rsm`: Replicated KV via RSM Abstraction

**Purpose:** Fault-tolerant KV service. The **RSM** (Replicated State Machine) layer decouples the consensus engine from the application logic.

**Architecture Layers:**
```
Client Clerk
    │  RPC (Put/Get)
    ▼
KVServer (kvraft1/server.go)
    │  implements StateMachine interface
    ▼
RSM Package (kvraft1/rsm/rsm.go)
    │  Submit(op) → result
    │  reader goroutine reads applyCh
    ▼
Raft (raft1/raft.go)
    │  Start(command)
    │  applyCh ← committed entries
    ▼
Raft Peers (via RPC)
```

**RSM Interface:**
```go
// StateMachine is implemented by kvserver
type StateMachine interface {
    DoOp(op any) any
    Snapshot() []byte
    Restore(snapshot []byte)
}

// RSM public API
func (r *RSM) Submit(op any) (any, error)
// Internally: wraps op in Op{UniqueID, Payload}, calls raft.Start(),
// waits for reader goroutine to signal committed result via channel map
```

**`Op` Struct** (submitted to Raft log):
```go
type Op struct {
    ID      int64       // unique per Submit call
    Payload interface{} // PutArgs or GetArgs
}
```

**KVServer DoOp Logic:**
```go
func (kv *KVServer) DoOp(op any) any {
    switch cmd := op.(type) {
    case PutArgs:
        // version-checked put; return PutReply
    case GetArgs:
        // return GetReply{Value, Version}
    }
}
```

**Snapshot / Restore (Part 4C):**
- `rsm` monitors `rf.PersistBytes()` vs `maxraftstate`
- When approaching threshold → calls `rf.Snapshot(lastApplied, kv.Snapshot())`
- On restart → reads snapshot via `persister.ReadSnapshot()` → calls `kv.Restore(data)`

**Linearizability Guarantee:**
- All writes go through Raft log → total order
- Clerk retries on `ErrWrongLeader`, redirects to discovered leader
- RSM detects stale leadership (term change at log index) → returns `ErrWrongLeader`

**File Layout:**
```
kvraft1/
├── client.go        # Clerk: Put/Get RPCs with leader-tracking retry
├── server.go        # KVServer: DoOp, Snapshot, Restore; starts RSM
└── rsm/
    └── rsm.go       # RSM: Submit(), reader goroutine, snapshot trigger
```

---

### 2D. Phase 4 — `shardkv`: Sharded KV Service

**Purpose:** Horizontally scale the KV service by partitioning keys across multiple Raft groups (**shardgrps**), with a central **ShardCtrler** managing configuration.

**Shard Assignment:**
```go
shardNum := shardcfg.Key2Shard(key)   // hash-based, 0..NShards-1
gid      := config.Shards[shardNum]   // which group owns this shard
servers  := config.Groups[gid]        // server list for that group
```

**Components:**

```
┌──────────────────────────────────────────────────────────────────────┐
│                       ShardCtrler                                     │
│   - stores ShardConfig in kvsrv (InitConfig, Query)                  │
│   - orchestrates shard moves (ChangeConfigTo)                         │
│     1. FreezeShard(src_gid, shard, configNum)                        │
│     2. InstallShard(dst_gid, shard, data, configNum)                 │
│     3. DeleteShard(src_gid, shard, configNum)                        │
│     4. Update config in kvsrv                                        │
└──────────────────────────────────────────────────────────────────────┘

┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│ ShardGrp 1  │  │ ShardGrp 2  │  │ ShardGrp 3  │
│ (Raft RSM)  │  │ (Raft RSM)  │  │ (Raft RSM)  │
│ owns shards │  │ owns shards │  │ owns shards │
│  {0,1,3}    │  │  {2,5}      │  │  {4,6,7}    │
└─────────────┘  └─────────────┘  └─────────────┘
```

**ShardConfig:**
```go
type ShardConfig struct {
    Num    int               // monotonically increasing config number
    Shards [NShards]int      // Shards[i] = GID that owns shard i
    Groups map[int][]string  // GID → list of server addresses
}
```

**Shard Migration Protocol (ChangeConfigTo):**
```
1. FREEZE  → tell source shardgrp to stop serving shard, return KV data
2. INSTALL → send KV data to destination shardgrp; it starts serving
3. DELETE  → tell source shardgrp to discard the shard
4. PUBLISH → update config in kvsrv so clients discover new owner
```

Each RPC carries the `configNum` so stale/replayed RPCs are rejected (shardgrp tracks max seen `configNum` per shard).

**Fault Tolerance:**
- Controller crashes → new controller re-runs `ChangeConfigTo`; idempotent because of `configNum` checks
- Concurrent controllers (Part 5C) → coordinated via versioned config in `kvsrv`
- Shardgrp failures → Raft ensures the group's state is durable

**File Layout:**
```
shardkv1/
├── client.go              # ShardKV Clerk: Key2Shard → Query → shardgrp.MakeClerk → Put/Get
├── shardcfg/
│   └── shardcfg.go        # ShardConfig struct, Key2Shard(), FromString(), String()
├── shardctrler/
│   └── shardctrler.go     # InitConfig, Query, ChangeConfigTo
└── shardgrp/
    ├── client.go          # ShardGrp Clerk: Put/Get/FreezeShard/InstallShard/DeleteShard
    ├── server.go          # ShardGrp server (extends kvraft), freeze/install/delete handlers
    └── shardrpc/
        └── shardrpc.go    # FreezeShard/InstallShard/DeleteShard RPC types
```

---

## 3. End-to-End Request Flow

```
Client.Put("foo", "bar", ver)
  │
  ▼ ShardKV Clerk
  ├── shard = Key2Shard("foo")                  # deterministic hash
  ├── config = shardctrler.Query()              # current ShardConfig
  ├── gid = config.Shards[shard]
  ├── servers = config.Groups[gid]
  ├── clerk = shardgrp.MakeClerk(servers)
  └── clerk.Put("foo", "bar", ver)
        │
        ▼ ShardGrp Clerk → leader server RPC
        ├── KVServer.Put() handler
        ├── rsm.Submit(PutArgs{...})
        │     ├── raft.Start(Op{id, PutArgs})
        │     └── wait for applyCh
        │           ├── all peers: DoOp(PutArgs) → update KV map
        │           └── leader: return PutReply to Submit()
        └── return PutReply to client
```

---

## 4. Docker Containerization Architecture

### 4.1 Container Strategy

Each logical process in the system runs in its own container, communicating via a Docker network over TCP.

```
docker-compose.yml
├── kvsrv          (1 container)  — Phase 1 single-node KV
├── raft-node-{0,1,2}  (3 or 5)  — Phase 2 Raft peers (for testing)
├── kvraft-{0,1,2} (3 or 5)      — Phase 3 KV+Raft servers
├── shardgrp-{gid}-{idx}         — Phase 4 shard group members
├── shardctrler                  — Phase 4 controller
└── client                       — Test/benchmark clients
```

### 4.2 Dockerfile

```dockerfile
# ── Base image ──────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY src/ ./src/
ARG BUILD_TARGET=kvsrv1d
RUN cd src && go build -o /bin/${BUILD_TARGET} main/${BUILD_TARGET}.go

# ── Runtime image ────────────────────────────────────────────────────
FROM alpine:3.19
ARG BUILD_TARGET=kvsrv1d
ENV BINARY=${BUILD_TARGET}
COPY --from=builder /bin/${BUILD_TARGET} /usr/local/bin/
EXPOSE 8000-8100
ENTRYPOINT ["/bin/sh", "-c", "/usr/local/bin/${BINARY} $@", "--"]
```

### 4.3 Docker Compose

```yaml
# docker-compose.yml
version: "3.9"

networks:
  kvnet:
    driver: bridge

# ── Shared environment ───────────────────────────────────────────────
x-raft-common: &raft-common
  build:
    context: .
    args:
      BUILD_TARGET: kvraft1d
  networks: [kvnet]
  restart: on-failure

services:

  # ── Phase 1: Single-node KV (used by ShardCtrler) ─────────────────
  kvsrv:
    build:
      context: .
      args:
        BUILD_TARGET: kvsrv1d
    container_name: kvsrv
    networks: [kvnet]
    ports:
      - "9000:9000"
    environment:
      LISTEN_ADDR: "0.0.0.0:9000"

  # ── Phase 3 / 4: KVRaft group (ShardGrp 1) — 3 Raft peers ─────────
  kvraft-0:
    <<: *raft-common
    container_name: kvraft-0
    environment:
      ME: "0"
      PEERS: "kvraft-0:8000,kvraft-1:8000,kvraft-2:8000"
      GID: "1"
    ports: ["8000:8000"]

  kvraft-1:
    <<: *raft-common
    container_name: kvraft-1
    environment:
      ME: "1"
      PEERS: "kvraft-0:8000,kvraft-1:8000,kvraft-2:8000"
      GID: "1"
    ports: ["8001:8000"]

  kvraft-2:
    <<: *raft-common
    container_name: kvraft-2
    environment:
      ME: "2"
      PEERS: "kvraft-0:8000,kvraft-1:8000,kvraft-2:8000"
      GID: "1"
    ports: ["8002:8000"]

  # ── Phase 4: ShardGrp 2 — 3 Raft peers ────────────────────────────
  shardgrp2-0:
    <<: *raft-common
    container_name: shardgrp2-0
    build:
      context: .
      args:
        BUILD_TARGET: shardgrp1d
    environment:
      ME: "0"
      PEERS: "shardgrp2-0:8010,shardgrp2-1:8010,shardgrp2-2:8010"
      GID: "2"
    ports: ["8010:8010"]

  shardgrp2-1:
    <<: *raft-common
    container_name: shardgrp2-1
    build:
      context: .
      args:
        BUILD_TARGET: shardgrp1d
    environment:
      ME: "1"
      PEERS: "shardgrp2-0:8010,shardgrp2-1:8010,shardgrp2-2:8010"
      GID: "2"
    ports: ["8011:8010"]

  shardgrp2-2:
    <<: *raft-common
    container_name: shardgrp2-2
    build:
      context: .
      args:
        BUILD_TARGET: shardgrp1d
    environment:
      ME: "2"
      PEERS: "shardgrp2-0:8010,shardgrp2-1:8010,shardgrp2-2:8010"
      GID: "2"
    ports: ["8012:8010"]

  # ── Phase 4: ShardCtrler ───────────────────────────────────────────
  shardctrler:
    build:
      context: .
      args:
        BUILD_TARGET: shardgrp1d   # re-uses same binary with CTRLER_MODE env
    container_name: shardctrler
    networks: [kvnet]
    depends_on: [kvsrv, kvraft-0, shardgrp2-0]
    environment:
      KVSRV_ADDR: "kvsrv:9000"
      SHARDGRPS: "1=kvraft-0:8000,kvraft-1:8000,kvraft-2:8000;2=shardgrp2-0:8010,shardgrp2-1:8010,shardgrp2-2:8010"
    ports: ["9100:9100"]
```

### 4.4 Network Topology

```
┌─────────────── kvnet (172.20.0.0/16) ──────────────────────┐
│                                                              │
│  kvsrv:9000          shardctrler:9100                        │
│                                                              │
│  kvraft-0:8000  ←──Raft RPC──►  kvraft-1:8000              │
│       └────────────────────────► kvraft-2:8000              │
│                                                              │
│  shardgrp2-0:8010 ←─Raft RPC──► shardgrp2-1:8010           │
│       └──────────────────────────► shardgrp2-2:8010          │
│                                                              │
│  client (ephemeral)                                          │
└──────────────────────────────────────────────────────────────┘
```

### 4.5 Persistence Volumes

```yaml
volumes:
  kvraft0-data:
  kvraft1-data:
  kvraft2-data:
  shardgrp2-0-data:
  # ...

# Attach to services:
kvraft-0:
  volumes:
    - kvraft0-data:/var/lib/raft
  environment:
    PERSIST_DIR: "/var/lib/raft"
```

Each Raft peer persists `currentTerm`, `votedFor`, `log[]`, and snapshot to its volume.

---

## 5. Project Directory Structure

```
DistributedKeyValueStore/
├── architecture.md            ← this file
├── Dockerfile
├── docker-compose.yml
├── docker-compose.test.yml    ← chaos/partition testing overrides
│
├── src/                       ← Go source
│   ├── go.mod
│   ├── Makefile
│   │
│   ├── labrpc/                # simulated RPC (for tests)
│   ├── labgob/                # gob encoding helpers
│   ├── raftapi/               # ApplyMsg, Raft interface
│   │
│   ├── kvsrv1/                # Phase 1
│   │   ├── server.go
│   │   ├── client.go
│   │   ├── rpc/rpc.go
│   │   └── lock/lock.go
│   │
│   ├── raft1/                 # Phase 2
│   │   └── raft.go
│   │
│   ├── kvraft1/               # Phase 3
│   │   ├── server.go
│   │   ├── client.go
│   │   └── rsm/rsm.go
│   │
│   ├── shardkv1/              # Phase 4
│   │   ├── client.go
│   │   ├── shardcfg/
│   │   ├── shardctrler/
│   │   └── shardgrp/
│   │       ├── server.go
│   │       ├── client.go
│   │       └── shardrpc/
│   │
│   └── main/                  # entry points
│       ├── kvsrv1d.go
│       ├── kvraft1d.go
│       ├── raft1d.go
│       └── shardgrp1d.go
│
├── scripts/
│   ├── run-tests.sh           # run all tests in containers
│   ├── chaos.sh               # kill/partition containers randomly
│   └── bench.sh               # throughput benchmark
│
└── docs/
    ├── raft-figure2.md        # reference for implementation
    └── linearizability.md     # correctness model notes
```

---

## 6. Implementation Milestones

```
Phase 1 ── kvsrv ────────────────────────────────────────────
  [1.1]  KVServer: in-memory map, versioned Put/Get handlers
  [1.2]  Clerk: RPC send, retry loop, ErrMaybe logic
  [1.3]  Lock: Acquire/Release via conditional Put
  [1.4]  Tests: TestReliablePut, TestUnreliableNet, lock tests

Phase 2 ── Raft ─────────────────────────────────────────────
  [2.1]  State Machine: Raft struct, RequestVote, ticker goroutine, heartbeats
  [2.2]  Log replication: AppendEntries, commitment, applyCh
  [2.3]  Persistence: persist/readPersist (currentTerm, votedFor, log)
  [2.4]  Compaction: Snapshot(), InstallSnapshot RPC, log trimming

Phase 3 ── kvraft + rsm ─────────────────────────────────────
  [3.1]  State Machine Submit: rsm.Submit(), reader goroutine, Op struct with unique ID
  [3.2]  State Machine Operations: KVServer DoOp, Clerk with leader tracking
  [3.3]  State Machine Compaction: Snapshot/Restore in KVServer; rsm triggers rf.Snapshot()

Phase 4 ── shardkv ──────────────────────────────────────────
  [4.1]  Config Management: ShardCtrler InitConfig/Query; ShardGrp from kvraft copy
  [4.2]  Configuration Shifts: ChangeConfigTo: Freeze→Install→Delete→PublishConfig
  [4.3]  Resilience: Fault-tolerant ChangeConfigTo (controller crash recovery)
  [4.4]  Dynamic Shards: Concurrent controllers (atomic config transitions)

Phase 5 ── Docker ──────────────────────────────────────────────────
  [5.1]  Dockerfile: multi-stage build (builder + alpine runtime)
  [5.2]  docker-compose.yml: kvsrv, kvraft cluster, shardgrp cluster
  [5.3]  Persistent volumes for Raft state
  [5.4]  scripts/run-tests.sh, chaos.sh, bench.sh
```

---

## 7. Key Design Constraints & Gotchas

| Concern | Rule |
|---------|------|
| **No shared memory** | All inter-process communication must be RPC only |
| **At-most-once** | Versioned puts prevent double-execution on retransmit |
| **Leader routing** | Clerks retry on `ErrWrongLeader` until they find the real leader |
| **Snapshot timing** | Take snapshot when `rf.PersistBytes() > 0.9 * maxraftstate` |
| **configNum on shard RPCs** | Reject FreezeShard/InstallShard/DeleteShard if configNum ≤ last seen |
| **Raft log GC** | Use `runtime.SetFinalizer` / nil slices to allow GC of discarded entries |
| **Election timeout** | Must be randomized 300–500ms; heartbeat interval ≤ 100ms |
| **Race detector** | Always test with `-race`; grade runs without it |
| **Linearizability checker** | The tester uses porcupine; all concurrent ops must be serializable |
| **ErrMaybe in shardkv** | shardkv1/client.go must propagate ErrMaybe from inner shardgrp Put |

---

## 8. Correctness Properties

1. **Safety (Raft):** At most one leader per term; committed entries are never lost.
2. **Liveness (Raft):** A leader is elected within 5 seconds if a majority is reachable.
3. **Linearizability (kvsrv / kvraft):** All operations appear to execute atomically at a single point between their invocation and response.
4. **Shard exclusivity (shardkv):** At any instant, exactly one shardgrp serves each shard.
5. **At-most-once puts:** Versioned Put ensures each client write is applied exactly once despite retransmits.
6. **Snapshot consistency:** Restored snapshots only advance state forward; they never regress.
