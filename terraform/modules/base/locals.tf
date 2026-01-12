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
# Details required for configuring Kubernetes providers and container image
# details along with version to be built and version to be deployed.

locals {
  connect_gateway_url = format(
    "https://connectgateway.googleapis.com/v1/projects/%s/locations/%s/gkeMemberships/%s",
    data.google_project.project.number,
    module.sdv_gke_cluster.location,
    module.sdv_gke_cluster.name
  )

  common_nginx_version = "1.28.0-alpine"

  images = {
    # build_version: version of container images to be built and pushed to Artifact Registry.
    # deploy_version: version of container images to be used for Argo CD post-jobs.

    "landingpage-app" = {
      directory      = "landingpage"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
      # Optional build args
      build_args = {
        NGINX_VERSION = local.common_nginx_version
      }
    }
    "gerrit-mcp-server-app" = {
      directory      = "gerrit-mcp-server"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "gerrit-post" = {
      directory      = "gerrit"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "mtk-connect-post" = {
      directory      = "mtk-connect"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "mtk-connect-post-key" = {
      directory      = "mtk-connect"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "grafana-post" = {
      directory      = "grafana"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-mcp-gateway-registry" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-gerrit" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-jenkins" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-argocd" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-headlamp" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-mtk-connect" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-grafana" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
  }
}