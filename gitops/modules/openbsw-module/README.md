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
| Child Application `mod-openbsw-module-argo-workflows` | `workflows` | Syncs the `argo-workflows` chart: WorkflowTemplates `openbsw-bsw-builder` and `openbsw-builder-image` (in `workflows`) and their webhook Sensors (in `argo-events`) |
| NetworkPolicies `default-deny-all`, `allow-dns` | `openbsw` | Baseline perimeter when `config.enableNetworkPolicies` is true |
| ServiceAccount `openbsw-can-kernel` | `openbsw` | Identity for the CAN DaemonSet. No Role or binding |
| DaemonSet `openbsw-can-kernel` | `openbsw` | Loads `can`, `can-raw`, and `vcan` on the OpenBSW node pool |
| Overview Deployment and Service `mod-openbsw-module-overview` | `openbsw-overview` | Portal overview page served on port 80 |

Module Manager injects `moduleName`, `config`, `repo.url`, and `repo.revision` when the module is enabled. `moduleName` stays empty in `values.yaml`.

## Workflows

The `argo-workflows/` chart ports the two OpenBSW Jenkins jobs. Both WorkflowTemplates carry `horizon-sdv.io/expose: "true"` and `horizon-sdv.io/module: openbsw-module`, so they appear in the Developer Portal and Horizon CLI. Each has a Sensor on EventSource `webhook`, event `workflow-dispatch`, matching `body.workflowTemplateName`.

| WorkflowTemplate | Sensor | Jenkins job | Notes |
|------------------|--------|-------------|-------|
| `openbsw-bsw-builder` | `webhook-openbsw-bsw-builder` | OpenBSW Workflows/Builds/BSW Builder | Build, AI review on failure, GCS storage, optional retention |
| `openbsw-builder-image` | `webhook-openbsw-builder-image` | OpenBSW Workflows/Environment/Docker Image Template | Calls ClusterWorkflowTemplate `common-docker-image-build` (workloads-common) |

Parameters use the Jenkins names in camelCase with the same defaults. They are defined once, in order, under `bswBuilder.parameters` and `builderImage.parameters` in `argo-workflows/values.yaml`. The WorkflowTemplate arguments and the Sensor mappings are both generated from those lists. Job details are in the [BSW Builder](../../../workloads/openbsw/pipelines/builds/bsw_builder/README.md) and [Docker Image Template](../../../workloads/openbsw/pipelines/environment/docker_image_template/README.md) READMEs.

`openbsw-bsw-builder` runs these steps:

1. `prepare-pipeline-git-creds` (when `config.scm.authMethod` is `app` or `userpass`).
2. `check-image`, then `build-image` (via `openbsw-builder-image`) when the `imageTag` image is missing or `forceImageBuild` is `true`.
3. `fetch-pipeline`: copies the Horizon repo (`config.workloads.android.url` / `branch`) onto the per-workflow volume `bsw-workspace` (`standard-rwo`, 50Gi, deleted on completion).
4. `build`: `bsw_initialise.sh`, then each Jenkins stage via `bsw_build.sh` in one pod, as user `builder`.
5. `ai-review`: on build failure when `enableGeminiAiAssistant` is `true`. Runs `workloads/common/agentic-ai/gemini/run_ai_review.sh` with `AI_REVIEW_RUNTIME=aiReviewRuntime` (default `agy`, the Google Antigravity SDK, as for the AAOS builder). Skill: `workloads/common/agentic-ai/antigravity/skills/openbsw/SKILL.md`.
6. `storage`: `bsw_storage.sh` to `gs://<project>-openbsw/OpenBSW/Builds/BSW_Builder/<date>-<time>_<workflow>` unless `storageBucketDestination` is set.
7. `retain`: keeps a pod with the workspace for `instanceRetentionTime` minutes.
8. `fail-if-build-failed`: marks the workflow Failed after storage and AI review.

Workflow pods run as `workflow-executor-elevated` (Artifact Registry push, GCS upload, Vertex AI) while `useElevatedWorkflowIam` is `true`, otherwise as `serviceAccountName`. They read the `horizon-workflow-cloud-env` ConfigMap, as in the AAOS builder.

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

Workflow checks:

```bash
kubectl -n workflows get workflowtemplates -l horizon-sdv.io/module=openbsw-module
# expect: openbsw-bsw-builder, openbsw-builder-image

kubectl -n argo-events get sensors webhook-openbsw-bsw-builder webhook-openbsw-builder-image
# expect: both listed

horizon workflow submit --module openbsw-module --template openbsw-bsw-builder
# expect: a workflow named openbsw-bsw-builder-*. The first run also builds the argowf-latest image
```

## After disable

Disabling the module deletes the parent Application `mod-openbsw-module`. Argo CD then prunes the child `mod-openbsw-module-argo-workflows` with its WorkflowTemplates and Sensors, the `openbsw` namespace with the CAN DaemonSet, its ServiceAccount, the workflow-scheduling ConfigMap and the NetworkPolicies, and the `openbsw-overview` namespace.

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

If the `argo` CLI is installed, the smoke test also covers the workflows, with `git.authMethod` set to `app` and to empty:

- `argo lint --offline` on both WorkflowTemplates and the ClusterWorkflowTemplates they call (`common-docker-image-build`, `prepare-pipeline-git-creds`).
- A client-side `argo submit --dry-run --from workflowtemplate/<name>` for both templates. It talks to a local fake Kubernetes API that serves the rendered templates, so it needs no cluster and does not touch your kube context.
- A simulated webhook event per Sensor. Each Sensor must fire on its own `workflowTemplateName` and on no other (including `<name>-extra` and `x<name>`). The Workflow it would create is then dry-run submitted, and body fields must land in the matching parameters.

Without the `argo` CLI those checks print `SKIPPED`.

Enable creates the parent Application `mod-openbsw-module`. Disable deletes that parent. The child Application is part of the parent chart, so the parent finalizer prunes it. Those two steps are covered by the Module Manager unit tests `TestEnableOpenBSW_CreatesParentApplication` and `TestPerformModuleDisable_OpenBSWDeletesParentApplication`.
