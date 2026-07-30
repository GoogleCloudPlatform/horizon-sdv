<!-- Copyright (c) 2026 The Qt Company Ltd., All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Squish Cloud Workstation Module

Helm chart path: `gitops/modules/squish-workstation`

Self-service [Google Cloud Workstations](https://cloud.google.com/workstations/docs/overview) with
[Squish](https://www.qt.io/quality-assurance/squish), the GUI test automation tool from The Qt Company,
preinstalled on the Horizon GNOME desktop. All GCP resources are provisioned declaratively with
Kubernetes Config Connector (KCC) when the module is enabled in the Developer Portal; no Terraform run is
needed per module.

## Table of contents

- [Components](#components)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Enabling the Module](#enabling-the-module)
- [Building the Squish Image](#building-the-squish-image)
- [Creating a Workstation](#creating-a-workstation)
- [Smoke Test](#smoke-test)
- [Configuration](#configuration)
- [Disabling the Module](#disabling-the-module)
- [Licensing](#licensing)

## Components

| Component | Name | Path |
| :--- | :--- | :--- |
| Child Application (KCC) | `mod-squish-workstation-squish-workstation-app` | `squish-workstation-app/` |
| Child Application (pipelines) | `mod-squish-workstation-argo-workflows` | `argo-workflows/` |
| Developer Portal overview | `mod-squish-workstation-overview` | `portal/overview.html` |
| Image build context | `squish` image | `image/` |

Config Connector resources (namespace `squish-workstation-app`), in Argo CD sync-wave order:

| Wave | Kind | Purpose |
| :--- | :--- | :--- |
| 1 | `ArtifactRegistryRepository` `squish-workstation` | Docker repository for the Squish image (abandoned on disable). |
| 1 | `IAMServiceAccount` `squish-workstation-vm` | Service account of the workstation VMs. |
| 2 | `IAMPolicyMember` (project level) | VM SA: `secretmanager.secretAccessor` (IAM condition: license secret only), `artifactregistry.reader`, `logging.logWriter`. Each entry of `squish.creators`: `workstations.workstationCreator` and `workstations.operationViewer`. |
| 3 | `WorkstationCluster` `squish-workstation-cluster` | Cluster on the platform VPC (`sdv-network` / `sdv-subnet`). |
| 4 | `WorkstationConfig` `squish-workstation-config` | Machine type, pool size, persistent home disk, Squish image. |

Argo Workflows (namespace `{namespacePrefix}workflows`, exposed in the Developer Portal and Horizon CLI):

| WorkflowTemplate | Sensor | Purpose |
| :--- | :--- | :--- |
| `squish-workstation-init` | `webhook-squish-workstation-init` | Checks prerequisites, then builds and pushes the Squish image with `common-docker-image-build`. |
| `squish-workstation-execute` | `webhook-squish-workstation-execute` | Smoke test: starts a test workstation and verifies the Squish license and installation. |

**Hard dependency:** `workloads-common` (provides the `common-docker-image-build` and
`prepare-pipeline-git-creds` ClusterWorkflowTemplates). It is enabled automatically.

There is no Gateway HTTPRoute: workstations are opened through their Cloud Workstations URL.

## Architecture

```text
horizon-preflight  →  horizon-gnome  →  squish            (image layers, Artifact Registry)
                       (Jenkins jobs)    (squish-workstation-init)

Module Manager → mod-squish-workstation
                 ├── mod-squish-workstation-squish-workstation-app  → KCC → GCP (AR, IAM, cluster, config)
                 ├── mod-squish-workstation-argo-workflows          → WorkflowTemplates + Sensors
                 └── mod-squish-workstation-overview                → Developer Portal overview
```

The Squish layer (`image/`) follows the Horizon GNOME hook model
(see [Workstation Images](../../../docs/workloads/cloud-workstations/workstation_images.md)):

- `post-install-hooks.d/01_install_squish.sh` (image build): downloads the Squish installer, verifies its
  SHA-256 checksum, reads the license key from Secret Manager, runs the unattended install into
  `/opt/qt/squish` and removes the license key and installer from the image layer.
- `etc/workstation-startup.d/200_setting-squish.sh` (each workstation start): reads the license key from
  Secret Manager into `~/.squish-license` (mode 600) and links `~/squish` to `/opt/qt/squish`.

## Prerequisites

1. **Squish license key in Secret Manager.** Request a license (or a free trial) from The Qt Group on the
   [Squish product page](https://www.qt.io/quality-assurance/squish), then store the key in the platform
   project. The default secret name is `squish-license-key` (`squish.licenseSecretName`):

   ```bash
   printf '%s' "${SQUISH_LICENSE_KEY}" | \
     gcloud secrets create squish-license-key \
       --project="${PROJECT_ID}" --replication-policy=automatic --data-file=-
   ```

   Never commit license keys to the repository. The `workflow-executor` service account (image build) already
   has project-level Secret Manager access; the workstation VM service account gets access through the module.

2. **Horizon GNOME base image.** The Squish image is layered on
   `<region>-docker.pkg.dev/<project>/horizon-sdv/cloud-ws-images/horizon-gnome:latest`. Publish it with the
   Jenkins job *Cloud Workstations > Workstation Images > Horizon GNOME* (or *Workstation Image Chain*), or
   pass another image as the `baseImage` workflow parameter.

3. **Config Connector IAM permissions.** The Config Connector service account `gke-config-connector-sa` needs
   `roles/resourcemanager.projectIamAdmin` to reconcile the `IAMPolicyMember` resources. It is granted in
   `terraform/env/main.tf` and applied with the regular Horizon platform deployment.

## Enabling the Module

1. Open the Developer Portal → **Administration** → **Modules**.
2. Enable **squish-workstation** (`workloads-common` is enabled automatically).
3. Wait for the Argo CD applications and KCC resources to become ready. Creating the workstation cluster
   takes about 20 minutes:

   ```bash
   kubectl get applications -n argocd | grep mod-squish-workstation
   kubectl get artifactregistryrepositories,iamserviceaccounts,iampolicymembers,workstationclusters,workstationconfigs \
     -n squish-workstation-app
   ```

## Building the Squish Image

Submit `squish-workstation-init` from the Developer Portal (module tab → **Workflow Templates**) or with the
Horizon CLI:

```bash
horizon workflow submit --module squish-workstation --template squish-workstation-init --output json
```

| Parameter | Default | Description |
| :--- | :--- | :--- |
| `baseImage` | `<region>-docker.pkg.dev/<project>/horizon-sdv/cloud-ws-images/horizon-gnome:latest` | Base image for the Squish layer. |
| `imageTag` | `latest` | Tag to push. The WorkstationConfig uses `squish.imageTag`. |
| `dryRun` | `false` | Build without pushing. |

Re-run the workflow to update Squish (change `SQUISH_DOWNLOAD_URL` and `SQUISH_SHA256` in `image/Dockerfile`)
or to pick up a new base image. Running workstations use the new image after their next start.

## Creating a Workstation

Users listed in `squish.creators` can create their own workstation:

1. Open **Cloud Workstations** → **Workstations** in the Google Cloud console and click **Create workstation**.
2. Select the configuration **Squish** (`squish-workstation-config`), then **Start** and **Launch**.

Or with gcloud:

```bash
gcloud workstations create my-squish \
  --config=squish-workstation-config --cluster=squish-workstation-cluster --region="${REGION}"
gcloud workstations start my-squish \
  --config=squish-workstation-config --cluster=squish-workstation-cluster --region="${REGION}"
```

Squish is installed in `/opt/qt/squish` (linked as `~/squish`); the license file is `~/.squish-license`.
See the [Squish documentation](https://doc.qt.io/squish/) to get started.

## Smoke Test

`squish-workstation-execute` creates the workstation `squish-smoke-test` (if missing), starts it, checks over
SSH that `~/.squish-license` and `~/squish` exist, and stops it again. It runs as `workflow-executor-elevated`
(`roles/workstations.admin`).

| Parameter | Default | Description |
| :--- | :--- | :--- |
| `workstationId` | `squish-smoke-test` | Test workstation ID. |
| `verifySquish` | `true` | Check the Squish license and installation over SSH. |
| `deleteWorkstation` | `false` | Delete the workstation afterwards. Its home disk is retained (`reclaimPolicy: RETAIN`) and must be deleted manually. |

## Configuration

Edit the `squish` block in [`values.yaml`](./values.yaml):

| Value | Default | Description |
| :--- | :--- | :--- |
| `region` | `config.region` | Region of the cluster, config and image repository. |
| `network` / `subnetwork` | `sdv-network` / `sdv-subnet` | Platform VPC (`config.network` / `config.subnetwork` take precedence if injected). |
| `licenseSecretName` | `squish-license-key` | Secret Manager secret with the Squish license key. |
| `creators` | `[]` | Principals allowed to create workstations, e.g. `group:qa@example.com`. |
| `workstation.machineType` | `e2-standard-8` | Workstation machine type. |
| `workstation.poolSize` | `1` | Pre-started VMs for fast startup (billed while idle). |
| `workstation.persistentDisk.sizeGB` | `500` | Home disk size. |
| `workstation.persistentDisk.diskType` | `pd-balanced` | Home disk type. |
| `workstation.idleTimeout` / `runningTimeout` | `1200s` / `43200s` | Automatic shutdown timers. |
| `deletionPolicy` | `abandon` | KCC deletion policy for the cluster and config (see below). |

The `creators` grants are project-wide because Config Connector cannot bind IAM on `WorkstationConfig`
resources; creators can therefore create workstations from any configuration in the project.

## Disabling the Module

With the default `deletionPolicy: abandon`, disabling the module removes the Kubernetes resources but keeps the
workstation cluster, configuration, users' workstations and home disks, and the image repository in GCP, so no
data is lost and re-enabling adopts them again. IAM bindings and the VM service account are deleted. To remove
everything, delete the workstations, the configuration and the cluster with `gcloud workstations`, then the
Artifact Registry repository `squish-workstation`.

## Licensing

The module (charts, manifests, scripts and documentation) is licensed under the Apache License 2.0.
**Squish is proprietary software of The Qt Company** and requires a valid license. It is not part of this
repository: the installer is downloaded from The Qt Company at image build time, so nothing is placed in
`third_party/`.
