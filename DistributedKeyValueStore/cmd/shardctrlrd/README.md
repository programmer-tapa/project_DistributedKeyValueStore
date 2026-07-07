# shardctrlrd

`shardctrlrd` runs the **Shard Controller** service. It functions as the orchestrator of the cluster control plane—detecting membership changes, balancing shard layout allocations, and managing active shard migrations.

---

## Capabilities

1. **Topology Initialization:** Sets up the cluster configurations in the metadata server (`kvsrvd`) during initial bootstrap.
2. **Re-balancing & Configuration Drift Migration:**
   * Compares the target group membership configuration against the current active layout stored in the metadata server.
   * If a membership change (add/remove group) is detected, it computes the new balanced layout and triggers the **Four-Phase Shard Migration Protocol** (`Freeze ➔ Install ➔ Delete ➔ Publish`) to migrate shards between replica groups safely.
3. **Exposes Prometheus Observability:** Exposes active configuration version numbers, migration durations, and RPC metrics.

---

## Command Line Flags

* `--kvsrv-addr`: The address of the single-node Config Metadata Server (`kvsrvd`) (default is `localhost:9000`).
* `--groups`: A semicolon-separated mapping of Group IDs (GIDs) to their replica group TCP addresses (e.g. `1=kvraft-0:8000,kvraft-1:8000;2=shardgrp2-0:8010`).
* `--metrics-addr`: The address (IP:port) to bind the Prometheus metric server (default is `:2112`).
