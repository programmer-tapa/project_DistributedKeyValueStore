# GKE Autopilot VM Visibility: Why VMs don't show in Compute Engine

If you run `kubectl get nodes` and see active, healthy nodes, but your **Google Compute Engine > VM Instances** console is completely empty, **this is by design**. 

It represents the core architectural and security boundary of **GKE Autopilot** (often referred to as "Serverless Kubernetes").

---

## 1. The GKE Autopilot Tenant Project Architecture

Unlike GKE Standard or raw GCE, where virtual machines run directly inside your own GCP project, GKE Autopilot utilizes a **Google-managed tenant project**:

*   **Google-Managed Project**: Google provisions, boots, secures, and maintains the actual Linux VM instances in a separate, Google-owned project.
*   **Network Peering**: This Google-managed project is peered securely with your custom Virtual Private Cloud (VPC) network, allowing GKE containers to talk to your private resources.
*   **Invisible to Compute Engine**: Because the VMs reside in Google's managed project, they **do not appear** on your project's **Compute Engine** dashboard, nor will they show up if you run `gcloud compute instances list` in your terminal.
*   **No Direct Access**: You cannot SSH into the nodes, manually delete them, or patch their operating systems. Google takes 100% operational ownership of the VM lifecycle.

---

## 2. Where can you see your nodes in the GCP Web Console?

While the VMs are hidden from Compute Engine, you can easily view them through the **Kubernetes Engine** console:

1.  Go to **Google Cloud Console**.
2.  Navigate to **Kubernetes Engine > Clusters**.
3.  Click on the name of your cluster (**`dkv-cluster`**).
4.  Select the **Nodes** tab at the top of the details page.
5.  This will display the active GKE Autopilot nodes, their zones, CPU/Memory utilization, and the pods currently scheduled on them.

---

## 3. Quick Comparison: GKE Autopilot vs. Standard VM Visibility

| Feature | GKE Autopilot | GKE Standard & GCE |
| :--- | :--- | :--- |
| **Where do VMs run?** | In a **Google-managed** tenant project. | In **your** GCP project. |
| **Visible in Compute Engine?** | **No** (keeps your VM console clean). | **Yes** (clutters your VM console). |
| **Visible in `kubectl get nodes`?** | **Yes** (standard Kubernetes API). | **Yes** (standard Kubernetes API). |
| **Can you SSH into the nodes?** | **No** (secured and locked down by Google). | **Yes** (you have root access to the VMs). |
| **Who patches the OS?** | **Google** (automatically, zero downtime). | **You** (manual updates and reboots). |
| **What do you pay for?** | Only the **CPU/Memory requested by running pods**. | The **full capacity of the running VMs** (even if idle). |
