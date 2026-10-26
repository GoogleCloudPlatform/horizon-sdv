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

# Common Docker Image Workflow

Reusable ClusterWorkflowTemplate for Docker image builds. Use it to build images from
different directories by overriding `dockerfileDir` and `buildArgs` at submit
time. When using a local repo PVC, pass absolute paths (e.g., `/workspace-local/...`).

This template **does not clone Git**; it expects a **`source`** input artifact at **`/workspace`** (or a local mount). Callers such as **aaos-builder** or **aaos-builder-runtime-image** supply the git artifact (or mount). No **`pipelineRepoUrl`** / credentials belong in this chart.

Thin wrapper charts are provided for convenience:
- `workloads/android/pipelines/environment/docker_image_template/helm`
- `workloads/android/pipelines/environment/abfs/docker_image_template/helm`
- `workloads/android/pipelines/environment/abfs/docker_infra_template/helm`
- `workloads/android/pipelines/environment/mirror/docker_image_template/helm`
- `workloads/openbsw/pipelines/environment/docker_image_template/helm`

## What’s in this chart

- `Chart.yaml`: Helm chart metadata.
- `values.yaml`: Default values; update for configuration.
- `templates/clusterworkflowtemplates.yaml`: ClusterWorkflowTemplate for shared builds.

## Deploy

```bash
helm template common-docker-image-build \
  workloads/common/common_docker_image/helm \
  | kubectl apply -f -
```

## Run (example overrides)

Example: build from different docker image template paths.

```bash
argo submit --from clusterworkflowtemplate/common-docker-image-build -n workflows \
  -p dockerfileDir=workloads/android/pipelines/environment/docker_image_template \
```

```bash
argo submit --from clusterworkflowtemplate/common-docker-image-build -n workflows \
  -p dockerfileDir=workloads/android/pipelines/environment/abfs/docker_image_template \
```

```bash
argo submit --from clusterworkflowtemplate/common-docker-image-build -n workflows \
  -p dockerfileDir=workloads/android/pipelines/environment/abfs/docker_infra_template \
```

```bash
argo submit --from clusterworkflowtemplate/common-docker-image-build -n workflows \
  -p dockerfileDir=workloads/android/pipelines/environment/mirror/docker_image_template \
```

```bash
argo submit --from clusterworkflowtemplate/common-docker-image-build -n workflows \
  -p dockerfileDir=workloads/openbsw/pipelines/environment/docker_image_template \
```

## Local repo on GKE (PVC-backed)

If you sync a local repo into a PVC, set:

```yaml
localRepoPvcName: "workloads-repo-pvc"
localRepoMountPath: "/workspace-local"
```

Then pass relative paths in `dockerfileDir`. The template automatically
prefixes them with `localRepoMountPath`.

## Common Options (values.yaml)

- `clusterWorkflowTemplateName`: ClusterWorkflowTemplate name (default: `common-docker-image-build`)
- `localRepoHostPath`: hostPath for local clusters (empty on GKE)
- `localRepoPvcName`: PVC for local repo on GKE
- `localRepoMountPath`: Mount path for local repo
- `spec.dockerCredentialsUrl`: URL or local path to docker-credential-gcr (supports https://, file://, or absolute path); empty uses `defaults.dockerCredentialsUrl`
- `defaults.dockerCredentialsUrl`: Fallback URL when `spec.dockerCredentialsUrl` is empty

GCP project, region, image path, and git clone are **not** configured here; callers (e.g. aaos-builder-runtime-image) pass parameters and supply **`/workspace`**.

**Platform GitOps:** Module Manager module **`workloads-common`** deploys an Argo CD child Application (**`gitops/modules/workloads-common`**) whose **`spec.sources`** include this chart. **ClusterWorkflowTemplate** resources use **`argocd.argoproj.io/sync-wave: "7"`** so they apply before dependent **WorkflowTemplates** (e.g. under **`workloads-android`**, wave **3**). Enable **`workloads-common`** before **`workloads-android`** (hard dependency).

## Same cluster as GitOps: pause sync, local `helm apply`, resync

If Argo CD manages this ClusterWorkflowTemplate, a manual `helm template | kubectl apply` (for example with **`-f values-local.yaml`**) can be **reverted** when Argo syncs from Git. To test local manifests without losing them immediately:

1. **Find the Application** (with GitOps **`namespacePrefix`**, e.g. **`dev-workflows-common-cluster-templates`** in **`argocd`**):

   ```bash
   kubectl get applications -A | grep workloads-common
   ```

2. **Pause automated sync** (replace `APP_NAME` and `ARGOCD_NS` with values from step 1):

   ```bash
   kubectl patch application APP_NAME \
     -n ARGOCD_NS \
     --type=json \
     -p='[{"op":"remove","path":"/spec/syncPolicy/automated"}]'
   ```

3. **Apply** your chart (example with local PVC overrides):

   ```bash
   helm template common-docker-image-build \
     workloads/common/common_docker_image/helm \
     -f workloads/common/common_docker_image/helm/values-local.yaml \
     --show-only templates/clusterworkflowtemplates.yaml \
     | kubectl apply -f -
   ```

4. **Resync from Git** when you want the cluster to match the repo again (one-off):

   ```bash
   argocd app sync APP_NAME
   ```

   Or use the Argo CD UI **Sync** button for that Application.

5. **Re-enable continuous sync** (optional):

   ```bash
   kubectl patch application APP_NAME \
     -n ARGOCD_NS \
     --type=merge \
     -p '{"spec":{"syncPolicy":{"automated":{}}}}'
   ```

While automated sync is off, avoid running a **Sync** on **`workloads-common`** if you still want to keep hand-applied YAML; a sync reapplies Git and overwrites chart sources for that Application.
