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

# Phase 1: GCP Foundation Setup

This guide details the creation and configuration of required Google Cloud Platform (GCP) services prior to running Terraform.

---

## 1. Prerequisites and IAM Roles

The deploying identity (user account or pipeline service account) requires the following IAM roles on the target GCP Project (`<GCP_PROJECT_ID>`):

- `roles/editor` (Editor)
- `roles/iam.serviceAccountAdmin` (Service Account Admin)
- `roles/resourcemanager.projectIamAdmin` (Project IAM Admin)
- `roles/secretmanager.admin` (Secret Manager Admin)
- `roles/storage.admin` (Storage Admin)
- `roles/compute.admin` (Compute Admin)
- `roles/container.admin` (Kubernetes Engine Admin)
- `roles/container.clusterAdmin` (Kubernetes Engine Cluster Admin)
- `roles/dns.admin` (DNS Administrator)
- `roles/artifactregistry.admin` (Artifact Registry Administrator)
- `roles/certificatemanager.admin` (Certificate Manager Admin)
- `roles/parametermanager.admin` (Parameter Manager Admin)

### CLI Tools & Interactive Authentication
Ensure Google Cloud SDK (`gcloud`, `gsutil`, `bq`) is authenticated for both CLI and Application Default Credentials (ADC):
```bash
# 1. The user runs this in their own terminal (authenticates gcloud and ADC)
gcloud auth login --update-adc

# 2. The agent verifies the login; stop if it fails
gcloud auth application-default print-access-token >/dev/null && \
  gcloud auth list --filter=status:ACTIVE --format="value(account)"

# 3. Set active target project
gcloud config set project <GCP_PROJECT_ID>
```
*Never ask the user to paste verification codes into chat.*

### Enable Required GCP APIs
Run the API list in `SKILL.md` Phase 1 step 4. That list is the single source; do not duplicate it here.

---

## 2. Early Organization Policy Pre-Flight Checks & User Confirmation

Inspect organization constraints early before configuring or deploying infrastructure:

### 2.1 Service Account Key Creation Constraint (`constraints/iam.disableServiceAccountKeyCreation`)
Check if the project restricts Service Account Key creation (`--effective` includes policies inherited from the folder or organization):
```bash
gcloud org-policies describe constraints/iam.disableServiceAccountKeyCreation --project=<GCP_PROJECT_ID> --effective
```

> [!IMPORTANT]
> **Prompt User on Enforced Policy**:
> If this constraint is active, notify the user and ask for confirmation:
> *"Your project enforces `constraints/iam.disableServiceAccountKeyCreation`. Horizon SDV stores a service account key in Secret Manager (`gce-creds`) for Jenkins to dynamically provision high-CPU build and Android emulator VMs on Google Compute Engine. Would you like me to disable this constraint for this project, or proceed without dynamic GCE VM workers?"*
>
> If confirmed by the user:
> ```bash
> gcloud resource-manager org-policies disable-enforce constraints/iam.disableServiceAccountKeyCreation \
>   --project=<GCP_PROJECT_ID>
> ```

---

## 3. Retrieve GCP Project Details & Grant Service Account Roles

Identify the default Google Compute Engine (GCE) service account and project number:

```bash
# Retrieve Project Number and ID
gcloud projects describe <GCP_PROJECT_ID> --format="value(projectNumber,projectId)"

# Retrieve default GCE Service Account email
gcloud iam service-accounts list \
  --project=<GCP_PROJECT_ID> \
  --filter="email ~ [0-9]+-compute@developer.gserviceaccount.com" \
  --format="value(email)"
```

Format of the default compute SA: `<GCP_PROJECT_NUMBER>-compute@developer.gserviceaccount.com`.  
*Save this email as it is required for `sdv_gcp_compute_sa_email` in `terraform.tfvars`.*

### Grant Cloud Build & Compute IAM Roles (Cloud Build variant only)
Only if the user chose the [Cloud Build variant](./06_cloud_build_deployment.md): grant `roles/owner` (or Editor + Project IAM Admin) to the Cloud Build and Compute service accounts so the build can provision the platform. The standard `deploy.sh` and `container-deploy.sh` paths run as the user and do not need these grants.
```bash
# Grant Cloud Build SA
gcloud projects add-iam-policy-binding <GCP_PROJECT_ID> \
  --member="serviceAccount:<GCP_PROJECT_NUMBER>@cloudbuild.gserviceaccount.com" \
  --role="roles/owner"

# Grant Compute Default SA
gcloud projects add-iam-policy-binding <GCP_PROJECT_ID> \
  --member="serviceAccount:<GCP_PROJECT_NUMBER>-compute@developer.gserviceaccount.com" \
  --role="roles/owner"
```

---

## 4. Create Terraform Backend GCS Bucket

Create a globally unique Cloud Storage bucket in your project to store Terraform remote state:

```bash
BUCKET_NAME="<GCP_PROJECT_ID>-horizon-tfstate"

gcloud storage buckets create gs://$BUCKET_NAME \
  --project=<GCP_PROJECT_ID> \
  --location=europe-west1 \
  --uniform-bucket-level-access

gcloud storage buckets update gs://$BUCKET_NAME --project=<GCP_PROJECT_ID> --versioning
```

*Save `<GCP_BACKEND_BUCKET_NAME>` as it will be used for `sdv_gcp_backend_bucket` in `terraform.tfvars` and `backend.tf`.*

---

## 5. Cloud DNS Managed Zone (Terraform-Managed)

Cloud DNS managed zones (`google_dns_managed_zone.sdv-cloud-dns-zone`) are provisioned and managed directly by Terraform as part of the core infrastructure deployment.

> [!NOTE]
> **No Manual Zone Pre-Creation**:
> Do **not** pre-create the Cloud DNS zone manually before Terraform executes, as this causes resource collision errors (`409 Conflict`) during `terraform apply`.
> Once Terraform provisions the zone, the deployment script queries the live nameserver records directly from Google Cloud (`gcloud dns managed-zones describe ...`) and presents the exact assigned nameservers to the user.

---

## 6. Create OAuth2 Consent Screen and Client ID

Google OAuth2 credentials provide Single Sign-On (SSO) via Keycloak across all Horizon applications.

> [!NOTE]
> Terraform deploys successfully even if this step is postponed, but Keycloak Google SSO login will require these credentials to be updated in Secret Manager and configured in the Keycloak admin console.

### Step 6.1: Configure OAuth Consent Screen
1. In GCP Console, navigate to **APIs & Services** → **OAuth consent screen**.
2. Click **Get Started**.
3. **App name**: `Horizon - SDV`.
4. **User support email**: Select an admin email.
5. **Audience**: Select **External** (or Internal if using Google Workspace and restricted to domain).
6. **Contact email**: Provide developer/admin email.
7. **App Domain**:
   - Application Home Page: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
   - Authorized Domains: `<HORIZON_DOMAIN>`
8. Click **Save and Continue**.

### Step 6.2: Create OAuth 2.0 Web Client ID
1. Navigate to **APIs & Services** → **Credentials**.
2. Click **Create Credentials** → **OAuth client ID**.
3. **Application type**: `Web application`.
4. **Name**: `Horizon`.
5. **Authorized redirect URIs**:
   ```
   https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/realms/horizon/broker/google/endpoint
   ```
6. Click **Create**.
7. Copy the generated **Client ID** and **Client Secret** immediately.

### Step 6.3: Store Client Secret in GCP Secret Manager (after deployment)
Terraform creates `keycloak-idp-client-secret` as a placeholder and does not overwrite later versions. Add the real value as a new version:
```bash
printf '%s' '<CLIENT_SECRET>' | gcloud secrets versions add keycloak-idp-client-secret --data-file=- --project=<GCP_PROJECT_ID>
```
This is storage only. Keycloak's Google login uses the Client ID and Client Secret entered in the Keycloak UI ([04_post_deployment_setup.md §3.2](./04_post_deployment_setup.md#32-configure-google-identity-provider)).
