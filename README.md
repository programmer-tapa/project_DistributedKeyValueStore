# Distributed Key-Value Store (DKV)

A production-grade, distributed, sharded, and fault-tolerant key-value store based on Raft consensus and replicated state machines.

---

## Quickstart

### 1. Start the Cluster
Run the automation script from the repository root to compile the client binary and boot up the 10-node Docker cluster (including observability stack):

```bash
./deploy.sh
```

---

## Interacting with the Store (Client CLI)

Because the replica nodes register using internal Docker hostnames (`dkv-kvraft-0:8000`), the client CLI must run within the Docker bridge network (`docker_kvnet`) to resolve hostnames and route requests correctly.

Depending on your current working directory, run the client as follows:

### Option A: From the Repository Root (Recommended)
If you are at the root directory of the repository, run:
```bash
# Write a key-value pair
docker run --rm --network docker_kvnet -v "$PWD"/DistributedKeyValueStore/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 put mykey myvalue

# Read a key-value pair
docker run --rm --network docker_kvnet -v "$PWD"/DistributedKeyValueStore/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 get mykey
```

### Option B: From the `DistributedKeyValueStore` Folder
If you have navigated into the Go module subdirectory (`cd DistributedKeyValueStore`), run:
```bash
# Write a key-value pair
docker run --rm --network docker_kvnet -v "$PWD"/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 put mykey myvalue

# Read a key-value pair
docker run --rm --network docker_kvnet -v "$PWD"/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 get mykey
```

---

## Observability & Monitoring

Once deployed, the following dashboards and endpoints are exposed locally:

*   **Grafana Dashboards**: [http://localhost:3003](http://localhost:3003) (Login: `admin` / `admin`)
*   **Prometheus Metrics**: [http://localhost:9090](http://localhost:9090)
*   **Shard Controller Endpoint**: `localhost:9100`
*   **Configuration Metadata Store (kvsrv)**: `localhost:9000`

---

## System Topology & Wire Diagram

The following diagram represents the network connectivity and interactions between the components of the DKV cluster:

```mermaid
flowchart TD
    subgraph Client ["Client Space"]
        CLI["dkv-client (CLI)"]
    end

    subgraph Control ["Control & Metadata Plane"]
        CTR["dkv-shardctrler"] <--> KVSRV["dkv-kvsrv (Config Store)"]
    end

    subgraph Group1 ["Shard Group 1 (GID 1)"]
        direction LR
        K0["dkv-kvraft-0"] <-->|Raft Consensus| K1["dkv-kvraft-1"]
        K1 <-->|Raft Consensus| K2["dkv-kvraft-2"]
        K2 <-->|Raft Consensus| K0
    end

    subgraph Group2 ["Shard Group 2 (GID 2)"]
        direction LR
        S0["dkv-shardgrp2-0"] <-->|Raft Consensus| S1["dkv-shardgrp2-1"]
        S1 <-->|Raft Consensus| S2["dkv-shardgrp2-2"]
        S2 <-->|Raft Consensus| S0
    end

    subgraph Observability ["Observability Stack"]
        PROM["dkv-prometheus"] -.->|Scrapes Metrics| KVSRV
        PROM -.->|Scrapes Metrics| K0
        PROM -.->|Scrapes Metrics| S0
        GRAF["dkv-grafana"] ===>|Queries| PROM
    end

    CLI -.->|1. Queries Topology| KVSRV
    CLI ===>|2. Routes Put/Get| K0
    CLI ===>|2. Routes Put/Get| S0
```
