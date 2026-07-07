# DKV Shard Architecture: Storage, Persistence, and Resilience

This document explains the network, storage, persistence, and crash resilience architecture of the Distributed Key-Value Store (DKV) Shard Groups, using **`shard-group-1.yaml`** as a case study.

---

## 1. Network Identity: Headless Services & Peer Discovery

At the top of `shard-group-1.yaml`, a **Headless Service** (`kvraft-1-service`) is defined with `clusterIP: None`. 

### Why Headless?
Unlike standard services that load-balance requests across pods, a Headless Service does not have a single IP. Instead, it interfaces with the **StatefulSet** to register stable, individual DNS records (Fully Qualified Domain Names - FQDNs) for each pod in the cluster:
*   `kvraft-1-0.kvraft-1-service.dkv.svc.cluster.local`
*   `kvraft-1-1.kvraft-1-service.dkv.svc.cluster.local`
*   `kvraft-1-2.kvraft-1-service.dkv.svc.cluster.local`

This stable network identity is **essential for the Raft consensus engine**, enabling individual replicas to discover and communicate directly with their specific peers.

---

## 2. Storage Architecture: Dynamic Cloud SSD Provisioning

Inside the StatefulSet definition, storage is managed via the **`volumeClaimTemplates`** block:

```yaml
  volumeClaimTemplates:
    - metadata:
        name: raft-state
      spec:
        accessModes: [ "ReadWriteOnce" ]
        storageClassName: premium-rwo  # High-performance SSD on GCP GKE
        resources:
          requests:
            storage: 10Gi
```

*   **Dedicated Allocation**: Rather than sharing a single volume, GKE Autopilot dynamically requests **three separate physical SSD volumes** from Google Cloud (one for each of the 3 replicas).
*   **Performance Tiering**: The `storageClassName: premium-rwo` maps directly to **Google Cloud Persistent Disk SSDs (PD-SSD)**, providing high IOPS and microsecond-level write latencies.
*   **Capacity**: Each replica is allocated its own dedicated **10 Gigabyte** storage volume.

---

## 3. Data Persistence: The File System

Inside the database container, the SSD volume is mounted and utilized for raw consensus logs:
*   **Mount Path**: The GCP SSD volume is mounted at the directory **`/var/lib/raft`** (configured via `volumeMounts`).
*   **Startup Configuration**: The database binary (`kvraftd`) is executed with the flag `--persist-dir "/var/lib/raft"`.
*   **Durable Files**: The Raft consensus engine writes its critical state files here:
    *   **Write-Ahead Log (WAL)**: The sequence of uncommitted and committed database transactions.
    *   **Consensus Metadata**: The replica's current election term and local vote details.
    *   **State Machine Snapshots**: Compacted snapshots of the database key-value state.
*   **Hardware Guarantee**: Every disk write bypasses the container's temporary layer and is written directly to GCP's redundant physical SSD hardware, ensuring absolute durability even if the container crashes or is deleted.

---

## 4. Crash Recovery & Resilience Models

Thanks to the combined power of **StatefulSets** and **Raft Consensus**, the DKV cluster can survive catastrophic failures with zero data loss and zero downtime.

### Scenario A: A Single Pod Crashes (e.g., `kvraft-1-0` dies)
1.  **Quorum Survival**: The remaining two replicas (`kvraft-1-1` and `kvraft-1-2`) stay online. Because 2 out of 3 represents a majority (quorum), they immediately hold an election (if the crashed pod was the leader) and continue serving client reads and writes. **No database downtime!**
2.  **Self-Healing Pod**: GKE automatically detects the pod failure and provisions a fresh replacement pod named `kvraft-1-0`.
3.  **Automatic Storage Re-attachment**: GKE detaches the persistent GCP SSD volume from the old node/pod and **automatically re-attaches it to the new `kvraft-1-0` pod**.
4.  **Instant Memory Recovery**: Upon startup, the new container reads `/var/lib/raft`, restoring its WAL, election terms, and snapshots instantly.
5.  **Catch-up Synchronization**: The active leader detects that `kvraft-1-0` has rejoined, sends it any new transactions it missed while it was offline, and the replica seamlessly resumes active participation in the consensus quorum.

### Scenario B: An Entire GCP Data Center Zone Goes Offline
1.  **Zonal Anti-Affinity**: The `podAntiAffinity` rules ensure that the 3 replicas are scheduled across **three distinct physical availability zones** (e.g., `us-central1-a`, `-b`, and `-c`).
2.  **Continuous Operations**: If an entire Google data center zone (e.g., `us-central1-b`) suffers a power outage or natural disaster:
    *   The pod in zone `b` goes offline.
    *   The pods in zones `a` and `c` remain online.
    *   Since 2 out of 3 replicas are online, they maintain **Raft write quorum**, elect a leader, and the database remains **100% operational and writable**.
3.  **Zone Recovery**: Once Google restores the affected zone, the node recovers, the pod starts up, the SSD re-attaches, and it catches up automatically.
