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
# 
# Description
# Configuration file containing variables for the "sdv-container-images" module.

variable "gcp_project_id" {
  description = "Project ID of the Google Artifact Registry"
  type        = string
}

variable "gcp_region" {
  description = "Region of the Google Artifact Registry"
  type        = string
}

variable "gcp_registry_id" {
  description = "Name of the Google Artifact Registry"
  type        = string
}

variable "images" {
  description = "A map of images to build. The key is the image name and the value is an object containing its build directory and version."
  type = map(object({
    directory  = string
    version    = string
    build_args = optional(map(string), {})
  }))
}