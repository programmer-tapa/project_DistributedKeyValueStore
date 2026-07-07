# DKV Docker Compose Architecture & Configuration

This document provides a detailed walkthrough of the `docker-compose.yml` file used to orchestrate the Distributed Key-Value Store (DKV) cluster.

---

## 1. Network Configuration (`networks:`)

```yaml
networks:
  kvnet:
    name: docker_kvnet
    driver: bridge
```
*   **`driver: bridge`**: Creates an isolated virtual network bridge on the host machine. All containers in this network can communicate with each other using their service names as hostnames (e.g., `kvsrv:9000`).
*   **`name: docker_kvnet`**: Explicitly names the network `docker_kvnet` to prevent Docker Compose from prefixing it with the project folder name (which would result in `project_distributedkeyvaluestore_kvnet`). This guarantees that scripts like `chaos.sh` can cleanly reference `docker_kvnet`.

---

## 2. YAML Anchors & Templates (`x-raft-common:`)

```yaml
x-raft-common: &raft-common
  build:
    context: ./DistributedKeyValueStore
    dockerfile: deployments/docker/Dockerfile
    args:
      BUILD_TARGET: kvraftd
  networks: [kvnet]
  restart: on-failure
```
To avoid repeating the boilerplate build configuration for 6 different Raft replica nodes, DKV uses a YAML anchor (`&raft-common`).
*   **`context`**: Points to the Go module root directory containing `go.mod`.
*   **`BUILD_TARGET: kvraftd`**: Passes a build-time argument to the multi-stage `Dockerfile` to build only the `kvraftd` binary for these replica nodes.
*   **`<<: *raft-common`**: Services reference this template to inherit its network, build, and restart rules.

---

## 3. Services Layout (`services:`)

The cluster contains **10 services** representing a complete distributed shard-managed storage architecture with built-in observability:

### A. Metadata Configuration Store (`kvsrv`)
```yaml
  kvsrv:
    build:
      context: ./DistributedKeyValueStore
      dockerfile: deployments/docker/Dockerfile
      args:
        BUILD_TARGET: kvsrvd
    container_name: dkv-kvsrv
    ports:
      - "9000:9000"
```
*   **Role**: Runs `kvsrvd` (from Lab 2). It acts as the centralized configuration/metadata store holding the topology assignments (e.g., which group handles which shard).
*   **Port**: Mapped to host port `9000` so local clients can read the current cluster topology configuration.

---

### B. Shard Group 1 (GID 1) & Shard Group 2 (GID 2)
```yaml
  kvraft-0:
    <<: *raft-common
    container_name: dkv-kvraft-0
    command: ["--me", "0", "--peers", "kvraft-0:8000,kvraft-1:8000,kvraft-2:8000", "--gid", "1", "--persist-dir", "/var/lib/raft", "--metrics-addr", ":9091"]
    ports: ["8000:8000"]
    volumes:
      - dkv-kvraft0-data:/var/lib/raft
```
*   **Structure**: Consists of two separate shard groups (GID 1 and GID 2). Each shard group is a 3-node Raft consensus cluster (`kvraft-0..2` and `shardgrp2-0..2`).
*   **Commands**:
    *   `--me`: The ID of this replica node within the group.
    *   `--peers`: Hostnames of all nodes in this group.
    *   `--gid`: Group ID identifying which shard partition cluster they belong to.
    *   `--persist-dir`: Directory where Raft logs and snapshots are persisted.
*   **Volume Mount**: Mounts a local persistent volume to retain Raft state database across container updates.

---

### C. Shard Controller (`shardctrler`)
```yaml
  shardctrler:
    build:
      context: ./DistributedKeyValueStore
      dockerfile: deployments/docker/Dockerfile
      args:
        BUILD_TARGET: shardctrlrd
    container_name: dkv-shardctrler
    command: ["--kvsrv-addr", "kvsrv:9000", "--groups", "1=kvraft-0:8000,...;2=shardgrp2-0:8010,...", "--metrics-addr", ":9091"]
    ports: ["9100:9100"]
```
*   **Role**: Runs `shardctrlrd`. This is the brain of the cluster. It periodically queries the metadata store (`kvsrv`) and balances shard ownership assignments. When a group joins or leaves, the controller updates the metadata configuration, triggering shard migrations between the groups.

---

### D. Observability Stack (`prometheus` & `grafana`)
```yaml
  prometheus:
    image: prom/prometheus:latest
    container_name: dkv-prometheus
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro

  grafana:
    image: grafana/grafana:latest
    container_name: dkv-grafana
    ports: ["3003:3000"]
```
*   **Prometheus**: Mounts `prometheus.yml` to scrape system metrics periodically from port `9091` on all 8 database nodes.
*   **Grafana**: Exposes port `3003` to visualize the performance metrics (e.g. Raft logs status, transaction latencies) via pre-configured admin dashboards.

---

## 4. Persistent Volumes (`volumes:`)

```yaml
volumes:
  dkv-kvraft0-data:
  dkv-kvraft1-data:
  dkv-kvraft2-data:
  dkv-shardgrp2-0-data:
  dkv-shardgrp2-1-data:
  dkv-shardgrp2-2-data:
  dkv-grafana-data:
```
*   These named volumes are managed by Docker. They prevent loss of database files and logs when containers are stopped, rebuilt, or restarted.
*   Prefixing them with `dkv-` ensures they are clearly categorized and distinct from other project volumes on your host.
