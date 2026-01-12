# Gitops documentation

## Table of contents
- [GitOps overview](#gitops-overview)
- [GitOps in Horizon SDV project](#gitops-in-horizon-sdv-project)
- [GitOps deployment process](#gitops-deployment-process)
- [ArgoCD overview](#argocd-overview)
- [Applications](#applications)
    - [Keycloak](#keycloak)
    - [Jenkins](#jenkins)
    - [Gerrit](#gerrit)
    - [MTK Connect](#mtk-connect)
    - [Headlamp](#headlamp)
    - [Gerrit MCP Server](#gerrit-mcp-server)
    - [MCP Gateway Registry](#mcp-gateway-registry)
    - [Landing Page](#landing-page)
- [Dependencies](#dependencies)
    - [Dynamic PVC Provisioner and Releaser](#dynamic-pvc-provisioner-and-releaser)
    - [PostgreSQL](#postgresql)
    - [Zookeeper](#zookeeper)
    - [MongoDB](#mongodb)
    - [Gerrit Operator](#gerrit-operator)
    - [Gerrit MCP Server](#gerrit-mcp-server)
    - [External Secrets](#external-secrets)
    - [External DNS](#external-dns)
    - [OAuth2 Proxy](#oauth-proxy)
    - [Token Injector](#token-injector)
    - [Post Jobs](#post-jobs)

## GitOps overview

GitOps is a deployment approach that uses git as the source of truth for infrastructure and application configurations. Changes are made through Git, and tools like ArgoCD automatically apply them to Kubernetes clusters, ensuring consistency between the repository and the running environment. This allows for automated, version-controlled deployments without manual intervention.


## GitOps in Horizon SDV project

In the Horizon SDV platform, GitOps is used to manage applications and their dependencies using ArgoCD. The platform includes applications such as Keycloak, Gerrit, Jenkins, MTK Connect, MCP Gateway Registry and LandingPage, along with dependencies like Dynamic PVC Provisioner and Releaser, PostgreSQL, Zookeeper, MongoDB, Gerrit Operator, Gerrit MCP Server, External DNS, OAuth2 Proxy, Token Injector and several custom Post Jobs. By managing these components within a GitOps workflow, the platform ensures consistent, automated, and scalable deployments.


## GitOps deployment process

Project executes successively following files and performs operations defined inside.

1. Create `terraform.tfvars` file at path `terraform/env/` if it does not exist already by copying `terraform/env/terraform.tfvars.sample` file.
2. Update `terraform.tfvars` with actual configuration values.
3. Execute file `tools/scripts/deployment/deploy.sh`:
    1. Check if the tools with required versions have been installed.
    2. Create `terraform.tfvars` file if missing. It is required to update this file with actual configuration values.
    3. Check if authenticate to Google Cloud Platform via `gcloud` CLI.
    4. Initialize Terraform.
    5. Execute `terraform apply` or `terraform destroy` based on supplied argument.
4. The Terraform module `terraform/modules/sdv-container-images`
    1. dockerize scripts that are stored in `terraform/modules/sdv-container-images/images` path.
5. The Terraform module `terraform/modules/sdv-gke-apps`
    1. Deploy Argo CD and External Secrets via Helm.
    2. Create required namespaces, secrets, service accounts, external secrets, secret stores.
    3. Create required Argo CD Application Project and create `horizon-sdv` Argo CD Application.
6. The project uses Helm to manage Kubernetes configurations. Source files are stored in `gitops/`:
    1. file `Chart.yaml` defines the Helm chart (name, version etc.),
    2. file `values.yaml` contains default configuration values,
    3. `gitops/templates` - contains Kubernetes resource definitions (Deployments, Jobs, Service Accounts, etc.).

### Input parameters

To start GitOps deployment process it is required to provide list of configure parameters. They are used to create applications in the Horizon SDV platform. These parameters are provided as environment variables. List of input configuration parameters is provided below:

- GITHUB_REPO_NAME (repository name, without https://github.com prefix)
- GITHUB_REPO_BRANCH_NAME (repository branch to be used for deployment)
- GCP_PROJECT_ID (GCP Project ID)
- GCP_CLOUD_REGION (GCP Cloud Region)
- GCP_CLOUD_ZONE (GCP Cloud Zone)
- GCP_BACKEND_BUCKET_NAME (GCP Bucket used to store tfstate)
- GITHUB_ENV_NAME (Environment name, also used as a subdomain)
- GITHUB_DOMAIN_NAME (top level domain name)


## ArgoCD overview

#### URL
https://<ENV_NAME>.<HORIZON_DOMAIN>/argocd
Ex: https://demo.horizon-sdv.com/argocd

ArgoCD ensures consistency between the source code and the current state of applications deployed in Kubernetes. It continuously connects to the git repository to monitor application states and detect any discrepancies. The source code for ArgoCD is provided in the form of YAML files, which define various Kubernetes resources, including fundamental objects such as Namespaces, Deployments, and Services, as well as Custom Resource Definitions (CRDs) and Helm charts. These Helm charts can be referenced either in their source form (git repository) or as pre-packaged Helm releases.

### ArgoCD sync waves

Additionally, ArgoCD utilizes sync waves, a feature that allows defining the order in which resources are deployed within Kubernetes. This ensures that dependencies are installed in the correct sequence, preventing issues related to resource availability during the deployment process.

| Sync-wave | 0          | 1                | 2                        | 3                        | 4                       | 5                     | 6                   | 7         | 8                    |
|-----------|------------|------------------|--------------------------|--------------------------|-------------------------|-----------------------|---------------------|-----------|----------------------|
|           | Namespaces | GKE Gateway      | Aplications              | Storage Classes          | Service Accounts        | HTTP Routes           | Jenkins Application | Post Jobs | MTK Connect Cron Job |
|           |            | Service Accounts | GCPGatewayPolicy         | Roles                    | Gerrit Cluster          | Health Check Policies |                     |           |                      |
|           |            | Secret Stores    | Storage Classes          | Persistent Volume Claims | Role Bindings           | GCP Backend Policies  |                     |           |                      |
|           |            | Init Secrets     | Persistent Volume Claims | Service Accounts         | Gerrit Application      | Post Jobs             |                     |           |                      |
|           |            |                  |                          | Cluster Roles            | Post Jobs               |                       |                     |           |                      |
|           |            |                  |                          | Cluster Role Bindings    | MTK Connect Application |                       |                     |           |                      |
|           |            |                  |                          | Keycloak Aplication      | Headlamp Application    |                       |                     |           |                      |


## Applications

### Landing Page

#### URL
https://<ENV_NAME>.<HORIZON_DOMAIN>
Ex: https://demo.horizon-sdv.com

#### Purpose
The Landing Page provides a simple and clear home page for the Horizon SDV project.

#### Installation
It is a static web application fully managed within the Horizon SDV project. The installation involves setting up the necessary Kubernetes resources, including Namespace, Deployment, and Service, to run the application.

#### Configuration
No additional configuration or integration with other applications is required.


### Keycloak

#### URL
https://<ENV_NAME>.<HORIZON_DOMAIN>/auth/admin/horizon/console
Ex: https://demo.horizon-sdv.com/auth/admin/horizon/console

#### Purpose
Keycloak is responsible for aggregating and unifying authentication across all applications within Horizon SDV. It also supports authentication delegation to external Identity Providers, with Google Identity used for this purpose in Horizon SDV.

#### Installation
Keycloak is deployed using its official Helm chart, with an initial configuration provided during installation. This setup is later extended through both automated and manual configuration steps.

#### Configuration
1. Automated configuration
    - Managed by post-jobs, including:
        - `keycloak-post` – Initial setup of the Horizon realm.
        - `keycloak-post-apps` – Configures authentication for Gerrit (OpenID), Jenkins (OpenID), and MTK Connect (SAML).
    - Realm and User Setup:
        - New realm: Horizon
        - Master realm admin: admin
        - Horizon realm admin: horizon-admin
        - Clients:
            - Gerrit (OpenID)
            - Jenkins (OpenID)
            - MTK Connect (SAML)
        - Users:
            - horizon-admin (realm administrator)
            - gerrit-admin (service account for Gerrit)

2. Manual additional  configuration
    - Identity provider delegation for Google Identity.
    - Restricting access to Horizon SDV to manually added users.
    - Assigning realm-admin privileges to specific users.


### Jenkins

#### URL
https://<ENV_NAME>.<HORIZON_DOMAIN>/jenkins
Ex: https://demo.horizon-sdv.com/jenkins

####  Purpose
Jenkins provides a CI/CD pipeline execution environment for workloads, currently supporting Android and Cloud Workstations workloads.

####  Installation
Jenkins is installed using the official OpenSource Helm chart, with custom configurations specific to the Horizon SDV project.

####  Configuration
Jenkins is configured using jenkins-init.yaml, jenkins.yaml, and values-jenkins.yaml, which define:
- Secrets management for applications.
- Persistent storage setup.
- Base Jenkins configuration.
- Installation and setup of required plugins.
- Reference Android workloads by linking to the corresponding Jenkinsfile in the repository.


### Gerrit

#### URL
https://<ENV_NAME>.<HORIZON_DOMAIN>/gerrit
Ex: https://demo.horizon-sdv.com/gerrit

#### Purpose
Gerrit provides a local git repository management system to optimize interactions between the CI/CD system and repositories. It also maintains a workflow similar to the one used in Android development.

#### Installation

- Gerrit is deployed using Gerrit Operator, which simplifies installation and configuration.
- Gerrit Operator and Gerrit are part of the k8g-gerrit OpenSource project.
- During installation, an initial configuration is applied, followed by two post-jobs:
    - `keycloak-post-gerrit` – Creates the gerrit-admin account.
    - `gerrit-post` – Uses this account to perform the initial Gerrit setup.

#### Configuration
Details can be verified by reviewing the `gerrit-post` job.


### MTK Connect

#### URL
https://<ENV_NAME>.<HORIZON_DOMAIN>/mtk-connect
Ex: https://demo.horizon-sdv.com/mtk-connect

#### Purpose
MTK Connect enables remote connections to both physical and virtual hardware using various communication protocols and a dedicated agent. It is an Accenture product and is not directly part of the Horizon SDV platform but is provided as a set of prebuilt container images.

#### Installation
MTK Connect is deployed by configuring and running its container images, which include:

- router
- authenticator
- wamprouter
- devices
- portal
- installers
- docs

Additionally, MongoDB is installed as a dependency.

#### Configuration
To enable authentication via Keycloak, the `keycloak-post-mtk-connect` post-job is executed, integrating Keycloak with MTK Connect using SAML authentication.


### Headlamp

#### URL
https://<ENV_NAME>.<HORIZON_DOMAIN>/headlamp/
Ex: https://demo.horizon-sdv.com/headlamp/

#### Purpose
Headlamp is a user-friendly Kubernetes dashboard that provides a interface for interacting with Kubernetes clusters. The Headlamp is a web-based user interface (GUI) for simplify the management and visualization of Kubernetes clusters.
It is particularly useful for developers, DevOps engineers to manage Kubernetes resources without relying solely on the command line in more intuitive way. Link : https://headlamp.dev/

#### Installation
Headlamp is deployed by helm chart. During installation, an initial configuration is applied. Additionally kubescape is installed as a dependency.

#### Configuration
To Enable authentication via Keycloak (SSO), the `keycloak-post-headlamp` post-job is executed, creates and configures required client, client scopes, user groups on Keycloak to enable SSO authentication. This is enabled by using OAuth2 Proxy and a custom Nginx based token injector solution.


### Gerrit MCP Server

#### URL
This app is not exposed via a public URL.

#### Purpose
Gerrit MCP Server is used to facilitate communication between AI tools and the Gerrit code review system. It provides a standardized API interface for AI tools to perform operations such as code reviews, submissions, and other interactions with Gerrit.

#### Installation
Gerrit MCP Server is built from [source code](https://gerrit.googlesource.com/gerrit-mcp-server) during platform infra deployment using `terraform/modules/sdv-container-images` module and deployed as an application using a custom Helm chart config `gitops/templates/gerrit-mcp-server.yaml` and `gitops/apps/gerrit-mcp-server`, created for Horizon SDV project.

#### Configuration
No additional configuration or integration with other applications is required.

Note that Gerrit MCP Server depends on Gerrit being installed and configured. Also, since it does not have any authentication mechanism of its own, it relies on MCP Gateway Registry's authentication and authorization system to control access.


### MCP Gateway Registry

#### URL
https://mcp.<ENV_NAME>.<HORIZON_DOMAIN>

Ex: https://mcp.demo.horizon-sdv.com

#### Purpose
MCP Gateway Registry is a centralized application for managing, monitoring and authenticating to MCP (Model Context Protocol) servers and agents deployed in Horizon platform or other environments.

#### Installation
MCP Gateway Registry is deployed using official prebuilt [container images](https://hub.docker.com/u/mcpgateway), with a custom helm chart config `gitops/templates/mcp-gateway-registry.yaml` and `gitops/apps/mcp-gateway-registry`, created for Horizon SDV project.

It uses the folowing container images:
- mcpgateway/registry:v1.6.0
- mcpgateway/auth-server:v1.6.0
- mcpgateway/mcpgw-server:v1.6.0

During installation, an initial configuration is applied to setup required resources using `gitops/templates/mcp-gateway-registry-init.yaml`.

#### Configuration
To enable authentication via Keycloak, the `keycloak-post-mcp-gateway-registry` post-job is executed, which creates and configures the necessary clients, admin user, groups, and client mappers in Keycloak. It also generates and updates the required client secrets in Kubernetes for secure communication.


## Dependencies

### Dynamic PVC Provisioner and Releaser
Ensures persistent storage remains available even after a pod is terminated. When a pod is restarted, it can reattach the storage, optimizing resource utilization. The primary goal is to reuse storage for Android builds, speeding up the build process by avoiding redundant steps like repository cloning and enabling incremental builds.

### PostgreSQL
A direct dependency for Keycloak, serving as the SQL database that stores all internal Keycloak data.

### Zookeeper
A direct dependency for Gerrit, acting as a key-value store that maintains RefDB information for Gerrit.

### MongoDB
A direct dependency for MTK Connect, functioning as a NoSQL database that stores all internal MTK Connect data.

### Gerrit Operator
A management tool designed to simplify the installation and configuration of Gerrit.

### External Secrets
While not directly visible in ArgoCD, External Secrets plays a crucial role in synchronizing secrets between GCP Secret Manager and Kubernetes Secrets.

### External DNS
Creates, configures and manages DNS records for GCP Cloud DNS Zone based on the set hostname values for gateway-httproute resources.

### OAuth Proxy
Headlamp is dependant on headlamp-oauth2-proxy for enabling Keycloak based authentication.

### Token Injector
Headlamp is dependant on headlamp-token-injector for fetching and injecting service account token into authorization header to enable Kubernetes API access to fetch and display required data on the headlamp UI.

### Post Jobs
A collection of scripts that handle application-specific configurations when standard methods are insufficient. They also ensure seamless integration between applications.

#### List of Post Jobs:
- **keycloak-post** – Initializes the Horizon realm in Keycloak, setting up the foundational authentication configuration.
- **keycloak-post-jenkins** – Configures Jenkins authentication with Keycloak by generating and updating the necessary secret in Kubernetes for secure communication.
- **keycloak-post-gerrit** – Prepares a gerrit-admin service account in Keycloak for Gerrit authentication.
- **keycloak-post-mtk-connect** – Integrates Keycloak with MTK Connect using SAML for centralized authentication.
- **keycloak-post-mcp-gateway-registry** – Configures authentication with Keycloak by creating and updating the necessary clients, admin user, groups and client mappers in Keycloak. Then generating and updating the necessary client secrets in Kubernetes for secure communication.
- **mtk-connect-post** – Configures MTK Connect after installation, ensuring it is properly set up for use.
- **mtk-connect-post-key** – Generates and configures necessary API keys for MTK Connect.
- **gerrit-post** – Uses the gerrit-admin account to perform the initial setup and configuration of Gerrit.
