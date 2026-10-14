---
name: horizon-sdv
description: Guides and automates the deployment, operation, and maintenance of the Horizon SDV platform on Google Cloud Platform.
---

# Horizon SDV Custom Deployment Skill (Release 4.1)

This skill contains the authoritative guides, runbooks, and automation flows to deploy and operate the Accenture Horizon SDV platform, specifically targeting **Horizon SDV Release 4.1**.

## 1. Prerequisites & Required GCP APIs

To deploy Horizon SDV successfully, ensure that the following GCP APIs are enabled on your project:
*   Artifact Registry API (`artifactregistry.googleapis.com`)
*   Certificate Manager API (`certificatemanager.googleapis.com`)
*   Cloud DNS API (`dns.googleapis.com`)
*   Compute Engine API (`compute.googleapis.com`)
*   Kubernetes Engine API (`container.googleapis.com`)
*   GKE Hub API (`gkehub.googleapis.com`)
*   Secret Manager API (`secretmanager.googleapis.com`)
*   Parameter Manager API (`parametermanager.googleapis.com`)
*   Service Usage API (`serviceusage.googleapis.com`)
*   Vertex AI API (`aiplatform.googleapis.com`)

## 2. Organization Policy Remediations

Corporate parent organizations often enforce strict rules that will cause standard GKE clusters or emulator VM deployment to fail. Apply these override policies at the project level:

### Policy 1: Service Account Key Creation
*   **Constraint**: `constraints/iam.disableServiceAccountKeyCreation`
*   **File**: `.agents/skills/horizon-sdv/resources/disable-sa-key-creation.yaml`
    ```yaml
    name: projects/<PROJECT_ID>/policies/iam.disableServiceAccountKeyCreation
    spec:
      rules:
      - enforce: false
    ```
*   **Command**: `gcloud org-policies set-policy .agents/skills/horizon-sdv/resources/disable-sa-key-creation.yaml --project=<PROJECT_ID>`

### Policy 2: Nested Virtualization (Required for Android Cuttlefish/ARM64 Emulators)
*   **Constraint**: `constraints/compute.disableNestedVirtualization`
*   **File**: `.agents/skills/horizon-sdv/resources/disable-nested-virtualization.yaml`
    ```yaml
    name: projects/<PROJECT_ID>/policies/compute.disableNestedVirtualization
    spec:
      rules:
      - enforce: false
    ```
*   **Command**: `gcloud org-policies set-policy .agents/skills/horizon-sdv/resources/disable-nested-virtualization.yaml --project=<PROJECT_ID>`

### Policy 3: External VM IPs (Required for Load Balancer endpoints)
*   **Constraint**: `constraints/compute.vmExternalIpAccess`
*   **File**: `.agents/skills/horizon-sdv/resources/allow-vm-external-ips.yaml`
    ```yaml
    name: projects/<PROJECT_ID>/policies/compute.vmExternalIpAccess
    spec:
      rules:
      - allowAll: true
    ```
*   **Command**: `gcloud org-policies set-policy .agents/skills/horizon-sdv/resources/allow-vm-external-ips.yaml --project=<PROJECT_ID>`

---

## 3. Configuration Setup (`terraform.tfvars`)

Copy the sample file and fill in your variables:
```bash
cp terraform/env/terraform.tfvars.sample terraform/env/terraform.tfvars
```
Ensure you have the following critical variables:
```hcl
sdv_gcp_project_id           = "<PROJECT_ID>"
sdv_gcp_backend_bucket       = "<TERRAFORM_STATE_BUCKET_NAME>"
sdv_root_domain              = "<SUB_DOMAIN>.<PARENT_DOMAIN>"
scm_auth_method              = "none" # For public repositories
sdv_dns_use_static_a_records = true   # For static A record configuration
```

---

## 4. Execution Procedures

Always run the deployment steps using the container wrapper to guarantee tool chain consistency:

### A. Run Terraform Plan (Dry-run)
```bash
./tools/scripts/deployment/container-deploy.sh --plan
```

### B. Run Terraform Apply (Deployment)
```bash
./tools/scripts/deployment/container-deploy.sh --apply
```

### C. Run Terraform Destroy (Teardown)
```bash
./tools/scripts/deployment/container-deploy.sh --destroy
```

---

## 5. Post-Deployment Verification (Static A Records Setup)

Once the deployment completes, the Load Balancer IP must be mapped to your custom subdomains:

1.  **Retrieve Load Balancer IP**:
    Find the external IP address of the deployed Ingress Load Balancer:
    ```bash
    gcloud compute addresses list --project=<PROJECT_ID>
    ```
2.  **Add A-Records**:
    Manually create two `A` records in your DNS domain registrar pointing to that Load Balancer IP:
    *   `horizon-<ID>.<PARENT_DOMAIN>` -> `LOAD_BALANCER_IP`
    *   `*.horizon-<ID>.<PARENT_DOMAIN>` -> `LOAD_BALANCER_IP`
3.  **Confirm Certificates**:
    Wait for the GKE managed certificates to validate. You can verify their status via the Google Cloud Console under **Network Security -> Certificate Manager**.

---

## 6. Comprehensive Troubleshooting & Diagnostics

This section provides authoritative, step-by-step remediation procedures and exact CLI commands to resolve the most common deployment, authentication, and runtime failures on the Horizon SDV platform.

### 6.1 Local Workstation & Environment Failures

#### A. Docker Daemon Permission Denied
*   **Symptom / Error**:
    ```
    ERROR: permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock: Head "http://%2Fvar%2Frun%2Fdocker.sock/_ping": dial unix /var/run/docker.sock: connect: permission denied
    ```
*   **Root Cause**: The current workspace user does not have read/write access to the Docker daemon UNIX socket.
*   **Remediation Commands**:
    Ensure the current user is in the `docker` group and apply the changes:
    ```bash
    sudo usermod -aG docker $USER
    # Log in to the new group without restarting the shell session
    newgrp docker
    ```
    If permissions are still restricted, temporarily broaden socket access:
    ```bash
    sudo chmod 666 /var/run/docker.sock
    ```

#### B. Docker Container Build Failures (Resource Exhaustion / OOM)
*   **Symptom / Error**:
    ```shell
    │ Error: Error running legacy build: process "/bin/sh -c apt-get update && apt-get install -y ..." did not complete successfully: exit code: 100
    │ ...
    │ with module.base.module.sdv_container_images.docker_image.sdv-container-images["gerrit-mcp-server-app"],
    ```
*   **Root Cause**: Building resource-heavy container images (like Gerrit, Jenkins, or Horizon API) exceeds the Docker daemon's allocated memory or CPU limits, resulting in silent crashes (OOM) or timeouts.
*   **Remediation**:
    1.  Open Docker Desktop / Docker Engine configuration on your workstation.
    2.  Navigate to **Settings** -> **Resources** -> **Advanced**.
    3.  Increase allocated resources:
        *   **CPUs**: Minimum **4 CPUs** (6 or 8 recommended).
        *   **Memory**: Minimum **8 GB** (12 GB or 16 GB recommended).
    4.  Apply changes, restart the Docker daemon, and re-run the container deployer:
        ```bash
        ./tools/scripts/deployment/container-deploy.sh --apply
        ```

---

### 6.2 GCP Resource & Deployment Failures

#### A. Certificate Manager Map Delete Lock (Precondition Failure)
*   **Symptom / Error**:
    ```shell
    │ Error: Error when reading or editing Certificate: googleapi: Error 400: can't delete certificate that is referenced by a CertificateMapEntry or other resources
    │ Details: [ { "type": "RESOURCE_STILL_IN_USE", "subject": "projects/<PROJECT_NUMBER>/locations/global/certificates/horizon-sdv" } ]
    ```
*   **Root Cause**: Terraform cannot tear down or update the Certificate Manager certificate because active GKE ingress maps are still locking it.
*   **Remediation Command**:
    Manually release the lock by deleting the blocking certificate map:
    ```bash
    gcloud certificate-manager maps delete horizon-sdv-map --quiet --project=<PROJECT_ID>
    ```

#### B. SslPolicy Already Exists Conflict
*   **Symptom / Error**:
    ```shell
    │ Error: Error creating SslPolicy: googleapi: Error 409: The resource 'projects/<PROJECT_ID>/global/sslPolicies/gke-ssl-policy' already exists, alreadyExists
    │ on ../modules/sdv-ssl-policy/main.tf line 21, in resource "google_compute_ssl_policy" "gke_ssl_policy":
    ```
*   **Root Cause**: A previous failed or partial deployment did not cleanly tear down the global GKE SSL policy.
*   **Remediation Command**:
    Manually delete the conflicting global SSL policy:
    ```bash
    gcloud compute ssl-policies delete gke-ssl-policy --global --quiet --project=<PROJECT_ID>
    ```

---

### 6.3 Authentication & Keycloak Failures

#### A. Redirect URI Mismatch
*   **Symptom / Error**: After GKE provisioning, attempts to sign-in via Google login result in a `Redirect URI mismatch` error screen.
*   **Root Cause**: The redirect URI registered in the Google Cloud Console's OAuth Credentials does not match the active Keycloak broker endpoint.
*   **Remediation Steps**:
    1.  Log in to the **Keycloak Administration Console** (accessible at `/auth`).
    2.  Select the **"Horizon"** realm from the top-left dropdown.
    3.  Go to **Identity providers** -> click on **"google"**.
    4.  Copy the URL in the **Redirect URI** field.
    5.  Go to the **Google Cloud Console** -> **APIs & Services** -> **Credentials**.
    6.  Open your **"Horizon"** OAuth 2.0 Client ID.
    7.  Paste the copied URI under **Authorized redirect URIs**. Save and allow up to 5 minutes for Google to sync the credentials.

#### B. User Does Not Exist Error
*   **Symptom / Error**:
    ```
    "User <USER_EMAIL> authenticated with Identity provider google does not exist. Please contact your administrator"
    ```
*   **Root Cause**: The Keycloak user profile database is missing or has a mismatch between the authenticated username and email address field.
*   **Remediation Steps**:
    1.  Access the **Keycloak Administration Console** -> select **"Horizon"** realm.
    2.  Go to **Users** -> find and click on the affected user's profile.
    3.  Verify that both the **Username** and **Email** fields are filled with the **identical, full email address** of the Google account. Save changes.

---

### 6.4 Application & Workload Operations

#### A. Homepage Unreachable / 404 / 502 (ArgoCD Sync Issue)
*   **Symptom**: `https://horizon-<env>.<domain>` is unreachable, but `https://horizon-<env>.<domain>/argocd` works normally.
*   **Root Cause**: The underlying infrastructure is successfully provisioned, but GKE application deployments inside ArgoCD are out of sync or stuck in a degraded state.
*   **Remediation Steps**:
    1.  Retrieve the ArgoCD Admin Password from GCP Secret Manager:
        ```bash
        gcloud secrets versions access latest --secret="argocd-admin-password-b64" --project=<PROJECT_ID>
        ```
    2.  Open `https://horizon-<env>.<domain>/argocd` and log in with user `admin` and the retrieved password.
    3.  Locate the parent application named **"horizon-sdv"**.
    4.  Click **Refresh** to poll the cluster state, and then click **Sync** (select "Prune" and "Force" options if individual resource conflicts exist).
    5.  Wait for the status to turn completely green (**Healthy** & **Synced**).

#### B. Jenkins Pipeline "Insufficient Permissions" for Admin
*   **Symptom / Error**: User logged in to Jenkins cannot run build/test pipelines or configure settings.
*   **Root Cause**: Keycloak group assignments have failed to synchronize with Jenkins Roles, or explicit user permissions are missing.
*   **Remediation Steps**:
    1.  In Jenkins, navigate to **Manage Jenkins** -> **Manage and Assign Roles** -> **Assign Roles**.
    2.  Under **Global Roles**:
        *   In **User/group to add**, input your exact Google email address.
        *   Click **Add**, then check the box under the **`administrators`** column.
    3.  Under **Item Roles**:
        *   In **User/group to add**, input your exact Google email address.
        *   Click **Add**, then check the box under the **`developers`** column.
    4.  Click **Save** at the bottom of the page. Log out and log back in to apply the permission roles.

