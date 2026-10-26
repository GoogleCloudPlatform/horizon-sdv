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

# Docker Image Template

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)
- [Argo WorkflowTemplate](#argo-workflowtemplate)
- [GitOps](#gitops)
- [Developer Portal](#developer-portal)

## Introduction <a name="introduction"></a>

This pipeline builds the container image used on Kubernetes for building use with Android Build File System(ABFS) targets and miscellaneous environment pipelines.

Enable **Ubuntu** and/or **COS** templates in one run (`ENABLE_UBUNTU_TEMPLATE` / `ENABLE_COS_TEMPLATE`). Jenkins defaults: Ubuntu `latest` (used by AAOS ABFS Builder), COS `cos-latest`. Argo Workflow defaults: Ubuntu `argowf-latest`, COS `argowf-cos-latest`.

This need only be run once, or when Dockerfile is updated. There is an option not to push the resulting image to the registry, so that devs can test their changes before committing the image.

### References
- [buildkit](https://hub.docker.com/r/moby/buildkit)

## Prerequisites<a name="prerequisites"></a>

This depends only on [`buildkit`](https://hub.docker.com/r/moby/buildkit) which should be installed by default.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `NO_PUSH`

Build the container image but don't push to the registry.

### `ENABLE_UBUNTU_TEMPLATE`

When true, build the **Ubuntu** ABFS builder image (for UBUNTU_CONTAINERD ABFS client pools). Use this path when casfs is compiled on the node.

### `IMAGE_TAG_UBUNTU`

This is the tag that will be applied when the container image for **Ubuntu** is pushed to the registry. For the current release we use `latest` because all pipelines that depend on this container image are using `latest`.

### `LINUX_DISTRIBUTION`

Define the Linux Distribution to create the Docker image from. Values must be supported by the Dockerfile FROM instruction.

### `ENABLE_COS_TEMPLATE`

When true, build the **COS** ABFS builder image (for the COS / embedded-CASFS client path when GKE ≥ 1.36 and COS 129 LTS are available).

### `IMAGE_TAG_COS`

This is the tag that will be applied when the container image for **COS** is pushed to the registry. The tag added for the image is `cos-latest` so it does not overwrite Ubuntu image tag `latest`.

### `COS_LINUX_DISTRIBUTION`

Define the Linux Distribution to create the Docker image from. Docker `FROM` for the COS-tagged builder **container** (GKE pod userspace). COS node images are not Docker Hub bases; default remains an Ubuntu/Debian image unless you supply another supported base.

### `ABFS_REPOSITORY`

ABFS Artifact Registry apt repository name from **Seed Workloads** (for example `abfs-apt-alpha-public`).
When `ABFS_DISTRIBUTION_REGISTRY` is empty, the pipeline composes:

`ar+https://us-apt.pkg.dev/projects/abfs-binaries <ABFS_REPOSITORY> main`

Update Seed `ABFS_REPOSITORY` when Google publishes a new ABFS apt channel, then re-seed and rebuild this image.

### `ABFS_VERSION`

ABFS client version from Seed (informational on this job). The image enables ABFS apt transport and installs `kmod`;
`abfs-client` / CASFS packages are installed later by **AAOS ABFS Builder** using Seed `ABFS_VERSION` / `ABFS_CASFS_VERSION`.

### `GOOGLE_DISTRIBUTION_REGISTRY`

Defines the artifact registry required for ABFS package installation (apt-transport-artifact-registry).

### `ABFS_DISTRIBUTION_REGISTRY`

Optional full ABFS apt sources line. Leave empty to compose from `ABFS_REPOSITORY` (recommended).
Set only when you need a non-default registry URL.

### `NODEJS_VERSION`

The version of NodeJS to install which is required by MTK Connect.

### `PACKER_VERSION`

Version of [Packer](https://github.com/hashicorp/packer/releases) installed in the shared Android/ABFS Dockerfile
(same as Android Docker Image Template). Default `1.15.0`.

### `BUILDKIT_RELEASE_TAG`

The version of Buildkit to use to build the container image.

### `DOCKER_CREDENTIALS_URL`

URL of Google docker credentials helper, required to allow access to the project artifact registry.

### `GCLOUD_CLI_VERSION`

Version of [Google Cloud CLI](https://docs.cloud.google.com/sdk/docs/release-notes) to install.
Define `latest` if wishing to use the latest available version.

### `KUBECTL_VERSION`

Version of `kubectl` to install. The version is typically `1:${GCLOUD_CLI_VERSION}`.
Define `latest` if wishing to use the latest available version.

### `ENABLE_GEMINI_AI_ASSISTANT`

Enable Gemini AI to support in diagnosis of build and test failures.

### `GEMINI_CLI_VERSION`

The version of gemini-cli to be installed.
Run `npm view @google/gemini-cli versions` for a full list of valid versions.

### `ANTIGRAVITY_SDK_VERSION`

The version of antigravity SDK to be installed.
Run `pip index versions google-antigravity` for a full list of valid versions.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `ABFS_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `HORIZON_SCM_URL`
    - The URL to the Horizon SDV git repository.

-   `HORIZON_SCM_BRANCH`
    - The branch name the job will be configured for from `HORIZON_SCM_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## Argo WorkflowTemplate <a name="argo-workflowtemplate"></a>

Helm chart at `workloads/android/pipelines/environment/abfs/docker_image_template/helm` installs
WorkflowTemplate `abfs-builder-runtime-image` and Sensor `webhook-abfs-builder-runtime-image`
so the Developer Portal can submit the same template as Argo UI / CLI.

It mirrors the Jenkins job: shared Android Dockerfile under
`workloads/android/pipelines/environment/docker_image_template` with `ABFS=true`, apt registry
build-args, and Packer. `templateFlavor` selects the Argo default registry tag (`ubuntu` →
`argowf-latest`, `cos` → `argowf-cos-latest`). Jenkins uses `latest` / `cos-latest`. Docker
`FROM` is `linuxDistribution` (Ubuntu/Debian userspace for both). `templateFlavor=ubuntu`
with a COS tag fails. Portal sends both fields; `cos` with the Ubuntu default tag is
rewritten to `argowf-cos-latest`. There is not a second Dockerfile.

**GCP project / region / zone:** chart `values.yaml` leaves these empty. When
**workloads-android** is enabled via Module Manager, the child Argo Application injects
`spec.cloudProject` / `cloudRegion` / `cloudZone` from root `MODULE_CONFIG`
(`config.projectID` / `region` / `zone`), which Terraform sets from `gcp_project_id` /
`gcp_cloud_region` / `gcp_cloud_zone` (via `container-deploy` → `sdv-gke-apps`).
Manual helm must `--set` them.

See chart [README](../../../../../workloads/android/pipelines/environment/abfs/docker_image_template/helm/README.md)
for `helm template` / `argo submit` examples.

## GitOps <a name="gitops"></a>

This wires the ABFS Helm chart into the **workloads-android** Argo CD Application so Module Manager
enable deploys it automatically (no manual `kubectl apply`).

Source entry:
`gitops/modules/workloads-android/templates/application-workloads-android.yaml`
→ path `workloads/android/pipelines/environment/abfs/docker_image_template/helm`
→ releaseName `workloads-android-abfs-builder-runtime-image`

**Verify on cluster**

1. Enable **workloads-common**, then **workloads-android** in Module Manager.
2. Confirm the child Application lists the ABFS Helm source and is Synced.
3. `kubectl get workflowtemplate abfs-builder-runtime-image -n workflows` (or `{prefix}workflows`).
4. Submit from Argo Workflows UI / CLI **or** the Developer Portal (below); cloud project/region/zone come from `MODULE_CONFIG` (do not set them in the submit form).

## Developer Portal <a name="developer-portal"></a>

Same pattern as the Android Docker Image Template (`aaos-builder-runtime-image`): the WorkflowTemplate
is labeled `horizon-sdv.io/expose: "true"` on **workloads-android**. After that module is READY:

1. Open **workloads-android** → **Workflow Templates**.
2. `abfs-builder-runtime-image` → **Configure & submit** (same generic parameter form as
   `aaos-builder-runtime-image`; no portal image rebuild).
3. Review the form (defaults come from the WorkflowTemplate):
   - `NO_PUSH`: default `true` (build only, same as Jenkins). Set `false` to push.
   - `templateFlavor`: `ubuntu` (tag `argowf-latest`) or `cos` (tag `argowf-cos-latest`).
   - `imageTag`: leave the Ubuntu default when flavor is `cos`; the workflow rewrites it to `argowf-cos-latest`. Do not use `templateFlavor=ubuntu` with a COS tag (`cos-*` / `argowf-cos-*`).
   - ABFS apt: `abfsRepository`, `abfsDistributionRegistry`, `googleDistributionRegistry`.
4. Submit. Horizon posts to the webhook Sensor; the run appears under **Running Workflows**.
