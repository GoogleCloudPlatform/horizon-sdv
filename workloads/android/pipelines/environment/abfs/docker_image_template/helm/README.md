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

# ABFS Builder Image Helm Chart

This chart installs the **ABFS builder image** Argo `WorkflowTemplate` (`abfs-builder-runtime-image`)
and calls the shared ClusterWorkflowTemplate `common-docker-image-build` (from **workloads-common**).

It builds the same shared Android Dockerfile used by the Jenkins ABFS Docker Image Template, with
`ABFS=true` and ABFS apt registry build-args. Ubuntu vs COS is selected by `templateFlavor`
(`ubuntu` → Argo tag `argowf-latest`, `cos` → Argo tag `argowf-cos-latest`). Jenkins uses
`latest` / `cos-latest`. `linuxDistribution` is Docker `FROM` (Ubuntu/Debian userspace for both).
The workflow rejects `templateFlavor=ubuntu` with a COS image tag. Portal always sends both
fields; `cos` plus the Ubuntu default tag is rewritten to `argowf-cos-latest`.

**GCP project / region / zone:** `values.yaml` leaves `spec.cloudProject` / `cloudRegion` /
`cloudZone` empty. Platform fills them:

1. Terraform (`container-deploy` → `sdv-gke-apps`) sets root Argo app `config.projectID` /
   `region` / `zone` from `gcp_project_id` / `gcp_cloud_region` / `gcp_cloud_zone`
2. Module Manager loads that as `MODULE_CONFIG` and merges into `mod-workloads-android`
3. Child Application `workloads-android` Helm-overrides this chart
   (`gitops/modules/workloads-android/templates/application-workloads-android.yaml`)

Enable **workloads-android** (and **workloads-common**) in Module Manager — no manual
`--set` for cloud identity on platform clusters. For local `helm template`, `--set` is required
(chart fails render if they are empty).

## GitOps deploy (platform)

On a Horizon cluster, do **not** use the manual Deploy section below as the primary path.
Enable Module Manager modules:

1. **workloads-common** (provides `common-docker-image-build`)
2. **workloads-android** (deploys this chart via multi-source Application)

Wiring: `gitops/modules/workloads-android/templates/application-workloads-android.yaml`

```text
path: workloads/android/pipelines/environment/abfs/docker_image_template/helm
releaseName: workloads-android-abfs-builder-runtime-image
spec.cloudProject/Region/Zone ← config.projectID/region/zone (MODULE_CONFIG)
```

Verify:

```bash
kubectl get application -n argocd | grep workloads-android
kubectl get workflowtemplate abfs-builder-runtime-image -n workflows
kubectl get sensor webhook-abfs-builder-runtime-image -n argo-events
```

Then submit as in **Run** below (cloud* already baked into the WorkflowTemplate).

## Prerequisites

- Argo Workflows in the cluster
- `common-docker-image-build` ClusterWorkflowTemplate (enable **workloads-common**)
- For cluster deploy: enable **workloads-android** (Module Manager), or manual `helm template | kubectl apply`
- `kubectl` / `helm`

## Deploy (manual / Stage 3 only)

```bash
helm template abfs-builder-runtime-image \
  workloads/android/pipelines/environment/abfs/docker_image_template/helm \
  --set spec.cloudProject=$CLOUD_PROJECT \
  --set spec.cloudRegion=$CLOUD_REGION \
  --set spec.cloudZone=$CLOUD_ZONE \
  --set spec.pipelineRepoUrl=$HORIZON_SCM_URL \
  --set spec.pipelineRepoRevision=$HORIZON_SCM_BRANCH \
  --set git.authMethod=userpass \
  | kubectl apply -f -
```

## Run

`NO_PUSH` defaults to `true` (same as Jenkins). Omit it to build without pushing; pass `NO_PUSH=false` to push.

Ubuntu-style tag (does not overwrite Jenkins `:latest` by default):

```bash
argo submit --from workflowtemplate/abfs-builder-runtime-image -n workflows \
  -p templateFlavor=ubuntu \
  -p imageTag=argowf-latest
```

COS-labeled tag:

```bash
argo submit --from workflowtemplate/abfs-builder-runtime-image -n workflows \
  -p templateFlavor=cos \
  -p imageTag=argowf-cos-latest
```

Push:

```bash
argo submit --from workflowtemplate/abfs-builder-runtime-image -n workflows \
  -p templateFlavor=ubuntu \
  -p imageTag=argowf-latest \
  -p NO_PUSH=false
```

Image path: `<region>-docker.pkg.dev/<project>/horizon-sdv/aaos-abfs-builder:<tag>`

## Developer Portal

After **workloads-android** is enabled, open **Workflow Templates** → `abfs-builder-runtime-image`
(same generic submit form as `aaos-builder-runtime-image`; no portal image rebuild). Horizon posts
to Sensor `webhook-abfs-builder-runtime-image`. Set:

- `NO_PUSH` (default `true` — build only. Set `false` to push)
- `templateFlavor` (`ubuntu` or `cos`) — selects the default registry tag
- `imageTag` — optional override; leave the Ubuntu default with `templateFlavor=cos` and the workflow uses `argowf-cos-latest`
- ABFS registry fields (`abfsRepository`, `abfsDistributionRegistry`, `googleDistributionRegistry`)
- `googleAdkVersion` (same pin as AAOS Docker Image Template / Dockerfile `GOOGLE_ADK_VERSION`)

## What’s in this chart

- `Chart.yaml`: Helm chart metadata.
- `values.yaml` / `values-local.yaml`: Default values and local PVC overrides.
- `templates/workflow/workflowtemplates.yaml`: Argo WorkflowTemplate wrapper for image builds.
- `templates/workflow/_templates.tpl`: Aggregates all split workflow templates.
- `templates/workflow/_resolve-flavor.tpl`: Maps `templateFlavor` to the Argo default tag; rejects ubuntu + COS tags.
- `templates/workflow/_build.tpl`: Build DAG templates (`build` and `build-defaults`) that `templateRef` **`common-docker-image-build`**.
- `templates/_helpers.tpl`: Namespace, service account, SCM auth, ABFS apt sources line, git artifact creds.
- `templates/workflow/sensors.yaml`: Argo Events Sensor `webhook-abfs-builder-runtime-image` (Developer Portal / Horizon API submit).
- `README.md`: This document.

## Why split templates

The workflow templates are split into per-step files to keep each task easy to find,
review, and maintain. The `templates/workflow/_templates.tpl` file aggregates them
in a stable order. Helm `define` names are `abfs-builder-runtime-image.*`.

## Local PVC repo

```bash
helm template abfs-builder-runtime-image \
  workloads/android/pipelines/environment/abfs/docker_image_template/helm \
  -f workloads/android/pipelines/environment/abfs/docker_image_template/helm/values-local.yaml \
  | kubectl apply -f -
```
