# GCP & GKE Operations: Command Reference Cheat Sheet

This document contains a comprehensive collection of all the essential commands used to authenticate, build, push, deploy, monitor, debug, and tear down the Distributed Key-Value Store (DKV) cluster on Google Cloud Platform (GCP) and GKE Autopilot.

---

## 1. GCP Authentication & Project Configuration

These commands configure your local terminal session to communicate with your GCP project and authorize access.

### Set active GCP Project
Sets your active working project context so all subsequent commands target the correct cloud resources:
```bash
gcloud config set project project-1227447b-a30c-4990-bb3
```

### Fetch GKE Cluster Credentials
Retrieves the cluster endpoint and security credentials, automatically configuring your local `kubectl` context to manage your GKE cluster:
```bash
gcloud container clusters get-credentials dkv-cluster --region us-central1
```

### Configure Docker Authentication for Artifact Registry
Registers GCP Artifact Registry credentials helper with Docker, allowing you to securely build and push container images to GCP:
```bash
gcloud auth configure-docker us-central1-docker.pkg.dev
```

---

## 2. Container Build & Registry Operations

These commands compile your Go binaries, package them into container images, and push them to Google Artifact Registry.

### Execute Container Build & Push Pipeline
Runs the custom automated shell script to compile all DKV binaries (`kvsrvd`, `kvraftd`, `shardctrlrd`), build their corresponding Docker images, authenticate with Artifact Registry, and push them:
```bash
./build_and_push_gcp.sh
```

---

## 3. Kubernetes Deployment & State Management

These commands manage the lifecycle of your deployments, StatefulSets, and namespaces.

### Apply/Update Namespace
Creates the dedicated `dkv` namespace in GKE where all DKV resources are isolated and managed:
```bash
kubectl apply -f DistributedKeyValueStore/deployments/kubernetes/namespace.yaml
```

### Deploy/Update All Cluster Services
Applies all manifests in the kubernetes directory, deploying the Metadata Store, Shard Controller, and both replicated Shard Groups in a single command:
```bash
kubectl apply -f DistributedKeyValueStore/deployments/kubernetes/
```

### Tear Down/Delete Cluster Services
Gracefully deletes all deployed resources (pods, services, statefulsets, claims) to stop all cloud computing costs when the database is inactive:
```bash
kubectl delete -f DistributedKeyValueStore/deployments/kubernetes/
```

---

## 4. Cluster Monitoring & Inspection

These commands let you inspect the health, scheduling, and logs of your running cluster.

### List Pods
Lists the status, readiness, and restarts of all pods in the `dkv` namespace:
```bash
kubectl get pods -n dkv
```

### Watch Pod Status Live
Monitors the pod status in real-time, displaying live updates as pods transition from pending, to container creating, and running:
```bash
kubectl get pods -n dkv -w
```

### Inspect Physical Pod Scheduling & Node IPs
Displays extended details for all pods, including their internal IP addresses and the exact GKE physical node (VM) they are scheduled on:
```bash
kubectl get pods -n dkv -o wide
```

### List Services
Lists all active services, showing their internal cluster IPs, port mappings, and any external load balancer IP provisions:
```bash
kubectl get svc -n dkv
```

### Check Pod Logs
Retrieves the last 40 lines of console output from a specific pod (or the first pod in a deployment) to inspect startup and execution logs:
```bash
# Get logs for a specific pod
kubectl logs -n dkv kvraft-1-0 --tail=40

# Get logs for a deployment
kubectl logs -n dkv deployment/shardctrlr --tail=40
```

---

## 5. Secure Networking & Port Forwarding

These commands bridge your private cloud network to your local host machine for testing and management.

### Forward Metadata Store Service
Opens a secure, encrypted tunnel from your local development machine (port `9000`) directly to the private GKE Metadata Store service, enabling local scripts (like `sample.py`) to connect:
```bash
kubectl port-forward -n dkv svc/metadata-store-service 9000:9000 --address 0.0.0.0
```

---

## 6. Network Diagnostics & In-Cluster Testing

These commands diagnose connectivity, DNS resolution, and execute test queries from within the cluster boundary.

### Test Internal DNS Resolution
Verifies that Kubernetes CoreDNS successfully creates and resolves internal FQDNs for StatefulSet replicas:
```bash
kubectl exec -it -n dkv dkv-client-test -- nslookup kvraft-1-0.kvraft-1-service
```

### Test Port Socket Connectivity
Verifies that network traffic can traverse the pod network and connect to the database service on its designated port:
```bash
kubectl exec -it -n dkv dkv-client-test -- nc -zv kvraft-1-0.kvraft-1-service 8000
```

### Run Direct CLI Reads/Writes in GKE
Launches a direct client command inside a GKE test container, connecting to the Metadata Store on port `9000` to execute database transactions:
```bash
# Write data
kubectl exec -it -n dkv dkv-client-test -- /bin/dkv-client --ctrler-addr metadata-store-service:9000 put mykey myval

# Read data
kubectl exec -it -n dkv dkv-client-test -- /bin/dkv-client --ctrler-addr metadata-store-service:9000 get mykey
```
