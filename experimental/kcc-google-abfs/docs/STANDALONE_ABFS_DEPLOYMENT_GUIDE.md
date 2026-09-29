# Standalone ABFS GKE Standard & KCC Deployment Guide
This guide provides a comprehensive, production-grade, step-by-step blueprint to deploy a standalone **Android Build File System (ABFS)** data plane on a clean Google Cloud Platform (GCP) project.

Our architecture leverages **GKE Standard** on the **Rapid Channel** combined with **Config Connector (KCC)** to provision and manage all supporting services (Cloud Spanner, Google Cloud Storage, Secret Manager, Private DNS, and Firewall rules) declaratively. It utilizes the native **COS-integrated CASFS kernel module (0.1.14)** to eliminate manual driver compilations, simplify node pool upgrades, and deliver sandbox-to-production scalability.

---

## 📖 Table of Contents
1. [Architecture Overview](#1-architecture-overview)
2. [Pre-Deployment Checklist & Quotas](#2-pre-deployment-checklist--quotas)
3. [Workspace Navigation](#3-workspace-navigation)
4. [Parameterizing your GCP Project ID](#4-parameterizing-your-gcp-project-id)
5. [VPC Networking & NAT Gateway](#5-vpc-networking--nat-gateway)
6. [GKE Cluster Provisioning](#6-gke-cluster-provisioning)
7. [Config Connector (KCC) Operator Setup](#7-config-connector-kcc-operator-setup)
8. [Core Infrastructure Declarative Deployment](#8-core-infrastructure-declarative-deployment)
9. [Two-Phase SA and License Provisioning](#9-two-phase-sa-and-license-provisioning)
10. [Dedicated Node Pool Creation](#10-dedicated-node-pool-creation)
11. [Deploying ABFS Workloads (Helm)](#11-deploying-abfs-workloads-helm)
12. [Live Validation & Verification](#12-live-validation--verification)

---

## 1. Architecture Overview

To provide sandbox isolation and maximum security, the standalone ABFS deployment segregates administrative control planes from data workloads. 

```mermaid
graph TD
    subgraph Client GCP Project
        VPC[VPC Network / Subnet] --> GKE[GKE Standard Cluster - Rapid Channel >= 1.36.0]
        GKE --> NP_SYS[Default Node Pool - System & KCC]
        GKE --> NP_DATA["abfs-data Node Pool - GCE Metadata Mode"]
        
        NP_DATA -->|Native Module| CASFS["CASFS Kernel Module 0.1.14 (Pre-installed in COS)"]
        NP_DATA -->|Service Account| RSA[Licensed Runtime SA]
        NP_DATA -->|Instance Metadata| LIC[abfs-license.b64]
        
        KCC[Config Connector] -->|Manages| SP[(Cloud Spanner)]
        KCC -->|Manages| GCS[(GCS Bucket)]
        KCC -->|Manages| SEC[(Secret Manager)]
        
        ABFS_SRV[abfs-server Pod] -->|Uses| CASFS
        ABFS_SRV -->|Identity| RSA
        ABFS_SRV -->|License Check| LIC
        ABFS_SRV -->|Reads/Writes| SP
        ABFS_SRV -->|Reads/Writes| GCS
    end
```

### Key Pillars
*   **Zero-Compile Node Management (`casfs.provider=image`)**: Bypasses the legacy ubuntu-based out-of-tree (OOT) installer. Nodes boot instantly on standard Container-Optimized OS (COS) images containing pre-signed modules, allowing transparent cluster upgrades.
*   **Metadata-Only Licensed Identity**: Workload Identity is bypassed (`--workload-metadata=GCE_METADATA`) on the `abfs-data` pool. This forces pods to use the node GCE VM identity token directly, ensuring robust, Google-signed VM identity entitlement check.
*   **Completely Declarative Control Plane**: KCC is utilized to spin up and keep GCP infrastructure in sync using native Kubernetes manifests.

---

## 2. Pre-Deployment Checklist & Quotas

Before executing the deployment, ensure your GCP target project is configured with billing and has sufficient regional quotas.

### A. Quota Increase Requests (QIR)
Go to **IAM & Admin > Quotas** in the GCP Console and ensure your target region (e.g., `europe-west3`) has quotas requested and approved:

| Quota Metric / Filter ID | Target Level (Sandbox) | Target Level (Production) | Description |
| :--- | :--- | :--- | :--- |
| `compute.googleapis.com/cpus_all_regions` | **8** | **64** | Global vCPUs across all regions. |
| `HDB-TOTAL-GB-per-project-region` | **500 GB** | **15,000 GB** | Total Hyperdisk Balanced storage capacity in target region. |
| Regional CPU limit (e.g., `N2_CPUS`) | **8** | **64** | Regional vCPUs for chosen machine family. |

---
 
## 3. Workspace Navigation

Before executing any commands, change your working directory to the consolidated module subdirectory. This guarantees that all relative paths for declarative YAML templates and Helm configurations (such as `rendered/`) resolve flawlessly:

```bash
cd experimental/kcc-google-abfs/
```

---

## 4. Configuration Strategy & Project Setup

This repository uses a layered configuration approach to keep code clean while allowing flexible deployments:
*   **`values.yaml` (Base):** The default Helm chart configuration with standard defaults.
*   **`values-sandbox.yaml` (Environment Template):** A committed template specifically tuned to utilize the high-performance sandbox nodes. It uses `YOUR_PROJECT_ID` placeholders so it remains reusable for anyone.
*   **`values-local.yaml` (Your Overrides):** A local file you create (ignored by Git) to inject your specific GCP Project ID. Helm safely merges this over the templates at deployment time.

Before deploying, you must substitute the template placeholders with your actual GCP Project ID.

1. **For the Data Plane (Helm Chart)**:

Create a `values-local.yaml` file (it is ignored by git) in the root of this module:

```yaml
projectId: YOUR_PROJECT_ID
bucket: YOUR_PROJECT_ID-abfs-blobs
```

2. **For the Infrastructure (Config Connector)**:

Edit the Kustomize overlay to set your project ID:

```bash
# Open rendered/standalone/infra/overlays/sandbox/kustomization.yaml
# Under configMapGenerator, change the last line PROJECT_ID=YOUR_PROJECT_ID to your actual project ID.
```

3. Connect to your project

```bash
export PROJECT_ID="YOUR_PROJECT_ID"

# Generate the Kustomize overlay configuration file (this file is gitignored)
echo "PROJECT_ID=${PROJECT_ID}" > rendered/standalone/infra/overlays/sandbox/setup/project.env
echo "KCC_SERVICE_ACCOUNT=cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" >> rendered/standalone/infra/overlays/sandbox/setup/project.env
echo "CLOUDBUILD_WI=serviceAccount:${PROJECT_ID}.svc.id.goog[abfs/abfs-cloudbuild]" >> rendered/standalone/infra/overlays/sandbox/setup/project.env

gcloud auth login
gcloud config set project ${PROJECT_ID}

# if not installed, install kubectl and gke-gcloud-auth-plugin
gcloud components install kubectl
gcloud components install gke-gcloud-auth-plugin
``` 

---

## 5. VPC Networking & NAT Gateway
 
Private nodes are recommended. Create a dedicated private VPC with custom subnets, a Cloud Router, and NAT for outbound internet access (required for image downloads and GKE registration).
 
```bash
# 1. Create a custom-mode VPC
gcloud compute networks create vpc-abfs --subnet-mode=custom
 
# 2. Create a private subnet with Private Google Access enabled (essential for KCC and GKE nodes)
gcloud compute networks subnets create subnet-abfs \
  --network=vpc-abfs \
  --region=europe-west3 \
  --range=10.0.0.0/20 \
  --enable-private-ip-google-access
 
# 3. Provision Cloud Router for egress
gcloud compute routers create router-abfs \
  --network=vpc-abfs \
  --region=europe-west3
 
# 4. Provision Cloud NAT Gateway attached to the router
gcloud compute routers nats create nat-abfs \
  --router=router-abfs \
  --region=europe-west3 \
 --auto-allocate-nat-external-ips \
  --nat-all-subnet-ip-ranges
```
 
---
 
## 6. GKE Cluster Provisioning
 
Deploy a GKE Standard cluster using the **Rapid Channel** to guarantee that the nodes boot into GKE version `1.36.0` or newer, ensuring Native CASFS module compatibility.
 
```bash
# Note: You can omit --cluster-version to automatically select the default Rapid Channel release,
# or list active Rapid channel releases via: gcloud container get-server-config --region=europe-west3
gcloud container clusters create abfs \
  --region=europe-west3 \
  --node-locations=europe-west3-a \
  --release-channel=rapid \
  --cluster-version=1.36.0-gke.4681000 \
  --network=vpc-abfs \
  --subnetwork=subnet-abfs \
  --enable-private-nodes \
  --enable-ip-alias \
  --master-ipv4-cidr=172.16.0.0/28 \
  --machine-type=n4-standard-16 \
  --num-nodes=1 \
  --workload-pool=${PROJECT_ID}.svc.id.goog \
  --enable-shielded-nodes \
  --shielded-secure-boot \
  --shielded-integrity-monitoring \
  --no-enable-master-authorized-networks

# Fetch cluster credentials for kubectl
gcloud container clusters get-credentials abfs \
  --region=europe-west3 \
  --project=${PROJECT_ID}
```

---

## 7. Config Connector (KCC) Operator Setup

Enable the Config Connector addon in your cluster and link its controller manager to a privileged GCP Service Account (SA, short for "Service Account") using Workload Identity.

Enable vertical autoscaling:
```bash
gcloud container clusters update abfs \
  --region=europe-west3 \
  --project=${PROJECT_ID} \
  --enable-vertical-pod-autoscaling
``` 

```bash
# 1. Enable KCC addon on the cluster
gcloud container clusters update abfs \
  --region=europe-west3 \
  --update-addons ConfigConnector=ENABLED

# 2. Set environment variables
export KCC_SA_NAME="cnrm-system" # The Google Cloud Service Account for Config Connector
export KCC_SA_EMAIL="${KCC_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

# 3. Create the GCP Service Account for KCC administration (if it doesn't already exist)
gcloud iam service-accounts create ${KCC_SA_NAME} --project=${PROJECT_ID} || true

# 4. Grant project resource management permissions (Owner) to the KCC SA
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${KCC_SA_EMAIL}" \
  --role="roles/owner"

# 5. Grant Workload Identity Impersonation (Cluster Mode)
gcloud iam service-accounts add-iam-policy-binding \
  ${KCC_SA_EMAIL} \
  --role="roles/iam.workloadIdentityUser" \
  --member="serviceAccount:${PROJECT_ID}.svc.id.goog[cnrm-system/cnrm-controller-manager]" \
  --project="${PROJECT_ID}"

# 6. Annotate the Kubernetes Service Account

# Wait for the KCC controller manager service account to be created by the GKE Addon manager
kubectl wait --for=jsonpath='{.status.phase}'=Active namespace/cnrm-system --timeout=120s

# Only annotate after the namespace and service account are ready.
kubectl annotate serviceaccount \
  --namespace cnrm-system \
  cnrm-controller-manager \
  iam.gke.io/gcp-service-account=${KCC_SA_EMAIL} \
  --overwrite

# 7. Force token refresh by restarting the controller pod
kubectl delete pod cnrm-controller-manager-0 -n cnrm-system
```

Apply the core operator ConfigConnector configuration and dedicated workload namespace (patched dynamically via Kustomize):

```bash
kubectl apply -k rendered/standalone/infra/overlays/sandbox/setup
```

---

## 8. Core Infrastructure Declarative Deployment

Under KCC, apply your declarative resource manifest bundle representing Spanner, GCS Storage, Secret Manager, DNS, and Firewall configurations. 

> [!NOTE]
> For sandbox-grade environments, ensure your SpannerInstance manifest specifies `processingUnits: 100` rather than advanced autoscaling configurations to bypass schema restrictions and reduce costs.

Apply the infrastructure layer:
```bash
# Apply using Kustomize (automatically handles project ID injection)
kubectl apply -k rendered/standalone/infra/overlays/sandbox

# Create custome storage class
kubectl apply -f rendered/standalone/infra/base/setup/storageclass-hyperdisk-balanced.yaml
```

Verify KCC reconciliation progress:

```bash
kubectl get gcp -n abfs
```
*Wait until all resources (Spanner Instance, Database, Bucket, SAs) show `READY: True`.*



---

## 9. Two-Phase SA and License Provisioning

ABFS uses a strict Google-signed VM Identity token check. Follow this two-phase flow to generate the Licensed Service Account and retrieve your license.

### Phase A: Retrieve SA Unique ID

1. The Service Account is created when you apply the Kustomize infrastructure overlay:

```bash
kubectl apply -k rendered/standalone/infra/overlays/sandbox
```
2. Retrieve the Unique ID (OAuth2 Client ID) of the newly created `abfs-runtime` service account:

```bash
gcloud iam service-accounts describe abfs-runtime@${PROJECT_ID}.iam.gserviceaccount.com \
  --project=${PROJECT_ID} \
  --format="value(uniqueId)"
```

3. Submit this **SA Email** and **Unique ID**  as well as your **Project ID** and **Project Number** to the Google license team to obtain your `abfs-license.json`.

### Phase B: Pre-stage the License

1. Once you receive `abfs-license.json`, Base64-encode it (without line wraps):

```bash
base64 -w0 abfs-license.json > abfs-license.b64
```

---

## 10. Dedicated Node Pool Creation

Provision the dedicated `abfs-data` node pool. **This pool bypasses Workload Identity** (`--workload-metadata=GCE_METADATA`) to expose the GCE metadata server directly to the pods, loaded with the base64-encoded license string.

> [!TIP]
> **High Performance Seeding Option:** To use the legacy Terraform maximum throughput specs (Server: 64 Cores/512Gi, Uploader: 48 Cores/192Gi), change the `--machine-type` below to `n2-highmem-80` to ensure the pod requests fit within Kubernetes's allocatable node overhead constraints. Then uncomment the corresponding resource blocks in `values-sandbox.yaml`.

```bash
gcloud container node-pools create abfs-data \
  --cluster=abfs \
  --region=europe-west3 \
  --node-locations=europe-west3-a \
  --project=${PROJECT_ID} \
  --service-account=abfs-runtime@${PROJECT_ID}.iam.gserviceaccount.com \
  --workload-metadata=GCE_METADATA \
  --scopes=cloud-platform \
  --metadata-from-file abfs-license=./abfs-license.b64 \
  --metadata disable-legacy-endpoints=true \
  --machine-type=n4-highmem-32 \
  --disk-type=hyperdisk-balanced \
  --disk-size=100 \
  --enable-autoscaling \
  --min-nodes=0 \
  --max-nodes=4 \
  --shielded-secure-boot \
  --shielded-integrity-monitoring \
  --node-taints=abfs.dev/dedicated=true:NoSchedule \
  --image-type=COS_CONTAINERD
```

---

## 11. Deploying ABFS Workloads (Helm)

Deploy the native **COS-integrated CASFS kernel module loader DaemonSet** to automatically load the pre-compiled `casfs` driver into kernel memory as nodes auto-scale:
```bash
kubectl apply -f manifests/casfs-image-loader.yaml
```

The repository contains a high-performance `values-sandbox.yaml` file tuned to utilize the full capacity of your dedicated physical nodes. 

Since you already created your `values-local.yaml` override file in **Step 4**, simply deploy the Helm chart by layering both configuration files together:

Deploy the Helm chart, passing both the sandbox configuration and your local overrides:

```bash
helm upgrade --install abfs ./rendered/standalone/chart/abfs \
  -f values-sandbox.yaml \
  -f values-local.yaml \
  -n abfs

# If helm is not installed, install it
curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
chmod 700 get_helm.sh
./get_helm.sh
```

---

## 12. Live Validation & Verification

1. Verify that the ABFS Server and Uploader pods schedule on the dedicated node pool and enter `Running` state:

```bash
# get pod list
kubectl get pods -n abfs -o wide

# optionally get a more interactive view with k9s
k9s -n abfs
```

2. Verify the server logs. You should see successful connection to Cloud Spanner and verification of the GCE VM licensed identity token:

```bash
kubectl logs -l app.kubernetes.io/name=abfs-server -n abfs
```

3. Verify that the FUSE mount directory inside the container is successfully mounted and can read/write data:

```bash
# Run a check on the uploader pod to ensure casfs is mounted
kubectl exec -it abfs-gerrit-uploader-0 -n abfs -- df -h | grep casfs
```

4. Track the seeding phase:

```bash
# inside horizon-sdv/experimental/kcc-google-abfs
python scripts/track-seeding.py
``` 
