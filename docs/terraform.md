
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
- [Module - sdv-bash-on-bastion-host](#Module-sdv-bash-on-bastion-host)
- [Module - sdv-bastion-host](#Module-sdv-bastion-host)
- [Module - sdv-certificate-manager](#[Module-sdv-certificate-manager)
- [Module - sdv-copy-to-bastion-host](#Module-sdv-copy-to-bastion-host)
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
GCP is used as a cloud platform for creating and configuring infrastructure, hosts, and other services that are needed by Horizon SDV platform. Google Cloud is also used in the Horizon SDV project for managing secrets and running scripts on the bastion host (described in details below). 
Main list of resources created by terraform scripts in the GCP Cloud is mentioned below:

- GKE Cluster (Google Kubernetes Engine Cluster) with 2 node pool:
  - sdv-node-pool - used for Horizon SDV services
  - sdv-build-node-pool - used for workloads
- Bastion Host - special-purpose host machine designed to withstand attacks and provide secure access to the Virtual Private Cloud (VPC)
- Main Horizon SDV Service Account, required secrets and other Service Accounts for GKE (Google Kubernetes Engine)
- Artifact Registry to store, docker images for services eg. Landing Paga, Post Jobs, Cron Jobs and AAOS Builder
- Certificate Manager, Certificate Manager Map and DNS authorization resources - for managing the TLS certificate
- GCS (Google Cloud Storage) - storage bucket that stores Android build output results, infastructure state and deployment helper script
- IAM (Identity and Access Management) area that helps managing IAM roles for users and Service Accounts
- VPC (Virtual Private Cloud) - that configures the Horizon SDV plaform networking
- Secret Manager - stores all Horizon SDV platform required secrets, most of them are then bridged to the inside if GKE Kubernetes Cluster

Terraform implementation in the Horizon SDV poject repository is organized into following subdirectories:

- bash-scripts - files that are copied to the bastion host and executed
- modules - implementation of the terraform modules
- env - stores all environmetn specific configuration options (most of them are needed to be provided up front with either `local-env.sh` script or with GitHub Workflows execution pipeline)


## Modules Overview

Main entry point for terraform execution is `env/main.tf` file. This file contains all input configuration parameters that are needed to be provided before execution. List of input configuration parameters is provided in the `local-env.sh` file which can be modified and sourced if there is a need of running terraform manually. If GitHub Workflows are used - all these input variables are provided automatically.

- sdv_gh_app_id (Github Application ID)
- sdv_gh_installation_id (GitHub Installation ID)
- sdv_gh_app_key (GitHub Application Private Key)
- sdv_gh_app_key_pkcs8 (GitHub Application Private Key in PKCS#8 format)
- sdv_gh_jenkins_initial_password (Jenkins initial admin account password)
- sdv_gh_keycloak_initial_password (Keycloak initial admin account password)
- sdv_gh_gerrit_admin_initial_password (Gerrit initial admin accont password)
- sdv_gh_gerrit_admin_private_key (Gerrit initial admin SSH private key)
- sdv_gh_keycloak_horizon_admin_password (Keycloak initial horizon realm admin account password)
- sdv_gh_cuttlefish_vm_ssh_private_key (GCE SSH access to Cuttlefish VMs private key)
- sdv_gh_repo_name (Repository Name)
- sdv_gh_env_name (Environment and SubDomain name)
- sdv_gh_domain_name (Top level Domain Name)
- sdv_gcp_project_id (GCP Project ID)
- sdv_computer_sa (Main GCE Computer Service Account)
- sdv_gcp_cloud_region (GCP Cloud Region)
- sdv_gcp_cloud_zone (GCP Cloud Zone)
- sdv_gcp_backend_bucket_name (GCP Backend Bucket to store tfstate)



Each module directory should contain files eg:
- `main.tf` - main terraform implementation file
- `variables.tf` - variables definition
- `output.tf` - (optional) - output variable definition 

env/main.tf file include all modules by its dependencies:

1. `base` - Define all list of needed modules. Each module defines source path, dependency and needed data eg. resource name, project_id, network data etc. 
2. `sdv-apis` - Defines list of google APIs to include. APIs is needed to most implementation modules.
3. `sdv-artifact-registry` - Defines restistry and roles for Artifacts Registry resource. Registry is an universal package manager for build artifacts and dependencies.
4. `sdv-bash-on-bastion-host` - Executes bash script on Bastion Host. Bash script install applications like kubectl, docker, gcloud tools.
5. `sdv-bastion-host` - Provisions Bastion Host from the instance template, adds service account, assign roles and enable IAP Tunneling.
6. `sdv-certificate-manager` - Define certificate manager maps and DNS authorization for specific domain.
7. `sdv-copy-to-bastion-host` - Copies local file into GCS storage and from GCS storage to the Bastion Host.
8. `sdv-gcs` - Creates Google Cloud Storage and Storage Bucket for the project.
9. `sdv-gke-cluster` - Defines Google Kubernetes Engine Cluster for project with proper configuration and properties.
11. `sdv-iam` - Configures IAM roles for users and Service Accounts.
12. `sdv-network` - Configures networking including, subnets, IP address ranges and filrewall.
13. `sdv-sa-key-secret` - Creates a JSON Key from the defined SA and saves it as a GCP Secret. Gives access to the defined GKE Service Account to the created secret.
14. `sdv-secrets` - Creates required secrets and gives the access to the defined Kubernetes Service Accounts.
15. `sdv-ssl-policy` - Creates a SSL Policy. SSL policies specify the set of SSL features that GCP load balancers use when negotiating SSL with clients. 
16. `sdv-wi` - module creates GCP Service Accounts which are going to be used in various parts of the Horizon SDV project ensuring a trust relationship between them. It helps using these account without setting any additional authentication methods like passwords. Also assigns required roles to these Service Accounts.

## Modules Description

Implementation consist of several modules responsible for particular feature or GCP service. List of modules:

- base
- sdv-apis
- sdv-artifact-registry   
- sdv-bash-on-bastion-host
- sdv-bastion-host
- sdv-certificate-manager
- sdv-copy-to-bastion-host
- sdv-gcs 
- sdv-gke-cluster
- sdv-iam
- sdv-network
- sdv-sa-key-secret
- sdv-secrets
- sdv-ssl-policy
- sdv-wi


## Module - env
Contains main configuration file , which contains GCP project details such as  project ID, region, zone, network etc. Set up service accounts and required secrets.
- The configuration uses a module 'base' sourced from ../modules/base and sets up various parameters such as project ID, region, zone, network, and subnetwork.
- Defines a list of GCP APIs to be enabled, including Compute, DNS, Monitoring, Secret Manager, IAM, and more.
- It sets up service accounts for many purposes, such as Jenkins, ArgoCD, Keycloak, and Gerrit, with specific roles and permissions.
- Defines a cluster named sdv-cluster with a node pool named sdv-node-pool. Additional configurations for build node pool and bastion host are also provided.
- Configuration includes a map of secrets with their IDs, values, and access rules for different GKE (Google Kubernetes Engine) namespaces and service accounts. Secrets include GitHub App ID, installation ID, private key, initial passwords for ArgoCD, Jenkins, Keycloak, etc.
- Bash command is defined to export various environment variables and execute a script `stage1.sh`.
 
## Module - base
Main configuration file for the "base" module. Configure and set data to for other modules to provision various resources.
Module `base` is responsible to set and config following parts:

- Modules - The configuration uses multiple modules eg ../sdv-apis, ../sdv-secrets, ../sdv-wi, ../sdv-gcs, ../sdv-network, etc. Each module is responsible for specific tasks such as managing APIs, secrets, service accounts, GCS buckets, network configurations, bastion host setup, GKE cluster setup, artifact registry, certificate management, SSL policy, and IAM roles.
- Service Accounts and IAM Roles - Sets up IAM roles for the service account ${var.sdv_computer_sa} including roles/storage.objectUser, roles/compute.instanceAdmin.v1, roles/compute.networkAdmin, roles/iap.tunnelResourceAccessor, and roles/iam.serviceAccountUser.
- GKE Cluster Configuration - It defines a GKE cluster with a default node pool and a build node pool. The node pools sets specific configurations for machine types, node counts, and locations.
- Secrets Management - The configuration includes a module for managing secrets with a map of secrets and their access rules for different GKE namespaces and service accounts.
- Network configuration - Sets up a network and subnetwork with a router for network egress. A custom VPN firewall rule is defined to allow TCP port 22 for the service account ${var.sdv_computer_sa}.
- Bastion Host - Bastion Host is configured with specific parameters such as host name, service account, network, subnetwork, zone, and members. There is a dedicated bash command defined to export various environment variables and execute a script called `stage1.sh`.
- Artifact Registry and Certificate Management - The configuration includes modules for setting up an artifact registry and managing SSL certificates with specific parameters like sdv_artifact_registry_repository_id, location, ssl_certificate_name or domain_name.

## Module - sdv-apis
This module enables the specified Google Cloud APIs from a provided list. List of APIs to set is defined in module `env`. This allows management of a each API service for the project.

## Module - sdv-artifact-registry
Creates Google Artifact Registry repository for docker repository for Horizon SDV. Assign memebers for role registry_writer or registry_reader with required IAM resources.

## Module - sdv-bash-on-bastion-host
Main configuration file for the "sdv-bash-on-bastion-host" module. It executes a command on the cluster's bastion host once it has been provisioned successfully.
A `triggers` block ensures that the resource is always run by using the current timestamp. The provisioner block executes a series of bash commands on the bastion host.

## Module - sdv-bastion-host
This module configures Bastion Host:
- Service Account resource `vm_sa` is created for the Bastion Host with the specified Project ID and Account ID.
- Instance Template Module - The configuration uses the instance_template to create a VM instance template. It also sets up parameters such as project ID, machine type, subnetwork, service account, metadata, and source image.
- Compute Instance from Template - A google_compute_instance_from_template resource named vm is created using the instance template defined in the module. It specifies the project ID, instance name, zone, and network interface.
- IAM Bindings - Additional OS login IAM bindings are set up using google_service_account_iam_member and google_project_iam_member resources. Roles such as roles/iam.serviceAccountUser, roles/compute.osAdminLogin, and roles/container.admin are assigned to the specified members.
- IAP Tunneling Module - The configuration uses the iap_tunneling module from terraform-google-modules/bastion-host/google/modules/iap-tunneling to set up IAP tunneling. It specifies parameters such as firewall name, project ID, network, service accounts, instances, and members. IAM Roles for Bastion Host SA - IAM roles are assigned to the Bastion Host Service Account using google_project_iam_member resources.
Roles include roles/container.admin, roles/storage.objectViewer, roles/artifactregistry.writer, and roles/secretmanager.secretAccessor.

## Module - sdv-certificate-manager
- Certificate Manager Certificate - The configuration creates a google_certificate_manager_certificate resource named horizon_sdv_cert. It specifies the project ID, certificate name, and scope as "DEFAULT". The certificate is managed with domains and DNS authorizations provided by the google_certificate_manager_dns_authorization resource.
- DNS Authorization - A google_certificate_manager_dns_authorization resource named instance is created. It specifies the name as "horizon-sdv-dns-auth" and the domain from the variable var.domain.
- Certificate Map - The configuration creates a google_certificate_manager_certificate_map resource named horizon_sdv_map. It includes the project ID, map name, and a description "Certificate Manager Map for Horizon SDV".
- Certificate Map Entry- A google_certificate_manager_certificate_map_entry resource named horizon_sdv_map_entry is created. It specifies the map entry name, description, map name, certificates, and matcher as "PRIMARY".

## Module - sdv-copy-to-bastion-host
Copies required files from GCS Bucket to the Bastion Host. It uses the gcloud beta compute ssh command to SSH into a bastion host. Once connected it creates the file structure on the Bastion Host, and prepares the file for execution.

## Module - sdv-gcs
Creates Google Cloud Storage (GCS) Bucket. Uniform bucket-level option control access to your Cloud Storage resources. When enabled, Access Control Lists (ACLs) are disabled, and only bucket-level Identity and Access Management (IAM) permissions grant access to that bucket and the objects it contains.

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

## Module - sdv-sa-key-secret
Creates JSON service account key and enable access for GKE cluster.
Defines replication policy of secret attached to the Secret.

## Module - sdv-secrets
Module manages secrets in Google Cloud Secret Manager by creating secrets, their versions, and setting IAM bindings for access control.The secrets are replicated to location. Resource "google_secret_manager_secret_version" "sdv_gsmsv_use_github_value"  creates secret versions for secrets that use GitHub values. It ignores changes to the secret data and depends on the creation of the secret resource. Resource "google_secret_manager_secret_version" "sdv_gsmsv_dont_use_github_value" creates secret versions for secrets that do not use GitHub values. It depends on the creation of both the secret resource and the secret versions that use GitHub values. 'secret_iam_binding his' sets IAM bindings for each secret, granting the roles/secretmanager.secretAccessor role to specified members.

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

Terraform scripts can be executed 2 ways:
- Locally (typically used only to check changes with `terraform plan` command execution)
- As GitHub Workflows

Typical procedure using GitHub Workflow
- Create a branch
- Implement new changes
- Create Pull Request
- Check results of potential changes (eg. run `terrafrorm plan` locally) 
- Review and merging into proper branch
- Changes can be applied into GCP via GitHub Workflows.

Terraform scripts can be executed automatically via GitHub Actions or manually.
GitHub Actions help executing the deployment procedure by providing all required input parameters and tracking the execution
It is recommended to deploy the infrastructure (using terraform scripts) via GitHub Actions.
Check details in workflows documentations `github_workflows.md`.

It is possible to check or execute terraform script locally.
Applying terraform changes and deployment into GCP from local environment is not recomended, but it's possible to run `terraform init`, `terraform plan` and `terraform destroy` locally.

To run terraform in the local environment:

- `source ./local-env.sh && terraform init -backend-config bucket=bucket="prj-<ENV_NAME>-horizon-sdv-tf"` - initialize a Terraform working directory. The terraform init command is used to initialize a working directory containing Terraform configuration files. This command performs downloading and installing the necessary provider plugins, initializing the backend configuration and preparing the working directory for other Terraform commands. Note: './local-env.sh' contains defined initial values of configuration. If not given before terraform init user will be prompted for entering some value or terraform init could fail.

- `terraform plan` - It shows you a preview of the changes that will be made to your infrastructure, including resources that will be created, modified, or destroyed. This command nothing change or apply in infrastructure. Command check and compare potencial changes and will show syntax, mismatch errors etc. Terraform store current infrastructure state in bucket (defined as parameter for terraform init). This state is used for compare differences.

- `terraform apply` - Apply changes shown in terraform plan ( proper permission is needed for apllying) It preapare and execute the changes into infrastructure, including resources that to be created, modified, or destroyed.

For more details about Terraform please reach the official documentation: https://cloud.google.com/docs/terraform/terraform-overview
