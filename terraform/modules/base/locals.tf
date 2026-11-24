# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

locals {
  connect_gateway_url = format(
    "https://connectgateway.googleapis.com/v1/projects/%s/locations/%s/gkeMemberships/%s",
    data.google_project.project.number,
    module.sdv_gke_cluster.location,
    module.sdv_gke_cluster.name
  )

  # nginx image tag for GitOps sidecars (token-injector, module-overview). Source: tools.nginx.tag
  # in the repo-root versions.yaml; do not hardcode it here.
  common_nginx_version = local.versions.tools.nginx.tag

  # Horizon platform version(s) from the repo-root versions.yaml. components.horizoncore is also the
  # Developer Portal image tag (build_version / deploy_version of "horizon-dev-portal" below).
  # tools.nginx.tag from the same file feeds common_nginx_version above.
  # tools.go.version is the Go image patch passed as build-arg GO_VERSION.
  versions             = yamldecode(file("${path.module}/../../../versions.yaml"))
  horizon_core_version = local.versions.components.horizoncore
  go_version           = local.versions.tools.go.version

  images = {
    # build_version: version of container images to be built and pushed to Artifact Registry.
    # deploy_version: version of container images to be used for Argo CD post-jobs.

    "storage-gcs-module-app" = {
      directory      = "storage-gcs-module"
      build_version  = "1.0.2"
      deploy_version = "1.0.2"
    }
    "cloud-workstations-module-app" = {
      directory      = "cloud-workstations-module"
      build_version  = "0.2.1"
      deploy_version = "0.2.1"
    }
    "mtk-connect-module-app" = {
      directory      = "mtk-connect-module"
      build_version  = "0.3.1"
      deploy_version = "0.3.1"
    }
    "gerrit-mcp-server-app" = {
      directory      = "gerrit-mcp-server"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "gerrit-post" = {
      directory      = "gerrit"
      build_version  = "1.0.2"
      deploy_version = "1.0.2"
    }
    "mtk-connect-post" = {
      directory      = "mtk-connect"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "mtk-connect-post-key" = {
      directory      = "mtk-connect"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "grafana-post" = {
      directory      = "grafana"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-mcp-gateway-registry" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-gerrit" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-jenkins" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-argocd" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-headlamp" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-mtk-connect" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-grafana" = {
      directory      = "keycloak"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "keycloak-post-argo-workflows" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    "keycloak-post-horizon-api" = {
      directory      = "keycloak"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
    }
    # Developer portal (Vite + Go proxy). context_path is set so sdv-container-images trigger hashing skips node_modules/dist (same as former external client tree).
    # Image tag (build_version / deploy_version) follows components.horizoncore from the repo-root versions.yaml; do not edit here.
    "horizon-dev-portal" = {
      directory      = "horizon-dev-portal"
      build_version  = local.horizon_core_version
      deploy_version = local.horizon_core_version
      context_path   = abspath("${path.module}/../sdv-container-images/images/horizon-dev-portal/horizon-dev-portal")
      platform       = "linux/amd64"
    }
    "module-manager-app" = {
      directory      = "module-manager"
      build_version  = "0.3.7"
      deploy_version = "0.3.7"
    }
    "workflow-namespace-drain-app" = {
      directory      = "workflow-namespace-drain"
      build_version  = "1.0.1"
      deploy_version = "1.0.1"
    }
    "horizon-api-app" = {
      directory      = "horizon-api"
      build_version  = "1.1.1"
      deploy_version = "1.1.1"
    }
    "kcc-webhook-cert-monitor" = {
      directory      = "kcc-webhook-cert-monitor"
      build_version  = "1.0.0"
      deploy_version = "1.0.0"
      platform       = "linux/amd64"
    }
  }

  # Merge Main + Sub-Envs into one map (for certificate manager domains)
  cert_domains = merge(
    { main = "${var.env_name}.${var.domain_name}" },
    { for env in var.sdv_sub_environments : env => "${env}.${var.env_name}.${var.domain_name}" }
  )
}