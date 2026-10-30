<!-- Copyright 2026 Google LLC
Modifications Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Phase 4: Post-Deployment Setup

This runbook covers critical post-deployment configurations required to bring the cluster to fully operational status.

---

## Step 1: DNS & Nameserver Configuration

To route internet traffic to the GKE ingress, update your domain nameservers.

### 1.1 Retrieve Live Nameservers from Google Cloud
When Terraform creates the subdomain zone `<SUB_DOMAIN>-horizon-sdv-com` (DNS Name: `<SUB_DOMAIN>.<HORIZON_DOMAIN>.`):
1. Query the live nameservers directly from Google Cloud:
   ```bash
   ZONE_NAME="<SUB_DOMAIN>-horizon-sdv-com"
   gcloud dns managed-zones describe $ZONE_NAME --project=<GCP_PROJECT_ID> --format="value(nameServers)"
   ```
2. In your DNS provider console for `<HORIZON_DOMAIN>`, add an `NS` record set for subdomain `<SUB_DOMAIN>` containing the exact 4 returned nameservers.
3. Save the records and allow standard DNS propagation.

### 1.2 DNSSEC Authorization (If `sdv_dns_dnssec_enabled = true`)
1. In [Google Search Console](https://search.google.com/search-console), add `<HORIZON_DOMAIN>` as a property. If it is not verified automatically, add the TXT record shown by Search Console to the root zone, then click **Verify** (see "Zone creation and authorization" in `docs/deployment_guide.md`).
2. *(Optional)* Retrieve the DS record from **Cloud DNS** → `<SUB_DOMAIN>-horizon-sdv-com` → **Registrar Setup**, add it to your root zone for `<SUB_DOMAIN>`, and check the chain at [DNSSEC Debugger](https://dnssec-debugger.verisignlabs.com/).

---

## Step 2: Connect to GKE via Connect Gateway

Obtain cluster credentials to run `kubectl` commands through Google Cloud Connect Gateway:

```bash
# 1. Authenticate with Google Cloud
gcloud auth login

# 2. Get fleet credentials
# The fleet membership is the GKE cluster name: sdv-cluster (terraform/env/main.tf).
# If that is not found (e.g. an older deployment), list memberships and use the one shown.
gcloud container fleet memberships get-credentials sdv-cluster --project=<GCP_PROJECT_ID>
# Fallback: gcloud container fleet memberships list --project=<GCP_PROJECT_ID> --format="value(name)"

# 3. Verify connectivity
kubectl get nodes
```

---

## Step 3: Keycloak Initial Configuration

Keycloak provides central IAM and SSO for all applications.

> [!IMPORTANT]
> **Proactive Credential Presentation Rule**:
> Only present the locally-generated administrator credentials below (and the Argo CD break-glass `admin` from `manual_secrets.s5`). Do **not** proactively display Terraform-generated service passwords (Jenkins, Postgres, etc.) as workloads receive credentials automatically via External Secrets and users authenticate via Keycloak SSO.

| Account | Realm | Username | Secret Manager Key | Purpose & Usage |
| :--- | :--- | :--- | :--- | :--- |
| **Horizon SSO Admin** *(Recommended)* | `horizon` | `horizon-admin` | `keycloak-horizon-admin-password-b64` (initial password only) | **Primary SSO account for all platform tools**.<br>Use when logging into Argo CD, Jenkins, Gerrit, Grafana, Headlamp, and Developer Portal via **"LOG IN VIA KEYCLOAK"**, or at `/auth/admin/horizon/console/`.<br>Keycloak requires a password change at first login. |
| **Keycloak Root Superadmin** | `master` | `admin` | `keycloak-admin-password-b64` | **Root Keycloak server administration only**.<br>Used at `/auth/admin/master/console/` to manage server-level settings and realms. |

### 3.1 Keycloak Administration Steps
1. Open the Keycloak Console:
   - For Horizon realm: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/admin/horizon/console/` (Log in with `horizon-admin`)
   - For Master realm: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/admin/master/console/` (Log in with `admin`)
2. Retrieve the respective password from GCP Secret Manager. The `horizon-admin` value is the **initial** password and is only valid until the first login:
   ```bash
   # Horizon realm admin initial password (valid until first login)
   gcloud secrets versions access latest --secret="keycloak-horizon-admin-password-b64" --project=<GCP_PROJECT_ID> | base64 --decode && echo

   # Master realm superadmin password
   gcloud secrets versions access latest --secret="keycloak-admin-password-b64" --project=<GCP_PROJECT_ID> | base64 --decode && echo
   ```
3. If the `horizon-admin` password was changed and lost, log in at `/auth/admin/master/console/` as `admin`, switch to the `horizon` realm, then **Users** → `horizon-admin` → **Credentials** → **Reset password**.

### 3.2 Configure Google Identity Provider
1. In the **Horizon** realm, navigate to **Identity Providers** → **Add provider** → **Google**.
2. Verify Redirect URI matches:
   `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/realms/horizon/broker/google/endpoint`
3. Enter `Client ID` and `Client Secret` (from GCP OAuth2 setup).
4. Click **Add**.

### 3.3 Create Custom Authentication Flow ("broker link existing user")
1. Navigate to **Authentication** → **Create flow**.
   - Flow Name: `broker link existing user`
   - Flow Type: `Generic flow` (or default) → Click **Create**.
2. Click **Add execution**:
   - Search: `Detect existing broker user` → Click **Add**.
   - Set **Requirement** to `Required`.
3. Click **Add execution**:
   - Search: `Automatically set existing user` → Click **Add**.
   - Set **Requirement** to `Required`.
4. Navigate to **Identity Providers** → **Google** → **Advanced settings**:
   - Set **First login flow override** to `broker link existing user`.
5. Click **Save**.

### 3.4 Create Human Admin User
1. Navigate to **Users** → **Add user**.
2. Toggle **Email Verified** to `On`.
3. Enter your full Google email for **Username** AND **Email** (must match exactly).
4. Enter First Name and Last Name → Click **Create**.
5. Go to **Role mapping** tab → **Assign role** → Select **Realm roles** → Check `realm_admin` → Click **Assign**.
6. Sign out and test signing in with **Google**.

---

## Step 4: Keycloak RBAC Group Mappings

> [!IMPORTANT]
> **Why group assignment is mandatory and not automatic by default**:
> By security design (Principle of Least Privilege), Keycloak provisions users (including `horizon-admin` and initial Google SSO users) with standard realm authentication, but does **not** grant cluster-wide admin/write rights across connected apps by default. Without explicit group membership:
> - **Argo CD** assigns `role:readonly` (causing `permission denied: applications, sync`).
> - **Jenkins** assigns view-only access.
> - **Grafana** refuses login unless the user is in `administrators` (Admin) or `viewers` (Viewer). `developers` alone is not enough (`gitops/templates/grafana.yaml`: `role_attribute_strict: true`).
>
> Therefore, joining `administrators` is a required post-deployment step for any platform administrator.

### Group Mapping Reference

| Application | Keycloak Group | Mapped Role / Access Level |
| :--- | :--- | :--- |
| **Jenkins** | `administrators` | Global: Admin (Full access) |
| | `developers` | Item: workloads-developers (Build & configure workloads) |
| | `viewers` | Item: workloads-viewers (Read / view builds) |
| **Argo CD** | `administrators` | `role: admin` (Full GitOps cluster admin) |
| **Headlamp** | `administrators` | `role: cluster-admin` (Full Kubernetes UI control) |
| **Grafana** | `administrators` | Admin (Edit dashboards and datasources) |
| | `viewers` | Viewer (Read-only monitoring) |
| **MCP Registry** | `administrators` | Admin (Full UI & API to add/edit MCP servers & agents) |
| | `viewers` | Viewer (View & execute MCP tools/agents) |

### Procedure to Assign User to Group
1. In Keycloak Admin Console, go to **Users** → Select target user.
2. Click the **Groups** tab.
3. Click **Join Group**, select the desired group (`administrators`, `developers`, or `viewers`), and click **Join**.
4. Have the user log out and log back in to the respective application for group claims to refresh.

---

## Step 5: Verify Argo CD Sync Waves Progression & Health

Horizon SDV orchestrates multi-application deployment via sequential **Argo CD Sync Waves**. Verify wave completion before configuring portal modules:

### 5.1 Sync Wave Sequence & Architecture

Argo CD Applications by `argocd.argoproj.io/sync-wave` annotation in `gitops/templates` (an Application without the annotation is wave 0). Some Applications are conditional on `terraform.tfvars` settings. Other resources (Secrets, Gateway HTTPRoutes, init jobs) also carry wave annotations and span waves 1-9.

| Wave | Applications |
| :---: | :--- |
| 0 | external-dns |
| 1 | *(no Applications; resources only: `*-init`, Config Connector, Gateway and Argo CD route)* |
| 2 | postgresql, zookeeper, gerrit-operator, module-manager, dynpvc-provisioner, dynpvc-releaser, workloads-openbsw |
| 3 | keycloak |
| 4 | gerrit, headlamp, headlamp-oauth2-proxy, headlamp-token-injector, mtk-connect, monitoring-tools, argo-events |
| 5 | grafana, kubescape-operator, argo-events-resources |
| 6 | jenkins, argo-workflows, horizon-dev-portal (Horizon Developer Portal) |
| 7 | horizon-api |
| 8 | gerrit-mcp-server, mcp-gateway-registry |
| 9 | *(no Applications; MCP Gateway Registry route resources only)* |

Regenerate the table from the repository if in doubt (prints `<wave> <file>`; files missing from the output have no annotation, i.e. wave 0):
```bash
awk 'FNR==1 || /^---/ {app=0} /^kind: Application$/ {app=1} app && /argocd.argoproj.io\/sync-wave/ {gsub(/[^0-9-]/, "", $2); print $2, FILENAME; app=0}' gitops/templates/*.yaml | sort -n
```

### 5.2 Wave Verification & Remediation Commands

```bash
# 1. Check health & sync status of all Argo CD applications
kubectl get applications -n argocd \
  -o custom-columns=NAME:.metadata.name,HEALTH:.status.health.status,SYNC:.status.sync.status,MESSAGE:.status.operationState.message

# 2. Inspect resources in a specific application ordered by sync wave
kubectl get application <APP_NAME> -n argocd \
  -o jsonpath='{range .status.resources[*]}{.kind}{"/"}{.name}{"\tWave: "}{.syncWave}{"\tStatus: "}{.status}{"\tHealth: "}{.health.status}{"\n"}{end}' | sort -k3,3n

# 3. Force-sync an OutOfSync / Missing application (e.g. if auto-sync hit retry limit)
kubectl patch application <APP_NAME> -n argocd \
  --type merge \
  -p '{"operation":{"sync":{"syncStrategy":{"hook":{"force":true}}}}}'
```

---

## Step 6: Enable Workload Modules in Developer Portal

To enable Jenkins pipelines and Android workload templates:

1. Open **Horizon Developer Portal**:
   ```
   https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/developer-portal/admin/modules
   ```
2. Navigate to **Administration** → **Modules**.
3. Enable **`workloads-common`**, then enable **`workloads-android`** in that order.
4. Run the Jenkins Seed job to generate the pipeline build jobs.
