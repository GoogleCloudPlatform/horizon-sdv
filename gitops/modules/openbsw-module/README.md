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

# OpenBSW

Helm chart: `gitops/modules/openbsw-module`

The module runs on the main environment only. It is not deployed on a sub-environment, because the OpenBSW node pool is shared by the cluster.

## Module Metadata

| Metadata Field | Value |
| :--- | :--- |
| **OWNER** | Accenture |
| **CONTACT** | [horizon-sdv@accenture.com](mailto:horizon-sdv@accenture.com) |
| **CHART VERSION** | 0.1.0 |
| **CONTAINS_COMMERCIAL** | no |

## What this chart deploys

| Resource | Namespace | Purpose |
|----------|-----------|---------|
| Namespace `openbsw` | — | Module-owned namespace. Annotated with `horizon-sdv.io/node-pool` |
| ConfigMap `openbsw-workflow-scheduling` | `openbsw` | Node selector and tolerations for OpenBSW workflow pods |
| Child Application `mod-openbsw-module-argo-workflows` | `workflows` | Placeholder. Syncs the `argo-workflows` chart, which renders no resources until WorkflowTemplates are added (follow-up ticket) |
| NetworkPolicies `default-deny-all`, `allow-dns` | `openbsw` | Baseline perimeter when `config.enableNetworkPolicies` is true |
| ServiceAccount `openbsw-can-kernel` | `openbsw` | Identity for the CAN DaemonSet. No Role or binding |
| DaemonSet `openbsw-can-kernel` | `openbsw` | Loads `can`, `can-raw`, and `vcan` on the OpenBSW node pool |
| Overview Deployment and Service `mod-openbsw-module-overview` | `openbsw-overview` | Portal overview page served on port 80 |

Module Manager injects `moduleName`, `config`, `repo.url`, and `repo.revision` when the module is enabled. `moduleName` stays empty in `values.yaml`.

## Status: placeholder workflows

The `argo-workflows/` chart has no WorkflowTemplates yet. Its child Application `mod-openbsw-module-argo-workflows` therefore syncs an empty chart, and enabling the module creates no workflows. Adding the OpenBSW WorkflowTemplates and their Sensors is tracked as a follow-up ticket. The scheduling helper below is ready for those templates but is not used by any template yet.

## Scheduling

WorkflowTemplates stay in the platform `workflows` namespace, which Horizon API and the Argo controller watch. Their pods schedule onto the existing Terraform pool `sdv-openbsw-build-node-pool` (`workloadLabel: openbsw`, taint `workloadType=openbsw:NoSchedule`). This module does not create that pool and does not change its labels or taint.

Each WorkflowTemplate in `argo-workflows/` sets:

```yaml
spec:
  templateDefaults:
    metadata:
      annotations:
        cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
      labels:
        horizon-sdv.io/openbsw-workflow: "true"
    nodeSelector:
      workloadLabel: openbsw
      cloud.google.com/gke-nodepool: sdv-openbsw-build-node-pool
    tolerations:
      - key: workloadType
        operator: Equal
        value: openbsw
        effect: NoSchedule
```

`argo-workflows/templates/_helpers.tpl` defines `openbsw.templateDefaults` for that block. The same selector and tolerations are in the `openbsw-workflow-scheduling` ConfigMap.

## CAN kernel

DaemonSet `openbsw-can-kernel` loads `can`, `can-raw`, and `vcan` on each node of `sdv-openbsw-build-node-pool`. It uses `nsenter` into the host mount and PID namespace and then `modprobe`. It does not install packages.

The container stays up only after `/proc/modules` shows each module (`can-raw` is listed as `can_raw`). A failed `modprobe` restarts the pod.

### Security and RBAC

| Control | Setting |
|---------|---------|
| ServiceAccount | `openbsw-can-kernel` |
| API token | not mounted (`automountServiceAccountToken: false`) |
| Role, ClusterRole, bindings | none. The process does not call the Kubernetes API |
| `privileged` | `true`. Inserting a kernel module needs `CAP_SYS_MODULE` |
| `hostPID` | `true`. `nsenter` has to see the host's PID 1 |
| `hostNetwork` | not set |
| `hostPath` | none |
| Root filesystem | read-only |
| Namespace policy | `openbsw` enforces the privileged Pod Security profile. `openbsw-overview` does not |

### Node labeling

Terraform already labels and taints the pool. This module does not change that.

| Pool (`sdv-openbsw-build-node-pool`) | DaemonSet |
|--------------------------------------|-----------|
| Label `workloadLabel=openbsw` | `nodeSelector` `workloadLabel: openbsw` |
| Label `cloud.google.com/gke-nodepool=sdv-openbsw-build-node-pool` (set by GKE) | `nodeSelector` on that pool name |
| Taint `workloadType=openbsw:NoSchedule` | matching toleration |

The pod does not use the Jenkins label `openbsw_pod`.

## Jenkins compatibility

Existing OpenBSW Jenkins jobs stay in place and keep working. This module does not change them.

The jobs live under `workloads/openbsw/pipelines/` and run on Jenkins agents in the `jenkins` namespace. Their CAN DaemonSet stays in `gitops/workloads/openbsw/templates/openbsw-host-module.yaml`, in the `jenkins` namespace, using `jenkins-sa`.

The module DaemonSet is a separate object in `openbsw`. It selects the same pool, so both load `can`, `can-raw`, and `vcan` there. `modprobe` of a module that is already loaded succeeds. The module DaemonSet does not use `jenkins-sa` or the label `openbsw_pod`.

The `openbsw` namespace and its NetworkPolicies apply only to pods in that namespace. Jenkins pods are outside it.

## Catalog

Registered in `gitops/apps/module-manager/templates/module-catalog.yaml`. Enabling this module enables **workloads-common** first, and workloads-common cannot be disabled while this module is on.

| Setting | Value |
|---------|-------|
| Module name | `openbsw-module` |
| Path | `gitops/modules/openbsw-module` |
| Overview service | `mod-openbsw-module-overview` |
| Overview namespace | `openbsw-overview` |
| Hard dependencies | `workloads-common` |

## Validate after enable

1. In the Developer Portal, wait until the module state changes from NOT INSTALLED to READY. Enable `workloads-common` first if it is not already on.
2. In Argo CD, wait until `mod-openbsw-module` and `mod-openbsw-module-argo-workflows` are Synced/Healthy.
3. Command-line checks:

```bash
kubectl -n argocd get applications -l horizon-sdv.io/module=openbsw-module
# expect: mod-openbsw-module, mod-openbsw-module-argo-workflows, Synced/Healthy

kubectl -n openbsw get ds,pods -o wide
# expect: DaemonSet openbsw-can-kernel. DESIRED is 0 until the pool has a node

kubectl get nodes -l cloud.google.com/gke-nodepool=sdv-openbsw-build-node-pool
# expect: one row per node in the OpenBSW pool

kubectl -n openbsw logs ds/openbsw-can-kernel --tail=20
# expect: no modprobe error. CrashLoopBackOff means modprobe or the /proc/modules check failed

kubectl -n openbsw exec ds/openbsw-can-kernel -- nsenter -t 1 -m -p -- lsmod | grep -E '^(can|can_raw|vcan) '
# expect: can, can_raw and vcan listed
```

The `argo-workflows` child is a placeholder, so no WorkflowTemplates appear in the Developer Portal yet (see [Status: placeholder workflows](#status-placeholder-workflows)).

## After disable

Disabling the module deletes the parent Application `mod-openbsw-module`. Argo CD then prunes the child `mod-openbsw-module-argo-workflows`, the `openbsw` namespace with the CAN DaemonSet, its ServiceAccount, the workflow-scheduling ConfigMap and the NetworkPolicies, and the `openbsw-overview` namespace.

```bash
kubectl -n argocd get applications -l horizon-sdv.io/module=openbsw-module
# expect: no results

kubectl get ns openbsw openbsw-overview
# expect: NotFound for both
```

- The `can`, `can_raw` and `vcan` kernel modules stay loaded on the nodes. The DaemonSet does not unload them.
- The Terraform node pool `sdv-openbsw-build-node-pool` is not removed. The Jenkins CAN DaemonSet in `jenkins` is not touched.
- Nothing depends on this module, so it can be disabled while `workloads-common` stays enabled.

## Tests

```bash
python3 gitops/modules/openbsw-module/tests/smoke_test.py
```

The smoke test lints both charts, renders them with `helm template`, checks the `openbsw-module` ModuleCatalog fields, and checks that the child Application is rendered with `prune: true` and the Argo CD resources finalizer. It is not part of CI.

Enable creates the parent Application `mod-openbsw-module`. Disable deletes that parent. The child Application is part of the parent chart, so the parent finalizer prunes it. Those two steps are covered by the Module Manager unit tests `TestEnableOpenBSW_CreatesParentApplication` and `TestPerformModuleDisable_OpenBSWDeletesParentApplication`.
