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
# Description:
# Main configuration file for the "sdv-parameters" module.
# Create required parameters with the required details.

data "google_project" "project" {}

locals {
  sdv_params = var.parameters_map
}

# Create the Parameter Container
resource "google_parameter_manager_parameter" "sdv_params" {
  for_each = local.sdv_params

  project      = var.project_id
  parameter_id = each.value.parameter_id
  format       = each.value.format

}

# Create the Parameter Version (Contains the Value)
resource "google_parameter_manager_parameter_version" "sdv_params_version" {
  for_each = local.sdv_params

  parameter            = google_parameter_manager_parameter.sdv_params[each.key].id
  parameter_version_id = each.value.parameter_version_id
  parameter_data       = each.value.value

  depends_on = [
    google_parameter_manager_parameter.sdv_params
  ]
}
