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
# Main configuration file for the "sdv-gke-apps" module.
# Create and configure required Kubernetes resources.

# Create Argo CD namespace
resource "kubernetes_namespace" "argocd" {
  metadata {
    name = var.argocd_namespace
  }

  timeouts {
    delete = "20m"
  }
}

# Deploy external secrets
resource "helm_release" "external_secrets" {
  name             = "external-secrets"
  chart            = "external-secrets"
  repository       = "https://charts.external-secrets.io"
  version          = var.es_chart_version
  namespace        = var.es_namespace
  create_namespace = true
  wait             = true
}

# Create the Service Account
resource "kubernetes_service_account" "argocd_sa" {
  metadata {
    name      = "argocd-sa"
    namespace = kubernetes_namespace.argocd.metadata[0].name
    annotations = {
      "iam.gke.io/gcp-service-account" = "gke-argocd-sa@${var.gcp_project_id}.iam.gserviceaccount.com"
    }
  }

  depends_on = [
    kubernetes_namespace.argocd
  ]
}

# Create the empty GitHub creds secret
resource "kubernetes_secret" "argocd_github_creds" {
  metadata {
    name      = "argocd-github-creds"
    namespace = kubernetes_namespace.argocd.metadata[0].name
    labels = {
      "argocd.argoproj.io/secret-type" = "repository"
    }
  }

  data = {
    "url"      = var.github_repo_url
    "type"     = "git"
    "username" = var.github_auth_method == "pat" ? "git" : null
  }

  depends_on = [
    kubernetes_namespace.argocd
  ]

  lifecycle {
    ignore_changes = [
      metadata[0].annotations,
      data
    ]
  }
}

# Create the empty Argo CD admin secret
resource "kubernetes_secret" "argocd_secret" {
  metadata {
    name      = "argocd-secret"
    namespace = kubernetes_namespace.argocd.metadata[0].name
  }

  depends_on = [
    kubernetes_namespace.argocd
  ]

  lifecycle {
    ignore_changes = [
      metadata[0].annotations,
      data
    ]
  }
}

# Deploy Argo CD
resource "helm_release" "argocd" {
  name       = "argocd"
  chart      = "argo-cd"
  repository = "https://argoproj.github.io/argo-helm"
  version    = var.argocd_chart_version
  namespace  = var.argocd_namespace

  create_namespace = false
  wait             = true

  values = [
    templatefile("${path.module}/argocd-values.yaml.tpl", {
      subdomain_name = var.subdomain_name
      domain_name    = var.domain_name
    })
  ]

  depends_on = [
    helm_release.external_secrets,
    kubernetes_service_account.argocd_sa,
    kubernetes_secret.argocd_github_creds,
    kubernetes_secret.argocd_secret
  ]
}

# Create the SecretStore
resource "kubectl_manifest" "argocd_secret_store" {
  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: external-secrets.io/v1beta1
    kind: SecretStore
    metadata:
      name: argocd-secret-store
      namespace: ${kubernetes_namespace.argocd.metadata[0].name}
    spec:
      provider:
        gcpsm:
          projectID: "${var.gcp_project_id}"
          auth:
            workloadIdentity:
              clusterLocation: ${var.gcp_cloud_region}
              clusterName: ${var.sdv_cluster_name}
              serviceAccountRef:
                name: ${kubernetes_service_account.argocd_sa.metadata[0].name}
  EOT

  depends_on = [
    helm_release.external_secrets,
    kubernetes_service_account.argocd_sa
  ]
}

# Create the ExternalSecret
resource "kubectl_manifest" "es_github_creds" {
  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: external-secrets.io/v1beta1
    kind: ExternalSecret
    metadata:
      name: argocd-github-creds
      namespace: ${kubernetes_namespace.argocd.metadata[0].name}
    spec:
      refreshInterval: 10s
      secretStoreRef:
        kind: SecretStore
        name: ${kubectl_manifest.argocd_secret_store.name}
      target:
        name: ${kubernetes_secret.argocd_github_creds.metadata[0].name}
        creationPolicy: Merge
      data:
      %{if var.github_auth_method == "app"}
      - secretKey: githubAppID
        remoteRef:
          key: github-app-id-b64
          decodingStrategy: Base64
      - secretKey: githubAppInstallationID
        remoteRef:
          key: github-app-installation-id-b64
          decodingStrategy: Base64
      - secretKey: githubAppPrivateKey
        remoteRef:
          key: github-app-private-key-b64
          decodingStrategy: Base64
      %{else}
      - secretKey: password
        remoteRef:
          key: github-pat-b64
          decodingStrategy: Base64
      %{endif}
  EOT

  depends_on = [
    kubectl_manifest.argocd_secret_store,
    kubernetes_secret.argocd_github_creds
  ]
}

# Create the ExternalSecret for the Argo CD
resource "kubectl_manifest" "es_argocd_secret" {
  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: external-secrets.io/v1beta1
    kind: ExternalSecret
    metadata:
      name: argocd-secret
      namespace: ${kubernetes_namespace.argocd.metadata[0].name}
    spec:
      refreshInterval: 10s
      secretStoreRef:
        kind: SecretStore
        name: ${kubectl_manifest.argocd_secret_store.name}
      target:
        name: ${kubernetes_secret.argocd_secret.metadata[0].name}
        creationPolicy: Merge
      data:
      - secretKey: admin.password
        remoteRef:
          key: argocd-admin-password-b64
          decodingStrategy: Base64
  EOT

  depends_on = [
    kubectl_manifest.argocd_secret_store,
    kubernetes_secret.argocd_secret
  ]
}

# Apply the Argo CD AppProject
resource "kubectl_manifest" "argocd_appproject" {
  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: argoproj.io/v1alpha1
    kind: AppProject
    metadata:
      name: ${var.argocd_application_name}
      namespace: ${kubernetes_namespace.argocd.metadata[0].name}
    spec:
      description: Horizon SDV
      sourceRepos:
      - "*"
      destinations:
      - namespace: "*"
        server: https://kubernetes.default.svc
      clusterResourceWhitelist:
      - group: "*"
        kind: "*"
      namespaceResourceWhitelist:
      - group: "*"
        kind: "*"
  EOT

  depends_on = [
    helm_release.argocd
  ]
}

# Apply the Argo CD Application
resource "kubectl_manifest" "argocd_application" {
  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: argoproj.io/v1alpha1
    kind: Application
    metadata:
      name: ${var.argocd_application_name}
      namespace: ${kubernetes_namespace.argocd.metadata[0].name}
    spec:
      project: horizon-sdv
      source:
        repoURL: ${var.github_repo_url}
        path: gitops
        targetRevision: ${var.github_repo_branch}
        helm:
          values: |
            github:
              authMethod: ${var.github_auth_method}
              username: "git"
              repoOwner: ${var.github_repo_owner}
              repoName: ${var.github_repo_name}
            config:
              domain: ${var.subdomain_name}.${var.domain_name}
              projectID: ${var.gcp_project_id}
              region: ${var.gcp_cloud_region}
              zone: ${var.gcp_cloud_zone}
              backendBucket: ${var.gcp_backend_bucket}
              apps:
                landingpage: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/landingpage-app:${var.images["landingpage-app"].version}
                gerritMcpServer: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/gerrit-mcp-server-app:${var.images["gerrit-mcp-server-app"].version}
              postjobs:
                keycloak: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post:${var.images["keycloak-post"].version}
                keycloakmtkconnect: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-mtk-connect:${var.images["keycloak-post-mtk-connect"].version}
                keycloakjenkins: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-jenkins:${var.images["keycloak-post-jenkins"].version}
                keycloakargocd: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-argocd:${var.images["keycloak-post-argocd"].version}
                keycloakheadlamp: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-headlamp:${var.images["keycloak-post-headlamp"].version}
                keycloakgerrit: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-gerrit:${var.images["keycloak-post-gerrit"].version}
                keycloakgrafana: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-grafana:${var.images["keycloak-post-grafana"].version}
                keycloakMcpGatewayRegistry: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-mcp-gateway-registry:${var.images["keycloak-post-mcp-gateway-registry"].version}
                mtkconnect: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/mtk-connect-post:${var.images["mtk-connect-post"].version}
                mtkconnectkey: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/mtk-connect-post-key:${var.images["mtk-connect-post-key"].version}
                grafana: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/grafana-post:${var.images["grafana-post"].version}
                gerrit: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/gerrit-post:${var.images["gerrit-post"].version}
              workloads:
                android:
                  url: ${var.github_repo_url}
                  branch: ${var.github_repo_branch}
            spec:
              source:
                repoURL: ${var.github_repo_url}
                targetRevision: ${var.github_repo_branch}
      path: gitops
      destination:
        server: https://kubernetes.default.svc
      revisionHistoryLimit: 1
      syncPolicy:
        syncOptions:
        - CreateNamespace=true
        automated:
          prune: true
          selfHeal: false
        retry:
          limit: 5
          backoff:
            duration: 5s
            maxDuration: 3m0s
            factor: 2
  EOT

  depends_on = [
    kubectl_manifest.argocd_appproject
  ]
}