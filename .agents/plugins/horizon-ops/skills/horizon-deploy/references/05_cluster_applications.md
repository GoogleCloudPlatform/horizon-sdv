# Phase 5: Cluster Applications & Endpoints Reference

This document summarizes the web applications deployed on the Horizon SDV platform, their URL paths, and access methods.

---

## 1. Application Endpoints Directory

All applications are hosted under the base domain `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`.

| Application | Path / URL | Authentication | Description |
| :--- | :--- | :--- | :--- |
| **Developer Portal** | `/developer-portal/` (or `/`) | Keycloak / Google | Central dashboard for platform workflows, module management, and application links. |
| **Argo CD** | `/argocd` | Keycloak SSO | Declarative GitOps deployment controller syncing Kubernetes manifests. |
| **Keycloak IAM** | `/auth/` (Admin console: `/auth/admin/horizon/console`) | Local Admin / Google | Central Identity & Access Management providing OAuth2 / OIDC SSO. |
| **Gerrit** | `/gerrit` | Google SSO | Code review and Git source repository hosting. |
| **Jenkins** | `/jenkins` | Google SSO | Continuous Integration & build automation for Android & Cuttlefish workloads. |
| **MTK Connect** | `/mtk-connect` | Google SSO | Remote device connectivity manager for physical and virtual test devices. |
| **Headlamp** | `/headlamp` | Google SSO | Extensible web UI for inspecting and managing Kubernetes cluster resources. |
| **Grafana** | `/grafana` | Google SSO | Metrics, dashboards, and cluster observability. |
| **MCP Gateway Registry** | `https://mcp.<SUB_DOMAIN>.<HORIZON_DOMAIN>` | Keycloak / Google | Centralized registry for discovering and executing Model Context Protocol (MCP) servers and AI agents. |

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
> During deployment handoff, the agent must **only present the two locally-generated Keycloak passwords** (`horizon-admin` and `admin`).
> Passwords generated natively inside Terraform (Argo CD, Jenkins, Gerrit, Grafana, PostgreSQL, MCP Gateway) are synced directly to workloads via External Secrets and must **not** be displayed proactively. They are listed below purely as a reference for emergency break-glass scenarios.

### Proactively Presented Credentials (Keycloak SSO)

| Account | Username | Purpose & Recommended Usage | Secret Manager Key | Retrieval Command |
| :--- | :--- | :--- | :--- | :--- |
| **Horizon SSO Admin** *(Recommended)* | `horizon-admin` | **Primary SSO login for all applications** (Argo CD, Jenkins, Gerrit, Grafana, Headlamp, Developer Portal) via **"LOG IN VIA KEYCLOAK"**, and at `/auth/admin/horizon/console/`. | `keycloak-horizon-admin-password-b64` | `gcloud secrets versions access latest --secret="keycloak-horizon-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Keycloak Root Superadmin** | `admin` | **Root Keycloak server administration only** at `/auth/admin/master/console/`. | `keycloak-admin-password-b64` | `gcloud secrets versions access latest --secret="keycloak-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |

### Emergency / Fallback Credentials (Terraform-Generated — On-Demand Only)

| Service | Login Account / Username | Purpose | Secret Manager Key | Terminal Retrieval Command |
| :--- | :--- | :--- | :--- | :--- |
| **Argo CD** *(Fallback)* | `admin` | Local fallback admin for direct Argo CD login. | `argocd-admin-password-b64` | `gcloud secrets versions access latest --secret="argocd-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Jenkins** *(Fallback)* | `admin` | Local fallback admin for direct Jenkins login. | `jenkins-admin-password-b64` | `gcloud secrets versions access latest --secret="jenkins-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Gerrit** *(Fallback)* | `admin` | Local fallback admin for direct Gerrit login. | `gerrit-admin-password-b64` | `gcloud secrets versions access latest --secret="gerrit-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **Grafana** *(Fallback)* | `admin` | Local fallback admin for direct Grafana login. | `grafana-admin-password-b64` | `gcloud secrets versions access latest --secret="grafana-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **PostgreSQL** | `postgres` | In-cluster relational database superuser. | `postgres-admin-password-b64` | `gcloud secrets versions access latest --secret="postgres-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
| **MCP Registry** | `admin` | Model Context Protocol gateway registry admin. | `mcp-gateway-registry-admin-password-b64` | `gcloud secrets versions access latest --secret="mcp-gateway-registry-admin-password-b64" --project=<PROJECT_ID> \| base64 --decode && echo` |
