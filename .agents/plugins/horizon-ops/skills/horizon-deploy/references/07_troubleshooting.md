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

# Troubleshooting & Operational Failure Modes

This document provides resolutions for common issues encountered during Horizon SDV deployment and initial configuration.

---

## 1. Keycloak Sign-In Failures

### 1.1 Redirect URI Mismatch
**Symptom**: Error when attempting Google login from Keycloak: *Invalid parameter: redirect_uri*.
**Fix**:
1. In Keycloak Admin Console (`/auth/admin/horizon/console`), go to **Identity Providers** → **Google**.
2. Copy the exact **Redirect URI**.
3. In GCP Console, go to **APIs & Services** → **Credentials** → Select the OAuth 2.0 Client ID (`Horizon`).
4. Ensure the **Authorized redirect URIs** list contains the exact URI copied from Keycloak:
   `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/realms/horizon/broker/google/endpoint`
5. Save changes.

### 1.2 User Does Not Exist in Identity Provider
**Symptom**: Error: *"User <USER_NAME> authenticated with Identity provider google does not exist. Please contact your administrator"*.
**Fix**:
1. In Keycloak Admin Console, go to **Users** → Select the user.
2. Ensure both the **Username** and **Email** fields contain the full Google email address.
3. Ensure the custom authentication flow `broker link existing user` has both `Detect existing broker user` and `Automatically set existing user` set to `Required`.

---

## 2. Certificate Manager In-Use Error

**Symptom**:
```text
Error: Error when reading or editing Certificate: googleapi: Error 400: can't delete certificate that is referenced by a CertificateMapEntry or other resources
```
**Cause**: Before Release 3.1.0 each environment had its own certificate map. Since 3.1.0 all environments share one map, `horizon-sdv-map`, which the live Gateway uses for HTTPS.

**Fix**:
- Do **not** delete `horizon-sdv-map`. It serves TLS for the main environment and all sub-environments.
- If the environment was upgraded from a release before 3.1.0, follow the matching upgrade guide in `docs/guides/`.
- Otherwise, stop and report the full error to the user. Do not delete certificates or map entries without explicit approval.

---

## 3. SSL Policy Collision (Error 409)

**Symptom**:
```text
Error: Error creating SslPolicy: googleapi: Error 409: The resource 'projects/<PROJECT_ID>/global/sslPolicies/gke-ssl-policy' already exists, alreadyExists
```
**Fix**:
The policy exists in the project but not in this Terraform state, usually left over from an earlier deployment.
1. Confirm it is not in use. This must return nothing:
   ```bash
   gcloud compute target-https-proxies list --project=<GCP_PROJECT_ID> --filter="sslPolicy~gke-ssl-policy" --format="value(name)"
   ```
2. If it is in use, stop: another deployment may own it. Report this to the user.
3. If unused, and only with explicit user approval (destructive), delete it and re-run the standard deployment:
   ```bash
   gcloud compute ssl-policies delete gke-ssl-policy --global --project=<GCP_PROJECT_ID>
   ```

---

## 4. Docker Build & Resource Errors

### 4.1 Docker Permission Denied
**Symptom**:
```text
ERROR: permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock
```
**Fix**:
Add your user to the `docker` group (Linux):
```bash
sudo usermod -aG docker $USER
newgrp docker
```

### 4.2 Docker Memory Exhaustion (Exit Code 100)
**Symptom**:
```text
Error: Error running legacy build: process "/bin/sh -c apt-get update && apt-get install -y ..." did not complete successfully: exit code: 100
```
**Fix**:
Increase resources assigned to Docker:
- Minimum recommended: 4 vCPUs, 8 GB RAM, 2 GB Swap.
- In Docker Desktop / Colima: Settings → Resources → Increase Memory and CPUs, then re-run `./container-deploy.sh -a`.

---

## 5. Homepage Not Reachable / Argo CD Out-of-Sync

**Symptom**: Accessing `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/` times out or fails, but `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd` is reachable.

**Fix**:
1. Access Argo CD directly at `/argocd`.
2. Log in with `admin` and the value of `manual_secrets.s5` in `terraform/env/terraform.tfvars` (Secret Manager `argocd-admin-password-b64` holds only a bcrypt hash).
3. Check the `horizon-sdv` root application health state.
4. Click **Refresh**, then click **Sync** → **Synchronize** with `Prune` and `Apply` enabled.
5. Wait for all Kubernetes applications and ingress resources to turn green / **Healthy**.

---

## 6. Jenkins Pipeline Permission Fallback

**Symptom**: Pipeline cannot be run even after assigning Keycloak groups.

**Fix**:
1. In Jenkins UI, navigate to **Manage Jenkins** → **Manage and Assign Roles** → **Assign Roles**.
2. Under **Global Roles**:
   - In *User/group to add*, type your full email.
   - Click **Add** and check `administrators`.
3. Under **Item Roles**:
   - In *User/group to add*, type your full email.
   - Click **Add** and check `developers`.
4. Click **Save**.

---

## 7. Cloud Build Deployment Issues

### 7.1 `terraform.tfvars` Missing in Build Context
**Symptom**: `[ERROR] Config file not found at /workspace/.../terraform.tfvars`.  
**Cause**: `.gitignore` contains `**/terraform.tfvars`, and Google Cloud SDK (`gcloud builds submit`) inherits `.gitignore` when no `.gcloudignore` exists.  
**Fix**: `cloud-deploy.sh` passes `--ignore-file=.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/gcloudignore`, which contains the exception below. Make sure `scripts/gcloudignore` still has it and that the build was submitted through `cloud-deploy.sh`:
```
!terraform/env/terraform.tfvars
```

### 7.2 Staging Bucket Location / Permission Denied
**Symptom**: `ERROR: (gcloud.builds.submit) PERMISSION_DENIED` during tarball upload to `gs://<PROJECT_ID>_cloudbuild`.  
**Cause**: The default Cloud Build staging bucket is in the US multi-region, which may be blocked by GCP Organization Resource Location policies.  
**Fix**: Pass your regional Terraform state bucket as the staging directory:
```bash
--gcs-source-staging-dir="gs://<GCP_PROJECT_ID>-horizon-tfstate/cloudbuild-source"
```

### 7.3 Cloud Build Service Account Permission Denied
**Symptom**: Cloud Build step fails during Terraform apply with IAM permission denied.  
**Cause**: The Cloud Build service account lacks permissions to provision GKE and GCP resources.  
**Fix**: Grant `roles/owner` (or Editor + Project IAM Admin) to the Cloud Build SA:
```bash
gcloud projects add-iam-policy-binding <GCP_PROJECT_ID> \
  --member="serviceAccount:<GCP_PROJECT_NUMBER>@cloudbuild.gserviceaccount.com" \
  --role="roles/owner"
```

---

## 8. Argo CD Sync Stalls (Sync Waves)

### Problem
When a multi-wave Argo CD application hangs or times out in an `OutOfSync` state (e.g., hitting the max retry limit of 5), later sync waves are blocked, including most Gateway HTTPRoutes (they span waves 1-9; see the wave table in [ref 04 §5.1](./04_post_deployment_setup.md#51-sync-wave-sequence--architecture)).

### Troubleshooting Checklist
1. **Inspect the phase where the app is stuck**:
   ```bash
   kubectl get application <app-name> -n argocd -o jsonpath='{.status.operationState}'
   ```
2. **Check for pending infrastructure dependencies** such as persistent volume claims (PVCs):
   ```bash
   kubectl get pvc -A
   ```
3. **Verify cloud infrastructure provider states**:
   For instance, the GKE Filestore CSI driver can take 3 to 5 minutes to provision a Filestore instance. Check the instance status using:
   ```bash
   gcloud filestore instances list --project=<project-id> --zone=<zone>
   ```

### Remediation
- If PVC creation/binding succeeds, Argo CD will resume syncing subsequent waves.
- To force-resume or re-trigger a manual sync without waiting for default back-off retry loops, apply a merge patch to the application:
  ```bash
  kubectl patch application <app-name> -n argocd --type=merge -p '{"operation": {"sync": {"prune": true, "syncOptions": ["CreateNamespace=true"]}}}'
  ```

---

## 9. GKE Gateway "fault filter abort"

### Problem
Accessing `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/` (or a route under it) returns an Envoy/GFE error with the message **"fault filter abort"**.

### Diagnostic Steps
1. **Verify if the target HTTPRoute has been applied to the GKE cluster**:
   ```bash
   kubectl get httproute -A
   ```
2. **Confirm that the HTTPRoute is bound and successfully accepted/programmed by the Gateway**:
   ```bash
   kubectl describe httproute <route-name> -n <namespace>
   ```
   *(Look for `Accepted: True` and `ReconciliationSucceeded: True` under the `Parents` status).*
3. **Verify the health of the backend endpoint group (NEG) on GCP**:
   - List backend services:
     ```bash
     gcloud compute backend-services list --project=<project-id>
     ```
   - Get backend health:
     ```bash
     gcloud compute backend-services get-health <backend-service-name> --global --project=<project-id>
     ```

### Expected Behavior
Changing or adding routes in GKE Gateway updates the Global L7 Load Balancer URL Map. These changes can take **3 to 10 minutes** to fully propagate to all GFE edge nodes globally. During this propagation window, requests to the newly added route may fallback to the default route action, which is configured to abort requests and return `"fault filter abort"`. **Always allow up to 10 minutes for propagation before concluding a route is broken.**

---

## 10. GCP Quota Management & Resource Exhaustion (QIRs)

### Problem
GKE pods remain in `Pending` with `FailedScheduling` events, and the cluster autoscaler is unable to scale up node pools (e.g. `sdv-abfs-build-node-pool`).

### Diagnostic Steps
1. **Confirm the scheduling failure**:
   ```bash
   kubectl describe pod <pod> -n <namespace> | grep -A5 Events
   ```
2. **List the machine types actually used by the node pools**:
   ```bash
   gcloud container node-pools list --cluster=sdv-cluster --region=<region> --project=<project-id> \
     --format="table(name,config.machineType,autoscaling.maxNodeCount)"
   ```
   Map the machine type prefix to its quota family: `c2d-*` → `C2D_CPUS`, `n2-*` → `N2_CPUS`, `n1-*` → `CPUS`.
3. **Check regional quota for those families**:
   ```bash
   gcloud compute regions describe <region> --project=<project-id> --flatten=quotas \
     --filter="quotas.metric:(C2D_CPUS N2_CPUS CPUS)" \
     --format="table(quotas.metric,quotas.usage,quotas.limit)"
   ```
   Also check the global quota:
   ```bash
   gcloud compute project-info describe --project=<project-id> --flatten=quotas \
     --filter="quotas.metric=CPUS_ALL_REGIONS" \
     --format="table(quotas.metric,quotas.usage,quotas.limit)"
   ```

### Remediation
Advise the user to submit a **Quota Increase Request (QIR)** via the GCP Console for the exhausted metric.

The build and ABFS build pools use the largest machines (`c2d-highcpu-112` at the time of writing, 112 vCPUs per node; confirm with step 2). Each concurrent Android build normally takes a whole node.

**Ask the user how many builds must run at the same time**, then size the request:
- Regional family quota (e.g. `C2D_CPUS`) ≥ vCPUs per build node × concurrent builds + other pools in that family (e.g. OpenBSW).
- Global `CPUS_ALL_REGIONS` ≥ the regional total above + main pool (`CPUS`) + utility pool (`N2_CPUS`).
- Example: 3 concurrent builds need at least 336 `C2D_CPUS`, before headroom.

---

## 11. Keycloak Admin Credentials & Realm Management

### Problem
Seeding and post-deployment configurations fail due to credential confusion between the root Keycloak `master` realm and the platform-specific `horizon` realm.

### Where to Retrieve Credentials
Keycloak initial credentials are defined in the project's Terraform variables file (`terraform/env/terraform.tfvars`):

| Realm | Username | Variable in `terraform.tfvars` | Sample / Note |
| :--- | :--- | :--- | :--- |
| **Master Realm** | `admin` | `sdv_keycloak_admin_password` | Root administrator for Keycloak server |
| **Horizon Realm** | `horizon-admin` | `sdv_keycloak_horizon_admin_password` | Platform administrator for Horizon realm. Initial password; must be changed at first login. |

### Remediation & Console Navigation
1. Always log in to the main administration console using the root admin user:
   ```
   https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/admin/master/console/
   ```
   - **Username**: `admin`
   - **Password**: Value of `sdv_keycloak_admin_password` in `terraform.tfvars`.
2. Once logged in, click the dropdown menu directly below the Keycloak logo in the top-left corner (labeled **master**) and switch to the **`horizon`** realm.
3. All application configuration (creating human admin users, assigning the `realm_admin` role, and adding users to the `administrators` group for full cluster, Jenkins, and Argo CD access) **must be done within the `horizon` realm**.

---

## 12. Argo CD "Permission Denied: applications, sync"

### Symptom
When clicking **SYNC** on any application in Argo CD, the UI shows a red error banner:
```text
Unable to sync: permission denied: applications, sync, <APP_NAME>, sub: <USER_UUID>, iat: ...
```

### Cause
By default, newly authenticated Keycloak users (including `horizon-admin` and Google SSO users) are assigned the fallback `role:readonly` in Argo CD. They do not have write/sync permissions until their user is explicitly joined to the **`administrators`** group in Keycloak.

### Remediation
1. Open the [Keycloak Horizon Console](https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/admin/horizon/console/).
2. Log in with `horizon-admin` (or root `admin` switched to the `horizon` realm).
3. Navigate to **Users** → Select the target user.
4. Click the **Groups** tab → Click **Join Group**.
5. Select **`administrators`** → Click **Join**.
6. In Argo CD, **Log out** and log back in via Keycloak for the new group claims to take effect.
