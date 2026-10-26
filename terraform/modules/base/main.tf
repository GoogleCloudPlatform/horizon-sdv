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

data "google_project" "project" {}

module "sdv_apis" {
  source = "../sdv-apis"

  list_of_apis = var.sdv_list_of_apis
}

module "sdv_secrets" {
  source = "../sdv-secrets"

  location        = var.sdv_location
  gcp_secrets_map = var.sdv_gcp_secrets_map
  project_id      = data.google_project.project.project_id

  depends_on = [
    module.sdv_wi
  ]
}

module "sdv_parameters" {
  source = "../sdv-parameters"

  project_id     = data.google_project.project.project_id
  location       = var.sdv_location
  parameters_map = var.sdv_gcp_parameters_map

  depends_on = [
    module.sdv_wi
  ]
}

module "sdv_wi" {
  source = "../sdv-wi"

  wi_service_accounts = var.sdv_wi_service_accounts
  project_id          = data.google_project.project.project_id

  depends_on = [
    module.sdv_gke_cluster
  ]
}

# IAM conditions cannot restrict storage.buckets.create; this is create-only (no object access).
resource "google_project_iam_custom_role" "storage_gcs_bucket_creator" {
  project     = data.google_project.project.project_id
  role_id     = "horizonStorageGcsBucketCreator"
  title       = "Horizon storage-gcs bucket creator"
  description = "storage.buckets.create only; object/bucket admin is prefix-conditioned on gke-storage-gcs-module-sa"
  permissions = [
    "storage.buckets.create",
  ]
}

resource "google_project_iam_member" "storage_gcs_bucket_creator" {
  project = data.google_project.project.project_id
  role    = google_project_iam_custom_role.storage_gcs_bucket_creator.id
  member  = "serviceAccount:gke-storage-gcs-module-sa@${data.google_project.project.project_id}.iam.gserviceaccount.com"

  depends_on = [module.sdv_wi]
}

module "sdv_network" {
  source = "../sdv-network"

  network                     = var.sdv_network
  create_internet_egress_route = true
  subnetwork           = var.sdv_subnetwork
  region               = var.sdv_region
  router_name          = var.sdv_network_egress_router_name
  pods_range           = var.pods_range
  services_range       = var.services_range
  enable_arm64_dedicated_subnet         = var.enable_arm64_dedicated_subnet
  arm64_region         = var.arm64_region
  arm64_subnetwork                    = var.arm64_subnetwork
  arm64_pods_range                    = var.arm64_pods_range
  arm64_services_range                = var.arm64_services_range
  arm64_pods_secondary_range_name     = var.arm64_pods_secondary_range_name
  arm64_services_secondary_range_name = var.arm64_services_secondary_range_name
}

module "sdv_artifact_registry" {
  source = "../sdv-artifact-registry"

  repository_id  = var.sdv_artifact_registry_repository_id
  location       = var.sdv_location
  members        = var.sdv_artifact_registry_repository_members
  reader_members = var.sdv_artifact_registry_repository_reader_members
}

module "sdv_container_images" {
  source = "../sdv-container-images"

  providers = {
    docker = docker
  }

  depends_on = [
    module.sdv_artifact_registry
  ]

  gcp_project_id  = var.sdv_project
  gcp_region      = var.sdv_region
  gcp_registry_id = var.sdv_artifact_registry_repository_id

  images = {
    for name, image in local.images : name => {
      directory       = image.directory
      version         = image.build_version
      build_args = merge(
        try(image.build_args, {}),
        name == "horizon-dev-portal" ? { HORIZON_CLI_VERSION = image.build_version } : {}
      )
      context_path    = try(image.context_path, null)
      dockerfile_path = try(image.dockerfile_path, null)
      platform        = try(image.platform, null)
    }
  }
}

module "sdv_gcs" {
  source = "../sdv-gcs"

  bucket_name = "${data.google_project.project.project_id}-aaos"
  location    = var.sdv_location
}

module "sdv_gcs_openbsw" {
  source = "../sdv-gcs"

  bucket_name = "${data.google_project.project.project_id}-openbsw"
  location    = var.sdv_location
}

# Former module.sdv_gcs_argo_workflows ({project}-argo-workflows) was moved to KCC
# (gitops/templates/argo-workflows-bucket.yaml). Drop from state without destroying the bucket.
# Equivalent CLI: terraform state rm 'module.base.module.sdv_gcs_argo_workflows.google_storage_bucket.bucket'
# (confirm address with: terraform state list | grep argo)
removed {
  from = module.sdv_gcs_argo_workflows

  lifecycle {
    destroy = false
  }
}

module "sdv_gke_cluster" {
  source = "../sdv-gke-cluster"
  depends_on = [
    module.sdv_apis,
    module.sdv_network,
    module.sdv_gcs,
    module.sdv_gcs_openbsw,
    module.sdv_container_images,
    module.sdv_certificate_manager,
    module.sdv_ssl_policy,
    module.sdv_kms
  ]

  project_id      = data.google_project.project.project_id
  cluster_name    = var.sdv_cluster_name
  cluster_version = var.sdv_cluster_version
  release_channel = var.sdv_cluster_release_channel
  location        = var.sdv_location
  network         = var.sdv_network
  subnetwork      = var.sdv_subnetwork
  service_account = var.sdv_gcp_compute_sa_email

  # Default node pool configuration
  node_pool_name           = var.sdv_cluster_node_pool_name
  machine_type             = var.sdv_cluster_node_pool_machine_type
  node_count               = var.sdv_cluster_node_pool_count
  node_locations           = var.sdv_cluster_node_locations
  node_pool_min_node_count = var.sdv_cluster_node_pool_min_node_count
  node_pool_max_node_count = var.sdv_cluster_node_pool_max_node_count

  # build node pool configuration
  build_node_pool_name           = var.sdv_build_node_pool_name
  build_node_pool_node_count     = var.sdv_build_node_pool_node_count
  build_node_pool_machine_type   = var.sdv_build_node_pool_machine_type
  build_node_pool_min_node_count = var.sdv_build_node_pool_min_node_count
  build_node_pool_max_node_count = var.sdv_build_node_pool_max_node_count

  # ABFS build node pool configuration
  abfs_build_node_pool_name           = var.sdv_abfs_build_node_pool_name
  abfs_build_node_pool_node_count     = var.sdv_abfs_build_node_pool_node_count
  abfs_build_node_pool_machine_type   = var.sdv_abfs_build_node_pool_machine_type
  abfs_build_node_pool_min_node_count = var.sdv_abfs_build_node_pool_min_node_count
  abfs_build_node_pool_max_node_count = var.sdv_abfs_build_node_pool_max_node_count
  abfs_build_node_pool_version        = var.sdv_abfs_build_node_pool_version

  maintenance_recurring_window_start_time = var.sdv_cluster_maintenance_recurring_window_start_time
  maintenance_recurring_window_end_time   = var.sdv_cluster_maintenance_recurring_window_end_time
  maintenance_recurring_window_recurrence = var.sdv_cluster_maintenance_recurring_window_recurrence
  maintenance_exclusions                  = var.sdv_cluster_maintenance_exclusions

  # OpenBSW node pool configuration
  openbsw_build_node_pool_name           = var.sdv_openbsw_build_node_pool_name
  openbsw_build_node_pool_node_count     = var.sdv_openbsw_build_node_pool_node_count
  openbsw_build_node_pool_machine_type   = var.sdv_openbsw_build_node_pool_machine_type
  openbsw_build_node_pool_min_node_count = var.sdv_openbsw_build_node_pool_min_node_count
  openbsw_build_node_pool_max_node_count = var.sdv_openbsw_build_node_pool_max_node_count

  # Utility node pool (Gemini/Vertex CLI; workloadLabel utility + taint workloadType=utility)
  utility_node_pool_name           = var.sdv_utility_node_pool_name
  utility_node_pool_node_count     = var.sdv_utility_node_pool_node_count
  utility_node_pool_machine_type   = var.sdv_utility_node_pool_machine_type
  utility_node_pool_min_node_count = var.sdv_utility_node_pool_min_node_count
  utility_node_pool_max_node_count = var.sdv_utility_node_pool_max_node_count

  # KMS encryption for GKE secrets
  enable_kms_encryption = var.sdv_enable_kms_encryption
  kms_crypto_key_id     = local.kms_crypto_key_id
}

module "sdv_gke_apps" {
  source = "../sdv-gke-apps"
  depends_on = [
    module.sdv_gke_cluster,
    module.sdv_wi,
  ]

  providers = {
    kubernetes = kubernetes
    helm       = helm
    kubectl    = kubectl
  }

  gcp_project_id             = var.sdv_project
  gcp_cloud_region           = var.sdv_region
  sdv_cluster_name           = var.sdv_cluster_name
  gcp_cloud_zone             = var.sdv_zone
  gcp_backend_bucket         = var.gcp_backend_bucket_name
  gcp_registry_id            = var.sdv_artifact_registry_repository_id
  gcp_storage_location       = var.sdv_location

  # SCM configuration
  scm_type        = var.scm_type
  scm_auth_method = var.scm_auth_method
  scm_repo_url    = var.scm_repo_url
  scm_repo_branch = var.scm_repo_branch
  scm_repo_owner  = var.scm_repo_owner
  scm_repo_name   = var.scm_repo_name
  scm_username    = var.scm_username

  domain_name      = var.domain_name
  subdomain_name   = var.env_name
  sub_environments = var.sdv_sub_environments
  sub_env_branches = var.sdv_sub_env_branches

  # Network policies configuration
  enable_network_policies = var.sdv_enable_network_policies

  use_static_dns_a_records = var.sdv_dns_use_static_a_records

  images = {
    for name, image in local.images : name => {
      directory = image.directory
      version   = image.deploy_version
    }
  }

  common_nginx_version = local.common_nginx_version

  enable_arm64_dedicated_subnet       = var.enable_arm64_dedicated_subnet
  arm64_region       = var.arm64_region
  arm64_zone         = var.arm64_zone
  arm64_subnetwork   = var.arm64_subnetwork
  primary_subnetwork = var.sdv_subnetwork
  nodes_range        = var.nodes_range
  arm64_nodes_range  = var.arm64_nodes_range
}

module "sdv_certificate_manager" {
  source = "../sdv-certificate-manager"

  name    = var.sdv_ssl_certificate_name
  domains = local.cert_domains

  certificate_authorization_type = var.sdv_dns_use_static_a_records ? "load_balancer" : "dns"

  depends_on = [
    module.sdv_apis,
  ]
}

# Only create Cloud DNS zone when not using static A records (zone delegation flow).
module "sdv_dns_zone" {
  source = "../sdv-dns-zone"
  count  = var.sdv_dns_use_static_a_records ? 0 : 1

  zone_name        = "${var.env_name}-${var.sdv_ssl_certificate_name}-com"
  dns_name         = "${var.env_name}.${var.domain_name}."
  dns_auth_records = module.sdv_certificate_manager.dns_auth_records
  dnssec_enabled   = var.sdv_dns_dnssec_enabled

  depends_on = [
    module.sdv_certificate_manager
  ]
}

module "sdv_ssl_policy" {
  source = "../sdv-ssl-policy"

  name            = "gke-ssl-policy"
  min_tls_version = "TLS_1_2"
  profile         = "RESTRICTED"
}

# KMS Encryption for GKE Secrets
# KMS resources (keyring and key) are always created and never destroyed
# enable_kms_encryption controls whether GKE uses the key and IAM binding
module "sdv_kms" {
  source = "../sdv-kms"

  project_id                = data.google_project.project.project_id
  location                  = var.sdv_location
  keyring_name              = "gke-secrets-keyring"
  crypto_key_name           = "gke-secrets-key"
  gke_service_account_email = "service-${data.google_project.project.number}@container-engine-robot.iam.gserviceaccount.com"
  enable_kms_encryption     = var.sdv_enable_kms_encryption
}

locals {
  kms_crypto_key_id = var.sdv_enable_kms_encryption ? module.sdv_kms.crypto_key_id : ""
}

module "sdv_sa_key_secret_gce_creds" {
  source = "../sdv-sa-key-secret"

  service_account_id = var.sdv_gcp_compute_sa_email
  secret_id          = "gce-creds"
  location           = var.sdv_location
  project_id         = data.google_project.project.project_id

  gke_access = concat(
    [
      {
        ns = "jenkins"
        sa = "jenkins-sa"
      }
    ],
    [for env in var.sdv_sub_environments : {
      ns = "${env}-jenkins"
      sa = "jenkins-sa"
    }]
  )

  depends_on = [
    module.sdv_wi
  ]
}

# assign role cloud

module "sdv_iam_gcs_users" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_gcp_compute_sa_email}"
  ]

  role = "roles/storage.objectUser"

}

module "sdv_iam_compute_instance_admin" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_gcp_compute_sa_email}"
  ]

  role = "roles/compute.instanceAdmin.v1"

}

module "sdv_iam_compute_network_admin" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_gcp_compute_sa_email}"
  ]

  role = "roles/compute.networkAdmin"

}

# permission: IAP-secured Tunnel User (roles/iap.tunnelResourceAccessor) for 268541173342-compute
module "sdv_iam_secured_tunnel_user" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_gcp_compute_sa_email}",
  ]

  role = "roles/iap.tunnelResourceAccessor"

}

# permission: Service Account User (roles/iam.serviceAccountUser) for 268541173342-compute
module "sdv_iam_service_account_user" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_gcp_compute_sa_email}"
  ]

  role = "roles/iam.serviceAccountUser"

}

# defininion for custom VPN Firewall to to and from the instances.
# All traffic to instances, even from other instances, is blocked by the firewall unless firewall rules are created to allow it.
# allow tcp port 22 for compute_sa

resource "google_compute_firewall" "allow_tcp_22" {
  name     = "cuttlefish-allow-tcp-22"
  network  = var.sdv_network
  priority = 900 # Higher priority than deny rule to ensure IAP SSH works

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = concat(
    ["35.235.240.0/20"], # Google Identity-Aware Proxy (IAP): required for IAP SSH
    [var.nodes_range],   # GKE node primary subnet (e.g. Jenkins agent SNAT'd to node)
    [var.pods_range],    # GKE pods secondary range
    var.enable_arm64_dedicated_subnet ? [var.arm64_nodes_range] : [],
    var.enable_arm64_dedicated_subnet ? [var.arm64_pods_range] : [] # ARM64 pods secondary range
  )

  target_service_accounts = [var.sdv_gcp_compute_sa_email]

  depends_on = [
    module.sdv_network
  ]

}

# ============================================
# Explicit Deny Rules
# Block SSH and RDP to satisfy security scanner
# 
# Priority ordering (lower number = higher priority, evaluated first):
# 900: allow_tcp_22 (Cuttlefish IAP SSH) - EVALUATED FIRST
# 900: allow_google_health_checks_gateway (Google health checks)
# 950: deny_ssh_rdp_security_compliance (Deny SSH/RDP) - EVALUATED LAST
#
# This ensures legitimate SSH access via IAP and health checks work,
# while blocking all other SSH/RDP attempts.
# ============================================

# First, allow Google health checks on all ports (higher priority)
# This ensures health checks continue to work even if they use uncommon ports
resource "google_compute_firewall" "allow_google_health_checks_gateway" {
  name      = "allow-google-health-checks-gateway"
  network   = var.sdv_network
  direction = "INGRESS"
  priority  = 900 # Higher priority than deny rule (950)

  allow {
    protocol = "tcp"
  }

  source_ranges = [
    "130.211.0.0/22", # Google Cloud health check IPs (legacy)
    "35.191.0.0/16"   # Google Cloud health check IPs (current)
  ]

  description = "Allow Google health checks from load balancer IPs (required for Gateway health checks)"

  depends_on = [
    module.sdv_network
  ]
}

# Then, explicitly deny SSH (port 22) and RDP (port 3389) from all other sources
# This rule has lower priority (950) than allow rules (900), so legitimate traffic
# is allowed first before this deny rule is evaluated
resource "google_compute_firewall" "deny_ssh_rdp_security_compliance" {
  name      = "deny-ssh-rdp-security-compliance"
  network   = var.sdv_network
  direction = "INGRESS"
  priority  = 950 # Lower priority than allow rules (900)

  deny {
    protocol = "tcp"
    ports    = ["22", "3389"]
  }

  source_ranges = ["0.0.0.0/0"]

  description = "Explicit deny SSH/RDP. Legitimate IAP SSH (priority 900) and Google health checks (priority 900) evaluated first and allowed."

  depends_on = [
    module.sdv_network
  ]
}

# Default deny all other ingress traffic
resource "google_compute_firewall" "deny_all_ingress" {
  name      = "deny-all-ingress"
  network   = var.sdv_network
  direction = "INGRESS"
  priority  = 65000

  deny {
    protocol = "all"
  }

  source_ranges = ["0.0.0.0/0"]

  depends_on = [
    module.sdv_network
  ]
}

# Allow internal VPC traffic (all protocols)
resource "google_compute_firewall" "allow_internal_egress" {
  name      = "allow-internal-egress"
  network   = var.sdv_network
  direction = "EGRESS"
  priority  = 1000

  allow {
    protocol = "tcp"
  }

  allow {
    protocol = "udp"
  }

  allow {
    protocol = "icmp"
  }

  destination_ranges = concat(
    ["10.0.0.0/8"],                                 # Internal VPC ranges
    [var.pods_range],                               # GKE pods range
    [var.services_range],                           # GKE services range
    var.enable_arm64_dedicated_subnet ? [var.arm64_pods_range] : [], # ARM64 pods range if enabled
    var.enable_arm64_dedicated_subnet ? [var.arm64_services_range] : []
  )

  depends_on = [
    module.sdv_network
  ]
}

# Allow egress to Google APIs via Private Google Access (HTTPS only)
resource "google_compute_firewall" "allow_google_apis_egress" {
  name      = "allow-google-apis-egress"
  network   = var.sdv_network
  direction = "EGRESS"
  priority  = 1001

  allow {
    protocol = "tcp"
    ports    = ["443"]
  }

  destination_ranges = [
    "199.36.153.8/30", # Google Private Access - restricted.googleapis.com
    "199.36.153.4/30"  # Google Private Access - private.googleapis.com
  ]

  depends_on = [
    module.sdv_network
  ]
}

# Allow DNS egress for name resolution
resource "google_compute_firewall" "allow_dns_egress" {
  name      = "allow-dns-egress"
  network   = var.sdv_network
  direction = "EGRESS"
  priority  = 1002

  allow {
    protocol = "tcp"
    ports    = ["53"]
  }

  allow {
    protocol = "udp"
    ports    = ["53"]
  }

  destination_ranges = [
    "169.254.169.254/32", # Google Cloud DNS (metadata server)
    "35.199.192.0/19",    # Cloud DNS servers
    "8.8.8.8/32",         # Google Public DNS primary
    "8.8.4.4/32"          # Google Public DNS secondary
  ]

  depends_on = [
    module.sdv_network
  ]
}

# Allow HTTP/HTTPS to internet (traffic goes through Cloud NAT)
resource "google_compute_firewall" "allow_http_https_egress" {
  name      = "allow-http-https-internet-egress"
  network   = var.sdv_network
  direction = "EGRESS"
  priority  = 1003

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  destination_ranges = ["0.0.0.0/0"]

  depends_on = [
    module.sdv_network
  ]
}

# Default deny all other egress traffic
resource "google_compute_firewall" "deny_all_egress" {
  name      = "deny-all-egress"
  network   = var.sdv_network
  direction = "EGRESS"
  priority  = 65000

  deny {
    protocol = "all"
  }

  destination_ranges = ["0.0.0.0/0"]

  depends_on = [
    module.sdv_network
  ]
}
