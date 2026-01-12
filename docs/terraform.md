
# Terraform

## Table of contents
- [Introduction](#Introduction)
- [Overview](#Overview)
- [Modules Overview](#ModulesOverview)
- [Modules Decription](#ModulesDecription)
- [Module - env](#Module-env)
- [Module - base](#Module-base)
- [Module - sdv-apis](#Module-sdv-apis)
- [Module - sdv-artifact-registry](#Module-sdv-artifact-registry)
- [Module - sdv-certificate-manager](#[Module-sdv-certificate-manager)
- [Module - sdv-gcs](#Module-sdv-gcs)
- [Module - sdv-gke-cluster](#Module-sdv-gke-cluster)
- [Module - sdv-iam](#Module-sdv-iam)
- [Module - sdv-network](#Module-sdv-network)
- [Module - sdv-sa-key-secret](#Module-sdv-sa-key-secret)
- [Module - sdv-secrets](#Module-sdv-secrets)
- [Module - sdv-ssl-policy](#Module-sdv-ssl-policy)
- [Module - sdv-wi](#Module-sdv-wi)
- [Execute terraform scripts](#Executeterraformscripts)




 ## Introduction <a name="Introduction"></a>

Terraform is an open-source tool developed by HashiCorp that allows you to define and provision infrastructure using a high-level configuration language. 
It allows managing infrastructure as code, which means it's possible to write, put it under version control, and share the infrastructure configuration.
In Horizon platfrom terraform is used to create the infrastucture in GCP (Google Cloud Platform).

## Overview 

Teraform scripts in Horizon SDV project define and create GCP infrastructure and configure services using terraform code.
Terraform use declarative language to describe the desired state of infrastructure, and takes care of provisioning and managing the resources to match that state.
GCP is used as a cloud platform for creating and configuring infrastructure, hosts, and other services that are needed by Horizon SDV platform. Google Cloud is also used in the Horizon SDV project for managing secrets and running scripts that interact with the Standard GKE cluster through Fleet. 
Main list of resources created by terraform scripts in the GCP Cloud is mentioned below:

- GKE Cluster (Google Kubernetes Engine Cluster) with 2 node pool:
  - sdv-node-pool - used for Horizon SDV services
  - sdv-build-node-pool - used for workloads
- Main Horizon SDV Service Account, required secrets and other Service Accounts for GKE (Google Kubernetes Engine)
- Artifact Registry to store, docker images for services eg. Landing Paga, Post Jobs, Cron Jobs and AAOS Builder
- Certificate Manager, Certificate Manager Map and DNS authorization resources - for managing the TLS certificate
- GCS (Google Cloud Storage) - storage bucket that stores Android build output results, infastructure state and deployment helper script
- IAM (Identity and Access Management) area that helps managing IAM roles for users and Service Accounts
- VPC (Virtual Private Cloud) - that configures the Horizon SDV plaform networking
- Secret Manager - stores all Horizon SDV platform required secrets, most of them are then bridged to the inside if GKE Kubernetes Cluster

Terraform implementation in the Horizon SDV poject repository is organized into following subdirectories:

- modules - implementation of the terraform modules
- env - stores all environmetn specific configuration options (most of them are needed to be provided up front with either `local-env.sh` script or with GitHub Workflows execution pipeline)


## Modules Overview

Main entry point for terraform execution is `env/main.tf` file. This file contains all input configuration parameters that are needed to be provided before execution. List of input configuration parameters is provided in the `local-env.sh` file which can be modified and sourced if there is a need of running terraform manually. If GitHub Workflows are used - all these input variables are provided automatically.

- sdv_github_app_id (Github Application ID)
- sdv_github_app_install_id (GitHub Installation ID)
- sdv_github_app_private_key (GitHub Application Private Key)
- sdv_github_app_private_key_pkcs8 (GitHub Application Private Key in PKCS#8 format)
- github_auth_method (Authentication method either GitHub app or pat)
- sdv_github_pat (GitHub Personal Access Token)
- sdv_jenkins_admin_password (Jenkins initial admin account password)
- sdv_keycloak_admin_password (Keycloak initial admin account password)
- sdv_gerrit_admin_password (Gerrit initial admin accont password)
- sdv_gerrit_ssh_private_key (Gerrit initial admin SSH private key)
- sdv_keycloak_horizon_admin_password (Keycloak initial horizon realm admin account password)
- sdv_cuttlefish_ssh_private_key (GCE SSH access to Cuttlefish VMs private key)
- sdv_github_repo_name (Repository Name)
- sdv_github_repo_owner (Repository Owner: GitHub Organization name or GitHub user name who owns the GitHub repo)
- sdv_env_name (Environment and SubDomain name)
- sdv_root_domain (Top level Domain Name)
- sdv_gcp_project_id (GCP Project ID)
- sdv_gcp_compute_sa_email (Main GCE Computer Service Account)
- sdv_gcp_region (GCP Cloud Region)
- sdv_gcp_zone (GCP Cloud Zone)
- sdv_gcp_backend_bucket (GCP Backend Bucket to store tfstate)
- enable_arm64 (Toggle to enable or disable ARM64 support)
- manual_secrets (Set Application Admin secret manually)



Each module directory should contain files eg:
- `main.tf` - main terraform implementation file
- `variables.tf` - variables definition
- `output.tf` - (optional) - output variable definition 

env/main.tf file include all modules by its dependencies:

1. `base` - Define all list of needed modules. Each module defines source path, dependency and needed data eg. resource name, project_id, network data etc. 
2. `sdv-apis` - Defines list of google APIs to include. APIs is needed to most implementation modules.
3. `sdv-artifact-registry` - Defines restistry and roles for Artifacts Registry resource. Registry is an universal package manager for build artifacts and dependencies.
4. `sdv-certificate-manager` - Define certificate manager maps and DNS authorization for specific domain.
5. `sdv-container-images` - Build and push container images to Artifact Registry.
6. `sdv-dns-zone` - Create and manage Cloud DNS Zone for the environment. Manages DNS records within it.
7. `sdv-gcs` - Creates Google Cloud Storage and Storage Bucket for the project.
8. `sdv-gke-apps` - Module which deployes essential apps once the Standard GKE cluster has been provisioned.
9. `sdv-gke-cluster` - Defines Google Kubernetes Engine Cluster for project with proper configuration and properties.
10. `sdv-iam` - Configures IAM roles for users and Service Accounts.
11. `sdv-network` - Configures networking including, subnets, IP address ranges and filrewall.
12. `sdv-parameters` - Manages configuration data in Google Parameter Manager. Store non-sensitive environment data.
13. `sdv-sa-key-secret` - Creates a JSON Key from the defined SA and saves it as a GCP Secret. Gives access to the defined GKE Service Account to the created secret.
14. `sdv-secrets` - Creates required secrets and gives the access to the defined Kubernetes Service Accounts.
15. `sdv-ssh-keypair` - Module to Generate SSH keys and saves them to local files.
16. `sdv-ssl-policy` - Creates a SSL Policy. SSL policies specify the set of SSL features that GCP load balancers use when negotiating SSL with clients. 
17. `sdv-wi` - module creates GCP Service Accounts which are going to be used in various parts of the Horizon SDV project ensuring a trust relationship between them. It helps using these account without setting any additional authentication methods like passwords. Also assigns required roles to these Service Accounts.

## Modules Description

Implementation consist of several modules responsible for particular feature or GCP service. List of modules:

- base
- sdv-apis
- sdv-artifact-registry   
- sdv-certificate-manager
- sdv-container-images
- sdv-dns-zone
- sdv-gcs 
- sdv-gke-apps
- sdv-gke-cluster
- sdv-iam
- sdv-network
- sdv-parameters
- sdv-sa-key-secret
- sdv-secrets
- sdv-ssh-keypair
- sdv-ssl-policy
- sdv-wi


## Module - env
Contains main configuration file , which contains GCP project details such as  project ID, region, zone, network etc. Set up service accounts and required secrets.
- The configuration uses a module 'base' sourced from ../modules/base and sets up various parameters such as project ID, region, zone, network, and subnetwork.
- Defines a list of GCP APIs to be enabled, including Compute, DNS, Monitoring, Secret Manager, IAM, and more.
- It sets up service accounts for many purposes, such as Jenkins, ArgoCD, Keycloak, and Gerrit, with specific roles and permissions.
- Defines a cluster named sdv-cluster with a node pool named sdv-node-pool. Additional configurations are also provided for the build node pool and Fleet-based cluster access.
- Configuration includes a map of secrets with their IDs, values, and access rules for different GKE (Google Kubernetes Engine) namespaces and service accounts. Secrets include GitHub App ID, installation ID, private key, initial passwords for ArgoCD, Jenkins, Keycloak, etc.
 
## Module - base
Main configuration file for the "base" module. Configure and set data to for other modules to provision various resources.
Module `base` is responsible to set and config following parts:

- Modules - The configuration uses multiple modules eg ../sdv-apis, ../sdv-secrets, ../sdv-wi, ../sdv-gcs, ../sdv-network, etc. Each module is responsible for specific tasks such as managing APIs, secrets, parameters service accounts, GCS buckets, network configurations, DNS Zone management, GKE cluster setup, artifact registry, Container images, certificate management, SSL policy, and IAM roles.
- Service Accounts and IAM Roles - Sets up IAM roles for the service account ${var.sdv_gcp_compute_sa_email} including roles/storage.objectUser, roles/compute.instanceAdmin.v1, roles/compute.networkAdmin, roles/iap.tunnelResourceAccessor, and roles/iam.serviceAccountUser.
- GKE Cluster Configuration - It defines a GKE cluster with a default node pool and a build node pool. The node pools sets specific configurations for machine types, node counts, and locations.
- Secrets Management - The configuration includes a module for managing secrets with a map of secrets and their access rules for different GKE namespaces and service accounts.
- Network configuration - Sets up a network and subnetwork with a router for network egress. A custom VPN firewall rule is defined to allow TCP port 22 for the service account ${var.sdv_gcp_compute_sa_email}.
- Artifact Registry and Certificate Management - The configuration includes modules for setting up an artifact registry and managing SSL certificates with specific parameters like sdv_artifact_registry_repository_id, location, ssl_certificate_name or domain_name.

## Module - sdv-apis
This module enables the specified Google Cloud APIs from a provided list. List of APIs to set is defined in module `env`. This allows management of a each API service for the project.

## Module - sdv-artifact-registry
Creates Google Artifact Registry repository for docker repository for Horizon SDV. Assign memebers for role registry_writer or registry_reader with required IAM resources.

## Module - sdv-certificate-manager
- Certificate Manager Certificate - The configuration creates a google_certificate_manager_certificate resource named horizon_sdv_cert. It specifies the project ID, certificate name, and scope as "DEFAULT". The certificate is managed with domains and DNS authorizations provided by the google_certificate_manager_dns_authorization resource.
- DNS Authorization - A google_certificate_manager_dns_authorization resource named instance is created. It specifies the name as "horizon-sdv-dns-auth" and the domain from the variable var.domain.
- Certificate Map - The configuration creates a google_certificate_manager_certificate_map resource named horizon_sdv_map. It includes the project ID, map name, and a description "Certificate Manager Map for Horizon SDV".
- Certificate Map Entry- A google_certificate_manager_certificate_map_entry resource named horizon_sdv_map_entry is created. It specifies the map entry name, description, map name, certificates, and matcher as "PRIMARY".

## Module - sdv-container-images
Builds and pushes the required container images to Google Artifact Registry. Detects changes in container build files within `images/`, rebuilds and pushes images to Google Artifact Registry.

## Module - sdv-dns-zone
Creates and configures Google Cloud DNS Zone. Manages DNS records within the Cloud DNS Zone. Creates Google certificate manager certificate CNAME record required for DNS Authz.

## Module - sdv-gcs
Creates Google Cloud Storage (GCS) Bucket. Uniform bucket-level option control access to your Cloud Storage resources. When enabled, Access Control Lists (ACLs) are disabled, and only bucket-level Identity and Access Management (IAM) permissions grant access to that bucket and the objects it contains.

## Module - sdv-gke-apps
Deploys and configures required Kubernetes resources post Standard GKE Cluster creation.
- Deploy External Secrets and Argo CD via Helm.
- Create required Service accounts, Secrets and Secret Stores.
- Create Argo CD App Project and Argo CD Application.

## Module - sdv-gke-cluster
Creates and manages a Google Kubernetes Engine (GKE) cluster along with its node pools.
This terraform configuration sets up a GKE cluster with specific configurations for network, security, maintenance, and add-ons, along with two node pools (main and build) with their respective configurations.

Resource "google_container_cluster" "sdv_cluster" defines a GKE cluster with various configurations:
- Project and Location: Specifies the project ID, cluster name, location, network, and subnetwork.
- Node Pool Management: Removes the default node pool and prepares to create 2 main Horizon SDV node pools.
- Workload Identity: Enables Workload Identity for the cluster.
- Network Configuration: Disables public CIDR access and configures IP allocation policies.
- Private Cluster: Enables private nodes and private endpoint with a specified master IPv4 CIDR block.
- Secret Manager: Enables Secret Manager integration.
- Maintenance Policy: Defines a recurring maintenance window (only days: Sat and Sun).
- Gateway API: Enables the Gateway API with the standard channel.
- Add-ons: Enales the Load Balancing feature and Filestore CSI driver.
- Autoscaling is disabled.

Resource "google_container_node_pool" "sdv_main_node_pool", "google_container_node_pool" "sdv_build_node_pool" and "google_container_node_pool" "sdv_openbsw_build_node_pool" define a main node pool, Android and OpenBSW build node pools for the GKE cluster with the configurations:
- Node Pool Details: configures the name, location, cluster, node count, and node locations.
- Node Configuration: specifies the machine type, service account, OAuth scopes, and workload metadata.
- Autoscaling: Configures autoscaling with minimum and maximum node counts (for sdv_build_node_pool qnd sdv_openbsw_node_pool).

## Module - sdv-iam
Module updates IAM policy to grant a role to a member or Service Account.

## Module - sdv-network
Module creates and manages a Virtual Private Cloud (VPC) network in GCP. Sets project ID, network name, and routing mode for the VPC.
Defines a subnet within the VPC with the following configurations:
- Private IP Google Access: Disables private IP Google access.

Module defines secondary IP ranges (secondary_ranges) for the subnet, which are used to differentiate GKE internal resources within the Cluster such as pod ranges and services ranges.
Module configure also route within the VPC with the following configurations:
- Route Name and Description: Specifies the route name and description.
- Destination Range: Sets the destination range for the route (any IP address)
- Sets the next hop to the internet gateway (IGW).
- Enables private IP Google access.

## Module - sdv-parameters
Manages configuration data in Google Parameter Manager. It acts as a store for non-sensitive environment variables like environment name, domain name, GCP Project ID, GCP Region, etc. 

## Module - sdv-sa-key-secret
Creates JSON service account key and enable access for GKE cluster.
Defines replication policy of secret attached to the Secret.

## Module - sdv-secrets
Module manages secrets in Google Cloud Secret Manager by creating secrets, their versions, and setting IAM bindings for access control.The secrets are replicated to location. Resource "google_secret_manager_secret_version" "sdv_gsmsv_use_github_value"  creates secret versions for secrets that use GitHub values. It ignores changes to the secret data and depends on the creation of the secret resource. Resource "google_secret_manager_secret_version" "sdv_gsmsv_dont_use_github_value" creates secret versions for secrets that do not use GitHub values. It depends on the creation of both the secret resource and the secret versions that use GitHub values. 'secret_iam_binding his' sets IAM bindings for each secret, granting the roles/secretmanager.secretAccessor role to specified members.

## Module - sdv-ssh-keypair
Module to Generate SSH keys required for Gerrit and Cuttlefish VMs. Converts the generated keys into required format and saves them to local files with required file system permissions.

## Module - sdv-ssl-policy
Module creates SSL policy to be used by the cluster. Profile and name is set.

## Module - sdv-wi
Module Workload Identity is used to manage Google Cloud Service Accounts and their roles. Key components:
- google_service_account.sdv_wi_sa: Creates service accounts for each entry in var.wi_service_accounts.
- flattened_roles_with_sa and flattened_gke_sas: Flattens the roles associated with each service account into a list.
- roles_with_sa_map and gke_sas_with_sa_map: Maps each role-service account combination to a unique key.
- google_project_iam_member.sdv_wi_sa_iam_2 and sdv_wi_sa_wi_users_gke_ns_sa: Assigns roles to service accounts based on roles_with_sa_map.
In case of GKE assigns the roles/iam.workloadIdentityUser role to GKE service accounts based on gke_sas_with_sa_map.


## Execute terraform scripts
The automated deployment scripts are located in `tools/scripts/deployment/`. These scripts standardize the deployment of the Horizon SDV platform infrastructure to the required GCP Project.

### What the script does
1. Requirements Check
2. Authentication Check
3. Terraform Initialization
4. Execution (Terraform apply or destroy based on supplied flag)

**Path**: `tools/scripts/deployment/`
**Main deployment script**: `deploy.sh`
**Container wrapper**: `container-deploy.sh` (Requires Docker to be installed on the host machine)

### Execution methods
There are two ways to run the deployment. Via Docker and Host Machine.

#### Docker
This runs the deployment script within a containerized environment. You do not need to install Terraform, Kubectl, or the specific Gcloud CLI version on your local machine, only Docker is required.

**Script Path**: `/tools/scripts/deployment/`
**Dockerfile Path**: `/tools/scripts/deployment/container/`
**Command**: 
```shell
./container-deploy.sh [args]
```

1. Builds the horizon-sdv-deployer Docker container image with pinned tool versions (Terraform, Kubectl, gcloud).
2. Creates a persistent Docker Volume to store `gcloud` login credentials.
3. Mounts current code (repository) directory into the container.
4. Executes `deploy.sh` within the container.

**First Run**: You will be prompted to authenticate by clicking/copying a link and pasting a code. Credentials are **saved to the volume**.

#### Host Machine
Runs the script directly on host machine. Ensure all the tools with the required versions are installed.

**Prerequisites**:
- Terraform `v1.14.2+`
- Kubectl `v1.34.3+`
- Google Cloud CLI `v549.0.1+`
- Authenticated gcloud session (gcloud auth login)

**Script Path**: `/tools/scripts/deployment`
**Command**: 
```shell
./deploy.sh [args]
```

### Arguments usage
Both scripts (Containerized and Host Machine) accept the same arguments, which are passed directly to Terraform.

#### Deploy (Apply):
- Initializes Terraform and runs `terraform apply -auto-approve`.   

**Containerized**:
```shell
./container-deploy.sh
```

**Host Machine**:
```shell
./deploy.sh
```

### Destroy (Teardown):
Initializes Terraform and runs `terraform destroy -auto-approve`.

**Containerized**:
```shell
./container-deploy.sh -d
```

**Host Machine**:
```shell
./deploy.sh -d
```
> Note: You can also use `--destroy`.

For more details about Terraform please reach the official documentation: https://cloud.google.com/docs/terraform/terraform-overview
