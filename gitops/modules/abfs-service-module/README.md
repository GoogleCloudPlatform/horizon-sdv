<!-- Copyright 2026 Google LLC

Modifications Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Android Build File System (ABFS) Service Module

Helm chart: `gitops/modules/abfs-service-module`  
Argo CD parent Application: **`mod-abfs-service-module`** (enabled from the Developer Portal, catalog name `abfs-service-module`).

The service module is the ABFS **runtime data plane**: the dedicated server and uploader node pools, the ABFS server, the Gerrit uploaders (git-pushers) that seed Android sources into ABFS, the pusher-config bootstrap, and the `casfs` kernel module installer for those pools. Persistent data (license, Spanner, GCS, uploader manifest table) belongs to `abfs-data-module`; build nodes belong to `abfs-client-module-ubuntu`. The platform-level picture, enable order and procedures are in [docs/workloads/android/abfs.md](../../../docs/workloads/android/abfs.md).

---

## Module Metadata

| Metadata Field | Value |
| :--- | :--- |
| **STATUS** | horizon-verified |
| **OWNER** | Accenture |
| **CONTACT** | module: [horizon-sdv@accenture.com](mailto:horizon-sdv@accenture.com), ABFS technology: [horizon-sdv@google.com](mailto:horizon-sdv@google.com) |
| **CHART VERSION** | 0.2.0 |
| **CONTAINS_COMMERCIAL** | no |

---

## Catalog

| Setting | Value |
|---------|-------|
| Module name | `abfs-service-module` |
| Hard dependencies | none declared in the catalog. Runtime prerequisite: `abfs-data` enabled **and** a license imported (PreSync validator) |
| Depended on by | `abfs-client-ubuntu` (hard) |
| Overview service | `mod-abfs-service-module-overview` in namespace `abfs` (live status page, `files/overview-server.py`) |

Enable order: `abfs-data` → import license → **abfs-service-module** → `abfs-client-ubuntu`.

---

## Directory Structure

```
gitops/modules/abfs-service-module/
├── Chart.yaml                  # Parent Helm chart metadata
├── values.yaml                 # Parent values (licensed, VPA, overview image, dataOverview link)
├── README.md                   # This file
├── files/
│   └── overview-server.py      # Live status HTTP server for the portal overview
├── portal/
│   └── overview.html           # Developer Portal overview (status placeholder injected)
├── templates/
│   ├── application-node-pool.yaml      # child app mod-<module>-node-pool (sync-wave -1)
│   ├── application-abfs.yaml           # child app mod-<module>-data-plane
│   ├── application-argo-workflows.yaml # child app mod-<module>-argo-workflows
│   └── module-overview-http.yaml       # overview Deployment/Service
├── node-pool/                  # Sub-chart: server + uploader ContainerNodePools, license metadata hooks
│   ├── values.yaml             # Pool shapes, license Secret/metadata settings
│   └── templates/
├── abfs/                       # Sub-chart: the ABFS workloads (license-gated)
│   ├── values.yaml             # image, server, uploader, pusher, casfs, KCC IAM, network policy
│   ├── scripts/                # pusher tree / pusher config / sync Job helpers
│   └── templates/              # Deployment, StatefulSet, Jobs, DaemonSet, KCC IAM, PDB, VPA, NetworkPolicy
└── argo-workflows/             # Sub-chart: abfs-smoke-test WorkflowTemplate + Sensor
```

`node-pool/` is a sub-chart with its own Application rather than a template under `abfs/`: everything in `abfs/` renders nothing while `licensed` is false, whereas the node pools have to exist before the license can be stamped into their metadata.

### External platform dependencies

- `gitops/apps/module-manager/templates/module-catalog.yaml` registers the module (chart path, overview page, overview Service and namespace).
- Module Manager injects `config.projectID`, `config.region`, `config.clusterName`, `config.nodeLocations`, `config.gcpComputeSaEmail`, `config.releaseChannel` into the child Applications.
- Secret `horizon/abfs-license`, service account `abfs-license`, ConfigMap `horizon/abfs-uploader-config`, Spanner `abfs` and bucket `<projectID>-abfs` come from `abfs-data-module`.

---

## What it deploys

| Child Application | Chart | Namespace | Purpose |
|-------------------|-------|-----------|---------|
| `mod-abfs-service-module-node-pool` (wave -1) | `node-pool/` | `abfs` | KCC `ContainerNodePool` `abfs-data-server-node-pool` and `abfs-data-uploader-node-pool`; PreSync/PostSync Job `abfs-license-metadata-inject` keeping GCE metadata `abfs-license` equal to the Secret; status ConfigMap `abfs-license-metadata-status` |
| `mod-abfs-service-module-data-plane` | `abfs/` | `abfs` | PreSync `abfs-license-validate`; KCC `IAMPolicyMember`s for the license SA; Deployment `abfs-server` + Service (`:50051`, internal LB by default); StatefulSet `abfs-gerrit-uploader` (3); Job `abfs-pusher-config`; DaemonSet `abfs-casfs-installer`; PDBs, VPAs (`Off`), NetworkPolicy, Helm test |
| `mod-abfs-service-module-argo-workflows` | `argo-workflows/` | `workflows`, `argo-events` | WorkflowTemplate `abfs-smoke-test` (exposed in the portal) and its webhook Sensor |

### Module ownership

| Module | Owns |
| --- | --- |
| `abfs-service-module` | server and uploader node pools, ABFS server, uploaders, pusher-config bootstrap, casfs installer for the data pools, IAM project roles of the license SA, policies |
| `abfs-data-module` | license SA and Secret, Spanner, GCS bucket, uploader manifest table, license / uploader-config / Spanner-backup workflows |
| `abfs-client-module-ubuntu` | build node pool `abfs-build-node-pool-ubuntu` and its casfs installer |
| `workloads-android` | the ABFS build and builder-image WorkflowTemplates and pipeline scripts |

The service module configures the connection to Spanner and GCS but does not create those resources, so disabling the service never removes persistent data.

---

## Runtime requirements

- **License**: `licensed: true` is the desired state; the PreSync hook `abfs-license-validate` (wave -10) fails the sync of the data plane when `horizon/abfs-license` is missing or `data.license` is empty. ABFS 0.1.14 reads the license from GCE instance metadata and checks for a Compute Engine identity, so both pools run with the license SA as node service account (`GCE_METADATA`); Workload Identity cannot provide that. The license value is never stored in Git.
- **Config Connector** in cluster mode, as deployed by Horizon; the parent Applications inject project, region and namespace.
- **Ubuntu nodes with Secure Boot off** (`UBUNTU_CONTAINERD`) on the uploader pool, where `casfs.provider=daemonset` builds the kernel module on the node (`casfs.daemonset.sourceUrl`, DaemonSet `abfs-casfs-installer` with `casfs.daemonset.nodeSelector` = uploader pool). The uploaders and the pusher Job run `abfs cacheman`, which mounts casfs, and gate on `/sys/module/casfs` before starting (`casfs.requireReady`, 600 s); the server itself does not need casfs. Both pools are kept Ubuntu so the DaemonSet can be widened to the server pool by setting a shared `nodeSelector`.
- **Enable `abfs-data` and import the license first**, so the pools are created with the metadata and do not have to be recreated.

---

## Node pools

| Pool | Machine type | Autoscaling | Taint / label | Workload |
| :--- | :--- | :--- | :--- | :--- |
| `abfs-data-server-node-pool` | `n2-highmem-64` (64 vCPU / 512 GiB) | 0–2 | `workloadType=android-abfs-data-server`, `workloadLabel=android-abfs-data` | `abfs-server` (requests 32 CPU / 256 Gi) |
| `abfs-data-uploader-node-pool` | `n2-standard-32` (32 vCPU / 128 GiB) | 0–5 | `workloadType=android-abfs-data-uploader`, `workloadLabel=android-abfs-data` | `abfs-gerrit-uploader-{0,1,2}` (24 CPU / 96 Gi each, one pod per node), `abfs-pusher-config`, casfs installer |

Both pools: 100 GB boot disk, `UBUNTU_CONTAINERD`, Secure Boot off, node SA = license SA, `autoUpgrade` on unless the cluster release channel is `UNSPECIFIED` (then `version` pins apply). Two pools rather than one shared data pool so server and uploaders scale and are sized independently.

### License metadata on the pools

The license reaches the ABFS processes as GCE instance attribute `abfs-license` (value = Secret `data.license`, i.e. base64 of the payload). KCC treats `nodeConfig.metadata` as immutable and `gcloud container node-pools update` has no `--metadata` flag, therefore:

- At create, Helm `lookup` of the Secret fills the metadata (best effort: Argo CD's `helm template` often cannot see Secrets).
- PostSync Job `abfs-license-metadata-inject` waits for each CR, compares metadata with the Secret and, when they differ, re-creates the CR with the metadata (pool recreate). It writes `abfs/abfs-license-metadata-status` (`status: applied`).
- PreSync of the same Job clears `UpdateFailed` CRs and orphan GKE pools (a pool that exists in GKE without its CR). A Ready CR is never deleted only because the license key is missing.
- The `abfs-license-import` / `abfs-license-clear` workflows of `abfs-data` also delete both CRs when they exist, so a license change is a pool recreate. Prefer importing before enabling this module.

### Sizing notes

Each uploader has a 1 Ti `premium-rwo` PVC (`uploader.dataDisk`). Existing StatefulSet PVCs do not resize in place; shrinking means deleting PVC and pod. `deletePvcWhenDisabled: true` maps to `persistentVolumeClaimRetentionPolicy.whenDeleted=Delete`, so disabling the module removes the staging disks; set it to `false` to keep them across disable/enable. One uploader per node is deliberate: disk throughput limits are per instance.

Rejected alternatives: `n2-standard-16` for uploaders (~58 GiB allocatable, OOM at 32 Gi), `c2d-highcpu-112` / `n2-highcpu-32` (memory profile, no Hyperdisk), a single shared data pool (server and uploaders compete).

### Autoscaling

Capacity scales at node level (min 0). VPAs run in `updateMode: "Off"` for recommendations only. There is deliberately **no HPA on the uploader StatefulSet**: VPA and HPA on one target is unsupported, `uploader.count` must equal the number of pusher entries, and uploader replicas are **not interchangeable** — each pod is assigned work by its hostname (`<namePrefix>-<ordinal>`) in the pusher config, so an extra replica would idle. Changing uploader capacity is a chart change (`uploader.count`), not runtime elasticity.

---

## Seeding: pusher config and the uploader table

Which Android manifest the uploaders seed is decided by the **active row** of ConfigMap `horizon/abfs-uploader-config` (owned by `abfs-data`, changed with its `abfs-uploader-config-select` workflow). The flow in this module:

1. Job `abfs-pusher-config` (sync-wave 10, after the server) reads the table, builds the pusher tree (`scripts/build_pusher_tree.sh`, `scripts/modify_pusher_config.py`) and compares its hash with the ABFS ref `config` of project `abfs-meta`. Equal hash → exit 0; different → `abfs push` the tree and point the ref at it. PreSync hook `abfs-pusher-config-presync` deletes a stale Job so a failed bootstrap can retry.
2. Uploader init container `select-uploader-config` repeats the table read on every start; pod `-0` additionally re-runs a copy of the Job as `abfs-pusher-config-sync` (`scripts/recreate_pusher_job.py`) so a row change takes effect on restart.
3. Init container `wait-for-server` blocks until `abfs-server` answers and the ref matches the active row; only then does the uploader start fetching.
4. If the ConfigMap is unreadable (abfs-data disabled), `pusher.fallbackRepo` from `abfs/values.yaml` is used with a WARNING.

`pusher.workers` (200) and `pusher.concurrentUploads` (200) must be identical on every pusher. The pusher names must equal the uploader pod hostnames.

---

## Validate after enable

1. In the Developer Portal wait until the module state changes from "NOT INSTALLED" to "READY".
2. In Argo CD wait until `mod-abfs-service-module`, `mod-abfs-service-module-node-pool`, `mod-abfs-service-module-data-plane` and `mod-abfs-service-module-argo-workflows` are Synced/Healthy. First enable takes 10–20 minutes.
3. Command-line checks:

```bash
kubectl get application -n argocd | grep abfs-service-module
# expect: mod-abfs-service-module, -node-pool, -data-plane, -argo-workflows — Synced/Healthy

kubectl get containernodepool -n abfs
kubectl get configmap abfs-license-metadata-status -n abfs -o jsonpath='{.data.status}'; echo   # applied
kubectl get pods -n abfs -o wide
kubectl get job -n abfs abfs-pusher-config -o jsonpath='{.status.conditions[*].type}'; echo      # Complete
kubectl logs -n abfs abfs-gerrit-uploader-0 -c wait-for-server | tail -3                       # ref matches the active row
kubectl get workflowtemplate abfs-smoke-test -n workflows

# From an uploader pod: the ref the pushers follow
kubectl exec -n abfs abfs-gerrit-uploader-0 -c abfs-uploader -- sh -c \
  'echo config | abfs --remote-servers abfs-server:50051 --tunnel-ports 0 --disable-tls=true --auth-type none test-client get-ref -p abfs-meta -s default-server'
```

The portal overview page (`mod-abfs-service-module-overview`) shows the same: license Secret state, GCE metadata applied, server Deployment, uploader StatefulSet and pods, casfs DaemonSet, pusher Job.

### After disable

Pods, Jobs, DaemonSet, NetworkPolicies and both `ContainerNodePool` CRs are pruned and KCC deletes the GKE pools; uploader PVCs are deleted (`deletePvcWhenDisabled`). Spanner, GCS, the license Secret and the uploader table remain (owned by `abfs-data`). Check the GKE pools are really gone; an orphan pool makes the next enable report `already exists` until the PreSync hook or `gcloud container node-pools delete` removes it.

---

## Chart configuration

- `abfs/values.yaml` provides defaults for every setting; the parent Application supplies project, region and namespace.
- `image.repository` / `image.tag`: `europe-docker.pkg.dev/abfs-binaries/abfs-containers-alpha/abfs-alpha:0.1.14`. The license SA needs Artifact Registry read on project `abfs-binaries` (granted by Google through the EAP allow-list, not by this chart).
- `spanner.instance` / `spanner.database` (`abfs`), `storage.bucket` (empty → `<projectID>-abfs`): connection settings for resources owned by `abfs-data`.
- `client.tls: false`, `client.authType: none`: the server listens without TLS and auth; build pods must pass `--disable-tls=true --auth-type none` (`abfs.clientFlags` in the builder chart).
- `server.service.internalLoadBalancer: true` adds an internal LB in front of `abfs-server` for clients outside the cluster (for example the standalone VM guide in `experimental/kcc-google-abfs/docs`); in-cluster builds use `abfs-server.abfs.svc.cluster.local:50051`.
- `ui.enabled: false`: the ABFS UI stays off until functional; `dataPlane.expose: false`.
- `otel.enabled`: metrics to the GKE managed OpenTelemetry collector.
- `casfs.daemonset.sourceUrl`: casfs source tarball built on every node of the pools; update together with `image.tag` on an ABFS release.

---

## Getting Started

Enable and disable the module through Module Manager in the Developer Portal (Administration → Modules). Argo CD then reconciles everything under `gitops/modules/abfs-service-module/`. Procedures (license, seeding a revision, builds, teardown, version update) are in [docs/workloads/android/abfs.md](../../../docs/workloads/android/abfs.md).
