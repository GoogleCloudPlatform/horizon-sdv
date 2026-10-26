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

variable "force_update_secret_ids" {
  description = "List of secret keys (e.g. ['s6']) to force regeneration for (only applies to auto-generated secrets)."
  type        = list(string)
  default     = []
}

variable "manual_secrets" {
  description = "Map of secret keys (e.g. s6, s12) to manual password values."
  type        = map(string)
  default     = {}
  sensitive   = true

  validation {
    condition = alltrue([
      for k, v in var.manual_secrets : (
        length(v) >= 12 &&            # At least 12 characters
        v != "Change_Me_123" &&       # Not default value ("Change_Me_123")
        can(regex("[A-Z]", v)) &&     # At least one Uppercase
        can(regex("[0-9]", v)) &&     # At least one Number
        can(regex("[^a-zA-Z0-9]", v)) # At least one Symbol (anything not letter/num)
      )
    ])
    error_message = "Invalid Password detected. All manual secrets must:\n1. Must be at least 12 characters long\n2. Not be 'Change_Me_123'\n3. Contain at least 1 Uppercase letter\n4. Contain at least 1 Number\n5. Contain at least 1 Symbol"
  }
}

# SCM Configuration
variable "scm_type" {
  description = "SCM type: 'github' (GitHub-specific features) or 'git' (generic Git server)"
  type        = string
  default     = "github"
  validation {
    condition     = contains(["github", "git"], var.scm_type)
    error_message = "SCM type must be either 'github' or 'git'."
  }
}

variable "scm_auth_method" {
  description = "Auth method: 'app' (GitHub App only), 'userpass' (username/password or token), or 'none' (public repos)"
  type        = string
  validation {
    condition     = contains(["app", "userpass", "none"], var.scm_auth_method)
    error_message = "Auth method must be 'app' (GitHub App), 'userpass' (username/password), or 'none' (public repository)."
  }
}

variable "scm_repo_url" {
  description = "Full repository URL (e.g., https://github.com/owner/repo or https://git.example.com/project/repo)"
  type        = string
}

variable "scm_repo_branch" {
  description = "Repository branch or ref"
  type        = string
}

variable "scm_username" {
  description = "SCM username (use 'git' for GitHub PAT, actual username for other Git servers)"
  type        = string
  default     = "git"
}

variable "scm_password" {
  description = "SCM password or token (GitHub PAT, Gerrit HTTP password, GitLab token, etc.)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "sdv_github_app_id" {
  description = "GitHub App ID (only for GitHub App auth)"
  type        = string
  default     = ""
}

variable "sdv_github_app_install_id" {
  description = "GitHub App Installation ID (only for GitHub App auth)"
  type        = string
  default     = ""
}

variable "sdv_github_app_private_key" {
  description = "The secret GH_APP_KEY value"
  type        = string
  default     = ""
  sensitive   = true
}

variable "sdv_keycloak_admin_password" {
  description = "The secret KEYCLOAK_INITIAL_PASSWORD value"
  type        = string
  default     = "Change_Me_123"

  validation {
    condition = (
      var.sdv_keycloak_admin_password != "Change_Me_123" &&
      length(var.sdv_keycloak_admin_password) >= 12 &&
      length(regexall("[a-z]", var.sdv_keycloak_admin_password)) > 0 &&
      length(regexall("[A-Z]", var.sdv_keycloak_admin_password)) > 0 &&
      length(regexall("[0-9]", var.sdv_keycloak_admin_password)) > 0 &&
      length(regexall("[^a-zA-Z0-9]", var.sdv_keycloak_admin_password)) > 0
    )
    error_message = local.password_policy_error
  }
}

variable "sdv_keycloak_horizon_admin_password" {
  description = "The secret KEYCLOAK_HORIZON_ADMIN_PASSWORD value"
  type        = string
  default     = "Change_Me_123"

  validation {
    condition = (
      var.sdv_keycloak_horizon_admin_password != "Change_Me_123" &&
      length(var.sdv_keycloak_horizon_admin_password) >= 12 &&
      length(regexall("[a-z]", var.sdv_keycloak_horizon_admin_password)) > 0 &&
      length(regexall("[A-Z]", var.sdv_keycloak_horizon_admin_password)) > 0 &&
      length(regexall("[0-9]", var.sdv_keycloak_horizon_admin_password)) > 0 &&
      length(regexall("[^a-zA-Z0-9]", var.sdv_keycloak_horizon_admin_password)) > 0
    )
    error_message = local.password_policy_error
  }
}

variable "sdv_env_name" {
  description = "Environment name (used as the sub-domain for the platform)"
  type        = string
}

variable "sdv_root_domain" {
  description = "Horizon domain name"
  type        = string
}

variable "sdv_gcp_project_id" {
  description = "GCP project id"
  type        = string
}

variable "sdv_gcp_compute_sa_email" {
  description = "GCP computer SA"
  type        = string
}

variable "sdv_gcp_region" {
  description = "GCP cloud region"
  type        = string
}

variable "sdv_gcp_zone" {
  description = "GCP cloud zone"
  type        = string
}

variable "sdv_gcp_backend_bucket" {
  description = "GCP cloud bucket name that stores tfstate file"
  type        = string
}

variable "enable_arm64_dedicated_subnet" {
  description = "When true, create a dedicated ARM64 VPC subnet and NAT in arm64_region (use when GKE and ARM64 metal differ). When false, ARM64 Cuttlefish uses the primary platform subnet (sdv-subnet) in sdv_gcp_region; ARM jobs still run."
  type        = bool
}

variable "arm64_region" {
  description = "GCP region for ARM64 bare-metal subnet (independent of sdv_gcp_region / GKE)."
  type        = string
  default     = "us-central1"
}

variable "arm64_zone" {
  description = "GCP zone for ARM64 Cuttlefish Packer builds and Jenkins GCE clouds (must be in arm64_region)."
  type        = string
  default     = "us-central1-b"
}

variable "arm64_subnetwork" {
  description = "Subnet name for ARM64 instances in arm64_region."
  type        = string
  default     = "sdv-subnet-arm64"
}

variable "arm64_pods_secondary_range_name" {
  description = "Secondary IP range name for pods on the ARM64 dedicated subnet. Default pods-range-arm64 (greenfield). Brownfield: override in tfvars to match GCP until migrated, or migrate GCP to -arm64."
  type        = string
  default     = "pods-range-arm64"
}

variable "arm64_services_secondary_range_name" {
  description = "Secondary IP range name for services on the ARM64 dedicated subnet. Default services-range-arm64 (greenfield). Brownfield: override in tfvars to match GCP until migrated, or migrate GCP to -arm64."
  type        = string
  default     = "services-range-arm64"
}

# --- SUB-ENVIRONMENT CONFIGURATION ---

variable "sdv_sub_env_configs" {
  description = "Configuration for each sub-environment including required passwords"
  type = map(object({
    keycloak_admin_password         = string
    keycloak_horizon_admin_password = string
    manual_secrets                  = optional(map(string), {})
    branch                          = optional(string, null)
  }))
  default   = {}
  sensitive = true

  validation {
    condition = alltrue([
      for env in keys(var.sdv_sub_env_configs) :
      can(regex("^[a-z0-9]([a-z0-9-]{0,2}[a-z0-9])?$", env))
    ])
    error_message = "Sub-environment names must be lowercase alphanumeric with hyphens, 1-4 characters."
  }

  validation {
    condition = alltrue([
      for env, config in var.sdv_sub_env_configs :
      config.keycloak_admin_password != "Change_Me_123" &&
      length(config.keycloak_admin_password) >= 12 &&
      can(regex("[A-Z]", config.keycloak_admin_password)) &&
      can(regex("[a-z]", config.keycloak_admin_password)) &&
      can(regex("[0-9]", config.keycloak_admin_password)) &&
      can(regex("[^a-zA-Z0-9]", config.keycloak_admin_password))
    ])
    error_message = "Each sub-env keycloak_admin_password must not be 'Change_Me_123' and must be at least 12 chars with uppercase, lowercase, numbers, and special characters."
  }

  validation {
    condition = alltrue([
      for env, config in var.sdv_sub_env_configs :
      config.keycloak_horizon_admin_password != "Change_Me_123" &&
      length(config.keycloak_horizon_admin_password) >= 12 &&
      can(regex("[A-Z]", config.keycloak_horizon_admin_password)) &&
      can(regex("[a-z]", config.keycloak_horizon_admin_password)) &&
      can(regex("[0-9]", config.keycloak_horizon_admin_password)) &&
      can(regex("[^a-zA-Z0-9]", config.keycloak_horizon_admin_password))
    ])
    error_message = "Each sub-env keycloak_horizon_admin_password must not be 'Change_Me_123' and must be at least 12 chars with uppercase, lowercase, numbers, and special characters."
  }

  validation {
    condition = alltrue([
      for env, config in var.sdv_sub_env_configs :
      alltrue([
        for k, v in config.manual_secrets :
        v != "Change_Me_123" &&
        length(v) >= 12 &&
        can(regex("[A-Z]", v)) &&
        can(regex("[a-z]", v)) &&
        can(regex("[0-9]", v)) &&
        can(regex("[^a-zA-Z0-9]", v))
      ])
    ])
    error_message = "Sub-env manual_secrets values must not be 'Change_Me_123' and must meet the password policy (12+ chars, uppercase, lowercase, number, symbol)."
  }
}
variable "sdv_abfs_build_node_pool_version" {
  description = "Kubernetes version for the ABFS build node pool (GKE node version string)."
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
  description = "Use static A records in parent zone instead of zone delegation. When true: no Cloud DNS zone, LB cert auth, DNSSEC off. Add A records (domain and mcp.domain) to parent zone manually; LB IP from GCP Console."
  type        = bool
  default     = false
}

variable "sdv_enable_kms_encryption" {
  description = "Enable KMS encryption for GKE secrets. Note: KMS keyrings cannot be deleted once created in GCP. Set to false to avoid KMS entirely."
  type        = bool
  default     = false
}
