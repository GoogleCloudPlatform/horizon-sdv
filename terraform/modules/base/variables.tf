# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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

variable "sdv_computer_sa" {
  description = "The Computer SA"
  type        = string
}

variable "sdv_cluster_name" {
  description = "Name of the cluster"
  type        = string
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

variable "sdv_cluster_node_locations" {
  description = "Define node locations/zones"
  type        = list(string)
}

variable "sdv_bastion_host_name" {
  description = "Name of the bastion host server"
  type        = string
}

variable "sdv_bastion_host_members" {
  description = "List of members allowed to access the bastion server"
  type        = list(string)
}

variable "sdv_bastion_host_sa" {
  description = "SA used by the bastion host and allow IAP to the host"
  type        = string
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
    roles = set(string)
  }))
}


#
# Define Secrets map id and value
variable "sdv_gcp_secrets_map" {
  description = "A map of secrets with their IDs and values."
  type = map(object({
    secret_id        = string
    value            = string
    use_github_value = bool
    gke_access = list(object({
      ns = string
      sa = string
    }))
  }))
}

variable "sdv_bastion_host_bash_command" {
  description = "Define the commands to run on the bastion host"
  type        = string
}

variable "sdv_list_of_apis" {
  description = "List of APIs for the project"
  type        = set(string)
}

# variable "sdv_bastion_host_files_to_copy" {
#   description = "List fo file that should be copied to the bastion host"
#   type        = list(string)
# }

# variable "sdv_bastion_host_destination_dir" {
#   description = "Destination dir on the bastion host"
#   type        = string
# }

variable "sdv_gh_abfs_license_b64" {
  description = "ABFS license base64"
  type        = string
}

