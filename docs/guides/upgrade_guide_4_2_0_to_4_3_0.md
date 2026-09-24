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

# Upgrade Guide: 4.2.0 to 4.3.0

This guide explains how to upgrade an existing Horizon SDV 4.2.0 environment to 4.3.0.

This is a brownfield cutover. It is not a greenfield install ([deployment_guide.md](../deployment_guide.md)). The steps below come from comparing Horizon SDV 4.2.0 to 4.3.0. Review the Terraform plan for **your** state before you apply.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration Placeholders](#configuration-placeholders)
- [Section #1 - Update the Repository](#section-1---update-the-repository)
- [Section #2 - Align terraform.tfvars](#section-2---align-terraformtfvars)
- [Section #3 - Run the Deployment Script](#section-3---run-the-deployment-script)
  - [Section #3a - Redeploy platform applications](#section-3a---redeploy-platform-applications)
  - [Section #3b - Horizon CLI download from the Developer Portal](#section-3b---horizon-cli-download-from-the-developer-portal)
  - [Section #3c - Optional RemotiveTopology module (EXPERIMENTAL)](#section-3c---optional-remotivetopology-module-experimental)
  - [Section #3d - Workload upgrades (release checklist) - CI/CD](#section-3d---workload-upgrades-release-checklist---cicd)
- [Section #4 - Verification](#section-4---verification)
- [Related documentation](#related-documentation)

---



## Overview

Release 4.3.0 adds a downloadable Horizon CLI to the Developer Portal, adds the optional **EXPERIMENTAL** `remotive-topology` module, and ships security updates for Go and npm dependencies. There are **no new `terraform.tfvars` keys** between 4.2.0 and 4.3.0 and no GKE version change. You still must review `terraform plan` because apply rebuilds platform images and updates Cloud NAT.

**Keep enabled modules as-is.** Do not disable modules before the upgrade. Disabling `**workloads-android**` deletes Cuttlefish `ComputeInstanceTemplate` custom resources and the matching GCE instance templates.


| Change                                                                                                                               | Action required                                                                                                                                         |
| ------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Horizon CLI binaries built into the Developer Portal image; `horizon-dev-portal` `1.1.0` → `4.3.0`; new Terraform provider `hashicorp/local` | [Section #3](#section-3---run-the-deployment-script) installs the provider (`terraform init -upgrade`). Verify the **Tools** page in [Section #3b](#section-3b---horizon-cli-download-from-the-developer-portal). |
| `module-manager` `0.3.2` → `0.3.3` (startup sync only patches parent Applications; per-module KCC namespaces; `remotive-topology` in the catalog) | [Section #3a](#section-3a---redeploy-platform-applications): sync and confirm the new pod.                                                              |
| Security rebuilds with **unchanged** tags: `horizon-api-app` `1.0.1`, `storage-gcs-module-app` `1.0.0`, `workflow-namespace-drain-app` `1.0.0` | [Section #3a](#section-3a---redeploy-platform-applications): restart these Deployments so they run the rebuilt images.                                  |
| Cloud NAT dynamic port allocation (64-4096 ports per VM) on the main and ARM64 NAT                                                   | In-place update; review in `terraform plan` during [Section #3](#section-3---run-the-deployment-script).                                               |
| Optional catalog module `remotive-topology` (EXPERIMENTAL partner contribution)                                                      | [Section #3c](#section-3c---optional-remotivetopology-module-experimental) only if you need it.                                                         |
| Android Mirror: new `REPO_SYNC_TIMEOUT` parameter on Sync Mirror                                                                     | [Section #3d](#section-3d---workload-upgrades-release-checklist---cicd): seed `android` if you use the AOSP Mirror.                                    |
| Cloud Workstations Preflight web dependencies (`postcss`, `nanoid`)                                                                  | [Section #3d](#section-3d---workload-upgrades-release-checklist---cicd): rebuild the image chain only if you use Android Studio or ASfP workstations.  |
| MTK Connect `axios` `1.20.0`                                                                                                         | None. `mtk_connect.sh` installs the pinned packages at job runtime.                                                                                     |


---



## Prerequisites

Before starting the upgrade:

- The **4.2.0 environment is fully deployed and healthy**. Argo CD applications should be `Synced` and `Healthy`.
- You can run the deployment workflow (`container-deploy.sh` or `deploy.sh`). The deployment host needs outbound access to the Terraform registry so `terraform init -upgrade` can install `hashicorp/local`.
- You have Argo CD admin access ([Deployment Guide - Homepage not reachable](../deployment_guide.md#section-6f---homepage-not-reachable-after-deployment)). Use Argo CD at `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd` for sync and recovery.
- **Keep enabled modules as-is during upgrade.**
- Inventory before apply (names only; do not change them yet):
  - Enabled Developer Portal modules
  - Image tags currently running for `module-manager`, `horizon-dev-portal`, `horizon-api`
  - Cloud Workstation configs and image names, if used

---



## Configuration Placeholders


| Placeholder       | Description                                                  | Example                    |
| ----------------- | ------------------------------------------------------------ | -------------------------- |
| `SUB_DOMAIN`      | Environment subdomain (`sdv_env_name` in tfvars)             | `sbx`                      |
| `HORIZON_DOMAIN`  | Root domain (`sdv_root_domain` in tfvars)                    | `example.com`              |
| `GCP_PROJECT_ID`  | GCP project ID                                               | `my-cloud-project-abc-123` |
| `PREFIX`          | Namespace prefix (empty on the main environment, `<name>-` on sub-environments) | `sub-`                     |


---

## Section #1 - Update the Repository

Check out the 4.3.0 published content and pull the latest changes. Use the branch this environment already deploys (`BRANCH_NAME` in tfvars). On the public [horizon-sdv](https://github.com/GoogleCloudPlatform/horizon-sdv) repository that is typically `main`.

```bash
git fetch origin
git checkout <BRANCH_NAME>
git pull
```

---

## Section #2 - Align terraform.tfvars

4.2.0 and 4.3.0 do **not** add or rename operator keys in `terraform/env/terraform.tfvars` / `terraform.tfvars.sample`. You do not need a tfvars rewrite.

Keep `**sdv_cluster_version**` and `**sdv_cluster_release_channel**` as this environment already uses. 4.3.0 does not require a GKE upgrade. If apply fails because GKE no longer serves that version on the channel, follow [Upgrade Guide 4.1.0 to 4.2.0 - Section #2a](upgrade_guide_4_1_0_to_4_2_0.md#section-2a---gke-version-if-apply-fails).

---

## Section #3 - Run the Deployment Script

From `tools/scripts/deployment`:

**Containerized:**

```bash
docker rmi horizon-sdv-deployer:latest   # optional: refresh deployer image
./container-deploy.sh --apply
```

**Linux native:**

```bash
./deploy.sh --apply
```

The script runs `terraform init -upgrade`, which installs the new `hashicorp/local` provider. No manual `terraform init` is needed.

Review the plan before you confirm apply. Expected themes (exact resource addresses depend on **your** state):


| Theme     | What to look for                                                                                                                                                                                                                                                                                                                     | Stop if                                                              |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Images    | `horizon-dev-portal` `4.3.0` (new tag); `module-manager-app` `0.3.3` (new tag); content-hash rebuilds of `horizon-api-app:1.0.1`, `storage-gcs-module-app:1.0.0`, `workflow-namespace-drain-app:1.0.0`                                                                                                                             |                                                                      |
| CLI files | New `local_file.horizon_cli_docker_src[...]` resources. They copy `tools/horizon/*.go`, `go.mod` and `go.sum` into the portal Docker build context (`.../horizon-dev-portal/cli/`, ignored by Git) so the portal image can cross-compile the CLI.                                                                                    | Paths outside `terraform/modules/sdv-container-images/images/horizon-dev-portal/` |
| Network   | In-place update of `google_compute_router_nat.vpc_nat` (and `arm64_nat` if ARM64 is enabled): `enable_dynamic_port_allocation = true`, `enable_endpoint_independent_mapping = false`, `min_ports_per_vm = 64`, `max_ports_per_vm = 4096`                                                                                          | Router or NAT IP **replacement** instead of an in-place update       |


Wait until Argo CD syncs `**horizon-sdv**` and child applications (`**Synced**` / `**Healthy**`) before [Section #3a](#section-3a---redeploy-platform-applications).

### Section #3a - Redeploy platform applications

> [!IMPORTANT]
> Run this **after** [Section #3](#section-3---run-the-deployment-script) completes and `**horizon-sdv**` is `**Synced**` / `**Healthy**`. Brief downtime for these apps is expected.

#### Argo CD - sync platform applications

Open Argo CD: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd`.

1. Confirm the root app `**horizon-sdv**` is `**Synced**` / `**Healthy**`. If not: **Refresh** → **Sync**.
2. Open each child Application (add your `<PREFIX>` if your environment uses one, for example `sbx-module-manager`):
  - `**module-manager**`
  - `**horizon-dev-portal**`
  - `**horizon-api**`
3. For each app: **Refresh** → **Sync** (use **Hard refresh** if **OutOfSync**).

#### Restart Deployments - required behavior differs by app

[Section #3](#section-3---run-the-deployment-script) rebuilds images in Artifact Registry. Kubernetes only starts a new pod when the Deployment spec changes or you recreate the pod. `**imagePullPolicy: Always**` does not refresh a running pod that still matches the same tag.


| App                            | Tag 4.2.0 → 4.3.0 | Auto-roll on sync? | Action                                                                                                             |
| ------------------------------ | ----------------- | ------------------ | ------------------------------------------------------------------------------------------------------------------ |
| `**module-manager**`           | `0.3.2` → `0.3.3` | Usually **yes**    | Sync; restart if the pod still shows `0.3.2`                                                                        |
| `**horizon-dev-portal**`       | `1.1.0` → `4.3.0` | Usually **yes**    | Sync; restart if the pod still shows `1.1.0`                                                                        |
| `**horizon-api**`              | `1.0.1` → `1.0.1` | Often **no**       | Sync, then **restart** the Deployment so it runs the rebuilt image                                                  |
| `**storage-gcs-module**`       | `1.0.0` → `1.0.0` | Often **no**       | Only if `storage-gcs` is enabled: **restart** the Deployment in `<PREFIX>horizon`                                   |
| `**workflow-namespace-drain**` | `1.0.0` → `1.0.0` | **No** (Terraform Helm release, not Argo CD) | **Restart** with `kubectl` (below)                                                                      |


Restart Argo-managed apps from the Argo resource tree (Deployment → ⋮ → Restart). Prefer **Restart** on the Deployment. Deleting the Argo Application is not a retry; it can prune live resources.

Restart the Terraform-managed drain controller:

```bash
kubectl -n <PREFIX>workflow-namespace-drain rollout restart deployment/workflow-namespace-drain
```

If a new pod still runs the old image digest, confirm [Section #3](#section-3---run-the-deployment-script) pushed a new image to Artifact Registry, then restart again.

#### Developer Portal - verify enabled modules

Modules enabled under 4.2.0 are not orphaned. After `module-manager` `0.3.3` starts:

1. Open **Administration → Modules**: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/developer-portal/admin/modules`
2. Enabled modules should settle on **READY**. `module-manager` `0.3.3` no longer patches the child Applications of `workloads-common` / `workloads-android` at startup, so these modules no longer stay in **UPDATE IN PROGRESS** after a Module Manager restart.
3. `**remotive-topology**` is listed as a new, disabled module. Leave it disabled unless you follow [Section #3c](#section-3c---optional-remotivetopology-module-experimental).

If a module stays in a transition label, follow [Upgrade Guide 4.1.0 to 4.2.0 - Section #3b](upgrade_guide_4_1_0_to_4_2_0.md#section-3b---stuck-modules-in-developer-portal). Do **not** disable the module to recover it.

#### When to skip

- **Greenfield 4.3.0 install:** [Section #3](#section-3---run-the-deployment-script) already deploys current images; restart only if pods are stale after sync.

### Section #3b - Horizon CLI download from the Developer Portal

The portal image `4.3.0` contains prebuilt Horizon CLI binaries for Linux amd64, Linux arm64, Windows amd64 and macOS arm64. They are served only to signed-in users through `/api/cli`.

1. Sign in to `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/developer-portal/` and open **Tools → Horizon CLI**.
2. Download the binary for your platform and compare its SHA-256 with the value on the page.
3. Run `horizon version`. The version and build date match the values on the Tools page.

The binaries are not code-signed. On macOS remove the quarantine attribute before the first run (`xattr -d com.apple.quarantine horizon`). Windows SmartScreen may warn. Details: [developer_portal_user_guide.md](../developer_portal_user_guide.md#workflows-via-horizon-cli).

If **Tools** is missing from the sidebar, the running pod is still on `1.1.0`: restart `horizon-dev-portal` ([Section #3a](#section-3a---redeploy-platform-applications)).

### Section #3c - Optional RemotiveTopology module (EXPERIMENTAL)

Skip this subsection if you do not run RemotiveTopology simulations.

`remotive-topology` is an **EXPERIMENTAL** partner contribution from RemotiveLabs. It has a **hard** dependency on `**workloads-common**`.

1. Create the RemotiveCloud auth Secret with a revocable, least-privilege service-account token (never personal credentials):

   ```bash
   kubectl -n <PREFIX>workflows create secret generic workflow-remotive-cloud-auth \
     --from-literal=token=<service-account-token> \
     --from-literal=organization=<organization-id>
   ```

2. Confirm the `workflow-mtk-connect-apikey` Secret exists in `<PREFIX>workflows`. It is created by the MTK Connect post job; the launcher needs it for interactive access.
3. Enable `**remotive-topology**` in **Administration → Modules** and wait for **READY**. The module creates the namespace `<PREFIX>remotive-kcc` for its `ComputeInstanceTemplate` custom resources.
4. Run the workflows in order: `remotive-builder-image` (once), `remotive-instance-template`, then `remotive-launcher` per topology run with `topologyDownloadUrl` and `topologyName`.

> [!IMPORTANT]
> Disabling `**remotive-topology**` deletes its `ComputeInstanceTemplate` custom resources and the matching GCE instance templates. Disabling `**workloads-android**` no longer touches them.

Details: [workloads/remotive/README.md](../../workloads/remotive/README.md).

### Section #3d - Workload upgrades (release checklist) - CI/CD

Run this checklist **after** [Section #3a](#section-3a---redeploy-platform-applications). Skip steps your environment does not use.

This release does **not** require regenerating Android, OpenBSW, utilities, or ABFS **Docker image templates**, or Cuttlefish GCE instance templates.

#### Jenkins seed job

1. Run **Seed Workloads** once with `**none**` so Jenkins job parameters refresh.
2. Seed `**android**` (or `**all**`) if you use the AOSP Mirror. **Android → Environment → Mirror → Sync Mirror** gains the `REPO_SYNC_TIMEOUT` parameter (default `20h`) and reports progress every 5 minutes during `repo sync`.

See [workloads/seed.md](../workloads/seed.md).

#### Cloud Workstations (only if you use Android Studio or ASfP)

The `horizon-preflight` image picks up the `postcss` / `nanoid` security updates only when it is rebuilt. Rebuild when convenient:

1. Run **Cloud Workstations → Workstation Image Chain → Horizon AOSP Build Image Chain** with Preflight and GNOME enabled and `NO_PUSH=false` on the children you publish. See [workstation_images.md](../workloads/cloud-workstations/workstation_images.md).
2. **Stop and start** existing workstations so they boot the new image.

`horizon-code-oss` is not affected.

---

## Section #4 - Verification

After [Section #3](#section-3---run-the-deployment-script), [Section #3a](#section-3a---redeploy-platform-applications) and any applicable [Section #3b](#section-3b---horizon-cli-download-from-the-developer-portal) / [Section #3c](#section-3c---optional-remotivetopology-module-experimental) / [Section #3d](#section-3d---workload-upgrades-release-checklist---cicd) steps:

1. Argo CD: `**horizon-sdv**` and children `**Synced**` / `**Healthy**`.
2. `**module-manager**` image is `**module-manager-app:0.3.3**` and `**horizon-dev-portal**` image is `**horizon-dev-portal:4.3.0**`.
3. `**horizon-api**`, `**workflow-namespace-drain**` and, if enabled, `**storage-gcs-module**` pods were recreated after apply.
4. Enabled modules are **READY** in **Administration → Modules** and stay **READY** after a `module-manager` restart.
5. **Tools → Horizon CLI** lists four platforms; a downloaded binary runs `horizon version` and matches the version and build date on the page.
6. Cloud NAT: `gcloud compute routers nats describe <network>-<region>-egress-nat --router=<network>-<region>-nat-router --region=<region>` shows `enableDynamicPortAllocation: true`.
7. Jenkins: **Sync Mirror** shows the `REPO_SYNC_TIMEOUT` parameter, if you seeded `android`.
8. Optional: `**remotive-topology**` READY and `<PREFIX>remotive-kcc` namespace present, if you enabled it.

If apply or sync fails mid-way: re-run the deploy script, Refresh → Sync `**horizon-sdv**`, then [Section #3a](#section-3a---redeploy-platform-applications). This guide does not define a full platform rollback to 4.2.0.

---



## Related documentation

Authoritative guides referenced by this upgrade. Use these for exact parameters, job names, and operational detail.


| Area                                         | Documentation                                                                                  |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Terraform variables                          | [terraform.md](../terraform.md)                                                                |
| Deployment workflow (greenfield)             | [deployment_guide.md](../deployment_guide.md)                                                  |
| Other upgrade paths                          | [guides/README.md](README.md#upgrade-guides)                                                   |
| Previous upgrade (4.1.0 to 4.2.0)            | [upgrade_guide_4_1_0_to_4_2_0.md](upgrade_guide_4_1_0_to_4_2_0.md)                             |
| Developer Portal and Horizon CLI install     | [developer_portal_user_guide.md](../developer_portal_user_guide.md)                            |
| RemotiveTopology module                      | [workloads/remotive/README.md](../../workloads/remotive/README.md)                             |
| Seed jobs and Jenkins parameters             | [workloads/seed.md](../workloads/seed.md)                                                      |
| Cloud Workstation images                     | [workstation_images.md](../workloads/cloud-workstations/workstation_images.md)                 |
| Container image CVE maintenance              | [container_image_security_upgrade_guide.md](container_image_security_upgrade_guide.md)         |

