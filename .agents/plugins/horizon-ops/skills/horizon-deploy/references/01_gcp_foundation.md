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
# 1. Initiate browser-based OAuth authentication flow
gcloud auth login --update-adc --no-launch-browser

# 2. Set active target project
gcloud config set project <GCP_PROJECT_ID>
```
*The agent presents the generated authentication link to the user, pauses execution, and awaits the verification code.*

### Enable Required GCP APIs
Enable core services, Cloud Resource Manager, and Cloud Build APIs:
```bash
gcloud services enable \
  compute.googleapis.com \
  container.googleapis.com \
  dns.googleapis.com \
  secretmanager.googleapis.com \
  certificatemanager.googleapis.com \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  cloudresourcemanager.googleapis.com \
  serviceusage.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  sts.googleapis.com \
  gkehub.googleapis.com \
  connectgateway.googleapis.com \
  monitoring.googleapis.com \
  oslogin.googleapis.com \
  autoscaling.googleapis.com \
  file.googleapis.com \
  iap.googleapis.com \
  networkconnectivity.googleapis.com \
  networkmanagement.googleapis.com \
  integrations.googleapis.com \
  aiplatform.googleapis.com \
  storage.googleapis.com \
  workstations.googleapis.com \
  spanner.googleapis.com \
  parametermanager.googleapis.com \
  --project=<GCP_PROJECT_ID>
```

---

## 2. Early Organization Policy Pre-Flight Checks & User Confirmation

Inspect organization constraints early before configuring or deploying infrastructure:

### 2.1 Service Account Key Creation Constraint (`constraints/iam.disableServiceAccountKeyCreation`)
Check if the project restricts Service Account Key creation:
```bash
gcloud org-policies describe constraints/iam.disableServiceAccountKeyCreation --project=<GCP_PROJECT_ID>
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

### Grant Cloud Build & Compute IAM Roles
Grant `roles/owner` (or Editor + Project IAM Admin) to the Cloud Build and Compute service accounts to allow automated provisioning:
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

## 3. Create Terraform Backend GCS Bucket & .gcloudignore

Create a globally unique Cloud Storage bucket in your project to store Terraform remote state:

```bash
BUCKET_NAME="<GCP_PROJECT_ID>-horizon-tfstate"

gcloud storage buckets create gs://$BUCKET_NAME \
  --project=<GCP_PROJECT_ID> \
  --location=europe-west1 \
  --uniform-bucket-level-access

gcloud storage buckets update gs://$BUCKET_NAME --project=<GCP_PROJECT_ID> --versioning
```

### Configure .gcloudignore
Ensure `.gcloudignore` includes `!terraform/env/terraform.tfvars` so the configuration is uploaded to Cloud Build:
```bash
cat << 'EOF' > .gcloudignore
.gcloudignore
.git
.gitignore
.DS_Store
**/.DS_Store
**/.terraform/*
*.tfstate
*.tfstate.*
**/node_modules/
**/dist/
**/.vite/

# Ensure terraform.tfvars is included for Cloud Build deployments
!terraform/env/terraform.tfvars
EOF
```

*Save `<GCP_BACKEND_BUCKET_NAME>` as it will be used for `sdv_gcp_backend_bucket` in `terraform.tfvars` and `backend.tf`.*

---

## 4. Upfront Cloud DNS Setup & Nameserver Delegation Notice

To ensure DNS propagation and Google-managed SSL certificate validation happen in parallel while Terraform provisions the GKE cluster, retrieve or create the Cloud DNS managed zone early:

```bash
ZONE_NAME="dev-horizon-sdv-com"
DNS_NAME="dev.<ROOT_DOMAIN>."

# Create zone if it does not exist
gcloud dns managed-zones create $ZONE_NAME \
  --dns-name=$DNS_NAME \
  --description="Managed by Horizon" \
  --project=<GCP_PROJECT_ID> || true

# Retrieve assigned Google nameservers
gcloud dns managed-zones describe $ZONE_NAME \
  --project=<GCP_PROJECT_ID> \
  --format="value(nameServers)"
```

> [!IMPORTANT]
> **Instruct User Upfront**:
> Output the 4 assigned nameservers directly to the user at the start of the workflow:
> ```text
> dev.<ROOT_DOMAIN>.   IN   NS   ns-cloud-d1.googledomains.com.
> dev.<ROOT_DOMAIN>.   IN   NS   ns-cloud-d2.googledomains.com.
> dev.<ROOT_DOMAIN>.   IN   NS   ns-cloud-d3.googledomains.com.
> dev.<ROOT_DOMAIN>.   IN   NS   ns-cloud-d4.googledomains.com.
> ```
> Instruct the user to add these records in their DNS provider now so DNS and SSL certificates are active by the time the deployment finishes.

---

## 5. Create OAuth2 Consent Screen and Client ID

Google OAuth2 credentials provide Single Sign-On (SSO) via Keycloak across all Horizon applications.

> [!NOTE]
> Terraform deploys successfully even if this step is postponed, but Keycloak Google SSO login will require these credentials to be updated in Secret Manager and configured in the Keycloak admin console.

### Step 4.1: Configure OAuth Consent Screen
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

### Step 4.2: Create OAuth 2.0 Web Client ID
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

### Step 4.3: Store Client Secret in GCP Secret Manager
1. Navigate to **Security** → **Secret Manager**.
2. Verify or create the secret named `oauth2-client-secret`.
3. Add a secret version containing the plain Client Secret value copied above.
