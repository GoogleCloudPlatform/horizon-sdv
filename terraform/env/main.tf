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
  algorithm          = "RSA"
  rsa_bits           = 4096
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

# Validate GitHub secrets
resource "terraform_data" "validate_github_auth" {
  lifecycle {
    precondition {
      condition     = var.github_auth_method != "pat" || (var.github_auth_method == "pat" && length(var.sdv_github_pat) > 0 && var.sdv_github_pat != "<OPTIONAL>")
      error_message = "Selected 'pat' auth but 'sdv_github_pat' is empty or invalid."
    }

    precondition {
      condition     = var.github_auth_method != "app" || (var.github_auth_method == "app" && length(var.sdv_github_app_id) > 0 && var.sdv_github_app_id != "<OPTIONAL>")
      error_message = "Selected 'app' auth but 'sdv_github_app_id' is empty or invalid."
    }

    precondition {
      condition     = var.github_auth_method != "app" || (var.github_auth_method == "app" && length(var.sdv_github_app_install_id) > 0 && var.sdv_github_app_install_id != "<OPTIONAL>")
      error_message = "Selected 'app' auth but 'sdv_github_app_install_id' is empty or invalid."
    }

    precondition {
      condition = var.github_auth_method != "app" || (
        var.github_auth_method == "app" &&
        length(var.sdv_github_app_private_key) > 50 &&
        !can(regex("paste content here", var.sdv_github_app_private_key))
      )
      error_message = "Selected 'app' auth but 'sdv_github_app_private_key' set to default sample. Replace it with your actual private key."
    }

    precondition {
      condition = var.github_auth_method != "app" || (
        var.github_auth_method == "app" &&
        length(var.sdv_github_app_private_key_pkcs8) > 50 &&
        !can(regex("paste content here", var.sdv_github_app_private_key_pkcs8))
      )
      error_message = "Selected 'app' auth but 'sdv_github_app_private_key_pkcs8' set to default sample. Jenkins requires correct key format."
    }
  }
}

module "base" {
  source = "../modules/base"

  github_repo_owner  = var.sdv_github_repo_owner
  github_repo_name   = var.sdv_github_repo_name
  github_auth_method = var.github_auth_method

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

  github_env_name         = var.sdv_env_name
  github_domain_name      = var.sdv_root_domain
  github_repo_branch      = var.sdv_github_repo_branch
  gcp_backend_bucket_name = var.sdv_gcp_backend_bucket

  sdv_network_egress_router_name = "sdv-egress-internet"

  sdv_artifact_registry_repository_id      = "horizon-sdv"
  sdv_artifact_registry_repository_members = []
  sdv_artifact_registry_repository_reader_members = [
    "serviceAccount:${var.sdv_gcp_compute_sa_email}",
  ]

  sdv_ssl_certificate_name   = "horizon-sdv"
  sdv_ssl_certificate_domain = "${var.sdv_env_name}.${var.sdv_root_domain}"

  sdv_abfs_license_key_b64 = var.sdv_abfs_license_key_b64
  #
  # To create a new SA with access from GKE to GC, add a new saN block.
  #
  sdv_wi_service_accounts = {
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
        "roles/resourcemanager.projectIamAdmin"
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
      account_id   = "gke-terraform-workloads-sa"
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
      account_id   = "external-dns-sa"
      display_name = "external-dns-sa"
      description  = "external-dns-sa/external-dsn-sa in GKE cluster makes use of this account through WI"

      gke_sas = [
        {
          gke_ns = "external-dns"
          gke_sa = "external-dns-sa"
        }
      ]
      roles = toset([
        "roles/dns.admin"
      ])
    },
    sa9 = {
      account_id   = "gke-mcp-gateway-registry-sa"
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
    }
  }

  #
  # Define the secrets and values and gke access rules
  sdv_gcp_secrets_map = merge(
    local.sdv_gcp_common_secrets_map,
    var.github_auth_method == "app" ? local.sdv_gcp_github_app_secrets_map : local.sdv_gcp_github_pat_secrets_map
  )

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
      parameter_id         = "sdv_github_repo_name"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_github_repo_name)
    }
    p9 = {
      parameter_id         = "sdv_github_repo_branch"
      parameter_version_id = "v1"
      value                = base64encode(var.sdv_github_repo_branch)
    }
    p10 = {
      parameter_id         = "sdv_github_auth_method"
      parameter_version_id = "v1"
      value                = base64encode(var.github_auth_method)
    }
  }

  #ARM64_ENABLEMENT
  enable_arm64 = var.enable_arm64
}