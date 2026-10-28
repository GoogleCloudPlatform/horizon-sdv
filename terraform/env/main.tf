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
#
# Description:
# Main configuration file contains GCP project details such as
# project ID, region, zone, network etc. Set up service accounts and
# the required secrets.

# Convert GitHub App private key to PKCS#8 format (only when using app auth)
data "external" "pkcs8_converter" {

  program = ["bash", "-c", "jq -r .key | openssl pkcs8 -topk8 -inform PEM -outform PEM -nocrypt | jq -Rs '{result: .}'"]

  query = {
    key = var.sdv_github_app_private_key
  }
}

resource "random_password" "pw" {
  for_each = {
    for k in local.ids_to_generate : k => local.secret_password_specs[k]
  }

  length      = each.value.length
  min_lower   = each.value.min_lower
  min_upper   = each.value.min_upper
  min_numeric = each.value.min_numeric
  min_special = each.value.min_special

  keepers = {
    rotation_trigger = contains(var.force_update_secret_ids, each.key) ? timestamp() : "static"
  }
}

# Generate SSH key pair for cuttlefish_vm
module "cuttlefish_key" {
  source             = "../modules/sdv-ssh-keypair"
  name               = "my_cuttlefish_vm_ssh_key"
  dir                = "./cuttlefish_vm_keys"
  algorithm          = "ED25519"
  write_files        = true
  convert_to_openssh = true
}

# Generate SSH key pair for gerrit_admin
module "gerrit_admin_key" {
  source             = "../modules/sdv-ssh-keypair"
  name               = "my_gerrit_admin_ssh_key"
  dir                = "./gerrit_admin_keys"
  algorithm          = "ECDSA"
  ecdsa_curve        = "P521" # ssh-keygen -t ecdsa -b 521
  write_files        = true
  convert_to_openssh = true
}

# --- Generate random passwords for SUB-ENVIRONMENTS ---
resource "random_password" "sub_env_pw" {
  for_each = toset(nonsensitive(local.sub_env_password_keys))

  length      = local.secret_password_specs[split("_", each.key)[1]].length
  min_lower   = local.secret_password_specs[split("_", each.key)[1]].min_lower
  min_upper   = local.secret_password_specs[split("_", each.key)[1]].min_upper
  min_numeric = local.secret_password_specs[split("_", each.key)[1]].min_numeric
  min_special = local.secret_password_specs[split("_", each.key)[1]].min_special

  keepers = {
    rotation_trigger = contains(var.force_update_secret_ids, each.key) ? timestamp() : "static"
  }
}

# Generate Cuttlefish SSH keys for each sub-environment
module "cuttlefish_key_subenv" {
  for_each = toset(nonsensitive(keys(var.sdv_sub_env_configs)))

  source             = "../modules/sdv-ssh-keypair"
  name               = "${each.key}_cuttlefish_vm_ssh_key"
  dir                = "./cuttlefish_vm_keys/${each.key}"
  algorithm          = "RSA"
  rsa_bits           = 4096
  write_files        = true
  convert_to_openssh = true
}

# Generate Gerrit SSH keys for each sub-environment
module "gerrit_admin_key_subenv" {
  for_each = toset(nonsensitive(keys(var.sdv_sub_env_configs)))

  source             = "../modules/sdv-ssh-keypair"
  name               = "${each.key}_gerrit_admin_ssh_key"
  dir                = "./gerrit_admin_keys/${each.key}"
  algorithm          = "ECDSA"
  ecdsa_curve        = "P521"
  write_files        = true
  convert_to_openssh = true
}

# Validate SCM configuration
resource "terraform_data" "validate_scm_config" {
  lifecycle {
    # GitHub App requires scm_type = "github"
    precondition {
      condition     = var.scm_auth_method != "app" || (var.scm_auth_method == "app" && var.scm_type == "github")
      error_message = "GitHub App authentication (scm_auth_method='app') can only be used with scm_type='github'."
    }
  }
}

module "base" {
  source = "../modules/base"

  # SCM configuration
  scm_type        = var.scm_type
  scm_auth_method = var.scm_auth_method
  scm_repo_url    = var.scm_repo_url
  scm_repo_branch = var.scm_repo_branch
  scm_repo_owner  = local.scm_repo_owner
  scm_repo_name   = local.scm_repo_name
  scm_username    = var.scm_username

  # The project is used by provider.tf to define the GCP project
  sdv_project  = var.sdv_gcp_project_id
  sdv_location = var.sdv_gcp_region
  sdv_region   = var.sdv_gcp_region
  sdv_zone     = var.sdv_gcp_zone

  sdv_network    = "sdv-network"
  sdv_subnetwork = "sdv-subnet"

  sdv_gcp_compute_sa_email = var.sdv_gcp_compute_sa_email

  sdv_list_of_apis = toset([
    "compute.googleapis.com",
    "dns.googleapis.com",
    "oslogin.googleapis.com",
    "monitoring.googleapis.com",
    "secretmanager.googleapis.com",
    "iamcredentials.googleapis.com",
    "container.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "autoscaling.googleapis.com",
    "iam.googleapis.com",
    "certificatemanager.googleapis.com",
    "file.googleapis.com",
    "sts.googleapis.com",
    "artifactregistry.googleapis.com",
    "iap.googleapis.com",
    "serviceusage.googleapis.com",
    "networkconnectivity.googleapis.com",
    "networkmanagement.googleapis.com",
    "integrations.googleapis.com",
    "aiplatform.googleapis.com",
    "storage.googleapis.com",
    "workstations.googleapis.com",
    "spanner.googleapis.com",
    "gkehub.googleapis.com",
    "parametermanager.googleapis.com",
    "connectgateway.googleapis.com"
  ])

  sdv_cluster_name                   = "sdv-cluster"
  sdv_cluster_node_pool_name         = "sdv-node-pool"
  sdv_cluster_node_pool_machine_type = "n1-standard-4"
  sdv_cluster_node_pool_count        = 3
  sdv_cluster_node_locations = [
    "${var.sdv_gcp_zone}"
  ]

  sdv_build_node_pool_machine_type   = "c2d-highcpu-112"
  sdv_build_node_pool_max_node_count = 20

  sdv_openbsw_build_node_pool_machine_type   = "c2d-highcpu-8"
  sdv_openbsw_build_node_pool_max_node_count = 20

  # Gemini AI Assistant Jenkins job / Vertex workloads: 32 CPU / 96Gi limits; n2-standard-32 allocatable CPU is often < 32 cores after kube-reserved, so n2-standard-48 fits reliably.
  sdv_utility_node_pool_machine_type   = "n2-standard-48"
  sdv_utility_node_pool_max_node_count = 10

  sdv_abfs_build_node_pool_version = var.sdv_abfs_build_node_pool_version
  sdv_cluster_version              = var.sdv_cluster_version
  sdv_cluster_release_channel      = var.sdv_cluster_release_channel

  sdv_cluster_maintenance_recurring_window_start_time = var.sdv_cluster_maintenance_recurring_window_start_time
  sdv_cluster_maintenance_recurring_window_end_time   = var.sdv_cluster_maintenance_recurring_window_end_time
  sdv_cluster_maintenance_recurring_window_recurrence = var.sdv_cluster_maintenance_recurring_window_recurrence
  sdv_cluster_maintenance_exclusions                  = var.sdv_cluster_maintenance_exclusions

  env_name                = var.sdv_env_name
  domain_name             = var.sdv_root_domain
  gcp_backend_bucket_name = var.sdv_gcp_backend_bucket

  sdv_network_egress_router_name = "sdv-egress-internet"

  sdv_artifact_registry_repository_id      = "horizon-sdv"
  sdv_artifact_registry_repository_members = []
  sdv_artifact_registry_repository_reader_members = [
    "serviceAccount:${var.sdv_gcp_compute_sa_email}",
  ]

  sdv_ssl_certificate_name   = "horizon-sdv"
  sdv_ssl_certificate_domain = "${var.sdv_env_name}.${var.sdv_root_domain}"
  sdv_sub_environments       = nonsensitive(keys(var.sdv_sub_env_configs))
  sdv_sub_env_branches = {
    for env, config in var.sdv_sub_env_configs :
    nonsensitive(env) => coalesce(config.branch, var.scm_repo_branch)
  }

  #
  # To create a new SA with access from GKE to GC, add a new saN block.
  # external-dns-sa (sa12) is only provisioned when dynamic DNS is used
  # (sdv_dns_use_static_a_records = false); with static A records external-dns is not deployed.
  #
  sdv_wi_service_accounts = merge(
    {
      sa1 = {
        account_id   = "gke-jenkins-sa"
        display_name = "jenkins SA"
        description  = "the deployment of jenkins in GKE cluster makes use of this account through WIF"

        gke_sas = [
          {
            gke_ns = "jenkins"
            gke_sa = "jenkins-sa"
          },
          {
            gke_ns = "jenkins"
            gke_sa = "jenkins"
          }
        ]

        roles = toset([
          "roles/storage.objectUser",
          "roles/artifactregistry.writer",
          "roles/secretmanager.secretAccessor",
          "roles/iam.serviceAccountTokenCreator",
          "roles/container.admin",
          "roles/iap.tunnelResourceAccessor",
          "roles/iam.serviceAccountUser",
          "roles/compute.instanceAdmin.v1",
          "roles/workstations.admin",
          "roles/storage.bucketViewer",
          "roles/spanner.admin",
          "roles/logging.admin",
          "roles/editor",
          "roles/iam.serviceAccountAdmin",
          "roles/resourcemanager.projectIamAdmin",
          "roles/aiplatform.user"
        ])
      },
      sa2 = {
        account_id   = "gke-argocd-sa"
        display_name = "gke-argocd SA"
        description  = "argocd/argocd-sa in GKE cluster makes use of this account through WI"

        gke_sas = [
          {
            gke_ns = "argocd"
            gke_sa = "argocd-sa"
          }
        ]
        roles = toset([
          "roles/secretmanager.secretAccessor",
          "roles/iam.serviceAccountTokenCreator",
        ])
      },
      sa3 = {
        account_id   = "gke-keycloak-sa"
        display_name = "keycloak SA"
        description  = "keycloak/keycloak-sa in GKE cluster makes use of this account through WI"

        gke_sas = [
          {
            gke_ns = "keycloak"
            gke_sa = "keycloak-sa"
          }
        ]

        roles = toset([
          "roles/secretmanager.secretAccessor",
          "roles/iam.serviceAccountTokenCreator",
        ])
      },
      sa4 = {
        account_id   = "gke-gerrit-sa"
        display_name = "gke-gerrit SA"
        description  = "gerrit/gerrit-sa in GKE cluster makes use of this account through WI"

        gke_sas = [
          {
            gke_ns = "gerrit"
            gke_sa = "gerrit-sa"
          }
        ]

        roles = toset([
          "roles/secretmanager.secretAccessor",
          "roles/iam.serviceAccountTokenCreator",
        ])
      },
      sa5 = {
        account_id   = "monitoring-sa"
        display_name = "monitoring-sa"
        description  = "monitoring-sa/monitoring-sa in GKE cluster makes use of this account through WI"

        gke_sas = [
          {
            gke_ns = "monitoring"
            gke_sa = "monitoring-sa"
          }
        ]

        roles = toset([
          "roles/iam.workloadIdentityUser",
          "roles/monitoring.viewer"
        ])
      },
      sa6 = {
        account_id   = "monitoring-writer-sa"
        display_name = "monitoring-writer-sa"
        description  = "monitoring-writer-sa/monitoring-writer-sa in GKE cluster makes use of this account through WI"

        gke_sas = [
          {
            gke_ns = "monitoring"
            gke_sa = "monitoring-writer-sa"
          }
        ]
        roles = toset([
          "roles/monitoring.metricWriter",
          "roles/monitoring.viewer",
          "roles/iam.serviceAccountTokenCreator",
          "roles/iam.serviceAccountUser",
          "roles/iam.workloadIdentityUser"
        ])
      },
      sa7 = {
        account_id   = "gke-tf-wl-sa"
        display_name = "terraform-workloads-sa"
        description  = "jenkins/terraform-workloads-sa in GKE cluster makes use of this account through WI to deploy extra on-demand resources via workload pipelines"

        gke_sas = [
          {
            gke_ns = "jenkins"
            gke_sa = "terraform-workloads-sa"
          }
        ]

        roles = toset([
          "roles/storage.objectUser",
          "roles/artifactregistry.writer",
          "roles/secretmanager.secretAccessor",
          "roles/iam.serviceAccountTokenCreator",
          "roles/container.admin",
          "roles/iap.tunnelResourceAccessor",
          "roles/iam.serviceAccountUser",
          "roles/compute.instanceAdmin.v1",
          "roles/workstations.admin",
          "roles/storage.bucketViewer",
          "roles/spanner.admin",
          "roles/logging.admin",
          "roles/editor",
          "roles/iam.serviceAccountAdmin",
          "roles/resourcemanager.projectIamAdmin",
          "roles/file.editor"
        ])
      },
      sa8 = {
        account_id   = "gke-mcp-gateway-sa"
        display_name = "mcp-gateway-registry SA"
        description  = "mcp-gateway-registry/mcp-gateway-registry-sa in GKE cluster makes use of this account through WI"

        gke_sas = [
          {
            gke_ns = "mcp-gateway-registry"
            gke_sa = "mcp-gateway-registry-sa"
          }
        ]
        roles = toset([
          "roles/secretmanager.secretAccessor",
          "roles/iam.serviceAccountTokenCreator",
        ])
      },
      sa9 = {
        account_id   = "gke-argo-workflows-sa"
        display_name = "Argo Workflows SA"
        description  = "argo-workflows namespace service accounts use this account through Workload Identity"

        gke_sas = [
          {
            gke_ns = "argo-workflows"
            gke_sa = "argo-workflows-server"
          },
          {
            gke_ns = "workflows"
            gke_sa = "workflow-executor"
          },
          # Horizon API uses this GSA for GCS V4 signed URLs (downloadArtifact); namespace matches gitops (prefix + horizon-api).
          {
            gke_ns = "horizon-api"
            gke_sa = "horizon-api"
          },
        ]
        roles = toset([
          "roles/storage.objectUser",
          "roles/secretmanager.secretAccessor",
          "roles/artifactregistry.reader",
          "roles/artifactregistry.writer",
          "roles/iam.serviceAccountTokenCreator",
        ])
      },
      sa10 = {
        account_id   = "gke-argo-workflows-elevated-sa"
        display_name = "Argo Workflows Elevated SA"
        description  = "workflows/workflow-executor-elevated in GKE cluster makes use of this account through WI for migration workloads that need Jenkins-equivalent permissions"

        gke_sas = [
          {
            gke_ns = "workflows"
            gke_sa = "workflow-executor-elevated"
          }
        ]

        roles = toset([
          "roles/storage.objectUser",
          "roles/artifactregistry.writer",
          "roles/secretmanager.secretAccessor",
          "roles/iam.serviceAccountTokenCreator",
          "roles/container.admin",
          "roles/iap.tunnelResourceAccessor",
          "roles/iam.serviceAccountUser",
          "roles/compute.instanceAdmin.v1",
          "roles/workstations.admin",
          "roles/storage.bucketViewer",
          "roles/spanner.admin",
          "roles/logging.admin",
          "roles/editor",
          "roles/iam.serviceAccountAdmin",
          "roles/resourcemanager.projectIamAdmin",
          "roles/aiplatform.user"
        ])
      },
      sa11 = {
        account_id   = "gke-config-connector-sa"
        display_name = "Config Connector Service Account"
        description  = "Service account used by Config Connector to manage GCP resources"

        gke_sas = [
          {
            gke_ns = "gcpcc"
            gke_sa = "default"
          },
          {
            gke_ns = "cnrm-system"
            gke_sa = "cnrm-controller-manager"
          },
          {
            gke_ns = "cnrm-system"
            gke_sa = "cnrm-controller-manager-gcp"
          },
          # Namespaced-mode Config Connector creates one controller ServiceAccount per
          # ConfigConnectorContext namespace (see gitops configConnectorNamespaces). Each
          # must impersonate gke-config-connector-sa or PubSubTopic and other CNRM
          # resources fail with iam.serviceAccounts.getAccessToken / Workload Identity 403.
          {
            gke_ns = "cnrm-system"
            gke_sa = "cnrm-controller-manager-sample-module-hello"
          },
          {
            gke_ns = "cnrm-system"
            gke_sa = "cnrm-controller-manager-sample-hard-module-hello"
          },
          {
            gke_ns = "cnrm-system"
            gke_sa = "cnrm-controller-manager-sample-soft-module-hello"
          },
          # Cuttlefish / workloads-android: ConfigConnectorContext in {namespacePrefix}workflows
          # (default namespace name: workflows). Without this binding, ComputeInstanceTemplate and
          # other CNRM resources fail with iam.serviceAccounts.getAccessToken / WI 403 when reconciling.
          {
            gke_ns = "cnrm-system"
            gke_sa = "cnrm-controller-manager-workflows"
          }
        ]
        roles = toset([
          "roles/editor",
          "roles/iam.serviceAccountTokenCreator"
        ])
      }
    },
    local.sub_env_service_accounts,
    var.sdv_dns_use_static_a_records ? {} : {
      sa12 = {
        account_id   = "external-dns-sa"
        display_name = "external-dns-sa"
        description  = "external-dns-sa/external-dns-sa in GKE cluster makes use of this account through WI"

        gke_sas = [
          {
            gke_ns = "external-dns"
            gke_sa = "external-dns-sa"
          }
        ]
        roles = toset([
          "roles/dns.admin"
        ])
      }
    },
    {
      sa13 = {
        account_id   = "gke-storage-gcs-module-sa"
        display_name = "Storage GCS Module (operator + REST API)"
        description  = "horizon/storage-gcs-module: GCSBucket reconciliation and GCS REST API via Workload Identity"

        gke_sas = [
          {
            gke_ns = "horizon"
            gke_sa = "storage-gcs-module"
          }
        ]

        roles = toset([])
        # SignBlob for GCS V4 signed URLs — bind on this SA only (not project-wide TokenCreator).
        sa_roles = toset([
          "roles/iam.serviceAccountTokenCreator",
        ])
        # storage.admin only on buckets/objects named {project}-* (not other project buckets).
        # storage.buckets.create is granted separately (IAM conditions do not apply to create).
        conditional_roles = [
          {
            role        = "roles/storage.admin"
            title       = "project_prefixed_gcs_buckets"
            description = "Admin only for buckets named {project}-*"
            expression  = "resource.name.startsWith(\"projects/_/buckets/${var.sdv_gcp_project_id}-\")"
          }
        ]
      }
    }
  )

  # Define the secrets and values and gke access rules

  sdv_gcp_secrets_map = merge(
    local.sdv_gcp_common_secrets_map,
    var.scm_auth_method == "app" ? local.sdv_gcp_github_app_secrets_map : local.sdv_gcp_userpass_secrets_map,
    local.sub_env_secrets,
    local.sub_env_git_secrets
  )

  #sdv_gcp_secrets_map = local.sdv_gcp_secrets_map


  sdv_gcp_parameters_map = {
    p1 = {
      parameter_id         = "sdv_environment"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_env_name)
    }
    p2 = {
      parameter_id         = "sdv_root_domain"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_root_domain)
    }
    p3 = {
      parameter_id         = "sdv_project_id"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_gcp_project_id)
    }
    p4 = {
      parameter_id         = "sdv_gcp_region"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_gcp_region)
    }
    p5 = {
      parameter_id         = "sdv_gcp_zone"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_gcp_zone)
    }
    p6 = {
      parameter_id         = "sdv_gcp_compute_sa_email"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_gcp_compute_sa_email)
    }
    p7 = {
      parameter_id         = "sdv_gcp_backend_bucket"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_gcp_backend_bucket)
    }
    p8 = {
      parameter_id         = "scm_repo_name"
      parameter_version_id = "v1"
      value                = base64encode(local.scm_repo_name)
    }
    p9 = {
      parameter_id         = "scm_repo_branch"
      parameter_version_id = "v1"
      value                = base64encode(var.scm_repo_branch)
    }
    p10 = {
      parameter_id         = "sdv_git_auth_method"
      parameter_version_id = "v1"
      value                = base64encode(var.scm_auth_method)
    }
    p11 = {
      parameter_id         = "sdv_sub_environments"
      parameter_version_id = "v1"
      value                = base64encode(jsonencode(nonsensitive(keys(var.sdv_sub_env_configs))))
    }
  }

  # ARM64 networking (Cuttlefish bare metal; region/zone independent of GKE)
  enable_arm64_dedicated_subnet       = var.enable_arm64_dedicated_subnet
  arm64_region                        = var.arm64_region
  arm64_zone                          = var.arm64_zone
  arm64_subnetwork                    = var.arm64_subnetwork
  arm64_pods_secondary_range_name     = var.arm64_pods_secondary_range_name
  arm64_services_secondary_range_name = var.arm64_services_secondary_range_name

  # Network policies configuration
  sdv_enable_network_policies = var.sdv_enable_network_policies
  # KMS encryption for GKE secrets (optional)
  sdv_enable_kms_encryption = var.sdv_enable_kms_encryption
  # DNSSEC configuration for Cloud DNS
  sdv_dns_dnssec_enabled = var.sdv_dns_dnssec_enabled
  # Static A records: no zone delegation, LB cert auth; add A records to parent zone manually
  sdv_dns_use_static_a_records = var.sdv_dns_use_static_a_records
}
