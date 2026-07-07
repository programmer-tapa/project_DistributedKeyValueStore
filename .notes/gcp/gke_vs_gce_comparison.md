# GCP Compute Layer Comparison: GKE vs. GCE for Distributed Key-Value Store (DKV)

This document provides a deep architectural and financial comparison between deploying the Distributed Key-Value Store (DKV) on **Google Kubernetes Engine (GKE)** versus **Google Compute Engine (GCE)**. Because DKV relies on the **Raft Consensus Protocol** and **dynamic sharding**, the underlying compute layer must support stateful stability and zone-level fault tolerance.

---

## 1. Architectural Feature Comparison

Raft-based stateful systems have strict infrastructure requirements. Here is how GKE and GCE stack up against them:

| Architectural Requirement | GKE (Google Kubernetes Engine) | GCE (Google Compute Engine) |
| :--- | :--- | :--- |
| **Stable Network Identities** | **Excellent (Native)**<br>Using a **Headless Service** (`clusterIP: None`), Kubernetes CoreDNS automatically generates stable, predictable A-records (e.g., `kvraft-0.kvraft-service.dkv.svc.cluster.local`). If a pod restarts, it retains its DNS name. | **Complex (Manual)**<br>Requires provisioning static internal IPs for VMs or setting up and updating private Cloud DNS zones. Managing DNS update propagation during VM recreation can introduce consensus split-brain risks. |
| **Storage Persistence & Mounting** | **Excellent (Native)**<br>StatefulSets use `volumeClaimTemplates` to automatically provision and bind **Zonal SSD Persistent Disks (`pd-ssd`)**. If a node fails, GKE handles the detach/attach lifecycle of the disk to the new node automatically. | **Complex**<br>Requires configuring **Stateful Disks** inside GCE **Stateful Managed Instance Groups (MIGs)**. This involves highly verbose configuration, disk detach/attach delays, and writing custom shell scripts to ensure mounting is successful. |
| **Zone-Level Fault Tolerance** | **Declarative & Simple**<br>Uses `podAntiAffinity` rules with `topologyKey: topology.kubernetes.io/zone`. You simply tell the scheduler: *"Do not run two replicas of `dkv-kvraft` in the same zone."* GKE guarantees this across your zonal node pools. | **Manual / Verbose**<br>Requires manually creating VMs across specific zones or configuring a complex regional MIG distribution policy. Scaling or shifting resources requires manual topological planning. |
| **Scaling & Shard Groups** | **Highly Dynamic & Cost-Effective**<br>To add a new Shard Group (e.g., Group 3), you just apply a new `StatefulSet` manifest. Dozens of Shard Groups can share the same node pool, packing resources efficiently without port conflicts. | **Expensive & Rigid**<br>Adding a Shard Group requires either provisioning 3 new VMs (high cost, slow boot) or running multiple `systemd` daemons per VM (leads to port conflicts, difficult resource isolation, and configuration hell). |
| **Updates & Self-Healing** | **Automated**<br>Supports native declarative rolling updates. GKE restarts unhealthy pods, performs readiness/liveness checks, and gathers logs out of the box via Google Cloud Logging. | **Manual Operations**<br>Requires managing `systemd` daemons, writing custom health-check scripts, managing log rotation (`journald`), and manually coordinating rolling updates to avoid dropping Raft quorum. |

---

## 2. Why GKE is the Clear Winner

The Distributed Key-Value Store relies on **sharding** and **Raft consensus**. This means:
1. You have **multiple shard groups** (each with 3 replicas) and a **metadata store** (another 3 replicas).
2. The infrastructure must coordinate **stable DNS, stable storage mounts, and strict zonal separation** for all of them.

If you choose **GCE**, you will spend 80% of your time writing Terraform, Ansible, and Bash scripts to handle VM orchestration, stateful disk attachment, DNS mapping, and health-check reboots. 

If you choose **GKE**, Kubernetes **StatefulSets** do 95% of this heavy lifting for you out of the box. You only need to write declarative YAML manifests.

---

## 3. When is GCE Actually Worth Considering?

You should only choose **GCE** if:
* **Zero-overhead performance** is your absolute top priority, and you cannot tolerate the minimal network/virtualization overhead of Kubernetes container networking.
* Your team has **zero Kubernetes experience** but has extensive, pre-existing VM automation tools (e.g., Ansible/Packer).
* There are organizational compliance constraints preventing the use of container orchestrators.

---

## 4. Cost Comparison (Summary)

| Cost Dimension | GCE (3-Node Manual Pack) | GKE Autopilot | GKE Standard |
| :--- | :--- | :--- | :--- |
| **Compute Cost** | ~$147 / mo | **~$111 / mo** | ~$147 / mo |
| **Management Fee** | **$0** | $0 (using Free Tier) | $0 (using Free Tier) |
| **Storage Cost (SSD)** | Same (~$0.17/GB) | Same (~$0.17/GB) | Same (~$0.17/GB) |
| **Idle Resource Waste** | High (paying for full VMs) | **None (zero waste)** | Medium (limited by VM boundaries) |
| **Ops Maintenance Cost** | **Extremely High** (manual labor) | **Near Zero** | **Near Zero** |
| **Total Financial Cost** | **Medium-High** | **Lowest** | **Medium** |

---

## 5. Recommendation

> [!IMPORTANT]
> **Choose GKE.** 
> It turns a complex, error-prone distributed systems orchestration nightmare into a standard, declarative configuration. GKE Autopilot or GKE Standard with regional node pools will give you production-grade, multi-zone resilience with minimal operational overhead.
