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
# Main configuration file for the "base" module.
# Makes use of other modules to provision various resources.

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

module "sdv_network" {
  source = "../sdv-network"

  network              = var.sdv_network
  subnetwork           = var.sdv_subnetwork
  region               = var.sdv_region
  router_name          = var.sdv_network_egress_router_name
  enable_arm64         = var.enable_arm64
  arm64_region         = var.arm64_region
  arm64_subnetwork     = var.arm64_subnetwork
  arm64_pods_range     = var.arm64_pods_range
  arm64_services_range = var.arm64_services_range
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
      directory  = image.directory
      version    = image.build_version
      build_args = try(image.build_args, {})
    }
  }
}

module "sdv_gke_cluster" {
  source = "../sdv-gke-cluster"
  depends_on = [
    module.sdv_apis,
    module.sdv_network,
    module.sdv_gcs,
    module.sdv_gcs_openbsw,
    module.sdv_container_images
  ]

  project_id      = data.google_project.project.project_id
  cluster_name    = var.sdv_cluster_name
  location        = var.sdv_location
  network         = var.sdv_network
  subnetwork      = var.sdv_subnetwork
  service_account = var.sdv_gcp_compute_sa_email

  # Default node pool configuration
  node_pool_name = var.sdv_cluster_node_pool_name
  machine_type   = var.sdv_cluster_node_pool_machine_type
  node_count     = var.sdv_cluster_node_pool_count
  node_locations = var.sdv_cluster_node_locations

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

  # OpenBSW node pool configuration
  openbsw_build_node_pool_name           = var.sdv_openbsw_build_node_pool_name
  openbsw_build_node_pool_node_count     = var.sdv_openbsw_build_node_pool_node_count
  openbsw_build_node_pool_machine_type   = var.sdv_openbsw_build_node_pool_machine_type
  openbsw_build_node_pool_min_node_count = var.sdv_openbsw_build_node_pool_min_node_count
  openbsw_build_node_pool_max_node_count = var.sdv_openbsw_build_node_pool_max_node_count
}

module "sdv_gke_apps" {
  source = "../sdv-gke-apps"
  depends_on = [
    module.sdv_gke_cluster,
  ]

  providers = {
    kubernetes = kubernetes
    helm       = helm
    kubectl    = kubectl
  }

  gcp_project_id     = var.sdv_project
  gcp_cloud_region   = var.sdv_region
  sdv_cluster_name   = var.sdv_cluster_name
  gcp_cloud_zone     = var.sdv_zone
  gcp_backend_bucket = var.gcp_backend_bucket_name
  gcp_registry_id    = var.sdv_artifact_registry_repository_id

  github_repo_url    = "https://github.com/${var.github_repo_owner}/${var.github_repo_name}"
  github_auth_method = var.github_auth_method
  github_repo_owner  = var.github_repo_owner
  github_repo_name   = var.github_repo_name
  github_repo_branch = var.github_repo_branch

  domain_name    = var.github_domain_name
  subdomain_name = var.github_env_name

  images = {
    for name, image in local.images : name => {
      directory = image.directory
      version   = image.deploy_version
    }
  }
}

module "sdv_certificate_manager" {
  source = "../sdv-certificate-manager"

  name   = var.sdv_ssl_certificate_name
  domain = var.sdv_ssl_certificate_domain

  depends_on = [
    module.sdv_apis,
  ]
}

module "sdv_dns_zone" {
  source = "../sdv-dns-zone"

  zone_name       = "${var.github_env_name}-${var.sdv_ssl_certificate_name}-com"
  dns_name        = "${var.github_env_name}.${var.github_domain_name}."
  dns_auth_record = module.sdv_certificate_manager.dns_auth_record

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

module "sdv_sa_key_secret_gce_creds" {
  source = "../sdv-sa-key-secret"

  service_account_id = var.sdv_gcp_compute_sa_email
  secret_id          = "gce-creds"
  location           = var.sdv_location
  project_id         = data.google_project.project.project_id

  gke_access = [
    {
      ns = "jenkins"
      sa = "jenkins-sa"
    }
  ]

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
  name    = "cuttflefish-allow-tcp-22"
  network = var.sdv_network

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  #source_ranges = ["10.1.0.0/24"]
  source_ranges = ["0.0.0.0/0"]

  target_service_accounts = [var.sdv_gcp_compute_sa_email]

  depends_on = [
    module.sdv_network
  ]

}