# DKV Client Integration & Communication Guide

This document explains the communication flow between application clients and the DKV cluster, and details how to integrate the client library wrappers into other services.

---

## 1. How the Client Connects and Routes Data

The client wrappers (Python, TypeScript, PHP, Java) act as subprocess-bridges to the compiled Go `dkv-client` binary. The actual network communication is managed automatically in Go using gRPC:

```
[ Your Application Service ]
             │
             ▼  (Subprocess Execution)
      [ dkv-client CLI ]
             │
             ├─► [ dkv-kvsrv (Config Store) ] ──► Queries shard topology configuration
             │
             ▼  (Direct gRPC Call)
      [ dkv-kvraft Leader Node ] ──────────────► Executes transactions (Put/Get)
```

### Flow Steps:
1.  **Subprocess Execution**: The application triggers `dkv-client` locally, passing parameters like `--ctrler-addr`, key, value, and action.
2.  **Topology Discovery**: The client makes a gRPC call to the Shard Controller/config store (`kvsrv:9000`) to find out which shard group (GID 1 or GID 2) is responsible for the target key.
3.  **Consensus Execution**: The client establishes a direct gRPC connection to the responsible Shard Group leader (e.g. `dkv-kvraft-0`). If that node is no longer the leader, the node redirects the client to the current leader.
4.  **Raft Commit**: The leader logs the write to Raft log, achieves consensus across the cluster, updates the state machine, and replies.
5.  **Parsing Output**: The client CLI prints the outcome to standard output (e.g. `OK` or `Value: val (Version: ver)`), which the wrapper parses and returns as native variables.

---

## 2. Integration Requirements for Application Services

To read or write data to the DKV store from another microservice, that microservice needs:

### A. The Wrapper File
Copy the corresponding client library wrapper from `/library` into your application project codebase:
*   **Python**: `dkv.py`
*   **TypeScript**: `dkv.ts`
*   **PHP**: `DKVClient.php`
*   **Java**: `DKVClient.java`

### B. The Compiled Binary
The compiled `dkv-client` binary executable must be present on your application's filesystem so the wrapper can call it.
*   **Local Execution**: Ensure `dkv-client` is in your host's `$PATH` or provide the path to `dkv-client` in the wrapper constructor.
*   **Docker Container Execution**: You can mount the compiled binary into your application container via a read-only volume mount.

---

## 3. Docker Compose Example for an App Container

Below is an example configuration for integrating an application service into the DKV network:

```yaml
services:
  my-app:
    image: python:3.11-slim
    command: python app.py
    networks:
      - docker_kvnet # Must connect to the same network bridge
    volumes:
      # Mount the compiled dkv-client binary from the host into the container's executable path
      - ./DistributedKeyValueStore/bin/dkv-client:/usr/local/bin/dkv-client:ro
    environment:
      # Point to kvsrv address inside the Docker bridge network
      - DKV_CTRLED_ADDR=kvsrv:9000
      - DKV_CLIENT_BIN=/usr/local/bin/dkv-client

networks:
  docker_kvnet:
    external: true
```
