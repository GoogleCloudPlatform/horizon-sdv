# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Description:
# Configuration file containing variables for the "base" module.

variable "sdv_sub_environments" {
  description = "List of sub-environments to create"
  type        = list(string)
  default     = []
}

variable "sdv_sub_env_branches" {
  description = "Map of sub-environment name to Git branch for ArgoCD sync"
  type        = map(string)
  default     = {}
}

# SCM Configuration
variable "scm_type" {
  description = "SCM type: 'github' or 'git'"
  type        = string
}

variable "scm_auth_method" {
  description = "SCM auth method: 'app' or 'userpass'"
  type        = string
}

variable "scm_repo_url" {
  description = "Full SCM repository URL"
  type        = string
}

variable "scm_repo_branch" {
  description = "SCM repository branch"
  type        = string
}

variable "scm_repo_owner" {
  description = "SCM repository owner (parsed from URL for GitHub)"
  type        = string
  default     = ""
}

variable "scm_repo_name" {
  description = "SCM repository name (parsed from URL for GitHub)"
  type        = string
  default     = ""
}

variable "scm_username" {
  description = "SCM username"
  type        = string
  default     = "git"
}

variable "github_repo_branch" {
  description = "[Legacy] Github repo branch"
  type        = string
  default     = ""
}

variable "env_name" {
  description = "Define the environment name"
  type        = string
}

variable "domain_name" {
  description = "Define the domain name"
  type        = string
}

variable "gcp_backend_bucket_name" {
  description = "Define the GCP backend bucket name"
  type        = string
}

variable "sdv_project" {
  description = "Define the GCP project id"
  type        = string
}

variable "sdv_network" {
  description = "Define the name of the VPC network"
  type        = string
}

variable "sdv_subnetwork" {
  description = "Define the subnet name"
  type        = string
}

variable "sdv_location" {
  description = "Define the default location for the project, should be the same as the region value"
  type        = string
}

variable "sdv_region" {
  description = "Define the default region for the project"
  type        = string
}

variable "sdv_zone" {
  description = "Define the default region zone for the project"
  type        = string
}

variable "sdv_gcp_compute_sa_email" {
  description = "The Computer SA"
  type        = string
}

variable "sdv_cluster_name" {
  description = "Name of the cluster"
  type        = string
}

variable "sdv_cluster_version" {
  description = "Cluster control plane min_master_version (floor; live master may be newer; not a downgrade pin)."
  type        = string
}

variable "sdv_cluster_release_channel" {
  description = "GKE cluster release_channel."
  type        = string
  default     = "UNSPECIFIED"
}

variable "sdv_cluster_node_pool_name" {
  description = "Name of the cluster node pool"
  type        = string
}

variable "sdv_cluster_node_pool_machine_type" {
  description = "Define the machine type of the node pool"
  type        = string
  default     = "n1-standard-4"
}

variable "sdv_cluster_node_pool_count" {
  description = "Define the number of nodes for the node pool"
  type        = number
  default     = 1
}

variable "sdv_cluster_node_pool_min_node_count" {
  description = "Minimum number of nodes for the cluster main node pool"
  type        = number
  default     = 1
}

variable "sdv_cluster_node_pool_max_node_count" {
  description = "Maximum number of nodes for the cluster main node pool"
  type        = number
  default     = 6
}

variable "sdv_cluster_node_locations" {
  description = "Define node locations/zones"
  type        = list(string)
}

variable "sdv_network_egress_router_name" {
  description = "Define the name of the egress router of the network"
  type        = string
}

variable "sdv_artifact_registry_repository_id" {
  description = "Define the name of the artifact registry repository name"
  type        = string
}

variable "sdv_artifact_registry_repository_members" {
  description = "List of members allowed to write access the artifact registry"
  type        = list(string)
}

variable "sdv_artifact_registry_repository_reader_members" {
  description = "List of members allowed to reader access the artifact registry"
  type        = list(string)
}

variable "sdv_ssl_certificate_name" {
  description = "Define the SSL Certificate name"
  type        = string
  default     = "horizon-sdv"
}

variable "sdv_ssl_certificate_domain" {
  description = "Define the SSL Certificate domain name"
  type        = string
}

variable "sdv_url_map_name" {
  description = "Define the URL map name"
  type        = string
  default     = "horizon-sdv-map"
}

variable "sdv_target_https_proxy_name" {
  description = "Define the HTTPs proxy name"
  type        = string
  default     = "horizon-sdv-https-proxy"
}

variable "sdv_build_node_pool_name" {
  description = "Name of the build node pool"
  type        = string
  default     = "sdv-build-node-pool"
}

variable "sdv_build_node_pool_node_count" {
  description = "Number of nodes for the build node pool"
  type        = number
  default     = 0
}

variable "sdv_build_node_pool_machine_type" {
  description = "Type fo the machine for the build node pool"
  type        = string
  default     = "c2d-highcpu-112"
}

variable "sdv_build_node_pool_min_node_count" {
  description = "Number of minimum of nodes for the build node pool"
  type        = number
  default     = 0
}

variable "sdv_build_node_pool_max_node_count" {
  description = "Number of max of nodes for the build node pool"
  type        = number
  default     = 20
}

variable "sdv_abfs_build_node_pool_name" {
  description = "Name of the ABFS build node pool"
  type        = string
  default     = "sdv-abfs-build-node-pool"
}

variable "sdv_abfs_build_node_pool_node_count" {
  description = "Number of nodes for the ABFS build node pool"
  type        = number
  default     = 0
}

variable "sdv_abfs_build_node_pool_machine_type" {
  description = "Type fo the machine for the ABFS build node pool"
  type        = string
  default     = "c2d-highcpu-112"
}

variable "sdv_abfs_build_node_pool_min_node_count" {
  description = "Number of minimum of nodes for the ABFS build node pool"
  type        = number
  default     = 0
}

variable "sdv_abfs_build_node_pool_max_node_count" {
  description = "Number of max of nodes for the build node pool"
  type        = number
  default     = 20
}

variable "sdv_abfs_build_node_pool_version" {
  description = "Kubernetes version for the ABFS build node pool (GKE node version string)."
  type        = string
}

variable "sdv_cluster_maintenance_recurring_window_start_time" {
  description = "GKE cluster recurring maintenance window start time."
  type        = string
  default     = "2025-01-04T00:00:00Z"
}

variable "sdv_cluster_maintenance_recurring_window_end_time" {
  description = "GKE cluster recurring maintenance window end time."
  type        = string
  default     = "2050-01-05T00:00:00Z"
}

variable "sdv_cluster_maintenance_recurring_window_recurrence" {
  description = "GKE cluster recurring maintenance window recurrence rule."
  type        = string
  default     = "FREQ=WEEKLY;BYDAY=SA,SU"
}

variable "sdv_cluster_maintenance_exclusions" {
  description = "GKE cluster maintenance exclusions (exclusion_name, start_time, end_time, scope)."
  type = list(object({
    exclusion_name = string
    start_time     = string
    end_time       = string
    scope          = string
  }))
  default = []
}

variable "sdv_openbsw_build_node_pool_name" {
  description = "Name of the OpenBSW build node pool"
  type        = string
  default     = "sdv-openbsw-build-node-pool"
}

variable "sdv_openbsw_build_node_pool_node_count" {
  description = "Number of nodes for the OpenBSW build node pool"
  type        = number
  default     = 0
}

variable "sdv_openbsw_build_node_pool_machine_type" {
  description = "Type of the machine for the OpenBSW build node pool"
  type        = string
  default     = "n1-standard-16"
}

variable "sdv_openbsw_build_node_pool_min_node_count" {
  description = "Number of minimum nodes for the OpenBSW build node pool"
  type        = number
  default     = 0
}

variable "sdv_openbsw_build_node_pool_max_node_count" {
  description = "Number of max nodes for the OpenBSW build node pool"
  type        = number
  default     = 20
}

variable "sdv_utility_node_pool_name" {
  description = "Name of the utility node pool (Gemini/Vertex CLI and similar workloads)"
  type        = string
  default     = "sdv-utility-node-pool"
}

variable "sdv_utility_node_pool_node_count" {
  description = "Number of nodes for the utility node pool"
  type        = number
  default     = 0
}

variable "sdv_utility_node_pool_machine_type" {
  description = "Machine type for the utility node pool (size for up to 32 CPU / 96Gi pod limits; n2-standard-48+ recommended over n2-standard-32 due to kube-reserved CPU)"
  type        = string
  default     = "n2-standard-48"
}

variable "sdv_utility_node_pool_min_node_count" {
  description = "Minimum number of nodes for the utility node pool"
  type        = number
  default     = 0
}

variable "sdv_utility_node_pool_max_node_count" {
  description = "Maximum number of nodes for the utility node pool"
  type        = number
  default     = 10
}

variable "sdv_wi_service_accounts" {
  description = "A map of service accounts and their configurations for WI"
  type = map(object({
    account_id   = string
    display_name = string
    description  = string
    gke_sas = list(object({
      gke_ns = string
      gke_sa = string
    }))
    roles    = set(string)
    sa_roles = optional(set(string), [])
    conditional_roles = optional(list(object({
      role        = string
      title       = string
      description = optional(string, "")
      expression  = string
    })), [])
  }))
}


#
# Define Secrets map id and value
variable "sdv_gcp_secrets_map" {
  description = "A map of secrets with their IDs and values."
  type = map(object({
    secret_id   = string
    value       = string
    apply_value = bool
    gke_access = list(object({
      ns = string
      sa = string
    }))
  }))
}

variable "sdv_list_of_apis" {
  description = "List of APIs for the project"
  type        = set(string)
}

# Define Parameters map
variable "sdv_gcp_parameters_map" {
  description = "Map of parameters for Parameter Manager"
  type        = any
}

variable "enable_arm64_dedicated_subnet" {
  description = "When true, create dedicated ARM64 subnet and NAT in arm64_region. When false, ARM64 Cuttlefish uses primary sdv-subnet; ARM workloads are still supported."
  type        = bool
}

variable "arm64_region" {
  description = "ARM64 region (example: us-central1)"
  type        = string
  default     = "us-central1"
}

variable "arm64_zone" {
  description = "ARM64 zone for Cuttlefish builds and Jenkins GCE clouds (must be in arm64_region)"
  type        = string
  default     = "us-central1-b"
}

variable "arm64_subnetwork" {
  description = "ARM64 subnet name"
  type        = string
  default     = "sdv-subnet-arm64"
}

variable "nodes_range" {
  description = "GKE node / primary subnet CIDR (source IP when pod egress is SNAT'd to node). Must match sdv-network primary subnet."
  type        = string
  default     = "10.1.0.0/24"
}

variable "pods_range" {
  description = "pod CIDR"
  type        = string
  default     = "10.10.0.0/16"
}

variable "services_range" {
  description = "service CIDR"
  type        = string
  default     = "10.12.0.0/16"
}
variable "arm64_nodes_range" {
  description = "ARM64 GKE node / primary subnet CIDR. Must match sdv-network ARM64 primary subnet."
  type        = string
  default     = "10.2.0.0/24"
}

variable "arm64_pods_range" {
  description = "ARM64 pod CIDR"
  type        = string
  default     = "10.20.0.0/16"
}

variable "arm64_services_range" {
  description = "ARM64 service CIDR"
  type        = string
  default     = "10.22.0.0/16"
}

variable "arm64_pods_secondary_range_name" {
  description = "Secondary IP range name for pods on the ARM64 dedicated subnet (GCP must already use this name; override in terraform.tfvars only if non-standard)."
  type        = string
  default     = "pods-range-arm64"
}

variable "arm64_services_secondary_range_name" {
  description = "Secondary IP range name for services on the ARM64 dedicated subnet (GCP must already use this name; override in terraform.tfvars only if non-standard)."
  type        = string
  default     = "services-range-arm64"
}

variable "sdv_enable_network_policies" {
  description = "Enable network policies for all workloads. When disabled, all network policies will be removed. Default is enabled."
  type        = bool
  default     = true
}

variable "sdv_dns_dnssec_enabled" {
  description = "Enable DNSSEC for Cloud DNS zone. Requires domain ownership verification. Enabled by default."
  type        = bool
  default     = true
}

variable "sdv_dns_use_static_a_records" {
  description = "Use static A records in parent zone instead of zone delegation. When true: no Cloud DNS zone for app domain, certificate uses Load Balancer authorization, DNSSEC off. Add A records (domain and mcp.domain) to parent zone manually; LB IP is visible in GCP Console."
  type        = bool
  default     = false
}

variable "sdv_enable_kms_encryption" {
  description = "Enable KMS encryption for GKE secrets"
  type        = bool
  default     = false
}

