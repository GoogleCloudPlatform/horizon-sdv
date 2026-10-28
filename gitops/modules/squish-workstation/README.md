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
Kubernetes Config Connector (KCC) when the module is enabled in the Developer Portal. **No Terraform change is
required**: the module does not need extra permissions for the Config Connector service account.

The module is supported in the main environment only; rendering fails when `config.isSubEnvironment` is true
because its GCP resource IDs (cluster, configuration, VM service account, image repository) are project-global.

## Table of contents

- [Components](#components)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Enabling the Module](#enabling-the-module)
- [Building the Squish Image](#building-the-squish-image)
- [Creating a Workstation](#creating-a-workstation)
- [Smoke Test](#smoke-test)
- [Configuration](#configuration)
- [Security Model](#security-model)
- [Disabling the Module](#disabling-the-module)
- [Licensing](#licensing)

## Components

| Component | Name | Path |
| :--- | :--- | :--- |
| Child Application (KCC) | `mod-squish-workstation-squish-workstation-app` | `squish-workstation-app/` |
| Child Application (pipelines) | `mod-squish-workstation-argo-workflows` | `argo-workflows/` |
| Developer Portal overview | `mod-squish-workstation-overview` | `portal/overview.html` |
| Image build context | `squish` image | `image/` |

Config Connector resources (namespace `squish-workstation-app`, created by the parent Application at wave 0),
in Argo CD sync-wave order. Every resource except the `squish.creators` bindings carries
`cnrm.cloud.google.com/deletion-policy: <squish.deletionPolicy>`:

| Wave | Kind | Purpose |
| :--- | :--- | :--- |
| 1 | `ArtifactRegistryRepository` `squish-workstation` | Docker repository for the Squish image. |
| 1 | `IAMServiceAccount` `squish-workstation-vm` | Service account of the workstation VMs. |
| 2 | `IAMPolicyMember` (resource level) | VM SA: `secretmanager.secretAccessor` on the license secret, `artifactregistry.reader` on the `squish-workstation` repository. |
| 2 | `IAMPolicyMember` (project level, opt-in) | Only with `squish.kcc.projectIamBindings.enabled: true`. VM SA: `logging.logWriter`. Each entry of `squish.creators`: `workstations.workstationCreator` and `workstations.operationViewer`. |
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
  `/opt/qt/squish` from a private temporary directory (with a temporary `HOME`) and removes that directory on
  exit, so neither the license key nor the installer's `~/.squish-license` ends up in the image layer. Only the
  secret *name* is stored in the image (`/opt/qt/.squish-gcp-license-secret`).
- `Dockerfile`: `configure_workstation.sh` only logs a warning when a hook fails, so the build asserts afterwards
  that `/opt/qt/squish/bin` and the secret reference exist and that no `/root/.squish-license`, `/root/.squish`
  or `/tmp/squish-license-key` is left. Build settings (`GCP_PROJECT`, `SQUISH_*`, `EXTRA_PKGS`, `CURL_OPTS`)
  are build arguments only and are not exported into the runtime image.
- `etc/workstation-startup.d/200_setting-squish.sh` (each workstation start): reads the license key from
  Secret Manager into `~/.squish-license` (owned by the workstation user, mode 600) and links `~/squish` to
  `/opt/qt/squish`.

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
   has project-level Secret Manager access; the workstation VM service account is granted
   `roles/secretmanager.secretAccessor` on this secret only, through the module.

2. **Horizon GNOME base image.** The Squish image is layered on
   `<region>-docker.pkg.dev/<project>/horizon-sdv/cloud-ws-images/horizon-gnome:latest`. Publish it with the
   Jenkins job *Cloud Workstations > Workstation Images > Horizon GNOME* (or *Workstation Image Chain*), or
   pass another image as the `baseImage` workflow parameter.

3. **Project-level IAM bindings (administrator).** By default (`squish.kcc.projectIamBindings.enabled: false`)
   the module creates no project-level IAM bindings, because that would require
   `roles/resourcemanager.projectIamAdmin` for the cluster-wide Config Connector service account. An administrator
   pre-provisions them once (after enabling the module, when the VM service account exists):

   | Member | Role |
   | :--- | :--- |
   | `serviceAccount:squish-workstation-vm@<project>.iam.gserviceaccount.com` | `roles/logging.logWriter` |
   | each workstation creator (user or group) | `roles/workstations.workstationCreator` |
   | each workstation creator (user or group) | `roles/workstations.operationViewer` |

   ```bash
   gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
     --member="serviceAccount:squish-workstation-vm@${PROJECT_ID}.iam.gserviceaccount.com" \
     --role=roles/logging.logWriter
   for role in roles/workstations.workstationCreator roles/workstations.operationViewer; do
     gcloud projects add-iam-policy-binding "${PROJECT_ID}" --member="group:qa-team@example.com" --role="${role}"
   done
   ```

   Without `roles/logging.logWriter` the workstation runs normally but does not ship its logs to Cloud Logging.
   Set `squish.kcc.projectIamBindings.enabled: true` only if your environment deliberately allows Config Connector
   to manage project IAM; `squish.creators` is then reconciled by the module.

   The two resource-level bindings (license secret, image repository) are always created by the module. If their
   `IAMPolicyMember` reports a `setIamPolicy` permission error, grant `gke-config-connector-sa` the corresponding
   admin role on that single resource (`roles/secretmanager.admin` on the secret,
   `roles/artifactregistry.admin` on the repository), not on the project.

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
| `baseImage` | `<region>-docker.pkg.dev/<project>/horizon-sdv/cloud-ws-images/horizon-gnome:latest` | Base image for the Squish layer. Must be an Artifact Registry image in the platform project with a tag or `@sha256:` digest; anything else is rejected. |
| `imageTag` | `latest` | Tag to push (`[A-Za-z0-9_][A-Za-z0-9._-]{0,127}`). The WorkstationConfig uses `squish.imageTag`. |
| `dryRun` | `false` | Build without pushing. |

Re-run the workflow to update Squish (change `SQUISH_DOWNLOAD_URL` and `SQUISH_SHA256` in `image/Dockerfile`)
or to pick up a new base image. Running workstations use the new image after their next start.

## Creating a Workstation

Users listed in `squish.creators` (or granted the creator roles by an administrator, see
[Prerequisites](#prerequisites)) can create their own workstation:

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
| `workstationId` | `squish-smoke-test` | Test workstation ID (`[a-z]([a-z0-9-]{0,61}[a-z0-9])?`). |
| `verifySquish` | `true` | Check the Squish license and installation over SSH. |
| `deleteWorkstation` | `false` | Delete the workstation afterwards. Its home disk is retained (`reclaimPolicy: RETAIN`) and must be deleted manually. |

## Configuration

Edit the `squish` block in [`values.yaml`](./values.yaml):

| Value | Default | Description |
| :--- | :--- | :--- |
| `region` | `config.region` | Region of the cluster, config and image repository. |
| `network` / `subnetwork` | `sdv-network` / `sdv-subnet` | Platform VPC (`config.network` / `config.subnetwork` take precedence if injected). |
| `licenseSecretName` | `squish-license-key` | Secret Manager secret with the Squish license key. |
| `artifactRepository` | `squish-workstation` | Artifact Registry repository for the Squish image; the VM SA gets `artifactregistry.reader` on it. |
| `creators` | `[]` | Principals allowed to create workstations, e.g. `group:qa@example.com`. Only applied when `kcc.projectIamBindings.enabled` is true. |
| `workstation.machineType` | `e2-standard-8` | Workstation machine type. |
| `workstation.poolSize` | `1` | Pre-started VMs for fast startup (billed while idle). |
| `workstation.persistentDisk.sizeGB` | `500` | Home disk size. |
| `workstation.persistentDisk.diskType` | `pd-balanced` | Home disk type. |
| `workstation.idleTimeout` / `runningTimeout` | `1200s` / `43200s` | Automatic shutdown timers. |
| `deletionPolicy` | `abandon` | KCC deletion policy for the repository, VM service account and its bindings, cluster and config (see below). |
| `kcc.projectIamBindings.enabled` | `false` | Let Config Connector create the project-level bindings (`logging.logWriter`, `creators`). See [Prerequisites](#prerequisites). |

The `creators` grants are project-wide because Config Connector cannot bind IAM on `WorkstationConfig`
resources; creators can therefore create workstations from any configuration in the project.

## Security Model

- **No project IAM administration by Config Connector.** Config Connector runs in cluster mode, so a grant of
  `roles/resourcemanager.projectIamAdmin` to its service account would let any chart grant arbitrary project
  roles. The module therefore binds only on its own resources (license secret, image repository) and keeps
  project-level bindings opt-in (`squish.kcc.projectIamBindings.enabled`, default `false`).
- **Least privilege for the workstation VM.** The VM service account can read only the license secret and pull
  only from the `squish-workstation` repository.
- **No license in the image.** The license key is read at build time into a private temporary directory that is
  removed on exit, and the build fails if license files remain. At runtime the key is fetched again into the
  user's `~/.squish-license` (mode 600).
- **Validated workflow input.** `squish-workstation-init` accepts only base images from the platform project's
  Artifact Registry and validates `imageTag` before anything is passed to the BuildKit build;
  `squish-workstation-execute` validates `workstationId`.
- **Explicit workflow service accounts.** The Sensors set `serviceAccountName` on submitted Workflows
  (`workflow-executor` for init, `workflow-executor-elevated` for execute), so the controller default cannot
  replace them.
- **Pruning.** Both child Applications use `automated.prune: true`, so removing an entry from `creators`
  removes its binding.

## Disabling the Module

With the default `deletionPolicy: abandon`, disabling the module removes the Kubernetes resources but keeps the
workstation cluster, configuration, users' workstations and home disks, and the image repository in GCP, so no
data is lost and re-enabling adopts them again. The VM service account and its bindings are kept as well, so
abandoned workstations keep working; `creators` bindings are always deleted. Bindings pre-provisioned by an
administrator are not touched. To remove everything, delete the workstations, the configuration and the cluster
with `gcloud workstations`, then the Artifact Registry repository `squish-workstation`, the service account
`squish-workstation-vm` and the administrator-provisioned bindings.

## Licensing

The module (charts, manifests, scripts and documentation) is licensed under the Apache License 2.0.
**Squish is proprietary software of The Qt Company** and requires a valid license. It is not part of this
repository: the installer is downloaded from The Qt Company at image build time, so nothing is placed in
`third_party/`.
