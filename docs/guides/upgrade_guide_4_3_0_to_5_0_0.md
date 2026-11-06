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

# Upgrade Guide: 4.3.0 to 5.0.0

This guide explains how to upgrade an existing Horizon SDV 4.3.0 environment to 5.0.0.

This is a brownfield cutover. It is not a greenfield install ([deployment_guide.md](../deployment_guide.md)). The steps below come from comparing Horizon SDV 4.3.0 (`rel/4.3.0`, `6befc94de`) to 5.0.0 RC1 (`rel/5.0.0_RC1`, `ba6f0f774`) and from one brownfield run of that path. Review the Terraform plan for **your** state before you apply.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration Placeholders](#configuration-placeholders)
- [Section #1 - Update the Repository](#section-1---update-the-repository)
- [Section #2 - Align terraform.tfvars](#section-2---align-terraformtfvars)
- [Section #3 - Run the Deployment Script](#section-3---run-the-deployment-script)
  - [Section #3a - Wait for the Config Connector cutover](#section-3a---wait-for-the-config-connector-cutover)
  - [Section #3b - Platform applications](#section-3b---platform-applications)
  - [Section #3c - Optional Cloud Workstations module](#section-3c---optional-cloud-workstations-module)
- [Section #4 - Verification](#section-4---verification)
- [Related documentation](#related-documentation)

---

## Overview

Release 5.0.0 moves Config Connector off the GKE add-on and onto a Git-pinned operator. The live `ConfigConnector` object stays in the root Argo CD app with `Prune=false`. There are **no new `terraform.tfvars` keys**. `versions.yaml` at the repository root is new and is required: Terraform reads `components.horizoncore` from it on every apply.

**Keep enabled modules as-is.** Do not disable them before the upgrade. The cutover is proven by existing Config Connector resources surviving the add-on being turned off. Disabling modules first deletes those resources and hides a bad prune.

| Change | Action required |
| --- | --- |
| GKE Config Connector add-on `enabled = true` to `enabled = false`. Vertical Pod Autoscaling is enabled on the cluster. | [Section #3](#section-3---run-the-deployment-script). Do not turn the add-on back on after the Git-pinned operator is in place. Google does not allow a CRD downgrade. |
| `ConfigConnector` custom resource stays in the root app (`gitops/templates/config-connector-init.yaml`), wave 1, `Prune=false,SkipDryRunOnMissingResource=true`. New Sync hooks `kcc-crd-gate` (wave 0) and `kcc-readiness-gate` (wave 1). Child app `configconnector-operator`. | [Section #3a](#section-3a---wait-for-the-config-connector-cutover). The object's `creationTimestamp` must not change. |
| Repository-root `versions.yaml`. Developer Portal image tag, Horizon CLI version, and the portal UI version follow `components.horizoncore`. | [Section #1](#section-1---update-the-repository). On the tested RC1 commit this value is `4.3.11`. Confirm the value on the branch you deploy. |
| Image tag changes: `module-manager-app` `0.3.3` to `0.3.7`, `horizon-api-app` `1.0.1` to `1.0.2`, `storage-gcs-module-app` `1.0.0` to `1.0.1`, `workflow-namespace-drain-app` `1.0.0` to `1.0.1`, `gerrit-post` `1.0.1` to `1.0.2`. New image `cloud-workstations-module-app` `0.2.1`. | [Section #3b](#section-3b---platform-applications). |
| Terraform no longer creates `google_container_node_pool.sdv_abfs_build_node_pool`. The Ubuntu ABFS build pool is created by `abfs-client-module-ubuntu` through Config Connector, and only when that module is enabled. | [Section #2](#section-2---align-terraformtfvars). Stop if the plan destroys a pool you still need. This path was not part of the brownfield run. |
| Optional catalog module `cloud-workstations`. | [Section #3c](#section-3c---optional-cloud-workstations-module) only if you need it. Not required to prove the cutover. |
| `mod-storage-gcs` can be `OutOfSync` / `Healthy` on its API token and TLS Secrets. | [Section #4](#section-4---verification). Sync it when nothing else is using that API. |

---

## Prerequisites

Before starting the upgrade:

- The **4.3.0 environment is fully deployed and healthy**. Argo CD applications should be `Synced` and `Healthy`.
- You can run the deployment workflow (`container-deploy.sh` or `deploy.sh`).
- You have Argo CD admin access ([Deployment Guide - Homepage not reachable](../deployment_guide.md#section-6f---homepage-not-reachable-after-deployment)). Use Argo CD at `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd`.
- **Keep enabled modules as-is during upgrade.**
- The 5.0 Git ref you deploy contains `versions.yaml` at the repository root. 4.3.0 does not have this file. `git clone -b` accepts a branch or a tag. It does not accept a bare commit SHA.
- Inventory before apply (record the values; do not change the objects):
  - `ConfigConnector` `creationTimestamp`, and that the GKE add-on is enabled
  - `StorageBucket` `horizon/argo-workflows` and any module-owned Config Connector resources (for example sample `PubSubTopic` objects)
  - Enabled Developer Portal modules, including their ids
  - Image tags currently running for `module-manager`, `horizon-dev-portal`, `horizon-api`, `storage-gcs-module`, and `workflow-namespace-drain`
  - Whether Terraform state contains `google_container_node_pool.sdv_abfs_build_node_pool`

```bash
kubectl get nodes
kubectl -n argocd get applications
kubectl get configconnector configconnector.core.cnrm.cloud.google.com \
  -o jsonpath='ct={.metadata.creationTimestamp} healthy={.status.healthy}{"\n"}'
gcloud container clusters describe sdv-cluster --region=<REGION> --project=<GCP_PROJECT_ID> \
  --format='yaml(addonsConfig.configConnectorConfig)'
kubectl get storagebucket -A
kubectl get pubsubtopic -A
```

---

## Configuration Placeholders

| Placeholder | Description | Example |
| --- | --- | --- |
| `SUB_DOMAIN` | Environment subdomain (`sdv_env_name` in tfvars) | `upgrade` |
| `HORIZON_DOMAIN` | Root domain (`sdv_root_domain` in tfvars) | `example.com` |
| `GCP_PROJECT_ID` | GCP project ID | `my-cloud-project-abc-123` |
| `REGION` | Cluster region | `us-central1` |
| `PREFIX` | Namespace prefix (empty on the main environment, `<name>-` on sub-environments) | `sub-` |
| `BRANCH_NAME` | Git branch this environment deploys. It must contain the 5.0 tree, including `versions.yaml` | `main` |

---

## Section #1 - Update the Repository

If this environment already deploys `main` on your fork, update that branch so it contains the 5.0.0 tree, then pull and deploy. Leave `scm_repo_branch` as `main`.

```bash
git fetch origin
git checkout main
git pull --rebase
test -f versions.yaml
```

`deploy.sh` applies this checkout. `container-deploy.sh` clones `scm_repo_branch`. Both must be the same 5.0.0 `main`, and that branch must contain `versions.yaml`.

If you deploy another branch, check that branch out instead and set `scm_repo_branch` to it in [Section #2](#section-2---align-terraformtfvars).

On the tested ref, `versions.yaml` sets `components.horizoncore` to `4.3.11`. The Developer Portal image tag follows that value, not the Git branch name. Read the file on your branch and expect that tag.

---

## Section #2 - Align terraform.tfvars

Leave `scm_repo_branch` unchanged when it is already the branch you updated in [Section #1](#section-1---update-the-repository). Otherwise set it to that branch. `scm_repo_url` must be the remote that has it.

5.0.0 does **not** add or rename operator keys in `terraform/env/terraform.tfvars`.

Keep `sdv_cluster_version` and `sdv_cluster_release_channel` as this environment already uses. 5.0.0 does not require a GKE version change. The sample still uses `1.34.9-gke.1322001`. The tested cluster was already on that version. If apply fails because GKE no longer serves that version on the channel, follow [Upgrade Guide 4.1.0 to 4.2.0 - Section #2a](upgrade_guide_4_1_0_to_4_2_0.md#section-2a---gke-version-if-apply-fails).

The Config Connector add-on no longer blocks the `EXTENDED` release channel. Do not enable the add-on to stay on `EXTENDED`.

`sdv_abfs_build_node_pool_version` is unchanged as a key. Its meaning is now the CASFS node version for the Config Connector pool created by `abfs-client-module-ubuntu`. If your state still has the Terraform pool, [Section #3](#section-3---run-the-deployment-script) will plan its destruction.

---

## Section #3 - Run the Deployment Script

From `tools/scripts/deployment`, run a plan first. `--apply` does not ask for confirmation.

`container-deploy.sh` stores the gcloud configuration in the Docker volume `horizon-deployer-gcloud-config` and reuses it on the next run. If that login is expired, or you need a different account, remove the volume before the plan. The script creates it again and asks you to log in.

```bash
docker volume rm horizon-deployer-gcloud-config
```

**Containerized** (clones `scm_repo_branch`; the clone must contain `versions.yaml`):

```bash
docker rmi horizon-sdv-deployer:latest   # optional: refresh deployer image
./container-deploy.sh --plan
```

**Linux native** (uses the checkout from [Section #1](#section-1---update-the-repository); it does not clone):

```bash
./deploy.sh --plan
```

Expected themes (exact resource addresses depend on **your** state):

| Theme | What to look for | Stop if |
| --- | --- | --- |
| Cluster | In-place update: `config_connector_config.enabled` becomes `false`. `vertical_pod_autoscaling` becomes enabled. | The plan replaces the cluster, or it turns the Config Connector add-on back on. |
| Images | `module-manager-app` `0.3.7`, `horizon-api-app` `1.0.2`, `storage-gcs-module-app` `1.0.1`, `workflow-namespace-drain-app` `1.0.1`, `horizon-dev-portal` equal to `components.horizoncore` (`4.3.11` on the tested RC1), new `cloud-workstations-module-app` `0.2.1`, `gerrit-post` `1.0.2`. | |
| Versions file | Terraform reads `versions.yaml`. A missing file fails apply with `no file exists at ".../versions.yaml"`. | The file is only on your laptop and not on `scm_repo_branch` when you use `container-deploy.sh`. |
| ABFS pool | Removal of `google_container_node_pool.sdv_abfs_build_node_pool` **only if that resource is in state**. | The plan destroys a node pool you still need. Enable `abfs-client-module-ubuntu` only when you intend the Config Connector pool to replace it. This was not exercised in the brownfield run. |

Apply only after the plan matches. Use the same mode you used for the plan.

**Containerized:**

```bash
./container-deploy.sh --apply
```

**Linux native:**

```bash
./deploy.sh --apply
```

A cluster update can replace a node. `NodeNotReady` and probe failures during that window are expected. Wait until `kubectl get nodes` shows every node `Ready` before you judge application health.

Do not delete the `ConfigConnector` object, and do not strip its finalizers, while the add-on is turning off.

### Section #3a - Wait for the Config Connector cutover

> [!IMPORTANT]
> The root app owns the `ConfigConnector` object and sets `Prune=false`. Do not move that object into the `configconnector-operator` child app. With prune enabled on `horizon-sdv`, a move deletes the live object before the child adopts it and tears down `cnrm-system`.

After apply, Argo CD moves `horizon-sdv` to `<BRANCH_NAME>`. Let it sync. The root app may be `Progressing` while the gate Jobs run. If it reports that it is skipping auto-sync after a failed attempt, the Git revision and the Terraform tree do not match. Point both at `<BRANCH_NAME>`, then **Refresh** and **Sync** `horizon-sdv`. Do not delete the Application.

Pass conditions:

1. `creationTimestamp` on `configconnector.core.cnrm.cloud.google.com` is the value you recorded in [Prerequisites](#prerequisites). `status.healthy` is `true`. `deletionTimestamp` is empty.
2. The GKE add-on stays disabled.
3. Jobs `kcc-crd-gate` and `kcc-readiness-gate` in namespace `gcp` are `Complete`. The chart uses the name `gcp` for these Jobs.
4. Namespace `cnrm-system` is `Active`. Pod `configconnector-operator-0` in `configconnector-operator-system` is `Running`.
5. No `*.cnrm.cloud.google.com` CRD has a `deletionTimestamp`. `validating-webhook.cnrm.cloud.google.com` still exists.
6. `StorageBucket` `horizon/argo-workflows` and the module topics you recorded are `Ready` / `UpToDate`.

```bash
kubectl get configconnector configconnector.core.cnrm.cloud.google.com \
  -o jsonpath='ct={.metadata.creationTimestamp} healthy={.status.healthy} del={.metadata.deletionTimestamp}{"\n"}'
gcloud container clusters describe sdv-cluster --region=<REGION> --project=<GCP_PROJECT_ID> \
  --format='yaml(addonsConfig.configConnectorConfig)'
kubectl get jobs -n gcp kcc-crd-gate kcc-readiness-gate
kubectl get ns cnrm-system
kubectl get pods -n configconnector-operator-system
kubectl get storagebucket -n <PREFIX>horizon argo-workflows
kubectl get pubsubtopic -A
```

A `403` from `storage.buckets.get` or `pubsub.topics.get` can appear while `gke-config-connector-sa` IAM is recreated. Wait and recheck. On the brownfield run the bucket and topics were `UpToDate` once the cluster had settled, with no manual IAM change.

If a module stays in a transition label in the Developer Portal, follow [Upgrade Guide 4.1.0 to 4.2.0 - Section #3b](upgrade_guide_4_1_0_to_4_2_0.md#section-3b---stuck-modules-in-developer-portal). Do **not** disable the module to recover it.

### Section #3b - Platform applications

Confirm the new tags are the ones actually running. A tag change rolls the pod. Restart only a Deployment that is still on the old tag after `horizon-sdv` is `Synced`.

| App | Tag 4.3.0 to 5.0.0 RC1 | Action |
| --- | --- | --- |
| `module-manager` | `0.3.3` to `0.3.7` | Confirm the new pod. Enabled module ids must match the inventory. |
| `horizon-dev-portal` | `4.3.0` to `components.horizoncore` (`4.3.11` on the tested RC1) | Confirm the image tag matches `versions.yaml`. |
| `horizon-api` | `1.0.1` to `1.0.2` | Confirm the new pod. |
| `storage-gcs-module` | `1.0.0` to `1.0.1` | Confirm the pod is `Ready`. If the app is `OutOfSync`, follow [Section #4](#section-4---verification). |
| `workflow-namespace-drain` | `1.0.0` to `1.0.1` | Terraform Helm release, not an Argo CD app. Confirm the pod image after apply. |

`gerrit-post` moves from `1.0.1` to `1.0.2`. It is a job image, not a standing Deployment.

### Section #3c - Optional Cloud Workstations module

Skip this subsection to finish the cutover. The brownfield run did not enable `cloud-workstations`.

Apply builds `cloud-workstations-module-app:0.2.1`. Enabling the module in **Administration → Modules** is a separate step after [Section #4](#section-4---verification) is green. It is not required for the Config Connector cutover.

---

## Section #4 - Verification

After [Section #3](#section-3---run-the-deployment-script) and [Section #3a](#section-3a---wait-for-the-config-connector-cutover):

1. Argo CD: `horizon-sdv` is `Synced` / `Healthy` on `<BRANCH_NAME>`.
2. Config Connector add-on is disabled. `ConfigConnector` `creationTimestamp` is unchanged and `healthy` is `true`.
3. Both KCC gate Jobs are `Complete`. `cnrm-system` is `Active`. The operator pod is `Running`. No Config Connector CRD is `Terminating`.
4. Recorded buckets and Pub/Sub topics are `Ready` / `UpToDate`.
5. Enabled modules in **Administration → Modules** match the pre-upgrade inventory and are `READY`.
6. Image tags match [Section #3b](#section-3b---platform-applications).
7. `mod-storage-gcs` is `Synced` / `Healthy`. If it is `OutOfSync` on the API Secrets, sync it when nothing else is using that API, as below. Every other child app should be `Synced` / `Healthy`.

### storage-gcs API secrets

`mod-storage-gcs` can be `OutOfSync` / `Healthy` after the upgrade. The drifted objects are `storage-gcs-module-api-tls` and `storage-gcs-module-api-token` in `horizon` and `workflows`, and the `storage-gcs-module` Deployment. The chart mints that cert and token in Helm. Argo CD's render does not reuse the live Secret, so the desired manifest does not match the cluster. The API pod stays `Ready`. This is not a Config Connector failure.

Sync the app when no workflow or other pod is using those Secrets. The sync writes a new cert and token and restarts the API.

Skip the sync when something is already using the current Secret. Those clients keep the old credentials until they mount the new Secret, and calls fail auth or TLS until then. Self-heal is off, so leaving the app `OutOfSync` does not keep rotating the Secret.

If apply or sync fails mid-way: re-run the deploy script from the same checkout, confirm `scm_repo_branch` is `<BRANCH_NAME>`, Refresh and Sync `horizon-sdv`, then repeat [Section #3a](#section-3a---wait-for-the-config-connector-cutover). This guide does not define a full platform rollback to 4.3.0. Do not re-enable the GKE Config Connector add-on to roll back.

---

## Related documentation

Authoritative guides referenced by this upgrade. Use these for exact parameters and operational detail.

| Area | Documentation |
| --- | --- |
| Terraform variables | [terraform.md](../terraform.md) |
| Deployment workflow (greenfield) | [deployment_guide.md](../deployment_guide.md) |
| `versions.yaml` missing, or the portal still shows the old version | [deployment_guide.md](../deployment_guide.md#section-6h---terraform-cannot-find-versionsyaml) |
| Other upgrade paths | [guides/README.md](README.md#upgrade-guides) |
| Previous upgrade (4.2.0 to 4.3.0) | [upgrade_guide_4_2_0_to_4_3_0.md](upgrade_guide_4_2_0_to_4_3_0.md) |
| Config Connector install behaviour | [Cloud Config Connector manual install](https://cloud.google.com/config-connector/docs/how-to/install-manually) |
