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

# Sub-Environment Developer Guide

This guide explains how the sub-environment feature works internally and provides a step-by-step checklist for adding new applications that must support sub-environments.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [How the Namespace Prefix Works](#how-the-namespace-prefix-works)
- [GCP Resources Provisioned Per Sub-Environment](#gcp-resources-provisioned-per-sub-environment)
  - [Workload Identity Service Accounts](#workload-identity-service-accounts)
  - [GCP Secret Manager Secrets](#gcp-secret-manager-secrets)
  - [GCP Parameter Manager](#gcp-parameter-manager)
  - [Certificate Manager](#certificate-manager)
  - [Cloud DNS](#cloud-dns)
- [Kubernetes Resources Provisioned Per Sub-Environment](#kubernetes-resources-provisioned-per-sub-environment)
- [The `all_environments` Local](#the-all_environments-local)
- [Helm Values: `namespacePrefix`, `isSubEnvironment`, `environmentName`](#helm-values-namespaceprefix-issubenvironment-environmentname)
- [How to Add a New Application](#how-to-add-a-new-application)
- [Components Not Replicated to Sub-Environments](#components-not-replicated-to-sub-environments)
- [Naming Conventions Reference](#naming-conventions-reference)

---

## Overview

The sub-environment feature allows multiple isolated copies of the Horizon SDV platform to run on the same GKE cluster. Isolation is achieved through a single principle: every Kubernetes resource name, namespace name, GCP secret ID, and GCP service account name belonging to a sub-environment is prefixed with the sub-environment's name followed by a hyphen (e.g. `dev-`). The main environment uses an empty prefix.

This prefix is threaded from `terraform.tfvars` all the way through Terraform, into the Argo CD `Application` Helm values, and finally into every GitOps template in the repository. No manual per-environment branching of manifests is required.

---

## Architecture

The following diagram shows how the sub-environment name flows through the system:

```
terraform.tfvars
  └── sdv_sub_env_configs = { "dev" = { ... } }
        │
        ▼
terraform/env/locals.tf
  └── Generates: sub_env_service_accounts, sub_env_secrets, sub_env_git_secrets
        │
        ▼
terraform/env/main.tf  ──►  GCP IAM Service Accounts
  └── module "base"    ──►  GCP Secret Manager Secrets
        │              ──►  Certificate Manager (per-domain)
        │              ──►  Cloud DNS (CNAME per sub-env)
        ▼
terraform/modules/sdv-gke-apps/main.tf
  └── all_environments local (main + sub-envs)
        │
        ├── kubernetes_namespace.argocd["dev"]    → dev-argocd
        ├── helm_release.argocd_subenvs["dev"]   → ArgoCD in dev-argocd
        └── kubectl_manifest.argocd_application["dev"]
              └── Helm values:
                    namespacePrefix:  "dev-"
                    isSubEnvironment: true
                    environmentName:  "dev"
                          │
                          ▼
              gitops/templates/*.yaml
                └── {{ .Values.config.namespacePrefix }}jenkins  →  dev-jenkins
                └── {{ .Values.config.namespacePrefix }}keycloak →  dev-keycloak
                └── ...
```

---

## How the Namespace Prefix Works

Every GitOps template that creates a Kubernetes resource uses `{{ .Values.config.namespacePrefix }}` as a prefix on all namespace and resource names. For the main environment this value is an empty string, so templates render identically to how they did before. For a sub-environment named `dev`, the prefix is `dev-`.

**Before (hardcoded namespace):**
```yaml
metadata:
  name: jenkins
  namespace: jenkins
```

**After (namespace-prefix-aware):**
```yaml
metadata:
  name: {{ .Values.config.namespacePrefix }}jenkins
  namespace: {{ .Values.config.namespacePrefix }}jenkins
```

Cross-namespace service references (e.g. a hostname used inside a Helm values block) follow the same pattern:

```yaml
# Before
hostname: postgresql

# After
hostname: {{ .Values.config.namespacePrefix }}postgresql
```

The `isSubEnvironment` flag (boolean) is used to gate resources that must only exist once per cluster (cluster-scoped operators, node-level DaemonSets, etc.):

```yaml
{{- if not .Values.config.isSubEnvironment }}
# This block is rendered for the main environment only
{{- end }}
```

---

## GCP Resources Provisioned Per Sub-Environment

### Workload Identity Service Accounts

One GCP IAM service account is created for each application in each sub-environment. Each SA is bound to the corresponding Kubernetes service account in the sub-environment's namespace via Workload Identity.

| Template Key       | GCP SA Name                             | Kubernetes Namespace / SA                                     | IAM Roles (summary)                                                   |
|--------------------|-----------------------------------------|---------------------------------------------------------------|-----------------------------------------------------------------------|
| `argocd`           | `gke-<env>-argocd-sa`                   | `<env>-argocd / argocd-sa`                                    | `secretmanager.secretAccessor`, `iam.serviceAccountTokenCreator`      |
| `jenkins`          | `gke-<env>-jenkins-sa`                  | `<env>-jenkins / jenkins-sa`, `<env>-jenkins / jenkins`       | Storage, Artifact Registry, Secret Manager, Container Admin, and more |
| `keycloak`         | `gke-<env>-keycloak-sa`                 | `<env>-keycloak / keycloak-sa`                                | `secretmanager.secretAccessor`, `iam.serviceAccountTokenCreator`      |
| `gerrit`           | `gke-<env>-gerrit-sa`                   | `<env>-gerrit / gerrit-sa`                                    | `secretmanager.secretAccessor`, `iam.serviceAccountTokenCreator`      |
| `monitoring`       | `<env>-monitoring-sa`                   | `<env>-monitoring / monitoring-sa`                            | `iam.workloadIdentityUser`, `monitoring.viewer`                       |
| `monitoring_writer`| `<env>-monitoring-writer-sa`            | `<env>-monitoring / monitoring-writer-sa`                     | `monitoring.metricWriter`, `monitoring.viewer`, and more              |
| `tf_wl`            | `gke-<env>-tf-wl-sa`                    | `<env>-jenkins / terraform-workloads-sa`                      | Storage, Artifact Registry, Container Admin, and more                 |
| `external_dns`     | `<env>-external-dns-sa`                 | `<env>-external-dns / external-dns-sa`                        | `dns.admin`                                                           |
| `mcp_gateway`      | `gke-<env>-mcp-gateway-sa`              | `<env>-mcp-gateway-registry / mcp-gateway-registry-sa`        | `secretmanager.secretAccessor`, `iam.serviceAccountTokenCreator`      |

**SA naming convention:**
- Most application SAs use the `gke` prefix style: `gke-<env>-<app>-sa`
- Monitoring and external-dns SAs use the plain style: `<env>-<app>-sa`

This is controlled by the `prefix_style` field in each `sa_templates` entry in `terraform/env/locals.tf`.

### GCP Secret Manager Secrets

One set of secrets is created per sub-environment. All secret IDs are prefixed with the sub-environment name.

| Secret Key                | GCP Secret ID                                      | Value source                    | Accessible by (namespace / SA)                  |
|---------------------------|----------------------------------------------------|---------------------------------|--------------------------------------------------|
| `keycloak_idp_client`     | `<env>-keycloak-idp-client-secret`                 | Placeholder (`dummy`)           | `<env>-keycloak / keycloak-sa`                  |
| `argocd_admin`            | `<env>-argocd-admin-password-b64`                  | Auto-generated (bcrypt + b64)   | `<env>-argocd / argocd-sa`                      |
| `jenkins_admin`           | `<env>-jenkins-admin-password-b64`                 | Auto-generated (b64)            | `<env>-jenkins / jenkins-sa`                    |
| `keycloak_admin`          | `<env>-keycloak-admin-password-b64`                | From `keycloak_admin_password`  | `<env>-keycloak / keycloak-sa`                  |
| `gerrit_admin`            | `<env>-gerrit-admin-password-b64`                  | Auto-generated (b64)            | `<env>-gerrit / gerrit-sa`                      |
| `gerrit_ssh`              | `<env>-gerrit-admin-ssh-key-b64`                   | Generated ECDSA P-521 keypair   | `<env>-gerrit / gerrit-sa`                      |
| `jenkins_cuttlefish_ssh`  | `<env>-jenkins-cuttlefish-ssh-key-b64`             | Generated RSA 4096 keypair      | `<env>-jenkins / jenkins-sa`                    |
| `grafana_admin`           | `<env>-grafana-admin-password-b64`                 | Auto-generated (b64)            | `<env>-monitoring / monitoring-sa`              |
| `keycloak_horizon_admin`  | `<env>-keycloak-horizon-admin-password-b64`        | From `keycloak_horizon_admin_password` | `<env>-jenkins / jenkins-sa`            |
| `postgres_admin`          | `<env>-postgres-admin-password-b64`                | Auto-generated (b64)            | `<env>-keycloak / keycloak-sa`                  |
| `mcp_gateway_admin`       | `<env>-mcp-gateway-registry-admin-password-b64`    | Auto-generated (b64)            | `<env>-mcp-gateway-registry / mcp-gateway-registry-sa` |

In addition, Git authentication secrets are created per sub-environment, prefixed with `<env>-`:

- **GitHub App** (`scm_auth_method = "app"`): `<env>-github-app-id-b64`, `<env>-github-app-installation-id-b64`, `<env>-github-app-private-key-b64`, `<env>-github-app-private-key-pkcs8-b64`
- **Username/password or token** (`scm_auth_method = "userpass"`): `<env>-scm-password-b64`

Secrets that can be set manually (instead of auto-generated) are identified by their legacy `s5`/`s6`/`s9`/`s12`/`s14`/`s17` keys in the `manual_secrets` map within `sdv_sub_env_configs`.

### GCP Parameter Manager

A single GCP Parameter Manager parameter `sdv_sub_environments` (parameter ID `p11` in `terraform/env/main.tf`) is created in the main environment. Its value is a base64-encoded JSON array of all active sub-environment names:

```json
["dev", "alice"]
```

This parameter is consumed by the deployment workflow and can be referenced at runtime to enumerate sub-environments without reading `terraform.tfvars` directly.

### Certificate Manager

For each sub-environment, the `sdv-certificate-manager` module creates:

- A **DNS authorisation** resource named `horizon-sdv-dns-auth-<env>` for the domain `<env>.<SUB_DOMAIN>.<HORIZON_DOMAIN>`, created when `certificate_authorization_type = "dns"`.
- A **certificate** named `horizon-sdv-<env>` covering both the domain and its wildcard (`*.<env>.<SUB_DOMAIN>.<HORIZON_DOMAIN>`), backed by the DNS authorisation above.
- **Certificate map entries** `horizon-sdv-entry-<env>` (apex) and `horizon-sdv-entry-<env>-wildcard` (DNS auth), or `horizon-sdv-entry-<env>-mcp` (load-balancer auth), in the shared certificate map, matching requests by hostname.

The main environment certificate uses the key `main` (e.g. `horizon-sdv-main`).

### Cloud DNS

For each sub-environment, one additional CNAME record is added to the existing Cloud DNS zone. The record points to the DNS authorisation token required by Certificate Manager to verify domain ownership for the sub-environment's sub-domain.

These records are managed via the `sdv-dns-zone` module using a `count`-based iteration over the `dns_auth_records` list output from the certificate manager module.

---

## Kubernetes Resources Provisioned Per Sub-Environment

The `sdv-gke-apps` Terraform module provisions the following Kubernetes resources for every entry in `all_environments` (using `for_each`):

| Resource                              | Name / Namespace (sub-env example with `dev-` prefix)         |
|---------------------------------------|---------------------------------------------------------------|
| `kubernetes_namespace`                | `dev-argocd`                                                  |
| `kubernetes_service_account`          | `argocd-sa` in `dev-argocd`                                   |
| `kubernetes_secret` (Git creds)       | `argocd-scm-creds` in `dev-argocd`                            |
| `kubernetes_secret` (ArgoCD admin)    | `argocd-secret` in `dev-argocd`                               |
| `kubectl_manifest` (SecretStore)      | `argocd-secret-store` in `dev-argocd`                         |
| `kubectl_manifest` (ExternalSecret)   | `argocd-scm-creds` ExternalSecret in `dev-argocd`             |
| `kubectl_manifest` (ExternalSecret)   | `argocd-secret` ExternalSecret in `dev-argocd`                |
| `kubectl_manifest` (AppProject)       | `dev-horizon-sdv` AppProject in `dev-argocd`                  |
| `kubectl_manifest` (Application)      | `dev-horizon-sdv` Application in `dev-argocd`                 |
| `helm_release` (ArgoCD)               | `dev-argocd` Helm release in `dev-argocd` (CRDs skipped)      |

The ArgoCD Helm release for sub-environments (`helm_release.argocd_subenvs`) sets `skip_crds = true` because the CRDs are already installed by the main environment's `helm_release.argocd_main`. The sub-env releases depend on `helm_release.argocd_main` to ensure CRDs are available before the sub-env Helm install runs.

---

## The `all_environments` Local

The `all_environments` local in `terraform/modules/sdv-gke-apps/main.tf` is the central data structure that drives all `for_each` iterations in the module. It merges the main environment entry (static) with one entry per sub-environment (from `var.sub_environments`).

```hcl
locals {
  all_environments = merge(
    {
      "main" = {
        namespace_prefix = ""
        argocd_namespace = var.argocd_namespace   # "argocd"
        subdomain        = var.subdomain_name
        is_main          = true
        env_name         = "main"
        branch           = var.scm_repo_branch
      }
    },
    {
      for env in var.sub_environments : env => {
        namespace_prefix = "${env}-"
        argocd_namespace = "${env}-argocd"
        subdomain        = "${env}.${var.subdomain_name}"
        is_main          = false
        env_name         = env
        branch           = lookup(var.sub_env_branches, env, var.scm_repo_branch)
      }
    }
  )
}
```

| Field              | Main environment | Sub-environment (`dev`)      | Purpose                                                   |
|--------------------|------------------|------------------------------|-----------------------------------------------------------|
| `namespace_prefix` | `""`             | `"dev-"`                     | Prepended to all Kubernetes resource and namespace names  |
| `argocd_namespace` | `"argocd"`       | `"dev-argocd"`               | ArgoCD Helm release and namespace target                  |
| `subdomain`        | `"sbx"`          | `"dev.sbx"`                  | Used in the Argo CD Application's `config.domain`         |
| `is_main`          | `true`           | `false`                      | Controls the `isSubEnvironment` Helm value                |
| `env_name`         | `"main"`         | `"dev"`                      | Used in AppProject descriptions and GCP SA annotation     |
| `branch`           | `"main"`         | `"feature/dev"` (or default) | Git branch for Argo CD to sync from                       |

---

## Helm Values: `namespacePrefix`, `isSubEnvironment`, `environmentName`

The `argocd_application` `kubectl_manifest` resource in `sdv-gke-apps/main.tf` passes three values into the Argo CD `Application`'s Helm `values` block that every GitOps template consumes:

```yaml
config:
  namespacePrefix: "dev-"       # empty string "" for main environment
  isSubEnvironment: true        # false for main environment
  environmentName: "dev"        # "main" for main environment
```

These values are available in all GitOps templates under `gitops/templates/` as:
- `.Values.config.namespacePrefix`
- `.Values.config.isSubEnvironment`
- `.Values.config.environmentName`

When an Argo CD sub-chart application (e.g. `gitops/apps/gerrit/`) needs the prefix, the parent template passes it down through the sub-chart's `values` block:

```yaml
# gitops/templates/gerrit.yaml (excerpt)
helm:
  values: |
    config:
      domain: {{ .Values.config.domain }}
      namespacePrefix: {{ .Values.config.namespacePrefix }}
```

Post-configuration container scripts (in `terraform/modules/sdv-container-images/`) receive the prefix through the `NAMESPACE_PREFIX` environment variable and use it when constructing `kubectl` or `curl` API calls:

```bash
kubectl get pods -n ${NAMESPACE_PREFIX}gerrit
curl ... ${APISERVER}/api/v1/namespaces/${NAMESPACE_PREFIX}jenkins/secrets/...
```

---

## How to Add a New Application

Follow this checklist when adding a new application that must run in both the main environment and all sub-environments.

### 1. Update the top-level ArgoCD Application template

File: `gitops/templates/<app>.yaml`

- Prefix the ArgoCD `Application` `metadata.name` and `metadata.namespace`:
  ```yaml
  metadata:
    name: {{ .Values.config.namespacePrefix }}<app>
    namespace: {{ .Values.config.namespacePrefix }}argocd
  ```
- Prefix `spec.project`:
  ```yaml
  spec:
    project: {{ .Values.config.namespacePrefix }}horizon-sdv
  ```
- Prefix the destination namespace:
  ```yaml
  destination:
    namespace: {{ .Values.config.namespacePrefix }}<app>
  ```
- Pass `namespacePrefix` into the Helm values block if the sub-chart needs it:
  ```yaml
  helm:
    values: |
      config:
        namespacePrefix: {{ .Values.config.namespacePrefix }}
  ```
- Apply `{{ .Values.config.namespacePrefix }}` to all `NetworkPolicy` `metadata.namespace` fields and all `namespaceSelector.matchLabels` values that reference other application namespaces.

### 2. Update the init template (if present)

File: `gitops/templates/<app>-init.yaml`

- Prefix all `ServiceAccount`, `Role`, `ClusterRole`, `RoleBinding`, `ClusterRoleBinding`, `StorageClass`, `PersistentVolumeClaim`, and `Secret` resource names and namespaces with `{{ .Values.config.namespacePrefix }}`.
- For `ClusterRole` and `ClusterRoleBinding` names, use the prefix to avoid name collisions between environments:
  ```yaml
  metadata:
    name: {{ .Values.config.namespacePrefix }}<app>-cluster-role
  ```
- For `ServiceAccount` WI annotations, prefix the GCP SA name accordingly:
  ```yaml
  annotations:
    iam.gke.io/gcp-service-account: gke-{{ .Values.config.namespacePrefix }}<app>-sa@{{ .Values.config.projectID }}.iam.gserviceaccount.com
  ```

### 3. Update the sub-chart app templates (if present)

File: `gitops/apps/<app>/templates/<app>.yaml`

- Apply `{{ .Values.config.namespacePrefix }}` to the primary resource's `metadata.name` and `metadata.namespace`.
- Prefix any internal hostname references that resolve Kubernetes services in other namespaces:
  ```yaml
  # Example: zookeeper connect string
  connectString: {{ .Values.config.namespacePrefix }}zookeeper.{{ .Values.config.namespacePrefix }}zookeeper.svc.cluster.local:2181
  ```

### 4. Update post-configuration scripts (if present)

File: `terraform/modules/sdv-container-images/images/<app>/<app>-post/configure.sh`

- Replace all hard-coded namespace strings with `${NAMESPACE_PREFIX}<namespace>`:
  ```bash
  # Before
  kubectl get pods -n my-app
  # After
  kubectl get pods -n ${NAMESPACE_PREFIX}my-app
  ```
- For Kubernetes API `curl` calls, apply the prefix to the namespace path segment:
  ```bash
  # Before
  curl ... /api/v1/namespaces/my-app/secrets/...
  # After
  curl ... /api/v1/namespaces/${NAMESPACE_PREFIX}my-app/secrets/...
  ```
- For any JSON secret template files that embed a namespace, add a `##NAMESPACE##` placeholder and substitute it with `sed`:
  ```bash
  sed -i "s/##NAMESPACE##/${NAMESPACE_PREFIX}my-app/g" ./my-secret.json
  ```

### 5. Add a Workload Identity service account template (if needed)

File: `terraform/env/locals.tf` — `sa_templates` map

If the new application needs its own GCP IAM service account with Workload Identity, add an entry to the `sa_templates` map. This causes Terraform to automatically generate one SA per sub-environment:

```hcl
my_app = {
  name         = "my-app"
  prefix_style = "gke"         # use "plain" if you don't want the "gke-" prefix
  gke_sas = [
    { ns = "my-app", sa = "my-app-sa" }
  ]
  roles = toset([
    "roles/secretmanager.secretAccessor",
    "roles/iam.serviceAccountTokenCreator",
  ])
}
```

Update the `jenkins-init.yaml` (or the appropriate init template) to annotate the Kubernetes `ServiceAccount` with the correct WI annotation using `namespacePrefix`.

### 6. Add a secret template (if needed)

File: `terraform/env/locals.tf` — `secret_templates` map

If the new application needs a GCP-managed secret per sub-environment, add an entry to `secret_templates` and provide the corresponding value in `_sub_env_secret_values`:

```hcl
# In secret_templates:
my_app_admin = {
  secret_suffix    = "my-app-admin-password-b64"
  apply_value      = false
  ns               = "my-app"
  sa               = "my-app-sa"
  value_key        = "my_app_admin"
}

# In _sub_env_secret_values (for env in sdv_sub_env_configs):
my_app_admin = base64encode(local.get_sub_env_password[env]["s_my_app"])
```

Add the corresponding auto-generate spec to `secret_password_specs` in `terraform/env/locals.tf` if a random password is needed.

### 7. Gate cluster-scoped resources with `isSubEnvironment`

If any resource in your application is **cluster-scoped** (e.g. a `ClusterRole`, `MutatingWebhookConfiguration`, `CustomResourceDefinition`, or a `DaemonSet` on host ports) and must only be installed once per cluster, wrap it with:

```yaml
{{- if not .Values.config.isSubEnvironment }}
# cluster-scoped resource here
{{- end }}
```

Document why the resource is gated in the [Components Not Replicated to Sub-Environments](#components-not-replicated-to-sub-environments) section of this guide and in the corresponding template file.

---

## Components Not Replicated to Sub-Environments

The following cluster-level components are gated with `{{- if not .Values.config.isSubEnvironment }}` and are only deployed by the main environment's Argo CD instance.

### External Secrets Operator

The External Secrets Operator installs cluster-wide CRDs (`ClusterSecretStore`, `ExternalSecret`, etc.) on its first Helm install. A second Helm release for the same CRDs on the same cluster will either fail or overwrite the existing CRD ownership, breaking the first installation. The operator is designed to run as a single cluster-wide instance; all sub-environment namespaces create their own namespace-scoped `SecretStore` resources and consume the operator without requiring a separate install.

### Node Exporter

Node Exporter is deployed as a `DaemonSet` that runs one pod per cluster node and binds to a fixed host network port (default `9100`). Because sub-environments share the same nodes as the main environment, a second `DaemonSet` for the same port causes pod scheduling failures or port conflicts. A single installation by the main environment provides node-level metrics for the entire cluster.

### kube-state-metrics

In this cluster, `kube-state-metrics` is deployed into the `gmp-public` namespace, which is a GKE-managed namespace created and owned by Google Managed Prometheus. This namespace cannot be replicated or substituted with a prefixed equivalent, and only one `kube-state-metrics` instance can exist in it per cluster.

### Kubescape Operator

Kubescape is a cluster-wide security scanner that:
- Installs cluster-scoped admission webhooks (`MutatingWebhookConfiguration`) with fixed names that cannot be namespaced.
- Produces scan results that cover the entire cluster, so multiple instances would duplicate reports.
- Creates Kubernetes resources automatically (scan jobs, results CRDs) that would be placed outside the sub-environment's prefixed namespaces, making them unmanageable by sub-environment Argo CD.

### Gerrit Operator

The Gerrit Operator (`gerrit-operator`) is a cluster-scoped Kubernetes operator. It:
- Creates `ClusterRoles` and `ClusterRoleBindings` with fixed, non-namespaced names. A second installation would attempt to reconcile and overwrite these, causing conflicts.
- Owns the `GerritCluster` CRD. Multiple Argo CD applications deploying the operator would fight over CRD ownership.

Sub-environments deploy their own `GerritCluster` custom resource (in `<env>-gerrit`) and rely on the single Gerrit Operator instance installed by the main environment to reconcile it.

---

## Naming Conventions Reference

The table below summarises the naming conventions used for all sub-environment resources, comparing the main environment (empty prefix) to a sub-environment named `dev`.

| Resource type                     | Main environment                     | Sub-environment (`dev`)                   |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| Kubernetes namespace (Jenkins)    | `jenkins`                            | `dev-jenkins`                             |
| Kubernetes namespace (ArgoCD)     | `argocd`                             | `dev-argocd`                              |
| ArgoCD Application name           | `horizon-sdv`                        | `dev-horizon-sdv`                         |
| ArgoCD AppProject name            | `horizon-sdv`                        | `dev-horizon-sdv`                         |
| GCP IAM SA (Jenkins)              | `gke-jenkins-sa`                     | `gke-dev-jenkins-sa`                      |
| GCP IAM SA (monitoring)           | `monitoring-sa`                      | `dev-monitoring-sa`                       |
| GCP Secret (ArgoCD admin)         | `argocd-admin-password-b64`          | `dev-argocd-admin-password-b64`           |
| GCP Secret (SCM password/token)   | `scm-password-b64`                   | `dev-scm-password-b64`                    |
| Certificate Manager certificate   | `horizon-sdv-main`                   | `horizon-sdv-dev`                         |
| Certificate Manager DNS auth      | `horizon-sdv-dns-auth`               | `horizon-sdv-dns-auth-dev`                |
| Certificate map entry             | `horizon-sdv-entry-main` and `horizon-sdv-entry-main-wildcard` or `horizon-sdv-entry-main-mcp` | `horizon-sdv-entry-dev` and `horizon-sdv-entry-dev-wildcard` or `horizon-sdv-entry-dev-mcp` |
| Sub-domain                        | `<SUB_DOMAIN>.<HORIZON_DOMAIN>`      | `dev.<SUB_DOMAIN>.<HORIZON_DOMAIN>`       |
| StorageClass (Gerrit)             | `gerrit-rwx`                         | `dev-gerrit-rwx`                          |
| ClusterRole (Terraform workloads) | `terraform-workloads-writer-cluster-role` | `dev-terraform-workloads-writer-cluster-role` |
