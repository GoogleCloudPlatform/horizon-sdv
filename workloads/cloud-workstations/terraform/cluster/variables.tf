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
variable "sdv_cloud_ws_project_id" {
  description = "GCP Project ID (existing) where Cloud Workstations is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv value
variable "sdv_cloud_ws_region" {
  description = "GCP region where Cloud Workstations is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv value
variable "sdv_cloud_ws_network_name" {
  description = "GCP network (VPC) name where Cloud Workstations is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv value
variable "sdv_cloud_ws_subnetwork_name" {
  description = "GCP subnetwork (VPC subnet) name where Cloud Workstations is deployed."
  type        = string
}

// from jenkins env; new resource
variable "sdv_cloud_ws_cluster_name" {
  description = "Name of the Cloud Workstations cluster."
  type        = string
}