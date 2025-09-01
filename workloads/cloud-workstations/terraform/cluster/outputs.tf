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
  deleted_cluster_output = "[DELETED CLUSTER] ${var.sdv_cloud_ws_cluster_name}"
}

output "cluster_name" {
  description = "Name of the created Cloud Workstations cluster"
  value       = try(google_workstations_workstation_cluster.sdv_cloud_ws_cluster.name, local.deleted_cluster_output)
}

output "project_id" {
  description = "GCP Project ID where the cluster is deployed"
  value       = var.sdv_cloud_ws_project_id
}

output "location" {
  description = "Region (Location) of the Cloud Workstations cluster"
  value       = var.sdv_cloud_ws_region
}

output "network_name" {
  description = "GCP network (VPC) name where Cloud Workstations is deployed."
  value       = var.sdv_cloud_ws_network_name
}

output "subnetwork_name" {
  description = "GCP subnetwork (VPC subnet) name where Cloud Workstations is deployed."
  value       = var.sdv_cloud_ws_subnetwork_name
}
