# Storage Engine Durability: Full Serialization vs. Write-Ahead Logging (WAL)

This document analyzes the durability design trade-offs of the Distributed Key-Value Store's persistence layer, comparing the current **Full State Serialization** implementation with **Append-Only Write-Ahead Logging (WAL)**.

---

## 1. Current Implementation: Full State Serialization

In our database, when `Raft.Start()` or log replication updates the consensus log, it calls `rf.persist()`. This triggers a complete rewrite of the state:

```
[ RAM (rf.log slice) ] 
       │
       ▼ (1. GOB encode entire log slice)
[ Binary Bytes Buffer ]
       │
       ▼ (2. writeAtomic: Write all bytes to raftstate.bin.tmp)
[ Temp File ]
       │
       ▼ (3. f.Sync(): Flush OS file cache to hardware disk)
[ Stable Disk ]
       │
       ▼ (4. os.Rename: Atomically swap pointer to raftstate.bin)
[ raftstate.bin ] (Old state replaced entirely)
```

### Advantages
* **Simplicity:** The recovery path is trivial. On boot, the engine reads one file, decodes it into a slice, and startup is complete.
* **No Corruption Indexing:** By completely swapping the file atomically via `os.Rename`, we eliminate the risk of half-written log entries at the tail of a file corrupting the database.
* **Trivial Conflicting Log Truncation:** If a follower receives entries from a new leader that overwrite uncommitted conflicting entries, the follower simply truncates its in-memory slice and calls `persist()`. The disk state mirrors the memory state automatically.

### Disadvantages (Performance Bottlenecks)
* **Write Amplification:** Writing a new 100-byte log entry when the existing log is 5 MB requires writing the entire 5 MB back to disk.
* **CPU Serialization Overhead:** Every single log change requires GOB-encoding the entire array of log entries, which consumes CPU cycles as the log grows.

---

## 2. Production Optimization: Segmented WAL & Snapshotting

High-throughput distributed systems (such as `etcd`'s `wal` package, CockroachDB, or RocksDB) implement an **Append-Only WAL** combined with **Compaction Checkpoints (Snapshots)**.

```
                      ┌──────────────────────────────────────────────┐
                      │                 On-Disk Layout               │
                      │                                              │
                      │  ┌──────────────┐     ┌───────────────────┐  │
                      │  │  Snapshot    │     │  WAL (Append-Only)│  │
                      │  │ (State up to │ ➔   │  [Entry 1001]     │  │
                      │  │  Entry 1000) │     │  [Entry 1002]     │  │
                      │  └──────────────┘     │  [Entry 1003]...  │  │
                      │                       └───────────────────┘  │
                      └──────────────────────────────────────────────┘
```

### The Write Path (Append-Only)
Instead of rewriting the entire history, new log entries are strictly appended to the end of an open WAL file.
* **$O(1)$ Complexity:** Appending is extremely cheap. The disk head only writes the new bytes to the end of the file.
* **Minimal Write Amplification:** Only the new entry's bytes and a small checksum header are written to disk.

### The Recovery Path (Replay)
On boot, the node reads the latest snapshot, loads it into memory, and then reads the WAL sequentially from the snapshot index forward, replaying log entries to rebuild the active state.

### Compaction (Log Truncation)
To prevent the WAL from growing infinitely (which would consume excessive disk space and cause long recovery startup times), the system periodically:
1. Writes the current state machine database memory map to a new snapshot file on disk.
2. Truncates/purges the WAL entries older than the snapshot index.

---

## 3. Engineering Complexities of WAL Implementation

Moving to an append-only WAL requires implementing several advanced storage systems:

| Requirement | Description |
| :--- | :--- |
| **Record Framing** | Defining length prefixes, headers, or boundary markers to know where one binary GOB record ends and the next begins in a continuous stream. |
| **Checksums & Truncation** | Attaching CRC32/Adler32 checksums to each record. On boot, if a partial write is detected (e.g. from a crash mid-write), the parser must identify the corruption and truncate the trailing invalid bytes safely. |
| **Conflict Truncation** | Implementing file seek and sector truncation (`os.Truncate`) to prune conflicting logs on followers when leaders rewrite uncommitted history. |
