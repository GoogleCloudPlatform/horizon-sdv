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
# Main configuration file contains GCP project details such as
# project ID, region, zone, network etc. Set up service accounts and
# the required secrets.

module "base" {
  source = "../modules/base"

  # The project is used by provider.tf to define the GCP project
  sdv_project  = var.sdv_gcp_project_id
  sdv_location = var.sdv_gcp_cloud_region
  sdv_region   = var.sdv_gcp_cloud_region
  sdv_zone     = var.sdv_gcp_cloud_zone

  sdv_network    = "sdv-network"
  sdv_subnetwork = "sdv-subnet"

  sdv_computer_sa = var.sdv_computer_sa

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
    "integrations.googleapis.com"
  ])

  sdv_cluster_name                   = "sdv-cluster"
  sdv_cluster_node_pool_name         = "sdv-node-pool"
  sdv_cluster_node_pool_machine_type = "n1-standard-4"
  sdv_cluster_node_pool_count        = 3
  sdv_cluster_node_locations = [
    "${var.sdv_gcp_cloud_zone}"
  ]

  sdv_build_node_pool_machine_type   = "c2d-highcpu-112"
  sdv_build_node_pool_max_node_count = 20

  sdv_openbsw_build_node_pool_machine_type   = "n1-standard-8"
  sdv_openbsw_build_node_pool_max_node_count = 20

  sdv_bastion_host_name    = "sdv-bastion-host"
  sdv_bastion_host_sa      = "sdv-bastion-host-sa-iap"
  sdv_bastion_host_members = []

  sdv_network_egress_router_name = "sdv-egress-internet"

  sdv_artifact_registry_repository_id      = "horizon-sdv"
  sdv_artifact_registry_repository_members = []
  sdv_artifact_registry_repository_reader_members = [
    "serviceAccount:${var.sdv_computer_sa}",
  ]

  sdv_ssl_certificate_name   = "horizon-sdv"
  sdv_ssl_certificate_domain = "${var.sdv_gh_env_name}.${var.sdv_gh_domain_name}"

  sdv_gh_abfs_license_b64 = var.sdv_gh_abfs_license_b64
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
      account_id   = "prometheus-ui"
      display_name = "prometheus-ui"
      description  = "prometheus-ui/prometheus-ui in GKE cluster makes use of this account through WI"

      gke_sas = [
        {
          gke_ns = "monitoring"
          gke_sa = "prometheus-ui"
        }
      ]

      roles = toset([
        "roles/monitoring.viewer"
      ])
    },
    sa6 = {
      account_id   = "monitoring"
      display_name = "monitoring-sa"
      description  = "monitoring/monitoring-sa in GKE cluster makes use of this account through WI"

      gke_sas = [
        {
          gke_ns = "monitoring"
          gke_sa = "monitoring-sa"
        }
      ]

      roles = toset([
        "roles/iam.workloadIdentityUser"
      ])
    },
    sa7 = {
      account_id   = "kube-state-metrics"
      display_name = "kube-state-metrics-sa"
      description  = "kube-state-metrics/kube-state-metrics-sa in GKE cluster makes use of this account through WI"

      gke_sas = [
        {
          gke_ns = "kube-state-metrics"
          gke_sa = "gmp-public"
        }
      ]
      roles = toset([
        #"roles/monitoring.metricWriter",
        "roles/monitoring.viewer",
        #"roles/iam.serviceAccountTokenCreator",
        #"roles/iam.serviceAccountUser",
        "roles/iam.workloadIdentityUser"
      ])
    },
  }

  #
  # Define the secrets and values and gke access rules
  sdv_gcp_secrets_map = {
    s1 = {
      secret_id        = "githubAppID"
      value            = var.sdv_gh_app_id
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s2 = {
      secret_id        = "githubAppInstallationID"
      value            = var.sdv_gh_installation_id
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s3 = {
      secret_id        = "githubAppPrivateKey"
      value            = var.sdv_gh_app_key
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s4 = {
      secret_id        = "keycloakIdpCredentials"
      value            = "dummy"
      use_github_value = false
      gke_access = [
        {
          ns = "keycloak"
          sa = "keycloak-sa"
        }
      ]
    }
    s5 = {
      secret_id        = "argocdInitialPassword"
      value            = var.sdv_gh_argocd_initial_password_bcrypt
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
      ]
    }
    s6 = {
      secret_id        = "jenkinsInitialPassword"
      value            = var.sdv_gh_jenkins_initial_password
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        },
      ]
    }
    s7 = {
      secret_id        = "keycloakInitialPassword"
      value            = var.sdv_gh_keycloak_initial_password
      use_github_value = true
      gke_access = [
        {
          ns = "keycloak"
          sa = "keycloak-sa"
        },
      ]
    }
    s8 = {
      secret_id        = "githubAppPrivateKeyPKCS8"
      value            = var.sdv_gh_app_key_pkcs8
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins"
        }
      ]
    }
    # GCP secret name:  gerrit-admin-initial-password
    # WI to GKE at ns/gerrit/sa/gerrit-sa.
    s9 = {
      secret_id        = "gerritAdminInitialPassword"
      value            = var.sdv_gh_gerrit_admin_initial_password
      use_github_value = true
      gke_access = [
        {
          ns = "gerrit"
          sa = "gerrit-sa"
        }
      ]
    }
    # GCP secret name:  gh-gerrit-admin-private-key
    # WI to GKE at ns/gerrit/sa/gerrit-sa.
    s10 = {
      secret_id        = "gerritAdminPrivateKey"
      value            = var.sdv_gh_gerrit_admin_private_key
      use_github_value = true
      gke_access = [
        {
          ns = "gerrit"
          sa = "gerrit-sa"
        }
      ]
    }
    # GCP secret name:  gh-keycloak-horizon-admin-password
    # WI to GKE at ns/jenkins/sa/jenkins-sa.
    s11 = {
      secret_id        = "keycloakHorizonAdminPassword"
      value            = var.sdv_gh_keycloak_horizon_admin_password
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    # GCP secret name:  gh-cuttlefish-vm-ssh-private-key
    # WI to GKE at ns/jenkins/sa/jenkins-sa.
    s12 = {
      secret_id        = "jenkinsCuttlefishVmSshPrivateKey"
      value            = var.sdv_gh_cuttlefish_vm_ssh_private_key
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    # GCP secret name:  gh_abfs_license_b64
    # WI to GKE at ns/jenkins/sa/jenkins-sa.
    s13 = {
      secret_id        = "jenkinsABFSLicenseB64"
      value            = var.sdv_gh_abfs_license_b64
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }

  }

  sdv_bastion_host_bash_command = <<EOT
    export GITHUB_ACCESS_TOKEN=${var.sdv_gh_access_token}
    echo $GITHUB_ACCESS_TOKEN
    export GITHUB_REPO_NAME=${var.sdv_gh_repo_name}
    echo $GITHUB_REPO_NAME
    export GITHUB_ENV_NAME=${var.sdv_gh_env_name}
    echo $GITHUB_ENV_NAME
    export GITHUB_DOMAIN_NAME=${var.sdv_gh_domain_name}
    echo $GITHUB_DOMAIN_NAME
    export GCP_PROJECT_ID=${var.sdv_gcp_project_id}
    echo $GCP_PROJECT_ID
    export GCP_COMPUTER_SA=${var.sdv_computer_sa}
    echo $GCP_COMPUTER_SA
    export GCP_CLOUD_REGION=${var.sdv_gcp_cloud_region}
    echo $GCP_CLOUD_REGION
    export GCP_CLOUD_ZONE=${var.sdv_gcp_cloud_zone}
    echo $GCP_CLOUD_ZONE
    export GCP_BACKEND_BUCKET_NAME=${var.sdv_gcp_backend_bucket_name}
    echo $GCP_BACKEND_BUCKET_NAME
    cd bash-scripts
    chmod +x stage1.sh
    ./stage1.sh
    cd -
  EOT

}
