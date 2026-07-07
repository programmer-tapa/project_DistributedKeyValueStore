# GCP Free Trial & Cost Management Guide: DKV Experimentation

This document outlines how to safely run the Distributed Key-Value Store (DKV) deployment experiment on Google Cloud Platform (GCP) using your **$300 Free Trial Credit** without incurring any real-world personal costs.

---

## 1. Google Cloud Free Trial Policies

*   **$300 Credit**: You receive $300 in credits valid for 90 days. All resource usage (compute, storage, networking) is billed against this credit.
*   **No Auto-Charge Policy**: Google Cloud has a strict policy: **they will never automatically charge your credit card** when your trial ends or your credits run out. You must explicitly click "Upgrade" in the billing console to transition to a paid account.

---

## 2. The Math: Short-Term Experimentation Costs

Cloud resources are billed **by the second** (or by the hour). You do not need to keep the cluster running 24/7. 

If a GKE cluster costs ~$150/month, that breaks down to:
*   **Hourly cost**: ~$0.20 per hour
*   **Daily cost**: ~$5.00 per day
*   **Weekly cost**: ~$35.00 per week

If you only spin up the cluster when you are actively working, test your DKV cluster for **3 hours**, and then delete it, it will only cost you **~$0.60** of your $300 credit! You could repeat this experiment hundreds of times.

---

## 3. Best Practices to Keep Costs at $0

To ensure you don't accidentally drain your $300 credit by leaving resources running in the background, follow these rules:

### Rule 1: Always Tear Down Your Infrastructure When Done
When you finish a study session, delete your GKE cluster or GCE VMs. This stops the billing meter instantly.

*   **To delete your GKE cluster**:
    ```bash
    gcloud container clusters delete dkv-cluster --region us-central1
    ```
*   **To delete your GCE VMs (if you chose GCE)**:
    ```bash
    gcloud compute instances delete dkv-node-0 dkv-node-1 dkv-node-2 --zone us-central1-a
    ```

### Rule 2: Use GKE Autopilot
Autopilot is perfect for learning because:
1.  It automatically provisions the smallest possible nodes for your pods.
2.  If you shut down your pods or scale them to 0, your compute costs drop to virtually $0.
3.  Google waives the GKE cluster management fee ($73/month) for the first cluster in your billing account.

### Rule 3: Clean up Persistent Disks (PDs)
When you delete a GKE cluster, the **Persistent Volume Claims (PVCs)** and their underlying GCP Persistent Disks might sometimes remain so that you don't lose data. Since SSD disks cost money even when the VM is off, make sure to delete them if you are completely finished:
```bash
# List any orphaned disks
gcloud compute disks list

# Delete a disk you no longer need
gcloud compute disks delete <disk-name> --zone <zone>
```

### Rule 4: Use Infrastructure as Code (Terraform)
Since you will be spinning the cluster up and tearing it down frequently to save money, **do not provision things manually in the GCP Web Console**. 
Instead, write a simple **Terraform** script. This allows you to:
*   Spin up the entire VPC, GKE cluster, and firewall rules in 5 minutes with: `terraform apply`
*   Safely destroy everything and stop all costs in 3 minutes with: `terraform destroy`

---

## 4. Suggested Study Workflow

1.  **Phase 1 (Local Prep)**: Make sure your Go code compiles, your Dockerfiles work, and you can run your DKV cluster locally using Docker Compose. Get everything ready *before* touching GCP.
2.  **Phase 2 (Deploy & Learn)**: Spin up the GKE cluster, deploy your manifests, test the shard controller, and observe the Raft leader election.
3.  **Phase 3 (Cleanup)**: Immediately run `terraform destroy` or the `gcloud delete` commands to tear everything down at the end of your study session.
4.  **Phase 4 (Chaos Engineering)**: Spin it up again, write a script to randomly delete a pod (simulate a zonal failure), and watch GKE reschedule it and reattach the SSD without data loss! Then tear it down again.
