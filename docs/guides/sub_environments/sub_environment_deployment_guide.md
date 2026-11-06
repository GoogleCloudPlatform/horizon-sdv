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

# Sub-Environment Deployment Guide

This guide explains what a sub-environment is, how to configure one, and how to deploy or destroy it within an existing Horizon SDV installation.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration Placeholders](#configuration-placeholders)
- [Configuring Sub-Environments](#configuring-sub-environments)
  - [Required Fields](#required-fields)
  - [Optional Fields](#optional-fields)
  - [Password Requirements](#password-requirements)
  - [Sub-Environment Name Rules](#sub-environment-name-rules)
- [Deploying a Sub-Environment](#deploying-a-sub-environment)
- [Accessing Sub-Environment Applications](#accessing-sub-environment-applications)
  - [Available Applications](#available-applications)
  - [Shared Components](#shared-components)
- [Managing Multiple Sub-Environments](#managing-multiple-sub-environments)
- [Destroying a Sub-Environment](#destroying-a-sub-environment)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)

---

## Overview

A **sub-environment** is a fully isolated copy of the Horizon SDV platform that runs alongside the main environment on the **same GKE cluster**. Every sub-environment gets:

- Its own set of **Kubernetes namespaces**, prefixed with the sub-environment name (e.g. `dev-jenkins`, `dev-keycloak`).
- Its own **Argo CD instance**, managing only that sub-environment's workloads.
- Its own **sub-domain**, derived from the main environment domain (e.g. `dev.<SUB_DOMAIN>.<HORIZON_DOMAIN>`).
- Its own **GCP Certificate Manager certificate** and **DNS authorisation** for its sub-domain.
- Its own **GCP Secret Manager secrets** and **Workload Identity service accounts** for every application.

Sub-environments are defined entirely in `terraform.tfvars` — no code changes are required to create or destroy them. Typical use cases include:

- Giving different teams isolated and independent platform instances without provisioning separate clusters.
- Testing platform changes on a branch before merging to the main environment.
- Running a stable long-lived environment alongside a short-lived experimental one.

---

## Prerequisites

Before creating a sub-environment, ensure the following are in place:

- The **main Horizon SDV environment is already deployed** and healthy. Sub-environments depend on cluster-level components (External Secrets Operator, Gerrit Operator, etc.) installed by the main environment.
- You have access to `terraform/env/terraform.tfvars` and can run the deployment workflow.
- The GCP project's **Certificate Manager** quota can accommodate one additional certificate per sub-environment.
- The sub-environment name you choose is unique and not already in use in `sdv_sub_env_configs`.

---

## Configuration Placeholders

Throughout this guide the following placeholders are used. Replace them with your actual values.

| Placeholder              | Description                                                        | Example                        |
|--------------------------|--------------------------------------------------------------------|--------------------------------|
| `SUB_ENV_NAME`           | The name of your sub-environment (used as namespace prefix)        | `dev`                          |
| `SUB_DOMAIN`             | The subdomain of your main environment (`sdv_env_name` in tfvars)  | `sbx`                          |
| `HORIZON_DOMAIN`         | Your root domain (`sdv_root_domain` in tfvars)                     | `example.com`                  |
| `BRANCH_NAME`            | The Git branch to sync for this sub-environment                    | `feature/my-feature`           |
| `KEYCLOAK_ADMIN_PASS`    | Keycloak admin password for this sub-environment                   | `MySubEnvPass123!`             |
| `HORIZON_ADMIN_PASS`     | Keycloak Horizon realm admin password for this sub-environment     | `MySubEnvHorizon123!`          |

---

## Configuring Sub-Environments

Sub-environments are defined using the `sdv_sub_env_configs` variable in `terraform/env/terraform.tfvars`. Each key in the map is the sub-environment name. The variable is absent (defaults to an empty map) when no sub-environments are needed.

Add the following block to your `terraform.tfvars`:

```hcl
sdv_sub_env_configs = {
  "<SUB_ENV_NAME>" = {
    keycloak_admin_password         = "<KEYCLOAK_ADMIN_PASS>"
    keycloak_horizon_admin_password = "<HORIZON_ADMIN_PASS>"

    # Optional: sync this sub-environment's ArgoCD from a specific branch.
    # Defaults to scm_repo_branch if not set.
    branch = "<BRANCH_NAME>"

    # Optional: override auto-generated passwords for specific secrets.
    manual_secrets = {
      # s5  = "ArgoCD_Admin_Pass123!"
      # s6  = "Jenkins_Admin_Pass123!"
      # s9  = "Gerrit_Admin_Pass123!"
      # s12 = "Grafana_Admin_Pass123!"
      # s14 = "Postgres_Admin_Pass123!"
      # s17 = "McpGateway_Admin_Pass123!"
    }
  }
}
```

### Required Fields

| Field                            | Description                                                 |
|----------------------------------|-------------------------------------------------------------|
| `keycloak_admin_password`        | Password for the Keycloak master realm admin account        |
| `keycloak_horizon_admin_password`| Password for the Keycloak Horizon realm admin account       |

### Optional Fields

| Field            | Description                                                                                         | Default                       |
|------------------|-----------------------------------------------------------------------------------------------------|-------------------------------|
| `branch`         | Git branch that Argo CD in this sub-environment will sync from                                      | `scm_repo_branch`          |
| `manual_secrets` | Map of secret IDs to plain-text values for secrets you want to set manually instead of auto-generating. Each value must not be `Change_Me_123` and must meet the same password policy as the required passwords. | `{}` (all auto-generated)  |

### Password Requirements

Both `keycloak_admin_password` and `keycloak_horizon_admin_password` must satisfy the following policy, which is enforced by Terraform validation before any resources are created:

- Must **not** be the literal value `Change_Me_123`
- Minimum **12 characters**
- At least one **uppercase** letter
- At least one **lowercase** letter
- At least one **number**
- At least one **special character** (e.g. `!`, `@`, `#`, `$`)
- **No whitespace** characters

### Sub-Environment Name Rules

The sub-environment name (the map key) is used directly as a Kubernetes namespace prefix and in GCP resource names. It must:

- Contain only **lowercase letters, numbers, and hyphens** (`-`)
- **Start and end** with a letter or number (not a hyphen)
- Be between **1 and 4 characters** long
- Be **unique** across all entries in `sdv_sub_env_configs`

Valid examples: `dev`, `stg`, `a1`

Invalid examples: `Dev` (uppercase), `-dev` (starts with hyphen), `my_env` (underscore), `staging` (too long)

---

## Deploying a Sub-Environment

Once you have added the `sdv_sub_env_configs` block to `terraform.tfvars`, run the standard deployment workflow. Terraform will detect the new sub-environment entries and create all required resources.

```bash
./deploy.sh --apply
```

Terraform will create the following for each new sub-environment:

1. **GCP IAM service accounts** — one per application (ArgoCD, Jenkins, Keycloak, Gerrit, monitoring, etc.) with the naming convention `gke-<SUB_ENV_NAME>-<app>-sa`.
2. **GCP Secret Manager secrets** — admin passwords, SSH keypairs, and Git credentials, prefixed with `<SUB_ENV_NAME>-`.
3. **GCP Certificate Manager certificate** — for the sub-environment's sub-domain `<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>`.
4. **Cloud DNS CNAME record** — for the certificate DNS authorisation.
5. **Kubernetes namespace** `<SUB_ENV_NAME>-argocd` and an Argo CD Helm release scoped to it.
6. **Argo CD `Application`** (`<SUB_ENV_NAME>-horizon-sdv`) that deploys all workloads into `<SUB_ENV_NAME>-*` namespaces.

**To verify the deployment is healthy:**

1. Check that all new namespaces are present:
   ```bash
   kubectl get namespaces | grep <SUB_ENV_NAME>
   ```
2. Open the sub-environment's Argo CD at `https://<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd` and confirm the `<SUB_ENV_NAME>-horizon-sdv` application is `Synced` and `Healthy`.

> **Note:** The TLS certificate provisioning involves DNS propagation and may take up to 10–15 minutes after the first deployment. The sub-domain will not be reachable over HTTPS until the certificate is active.

---

## Accessing Sub-Environment Applications

All applications in a sub-environment are served under the sub-environment's sub-domain:

```
https://<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>/<app-path>
```

For example, if `SUB_ENV_NAME=dev`, `SUB_DOMAIN=sbx`, and `HORIZON_DOMAIN=example.com`:

| Application          | URL                                                             |
|----------------------|-----------------------------------------------------------------|
| Developer Portal     | `https://dev.sbx.example.com`                                   |
| Argo CD              | `https://dev.sbx.example.com/argocd`                            |
| Keycloak             | `https://dev.sbx.example.com/auth`                              |
| Gerrit               | `https://dev.sbx.example.com/gerrit`                            |
| Jenkins              | `https://dev.sbx.example.com/jenkins`                           |
| Headlamp             | `https://dev.sbx.example.com/headlamp`                          |
| Grafana              | `https://dev.sbx.example.com/grafana`                           |
| MTK Connect          | `https://dev.sbx.example.com/mtk-connect`                       |
| MCP Gateway Registry | `https://mcp.dev.sbx.example.com`                               |

### Available Applications

The following applications are fully deployed and isolated per sub-environment:

- Argo CD
- Keycloak
- Jenkins
- Gerrit *(managed by the shared Gerrit Operator from the main environment)*
- Headlamp
- Grafana
- MTK Connect
- MCP Gateway Registry
- Developer Portal
- PostgreSQL
- Zookeeper
- External DNS

### Shared Components

The following components are **only deployed once** in the main environment and are shared across all sub-environments on the cluster. See [Limitations](#limitations) for the technical reasons.

| Component               | Reason shared                                                  |
|-------------------------|----------------------------------------------------------------|
| External Secrets Operator | Cluster-wide CRDs; multiple installs conflict               |
| Node Exporter           | DaemonSet on host ports; multiple installs cause port conflicts|
| kube-state-metrics      | Deployed in GKE-managed `gmp-public` namespace; not replicable |
| Kubescape Operator      | Cluster-wide scanner; multiple installs cause webhook conflicts|
| Gerrit Operator         | Cluster-scoped operator with non-namespaced RBAC resources     |

---

## Managing Multiple Sub-Environments

You can define as many sub-environments as needed by adding multiple entries to `sdv_sub_env_configs`. Each entry is fully independent.

```hcl
sdv_sub_env_configs = {
  dev = {
    keycloak_admin_password         = "<KEYCLOAK_ADMIN_PASS_DEV>"
    keycloak_horizon_admin_password = "<HORIZON_ADMIN_PASS_DEV>"
    branch                          = "develop"
  }
  stg = {
    keycloak_admin_password         = "<KEYCLOAK_ADMIN_PASS_STG>"
    keycloak_horizon_admin_password = "<HORIZON_ADMIN_PASS_STG>"
    branch                          = "feature/stg-work"
  }
}
```

Each sub-environment will have its own:
- Sub-domain: `dev.<SUB_DOMAIN>.<HORIZON_DOMAIN>` and `stg.<SUB_DOMAIN>.<HORIZON_DOMAIN>`
- Argo CD instance syncing from its configured branch
- Isolated set of namespaces, secrets, and service accounts

> **Tip:** Each sub-environment can track a different Git branch. This lets you validate a feature branch against the live platform before merging.

---

## Destroying a Sub-Environment

To destroy a specific sub-environment without affecting the main environment or other sub-environments, remove its entry from `sdv_sub_env_configs` in `terraform.tfvars` and run the deployment workflow again.

For example, to destroy the `dev` sub-environment:

1. Open `terraform/env/terraform.tfvars`.
2. Remove the `dev` block from `sdv_sub_env_configs` (or remove the entire variable if no other sub-environments exist).
3. Run the deployment again so Terraform drops only that sub-environment:
   ```bash
   ./deploy.sh --apply
   ```

`./deploy.sh --destroy` deprovisions all Terraform-managed infrastructure, including the main environment. Do not use it to remove a single sub-environment.

Terraform will detect the removed entry and destroy only the resources belonging to that sub-environment. The main environment and any remaining sub-environments are not affected.

> **Note:** Destroying a sub-environment removes all its GCP resources (service accounts, secrets, certificates, DNS records) and all its Kubernetes namespaces and workloads. This action is irreversible. Ensure any data you need (e.g. Gerrit repositories, Jenkins build history) is backed up before proceeding.

---

## Limitations

- **Shared cluster infrastructure.** Sub-environments share the same GKE node pool as the main environment. There is no resource quota isolation between environments at the node level.
- **Shared cluster-level operators.** External Secrets, Kubescape, Gerrit Operator, Node Exporter, and kube-state-metrics are installed once and serve the whole cluster. See [Shared Components](#shared-components).
- **Main environment must remain deployed.** Destroying the main environment while sub-environments exist will break the shared cluster-level operators that sub-environments depend on.
- **Sub-domain certificate DNS propagation.** After a new sub-environment is created, TLS will not be active until the Certificate Manager DNS authorisation CNAME propagates and the certificate is provisioned (typically up to 15 minutes).
- **Sub-environment name is immutable.** Renaming a sub-environment requires destroying the old one and creating a new one, as the name is embedded in all GCP and Kubernetes resource names.

---

## Troubleshooting

### Certificate not provisioning / sub-domain not reachable over HTTPS

The Certificate Manager certificate requires a DNS CNAME record to be resolvable before it can be issued. This normally resolves within 15 minutes. Verify the CNAME record exists in your Cloud DNS zone:

```bash
gcloud dns record-sets list --zone=<DNS_ZONE_NAME> --filter="type=CNAME"
```

If the record exists but the certificate is still not active, check the certificate status in the GCP Console under **Certificate Manager → Certificates**.

### Argo CD sub-environment instance not syncing

If the Argo CD instance for the sub-environment is stuck in `Unknown` or `Progressing`, the most likely cause is that the ExternalSecret for the Argo CD admin password has not yet been populated. Check the ExternalSecret status:

```bash
kubectl get externalsecret -n <SUB_ENV_NAME>-argocd
```

If the ExternalSecret shows an error, verify the GCP Secret Manager secret `<SUB_ENV_NAME>-argocd-admin-password-b64` exists and is accessible by the sub-environment's Workload Identity service account.

### Terraform validation error on password

If Terraform fails with a password validation error, ensure both `keycloak_admin_password` and `keycloak_horizon_admin_password` meet all the requirements listed in [Password Requirements](#password-requirements). They must not be the literal value `Change_Me_123`, must not contain whitespace, and must include at least one character from each of the four categories: uppercase, lowercase, number, and special character.

### Terraform validation error on sub-environment name

If Terraform fails with a name validation error, ensure the sub-environment name follows the rules in [Sub-Environment Name Rules](#sub-environment-name-rules). The name must be all lowercase, must not start or end with a hyphen, must not contain underscores or uppercase letters, and must be between 1 and 4 characters long.
