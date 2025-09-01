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

variable "abfs_gerrit_uploader_count" {
  type        = number
  description = "The number of gerrit uploader instances to create"
}

variable "abfs_gerrit_uploader_machine_type" {
  type        = string
  description = "Machine type for ABFS gerrit uploaders"
}
variable "abfs_gerrit_uploader_datadisk_size_gb" {
  type        = number
  description = "Size in GB for the ABFS gerrit uploader datadisk(s) that will be attached to the VM(s)"
}

variable "abfs_gerrit_uploader_datadisk_type" {
  type        = string
  description = "The PD regional disk type to use for the ABFS gerrit uploader datadisk(s)"
}

variable "abfs_docker_image_uri" {
  type        = string
  description = "Docker image URI for main ABFS server"
}

variable "abfs_gerrit_uploader_git_branch" {
  type        = set(string)
  description = "Branches from where to find projects (e.g. [\"main\",\"v-keystone-qcom-release\"]) (default [\"main\"])"
  default     = ["main"]
}

variable "abfs_manifest_project_name" {
  type        = string
  description = "Name of the git project on the manifest-server containing manifests"
  default     = "platform/manifest"
}

variable "abfs_manifest_file" {
  type        = string
  description = "Relative path from the manifest project root to the manifest file"
  default     = "default.xml"
}

variable "abfs_gerrit_uploader_manifest_server" {
  type        = string
  description = "The manifest server to assume"
  default     = "android.googlesource.com"
}

variable "abfs_license" {
  type        = string
  description = "ABFS license (JSON)"
}

variable "abfs_gerrit_uploader_allow_stopping_for_update" {
  type        = bool
  description = "Allow to stop uploaders to update properties"
  default     = true
}

