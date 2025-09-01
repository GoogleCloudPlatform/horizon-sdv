# Gitops documentation

## Table of contents
- [GitOps overview](#gitops-overview)
- [GitOps in Horizon SDV project](#gitops-in-horizon-sdv-project)
- [GitOps deployment process](#gitops-deployment-process)
- [ArgoCD overview](#argocd-overview)
- [Aplications](#aplications)
    - [Keycloak](#keycloak)
    - [Jenkins](#jenkins)
    - [Gerrit](#gerrit)
    - [MTK Connect](#mtk-connect)
    - [Headlamp](#headlamp)
    - [Landing Page](#landing-page)
- [Dependencies](#dependencies)
    - [Dynamic PVC Provisioner and Releaser](#dynamic-pvc-provisioner-and-releaser)
    - [PostgreSQL](#postgresql)
    - [Zookeeper](#zookeeper)
    - [MongoDB](#mongodb)
    - [Gerrit Operator](#gerrit-operator)
    - [External Secrets](#external-secrets)
    - [Post Jobs](#post-jobs)

## GitOps overview

GitOps is a deployment approach that uses git as the source of truth for infrastructure and application configurations. Changes are made through Git, and tools like ArgoCD automatically apply them to Kubernetes clusters, ensuring consistency between the repository and the running environment. This allows for automated, version-controlled deployments without manual intervention.


## GitOps in Horizon SDV project

In the Horizon SDV platform, GitOps is used to manage applications and their dependencies using ArgoCD. The platform includes applications such as Keycloak, Gerrit, Jenkins, MTK Connect, and LandingPage, along with dependencies like Dynamic PVC Provisioner and Releaser, PostgreSQL, Zookeeper, MongoDB, Gerrit Operator, and several custom Post Jobs. By managing these components within a GitOps workflow, the platform ensures consistent, automated, and scalable deployments.


## GitOps deployment process

Project executes successively following files and performs operations defined inside.

1. Execute file `terraform/bash-scripts/stage1.sh`:
    1. perform initial operations done by Terraform,
    2. clone git repository and checkout on branch pointed by variable `GITHUB_ENV_NAME`.
2. Execute `gitops/env/stage2/configs/build.sh`:
    1. dockerize scripts that are stored in `gitops/env/stage2/configs` path.
3. Execute `gitops/env/stage1/deploy.sh`:
    1. add the ArgoCD and External Secrets Helm repositories to Helm and update the local repository index to fetch the latest chart versions,
    2. update variables values in `argocd-secrets.yaml`, `argocd-values.yaml`, `argocd-config.yaml` files.
4. Apply kubernetes resource in file `argocd-secrets.yaml`:
    1. configure basic secrets configuration in `argocd` namespace.
5. Apply kubernetes resource in file `argocd-config.yaml`:
    1. define configuration of ArgoCD aplication,
    2. ArgoCD looks in path `gitops/env/stage2` for Kubernetes manifests,
    3. if aplication sync fails, system retries to sync up to 5 times.
6. The project uses Helm to manage Kubernetes configurations. Source files are stored in `gitops/env/stage2`:
    1. file `Chart.yaml` defines the Helm chart (name, version etc.),
    2. file `values.yaml` contains default configuration values,
    3. `gitops/env/stage2/templates` - contains Kubernetes resource definitions (Deployments, Jobs, Service Accounts, etc.).


### Input parameters

To start GitOps deployment process it is required to provide list of configure parameters. They are used to create applications in the Horizon SDV platform. These parameters are provided as environment variables. List of input configuration parameters is provided below:

- GITHUB_REPO_NAME (repository name, without https://github.com prefix)
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


## Aplications

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
Jenkins is configured using jenkins-init.yaml and jenkins.yaml, which define:
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
To authenticate to headlamp is needed token. Token should be generated via bastion host :
`kubectl create token headlamp-admin -n headlamp` 

Then just copy generated token and paste it on headlamp login sceen. Access should be possible. Token is valid 1h for first time login.
SSO autentication is planned in future releases.

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

### Post Jobs
A collection of scripts that handle application-specific configurations when standard methods are insufficient. They also ensure seamless integration between applications.

#### List of Post Jobs:
- **keycloak-post** – Initializes the Horizon realm in Keycloak, setting up the foundational authentication configuration.
- **keycloak-post-jenkins** – Configures Jenkins authentication with Keycloak by generating and updating the necessary secret in Kubernetes for secure communication.
- **keycloak-post-gerrit** – Prepares a gerrit-admin service account in Keycloak for Gerrit authentication.
- **keycloak-post-mtk-connect** – Integrates Keycloak with MTK Connect using SAML for centralized authentication.
- **mtk-connect-post** – Configures MTK Connect after installation, ensuring it is properly set up for use.
- **mtk-connect-post-key** – Generates and configures necessary API keys for MTK Connect.
- **gerrit-post** – Uses the gerrit-admin account to perform the initial setup and configuration of Gerrit.