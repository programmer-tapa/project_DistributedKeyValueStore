# kvsrvd

`kvsrvd` starts a single-node linearizable key-value server. In this architecture, it is deployed as the **Config Metadata Server** (the single source of truth for the cluster's shard configurations).

---

## Capabilities

1. **Linearizability & Versioning:** Enforces version checks on `Put` operations to guarantee linearizable write outcomes.
2. **Backing Store for Shard Controller:** Used exclusively by the `shardctrler` control plane to persist and read global topology configurations.
3. **Observability:** Supports exposing a Prometheus metrics HTTP server.

---

## Command Line Flags

* `--addr`: The TCP address for the server to listen on (default is `:9000`).
* `--metrics-addr`: Address (IP:port) to bind the Prometheus metric server (e.g. `:9091`).
