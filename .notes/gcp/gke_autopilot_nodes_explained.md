# GKE Autopilot Node Status Explained

When you first spin up a **GKE Autopilot** cluster and run `kubectl get nodes`, you will often see the default bootstrap node in a `NotReady,SchedulingDisabled` status:

```
NAME                                         STATUS                        ROLES    AGE     VERSION
gk3-dkv-cluster-default-pool-f81d134f-kgvc   NotReady,SchedulingDisabled   <none>   3m57s   v1.35.5-gke.1057002
```

**This is 100% normal, expected behavior, and means your cluster is completely healthy.**

---

## 1. Why does this happen?

### The Bootstrap Node
In GKE Autopilot, Google manages the virtual machines (nodes) for you. The initial node you see belongs to the **default-pool** and is a system bootstrap node. GKE uses this node to run internal cluster management services (such as CoreDNS, logging agents, and metrics collectors).

### `SchedulingDisabled`
GKE Autopilot cordons this system node (`SchedulingDisabled`) to guarantee that your user application workloads cannot be scheduled on it. This isolates critical Kubernetes system services from your custom applications.

### `NotReady` (During Initial Boot)
Because the cluster was *just* created, this system bootstrap node is running its final cloud-init scripts, mounting security configurations, and starting up daemonsets. It will transition to `Ready` shortly, but it will remain `SchedulingDisabled` for your custom deployments.

---

## 2. How Autopilot provisions nodes for your app

When you deploy your Distributed Key-Value Store (DKV) workloads (such as your Shard Groups or Metadata Store):

1.  You will apply your manifests (e.g., `kubectl apply -f dkv-kvraft.yaml`).
2.  GKE Autopilot will detect that new pods have been created and are requesting compute resources (e.g., `0.25 vCPU`, `256MB RAM`).
3.  Autopilot will **automatically provision new, optimized worker nodes** in the appropriate availability zones.
4.  It will schedule your pods on these newly created nodes, and automatically attach your Zonal Persistent Disks.
5.  If you run `kubectl get nodes` during this time, you will see new nodes appear in the list that are **`Ready`** and hosting your application.

---

## 3. Verify Cluster Health

To confirm that the Kubernetes control plane is fully active and healthy, you can inspect the running system pods in the `kube-system` namespace:

```bash
kubectl get pods -A
```

You should see system services like `kube-dns` (CoreDNS) and network agents in a `Running` state.
