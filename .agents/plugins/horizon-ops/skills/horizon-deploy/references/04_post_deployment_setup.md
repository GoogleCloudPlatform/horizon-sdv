# Phase 4: Post-Deployment Setup

This runbook covers critical post-deployment configurations required to bring the cluster to fully operational status.

---

## Step 1: DNS & Nameserver Configuration

To route internet traffic to the GKE ingress, update your domain nameservers.

### 1.1 For GCP-Registered Domains or Cloud DNS Root Zones
When Terraform creates the subdomain zone `<SUB_DOMAIN>-horizon-sdv-com` (DNS Name: `<SUB_DOMAIN>.<HORIZON_DOMAIN>`):
1. Navigate to **Network Services** → **Cloud DNS**.
2. Click the Terraform-managed zone: `<SUB_DOMAIN>-horizon-sdv-com`.
3. Locate the `NS` record and copy all 4 values (`ns-cloud-xx.googledomains.com.`).
4. Go to your root DNS zone for `<HORIZON_DOMAIN>` (e.g. `your-domain-com`).
5. Click **+ Add Standard** / **Add Record Set**:
   - **DNS Name**: `<SUB_DOMAIN>`
   - **Resource Record Type**: `NS`
   - **Data**: Paste the 4 nameserver lines copied above.
6. Save the record.

### 1.2 DNSSEC Authorization (If `sdv_dns_dnssec_enabled = true`)
1. In [Google Search Console](https://search.google.com/search-console), verify ownership of `<HORIZON_DOMAIN>`.
2. Retrieve the DS record from **Cloud DNS** → `<SUB_DOMAIN>-horizon-sdv-com` → **Registrar Setup**.
3. Add the `DS` record to your root zone pointing to `<SUB_DOMAIN>`.
4. Verify DNSSEC at [DNSSEC Debugger](https://dnssec-debugger.verisignlabs.com/).

---

## Step 2: Connect to GKE via Connect Gateway

Obtain cluster credentials to run `kubectl` commands through Google Cloud Connect Gateway:

```bash
# 1. Authenticate with Google Cloud
gcloud auth login

# 2. Get fleet credentials
# Replace <MEMBERSHIP_NAME> with your GKE cluster name (typically sdv-<env-name>-cluster)
gcloud container fleet memberships get-credentials <MEMBERSHIP_NAME> --project=<GCP_PROJECT_ID>

# 3. Verify connectivity
kubectl get nodes
```

---

## Step 3: Keycloak Initial Configuration

Keycloak provides central IAM and SSO for all applications.

> [!IMPORTANT]
> **Proactive Credential Presentation Rule**:
> Only present the two locally-generated Keycloak administrator credentials below. Do **not** proactively display Terraform-generated service passwords (Argo CD, Jenkins, Postgres, etc.) as workloads receive credentials automatically via External Secrets and users authenticate via Keycloak SSO.

| Account | Realm | Username | Secret Manager Key | Purpose & Usage |
| :--- | :--- | :--- | :--- | :--- |
| **Horizon SSO Admin** *(Recommended)* | `horizon` | `horizon-admin` | `keycloak-horizon-admin-password-b64` | **Primary SSO account for all platform tools**.<br>Use when logging into Argo CD, Jenkins, Gerrit, Grafana, Headlamp, and Developer Portal via **"LOG IN VIA KEYCLOAK"**, or at `/auth/admin/horizon/console/`. |
| **Keycloak Root Superadmin** | `master` | `admin` | `keycloak-admin-password-b64` | **Root Keycloak server administration only**.<br>Used at `/auth/admin/master/console/` to manage server-level settings and realms. |

### 3.2 Keycloak Administration Steps
1. Open the Keycloak Console:
   - For Horizon realm: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/admin/horizon/console/` (Log in with `horizon-admin`)
   - For Master realm: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/admin/master/console/` (Log in with `admin`)
2. Retrieve the respective password from GCP Secret Manager:
   ```bash
   # Horizon realm admin password
   gcloud secrets versions access latest --secret="keycloak-horizon-admin-password-b64" --project=<GCP_PROJECT_ID> | base64 --decode && echo

   # Master realm superadmin password
   gcloud secrets versions access latest --secret="keycloak-admin-password-b64" --project=<GCP_PROJECT_ID> | base64 --decode && echo
   ```

### 3.3 Configure Google Identity Provider
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
> - **Grafana** assigns viewer role.
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

| Sync Wave | Components / Resources | Verification & Purpose |
| :---: | :--- | :--- |
| **Wave 0** | Core Namespaces, CRDs, ExternalSecrets, NetworkPolicies, ServiceAccounts | Foundational Kubernetes schemas & IAM access. |
| **Wave 1** | StorageClasses, PVCs, Filestore CSI driver, Redis, Zookeeper | Persistent storage & caching backends. |
| **Wave 2** | PostgreSQL Database, Config Connector GCP resources | Relational database & Cloud SQL backends. |
| **Wave 3** | Core Controllers, Keycloak deployment, Gerrit Operator, Argo Workflows | Central IAM & workflow orchestrators. |
| **Wave 4** | Jenkins Controller, Gerrit instance, Grafana, Headlamp, MTK Connect, Horizon API | Workload execution engines & platform UI tools. |
| **Wave 5** | Post-install Jobs (`keycloak-post-*`) & Core Gateway Routes (`gateway-*.yaml`) | Automatic client creation in Keycloak & Gateway HTTPS ingress for core apps. |
| **Wave 6** | Backstage Developer Portal Application (`horizon-dev-portal`) | Portal frontend and backend proxy pods. |
| **Wave 7** | Developer Portal Ingress Routes (`gateway-horizon-dev-portal.yaml`) | Maps root `/` and `/developer-portal/` to the Portal on Google Cloud Gateway. |

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
