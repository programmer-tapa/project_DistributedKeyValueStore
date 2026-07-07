# kvraftd

`kvraftd` runs a single replica server process belonging to a **Shard Group** (replicated shard partition). Each Shard Group runs a cluster of `kvraftd` nodes utilizing Raft consensus to synchronize key-value changes.

---

## Capabilities

1. **Raft Consensus Integration:** Hosts a local `Raft` peer instance to replicate client logs, handle leader elections, and track commit status.
2. **Replicated State Machine (RSM):** Connects Raft consensus with the `ShardGroup` state machine, applying committed commands to the database partition.
3. **Log Compaction & Snapshotting:** Triggers automatic log compaction when the Raft persistent log bytes exceed `--max-raft-state`.
4. **Prometheus Monitoring:** Exposes an optional HTTP `/metrics` endpoint for real-time observability of consensus state and system RPCs.

---

## Command Line Flags

* `--me`: The index of this node in the peers list (0-indexed).
* `--peers`: A comma-separated list of all server peer addresses in the shard group (e.g. `node0:8000,node1:8000,node2:8000`).
* `--gid`: The Group ID (GID) associated with this replica group.
* `--persist-dir`: Directory path where the Raft engine saves its persistent metadata log and database snapshots.
* `--max-raft-state`: The threshold size in bytes before log compaction/snapshotting is triggered (default is `10000` bytes).
* `--metrics-addr`: Address (IP:port) to bind the Prometheus metric server (e.g. `:9091`).
