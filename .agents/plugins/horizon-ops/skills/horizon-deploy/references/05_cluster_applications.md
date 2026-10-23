<!-- Copyright (c) 2026 Accenture and Google LLC, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Phase 5: Cluster Applications & Endpoints Reference

This document summarizes the web applications deployed on the Horizon SDV platform, their URL paths, and access methods.

---

## 1. Application Endpoints Directory

All applications are hosted under the base domain `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`.

| Application | Path / URL | Authentication | Description |
| :--- | :--- | :--- | :--- |
| **Developer Portal** | `/developer-portal/` (or `/`) | Keycloak SSO | Central dashboard for platform workflows, module management, and application links. |
| **Argo CD** | `/argocd` | Keycloak SSO | Declarative GitOps deployment controller syncing Kubernetes manifests. |
| **Keycloak IAM** | `/auth/` (Admin console: `/auth/admin/horizon/console`) | Keycloak SSO | Central Identity & Access Management providing OAuth2 / OIDC SSO. |
| **Gerrit** | `/gerrit` | Keycloak SSO | Code review and Git source repository hosting. |
| **Jenkins** | `/jenkins` | Keycloak SSO | Continuous Integration & build automation for Android & Cuttlefish workloads. |
| **MTK Connect** | `/mtk-connect` | Keycloak SSO | Remote device connectivity manager for physical and virtual test devices. |
| **Headlamp** | `/headlamp/` | Keycloak SSO | Extensible web UI for inspecting and managing Kubernetes cluster resources. |
| **Grafana** | `/grafana` | Keycloak SSO | Metrics, dashboards, and cluster observability. |
| **Argo Workflows** | `/workflows` | Keycloak SSO | Workflow engine UI for Horizon pipelines (Argo Workflows server). |
| **Horizon API** | `/horizon-api/` | Keycloak token (OIDC) | REST API used by the Developer Portal, the horizon CLI and CI clients. |
| **MCP Gateway Registry** | `https://mcp.<SUB_DOMAIN>.<HORIZON_DOMAIN>` | Keycloak SSO | Centralized registry for discovering and executing Model Context Protocol (MCP) servers and AI agents. |

> [!NOTE]
> Google sign-in is optional. It is an identity provider configured in Keycloak ([04_post_deployment_setup.md §3.2](./04_post_deployment_setup.md#32-configure-google-identity-provider)). Without it, users sign in with Keycloak username and password.

---

## 2. Multi-Tenant Sub-Environments

When sub-environments are configured in `sdv_sub_env_configs`, each sub-environment has isolated instances of applications reachable via:

```
https://<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>/
```

Examples:
- Developer Portal: `https://<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>/developer-portal/`
- Jenkins: `https://<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>/jenkins`
- Argo CD: `https://<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd`

---

## 3. Platform Admin Passwords & Secret Manager Retrieval

> [!IMPORTANT]
> **Proactive Display Policy**:
> During deployment handoff, the agent must **only present the locally-generated passwords** (`horizon-admin`, Keycloak `admin`, and Argo CD `admin` from `manual_secrets.s5`).
> Passwords generated natively inside Terraform (Jenkins, Gerrit, Grafana, PostgreSQL, MCP Gateway) are synced directly to workloads via External Secrets and must **not** be displayed proactively. They are listed below purely as a reference for emergency break-glass scenarios.

### Proactively Presented Credentials

| Account | Username | Purpose & Recommended Usage | Secret Manager Key | Retrieval Command |
| :--- | :--- | :--- | :--- | :--- |
| **Horizon SSO Admin** *(Recommended)* | `horizon-admin` | **Primary SSO login for all applications** (Argo CD, Jenkins, Gerrit, Grafana, Headlamp, Developer Portal) via **"LOG IN VIA KEYCLOAK"**, and at `/auth/admin/horizon/console/`. **Initial password only:** it must be changed at first login. | `keycloak-horizon-admin-password-b64` | `gcloud secrets versions access latest --secret="keycloak-horizon-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Keycloak Root Superadmin** | `admin` | **Root Keycloak server administration only** at `/auth/admin/master/console/`. | `keycloak-admin-password-b64` | `gcloud secrets versions access latest --secret="keycloak-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Argo CD** *(Break-glass)* | `admin` | Direct Argo CD login at `/argocd` if SSO is unavailable. | — (Secret Manager holds only a bcrypt hash) | Read `manual_secrets.s5` from `terraform/env/terraform.tfvars` |

### Emergency / Fallback Credentials (Terraform-Generated — On-Demand Only)

| Service | Login Account / Username | Purpose | Secret Manager Key | Terminal Retrieval Command |
| :--- | :--- | :--- | :--- | :--- |
| **Jenkins** *(Fallback)* | `admin` | Local fallback admin for direct Jenkins login. | `jenkins-admin-password-b64` | `gcloud secrets versions access latest --secret="jenkins-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Gerrit** *(Fallback)* | `admin` | Local fallback admin for direct Gerrit login. | `gerrit-admin-password-b64` | `gcloud secrets versions access latest --secret="gerrit-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Grafana** *(Fallback)* | `admin` | Local fallback admin for direct Grafana login. | `grafana-admin-password-b64` | `gcloud secrets versions access latest --secret="grafana-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **PostgreSQL** | `postgres` | In-cluster relational database superuser. | `postgres-admin-password-b64` | `gcloud secrets versions access latest --secret="postgres-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **MCP Registry** | `admin` | Model Context Protocol gateway registry admin. | `mcp-gateway-registry-admin-password-b64` | `gcloud secrets versions access latest --secret="mcp-gateway-registry-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
