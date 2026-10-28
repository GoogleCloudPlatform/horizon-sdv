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

# Upgrade Guide: 3.0.0 to 3.1.0

This guide explains how to upgrade an existing Horizon SDV 3.0.0 environment to 3.1.0.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration Placeholders](#configuration-placeholders)
- [Section #1 - Update the Repository](#section-1---update-the-repository)
- [Section #2 - Run the Deployment Script](#section-2---run-the-deployment-script)
- [Section #3 - Post-Upgrade Steps](#section-3---post-upgrade-steps)
  - [Section #3a - Access Argo CD](#section-3a---access-argo-cd)
  - [Section #3b - Wait for ExternalSecrets to Recover](#section-3b---wait-for-externalsecrets-to-recover)
  - [Section #3c - Delete and Recreate Affected Resources](#section-3c---delete-and-recreate-affected-resources)
  - [Section #3d - Sync PostgreSQL with Prune](#section-3d---sync-postgresql-with-prune)
- [Section #4 - Verification](#section-4---verification)
- [Troubleshooting](#troubleshooting)

---

## Overview

Release 3.1.0 introduces the following breaking changes that require manual intervention when upgrading a live 3.0.0 environment:

- **Certificate Manager refactor (TAA-1057):** DNS Authorization and certificate resources were converted to use `for_each`, changing both their Terraform state addresses and their GCP resource names. Terraform `moved {}` blocks and a name-preserving conditional are included in 3.1.0 to prevent destroy and recreate of the live GCP resources.
- **ArgoCD resource refactor (TAA-1057):** All ArgoCD-related Kubernetes resources managed by Terraform were converted to `for_each`. Terraform `moved {}` blocks in 3.1.0 migrate the state addresses without destroying the live resources.
- **Gerrit StorageClass parameter change (TAA-1527):** A new `reserved-ipv4-cidr` parameter was added to the `gerrit-rwx` StorageClass. Kubernetes forbids in-place updates to StorageClass parameters, so the resource must be deleted and recreated by Argo CD.
- **MTK Connect Deployment selector change:** The `spec.selector.matchLabels` field was changed, which is immutable in Kubernetes. The affected Deployments must be deleted and recreated by Argo CD.
- **PostgreSQL schema update:** PostgreSQL requires a sync with prune to apply the updated configuration cleanly.

> **Note:** The `moved {}` blocks and the conditional DNS Authorization name introduced in 3.1.0 are temporary migration aids. They must be removed in a subsequent release once all environments have been upgraded from 3.0.0.

---

## Prerequisites

Before starting the upgrade, ensure the following:

- The **3.0.0 environment is fully deployed and healthy**. All Argo CD applications must be `Synced` and `Healthy` before starting.
- You have access to `terraform/env/terraform.tfvars` and can run the deployment workflow.
- You have `kubectl` connectivity to the cluster. Refer to [Connect to GKE via Connect Gateway](../deployment_guide.md#section-3c---connect-to-gke-via-connect-gateway) if needed.
- You have the Argo CD admin credentials available. The admin password is stored in GCP Secret Manager under the secret named `argocd-admin-password-b64`.

---

## Configuration Placeholders

Throughout this guide the following placeholders are used. Replace them with your actual values.

| Placeholder      | Description                                                       | Example          |
|------------------|-------------------------------------------------------------------|------------------|
| `SUB_DOMAIN`     | The subdomain of your environment (`sdv_env_name` in tfvars)      | `sbx`            |
| `HORIZON_DOMAIN` | Your root domain (`sdv_root_domain` in tfvars)                    | `example.com`    |

---

## Section #1 - Update the Repository

Switch your local repository to the 3.1.0 branch (or the branch containing the 3.1.0 changes) and pull the latest changes.

```bash
git fetch origin
git checkout <BRANCH_NAME>
git pull
```

If your `terraform/env/terraform.tfvars` requires any updates for new variables introduced in 3.1.0, apply them now. No new required variables are introduced in this release. Refer to the [Deployment Guide – Configure Terraform Variables](../deployment_guide.md#section-2c---configure-terraform-variables) for the full variable reference.

---

## Section #2 - Run the Deployment Script

Run the standard deployment script from `tools/scripts/deployment`.

**Containerized deployment:**

> [!NOTE]
> If you already have the deployer container image from a previous deployment, remove it first so that a new image is built with the latest `deploy.sh` and repository code. For example: `docker rmi horizon-sdv-deployer:latest`. Then run the following from `tools/scripts/deployment`:

```bash
./container-deploy.sh
```

**Linux Native deployment:**

```bash
./deploy.sh
```

> [!NOTE]
> On the first run, `terraform apply` may fail with the following error:
> ```
> Error creating DnsAuthorization: googleapi: Error 400: DNS authorization for the tuple
> (project, domain, FIXED_OR_FLEXIBLE) already exists
> ```
> This is caused by GCP eventual consistency during the DNS Authorization resource rename. If this occurs, wait approximately 2-3 minutes and run the deployment script again. The second run will succeed.

---

## Section #3 - Post-Upgrade Steps

### Section #3a - Access Argo CD

> [!IMPORTANT]
> The Horizon landing page will be temporarily broken during the upgrade due to the resources being deleted and recreated in the steps below. Use the direct Argo CD URL to access the UI.

Access Argo CD directly at:

```
https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd
```

Log in with the Argo CD admin credentials:
- **Username:** `admin`
- **Password:** Retrieve from GCP Secret Manager under the secret named `argocd-admin-password-b64`.

### Section #3b - Wait for ExternalSecrets to Recover

Once `terraform apply` finishes successfully, the ExternalSecret and SecretStore resources managed by Argo CD will temporarily enter a **Degraded** health state. This happens because the ArgoCD namespace and related resources were migrated and the External Secrets Operator needs to re-establish its connections.

Wait until they recover to **Healthy**. This may take several minutes. To speed up recovery:

1. Open the `horizon-sdv` application in Argo CD (you are already logged in from the previous step).
2. Click the **Refresh** button, then click **Sync** if the application remains out of sync.

Do not proceed to the next section until all ExternalSecret and SecretStore resources are `Healthy`.

### Section #3c - Delete and Recreate Affected Resources

Several resources must be deleted so that Argo CD can recreate them with the updated configuration. Follow the steps below in order.

> [!IMPORTANT]
> Deleting the resources below will cause brief downtime for the affected applications.

#### Delete resources in the `horizon-sdv` Argo CD application

In Argo CD, open the `horizon-sdv` application and delete the following resources:

1. **StorageClass** `gerrit-rwx`
   - Find the resource with kind `StorageClass` and name `gerrit-rwx`.
   - Click on it and select **Delete**.
   - This is required because Kubernetes forbids in-place updates to StorageClass parameters (TAA-1527 added `reserved-ipv4-cidr`).

#### Delete the following Argo CD child applications

From the Argo CD home screen, delete each of the following applications individually by clicking on the application and selecting **Delete**:

2. `gerrit-mcp-server`
3. `landingpage`
4. `monitoring-tools`
5. `mtk-connect`
6. `mcp-gateway`

#### Sync the `horizon-sdv` application with Prune enabled

Once the above resources are deleted:

1. Click on the `horizon-sdv` application.
2. Click **Sync**.
3. In the sync dialog, enable the **Prune** option.
4. Click **Synchronize**.

This will recreate all deleted resources with the updated 3.1.0 configuration.

### Section #3d - Sync PostgreSQL with Prune

PostgreSQL requires a separate sync with prune to apply the updated configuration cleanly.

1. In Argo CD, find the `postgresql` application (it may appear as a resource within `horizon-sdv` or as a child app depending on your configuration).
2. Click **Sync**.
3. Enable the **Prune** option.
4. Click **Synchronize**.

---

## Section #4 - Verification

Once all post-upgrade steps are complete, verify the environment is healthy:

1. In Argo CD, confirm that all applications under `horizon-sdv` are `Synced` and `Healthy`.
2. Confirm the Horizon landing page is accessible at `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`.
3. Confirm that the following applications are reachable and functioning:
   - Landing Page: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
   - Argo CD: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd`
   - Keycloak: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/keycloak`
   - Gerrit: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/gerrit`
   - Jenkins: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/jenkins`
   - MTK Connect: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/mtk-connect`
   - MCP Gateway Registry: `https://mcp.<SUB_DOMAIN>.<HORIZON_DOMAIN>`

---

## Troubleshooting

### DNS Authorization 400 error on first `terraform apply`

```
Error creating DnsAuthorization: googleapi: Error 400: DNS authorization for the tuple
(project, domain, FIXED_OR_FLEXIBLE) already exists
```

This is caused by GCP eventual consistency. The old DNS Authorization resource was accepted for deletion but has not yet fully propagated as removed when the new resource creation is attempted. Wait 2-3 minutes and run the deployment script again.

### Argo CD namespace stuck in `Terminating`

If the `argocd` namespace remains in a `Terminating` state for more than a few minutes after `terraform apply`, it is likely blocked by a finalizer on the `horizon-sdv` Argo CD `Application` resource whose controller was torn down during the migration. Remove the finalizer manually:

1. Remove the finalizer from the `horizon-sdv` Application:
   ```bash
   kubectl patch application horizon-sdv -n argocd \
     --type=json \
     -p='[{"op":"remove","path":"/metadata/finalizers"}]'
   ```
2. If the namespace is still stuck, remove its finalizers directly:
   ```bash
   kubectl patch namespace argocd \
     --type=json \
     -p='[{"op":"remove","path":"/metadata/finalizers"}]'
   ```
3. Terraform will recreate the namespace and all ArgoCD resources on the next apply.

### ExternalSecret or SecretStore remains Degraded after a long wait

If ExternalSecrets or SecretStores do not recover to `Healthy`:

1. Check the External Secrets Operator pod is running:
   ```bash
   kubectl get pods -n external-secrets
   ```
2. Force a refresh in Argo CD by clicking **Refresh** on the `horizon-sdv` application, then **Sync**.
3. If a specific ExternalSecret continues to fail, check its events:
   ```bash
   kubectl describe externalsecret <NAME> -n <NAMESPACE>
   ```
   Verify that the referenced GCP Secret Manager secret exists and that the Workload Identity service account has the `Secret Manager Secret Accessor` role.
