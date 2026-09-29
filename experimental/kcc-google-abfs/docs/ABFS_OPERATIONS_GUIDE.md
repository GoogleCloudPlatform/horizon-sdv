# ABFS Standalone Day 2 Operations Guide

This guide covers key operational tasks for managing and maintaining a standalone Android Build File System (ABFS) deployment on GKE, focusing on configuring and reconfiguring git-pushers to synchronize and seed new repositories or branches.

---

## 0. Re-authenticate with GKE (if needed)


```bash
export PROJECT_ID="YOUR_PROJECT_ID"
gcloud auth login
gcloud config set project ${PROJECT_ID}
gcloud auth application-default login
# Re-fetch your kubectl credentials
gcloud container clusters get-credentials abfs --region europe-west3 --project ${PROJECT_ID}
```


## 1. Reconfiguring Git-Pushers to Sync New Branches or Android Versions

The git-pushers (uploaders) fetch Git metadata and repository packs based on a central configuration. This configuration is declared in the Helm values and seeded into the ABFS metadata repository (`abfs-meta`) via a post-upgrade bootstrap Job.

Follow these steps to synchronize an additional Android release branch or version.

### Step 1.1: Modify values-sandbox.yaml

Add the desired branch and manifest file definition to the `pusher.branchFiles` list in your Helm values file.

Example: To add the `android-14.0.0_r2` branch alongside the existing branches:

```yaml
pusher:
  manifestProjectUrl: https://android.googlesource.com/platform/manifest
  branchFiles:
    - branch: main
      file: default.xml
    - branch: android-14.0.0_r1
      file: default.xml
    - branch: android-14.0.0_r2
      file: default.xml
```

### Step 1.2: Apply the Configuration via Helm

Run a Helm upgrade to update the pusher ConfigMap and trigger the post-upgrade bootstrap Job:

```bash
helm upgrade --install abfs ./rendered/standalone/chart/abfs \
  -f values-sandbox.yaml \
  -f values-local.yaml \
  -n abfs
```

### Step 1.3: Verify Bootstrap Job Execution

During the upgrade, Helm launches a new `abfs-pusher-config` Job pod. Verify that this job completes successfully:

```bash
kubectl get pods -n abfs -l job-name=abfs-pusher-config
```

Inspect the logs of the bootstrap container to ensure that the new config has been pushed and the server ref updated:

```bash
kubectl logs -n abfs job/abfs-pusher-config -c pusher-config
```

The logs should conclude with a successful `put-ref` output.

### Step 1.4: Monitor Uploader Logs

The uploader pods (`abfs-gerrit-uploader-<idx>`) poll the central configuration from the ABFS server. They should automatically pick up the new branch and begin fetching the new manifest:

```bash
kubectl logs -n abfs abfs-gerrit-uploader-0 --tail=100
```

Alternatively, you can track the active seeding progress using the monitor script:

```bash
python scripts/track-seeding.py
```

To force an immediate reload or ensure clean state, you can perform a rolling restart of the uploader StatefulSet:

```bash
kubectl rollout restart statefulset/abfs-gerrit-uploader -n abfs
kubectl rollout status statefulset/abfs-gerrit-uploader -n abfs
```

---

## 2. Adding a Custom/External Manifest Repository

To sync a completely different set of repositories (such as a custom platform integration, hardware vendor drivers, or a private Git repository), you can point the git-pusher to a custom manifest.

### Step 2.1: Update values-sandbox.yaml with Custom Manifest

Point `manifestProjectUrl` to your custom manifest Git repository and specify the branch/file:

```yaml
pusher:
  manifestProjectUrl: https://github.com/your-org/custom-manifest.git
  branchFiles:
    - branch: main
      file: default.xml
```

> [!NOTE]
> If the custom manifest repository or target repositories are private, ensure the uploader service account has the necessary IAM/SSH keys configured, or use an unauthenticated public HTTPS mirror.

### Step 2.2: Apply and Verify

Apply using Helm as described in Section 1.2, and monitor the uploader logs to ensure the new custom manifest projects are being discovered and synced.

---

## 3. Cost Optimization: Post-Seeding Scale-Down

After initial Spanner seeding of the AOSP manifest completes, the `abfs-gerrit-uploader` pods perform little work, and `abfs-server` only serves lightweight metadata queries because `abfs mount` caches file blobs locally on the client VM (`~/src` and `~/.abfs`). Cloud Spanner's processing load drops by >90%.

To reduce total infrastructure costs by **>70%** without deleting any seeded data in Spanner/GCS or interrupting your running client VM, execute this three-step surgical scale-down:

### Step 3.1: Apply the Post-Seeding Helm Overlay (`values-scaledown.yaml`)
Apply `values-scaledown.yaml` alongside your base values. This scales `abfs-gerrit-uploader` to **1 minimal replica** (requests: `4 CPU / 32Gi RAM`, limits: `16 CPU / 128Gi RAM`) to continuously fetch incremental remote branch updates without OOMing, disables the uploader PodDisruptionBudget (`pdb.enabled: false`) so GKE node drains are never blocked, and resizes `abfs-server` to a lightweight build profile (requests: `4 CPU / 16Gi RAM`, limits: `16 CPU / 64Gi RAM`):

```bash
helm upgrade --install abfs ./rendered/standalone/chart/abfs \
  -f values-sandbox.yaml \
  -f values-local.yaml \
  -f values-scaledown.yaml \
  -n abfs
```

### Step 3.2: Scale Down Cloud Spanner Processing Units
Reduce Spanner Processing Units from `4000` (4 nodes) down to `200` (0.2 nodes), cutting Spanner billing by 95%:

```bash
# Via kubectl patch (if managing Spanner via KCC):
kubectl patch spannerinstance abfs -n abfs --type='merge' -p '{"spec":{"processingUnits":200}}'

# Or via gcloud CLI directly:
gcloud spanner instances update abfs \
  --processing-units=200 \
  --project=YOUR_PROJECT_ID
```

### Step 3.3: Scale the GKE Data Node Pool Down to 1 Node
Because all ABFS pods (`abfs-server-0` and `abfs-gerrit-uploader-0`) require `nodeSelector: cloud.google.com/gke-nodepool: abfs-data` and the `abfs-runtime` service account for licensing and IAM permissions, they **live on the `abfs-data` node pool**.

During active seeding, `abfs-data` ran 4 nodes (1 per heavy uploader + 1 for the server). With both pods scaled to lightweight `2 CPU / 8Gi RAM` profiles, they comfortably share a **single node**. Resize `abfs-data` from 4 nodes down to 1:

```bash
gcloud container clusters resize abfs \
  --node-pool=abfs-data \
  --num-nodes=1 \
  --region=europe-west3 \
  --quiet
```

### Step 3.4: Reverting (Scaling Back Up to Seed New Branches)
When you need to ingest or seed new Android release branches:
1. Scale Spanner back to 4000 Processing Units (`--processing-units=4000`).
2. Scale `abfs-data` node pool back up (`--num-nodes=4`).
3. Re-run `helm upgrade` **without** `-f values-scaledown.yaml`.
