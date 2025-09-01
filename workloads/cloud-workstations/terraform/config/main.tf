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

locals {
  sdv_cloud_ws_host_vm_roles = [
    "roles/artifactregistry.reader",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/storage.objectViewer",
    "roles/storage.objectCreator"
  ]
}

// Get current project data
data "google_project" "project_data" {
  project_id = var.sdv_cloud_ws_project_id
}

// Get iam_bindings for each existing ws config
data "google_workstations_workstation_config_iam_policy" "ws_config_admin_iam_members_data" {
  for_each = {
    for config_name, config in var.sdv_cloud_ws_configs :
    config_name => config if config_name != var.sdv_cloud_ws_input_config_name
  }

  provider               = google-beta
  project                = var.sdv_cloud_ws_project_id
  location               = var.sdv_cloud_ws_region
  workstation_cluster_id = var.sdv_cloud_ws_cluster_name
  workstation_config_id  = each.key
}

// Service Account for use in Cloud Workstations Host VM
resource "google_service_account" "sdv_cloud_ws_service_account" {
  account_id   = "sdv-cloud-ws-host-vm-sa"
  display_name = "Service Account for Cloud Workstations Host VM"
}
resource "google_project_iam_member" "sdv_cloud_ws_sa_roles" {
  for_each = toset(local.sdv_cloud_ws_host_vm_roles)

  project = var.sdv_cloud_ws_project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.sdv_cloud_ws_service_account.email}"
}


resource "google_workstations_workstation_config" "sdv_cloud_ws_config" {
  for_each = var.sdv_cloud_ws_configs

  provider               = google-beta
  workstation_config_id  = each.key
  workstation_cluster_id = var.sdv_cloud_ws_cluster_name
  location               = var.sdv_cloud_ws_region

  idle_timeout    = "${each.value.ws_idle_timeout}s"
  running_timeout = "${each.value.ws_running_timeout}s"
  replica_zones   = each.value.ws_replica_zones

  host {
    gce_instance {
      machine_type                 = each.value.host_machine_type
      service_account              = google_service_account.sdv_cloud_ws_service_account.email
      pool_size                    = each.value.host_quickstart_pool_size
      boot_disk_size_gb            = each.value.host_boot_disk_size_gb
      disable_public_ip_addresses  = each.value.host_disable_public_ip_addresses
      disable_ssh                  = each.value.host_disable_ssh
      enable_nested_virtualization = each.value.host_enable_nested_virtualization
    }
  }

  dynamic "persistent_directories" {
    for_each = (
      each.value.pd_required ? [1] : []
    )
    content {
      mount_path = each.value.pd_mount_path

      gce_pd {
        fs_type         = each.value.pd_fs_type
        disk_type       = each.value.pd_disk_type
        size_gb         = each.value.pd_size_gb
        reclaim_policy  = each.value.pd_reclaim_policy
        source_snapshot = each.value.pd_source_snapshot != null ? "projects/${var.sdv_cloud_ws_project_id}/global/snapshots/${each.value.pd_source_snapshot}" : null
      }
    }
  }

  dynamic "ephemeral_directories" {
    for_each = (
      each.value.ed_required ? [1] : []
    )
    content {
      mount_path = each.value.ed_mount_path

      gce_pd {
        disk_type       = each.value.ed_disk_type
        source_snapshot = each.value.ed_source_snapshot
        source_image    = each.value.ed_source_image
        read_only       = each.value.ed_read_only
      }
    }
  }

  container {
    image       = each.value.container_image != null ? each.value.container_image : var.sdv_cloud_ws_horizon_code_oss_image_full_path
    command     = each.value.container_entrypoint_commands
    args        = each.value.container_entrypoint_args
    working_dir = each.value.container_working_dir

    env = each.value.container_env_vars

    run_as_user = each.value.container_user
  }

  dynamic "allowed_ports" {
    for_each = each.value.ws_allowed_ports
    iterator = each_port_range # iterates on `ws_allowed_ports` list of objects
    content {
      first = each_port_range.value.first
      last  = each_port_range.value.last
    }
  }
}

resource "google_workstations_workstation_config_iam_binding" "sdv_cloud_ws_config_iam_binding" {
  # create a filtered map of only those ws configs that have atleast one ws admin IAM member listed
  for_each = {
    for config_name, config in var.sdv_cloud_ws_configs :
    config_name => config if length(config.ws_admin_iam_members) > 0
  }
  # now iterate over this map to create iam bindings for each ws config (new input + existing)
  provider               = google-beta
  project                = var.sdv_cloud_ws_project_id
  location               = var.sdv_cloud_ws_region
  workstation_cluster_id = var.sdv_cloud_ws_cluster_name
  workstation_config_id  = each.key
  role                   = "roles/workstations.admin"
  members                = distinct([for email in each.value.ws_admin_iam_members : "user:${email}"])
}