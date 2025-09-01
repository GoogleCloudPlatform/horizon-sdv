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

// -------PRESET-------

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

// from jenkins env; resource created by cluster tf workflow
variable "sdv_cloud_ws_cluster_name" {
  description = "Name of the Cloud Workstations cluster."
  type        = string
}

// from jenkins env; resource created by workstation-images
variable "sdv_cloud_ws_horizon_code_oss_image_full_path" {
  description = "Name of the `Horizon Code OSS (VS Code)` docker image on artifact registry, for use as the default container image for Cloud Workstations."
  type        = string
  default     = null
}


// -------USER PROVIDED-------

variable "sdv_cloud_ws_input_config_name" {
  description = "Name of the new Cloud WS config input by user."
  type        = string
} // useful when output is shown post apply

variable "sdv_cloud_ws_configs" {
  description = "Map of workstation configs. Each object is a cloud-ws config"
  type = map(object({
    ws_idle_timeout    = optional(number, 1200)                                       # def: 20 mins
    ws_running_timeout = optional(number, 43200)                                      # def: 12 hours
    ws_replica_zones   = optional(list(string), ["europe-west1-b", "europe-west1-d"]) # def: two zones within region if empty string

    host_machine_type                 = optional(string, "e2-standard-4")
    host_quickstart_pool_size         = optional(number, 0)  # quickstart ws pool size, def: 0, min: 1 if quickstart enabled
    host_boot_disk_size_gb            = optional(number, 30) # def: 50 GB, but min 30 GB
    host_disable_public_ip_addresses  = optional(bool, true)
    host_disable_ssh                  = optional(bool, true)
    host_enable_nested_virtualization = optional(bool, false) # enable for emulators

    pd_required        = optional(bool, false) # PD values are only set if this is true
    pd_mount_path      = optional(string, "/home")
    pd_fs_type         = optional(string, "ext4")
    pd_disk_type       = optional(string, "pd-standard") # options: [pd-standard, pd-balanced, pd-ssd, pd-extreme]
    pd_size_gb         = optional(number, 200)           # def: 200; options: [10, 50, 100, 200, 500, 1000]
    pd_reclaim_policy  = optional(string, "DELETE")      # options: [DELETE, RETAIN]
    pd_source_snapshot = optional(string)

    ed_required        = optional(bool, false) # ED values are only set if this is true
    ed_mount_path      = optional(string, "/tmp")
    ed_disk_type       = optional(string, "pd-standard")
    ed_source_snapshot = optional(string)
    ed_source_image    = optional(string)
    ed_read_only       = optional(bool, false)

    # container is always created no matter user specifies or not
    container_image               = optional(string)           # def: horizon-code-oss, set in resource def
    container_entrypoint_commands = optional(list(string), []) # null not allowed on apply
    container_entrypoint_args     = optional(list(string), []) # null not allowed on apply
    container_working_dir         = optional(string)
    container_env_vars            = optional(map(string), {}) # null not allowed on apply
    container_user                = optional(string)

    ws_allowed_ports = optional(list(object({ first = number, last = number })), [{ first = 80, last = 80 }, { first = 1024, last = 65535 }])

    ws_admin_iam_members = optional(list(string), [])
  }))

  #------PD validations------
  validation {
    condition = alltrue([
      for config_name, config in var.sdv_cloud_ws_configs :
      !(
        config.pd_source_snapshot != null &&
        (config.pd_fs_type != null || config.pd_size_gb != null)
      )
    ])
    error_message = "If a Persistent disk 'source snapshot' is set then 'file system type' or 'disk size' cannot be specified."
  }
  validation {
    condition = alltrue([
      for config_name, config in var.sdv_cloud_ws_configs :
      (
        contains([10, 50, 100, 200, 500, 1000], config.pd_size_gb)
      )
    ])
    error_message = "Persistent disk size must be one of: 10, 50, 100, 200, 500, 1000 GB."
  }
  validation {
    condition = alltrue([
      for config_name, config in var.sdv_cloud_ws_configs :
      (
        config.pd_size_gb >= 200 ||
        contains(["pd-balanced", "pd-ssd"], config.pd_disk_type)
      )
    ])
    error_message = "If Persistent disk size is less than 200 GB, disk type must be 'pd-balanced' or 'pd-ssd'."
  }

  #------ED validations------
  validation {
    condition = alltrue([
      for config_name, config in var.sdv_cloud_ws_configs :
      !(
        config.ed_source_snapshot != null && config.ed_source_image != null
      )
    ])
    error_message = "Ephemeral disk 'source snapshot' and 'source image' cannot be specified together."
  }
  validation {
    condition = alltrue([
      for config_name, config in var.sdv_cloud_ws_configs :
      !(
        config.ed_source_snapshot != null && config.ed_read_only == false
      )
    ])
    error_message = "If Ephemeral disk 'source snapshot' is set then disk must be 'read-only'."
  }
  validation {
    condition = alltrue([
      for config_name, config in var.sdv_cloud_ws_configs :
      !(
        config.ed_read_only == true && config.ed_source_snapshot == null
      )
    ])
    error_message = "If Ephemeral disk is set as 'read-only' then a 'disk source snapshot' must also be specified."
  }
}