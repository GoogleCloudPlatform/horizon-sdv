# Copyright (c) 2026 Accenture, All Rights Reserved.
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

// Get vpc details of existing horizon-sdv network
data "google_compute_network" "sdv_network_data" {
  name = var.sdv_aosp_mirror_network_name
  project = var.sdv_aosp_mirror_project_id
}
// Get subnetwork details of existing horizon-sdv subnetwork
data "google_compute_subnetwork" "sdv_aosp_mirror_subnetwork_data" {
  name    = var.sdv_aosp_mirror_subnetwork_name
  project = var.sdv_aosp_mirror_project_id
  region  = var.sdv_aosp_mirror_region
}

// Create Filestore instance for Mirror
resource "google_filestore_instance" "sdv_aosp_mirror_filestore_instance" {
  name     = var.sdv_aosp_mirror_filestore_instance_name
  location = var.sdv_aosp_mirror_zone
  project  = var.sdv_aosp_mirror_project_id
  tier     = "ZONAL"

  file_shares {
    name        = var.sdv_aosp_mirror_filestore_share_name
    capacity_gb = var.sdv_aosp_mirror_filestore_share_capacity_gb

    nfs_export_options {
      ip_ranges   = ["${data.google_compute_subnetwork.sdv_aosp_mirror_subnetwork_data.ip_cidr_range}"]
      access_mode = "READ_WRITE"
      squash_mode = "NO_ROOT_SQUASH"
    }
  }

  networks {
    network      = data.google_compute_network.sdv_network_data.name
    modes        = ["MODE_IPV4"]
    connect_mode = "DIRECT_PEERING" # filestore and gke cluster in same vpc
  }
}

// Create Persistent Volume for Mirror Filestore
resource "kubernetes_persistent_volume" "sdv_aosp_mirror_filestore_pv" {
  metadata {
    name = var.sdv_aosp_mirror_filestore_pv_name
  }
  spec {
    capacity = {
      storage = "${var.sdv_aosp_mirror_filestore_share_capacity_gb}Gi"
    }
    access_modes = ["ReadWriteMany"]
    persistent_volume_source {
      nfs {
        path   = "/${var.sdv_aosp_mirror_filestore_share_name}"
        server = google_filestore_instance.sdv_aosp_mirror_filestore_instance.networks[0].ip_addresses[0]
      }
    }
    persistent_volume_reclaim_policy = "Delete"
  }
  depends_on = [google_filestore_instance.sdv_aosp_mirror_filestore_instance]
}

// Create Persistent Volume Claim for Mirror Filestore
resource "kubernetes_persistent_volume_claim" "sdv_aosp_mirror_filestore_pvc" {
  metadata {
    name      = var.sdv_aosp_mirror_filestore_pvc_name
    namespace = var.sdv_aosp_mirror_filestore_pvc_namespace
    annotations = {
      // to bind to a specific PV (sdv_aosp_mirror_filestore_pv)
      // also prevents k8s from assigning a default storage class
      "volume.beta.kubernetes.io/storage-class" = ""
    }
  }
  spec {
    access_modes = ["ReadWriteMany"]
    storage_class_name = "" // to bind to a specific PV (sdv_aosp_mirror_filestore_pv)
    resources {
      requests = {
        storage = "${var.sdv_aosp_mirror_filestore_share_capacity_gb}Gi"
      }
    }
    volume_name = kubernetes_persistent_volume.sdv_aosp_mirror_filestore_pv.metadata[0].name
  }
  timeouts {
    create = "10m"
  }
}