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

  network     = var.sdv_network
  subnetwork  = var.sdv_subnetwork
  region      = var.sdv_region
  router_name = var.sdv_network_egress_router_name
}

module "sdv_bastion_host" {
  source = "../sdv-bastion-host"
  depends_on = [
    module.sdv_apis,
    module.sdv_network
  ]

  host_name       = var.sdv_bastion_host_name
  service_account = var.sdv_bastion_host_sa
  network         = var.sdv_network
  subnetwork      = var.sdv_subnetwork
  zone            = var.sdv_zone
  members         = var.sdv_bastion_host_members
}

module "sdv_gke_cluster" {
  source = "../sdv-gke-cluster"
  depends_on = [
    module.sdv_apis,
    module.sdv_network,
    module.sdv_gcs,
    module.sdv_gcs_openbsw
  ]

  project_id      = data.google_project.project.project_id
  cluster_name    = var.sdv_cluster_name
  location        = var.sdv_location
  network         = var.sdv_network
  subnetwork      = var.sdv_subnetwork
  service_account = var.sdv_computer_sa

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

module "sdv_artifact_registry" {
  source = "../sdv-artifact-registry"

  repository_id  = var.sdv_artifact_registry_repository_id
  location       = var.sdv_location
  members        = var.sdv_artifact_registry_repository_members
  reader_members = var.sdv_artifact_registry_repository_reader_members
}

module "sdv_certificate_manager" {
  source = "../sdv-certificate-manager"

  name       = var.sdv_ssl_certificate_name
  domain     = var.sdv_ssl_certificate_domain
  depends_on = [module.sdv_apis]
}

module "sdv_ssl_policy" {
  source = "../sdv-ssl-policy"

  name            = "gke-ssl-policy"
  min_tls_version = "TLS_1_2"
  profile         = "RESTRICTED"
}

module "sdv_gcs_scripts" {
  source = "../sdv-gcs"

  bucket_name = "${data.google_project.project.project_id}-scripts"
  location    = var.sdv_location
}

module "sdv_copy_to_bastion_host" {
  source = "../sdv-copy-to-bastion-host"

  bastion_host            = var.sdv_bastion_host_name
  local_file_path         = "../bash-scripts/stage1.sh"
  destination_directory   = "~/bash-scripts"
  destination_filename    = "stage1.sh"
  zone                    = var.sdv_zone
  location                = var.sdv_location
  bucket_name             = "${data.google_project.project.project_id}-scripts"
  bucket_destination_path = "bash-scripts/stage1.sh"

  depends_on = [
    module.sdv_bastion_host,
    module.sdv_gcs_scripts,
    module.sdv_gke_cluster,
    module.sdv_wi
  ]
}

module "sdv_bash_on_bastion_host" {
  source = "../sdv-bash-on-bastion-host"

  bastion_host = var.sdv_bastion_host_name
  zone         = var.sdv_zone
  command      = var.sdv_bastion_host_bash_command

  depends_on = [
    module.sdv_bastion_host,
    module.sdv_copy_to_bastion_host,
    module.sdv_gke_cluster,
    module.sdv_wi,
    module.sdv_artifact_registry
  ]
}


module "sdv_sa_key_secret_gce_creds" {
  source = "../sdv-sa-key-secret"

  service_account_id = var.sdv_computer_sa
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
    "serviceAccount:${var.sdv_computer_sa}"
  ]

  role = "roles/storage.objectUser"

}

module "sdv_iam_compute_instance_admin" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_computer_sa}"
  ]

  role = "roles/compute.instanceAdmin.v1"

}

module "sdv_iam_compute_network_admin" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_computer_sa}"
  ]

  role = "roles/compute.networkAdmin"

}

# permission: IAP-secured Tunnel User (roles/iap.tunnelResourceAccessor) for 268541173342-compute
module "sdv_iam_secured_tunnel_user" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_computer_sa}",
  ]

  role = "roles/iap.tunnelResourceAccessor"

}

# permission: Service Account User (roles/iam.serviceAccountUser) for 268541173342-compute
module "sdv_iam_service_account_user" {
  source = "../sdv-iam"
  member = [
    "serviceAccount:${var.sdv_computer_sa}"
  ]

  role = "roles/iam.serviceAccountUser"

}

# defininion for custom VPN Firewall to to and from the instances.
# All traffic to instances, even from other instances, is blocked by the firewall unless firewall rules are created to allow it.
# allow tcp port 22 for computer_sa

resource "google_compute_firewall" "allow_tcp_22" {
  name    = "cuttflefish-allow-tcp-22"
  network = var.sdv_network

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  #source_ranges = ["10.1.0.0/24"]
  source_ranges = ["0.0.0.0/0"]

  target_service_accounts = [var.sdv_computer_sa]

  depends_on = [
    module.sdv_network
  ]

}

