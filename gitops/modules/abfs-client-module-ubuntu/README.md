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

# abfs-client-ubuntu module

Helm chart: `gitops/modules/abfs-client-module-ubuntu`  
Argo CD parent Application: **`mod-abfs-client-ubuntu`** (enabled from the Developer Portal, catalog name `abfs-client-ubuntu`).

Provides the **build side** of ABFS: the Ubuntu GKE node pool on which ABFS-backed Android builds run, and the DaemonSet that installs the `casfs` kernel module on every node of that pool. It deploys no ABFS processes of its own; the builds are the `aaos-builder-abfs` Argo WorkflowTemplate (and the Jenkins `AAOS Builder ABFS` job) from `workloads-android`, which schedule onto this pool.

The platform-level picture and procedures are in [docs/workloads/android/abfs.md](../../../docs/workloads/android/abfs.md).

## Module Metadata

| Metadata Field | Value |
| :--- | :--- |
| **STATUS** | horizon-verified |
| **OWNER** | Accenture |
| **CONTACT** | module: [horizon-sdv@accenture.com](mailto:horizon-sdv@accenture.com), ABFS technology: [horizon-sdv@google.com](mailto:horizon-sdv@google.com) |
| **CHART VERSION** | 0.1.0 |
| **CONTAINS_COMMERCIAL** | no |

## Catalog

| Setting | Value |
|---------|-------|
| Module name | `abfs-client-ubuntu` |
| Hard dependencies | `workloads-android`, `abfs-service-module` (enabling this module enables them first; they cannot be disabled while this module is enabled) |
| Overview service | `mod-abfs-client-module-ubuntu-overview` in namespace `abfs-client-module-ubuntu-overview` |

Enable order: `abfs-data` → license → `abfs-service-module` → **abfs-client-ubuntu**. Disable it first when tearing ABFS down.

## What it deploys

| Resource | Where | Purpose |
|----------|-------|---------|
| Child Application `mod-abfs-client-ubuntu-node-pool` (chart `node-pool/`) | `argocd` | KCC `ContainerNodePool` **`abfs-build-node-pool-ubuntu`** |
| DaemonSet `abfs-casfs-installer-client` + ConfigMap (`install.sh`) | `module-manager` (destination namespace of the parent module Application) | Loads `casfs` on each node of the pool |
| Overview Deployment/Service | `abfs-client-module-ubuntu-overview` | Portal overview page |

### Node pool

| Setting | Value |
|---------|-------|
| Machine type | `c2d-highcpu-112` (112 vCPU / 224 GiB), 500 GB boot disk |
| Image | `UBUNTU_CONTAINERD`, Secure Boot **off** (immutable after create; required to load a self-built kernel module) |
| Autoscaling | 0–20 nodes, starts at 0; a node is added per build pod and removed after |
| Taint / labels | `workloadType=android-abfs-build:NoSchedule`, `workloadLabel=android-abfs-build`, `cloud.google.com/gke-nodepool=abfs-build-node-pool-ubuntu` |
| Node SA | the cluster compute SA (`config.gcpComputeSaEmail`), no license metadata — the license is checked by the server, not by clients |
| Version | `config.abfsBuildNodePool.version` from Terraform when set, else the cluster release channel (`autoUpgrade` unless `UNSPECIFIED`) |

Build pods select the pool with the `nodeSelector` / `tolerations` above (`values.yaml`; the builder chart uses the same values), one build pod per node (pod anti-affinity in the builder chart). The Jenkins ABFS agents use the same label and toleration.

### casfs installer

The DaemonSet and its script are the shared library chart [`abfs-common/casfs-installer`](../abfs-common/casfs-installer/README.md), which `abfs-service-module` uses as well; this module only sets `casfs.*` in `values.yaml`.

`casfs.provider: daemonset`. The script, run privileged with the host filesystem mounted, first looks for the prebuilt package `casfs-kmod-<kernel>_<casfs.daemonset.version>` in Artifact Registry `abfs-binaries/us/abfs-apt-alpha-public` and installs it; if the package is missing or does not load, it downloads `casfs-<version>.tar.gz` from `casfs.daemonset.sourceUrl` (`gs://cos-localmirror/distfiles`) and builds the module for the running kernel with the node toolchain (`make ko KERNEL_RELEASE=$(uname -r)`), then `modprobe casfs`. The build pod waits up to 600 s for `/sys/module/casfs` (`abfs_wait_for_casfs` in `aaos_initialise.sh`) before installing the ABFS client.

Versions to keep aligned on an ABFS release: `casfs.daemonset.version` and `image.tag` here (0.1.14), `casfs.daemonset.version` in `abfs-service-module/abfs/values.yaml`, and the client package versions in the builder chart (`values-abfs.yaml`) and Jenkins seed.

### COS variant

`gitops/modules/abfs-client-module-cos` is the same module for Container-Optimized OS nodes that ship casfs in the image (`casfs.provider=image`, no DaemonSet). It is not in the module catalog: it requires GKE ≥ 1.36 with COS 129 LTS carrying casfs 0.1.14 and has not been tested.

## Validate after enable

1. In the Developer Portal wait until the module state changes from "NOT INSTALLED" to "READY".
2. In Argo CD wait until `mod-abfs-client-ubuntu` and `mod-abfs-client-ubuntu-node-pool` are Synced/Healthy.
3. Command-line checks:

```bash
kubectl get application -n argocd | grep abfs-client-ubuntu
# expect: mod-abfs-client-ubuntu, mod-abfs-client-ubuntu-node-pool — Synced/Healthy
kubectl get containernodepool abfs-build-node-pool-ubuntu -n abfs-client-module-ubuntu-overview
gcloud container node-pools describe abfs-build-node-pool-ubuntu --cluster <cluster> --region <region> \
  --format='value(status,autoscaling.maxNodeCount,config.shieldedInstanceConfig.enableSecureBoot)'
# expect: RUNNING 20 False
kubectl get daemonset abfs-casfs-installer-client -n module-manager
# expect: 0/0 until a build scales the pool

# During a build
kubectl get nodes -l workloadLabel=android-abfs-build
kubectl logs -n module-manager -l app.kubernetes.io/component=casfs-installer -c casfs-installer --tail=5
```

Then submit a build: Developer Portal → `workloads-android` → Workflow Templates → `aaos-builder-abfs` (see [ABFS Builder](../../../docs/workloads/android/builds/aaos_abfs_builder.md)).

## After disable

The DaemonSet, overview and the `ContainerNodePool` CR are pruned, and Config Connector deletes the GKE pool with the CR. Running builds on the pool are killed. Two hook Jobs of the node-pool Application (namespace `workflows`) remove with `gcloud` a pool that Config Connector did not delete:

- PostDelete `abfs-build-node-pool-ubuntu-orphan-postdelete`: deletes the GKE pool after the disable if it is still there. It never fails and stops waiting after 15 minutes.
- PreSync `abfs-build-node-pool-ubuntu-orphan-presync`: on enable, deletes a pool that exists without a `ContainerNodePool`, so the sync creates a fresh one. A pool whose CR carries the cluster name in `clusterRef.external` is never touched.
- Upgrade: a CR written by an earlier release has the full cluster path (`projects/.../clusters/<name>`) in `clusterRef.external`. Config Connector looked up a wrong pool through it (`UpdateFailed ... already exists`, GKE pool left behind on disable) and the field cannot be changed. On the first sync after the upgrade the PreSync Job deletes that CR and its pool once and the sync creates both again; running builds on the pool are killed.

Both delete only a pool that carries the node label `workloadLabel: android-abfs-build`. A pool with the same name without that label is left alone and the enable stops with `does not carry workloadLabel`: rename or delete that pool by hand. When `gcloud` or `kubectl` fails, the PreSync Job stops with `ERROR: cannot read ...` and deletes nothing; check `kubectl logs -n workflows job/abfs-build-node-pool-ubuntu-orphan-presync`, fix the cause and sync `mod-abfs-client-ubuntu-node-pool` again.
