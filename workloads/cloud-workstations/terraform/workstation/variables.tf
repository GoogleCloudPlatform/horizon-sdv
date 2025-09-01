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

// from jenkins env; same as horizon-sdv value
variable "sdv_cloud_ws_bucket" {
  description = "GCS bucket name where Cloud Workstations tfstate is stored."
  type        = string
  default     = "prj-sbx-horizon-sdv-tf"
}

// from jenkins env; same as horizon-sdv value
variable "sdv_cloud_ws_project_id" {
  description = "GCP Project ID (existing) where Cloud Workstations is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv value
variable "sdv_cloud_ws_region" {
  description = "GCP region where Cloud Workstations is deployed."
  type        = string
}

// from jenkins env; new resource
variable "sdv_cloud_ws_cluster_name" {
  description = "Name of the Cloud Workstations cluster."
  type        = string
}

# A map of workstations, keyed by workstation_id:
variable "workstations" {
  type = map(object({
    sdv_cloud_ws_workstation_config_id = string
    sdv_cloud_ws_workstation_id        = string
    sdv_cloud_ws_display_name          = string
    sdv_cloud_ws_user_emails           = list(string)
  }))
  description = "List of Cloud Workstations to manage"

  validation {
    condition = alltrue([
      for ws in values(var.workstations) : (
        # All three required fields must be non-empty
        trim(ws.sdv_cloud_ws_workstation_config_id," ") != "" &&
        trim(ws.sdv_cloud_ws_workstation_id," ") != ""
      )
    ])
    error_message = "WORKSTATION_CONFIG_NAME and WORKSTATION_NAME cannot be empty."
  }

  validation {
    condition = alltrue([
      for ws in values(var.workstations) : (
        # Validate workstation_id: lowercase, hyphens allowed, but cannot start or end with '-'
        can(regex("^([a-z0-9]+(-[a-z0-9]+)*)$", ws.sdv_cloud_ws_workstation_id))
      )
    ])
    error_message = "Each WORKSTATION_NAME must be lowercase, can contain hyphens, and cannot start or end with a hyphen."
  }

  validation {
    condition = alltrue([
      for ws in values(var.workstations) : (
        length(distinct(ws.sdv_cloud_ws_user_emails)) == length(ws.sdv_cloud_ws_user_emails)
      )
    ])
    error_message = "Each workstation's INITIAL_WORKSTATION_USER_EMAILS_TO_ADD list cannot contain duplicates."
  }
}

