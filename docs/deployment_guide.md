# Horizon SDV

## Overview   
Horizon SDV is designed to simplify the deployment and management of Android workloads on Google Kubernetes Engine clusters. By leveraging Infrastructure as Code (IaC) and GitOps, ensuring the cluster consistently matches the desired state, enabling scalable and efficient Android workload operations.

## Table of contents
- [Overview](#overview)
- [Technologies](#technologies)
- [Project directories and files](#project-directories-and-files)
- [Configuration Placeholders](#configuration-placeholders)
- [Section #1 - GCP Foundation Setup](#section-1---gcp-foundation-setup)
  - [Section #1a - GCP Prerequisites](#section-1a---gcp-prerequisites)
  - [Section #1b - GCP Project Details](#section-1b---gcp-project-details)
  - [Section #1c - Create a Bucket in GCP](#section-1c---create-a-bucket-in-gcp)
  - [Section #1d - Create OAuth2 Client and Secret](#section-1d---create-oauth2-client-and-secret)
- [Section #2 - Deployment](#section-2---deployment)
  - [Section #2a - Prerequisites](#section-2a---prerequisites)
  - [Section #2b - Clone the Repository](#section-2b---clone-the-repository)
  - [Section #2c - Configure Terraform Variables](#section-2c---configure-terraform-variables)
  - [Section #2d - Run the Deployment Script](#section-2d---run-the-deployment-script)
- [Section #3 - Post-Deployment Setup](#section-3---post-deployment-setup)
  - [Section #3a - Update Nameservers](#section-3a---update-nameservers)
  - [Section #3b - Connect to GKE via Connect Gateway](#section-3b---connect-to-gke-via-connect-gateway)
  - [Section #3c - Setup Keycloak](#section-3c---setup-keycloak)
  - [Section #3d - Jenkins Access via Keycloak Groups](#section-3d---jenkins-access-via-keycloak-groups)
  - [Section #3e - Argo CD Access via Keycloak Groups](#section-3e---argo-cd-access-via-keycloak-groups)
  - [Section #3f - Headlamp Access via Keycloak Groups](#section-3f---headlamp-access-via-keycloak-groups)
  - [Section #3g - Grafana Access via Keycloak Groups](#section-3g---grafana-access-via-keycloak-groups)
  - [Section #3h - MCP Gateway Registry Access via Keycloak Groups](#section-3h---mcp-gateway-registry-access-via-keycloak-groups)
  - [Section #3i - Enable Workloads Modules in Developer Portal](#section-3i---enable-workloads-modules-in-developer-portal)
- [Section #4 - Run Cluster Apps](#section-4---run-cluster-apps)
  - [Section #4a - Horizon Landing Page](#section-4a---horizon-landing-page)
  - [Section #4b - Argo CD](#section-4b---argo-cd)
  - [Section #4c - Keycloak](#section-4c---keycloak)
  - [Section #4d - Gerrit](#section-4d---gerrit)
  - [Section #4e - Jenkins](#section-4e---jenkins)
  - [Section #4f - MTK connect](#section-4f---mtk-connect)
  - [Section #4g - Headlamp](#section-4g---headlamp)
  - [Section #4h - Grafana](#section-4h---grafana)
  - [Section #4i - MCP Gateway Registry](#section-4i---mcp-gateway-registry)
- [Section #5 - Optional - GitHub Setup](#section-5---optional---github-setup)
  - [Section #5a - GitHub Prerequisites](#section-5a---github-prerequisites)
  - [Section #5b - Create GitHub Organization](#section-5b---create-github-organization)
  - [Section #5c - Create and Install GitHub Application](#section-5c---create-and-install-github-application)
  - [Section #5d - Fork the Repository](#section-5d---fork-the-repository)
- [Section #6 - Troubleshooting](#section-6---troubleshooting)
  - [Section #6a - Keycloak sign-in failure](#section-6a---keycloak-sign-in-failure)
  - [Section #6b - Error when reading or editing Certificate](#section-6b---error-when-reading-or-editing-certificate)
  - [Section #6c - Docker permission denied](#section-6c---docker-permission-denied)
  - [Section #6d - Docker Container build resource issues](#section-6d---docker-container-build-resource-issues)
  - [Section #6e - Error creating SslPolicy](#section-6e---error-creating-sslpolicy)

## Technologies
Technologies being used to provision the infrastructure along with the required applications for the GKE cluster.
* Google Cloud Platform - cloud service provider facilitating infrastructure provisioning.
* Terraform             - IaC tool used to provision the infrastructure and maintain infrastructure consistency.
* Git repository        - source code management where infrastructure configuration, Kubernetes application manifests, and workload scripts are stored. GitHub is one supported option.
* Argo CD               - declarative, GitOps continuous delivery tool for Kubernetes.

## Project directories and files
The project is implemented in the following directories:

+ **gitops**    - Kubernetes application manifests for Argo CD, contains desired state of the cluster.
+ **terraform** - IaC configuration files to provision the infrastructure required for the GKE cluster.
+ **workloads** - Jenkins workflow scripts for the pipeline build jobs.
+ **tools**     - User management and automated deployment scripts.

## Configuration Placeholders
Throughout this document, you will encounter placeholders (e.g., <GCP_PROJECT_ID>) which represent values specific to your environment.
It is required to replace them with actual values as you follow the setup instructions. Below is a list of all the placeholders,

| Placeholder          | Description                                                                           | Example Value                             |
|----------------------|---------------------------------------------------------------------------------------|-------------------------------------------|
| `GCP_PROJECT_NUMBER` | Your GCP Project Number. ([more](#section-1b---gcp-project-details))                 | `9876543210`                              |
| `SUB_DOMAIN`         | The desired subdomain for your cluster apps.                                          | `dev`                                     |
| `HORIZON_DOMAIN`     | Your desired primary domain name.                                                     | `your-domain.com`                         |
| `GCP_PROJECT_ID`     | Your GCP Project ID. ([more](#section-1b---gcp-project-details))                     | `my-cloud-project-abc-123`                |
| `REPOSITORY_URL`     | The URL of your git repository.                                                       | `https://github.com/your-org/horizon-sdv` |
| `BRANCH_NAME`        | The branch of your repository to use for deployment.                                  | `feature-xyz` or `main`                   |
| `GIT_REPO_OWNER`     | Your git repository owner (user or organization name). ([more](#section-2c---configure-terraform-variables)) | `your-org`      |
| `GIT_REPO_NAME`      | Your git repository name. ([more](#section-2c---configure-terraform-variables))       | `horizon-sdv`                             |

## Section #1 - GCP Foundation Setup
This section covers creation and configuration of required Google Cloud Platform (GCP) services.

### Section #1a - GCP Prerequisites
#### Command-line Tools
> [!Note]
> Some of the resources can only be configured via the GUI.

If you do not prefer using the Cloud Shell on GCP Console, install GCP CLI tools like gcloud, gsutil and bq locally. (Install instructions [here](https://cloud.google.com/sdk/docs/install)).

#### Google Cloud Platform
* Configured GCP account/project.
* IAM Roles to be granted to new user accounts added by the owner of the project 
   - Editor
   - Service Account Admin
   - Project IAM Admin
   - Secret Manager Admin
   - Storage Admin
   - Compute Admin
   - Kubernetes Engine Admin
   - Kubernetes Engine Cluster Admin
   - DNS Administrator
   - Artifact Registry Administrator
   - Certificate Manager Admin
   - Parameter Manager Admin

   Refer this document for detailed instructions for adding new user accounts: [Add an account for a new user](https://support.google.com/cloudidentity/answer/33310?sjid=12027755488314741556-NC)

### Section #1b - GCP Project Details
> [!NOTE]
> The details shown below are only for example and may vary on your environment. 

It is required to perform the checks mentioned in this section as this information will be required later in the setup process.     

1. Default Google Compute Engine (GCE) Service Account:
   * On the console, click on IAM & Admin then, click on Service Accounts and confirm a Service Account for the GCE service is present.  
     <img src="images/gcp_gce_sa.png" width="400" />
2. Project ID and Project Number.
   * Click on the Google Cloud logo on the top left of the page which leads to the welcome page.   
      <img src="images/gcp_welcome_page.png" width="400" />
   * Click on Dashboard where you can find the Project ID and Project Number details mentioned in the Project info Card as below.   
      <img src="images/gcp_dashboard.png" width="400" />

<details>
<summary><code>gcloud</code> CLI</summary>
<ol>
<li>
Default Compute Service Account
<pre>
<code>gcloud iam service-accounts list \
   --filter="email ~ [0-9]+-compute@developer.gserviceaccount.com" \
   --format="value(email)"</code>
</pre>
</li>
<li>
GCP Project details
<pre>
<code>gcloud projects describe &lt;GCP_PROJECT_ID&gt;</code>
</pre>
</li>
</ol>
</details>

### Section #1c - Create a Bucket in GCP
In the current GCP project, it is required to create a GCP Bucket to store data related to the infrastructure. Follow the below steps to create a Bucket.
1. On the GCP Console, navigate to Cloud Storage and click on Buckets.
2. Click on CREATE/CREATE BUCKET button.
3. Enter a globally unique name for the bucket. (Example: `my-cloud-project-abc-123-bucket`)
4. Click on CONTINUE with default options for other sections.
5. Click on CREATE.
6. If any pop-up window appears, click on CONFIRM.

<details>
<summary><code>gcloud</code> CLI</summary>
<pre>
<code>gcloud storage buckets create gs://&lt;GCP_BACKEND_BUCKET_NAME&gt;</code>
</details>

### Section #1d - Create OAuth2 Client and Secret

> [!NOTE]
> This section covers setting up Google as the identity provider for Keycloak SSO. It is the documented and tested authentication path.
>
> The platform does not enforce Google as the only option. Keycloak supports many identity providers including Microsoft Entra ID (Azure AD), Okta, GitHub, SAML 2.0, LDAP, and any OIDC-compatible provider. Configuring a different provider is possible but not covered in this guide.
>
> Terraform does not require these credentials during deployment. The GCP Secret Manager entry for the OAuth2 client secret is initialized with a placeholder value by Terraform and must be updated manually after the cluster is running.
>
> If you already have a Google OAuth2 client, you can reuse it by adding the redirect URI: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/realms/horizon/broker/google/endpoint`
>
> Without completing this section, the platform deploys successfully and admin access via Keycloak username/password still works.

It is required to setup OAuth consent screen before creating the OAuth client and secret. Navigate to APIs & Services and follow the below mentioned steps

#### Setting up OAuth consent screen
Once in APIs & Services, click on OAuth consent screen to start the setup process.

1. Under Overview, click on GET STARTED.
2. Enter "Horizon - SDV" as the App name and select User support email of your choice.
3. Click on NEXT.
4. Under Audience, select External and click on NEXT.
5. Under Contact Information, provide an email address of your choice and click on NEXT.
6. Click on the checkbox under Finish and click on CONTINUE.
7. Click on CREATE.
8. Now, click on Branding. Scroll down and find App domain section.
9. Provide Application home page under App domain, example: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
10. Under Authorized domains, click on ADD DOMAIN and Provide Authorized domain 1 name, example: `<HORIZON_DOMAIN>`
11. Click on SAVE.   
   <img src="images/gcp_oauth_consent_screen.png" width="650" />   

#### Create OAuth client ID
1. Go to APIs & Services, click on Credentials.
2. Click on CREATE CREDENTIALS and select "OAuth client ID" from the list.
3. Select Application type as "Web application".
4. Provide Name as "Horizon".
5. Under Authorized redirect URIs enter the URI which points google endpoint of Keycloak.   
   Example: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/auth/realms/horizon/broker/google/endpoint`.
6. Clicking on CREATE opens a pop-up window containing client ID and secret which can be copied and saved locally to a file or download the credential detail as a JSON file. Click on OK for the pop-up window to close.  
   > **Note:** Please ensure you save the client secret immediately. For security reasons, the secret will be hidden later and cannot be copied again.   
   <img src="images/gcp_oauth_client_details.png" width="325" />
7. The credential will appear Under OAuth 2.0 Client IDs as below and credential details can be viewed and edited by clicking on the Name of the OAuth 2.0 Client ID.   
   <img src="images/gcp_oauth_client_list.png" width="325" />

<details>
<summary><strong>Managing Client Secret (Rotation & Storage)</strong></summary>

<br>

**If you lose the secret or need to rotate it:**
1. Naviagte to **APIs & Services** and click on **Credentials**.
2. Click on the name of your OAuth 2.0 Client ID (Horizon - SDV) in the Credentials list.
3. Click the **Add secret** button.
4. Copy the new **Client secret** and save it for future use.
5. Click on **Disable** and **Delete** (Bin icon) the old Client secret.
6. Click **Save**.
7. This will immediately revoke the old client secret and generate a new one.

**Store in GCP Secret Manager:**
1.  Navigate to **Security** > **Secret Manager** in the GCP Console.
2.  Click **CREATE SECRET**.
3.  Name the secret (e.g., `oauth2-client-secret`).
4.  In the **Secret value** field, paste the Client Secret you copied earlier.
5.  Click **Create secret**.
   
> **Important:** If you reset the OAuth Client secret manually in the future, you must manually add a new version to this Secret in Secret Manager with the new value. Terraform will not automatically update this value as the secret is generated outside of Terraform.

</details>

## Section #2 - Deployment
This section covers the steps to deploy the Horizon platform.

### Section #2a - Prerequisites
The below listed tools are required to be installed.
1. [Kubectl](https://kubernetes.io/docs/tasks/tools/) 
2. [Terraform](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
3. Docker ([Linux](https://docs.docker.com/engine/install/) | [Windows](https://docs.docker.com/desktop/setup/install/windows-install/))
4. [Google Cloud CLI](https://docs.cloud.google.com/sdk/docs/install-sdk)
5. The script can be run on [Google Cloud Shell](https://docs.cloud.google.com/shell/docs/launching-cloud-shell), Linux or Windows (with [WSL](https://learn.microsoft.com/en-us/windows/wsl/install)).

Container based (Requires Docker to be installed on the host machine) deployment is also provided with above-listed tools pre-installed. 

### Section #2b - Clone the Repository
Clone the [horizon-sdv](https://github.com/GoogleCloudPlatform/horizon-sdv) OSS repository. If you do not need to customize the platform, you can clone and deploy directly from the upstream repository.

If you wish to customize the platform or host the repository on your own git provider (GitHub, Gerrit, etc.), fork or mirror the repository to a location accessible from wherever you are running Terraform. If you are using GitHub, refer to [Section #5d - Fork the Repository](#section-5d---fork-the-repository) for instructions on creating a fork.

```shell
git clone <REPOSITORY_URL>
```

### Section #2c - Configure Terraform Variables
> [!IMPORTANT]
> The credentials below are for demonstration purposes and may be different for your environment.
> Create a strong password with at least 12 characters in length and containing a combination of,
> - Uppercase letters [A -Z]
> - Lowercase letters [a -z]
> - Numbers [0 - 9]
> - Symbols [!@#$%^&* etc.]
>
> Failing to do so might break the cluster and lead to unstable and insecure cluster behavior.

1. Navigate to `terraform/env` and create a copy of `terraform.tfvars.sample` in the same directory with name `terraform.tfvars`.
2. Edit `terraform.tfvars` and replace `<REQUIRED>` placeholders with actual values.
  - GCP Configuration
    - `sdv_gcp_project_id`: GCP Project ID from [Section #1b - GCP Project Details](#section-1b---gcp-project-details).
    - `sdv_gcp_region`: GCP Region of your choice (default: `europe-west1`).
    - `sdv_gcp_zone`: GCP Zone of your choice (default: `europe-west1-d`).
    - `sdv_gcp_compute_sa_email`: GCP Compute Engine service account email from [Section #1b - GCP Project Details](#section-1b---gcp-project-details).
    - `sdv_gcp_backend_bucket`: GCP storage bucket name from [Section #1c - Create a Bucket in GCP](#section-1c---create-a-bucket-in-gcp).
  - Environment Metadata
    - `sdv_env_name`: Environment name of Horizon SDV platform of your choice which will also be used as sub-domain name. Preferably set same value as `<SUB_DOMAIN>`.
    - `sdv_root_domain`: Domain name, same value as `<HORIZON_DOMAIN>`.
  - SCM Configuration (Source Code Management)
    - `scm_type`: Set to `github` for GitHub or `git` for other Git servers (Gerrit, GitLab, etc.).
    - `scm_auth_method`: Select authentication method:
      - `app`: GitHub App authentication (only for GitHub)
      - `userpass`: Username/Password or token (works with any Git server)
      - `none`: Public repository (no authentication required)
    - `scm_repo_url`: Full repository URL (e.g., `https://github.com/owner/repo` or `https://gerrit.example.com/a/project`)
    - `scm_repo_branch`: Branch name to deploy from
    - If using `userpass` authentication:
      - `scm_username`: Username (use `git` for GitHub PAT, actual username for Gerrit/GitLab)
      - `scm_password`: Password or token (GitHub PAT, Gerrit HTTP password, GitLab token, etc.)
    - If using `app` authentication (GitHub only):
      - `sdv_github_app_id`: GitHub App ID from [Section #1c - Create and Install GitHub Application](#section-1c---create-and-install-github-application).
      - `sdv_github_app_install_id`: 
         - Navigate to Organization Settings, GitHub Apps and click on "Configure".
         - Once in the GitHub App configuration page, the `GH_INSTALLATION_ID` is present in the URL of the page as below,
         - `https://github.com/organizations/<GH-ORG-NAME>/settings/installations/<INSTALLATION_ID>`
         - Enter the value of `<INSTALLATION_ID>`
       - `sdv_github_app_private_key`: GitHub App Private key downloaded from [Section #1c - Create and Install GitHub Application](#section-1c---create-and-install-github-application). Paste the content between `-----BEGIN RSA PRIVATE KEY-----` and `-----END RSA PRIVATE KEY-----`.
    - If using `none` authentication (public repos):
      - No credentials needed - simply provide the repository URL and branch
  - Required Admin Secrets
    - `sdv_keycloak_admin_password`: Plain text value following the required password rules mentioned above.
    - `sdv_keycloak_horizon_admin_password`: Plain text value following the required password rules mentioned above.
  - Optional
    - `enable_arm64`: ARM64 Support (enable: `true` or disable: `false`).
    - `sdv_enable_network_policies`: Enable or disable network policies for all workloads. Set to `false` to disable all NetworkPolicy resources (useful for debugging connectivity issues). Default: `true`.
    - `sdv_dns_dnssec_enabled`: Enable DNSSEC for the Cloud DNS zone. Requires domain ownership verification at [Google Search Console](https://search.google.com/search-console). Set to `false` to disable DNSSEC. Default: `true`.
    - `sdv_enable_kms_encryption`: Enable KMS encryption for Kubernetes secrets at rest using customer-managed encryption keys. Set to `false` (default) to skip KMS encryption and allow clean Terraform destroy. Set to `true` to enable application-layer secrets encryption. **Warning:** Once enabled and applied, KMS keyrings cannot be deleted from GCP; the only way to remove them is to delete the entire project.
    - `manual_secrets` : Optional map of secret values that can be set manually.
    - **Sub-Environments:** To deploy one or more sub-environments on the same cluster, define the `sdv_sub_env_configs` variable in `terraform.tfvars`. See the [Sub-Environment Deployment Guide](guides/sub_environments/sub_environment_deployment_guide.md#configuring-sub-environments) for the variable structure, required passwords, and naming rules.

### Section #2d - Run the Deployment Script
Steps to start the Terraform workflow.

**Containerized deployment:**
1. Navigate to `tools/scripts/deployment`
2. Provisioning can be done by running `./container-deploy.sh [options]` or (Requires **Docker** to be installed on the machine).
>**Options :**
  - `-p, --plan` Preview infrastructure changes without applying them. Example: `./container-deploy.sh -p` or `./container-deploy.sh --plan`
  - `-a, --apply` Provision or update the infrastructure using Terraform. Example: `./container-deploy.sh -a` or `./container-deploy.sh --apply`
  - `-d, --destroy` Deprovision all Terraform-managed infrastructure resources. Example: `./container-deploy.sh -d` or `./container-deploy.sh --destroy`
  - `-h, --help` Display the help message with available commands. Example: `./container-deploy.sh -h` or `./container-deploy.sh --help`

3. The script `./container-deploy.sh` supports [**Google Cloud Shell**](https://docs.cloud.google.com/shell/docs/launching-cloud-shell), Linux or Windows (with [WSL](https://learn.microsoft.com/en-us/windows/wsl/install)).

> [!Note]
> GCE Disks or other GCE resources provisioned by the GKE cluster will not be automatically removed while running
> deployment script with `--destroy` or `-d` flag as they are not managed by Terraform. A manual cleanup is required.

**Linux Native deployment:**
> [!Important]
> Use this deployment method ONLY on **Linux** (Ubuntu or Debian based distros), not tested on WSL or other platforms.

1. The below listed tools are required to be installed.
   1. [Kubectl](https://kubernetes.io/docs/tasks/tools/)
   2. [Terraform](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
   3. Docker - [Linux](https://docs.docker.com/engine/install/)
   4. [Google Cloud CLI](https://docs.cloud.google.com/sdk/docs/install-sdk)
2. Navigate to `tools/scripts/deployment`
3. Provisioning can be done by running `./deploy.sh [options]` (Requires all above-mentioned tools to be installed on the machine).
>**Options :**
  - `-p, --plan` Preview infrastructure changes without applying them. Example: `./deploy.sh -p` or `./deploy.sh --plan`
  - `-a, --apply` Provision or update the infrastructure using Terraform. Example: `./deploy.sh -a` or `./deploy.sh --apply`
  - `-d, --destroy` Deprovision all Terraform-managed infrastructure resources. Example: `./deploy.sh -d` or `./deploy.sh --destroy`
  - `-h, --help` Display the help message with available commands. Example: `./deploy.sh -h` or `./deploy.sh --help`

4. The script `./deploy.sh` supports Linux distributions (Ubuntu/Debian based distros).

> [!Note]
> GCE Disks or other GCE resources provisioned by the GKE cluster will not be automatically removed while running
> deployment script with `--destroy` or `-d` flag as they are not managed by Terraform. A manual cleanup is required.

## Section #3 - Post-Deployment Setup

### Domain Setup via GCP

If you do not have a domain registered already you can do so via GCP.

1. Go to: Network Services → Cloud Domains → Register Domain
1. Choose a domain name `<HORIZON_DOMAIN>`
1. Select Cloud DNS and enable DNSSEC
1. create

### Section #3a - Update Nameservers
To make your Cloud DNS zone active and reachable on the internet, you must update the Nameservers at your Domain Registrar. You must "point" your domain from the registrar to Google Cloud.

1. Navigate to Network Services, Cloud DNS.
2. Click on the Zone Name you created for`<HORIZON_DOMAIN>` .
3. Look at the list of records. You will see a record with Type NS.
4. Expand that record (if necessary) and copy all 4 values. They will look like:
  - `ns-cloud-a1.googledomains.com.`
  - `ns-cloud-b1.googledomains.com.`
  - `ns-cloud-c1.googledomains.com.`
  - `ns-cloud-d1.googledomains.com.`
5. Update your Domain Registrar with custom nameservers.

The process may differ based on your Domain Registrar.

#### Update Nameservers for GCP Registered Domains

Grab Terraform's Name Servers
1. Go to Network Services > Cloud DNS in your console.
1. Click on the Terraform zone: `dev-horizon-sdv-com.`
1. Look for the record with the type `NS`.
1. Under the "Data" column, you will see 4 lines that look like `ns-cloud-xx.googledomains.com.`. Copy all 4 of those lines.

Link them together
1. Go back to your Cloud DNS zones list and click on the root zone: `<HORIZON_DOMAIN>-com.`
1. Click + Add Standard (or Add Record Set) at the top.
1. DNS Name: type `<SUB_DOMAIN>` in the box (so the full name below it reads `<SUB_DOMAIN>.<HORIZON_DOMAIN>-com.`).
1. Resource Record Type: select `NS` from the dropdown.
1. Data: Paste the 4 lines you copied earlier.
1. Click Create.

#### DNSSEC implementation details (if enabled)

If you set `sdv_dns_dnssec_enabled = true` in `terraform.tfvars`, Terraform creates a Cloud DNS zone with DNSSEC. The following applies only when DNSSEC is enabled; you can set `sdv_dns_dnssec_enabled = false` to skip these steps.

##### Zone creation and authorization 

When DNSSEC is enabled, Terraform creates a zone during deployment. You must verify that the account is authorized to create that zone.
This can be done in [Google Search Console](https://search.google.com/search-console).

1. In the Domain box on the left, type `<HORIZON_DOMAIN>.com` and click Continue.
1. If it says "Ownership auto-verified": You are done. You can close the tab.
1. If it gives you a TXT record to copy:
   1. Copy the TXT record.
   1. Go back to your root Cloud DNS zone (`<HORIZON_DOMAIN>-com`).
   1. Click + Add Standard. Leave the DNS name blank (so it applies to the root domain), select TXT as the type, paste the code, and save it.
   1. Go back to the Search Console and click Verify.

##### DS record (optional)
After the zone is created by Terraform, you can retrieve a **DS** record and add it to the main `horizon-sdv.com` zone to make the domain fully DNSSEC compliant. The setup works even without this step.

1. Get the Key from Terraform:
   1. Go to Network Services > Cloud DNS.
   1. Click on your Terraform zone: `dev-horizon-sdv-com.`
   1. Look in the top-right corner of the screen and click Registrar Setup.
   1. You will see a section for DS Records. Copy the data.
1. Paste the Key into your Main Zone:
   1. Go back to your Cloud DNS zones list and click your root zone: `<HORIZON_DOMAIN>-com`.
   1. Click + Add Standard (Add record).
   1. DNS Name: type `<SUB_DOMAIN>.`
   1. Resource Record Type: select `DS`.
   1. Data: Paste the DS record data you copied from the other tab.
   1. Click Create.

- **Verification:** DNSSEC can be verified on the [DNSSEC Debugger](https://dnssec-debugger.verisignlabs.com/) site.

- **Disabling DNSSEC:** DNSSEC can be disabled in `terraform.tfvars`; in that case the steps above are not needed.

### Section #3b - Connect to GKE via Connect Gateway
Follow the steps mentioned in this section to connect to the GKE cluster using the Connect Gateway.   

1. Install the pre-requisite tools `gcloud` (with `gke-gcloud-auth-plugin` component) and `kubectl` CLI tools.
2. Authenticate to Google Cloud using below command,
    ```
    gcloud auth login
    ```
3. Use the Fleet membership name to generate the credentials. Replace `<MEMBERSHIP_NAME>` with your GKE cluster name.
    ```
    gcloud container fleet memberships get-credentials <MEMBERSHIP_NAME> --project=<GCP_PROJECT_ID>
    ```
4. Verify the connection using below command,
    ```
    kubectl get nodes
    ```

### Section #3c - Setup Keycloak
Follow the steps mentioned in this section once the cluster is provisioned and is running successfully to configure Keycloak.

#### Login as Horizon admin
1. Keycloak UI can be accessed here from the Landing page under 'Admin Applications': `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`   
   <img src="images/keycloak_launch.png" width="325" />
2. Login to Keycloak as admin with below credentials,
   - username: `horizon-admin`
   - password:  Use value of `sdv_keycloak_horizon_admin_password` as configured in [Section #2c - Configure Terraform Variables](#section-2c---configure-terraform-variables).
3. On your first login, you may be prompted to update your password. Create a new, strong password, confirm it, and click Submit.
4. On the next screen, enter your email address, first name, and last name as below.
   - Email Adress: `horizon.admin@keycloak.com`
   - First name: `horizon`
   - Last name: `admin`
5. Click Submit.

#### Configure Google Identity Provider
1. Login to Keycloak and click on "Identity providers".
2. Click "Add provider" and select "Google" provider.
3. Ensure that the Redirect URI matches the one set in the [Create OAuth client ID](#create-oauth-client-id) section.
4. Enter the values for `Client ID` and `Client Secret` as mentioned in the step 7 of the section [Create OAuth client ID](#create-oauth-client-id).   
   <img src="images/keycloak_google_identity_provider.png" width="750" />
5. Click on Add.

#### Configure a new Authentication flow
1. Login to Keycloak, navigate to Authentication and click "Create flow". Name it as "broker link existing user" and click on Create.
2. Click "Add execution".
3. In the pop-up search box, search for "Detect existing broker user" and click on "Add".   
   <img src="images/keycloak_create_authentication_flow_1.png" width="750" />
4. Switch the value of "Requirement" field for the step "Detect existing broker user" to "Required" from the drop-down list as below   
   <img src="images/keycloak_create_authentication_flow_2.png" width="750" />
5. Click on "Add execution".
6. In the pop-up search box, search for "Automatically set existing user" and click on "Add".   
   <img src="images/keycloak_create_authentication_flow_3.png" width="750" />
7. Switch the value of "Requirement" field for the step "Automatically set existing user" to "Required" from the drop-down list as below   
   <img src="images/keycloak_create_authentication_flow_4.png" width="750" />
8. Next, go to "Identity providers" and select "google".
9. Click on "Advanced settings" look for "First login flow override". Switch the value of "First login flow override" to "broker link existing user" as shown below   
   <img src="images/keycloak_create_authentication_flow_5.png" width="750" />
10. Finally, click on "Save".

#### Create first human admin user
After logging in as the Horizon admin, follow these steps to create a human user account with administrative privileges:

1. Click on Users, then click on Add user.
2. Toggle Email Verified to On.
3. Enter the required details for username, email, first name, and last name. (Note: Use the full email address for both the username and email fields.)
4. Click on Create.
5. On the next page, switch to Role mapping tab and click on Assign role.
6. Click on "Realm roles"   
   <img src="images/keycloak_assign_role_1.png" width="750" />
7. Search for "realm_admin" and click on the checkbox to select the role.
8. Click on Assign.   
   <img src="images/keycloak_assign_role_2.png" width="750" />
9. Switch to the Details tab, click on Save.
10. Click on username on top right corner of the page and click on Sign out.
11. Log in using the newly created human admin account to perform subsequent configurations. (Sign in with Google)

Repeat the above steps to add additional users with the required access privilege level.

### Section #3d - Jenkins Access via Keycloak Groups
>[!NOTE]
>Admin access to Keycloak is required for this section.

This section includes the steps to assign a user to a Keycloak group to enable Jenkins access. Jenkins integrates with Keycloak using Role-based Authorization Strategy (RBAC).
Group membership determines the level of access granted to the user.

#### Available Groups
Below table details the Keycloak to jenkins RBAC mapping with their access level granted to users within the respective groups.

| Keycloak Group                                 | Jenkins Role                         | Access Level                           |
|------------------------------------------------|--------------------------------------|----------------------------------------|
| `administrators`                               | Global: Admin                        | Full admin access                      |
| `developers`                                   | Item: workloads-developers           | Full build/config rights for Workloads |
| `viewers`                                      | Item: workloads-viewers              | Limited build access for Workloads     |

#### Steps to Assign a User to a Group
>[!NOTE]
>Log out and log in again from Jenkins for the new permissions to take effect.

Follow the below steps to assign a user to required Keycloak group,

1. Keycloak UI can be accessed here from the Landing page under 'Admin Applications': `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
   - Login to Keycloak as admin.   
      <img src="images/keycloak_launch.png" width="325" />
2. Find the User
   - Go to `Users` in the left sidebar.
   - Use the search bar to locate the user.
   - Click on the username to open their details.
3. Assign the Group
   - **admininstrators**
      - Click on the **Groups** tab.
      - Click on **Join Group** which opens a new pop-up window.
      - Select the group `admininstrators`.
      - Click **Join**.   
         <img src="images/keycloak-jenkins-groups-1.png" width="325" />
   - **developers**
      - Click on the **Groups** tab.
      - Click on **Join Group** which opens a new pop-up window.
      - Select the group `developers`.
      - Click **Join**.   
         <img src="images/keycloak-jenkins-groups-2.png" width="325" />
   - **viewers**
      - Click on the **Groups** tab.
      - Click on **Join Group** which opens a new pop-up window.
      - Select the group `viewers`.
      - Click **Join**.   
         <img src="images/keycloak-jenkins-groups-3.png" width="325" />
4. Verify Group Assignment
   - The group should now appear under the user's "Group Membership".

### Section #3e - Argo CD Access via Keycloak Groups
This section includes the steps to assign a user to a Keycloak group to enable Argo CD access. Group membership determines the level of access granted to the user.

#### Available Groups
Below table details the Keycloak to Argo CD mapping with their access level granted to users within the respective groups.

| Keycloak Group                                 | Argo CD Role                         | Access Level                             |
|------------------------------------------------|--------------------------------------|------------------------------------------|
| `administrators`                               | role: admin                          | Full admin access                        |

#### Steps to Assign a User to a Group
>[!NOTE]
>Log out and log in again from Argo CD for the new permissions to take effect.

Follow the below steps to assign a user to required Keycloak group,

1. Keycloak UI can be accessed here from the Landing page under 'Admin Applications': `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
   - Login to Keycloak as admin.   
      <img src="images/keycloak_launch.png" width="325" />
2. Find the User
   - Go to `Users` in the left sidebar.
   - Use the search bar to locate the user.
   - Click on the username to open their details.
3. Assign the Group
   - **admininstrators**
      - Click on the **Groups** tab.
      - Click on **Join Group** which opens a new pop-up window.
      - Select the group `admininstrators`.
      - Click **Join**.   
         <img src="images/keycloak-argocd-groups-1.png" width="325" />
4. Verify Group Assignment
   - The group should now appear under the user's "Group Membership".

### Section #3f - Headlamp Access via Keycloak Groups
This section includes the steps to assign a user to a Keycloak group to enable Headlamp access. Group membership determines the level of access granted to the user.

#### Available Groups
Below table details the Keycloak to Headlamp mapping with their access level granted to users within the respective groups.

| Keycloak Group                                 | Headlamp Role                        | Access Level                             |
|------------------------------------------------|--------------------------------------|------------------------------------------|
| `administrators`                               | role: cluster-admin                  | Full admin access                        |

#### Steps to Assign a User to a Group
>[!NOTE]
>Once logged-in to Headlamp, you may not be able to sign-out.

Follow the below steps to assign a user to required Keycloak group,

1. Keycloak UI can be accessed here from the Landing page under 'Admin Applications': `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
   - Login to Keycloak as admin.   
      <img src="images/keycloak_launch.png" width="325" />
2. Find the User
   - Go to `Users` in the left sidebar.
   - Use the search bar to locate the user.
   - Click on the username to open their details.
3. Assign the Group
   - **admininstrators**
      - Click on the **Groups** tab.
      - Click on **Join Group** which opens a new pop-up window.
      - Select the group `admininstrators`.
      - Click **Join**.
         <img src="images/keycloak-headlamp-groups-1.png" width="325" />
4. Verify Group Assignment
   - The group should now appear under the user's "Group Membership".

### Section #3g - Grafana Access via Keycloak Groups
This section includes the steps to assign a user to a Keycloak group to enable Grafana access. Group membership determines the level of access granted to the user. 

#### Available Groups
Below table details the Keycloak to Grafana mapping with their access level granted to users within the respective groups.

| Keycloak Group                                 | Access Level                                                                   |
|------------------------------------------------|--------------------------------------------------------------------------------|
| `administrators`                               | Full admin access. Edit dashboard, and other settings admin permissions        |
| `viewers`                                      | Viewers access. Only limited access, possible to view dashboards               |

#### Steps to Assign a User to a Group
>[!NOTE]
>Users not assigned to any group cannot log in to Grafana.

Follow the below steps to assign a user to required Keycloak group,

1. Keycloak UI can be accessed here from the Landing page under 'Admin Applications': `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
   - Login to Keycloak as admin.   
      <img src="images/keycloak_launch.png" width="325" />
2. Find the User
   - Go to `Users` in the left sidebar.
   - Use the search bar to locate the user.
   - Click on the username to open their details.
3. Assign the Group
   - **admininstrators** or **viewers**
      - Click on the **Groups** tab.
      - Click on **Join Group** which opens a new pop-up window.
      - Select the group `admininstrators` or `viewers`.
      - Click **Join**.   
         <img src="images/keycloak-grafana-groups-1.png" width="325" />
4. Verify Group Assignment
   - The group should now appear under the user's "Group Membership".

### Section #3h - MCP Gateway Registry Access via Keycloak Groups
This section includes the steps to assign a user to a Keycloak group to enable MCP Gateway Registry access. Group membership determines the level of access granted to the user.

#### Available Groups
Below table details the Keycloak to MCP Gateway Registry mapping with their access level granted to users within the respective groups.

| Keycloak Group                                 | Access Level                                                                   |
|------------------------------------------------|--------------------------------------------------------------------------------|
| `administrators`                               | Full admin access for both UI and API. Add new or Edit registered MCP servers and agents, and other admin permissions        |
| `viewers`                                      | View-only access for UI to view existing MCP Servers and agents added by admins. Full usage access for MCP servers and agents. Read-only access for API.               |

#### Steps to Assign a User to a Group
>[!NOTE]
>Users not assigned to any group can log in to Web UI but cannot see any MCP servers or agents.

Follow the below steps to assign a user to required Keycloak group,

1. Keycloak UI can be accessed here from the Landing page under 'Admin Applications': `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>`
   - Login to Keycloak as admin.   
      <img src="images/keycloak_launch.png" width="325" />
2. Find the User
   - Go to `Users` in the left sidebar.
   - Use the search bar to locate the user.
   - Click on the username to open their details.
3. Assign the Group
   - **administrators** or **viewers**
      - Click on the **Groups** tab.
      - Click on **Join Group** which opens a new pop-up window.
      - Select the group `administratord` or `viewers`.
      - Click **Join**.   
         <img src="images/keycloak_mcp_gateway_registry_groups.png" width="325" />
4. Verify Group Assignment
   - The group should now appear under the user's "Group Membership".


### Section #3i - Enable Workloads Modules in Developer Portal

> [!NOTE]
> Admin access via Keycloak `administrators` group is required. See [Section #3d - Jenkins Access via Keycloak Groups](#section-3d---jenkins-access-via-keycloak-groups) for group assignment steps.

Where Android workloads are required, enable modules in the **Horizon Developer Portal** so that Jenkins seed jobs and workflow templates work properly:

1. Open **Administration → Modules** at
   `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/developer-portal/admin/modules`
2. Under **Administration**, enable **`workloads-common`**, then **`workloads-android`**, in that order.

Skip or adjust modules if your environment intentionally does not use Android workloads.

After modules are enabled, run the Jenkins seed job and regenerate workload templates as described in [`workloads/seed.md`](workloads/seed.md).

---

## Section #4 - Run Cluster Apps
This section details how to sign in to and use cluster applications, including their functionalities within the cluster environment. When using sub-environments, each sub-environment has its own landing page and application URLs at `https://<SUB_ENV_NAME>.<SUB_DOMAIN>.<HORIZON_DOMAIN>`. See the [Sub-Environment Deployment Guide – Accessing Sub-Environment Applications](guides/sub_environments/sub_environment_deployment_guide.md#accessing-sub-environment-applications) for the URL pattern and application list.

### Section #4a - Horizon Landing Page
You can access the landing page by going to `https://<SUB_DOMAIN><HORIZON_DOMAIN>` which enables you to launch any of the applications running within the Horizon GKE Cluster.    

There are two types of Apps
- Applications - Cluster Apps non-admin users can access.
- Admin Applications - Cluster Apps only the admin users can access and perform cluster administrative tasks.

You can click on the 'Launch' button within each cluster application's card on the Horizon landing page to open the application of your choice.   
<img src="images/horizon_landing_page.png" width="750" />

### Section #4b - Argo CD
Argo CD is the GitOps tool being used with a git repository as the "source of truth" where the desired state of Kubernetes applications have been configured. 
It ensures the Kubernetes Cluster (GKE) always matches that desired state. Here, Argo CD is configured to automatically sync so, no user intervention is usually required.  

1. To Access Argo CD UI, go to the Horizon Landing page here: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>` and click on the Launch button within the Argo CD app card as below.   
   <img src="images/argocd_launch.png" width="325" />
2. Log-in to Argo CD by clicking on the "Log in via Keycloak" button. (Your user must be assigned to `administrators` on Keycloak for SSO access)   

<details>
  <summary>Click for more details on Argo CD</summary>

   #### View Application Status
   Check the health and sync status (synced or out-of-sync) and overall status of the deployed applications as below from the home page.   
   <img src="images/argo_cd_1.png" width="750" />

   #### Monitor Application Health
   Check the health status of individual resources within an application (deployments, services, etc.).
   You can click on any application to further investigate the status and health of individual apps.   
   <img src="images/argo_cd_2.png" width="750" />

   #### Inspect Application Resources
   > :warning: WARNING   
   > It is not recommended to edit files using Argo CD GUI as it may lead to conflict in desired
   > state of the cluster

   View the Kubernetes resources `.yaml` manifests that defines an application. You can do this by clicking on any application of your choice from the home page and then click on any resource you would like to inspect, change to "manifest" tab.   
   <img src="images/argo_cd_3.png" width="750" />

   #### View Application Events
   View events related to application deployment, sync, and health changes. You can do this by clicking on any application of your choice from the home page and then click on any resource you would like to inspect, change to "events" tab.   
   <img src="images/argo_cd_4.png" width="750" />

   #### Trigger Manual Sync
   Force an application to synchronize with the Git repository to apply latest changes. To force an application to sync, you can click the sync button as below,   
   <img src="images/argo_cd_5.png" width="325" />   
   This will open another window where you can further apply more sync options based on your needs and click on "synchronize".

   #### Additional operations that you can perform using Argo CD
   1. View Application History/Rollback: See the deployment history and rollback to a previous application version.
   2. Compare Application Versions (Diff): See the differences between the currently deployed application and the desired state in Git.
   3. Delete Applications: Remove applications deployed by Argo CD.
   4. View Application Logs (Potentially): Access logs of application components (depending on setup and integrations).
</details>   

### Section #4c - Keycloak
Keycloak is the Identity and Access Management (IAM) application provides features like authentication and authorization. It centralizes user management for all applications on the cluster.   

1. To Access Keycloak UI, go to the Horizon Landing page here: https://<SUB_DOMAIN>.<HORIZON_DOMAIN> and click on the Launch button within the Keycloak app card as below.   
   <img src="images/keycloak_launch.png" width="325" />
2. Log-in to Keycloak using the credentials configured in [Section #2c - Configure Terraform Variables](#section-2c---configure-terraform-variables).

Refer section [Section #3c - Setup Keycloak](#section-3c---setup-keycloak) for steps to create and manage users.   
   
<img src="images/keycloak_homepage.png" width="750" />

### Section #4d - Gerrit
Gerrit is a web-based code review tool built on top of the git version control system.

1. To Access Gerrit, go to the Horizon Landing page here: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>` and click on the Launch button within the Gerrit app card as below.   
   <img src="images/gerrit_launch.png" width="325" />
2. Login using google sign-in.   
   <img src="images/horizon_login_with_google.png" width="300" />   

Below is a view of Gerrit homepage,   
<img src="images/gerrit_homepage.png" width="750" />

### Section #4e - Jenkins
Jenkins is an open-source automation server. It's primary use-case is to automate tasks related to running Android workloads on the cluster. It is a core tool for Continuous Integration and Continuous Delivery pipelines.

1. To Access Jenkins, go to the Horizon Landing page here: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>` and click on the Launch button within the Jenkins app card as below.   
   <img src="images/jenkins_launch.png" width="325" />
2. Login using google sign-in.   
   <img src="images/horizon_login_with_google.png" width="300" />   

Below is a view of the Jenkins dashboard,   
<img src="images/jenkins_dashboard.png" width="750" />

### Section #4f - MTK connect
MTK Connect provides connectivity to remote devices for automated and manual testing.   

1. To Access MTK Connect, go to the Horizon Landing page here: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>` and click on the Launch button within the MTK Connect app card as below.   
   <img src="images/mtk-connect_launch.png" width="325" />
2. Login using google sign-in.   
   <img src="images/horizon_login_with_google.png" width="300" />    

Below is a view of the MTK connect homepage,   
<img src="images/mtk-connect_homepage.png" width="750" />

### Section #4g - Headlamp
The Headlamp application in Kubernetes provides, extensible web-based user interface (UI) designed to simplify the management and visualization of Kubernetes clusters.  

1. To Access Headlamp, go to the Horizon Landing page here: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>` and click on the Launch button within the Headlamp app card as below.   
   <img src="images/headlamp_launch.png" width="325" />
2. Login using google sign-in.   
   <img src="images/horizon_login_with_google.png" width="300" /> 

Below is a view of the Headlamp homepage,   
<img src="images/headlamp_home.png" width="750" />

### Section #4h - Grafana
Grafana is an open-source platform for monitoring and observability that enables users to visualize, analyze, metrics, logs, and traces from various data sources. It is widely used to build interactive dashboards for real-time system performance tracking and troubleshooting. Used to monitoring pods and intances in horizon-sdv GCP

1. To Access Grafana, go to the Horizon Landing page here: `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>` and click on the Launch button within the Grafana app card as below.   
   <img src="images/grafana_launch.png" width="325" />
2. Login using google sign-in.   
   <img src="images/horizon_login_with_google.png" width="300" /> 

Below is a view of the Grafana homepage,   
<img src="images/grafana_home.png" width="750" />

### Section #4i - MCP Gateway Registry
MCP Gateway Registry is a centralized platform for managing, monitoring and authenticating to MCP (Model Context Protocol) servers and agents deployed in Horizon cluster or other environments.

1. To Access MCP Gateway Registry, directly go to `https://mcp.<SUB_DOMAIN>.<HORIZON_DOMAIN>`.
2. Click on "Login with Keycloak" button.   
   <img src="images/mcp_gateway_registry_login_page.png" width="325" />
3. Login using google sign-in.   
   <img src="images/horizon_login_with_google.png" width="300" /> 

Below is a view of the MCP Gateway Registry homepage where you should see the pre-registered gerrit-mcp-server,   
<img src="images/mcp_gateway_registry_home_page.png" width="750" />


## Section #5 - Optional - GitHub Setup

> [!NOTE]
> This section is only required if you are using GitHub as your git provider. If you are using a GitHub Personal Access Token (PAT) without a GitHub organization, only [Section #5a - GitHub Prerequisites](#section-5a---github-prerequisites) and [Section #5d - Fork the Repository](#section-5d---fork-the-repository) apply. If you are using GitHub App authentication, all sub-sections apply.

### Section #5a - GitHub Prerequisites
#### Command-line Tools
> [!Note]
> Some of the resources can only be configured via the GUI.

- If you prefer using the GitHub CLI install ([windows](https://github.com/cli/cli/blob/trunk/README.md#windows) or [Linux](https://github.com/cli/cli/blob/trunk/docs/install_linux.md#installing-gh-on-linux-and-bsd)) and [login](https://cli.github.com/manual/gh_auth_login).
- [Git](https://git-scm.com/downloads) is installed and configured.

#### GitHub
- Each team-member has a GitHub account.
- Team-member with admin privileges able to update configuration in settings such as Secrets and Variables to customize it to use by the team.

### Section #5b - Create GitHub Organization
> [!Note]
> Creating GitHub Organization is optional. If not using GitHub organization, the repository owner changes to user account forking the repository.

Before we get started on creating a GitHub organization, it is required to have a GitHub account. If you do not have a GitHub account already, sign up [here](https://docs.github.com/en/get-started/start-your-journey/creating-an-account-on-github).

1. Log in to GitHub, click on your profile (profile icon located at top-right corner of the page) and select "Your organizations".
2. Click on "New Organization" under Organizations.
3. Click on "Create a free organization".
4. Enter Organization name of your choice.
5. Enter your email address as the Contact email.
6. Set organization belonging to "My personal account".
7. Complete the verification challenge under Verify your account.
8. Accept the terms of service and click on Next.
9. In the next step, you can add members to the organization or skip and add members later. Click on "Complete setup".

### Section #5c - Create and Install GitHub Application
> [!Note]
> GitHub App creation and installation is optional if using GitHub PAT (Personal Access Token) as the authentication method, more details in later sections of the guide.
>
> When setting up GitHub Apps for your GitHub organization, there are two distinct sections within your GitHub organization's settings where "GitHub Apps" are listed. This can sometimes cause confusion.
> - **Organization settings > GitHub Apps** to see which apps are already connected to your organization.   
> - **Organization settings > Developer settings > GitHub Apps** to create and configure new GitHub Apps that your organization will use or offer.

1. Go to the GitHub organization settings tab, scroll down and click on Developer settings and select "GitHub Apps".
2. Click on "New GitHub App".
   * Enter the GitHub App name as "horizon-sa" (If the name has already been taken, provide your desired post-fix for the app name instead of "sa")
   * Enter `https://github.com/` as the Homepage URL.
   * Scroll down and uncheck the "Active" checkbox Under Webhook.
   * Under Permissions, click on Repository permissions and update Access for Contents to "Read-only". (This change will update Metadata permission to Read-only)   
      <img src="images/github_app_contents_permission.png" width="750" />
3. Click on Create GitHub App.
4. To create a Private Key, 
   * Go to your GitHub organization, Settings, Developer settings, GitHub Apps and click on the "Edit" Button for "horizon-sa".
   * Scroll down and under Private keys, click on "Generate a private key"   
      <img src="images/github_app_private_key_generation.png" width="425" />
   * Download and Save the `.pem` file to your machine locally.   
5. To note down the GitHub App ID, navigate to your GitHub organization, Settings, Developer settings, GitHub Apps and click on "horizon-sa" and note down the info as shown below   
   <img src="images/github_app_id.png" width="450" />
6. Installing the GitHub App
   * Go to your GitHub organization, Settings, Developer settings, GitHub Apps and click on "horizon-sa".
   * Click on Install App.
   * Click on Install, select "All repositories" and click on "Install" again.
7. To verify the installation, go to your GitHub organization settings and click on GitHub Apps and it should look like below.   
   (GitHub App name may differ in your environment)     
   <img src="images/github_app_confirm_installation.png" width="750" />

### Section #5d - Fork the Repository
Use the following steps to fork the [horizon-sdv](https://github.com/GoogleCloudPlatform/horizon-sdv) GitHub repository into your GitHub organization (or your personal account, if not using an organization):

1. Open the [horizon-sdv](https://github.com/GoogleCloudPlatform/horizon-sdv) repository on GitHub.
2. Click the **Fork** button dropdown and select **Create a new fork**, as shown below.  
   <img src="images/github_create_fork_1.png" width="425" />
3. In the **Owner** field, choose your GitHub organization (or your user account), then click **Create fork**.  
   <img src="images/github_fork_horizon_repository.png" width="750" />
4. Once created, the forked repository will appear under your selected GitHub organization (or your user account).
<details>
<summary><code>gh</code> CLI command</summary>

<pre><code>gh repo fork &lt;SOURCE_GITHUB_ORGANIZATION_NAME&gt;/&lt;SOURCE_GITHUB_REPOSITORY_NAME&gt; --org &lt;GITHUB_ORGANIZATION_NAME&gt;</code></pre>
</details>

## Section #6 - Troubleshooting

### Section #6a - Keycloak sign-in failure
Check the below sections for issues related to sign-in after the cluster has been provisioned and Keycloak is setup with google identity provider.

#### Redirect URI mismatch
- Log-in to Keycloak and select "Horizon" realm.
- Go to Identity providers and click on "google" provider.
- Copy the redirect URI available in the google identity provider.
- Now, go to "APIs & Services" click and open "Credentials".
- Click on "Horizon" under OAuth 2.0 Client IDs.
- Make sure the URI mentioned under "Authorized redirect URIs" is the same as the one in Keycloak google provider.
- If the URIs do not match, replace the URI mentioned under "Authorized redirect URIs".

#### User does not exist with google identity provider
If you are getting the error "User <USER_NAME> authenticated with Identity provider google does not exist. Please contact your administrator".   
- Log-in to Keycloak and select "Horizon" realm.
- Go to Users, click on your user.
- Make sure both Username and Email fields have the complete Email address.

### Section #6b - Error when reading or editing Certificate
```shell
│  Error: Error when reading or editing Certificate: googleapi: Error 400: can't delete certificate that is referenced by a CertificateMapEntry or other resources
│ Details:
│ [
│   {
│     "@type": "type.googleapis.com/google.rpc.PreconditionFailure",
│     "violations": [
│       {
│         "description": "can't delete certificate that is referenced by a CertificateMapEntry or other resources",
│         "subject": "projects/<PROJECT_NUMBER>/locations/global/certificates/horizon-sdv",
│         "type": "RESOURCE_STILL_IN_USE"
│       }
│     ]
│   }
│ ]
```
If facing above error, as a workaround, run the below command to temporarily enable edit or removal of the certificate.

```shell
gcloud certificate-manager maps delete horizon-sdv-map
```

### Section #6c - Docker permission denied
If you encounter below error on Linux,
```
Building deployer image...
ERROR: permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock: Head "http://%2Fvar%2Frun%2Fdocker.sock/_ping": dial unix /var/run/docker.sock: connect: permission denied
```

Follow post-install steps as shown in [Manage Docker as a non-root user](https://docs.docker.com/engine/install/linux-postinstall/)

### Section #6d - Docker Container build resource issues
```shell
│ Error: Error running legacy build: process "/bin/sh -c apt-get update && apt-get install -y     curl     git     build-essential     && rm -rf /var/lib/apt/lists/*" did not complete successfully: exit code: 100
│
│
│
│   with module.base.module.sdv_container_images.docker_image.sdv-container-images["gerrit-mcp-server-app"],
│   on ../modules/sdv-container-images/main.tf line 20, in resource "docker_image" "sdv-container-images":
│   20: resource "docker_image" "sdv-container-images" {
│
```
 Building Docker containers can be resource-intensive. If you encounter build failures (such as `exit code: 100` as shown above or unexpected timeouts), please increase the CPU and Memory allocated to Docker and re-run the deployment script.

### Section #6e - Error creating SslPolicy
```shell
│ Error: Error creating SslPolicy: googleapi: Error 409: The resource 'projects/<PROJECT_ID>/global/sslPolicies/gke-ssl-policy' already exists, alreadyExists
│
│   with module.base.module.sdv_ssl_policy.google_compute_ssl_policy.gke_ssl_policy,
│   on ../modules/sdv-ssl-policy/main.tf line 21, in resource "google_compute_ssl_policy" "gke_ssl_policy":
│   21: resource "google_compute_ssl_policy" "gke_ssl_policy" {
│
```
A manual workaround for this error, run the below command,

```shell
gcloud compute ssl-policies delete gke-ssl-policy --global --project=sdva-2108202401
```

### Section #6f Homepage not reachable after deployment

If you cannot access `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>` but you can access `https://<SUB_DOMAIN>.<HORIZON_DOMAIN>/argocd` you most likely encounter a cluster issue. 

Log in with the Argo CD admin credentials:
- **Username:** `admin`
- **Password:** Retrieve from GCP Secret Manager under the secret named `argocd-admin-password-b64`. Could be retrieve from terraform.tfvars configuration file manual_secrets named s5 (if set).

  - If the password should not work you need to set a new password in `terraform.tfvars` appr. line 184 `manual_secrets = { s5 = <mypw> ...`
  - redeploy

- After login to argocd check the health state of the `horizon-sdv` application.
- If it should be unhealthy hit “refresh” and then “sync”.
- Wait for the cluster to update and recover to “Healthy” state.


### Section #6g Jenkins cant run pipeline as admin

If you should be unable to run any pipeline as admin while getting an error regarding unsufficient permissions try the steps below.

1. Go to Manage Jenkins > Manage and Assign Roles > Assign Roles
1. Under Global Roles:
   1. In the "User/group to add" box, type your exact email you are logged in with (see user profile in jenkins): yourname@yourmail.com
   1. Click Add.
   1. In the new row that appears, check the box for horizon-jenkins-administrators.
1. Under Item Roles:
   1. In the "User/group to add" box, type your exact email again: yourname@yourmail.com
   1. Click Add.
   1. Check the box for workloads-developers.
1. Click Save at the bottom of the page.
