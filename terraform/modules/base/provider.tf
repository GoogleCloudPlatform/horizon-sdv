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
# Contains blocks to configure the Google Cloud provider for Terraform with 
# the necessary project details.

data "google_client_config" "default" {}

provider "google" {
  project = var.sdv_project
  region  = var.sdv_region
  zone    = var.sdv_zone
}

provider "google-beta" {
  project = var.sdv_project
  region  = var.sdv_region
  zone    = var.sdv_zone
}

provider "docker" {
  registry_auth {
    address  = "${var.sdv_region}-docker.pkg.dev"
    username = "oauth2accesstoken"
    password = data.google_client_config.default.access_token
  }
}

provider "kubernetes" {
  host  = local.connect_gateway_url
  token = data.google_client_config.default.access_token

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "gke-gcloud-auth-plugin"
  }
}

provider "helm" {
  kubernetes = {
    host  = local.connect_gateway_url
    token = data.google_client_config.default.access_token
  }
}

provider "kubectl" {
  host  = local.connect_gateway_url
  token = data.google_client_config.default.access_token

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "gke-gcloud-auth-plugin"
  }

  load_config_file = false
}
