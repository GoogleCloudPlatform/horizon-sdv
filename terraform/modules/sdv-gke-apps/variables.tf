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
# Description
# Configuration file containing variables for the "sdv-gke-apps" module.

variable "sub_environments" {
  type        = list(string)
  description = "List of sub-environments to be deployed."
}

variable "sub_env_branches" {
  description = "Map of sub-environment name to Git branch for ArgoCD sync"
  type        = map(string)
  default     = {}
}

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
  description = "SCM repository owner (for GitHub only)"
  type        = string
  default     = ""
}

variable "scm_repo_name" {
  description = "SCM repository name (for GitHub only)"
  type        = string
  default     = ""
}

variable "scm_username" {
  description = "SCM username"
  type        = string
  default     = "git"
}

variable "es_namespace" {
  description = "Namespace for External Secrets"
  type        = string
  default     = "external-secrets"
}

variable "es_chart_version" {
  description = "Chart version for External Secrets"
  type        = string
  default     = "0.10.4"
}

variable "argocd_namespace" {
  description = "Namespace for Argo CD"
  type        = string
  default     = "argocd"
}

variable "argocd_chart_version" {
  description = "Chart version for Argo CD"
  type        = string
  default     = "9.1.4"
}

variable "gcp_project_id" {
  description = "The GCP Project ID."
  type        = string
}

variable "gcp_cloud_region" {
  description = "The GCP cluster region"
  type        = string
}

variable "sdv_cluster_name" {
  description = "Name of the GKE Cluster"
  type        = string
}

variable "argocd_application_name" {
  description = "Name of the Argo CD Application"
  type        = string
  default     = "horizon-sdv"
}

variable "domain_name" {
  description = "The base domain name."
  type        = string
}

variable "subdomain_name" {
  description = "The subdomain for the environment"
  type        = string
}

variable "gcp_cloud_zone" {
  description = "The GCP zone"
  type        = string
}

variable "gcp_backend_bucket" {
  description = "The name of the GCS backend bucket."
  type        = string
}

variable "gcp_registry_id" {
  description = "The ID of the Artifact Registry repository (e.g., 'horizon-sdv')."
  type        = string
}

variable "gcp_storage_location" {
  description = "GCS bucket location for GCSBucket CRs / storage chart (same as sdv_location)."
  type        = string
}

variable "nodes_range" {
  description = "GKE node / primary subnet CIDR (kubelet probe source). Passed to GitOps MODULE_CONFIG probeSourceCIDRs."
  type        = string
  default     = "10.1.0.0/24"
}

variable "arm64_nodes_range" {
  description = "ARM64 GKE node / primary subnet CIDR. Included in probeSourceCIDRs when enable_arm64_dedicated_subnet is true."
  type        = string
  default     = "10.2.0.0/24"
}

variable "images" {
  description = "A map of images to deploy. The key is the image name and the value is an object containing its build directory and version."
  type = map(object({
    directory = string
    version   = string
  }))
}
variable "enable_network_policies" {
  description = "Enable network policies for all workloads. When disabled, all network policies will be removed. Default is enabled."
  type        = bool
  default     = true
}

variable "use_static_dns_a_records" {
  description = "When true, no Cloud DNS zone and no external-dns; use static A records in parent zone instead."
  type        = bool
  default     = false
}

variable "enable_arm64_dedicated_subnet" {
  description = "When true, arm64_placement_* uses arm64_* tfvars; when false, uses primary GKE region/zone/sdv-subnet for GitOps ARM64_* keys."
  type        = bool
  default     = false
}

variable "arm64_region" {
  description = "GCP region for ARM64 bare-metal Cuttlefish."
  type        = string
  default     = "us-central1"
}

variable "arm64_zone" {
  description = "GCP zone for ARM64 Cuttlefish (must be in arm64_region)."
  type        = string
  default     = "us-central1-b"
}

variable "arm64_subnetwork" {
  description = "Subnet name for ARM64 instances in arm64_region."
  type        = string
  default     = "sdv-subnet-arm64"
}

variable "primary_subnetwork" {
  description = "Primary platform subnet name (used for ARM64 placement when enable_arm64_dedicated_subnet is false)."
  type        = string
  default     = "sdv-subnet"
}

variable "common_nginx_version" {
  description = "nginx version to be used by horizon apps and workloads"
  type        = string
}
