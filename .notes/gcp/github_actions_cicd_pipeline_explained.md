# CI/CD Pipeline: Automation, Security, and Deployment Flow

This document details where the automated CI/CD pipeline runs, how the workflow is triggered, and the end-to-end security and deployment steps that transition code from your local Git repository to your GKE Autopilot cluster.

---

## 1. Where Does the Pipeline Run?

The CI/CD pipeline defined in **`deploy.yml`** does not run on your local machine or your GCP virtual machines. It is executed entirely by **GitHub Actions** in the cloud:

*   **GitHub-Hosted Runners**: Whenever the pipeline is triggered, GitHub dynamically provisions clean, isolated virtual machines (running Ubuntu Linux, as defined by `runs-on: ubuntu-latest`) to execute the jobs.
*   **Triggers**: The pipeline is automatically triggered by two Git events:
    1.  **Pull Requests**: Any Pull Request targeting the `main` branch (triggers the verification/testing phase only).
    2.  **Pushes / Merges**: Any direct push or merged Pull Request into the `main` branch (triggers both the testing and automated cloud deployment phases).

---

## 2. The End-to-End CI/CD Workflow

Here is the step-by-step lifecycle of your code during a pipeline run:

### Phase 1: Continuous Integration (CI) - Verification & Testing
The first job, `test`, runs on every trigger to guarantee that the new code does not introduce regressions or distributed state bugs:
1.  **Code Checkout**: The runner pulls your code repository.
2.  **Go Environment Setup**: Configures Go 1.22 with dependency caching enabled to optimize build times.
3.  **Linter & Compiler Check**: Runs `go fmt` and `go vet` to catch formatting issues, structural anomalies, or static analysis bugs.
4.  **Consensus Testing**: Runs all DKV unit and integration tests with Go's **race detector enabled** (`go test -race`).
    *   *If any test fails or a concurrent read/write data race is detected, the pipeline halts immediately. Broken code is blocked from ever reaching your cloud environment.*

### Phase 2: Continuous Deployment (CD) - Build & GKE Rollout
If the CI phase passes successfully, and the event is a push or merge directly to the `main` branch, the `deploy` job starts:

1.  **Secure Authentication (Workload Identity Federation)**:
    Rather than storing long-lived, highly sensitive GCP Service Account JSON keys in GitHub Secrets, the runner uses **OIDC (OpenID Connect)**. It exchanges a short-lived GitHub token for a temporary, highly restricted GCP access token.
2.  **Docker Registry Login**:
    Authenticates the runner with your private Google Artifact Registry.
3.  **Docker Buildx (BuildKit) Compilation**:
    The runner utilizes BuildKit to compile your three Go binaries (`kvsrvd`, `kvraftd`, `shardctrlrd`), packages them into container images, and pushes them to Artifact Registry. 
    *   *GitHub Actions Cache (`cache-from/to: type=gha`) is utilized, ensuring unchanged layers are retrieved instantly, reducing build times from minutes to seconds.*
4.  **Connect to GKE**:
    Authenticates the runner with your GKE Autopilot cluster using secure credentials.
5.  **Apply Manifests**:
    Executes `kubectl apply` to apply any changes made to your Kubernetes YAML manifests.
6.  **Zero-Downtime Rolling Restarts**:
    Executes `kubectl rollout restart` on all workloads. GKE gracefully replaces the database pods one by one. Because only one replica restarts at any given time, the remaining replicas maintain **Raft write quorum**, ensuring your database remains 100% online and serving clients throughout the entire update!
