# Running Python Clients in GKE

This document provides a step-by-step guide to executing Python-based client scripts (such as `sample.py`) against a highly secure, private Distributed Key-Value Store (DKV) cluster deployed in GKE Autopilot.

---

## 1. The Networking Challenge

By default, the DKV cluster is deployed with production-grade security:
*   All services are of type **`ClusterIP`** (private to the VPC).
*   Individual Raft pod IPs (`10.11.x.x`) and internal DNS names (`*.dkv.svc.cluster.local`) are **only routable from inside the GKE cluster network**.
*   Running `sample.py` directly on your local host machine will fail during the data operation phase because your host machine cannot resolve or route to these private pod endpoints.

---

## 2. The Cloud-Native Solution: In-Cluster Testing

To test your database using your Python scripts, we spin up a temporary, lightweight Python pod directly inside the `dkv` namespace, copy your client files and compiled Go binary into it, and execute it. 

Because this pod resides inside the cluster network, it has native access to both the Metadata Store and the Raft shard groups!

---

## 3. Step-by-Step Execution Manual

Execute these commands from your container workspace directory (`/workspace`):

### Step 1: Start a temporary Python pod
Spin up a pod running `python:3.11-slim` and tell it to sleep for an hour to keep it alive:
```bash
kubectl run dkv-python-test -n dkv --image=python:3.11-slim --restart=Never -- sleep 3600
```
*Wait 5-10 seconds for the pod status to transition to `Running`.*

### Step 2: Copy your files into the pod
Create the required directory structure inside the pod, then copy your Python script, client library, and the compiled Go client binary into the container:
```bash
# 1. Create directory structure in the pod
kubectl exec -n dkv dkv-python-test -- mkdir -p /playground /library /DistributedKeyValueStore/bin

# 2. Copy the playground test script
kubectl cp -n dkv ./playground/sample.py dkv-python-test:/playground/sample.py

# 3. Copy the Python DKV library
kubectl cp -n dkv ./library/dkv.py dkv-python-test:/library/dkv.py

# 4. Copy the compiled Go client binary
kubectl cp -n dkv ./DistributedKeyValueStore/bin/dkv-client dkv-python-test:/DistributedKeyValueStore/bin/dkv-client
```

### Step 3: Run the client script
Execute the Python script inside the GKE pod using `kubectl exec`. Pass the internal Metadata Store address as an environment variable:
```bash
kubectl exec -it -n dkv dkv-python-test -- env DKV_CTRLED_ADDR=metadata-store-service:9000 python /playground/sample.py
```

### Step 4: Clean up
Once testing is complete, delete the temporary pod to release GKE compute resources:
```bash
kubectl delete pod -n dkv dkv-python-test
```

---

## 4. How It Works Under the Hood

When you execute the script in **Step 3**:
1.  The Python interpreter inside the pod runs `sample.py`.
2.  The `DKVClient` wrapper calls the Go binary `/DistributedKeyValueStore/bin/dkv-client` using the `--ctrler-addr metadata-store-service:9000` flag.
3.  Because the pod is inside the cluster, it successfully resolves `metadata-store-service:9000` via CoreDNS and gets the active shard routing configuration.
4.  When performing the `PUT` or `GET` operations, the Go client successfully resolves and connects directly to the private Raft replicas (e.g., `kvraft-1-0.kvraft-1-service.dkv.svc.cluster.local:8000`) to complete the transactions in milliseconds.
