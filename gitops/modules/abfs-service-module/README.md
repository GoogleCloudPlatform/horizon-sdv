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

# Android Build File System (ABFS) Module

The ABFS module provides integrated support for high-performance Android Open Source Project (AOSP) builds on Google Cloud Platform, using a specialized kernel-level Android Build File System (`casfs`) and high-efficiency background uploader/synchronizer components.

---

## Module Metadata

| Metadata Field | Value |
| :--- | :--- |
| **STATUS** | experimental |
| **OWNER** | Google |
| **CONTACT** | [horizon-sdv@google.com](mailto:horizon-sdv@google.com) |
| **VERSION** | 0.1.14 |
| **CONTAINS_COMMERCIAL** | no |

---

## Features

- **Painless CASFS Kernel Compilation**: Installs and loads the customized `casfs` filesystem kernel module onto GKE worker nodes dynamically.
- **Dedicated ABFS Node Pools**: The module creates and owns separate server and uploader pools (see *Node pool* below), so each side has independent scaling and limits.
- **Node Pool Service Account Identity**: Server and uploader pools both use the licensed runtime SA, `GCE_METADATA`, and GCE metadata key `abfs-license` (git-pusher has the same entitlement check as the server).
- **Native GitOps Lifecycle**: Packaged as a standard Helm chart managed natively by Argo CD.

---

## Directory Structure

```
gitops/modules/abfs-service-module/
├── Chart.yaml                  # Parent Helm chart metadata
├── values.yaml                 # Parent values (licensed, VPA, overview image)
├── README.md                   # This module documentation file
├── files/
│   └── overview-server.py      # Live mobilisation status HTTP server
├── portal/
│   └── overview.html           # Developer overview (status placeholder injected)
├── node-pool/                  # Sub-chart: the dedicated ABFS data node pool
│   ├── Chart.yaml              # Node pool Helm chart
│   ├── values.yaml             # Pool shape (machine type, autoscaling, taint)
│   └── templates/              # KCC ContainerNodePool + license metadata hooks
└── abfs/                       # Sub-chart containing resources & templates
    ├── Chart.yaml              # ABFS workloads Helm chart
    ├── values.yaml             # Default workload configuration parameters
    └── templates/              # Deployment, StatefulSet, VPA, license validator, …
```

`node-pool/` is a sub-chart of its own, with its own Application, rather than a
template under `abfs/`: everything in `abfs/` is licence-gated and renders nothing
while `licensed` is false, whereas the node pool has to exist *before* the licence.

---

## External Platform Dependencies

One file outside `gitops/modules/abfs-service-module/` is required, to register the module
with Horizon's central catalog:

- **`gitops/apps/module-manager/templates/module-catalog.yaml`**: registers
  `abfs-service-module` (chart path, overview page, overview Service and namespace) with
  Module Manager, making the module visible and enable/disable-able in the Developer Portal.

---

## Module Overview

The module is integrated with Horizon's GitOps deployment and uses the platform's
cluster-mode Config Connector setup.

### Module Ownership

| Module | Owns |
| --- | --- |
| `abfs-service-module` | ABFS server, uploaders, bootstrap job, policies, and data node pools.|
| `abfs-data-module` | Spanner, GCS bucket, runtime service account, and licence Secret. |
| `abfs-client-module-ubuntu` / `-cos` | Build node pools, CASFS for build nodes, and ABFS-enabled build workflows. |

The service module configures connections to Spanner and GCS but does not create
those resources. The data module owns their lifecycle so disabling the service
does not remove persistent data. The service module also manages the IAM roles
and pusher configuration required by its workloads.

### Node Pool Lifecycle

Config Connector manages the server and uploader node pools; Terraform manages
the GKE cluster. Argo CD creates or removes the pools with this module.

### Chart Configuration

- The values file provides defaults for every setting used by the chart. The parent
  Application supplies environment-specific values such as project, region, and namespace.

- `parentModuleName` is retained because the parent Application injects it for resource labels.
- Spanner and bucket values configure external resources owned by `abfs-data-module`.
  Initialize the database schema before enabling the ABFS workloads.

### Platform Integration

Horizon runs Config Connector in cluster mode. Parent Applications inject the
project, namespace, and region; no per-module Config Connector resource is needed.

### Runtime Requirements

The licence Secret is validated before workload sync. ABFS 0.1.14 reads licence
data from GCE instance metadata and checks for a Compute Engine identity, so
both pools use `GCE_METADATA` and the licensed runtime service account. The
licence value is sourced from the data module's Secret and is never stored in Git.

With `casfs.provider=daemonset`, the installer builds CASFS for the running
kernel. Use Ubuntu nodes with Secure Boot disabled and configure
`casfs.daemonset.sourceUrl`.

Enable `abfs-data-module` and import a valid ABFS licence through its
`abfs-license-import` workflow before enabling this module. This ensures the
required data resources are available and the `abfs-license` Secret is populated
when the node pools are created.

---

## Node Pool Sizing

| Pool | Machine type | Autoscaling | Taint | License identity |
| :--- | :--- | :--- | :--- | :--- |
| server | `n2-highmem-64` | min 0, max 2 | `workloadType=android-abfs-data-server` | `GCE_METADATA` + `abfs-license` |
| uploader | `n2-standard-32` | min 0, max 5 | `workloadType=android-abfs-data-uploader` | `GCE_METADATA` + `abfs-license` |

Both pools use `UBUNTU_CONTAINERD`, 100 GB boot disks, and the cluster's release
channel for node upgrades (`autoUpgrade` is true unless the channel is `UNSPECIFIED`).

### Identity and Licence

ABFS server and git-pusher read `abfs-license` from GCE instance metadata and
require a Compute Engine identity. Both pools therefore use `GCE_METADATA` and
the licensed runtime service account; Workload Identity does not provide the
metadata ABFS requires. The account is owned by `abfs-data-module` and must not
be recreated because the licence is bound to its numeric identity.

The licence Secret is never committed to Git. KCC applies metadata at pool
creation, and changing the Secret may require a pool recreation. Import the
licence before enabling this module.

### Sizing

The server pool uses **`n2-highmem-64`** (64 vCPU / 512 GiB), The uploader pool uses
`n2-standard-32` (32 vCPU / 128 GiB). Both boot disks are **100 GB**.

Pods differ by requests only (`abfs/values.yaml`):

| Workload | Machine type | Pod CPU | Pod memory |
| :--- | :--- | :--- | :--- |
| Server (1) | `n2-highmem-64` (64 vCPU / 512 GiB) | 32 | 256 Gi |
| Uploaders (3) | `n2-standard-32` | 24 | 96 Gi |

One pod per node on the uploader pool (max 5 = 3 uploaders plus headroom). Each uploader has
a **1 Ti** (`1024Gi`) `premium-rwo` PVC. Requests use most of the dedicated node
while leaving kubelet/casfs headroom. **Existing StatefulSet PVCs do not resize
in place** — shrinking from 4 Ti requires deleting the PVC (and usually the pod)
so a new claim is created.

`uploader.dataDisk.deletePvcWhenDisabled: true` sets
`persistentVolumeClaimRetentionPolicy.whenDeleted=Delete` so disabling the
module (Argo prune of the StatefulSet) also removes those PVCs. Set it `false`
to Retain staging disks across disable/enable. Already-orphaned PVCs from
before this flag must be deleted manually once.

One pod per node is deliberate: **disk performance limits are shared across
volumes on the same instance**, so packing uploaders together would make them
compete for the throughput a dedicated pool exists to guarantee.

Rejected candidates:

- **`n2-standard-16`** — ~58 GiB allocatable; uploaders OOM at 32 Gi.
- **Separate server/uploader pools** — not used; one shared data pool only.
- **`c2d-highcpu-112`** / **`n2-highcpu-32`** — wrong memory profile or no Hyperdisk.

Review requests against production measurements and update the machine type and
maximum node count together when sizing changes.

### Autoscaling

Capacity scales at the node level (`minNodeCount: 0`, `maxNodeCount: 5`). VPAs
provide sizing recommendations in `updateMode: "Off"`.

There is deliberately **no HorizontalPodAutoscaler on the uploader StatefulSet**.
Three reasons, in increasing order of how hard they are to remove:

1. `vpa.yaml` already renders a VerticalPodAutoscaler against that StatefulSet.
   VPA and HPA on the same target, both managing CPU and memory, is unsupported.
2. `uploader.count` generates the pusher config, which `values.yaml` requires to
   match the replica count exactly. Changing replicas at runtime desynchronises it.
3. Most fundamentally, uploader replicas are **not interchangeable**. Each pod is
   assigned work by name in the pusher config, keyed on its StatefulSet hostname
   (`<namePrefix>-<ordinal>`). A replica added by an HPA has no pusher entry and
   therefore does no work — it would occupy a node and idle.

An HPA assumes interchangeable replicas sharing a queue, which is not how the ABFS
uploaders are built. Changing uploader capacity is therefore a configuration change
(`uploader.count` and the pusher config together), not a runtime elasticity
decision. Keep `uploader.count` and the pusher configuration aligned when changing
capacity.

### CASFS and Upgrades

The workloads wait for CASFS to load. The installer requires Ubuntu nodes,
Secure Boot disabled, and a configured source URL. An empty `releaseChannel`
enables node auto-upgrades; version pins apply only when the channel is
`UNSPECIFIED`.

---

## Getting Started

The module is enabled and disabled through the Module Manager in the Developer Portal. Argo CD then reconciles everything under `gitops/modules/abfs-service-module/`.

