<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Android Build Filesystem (ABFS) Integration in Horizon SDV Platform

## Table of contents
- [Introduction](#overview)
- [Architecture](#architecture)
  - [ABFS components](#components)
  - [Horizon modules](#modules)
  - [How the modules interact](#interaction)
- [Prerequisites](#prerequisites)
- [Procedures](#procedures)
  - [Enable the ABFS feature](#enable)
  - [Import the ABFS license](#license)
  - [Seed Spanner with a new AAOS revision](#seed)
  - [Prepare the Docker image template](#image)
  - [Perform an AAOS build with ABFS](#build)
  - [Migrate an existing Spanner database to the new ABFS](#migrate)
- [Day-2 operations](#day2)
  - [Spanner backup schedules](#backup)
  - [Disable ABFS](#disable)
  - [Update the ABFS version](#update)
- [Known issues](#knownissues)
- [References](#references)

## Introduction <a name="overview"></a>

This document describes the integration of Google's Android Build Filesystem (ABFS) **0.1.14** into the Horizon SDV platform. ABFS serves the Android source tree and build caches from a central server, so build pods mount the sources instead of running `repo sync`, and reuse cached build outputs (Soong/ninja) across builds.

> [!IMPORTANT]
> **Early Access Program (EAP)**: ABFS is available under Google's Early Access Program and is subject to its restrictions and requirements.
>
> **Licensed feature**: the ABFS server and uploaders only start with a Google-issued license. The license is bound to one GCP service account (its numeric UniqueID) and to the GCP project number.
>
> **Contact Google**: for ABFS itself (features, limitations, versions, licenses) contact your Google representative.

Since ABFS 0.1.14 the whole ABFS infrastructure is reconciled by Argo CD through three Horizon **modules**, enabled from the Developer Portal. The Jenkins jobs that previously created the ABFS Server and Uploader VMs with Terraform (ABFS 0.0.33) have been removed. Jenkins keeps only two optional roles: building the ABFS-enabled builder image and running ABFS builds; both are also available as Argo Workflows from the portal.

## Architecture <a name="architecture"></a>

### ABFS components <a name="components"></a>

| Component | Where it runs | Role |
|-----------|---------------|------|
| **ABFS server** (`abfs-server`, gRPC `:50051`) | Deployment in namespace `abfs`, node pool `abfs-data-server-node-pool` | Serves tree objects and blobs to clients, stores metadata in Spanner and blobs in GCS. Reads the license from GCE instance metadata key `abfs-license`. |
| **Uploaders / git-pushers** (`abfs-gerrit-uploader-0..2`) | StatefulSet in `abfs`, node pool `abfs-data-uploader-node-pool`, 1 Ti PVC each | Fetch the repositories listed by an Android manifest from `android.googlesource.com` and push them into ABFS ("seeding"). Same license check as the server. |
| **Pusher config** (`abfs-pusher-config` Job, ABFS ref `config` in project `abfs-meta`) | Job in `abfs` | Publishes which manifest URL, branch and file the uploaders seed. Derived from the active row of ConfigMap `horizon/abfs-uploader-config`. |
| **Cloud Spanner** (instance `abfs`, database `abfs`, `regional-<region>`) | GCP | ABFS metadata (tables Objects, Links, Chunks, ChunkTouches, Maps, Projects, Refs). |
| **GCS bucket** `<projectID>-abfs` | GCP | ABFS blob storage. |
| **casfs kernel module** | Every node of the ABFS pools (installer DaemonSets) | Content-addressed filesystem used by `abfs mount` and by `abfs cacheman`. Built per node for the running kernel, which is why the pools are Ubuntu with Secure Boot off. |
| **ABFS client** (`abfs` CLI, `cacheman`) | Inside each build pod, node pool `abfs-build-node-pool-ubuntu` | `abfs init / config / cacheman run / mount -b <revision>`; the build runs on the mount, ninja asks `cacheman` for cache hits. |

### Horizon modules <a name="modules"></a>

ABFS is split into three modules plus the Android workloads. Each module is an Argo CD app-of-apps created by Module Manager; the catalog entry is `gitops/apps/module-manager/templates/module-catalog.yaml`.

| Module (catalog name) | Chart | Owns | Child Argo CD Applications | Dependencies |
|-----------------------|-------|------|----------------------------|--------------|
| **abfs-data** | `gitops/modules/abfs-data-module` | Everything persistent: license service account `abfs-license@<project>.iam.gserviceaccount.com`, Secret `horizon/abfs-license`, Spanner instance and database `abfs`, GCS bucket `<projectID>-abfs`, uploader manifest table `horizon/abfs-uploader-config`, license / uploader-config / Spanner-backup workflows | `mod-abfs-data-sa`, `mod-abfs-data-license-workflows`, `mod-abfs-data-uploader-config`, `mod-abfs-data-spanner-workflows` | none; `autoDisableWhenUnused: false` |
| **abfs-service-module** | `gitops/modules/abfs-service-module` | The runtime data plane: server and uploader node pools (KCC `ContainerNodePool`), license GCE metadata, `abfs-server`, uploaders, pusher-config Job, casfs installer for those pools, IAM project roles for the license SA, NetworkPolicies, PDBs, VPAs, `abfs-smoke-test` workflow | `mod-abfs-service-module-node-pool`, `mod-abfs-service-module-data-plane`, `mod-abfs-service-module-argo-workflows` | needs **abfs-data** enabled and the license imported (PreSync validator); not a catalog dependency |
| **abfs-client-ubuntu** | `gitops/modules/abfs-client-module-ubuntu` | Build node pool `abfs-build-node-pool-ubuntu` (c2d-highcpu-112, 0–20 nodes) and the casfs installer DaemonSet for it | `mod-abfs-client-ubuntu-node-pool` | hard: **workloads-android**, **abfs-service-module** |
| **workloads-android** (with **workloads-common**) | `gitops/modules/workloads-android` | Argo WorkflowTemplates `aaos-builder-abfs` (the ABFS build) and `abfs-builder-runtime-image` (the ABFS builder image), their webhook Sensors, pipeline scripts | `workloads-android-aaos-builder-abfs`, `workloads-android-abfs-builder-runtime-image` sources | hard: workloads-common |

`abfs-client-module-cos` (COS nodes with casfs shipped in the image) exists in the repository but is not registered in the catalog: it needs GKE ≥ 1.36 with COS 129 LTS carrying casfs 0.1.14 and was not tested.

Module READMEs: [abfs-data](../../../gitops/modules/abfs-data-module/README.md), [abfs-service-module](../../../gitops/modules/abfs-service-module/README.md), [abfs-client-ubuntu](../../../gitops/modules/abfs-client-module-ubuntu/README.md).

### How the modules interact <a name="interaction"></a>

```
abfs-data ──Secret horizon/abfs-license──────────────▶ abfs-service-module (PreSync validator; GCE metadata on both data pools)
abfs-data ──SA abfs-license (UniqueID = license)─────▶ abfs-service-module (node SA of the data pools, IAM roles)
abfs-data ──ConfigMap horizon/abfs-uploader-config──▶ abfs-service-module (pusher-config Job, uploader init container)
abfs-data ──Spanner abfs / GCS <project>-abfs───────▶ abfs-service-module (server connection settings only)
abfs-service-module ──Service abfs-server.abfs.svc.cluster.local:50051──▶ build pods (abfs-client-ubuntu pool)
workloads-android ──WorkflowTemplate aaos-builder-abfs──▶ pod on abfs-build-node-pool-ubuntu (taint/label android-abfs-build)
```

- **License**: `abfs-data` stores the license in the Secret; nothing is committed to Git. `abfs-service-module` refuses to sync its data plane while the Secret is missing or empty (`abfs-license-validate` PreSync hook), and stamps the license into GCE instance metadata `abfs-license` of the server and uploader pools (a PostSync Job in `mod-abfs-service-module-node-pool`; status in ConfigMap `abfs/abfs-license-metadata-status`). ABFS 0.1.14 reads the license from GCE metadata and requires a Compute Engine identity, so both data pools run with the license SA as node service account (`GCE_METADATA`), not Workload Identity.
- **Seeding configuration**: the uploader manifest table lives in `abfs-data` so it survives a disable of the service. Uploader pod `-0` reads the active row at start-up, re-runs a copy of the pusher-config Job which writes the ABFS ref `config` only when the row changed, and every uploader waits until the ref matches before it starts.
- **Builds**: the builder image is pulled from Artifact Registry `horizon-sdv/aaos-abfs-builder`, installs `abfs-client_<ABFS_VERSION>` from Google's apt repository `abfs-apt-alpha-public`, waits for casfs on the node (installed by the DaemonSet of `abfs-client-ubuntu`), then mounts the requested Android revision from `abfs-server`.
- **Argo CD vs. Developer Portal**: the pipeline scripts run by `aaos-builder-abfs` are cloned from the platform branch (`config.workloads.android.branch` from Terraform), not from the module's **Custom Git ref** in the portal. The Custom Git ref only changes the module's charts.

## Prerequisites <a name="prerequisites"></a>

1. **Horizon SDV deployed** with Module Manager and the Developer Portal; `workloads-common` and `workloads-android` enabled (see [Developer Portal user guide, Module manager](../../developer_portal_user_guide.md#module-manager)).
2. **GCP quotas** in the cluster region for the node pools: `n2-highmem-64` (server, max 2), `n2-standard-32` (uploaders, max 5), `c2d-highcpu-112` (builds, max 20, 112 vCPU each), Persistent Disk for 1 Ti per uploader plus build caches, one regional Spanner node.
3. **Config Connector** (KCC) in cluster mode, as deployed by the Horizon Terraform. The ABFS node pools, Spanner, GCS bucket and IAM are KCC resources.
4. **Google allow-listing** of the license service account for Artifact Registry projects `abfs-binaries` (images and apt packages). The account is created by `abfs-data` (see [Import the ABFS license](#license)); the former Jenkins-era account `abfs-server` is no longer used.
5. **ABFS license** from Google, bound to that service account and to the GCP project number. Request it through the EAP form linked by your Google representative.

## Procedures <a name="procedures"></a>

All portal paths below start at `https://<horizon-domain>/developer-portal/`. Module enable/disable is under **Administration → Modules**; a module's workflows are under the module's own page → **Workflow Templates** → **Configure & submit**; runs appear under **Running Workflows** / **History** and in Argo Workflows UI.

### Enable the ABFS feature <a name="enable"></a>

Enable order matters: `abfs-data` → license import → `abfs-service-module` → `abfs-client-ubuntu`. `workloads-common` and `workloads-android` must already be enabled (the client module declares them as hard dependencies).

1. **Enable `abfs-data`.** Wait until `mod-abfs-data`, `mod-abfs-data-sa`, `mod-abfs-data-license-workflows`, `mod-abfs-data-uploader-config` and `mod-abfs-data-spanner-workflows` are Synced/Healthy.
   ```bash
   kubectl get application -n argocd | grep abfs-data
   kubectl get secret abfs-license -n horizon                      # exists, data.license empty
   kubectl get spannerinstance,spannerdatabase,storagebucket -n horizon -l horizon-sdv.io/module=abfs-data
   kubectl get configmap abfs-uploader-config -n horizon -o jsonpath='{.data.table}'
   ```
2. **Import the license** (next section). Without it `abfs-service-module` cannot sync.
3. **Enable `abfs-service-module`.** The node-pool child Application syncs first (wave -1) and creates the two data pools with the license in their GCE metadata; the data-plane Application then starts `abfs-server`, the pusher-config Job and the three uploaders. First enable takes 10–20 minutes (node pool creation, casfs build on each node, pusher bootstrap).
   ```bash
   kubectl get application -n argocd | grep abfs-service-module
   kubectl get containernodepool -n abfs
   kubectl get configmap abfs-license-metadata-status -n abfs -o jsonpath='{.data.status}'   # applied
   kubectl get pods -n abfs
   kubectl logs -n abfs job/abfs-pusher-config
   ```
   The module overview page in the portal shows the same status (server, uploaders, casfs DaemonSet, pusher Job, GCE metadata).
4. **Enable `abfs-client-ubuntu`.** Creates `abfs-build-node-pool-ubuntu` (scaled to zero until a build runs) and its casfs installer DaemonSet.
   ```bash
   kubectl get application -n argocd | grep abfs-client-ubuntu
   kubectl get containernodepool abfs-build-node-pool-ubuntu -n abfs-client-module-ubuntu-overview
   ```
5. **Check the ABFS workflows are visible**: `abfs-license-import`, `abfs-license-clear`, `abfs-uploader-config-select`, `abfs-spanner-backup-schedule` on the `abfs-data` page, `abfs-smoke-test` on `abfs-service-module`, `aaos-builder-abfs` and `abfs-builder-runtime-image` on `workloads-android`.

Seeding starts as soon as the uploaders are Ready, for the row that is active in the uploader table (default `Android15_r36`, see [Seed Spanner](#seed)). A full Android branch takes roughly a day; builds for that revision work only after seeding completed.

### Import the ABFS license <a name="license"></a>

**Request the license.** After `abfs-data` is enabled, read the identity the license must be issued for and send it to Google with the EAP form:

```bash
PROJECT=<gcp-project-id>
gcloud iam service-accounts describe abfs-license@${PROJECT}.iam.gserviceaccount.com --format='value(email,uniqueId)'
gcloud projects describe ${PROJECT} --format='value(projectNumber)'
```

The UniqueID is persisted in ConfigMap `horizon/abfs-license-sa-identity` (not Helm-owned, survives disable). On re-enable a PreSync Job undeletes the soft-deleted account so the UniqueID, and therefore the license, is kept. If the account cannot be undeleted (after ~30 days) the sync fails on purpose; clearing `uniqueId` in that ConfigMap lets KCC create a new account, which needs a new license.

**Import.** Portal → `abfs-data` → Workflow Templates → `abfs-license-import`, paste the license into `licensePayload`, submit. The workflow verifies the payload before storing it and fails with one of: `license type unknown`, `license service account incorrect`, `license not applicable to this project`, `license already expired`. On success it writes Secret `horizon/abfs-license` key `license`. A later import replaces the license.

```bash
# CLI alternative and check
argo submit -n workflows --from workflowtemplate/abfs-license-import -p licensePayload="$(cat license.json)" --watch
kubectl get secret abfs-license -n horizon -o jsonpath='{.data.license}' | base64 -d | head -c 80; echo
```

**Changing a license while the service is enabled.** KCC treats `nodeConfig.metadata` as immutable and `gcloud container node-pools update` has no `--metadata` flag, so the import and clear workflows delete the KCC `ContainerNodePool` of the server and uploader pools when they exist; Argo recreates them with the new metadata. This drains the ABFS server and uploaders for the duration of the pool recreation. Import the license **before** enabling `abfs-service-module` whenever possible. `abfs-license-clear` (`confirm=yes`) empties the key but keeps the Secret.

### Seed Spanner with a new AAOS revision <a name="seed"></a>

Seeding is driven by the uploader table, ConfigMap `horizon/abfs-uploader-config`, owned by `abfs-data`. Exactly one row is `active`; the uploaders seed that row's manifest. Rows shipped by default (`gitops/modules/abfs-data-module/uploader-config/values.yaml`):

| Row | Manifest branch | Note |
|-----|-----------------|------|
| `Android14_r30` | `android-14.0.0_r30` | |
| `Android15_r36` | `android-15.0.0_r36` | active by default |
| `Android16_r3` | `android-16.0.0_r3` | |
| `AndroidLatest` | `android-latest-release` | moving branch |
| `AndroidCustom` | editable | any manifest `url`, `branch`, `file`, `fetch-ref-spec` |

Manifest URL `https://android.googlesource.com/platform/manifest`, file `default.xml`. The build parameter `UPLOADER_MANIFEST_SERVER` / `abfs.uploaderManifestServer` (`android.googlesource.com`) must stay the host the uploaders seed from.

**Switch or add a revision.**

1. Portal → `abfs-data` → Workflow Templates → `abfs-uploader-config-select`.
2. `Android_Versions`: pick a predefined row, or `AndroidCustom` and edit `Configuration` (a YAML block with `name: AndroidCustom` and `repo: {url, branch, file, fetch-ref-spec}`). For predefined rows `Configuration` is read-only. The custom block is saved and shown again the next time the form is opened.
3. Submit. The workflow patches the ConfigMap, then, when `abfs-service-module` is enabled, deletes uploader pod `-0` and restarts the StatefulSet. Pod `-0` publishes the new ABFS ref through a fresh `abfs-pusher-config-sync` Job, every uploader waits until the ref matches the active row, and seeding of the new manifest starts. The workflow waits up to 90 minutes for the ref and the rollout, then exits.
4. Monitor:
   ```bash
   kubectl logs -n abfs job/abfs-pusher-config-sync
   kubectl rollout status statefulset/abfs-gerrit-uploader -n abfs
   kubectl logs -n abfs abfs-gerrit-uploader-0 -c abfs-uploader --tail=100
   ```
   Seeding a new release branch takes about 24 hours; previously seeded revisions stay in Spanner/GCS and remain buildable. Selecting a row that is already active changes nothing and does not restart the uploaders.

To ship a new predefined row, add it to `uploaderConfig.entries` in the uploader-config values and sync `abfs-data`; the parent Application ignores `data` of the ConfigMap, so a row made active from the portal is not reverted by a sync. If the ConfigMap is absent (abfs-data disabled) the uploaders fall back to `pusher.fallbackRepo` in `abfs-service-module/abfs/values.yaml` and log a WARNING.

### Prepare the Docker image template <a name="image"></a>

The ABFS build pod uses image `horizon-sdv/aaos-abfs-builder` from the project's Artifact Registry: the shared Android builder Dockerfile built with `ABFS=true` (adds Google's apt transport, the ABFS apt source and `kmod`). Build it once per project, and again when the Dockerfile or the ABFS apt channel changes.

- **Argo (portal)**: `workloads-android` → Workflow Templates → `abfs-builder-runtime-image` → `NO_PUSH=false`, `templateFlavor=ubuntu` (tag `argowf-latest`). `aaos-builder-abfs` calls this template automatically when its image is missing, but with the template defaults (`noPush=true`) the image is **not** pushed, so run it once by hand.
- **Jenkins**: `Android Workflows → Environment → ABFS → Docker Image Template`, deselect `NO_PUSH` (tag `latest`). Review `ABFS_REPOSITORY` in **Seed Workloads** first.

Details: [ABFS Docker Image Template](environment/abfs/docker_image_template.md).

### Perform an AAOS build with ABFS <a name="build"></a>

Prerequisites: all three ABFS modules enabled, the builder image pushed, and the requested `AAOS_REVISION` seeded (its row was active long enough to complete).

**Argo (portal)**: `workloads-android` → Workflow Templates → `aaos-builder-abfs` → Configure & submit. Main parameters (defaults from `workloads/android/pipelines/builds/aaos_builder/helm/values-abfs.yaml`):

| Parameter | Default | Notes |
|-----------|---------|-------|
| `androidRevision` | `android-15.0.0_r36` | Tag or branch to mount (`abfs mount -b`). Must be seeded. |
| `lunchTarget` | `aosp_cf_x86_64_auto-bp1a-userdebug` | Any AOSP target; `sdk_car_*`, `aosp_cf_*`, `aosp_tangorpro_car`. |
| `postRepoCommand` | empty | Extra commands after mount. The build/soong sandbox fix is applied automatically per revision (table below). |
| `parallelBuildJobs` | `16` | Lower it if cacheman reports `resource temporarily unavailable`. |
| `abfs.server` | `abfs-server.abfs.svc.cluster.local:50051` | In-cluster Service of the server. |
| `abfs.clientFlags` | `--disable-tls=true --auth-type none` | Must match how the server listens (no TLS, no auth in this chart). |
| `abfs.version` / `abfs.casfsVersion` / `abfs.repository` | `0.1.14` / `0.1.14` / `abfs-apt-alpha-public` | Client package to install; casfs itself comes from the node DaemonSet. |
| `abfs.cachedBuild`, `abfs.cleanCache`, `abfs.cachemanTimeout` | `false`, `false`, `180` | Keep the cacheman cache on the per-workflow PVC (`750Gi`, class `reclaimable-storage-class-android-abfs`). |

Soong runs `soong_build` in an nsjail sandbox where the ABFS mount is read-only and `out` is writable only at `/src/out`; `aaos_environment.sh` fixes `build/soong` per `AAOS_REVISION`:

| `AAOS_REVISION` | Applied automatically |
|-----------------|-----------------------|
| `android-14*`, `android-15.0.0_r4` | nothing |
| `android-15*` (tested `android-15.0.0_r36`) | build/soong cherry-picks 3619490–3619492 plus the `ui/build/soong.go` sed |
| `android-16*` (tested `android-16.0.0_r3`) | `ui/build/soong.go` sed (`TOP`, `LOG_DIR` through `config.sandboxPath`) |
| branch names (`android-latest-release`, `main`, custom) | nothing; add the fix via `postRepoCommand` if the build needs it |

Each workflow runs as one pod on `abfs-build-node-pool-ubuntu` (one pod per node, pool autoscaling 0–20, so up to 20 builds in parallel). Artifacts go to `gs://<project>-aaos/Android/Builds/AAOS_Builder_ABFS/<date>_<workflow>/`; logs are in the portal/Argo UI and archived to the Argo workflows bucket. A build that is fully served from the ninjago cache finishes in 10–15 minutes; a cold arm64 build takes about 3 hours.

**Jenkins**: `Android Workflows → Builds → AAOS Builder ABFS`, same parameters in upper case (`AAOS_REVISION`, `AAOS_LUNCH_TARGET`, `POST_REPO_COMMAND`, `ABFS_*`), defaults from **Seed Workloads**.

Details: [ABFS Builder](builds/aaos_abfs_builder.md).

### Migrate an existing Spanner database to the new ABFS <a name="migrate"></a>

Applies to projects that ran ABFS 0.0.33 from the former Jenkins jobs (`Android Workflows → Environment → ABFS → Server/Uploader Administration`, Terraform-managed VMs, service account `abfs-server`, Spanner instance and bucket named by the job parameters `ABFS_DB_NAME` / `ABFS_DB_BUCKET_NAME`).

What the modules expect:

| Resource | Module name | Behaviour on enable |
|----------|-------------|---------------------|
| Spanner instance / database | `abfs` / `abfs` in `regional-<region>` | KCC acquires an existing instance and database with these ids (`deletion-policy: abandon`, so a later disable never deletes them). The database CR carries the ABFS DDL; it must not be changed after the first apply. |
| GCS bucket | `<projectID>-abfs` | Acquired when it exists. |
| License service account | `abfs-license@<project>.iam.gserviceaccount.com` | Created by KCC, or acquired/undeleted when an account with that name exists. The license is bound to the UniqueID, so the Jenkins-era account `abfs-server` is **not** reused: a new license from Google is required for `abfs-license`. |

Procedure:

1. Stop the old uploaders and server (destroy the Terraform-managed VMs with the old Jenkins jobs if they still exist, or delete the instances), so nothing writes to Spanner during the migration.
2. Back up the old database: `gcloud spanner backups create ... --instance=<old> --database=<old>` (or let the old destroy job's backup logic run).
3. Bring the data to the module names **before** enabling `abfs-data`. If the old resources already use `abfs` / `abfs` / `<projectID>-abfs`, nothing to do. Otherwise create instance `abfs` (`regional-<region>`, 1 node), copy the backup to it (`gcloud spanner backups copy`) and restore it as database `abfs` (`gcloud spanner databases restore`), then copy the blobs with `gcloud storage cp -r gs://<old-bucket>/ gs://<projectID>-abfs/`. The order matters: if `abfs-data` is enabled first, KCC creates an empty database `abfs` with the ABFS DDL, and a Spanner restore cannot target an existing database.
4. Enable `abfs-data` and check KCC reports the existing resources as Ready without altering them:
   ```bash
   kubectl get spannerinstance,spannerdatabase,storagebucket -n horizon -o wide
   gcloud spanner databases ddl describe abfs --instance=abfs --project=${PROJECT}
   ```
5. Request and import the new license for `abfs-license`, enable `abfs-service-module`, and confirm the server serves the existing refs before re-seeding anything (`abfs test-client get-ref` from an uploader pod, or submit a build for an already seeded revision).
6. Clean up the Jenkins-era leftovers (`abfs-server` / `abfs-runtime` accounts, old firewall rules, the old Spanner instance once the restore is verified). The `abfs-data` README has the `abfs-runtime` clean-up commands.

> [!NOTE]
> The Spanner schema shipped with the modules is the one ABFS has used since 0.0.31, and KCC acquisition of a database that already holds that schema has not been exercised in Horizon test environments. Validate the procedure on a copy of the database first and confirm the 0.0.33 → 0.1.14 data compatibility with Google. Re-seeding from scratch (about a day per branch) is the fallback when the data cannot be carried over.

## Day-2 operations <a name="day2"></a>

### Spanner backup schedules <a name="backup"></a>

Portal → `abfs-data` → `abfs-spanner-backup-schedule`: `backupScheduleAction` `DETAILS` (list), `CREATE` / `UPDATE` (`backupScheduleId`, `cron` default `0 23 * * *`, `retentionDuration` default 604800 s) or `DELETE` (`confirm=yes`). Schedules are created against database `abfs` on instance `abfs`.

### Disable ABFS <a name="disable"></a>

Disable in reverse order: `abfs-client-ubuntu` → `abfs-service-module` → `abfs-data`. The portal refuses to disable a module that is still a hard dependency of an enabled one.

What remains after each disable:

- **abfs-client-ubuntu**: nothing in the cluster; the GKE node pool is deleted by KCC.
- **abfs-service-module**: pods, Jobs and the two data node pools are removed; uploader PVCs are deleted (`uploader.dataDisk.deletePvcWhenDisabled: true`). Spanner/GCS content is untouched.
- **abfs-data**: Secret, workflows, uploader table and the KCC CRs are pruned. The Spanner instance and database and the GCS bucket are **abandoned** in GCP (data kept, still billed); delete them with `gcloud` when ABFS is retired for good. The license service account is soft-deleted (recoverable ~30 days, see [Import the ABFS license](#license)).

After a disable, confirm the GKE pools are gone (`gcloud container node-pools list --cluster <cluster> --region <region>`). A pool can be left behind when the CRs are removed while KCC is still reconciling. For the two data pools the service module's PreSync Job deletes such an orphan GKE pool before the next create; if it is still there the `ContainerNodePool` reports `UpdateFailed ... already exists` (the pool keeps working, but the status stays red) and the fix is `gcloud container node-pools delete` followed by an Argo sync. The build pool has no such hook: delete an orphan `abfs-build-node-pool-ubuntu` by hand before re-enabling `abfs-client-ubuntu`.

### Update the ABFS version <a name="update"></a>

ABFS versions are pinned in Git; a new release from Google means a change in:

| Setting | File |
|---------|------|
| Server/uploader image tag | `gitops/modules/abfs-service-module/abfs/values.yaml` `image.tag` |
| casfs source for the data pools | same file, `casfs.daemonset.sourceUrl` (`gs://cos-localmirror/distfiles/casfs-<version>.tar.gz`) |
| casfs version / apt repository for the build pool | `gitops/modules/abfs-client-module-ubuntu/values.yaml` `casfs.daemonset.version`, `aptRepository`, `image.tag` |
| Client package for Argo builds | `workloads/android/pipelines/builds/aaos_builder/helm/values-abfs.yaml` `abfs.version`, `abfs.casfsVersion`, `abfs.repository` |
| Client package for Jenkins builds | **Seed Workloads** `ABFS_VERSION`, `ABFS_CASFS_VERSION`, `ABFS_REPOSITORY`; re-run the seed |

Every ABFS build writes `abfs_repository_list.txt` (listing of the apt repository together with the node kernel) to its artifacts; use it to pick versions that exist for the running GKE kernel. casfs must match the node kernel: the installer first looks for a prebuilt `casfs-kmod-<kernel>_<version>` package and otherwise builds from the source tarball, so a GKE node upgrade to a kernel Google has no package for still works as long as the source builds. If it does not, pin the node pool version (`config.abfsBuildNodePool.version` from Terraform for the build pool) until Google publishes a matching release.

## Known issues <a name="knownissues"></a>

- **Resource temporarily unavailable** in ninja right after the modules were (re-)enabled: casfs blob fetches time out while the uploaders re-verify their refs. Resubmit the build with the same parameters, or lower `parallelBuildJobs`.
- **Builds of unseeded revisions** fail at `abfs mount`. Seed the row first; a branch takes ~24 h.
- **License changes recreate the data node pools** (see [Import the ABFS license](#license)); plan them outside build hours.
- **Portal Custom Git ref does not switch pipeline scripts**: `aaos-builder-abfs` clones `aaos_environment.sh` and friends from the platform branch set in Terraform. Test script changes by merging to that branch or by passing the commands through `postRepoCommand`.
- **Gemini / AI review on ABFS builds** has no access to the mount (`/abfs`), so its findings are of little use for ABFS-specific failures; read the raw build log.
- **ABFS UI** (`abfs-ui`) is disabled in the chart (`ui.enabled: false`) until it is functional.
- **Uploader capacity is not elastic**: uploader replicas are addressed by name in the pusher config, so there is no HPA. Changing the count is a chart change (`uploader.count`), see the service module README.

## References <a name="references"></a>

- [abfs-data module README](../../../gitops/modules/abfs-data-module/README.md)
- [abfs-service-module README](../../../gitops/modules/abfs-service-module/README.md)
- [abfs-client-ubuntu module README](../../../gitops/modules/abfs-client-module-ubuntu/README.md)
- [ABFS Builder (Argo and Jenkins parameters)](builds/aaos_abfs_builder.md)
- [ABFS Docker Image Template](environment/abfs/docker_image_template.md)
- [Developer Portal user guide](../../developer_portal_user_guide.md)
- Google's standalone ABFS guides (reference only, not the Horizon deployment): `experimental/kcc-google-abfs/docs/`
