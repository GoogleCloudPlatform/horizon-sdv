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
  # Cuttlefish ARM64 placement for GitOps/Jenkins/Argo (always published). When enable_arm64_dedicated_subnet is false,
  # metal uses the primary platform region/zone/subnet; when true, uses dedicated arm64_* tfvars.
  arm64_placement_region     = var.enable_arm64_dedicated_subnet ? var.arm64_region : var.gcp_cloud_region
  arm64_placement_zone       = var.enable_arm64_dedicated_subnet ? var.arm64_zone : var.gcp_cloud_zone
  arm64_placement_subnetwork = var.enable_arm64_dedicated_subnet ? var.arm64_subnetwork : var.primary_subnetwork

  all_environments = merge(
    {
      "main" = {
        namespace_prefix = ""
        argocd_namespace = var.argocd_namespace
        subdomain        = var.subdomain_name
        is_main          = true
        env_name         = "main"
        branch           = var.scm_repo_branch
      }
    },
    {
      for env in var.sub_environments : env => {
        namespace_prefix = "${env}-"
        argocd_namespace = "${env}-argocd"
        subdomain        = "${env}.${var.subdomain_name}"
        is_main          = false
        env_name         = env
        branch           = lookup(var.sub_env_branches, env, var.scm_repo_branch)
      }
    }
  )
}

# TAA-1571: State migration blocks for 3.0.0 -> 3.1.0 upgrade.
# Remove after all environments have been upgraded.
moved {
  from = kubernetes_namespace.argocd
  to   = kubernetes_namespace.argocd["main"]
}
moved {
  from = kubernetes_service_account.argocd_sa
  to   = kubernetes_service_account.argocd_sa["main"]
}
moved {
  from = kubernetes_secret.argocd_secret
  to   = kubernetes_secret.argocd_secret["main"]
}
moved {
  from = helm_release.argocd
  to   = helm_release.argocd_main
}
moved {
  from = kubectl_manifest.argocd_secret_store
  to   = kubectl_manifest.argocd_secret_store["main"]
}
moved {
  from = kubectl_manifest.es_argocd_secret
  to   = kubectl_manifest.es_argocd_secret["main"]
}
moved {
  from = kubectl_manifest.argocd_appproject
  to   = kubectl_manifest.argocd_appproject["main"]
}
moved {
  from = kubectl_manifest.argocd_application
  to   = kubectl_manifest.argocd_application["main"]
}

# Create Argo CD namespace for each environment
resource "kubernetes_namespace" "argocd" {
  for_each = local.all_environments

  metadata {
    name = each.value.argocd_namespace
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

# Create the Service Account for Argo CD on each environment
resource "kubernetes_service_account" "argocd_sa" {
  for_each = local.all_environments

  metadata {
    name      = "argocd-sa"
    namespace = kubernetes_namespace.argocd[each.key].metadata[0].name
    annotations = {
      "iam.gke.io/gcp-service-account" = each.value.is_main ? "gke-argocd-sa@${var.gcp_project_id}.iam.gserviceaccount.com" : "gke-${each.value.env_name}-argocd-sa@${var.gcp_project_id}.iam.gserviceaccount.com"
    }
  }

  depends_on = [
    kubernetes_namespace.argocd
  ]
}

# Create the SCM credentials secret for each environment
resource "kubernetes_secret" "argocd_scm_creds" {
  for_each = local.all_environments

  metadata {
    name      = "argocd-scm-creds"
    namespace = kubernetes_namespace.argocd[each.key].metadata[0].name
    labels = {
      "argocd.argoproj.io/secret-type" = "repository"
    }
  }

  data = {
    "url"      = var.scm_repo_url
    "type"     = "git"
    "username" = var.scm_auth_method == "userpass" ? var.scm_username : null
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

# Create the empty Argo CD admin secret for each environment
resource "kubernetes_secret" "argocd_secret" {
  for_each = local.all_environments

  metadata {
    name      = "argocd-secret"
    namespace = kubernetes_namespace.argocd[each.key].metadata[0].name
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

# Deploy Argo CD - Main environment first
resource "helm_release" "argocd_main" {
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
    kubernetes_secret.argocd_scm_creds,
    kubernetes_secret.argocd_secret
  ]
}

# Deploy Argo CD for sub-environments
resource "helm_release" "argocd_subenvs" {
  for_each = { for k, v in local.all_environments : k => v if !v.is_main }

  name       = "${each.key}-argocd"
  chart      = "argo-cd"
  repository = "https://argoproj.github.io/argo-helm"
  version    = var.argocd_chart_version
  namespace  = each.value.argocd_namespace

  create_namespace = false
  wait             = true
  skip_crds        = true # CRDs already installed by main

  values = [
    templatefile("${path.module}/argocd-values.yaml.tpl", {
      subdomain_name = each.value.subdomain
      domain_name    = var.domain_name
    }),
    yamlencode({
      crds = {
        install = false
      }
      global = {
        rbac = {
          create = true
        }
      }
    })
  ]

  depends_on = [
    helm_release.argocd_main,
    kubernetes_service_account.argocd_sa,
    kubernetes_secret.argocd_scm_creds,
    kubernetes_secret.argocd_secret
  ]
}

# Deploy workflow namespace drain outside the ArgoCD cascade so it survives platform destroy.
resource "helm_release" "workflow_namespace_drain" {
  for_each = local.all_environments

  name             = "${each.value.namespace_prefix}workflow-namespace-drain"
  chart            = "${path.module}/../../../gitops/apps/workflow-namespace-drain"
  namespace        = "${each.value.namespace_prefix}workflow-namespace-drain"
  create_namespace = true
  wait             = true

  values = [
    yamlencode({
      image              = "${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/workflow-namespace-drain-app:${var.images["workflow-namespace-drain-app"].version}"
      namespace          = "${each.value.namespace_prefix}workflow-namespace-drain"
      argocd             = { namespace = each.value.argocd_namespace }
      workflowsNamespace = "${each.value.namespace_prefix}workflows"
      gitopsRootAppName  = "${each.value.namespace_prefix}${var.argocd_application_name}"
    })
  ]

  depends_on = [
    helm_release.argocd_main,
    helm_release.argocd_subenvs
  ]
}

# GCS project buckets and cluster Config Connector are GitOps-managed in gitops/modules/storage-gcs-module (KCC + GCSBucket CRs).

# Create SecretStore for each environment
resource "kubectl_manifest" "argocd_secret_store" {
  for_each = local.all_environments

  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: external-secrets.io/v1beta1
    kind: SecretStore
    metadata:
      name: argocd-secret-store
      namespace: ${kubernetes_namespace.argocd[each.key].metadata[0].name}
    spec:
      provider:
        gcpsm:
          projectID: "${var.gcp_project_id}"
          auth:
            workloadIdentity:
              clusterLocation: ${var.gcp_cloud_region}
              clusterName: ${var.sdv_cluster_name}
              serviceAccountRef:
                name: ${kubernetes_service_account.argocd_sa[each.key].metadata[0].name}
  EOT

  depends_on = [
    helm_release.external_secrets,
    kubernetes_service_account.argocd_sa,
    helm_release.argocd_main,
    helm_release.argocd_subenvs
  ]
}

# Create ExternalSecret for Git creds for each environment
resource "kubectl_manifest" "es_scm_creds" {
  for_each = var.scm_auth_method != "none" ? local.all_environments : {}

  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: external-secrets.io/v1beta1
    kind: ExternalSecret
    metadata:
      name: argocd-scm-creds
      namespace: ${kubernetes_namespace.argocd[each.key].metadata[0].name}
    spec:
      refreshInterval: 10s
      secretStoreRef:
        kind: SecretStore
        name: argocd-secret-store
      target:
        name: ${kubernetes_secret.argocd_scm_creds[each.key].metadata[0].name}
        creationPolicy: Merge
      data:
      %{if var.scm_auth_method == "app"}
      - secretKey: githubAppID
        remoteRef:
          key: ${each.value.namespace_prefix}github-app-id-b64
          decodingStrategy: Base64
      - secretKey: githubAppInstallationID
        remoteRef:
          key: ${each.value.namespace_prefix}github-app-installation-id-b64
          decodingStrategy: Base64
      - secretKey: githubAppPrivateKey
        remoteRef:
          key: ${each.value.namespace_prefix}github-app-private-key-b64
          decodingStrategy: Base64
      %{else}
      - secretKey: password
        remoteRef:
          key: ${each.value.namespace_prefix}scm-password-b64
          decodingStrategy: Base64
      %{endif}
  EOT

  depends_on = [
    kubectl_manifest.argocd_secret_store,
    kubernetes_secret.argocd_scm_creds
  ]
}


# Create ExternalSecret for ArgoCD admin password for each environment
resource "kubectl_manifest" "es_argocd_secret" {
  for_each = local.all_environments

  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: external-secrets.io/v1beta1
    kind: ExternalSecret
    metadata:
      name: argocd-secret
      namespace: ${kubernetes_namespace.argocd[each.key].metadata[0].name}
    spec:
      refreshInterval: 10s
      secretStoreRef:
        kind: SecretStore
        name: argocd-secret-store
      target:
        name: ${kubernetes_secret.argocd_secret[each.key].metadata[0].name}
        creationPolicy: Merge
      data:
      - secretKey: admin.password
        remoteRef:
          key: ${each.value.namespace_prefix}argocd-admin-password-b64
          decodingStrategy: Base64
  EOT

  depends_on = [
    kubectl_manifest.argocd_secret_store,
    kubernetes_secret.argocd_secret
  ]
}

# Create AppProject for each environment
resource "kubectl_manifest" "argocd_appproject" {
  for_each = local.all_environments

  validate_schema = false

  yaml_body = <<-EOT
    apiVersion: argoproj.io/v1alpha1
    kind: AppProject
    metadata:
      name: "${each.value.namespace_prefix}${var.argocd_application_name}"
      namespace: ${kubernetes_namespace.argocd[each.key].metadata[0].name}
    spec:
      description: "${each.value.is_main ? "Main Environment" : "Sub-Environment ${each.value.env_name}"}"
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
    helm_release.argocd_main,
    helm_release.argocd_subenvs
  ]
}

# Create Application for each environment
resource "kubectl_manifest" "argocd_application" {
  for_each = local.all_environments

  validate_schema = false
  wait            = true
  # Avoid "metadata.resourceVersion: Invalid value: 0x0: must be specified for an update" on
  # Application CRD updates (e.g. after out-of-band kubectl apply) by using server-side apply.
  server_side_apply = true
  # Argo CD (argocd-server) also updates spec.source.targetRevision; allow Terraform to set scm_repo_branch.
  force_conflicts = true

  yaml_body = <<-EOT
    apiVersion: argoproj.io/v1alpha1
    kind: Application
    metadata:
      name: "${each.value.namespace_prefix}${var.argocd_application_name}"
      namespace: ${kubernetes_namespace.argocd[each.key].metadata[0].name}
      finalizers:
        - resources-finalizer.argocd.argoproj.io
        - horizon-sdv.io/module-manager-platform-drain
        - horizon-sdv.io/workflow-namespace-drain
    spec:
      project: "${each.value.namespace_prefix}${var.argocd_application_name}"
      source:
        repoURL: ${var.scm_repo_url}
        path: gitops
        targetRevision: ${each.value.branch}
        helm:
          values: |
            scm:
              type: ${var.scm_type}
              authMethod: ${var.scm_auth_method}
              username: ${var.scm_username}
              repoUrl: ${var.scm_repo_url}
              branch: ${var.scm_repo_branch}
              %{if var.scm_type == "github"}repoOwner: ${var.scm_repo_owner}
              repoName: ${var.scm_repo_name}
              %{endif}
            config:
              domain: ${each.value.subdomain}.${var.domain_name}
              projectID: ${var.gcp_project_id}
              region: ${var.gcp_cloud_region}
              zone: ${var.gcp_cloud_zone}
              backendBucket: ${var.gcp_backend_bucket}
              namespacePrefix: "${each.value.namespace_prefix}"
              isSubEnvironment: ${!each.value.is_main}
              environmentName: "${each.value.env_name}"
              enableNetworkPolicies: ${var.enable_network_policies}
              storageLocation: ${var.gcp_storage_location}
              probeSourceCIDRs:
                - ${var.nodes_range}
%{if var.enable_arm64_dedicated_subnet~}
                - ${var.arm64_nodes_range}
%{endif~}
              useStaticDnsARecords: ${var.use_static_dns_a_records}
              arm64:
                region: ${local.arm64_placement_region}
                zone: ${local.arm64_placement_zone}
                subnetwork: ${local.arm64_placement_subnetwork}
              nginx:
                image: nginx
                tag: "${var.common_nginx_version}"
              containerImages:
                kccWebhookCertMonitor: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/kcc-webhook-cert-monitor:${var.images["kcc-webhook-cert-monitor"].version}
                horizondevelopmentportal: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/horizon-dev-portal:${var.images["horizon-dev-portal"].version}
                gerritMcpServer: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/gerrit-mcp-server-app:${var.images["gerrit-mcp-server-app"].version}
                moduleManager: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/module-manager-app:${var.images["module-manager-app"].version}
                storageGcsModule: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/storage-gcs-module-app:${var.images["storage-gcs-module-app"].version}
                horizonApi: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/horizon-api-app:${var.images["horizon-api-app"].version}
              apps:
                kccWebhookCertMonitor: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/kcc-webhook-cert-monitor:${var.images["kcc-webhook-cert-monitor"].version}
                horizonPortal: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/horizon-dev-portal:${var.images["horizon-dev-portal"].version}
                horizondevelopmentportal: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/horizon-dev-portal:${var.images["horizon-dev-portal"].version}
                gerritMcpServer: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/gerrit-mcp-server-app:${var.images["gerrit-mcp-server-app"].version}
                moduleManager: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/module-manager-app:${var.images["module-manager-app"].version}
                horizonApi: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/horizon-api-app:${var.images["horizon-api-app"].version}
              postjobs:
                keycloak: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post:${var.images["keycloak-post"].version}
                keycloakmtkconnect: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-mtk-connect:${var.images["keycloak-post-mtk-connect"].version}
                keycloakjenkins: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-jenkins:${var.images["keycloak-post-jenkins"].version}
                keycloakargocd: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-argocd:${var.images["keycloak-post-argocd"].version}
                keycloakheadlamp: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-headlamp:${var.images["keycloak-post-headlamp"].version}
                keycloakgerrit: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-gerrit:${var.images["keycloak-post-gerrit"].version}
                keycloakgrafana: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-grafana:${var.images["keycloak-post-grafana"].version}
                keycloakMcpGatewayRegistry: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-mcp-gateway-registry:${var.images["keycloak-post-mcp-gateway-registry"].version}
                keycloakArgoWorkflows: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-argo-workflows:${var.images["keycloak-post-argo-workflows"].version}
                keycloakhorizonapi: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/keycloak-post-horizon-api:${var.images["keycloak-post-horizon-api"].version}
                mtkconnect: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/mtk-connect-post:${var.images["mtk-connect-post"].version}
                mtkconnectkey: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/mtk-connect-post-key:${var.images["mtk-connect-post-key"].version}
                grafana: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/grafana-post:${var.images["grafana-post"].version}
                gerrit: ${var.gcp_cloud_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/gerrit-post:${var.images["gerrit-post"].version}
              workloads:
                android:
                  url: ${var.scm_repo_url}
                  branch: ${var.scm_repo_branch}
            spec:
              source:
                repoURL: ${var.scm_repo_url}
                targetRevision: ${var.scm_repo_branch}
      destination:
        server: https://kubernetes.default.svc
      revisionHistoryLimit: 1
      syncPolicy:
        syncOptions:
        - CreateNamespace=true
        automated:
          enabled: true
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
    kubectl_manifest.argocd_appproject,
    helm_release.argocd_main,
    helm_release.argocd_subenvs,
    helm_release.workflow_namespace_drain
  ]
}
