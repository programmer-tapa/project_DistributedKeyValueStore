# Zero-Downtime Redeployment on GKE

This document explains the mechanism and operational procedure for redeploying newly built container images to a GKE Autopilot cluster with **zero downtime**, specifically detailing how this maintains the **Raft consensus quorum** during updates.

---

## 1. The Challenge: Image Tag Caching

When you rebuild and push new container images to Google Artifact Registry using the same tag (like `:latest` or a stable version tag):
*   **No Manifest Changes**: The image reference in your Kubernetes YAML files (e.g., `.../kvraftd:latest`) remains exactly the same.
*   **Apply is Ignored**: Running `kubectl apply -f` will **do nothing** because Kubernetes compares the YAML schemas and detects no changes.
*   **Node Caching**: GKE nodes cache container images locally. They will not automatically poll the registry or pull the fresh image unless forced to restart the pods.

---

## 2. The Solution: Rolling Restarts

To force GKE to pull your newly pushed images, you must trigger a **Rolling Restart** (also known as a rolling update). 

Run these commands in your container terminal:

```bash
# Gracefully perform rolling restarts of all core database components
kubectl rollout restart -n dkv deployment/shardctrlr
kubectl rollout restart -n dkv statefulset/kvraft-1
kubectl rollout restart -n dkv statefulset/kvraft-2
kubectl rollout restart -n dkv statefulset/metadata-store
```

---

## 3. How Rolling Restarts Maintain Raft Quorum

In a distributed system, maintaining availability during deployments is critical. Because our Shard Groups are deployed with **3 replicas** and strict **zonal anti-affinity**, Kubernetes executes the rollout sequentially:

1.  **Step-by-Step Replacement**: GKE terminates a single pod (e.g., `kvraft-1-0`) and immediately starts a new pod with the fresh image.
2.  **Quorum Preservation**: While `kvraft-1-0` is restarting, the other two replicas (`kvraft-1-1` and `kvraft-1-2`) **remain online and active**.
3.  **No Downtime**: Because **2 out of 3 replicas are online**, the Shard Group retains its **Raft write quorum**. Clients can continue to read and write keys without interruption.
4.  **Sequential Rollout**: Once the new `kvraft-1-0` pod passes its readiness probes and joins the cluster, GKE moves on to restart `kvraft-1-1`, repeating the process until all replicas are updated.

---

## 4. Monitoring & Verifying the Rollout

You can monitor the rollout progress and verify its health using these operational commands:

### Watch Pod Transitions Live
Observe the old pods terminating and the new pods starting up in real-time:
```bash
kubectl get pods -n dkv -w
```

### Verify Rollout Completion Status
Block the terminal and wait until the rolling update is fully completed, healthy, and verified:
```bash
kubectl rollout status -n dkv statefulset/kvraft-1
kubectl rollout status -n dkv statefulset/kvraft-2
kubectl rollout status -n dkv deployment/shardctrlr
```

---

## 5. Alternative: When to use `kubectl apply`

You should **only** use `kubectl apply -f DistributedKeyValueStore/deployments/kubernetes/` if:
1.  You modified the actual Kubernetes YAML files (e.g., changed CPU/Memory limits, updated environment variables, or added volume claims).
2.  In this case, Kubernetes detects the schema difference and automatically triggers the rolling update for you.
