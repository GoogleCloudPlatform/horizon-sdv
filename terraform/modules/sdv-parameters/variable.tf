# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law_ or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Description:
# This file contains the variables for the sdv-parameters module.

variable "project_id" {
  description = "The GCP Project ID."
  type        = string
}

variable "location" {
  description = "The location for the parameters."
  type        = string
  default     = "us-central1"
}

variable "parameters_map" {
  description = "A map of parameters with their IDs and values."
  type = map(object({
    parameter_id         = string
    parameter_version_id = string
    value                = string
    format               = optional(string, "UNFORMATTED")
  }))
}