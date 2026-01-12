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
#
# Description:
# Configuration file containing variables for the "sdv-network" module.

variable "region" {
  description = "Define the Region"
  type        = string
}

variable "network" {
  description = "Define the Network"
  type        = string
}

variable "subnetwork" {
  description = "Define the Sub Network"
  type        = string
}

variable "router_name" {
  description = "Define the router name"
  type        = string
}

variable "enable_arm64" {
  description = "Enable ARM64 networking"
  type        = bool
}

variable "arm64_region" {
  description = "Define the ARM64 region"
  type        = string
}

variable "arm64_subnetwork" {
  description = "Define the ARM64 Subnetwork name"
  type        = string
}

variable "arm64_pods_range" {
  type = string
}

variable "arm64_services_range" {
  type = string
}