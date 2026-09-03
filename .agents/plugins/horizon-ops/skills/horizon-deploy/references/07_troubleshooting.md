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
**Fix**:
Delete the blocking Certificate Map to unblock certificate recreation/update:
```bash
gcloud certificate-manager maps delete horizon-sdv-map --project=<GCP_PROJECT_ID>
```

---

## 3. SSL Policy Collision (Error 409)

**Symptom**:
```text
Error: Error creating SslPolicy: googleapi: Error 409: The resource 'projects/<PROJECT_ID>/global/sslPolicies/gke-ssl-policy' already exists, alreadyExists
```
**Fix**:
Delete the orphaned SSL policy manually:
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
2. Log in with `admin` and the password stored in GCP Secret Manager under `argocd-admin-password-b64` (or key `s5` in `manual_secrets`).
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
**Fix**: Ensure a `.gcloudignore` file exists in the repository root with an explicit exception:
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
When a multi-wave Argo CD application hangs or times out in an `OutOfSync` state (e.g., hitting the max retry limit of 5), subsequent deployment waves (such as Gateway routing rules in Wave 5) are completely blocked.

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
Accessing a domain name (e.g., `https://sdv-dev.horizon-sdv3.automobility.cloud/` or `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/`) returns an Envoy/GFE error with the message **"fault filter abort"**.

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
1. **Check global CPU quota usage**:
   ```bash
   gcloud compute project-info describe --project=<project-id> --format="yaml(quotas)"
   ```
2. **Check regional CPU limits** (e.g. `N2_CPUS` in target region):
   ```bash
   gcloud compute regions describe <region> --project=<project-id> --format="yaml(quotas)"
   ```

### Remediation
Advise the user to submit a **Quota Increase Request (QIR)** via the GCP Console:
- **Service**: Compute Engine API
- **Metric Name**: `compute.googleapis.com/cpus_all_regions` (Global CPUs)
- **Target**: e.g., `128` or `160` (providing comfortable headroom for multiple `n2-highcpu-32` nodes alongside core platform services).

---

## 11. Keycloak Admin Credentials & Realm Management

### Problem
Seeding and post-deployment configurations fail due to credential confusion between the root Keycloak `master` realm and the platform-specific `horizon` realm.

### Where to Retrieve Credentials
Keycloak initial credentials are defined in the project's Terraform variables file (`terraform/env/terraform.tfvars`):

| Realm | Username | Variable in `terraform.tfvars` | Sample / Note |
| :--- | :--- | :--- | :--- |
| **Master Realm** | `admin` | `sdv_keycloak_admin_password` | Root administrator for Keycloak server |
| **Horizon Realm** | `horizon-admin` | `sdv_keycloak_horizon_admin_password` | Platform administrator for Horizon realm |

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

## 12. Shielded VM & Secure Boot Constraint (`constraints/compute.requireShieldedVm`)

### Problem
GKE cluster creation fails with:
```text
Error waiting for creating GKE cluster:
Instance 'gke-sdv-cluster-default-pool-...' creation failed: 
Constraint constraints/compute.requireShieldedVm violated for project projects/<PROJECT_ID>. 
Secure Boot is not enabled in the 'shielded_instance_config' field.
```

### Cause
The GCP Organization enforces the constraint `constraints/compute.requireShieldedVm`. GKE cluster and node pools cannot launch virtual machines without Secure Boot explicitly enabled.

### Remediation
Ensure `shielded_instance_config` with `enable_secure_boot = true` and `enable_integrity_monitoring = true` is declared in `terraform/modules/sdv-gke-cluster/main.tf` under:
1. `google_container_cluster.sdv_cluster` (`node_config` block)
2. Every `google_container_node_pool` (`sdv_main_node_pool`, `sdv_build_node_pool`, `sdv_abfs_build_node_pool`, `sdv_openbsw_build_node_pool`, `sdv_utility_node_pool`):
```hcl
    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }
```

---

## 13. Argo CD "Permission Denied: applications, sync"

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
