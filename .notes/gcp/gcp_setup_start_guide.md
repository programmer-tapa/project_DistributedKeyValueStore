# GCP Project Setup & GKE Cluster Initialization Guide

This document outlines the step-by-step commands required to set up your Google Cloud Platform (GCP) environment, configure your local command-line tools, and provision a Google Kubernetes Engine (GKE) Autopilot cluster to run the Distributed Key-Value Store (DKV) experiment.

---

## Step 1: Install the Prerequisites (On your local machine)
Ensure the following tools are installed on your local system:
1. **Google Cloud SDK (`gcloud` CLI)**: [Installation Guide](https://cloud.google.com/sdk/docs/install)
2. **Kubernetes CLI (`kubectl`)**: Can be installed via `gcloud` once the SDK is set up:
   ```bash
   gcloud components install kubectl
   ```
3. **Docker**: Ensure Docker is installed and running locally to build and push container images.

---

## Step 2: Authenticate and Create your GCP Project
Open your terminal and run the following commands to authenticate the CLI and initialize your project:
```bash
# 1. Log in via your browser
gcloud auth login

# 2. Create a new project (replace 'dkv-learning-lab' with a unique ID)
gcloud projects create dkv-learning-lab --name="DKV Learning Lab"

# 3. Set this project as your default active project
gcloud config set project dkv-learning-lab
```

> [!IMPORTANT]
> Go to the **GCP Web Console > Billing**, and ensure your new project is linked to your **$300 Free Trial Billing Account** so that costs are billed against your free credits.

---

## Step 3: Enable the Required APIs
Enable the necessary GCP services for running Kubernetes clusters, storing Docker containers, and managing compute resources:
```bash
gcloud services enable \
    container.googleapis.com \
    artifactregistry.googleapis.com \
    compute.googleapis.com
```

---

## Step 4: Create a Docker Registry (Artifact Registry)
Create a private Docker registry in GCP to host your DKV container images in your chosen region (e.g., `us-central1`):
```bash
# 1. Create the repository
gcloud artifacts repositories create dkv-repo \
    --repository-format=docker \
    --location=us-central1 \
    --description="DKV Docker Registry"

# 2. Configure Docker to authenticate with GCP registries automatically
gcloud auth configure-docker us-central1-docker.pkg.dev
```

---

## Step 5: Provision your GKE Autopilot Cluster
Spin up a **GKE Autopilot** cluster named `dkv-cluster` in the `us-central1` region. Autopilot automatically handles node management, auto-scaling, and security:
```bash
gcloud container clusters create-auto dkv-cluster \
    --region us-central1
```
*(This command will take about 4–6 minutes to complete as GCP configures the control plane across 3 availability zones.)*

---

## Step 6: Connect your Local `kubectl` to the Cluster
Once the cluster is created, fetch its access credentials so your local `kubectl` command-line tool can communicate with it:
```bash
gcloud container clusters get-credentials dkv-cluster \
    --region us-central1
```

To verify that you are connected and your cluster is healthy, run:
```bash
kubectl get nodes
```
You should see a list of GKE-managed virtual machines active and ready to accept your workloads.

---

## Next Steps
Now that your cloud environment is ready, you can proceed to:
1. **Build and push your Docker images** to your new Artifact Registry.
2. **Apply your Kubernetes manifests** (using `kubectl apply -f <manifest>.yaml`) to launch the Metadata Store, Shard Groups, and Shard Controllers.
