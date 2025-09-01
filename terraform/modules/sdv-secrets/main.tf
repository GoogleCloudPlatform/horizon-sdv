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
# This module creates several secrets and give the access to the defined
# Kubernetes Service Accounts.

data "google_project" "project" {}

locals {
  sdv_secrets = var.gcp_secrets_map
}

# Use to debug the secret values
# resource "terraform_data" "project_info" {
#   input = local.sdv_secrets
# }

resource "google_secret_manager_secret" "sdv_gsms" {
  for_each = local.sdv_secrets

  secret_id = each.value.secret_id

  replication {
    user_managed {
      replicas {
        location = var.location
      }
    }
  }
}

resource "google_secret_manager_secret_version" "sdv_gsmsv_use_github_value" {
  for_each = { for idx, secret in local.sdv_secrets : idx => secret if secret.use_github_value }

  secret      = google_secret_manager_secret.sdv_gsms[each.key].id
  secret_data = each.value.value

  #lifecycle {
  #  ignore_changes = [
  #    secret_data
  #  ]
  #}

  depends_on = [
    google_secret_manager_secret.sdv_gsms
  ]
}

resource "google_secret_manager_secret_version" "sdv_gsmsv_dont_use_github_value" {
  for_each = { for idx, secret in local.sdv_secrets : idx => secret if !secret.use_github_value }

  secret      = google_secret_manager_secret.sdv_gsms[each.key].id
  secret_data = each.value.value

  depends_on = [
    google_secret_manager_secret.sdv_gsms,
    google_secret_manager_secret_version.sdv_gsmsv_use_github_value
  ]
}

resource "google_secret_manager_secret_iam_binding" "sdv_secret_accessor" {
  for_each  = google_secret_manager_secret.sdv_gsms
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  members = [
    for gke_cfg in local.sdv_secrets[each.key].gke_access : "principal://iam.googleapis.com/projects/${data.google_project.project.number}/locations/global/workloadIdentityPools/${var.project_id}.svc.id.goog/subject/ns/${gke_cfg.ns}/sa/${gke_cfg.sa}"
  ]

  depends_on = [
    google_secret_manager_secret_version.sdv_gsmsv_dont_use_github_value,
    google_secret_manager_secret_version.sdv_gsmsv_use_github_value
  ]
}

# Use to debug the google project details
# resource "terraform_data" "project_info" {
#   input = data.google_project.project
# }
#

