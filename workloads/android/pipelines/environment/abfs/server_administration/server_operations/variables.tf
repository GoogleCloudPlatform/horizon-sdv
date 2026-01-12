# Copyright (c) 2025 Accenture, All Rights Reserved.
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
variable "project_id" {
  type        = string
  description = "Google Cloud project ID"
}

variable "region" {
  type        = string
  description = "Region for ABFS servers"
}

variable "zone" {
  type        = string
  description = "Zone for ABFS servers"
}

variable "sdv_network" {
  description = "Name of the network"
  type        = string
}

variable "abfs_server_cos_image_ref" {
  type        = string
  description = "Reference to the COS boot image to use for the ABFS server"
  default     = "projects/cos-cloud/global/images/family/cos-113-lts"
}

variable "abfs_server_machine_type" {
  type        = string
  description = "Machine type for ABFS gerrit server"
}

variable "abfs_docker_image_uri" {
  type        = string
  description = "Docker image URI for main ABFS server"
}

variable "abfs_license" {
  type        = string
  description = "ABFS license (JSON)"
}

variable "abfs_server_allow_stopping_for_update" {
  type        = bool
  description = "Allow to stop the server to update properties"
  default     = true
}
