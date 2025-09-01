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

// Get vpc details of existing horizon-sdv network
data "google_compute_network" "sdv_network_data" {
  name = var.sdv_cloud_ws_network_name
  project = var.sdv_cloud_ws_project_id
}
// Get subnetwork details of existing horizon-sdv subnetwork
data "google_compute_subnetwork" "sdv_cloud_ws_subnetwork_data" {
  name    = var.sdv_cloud_ws_subnetwork_name
  project = var.sdv_cloud_ws_project_id
  region  = var.sdv_cloud_ws_region
}

resource "google_workstations_workstation_cluster" "sdv_cloud_ws_cluster" {
  provider               = google-beta
  project                = var.sdv_cloud_ws_project_id
  workstation_cluster_id = var.sdv_cloud_ws_cluster_name
  network                = data.google_compute_network.sdv_network_data.id
  subnetwork             = data.google_compute_subnetwork.sdv_cloud_ws_subnetwork_data.id
  location               = var.sdv_cloud_ws_region

  # private_cluster_config {
  #   enable_private_endpoint = true
  # }

  # domain_config {
  #   domain = "workstations.example.com"
  # }
}