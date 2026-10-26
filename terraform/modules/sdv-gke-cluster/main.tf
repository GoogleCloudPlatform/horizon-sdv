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

# Plan-time guard: GKE only accepts NO_MINOR_UPGRADES and NO_MINOR_OR_NODE_UPGRADES
# exclusion scopes on clusters enrolled in a release channel. With release_channel =
# "UNSPECIFIED" the API rejects anything other than NO_UPGRADES, so fail fast at
# `terraform plan` instead of partway through `apply`.
resource "terraform_data" "validate_maintenance_exclusion_scopes" {
  lifecycle {
    precondition {
      condition = (
        var.release_channel != "UNSPECIFIED" || alltrue([
          for e in var.maintenance_exclusions : e.scope == "NO_UPGRADES"
        ])
      )
      error_message = <<-EOT
        Invalid maintenance_exclusions for release_channel = "UNSPECIFIED".

        GKE only accepts NO_MINOR_UPGRADES and NO_MINOR_OR_NODE_UPGRADES exclusion
        scopes on clusters enrolled in a release channel. On UNSPECIFIED clusters
        the API rejects them with an error.

        Either set release_channel to RAPID, REGULAR, STABLE, or EXTENDED, or change every
        maintenance_exclusions[*].scope to "NO_UPGRADES".
      EOT
    }
  }
}

resource "google_container_cluster" "sdv_cluster" {
  project                  = data.google_project.project.project_id
  name                     = var.cluster_name
  location                 = var.location
  network                  = var.network
  subnetwork               = var.subnetwork
  remove_default_node_pool = true
  initial_node_count       = 1
  fleet {
    project = var.project_id
  }

  # Set `deletion_protection` to `true` will ensure that one cannot
  # accidentally delete this instance by use of Terraform.
  deletion_protection = false

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  master_authorized_networks_config {
    gcp_public_cidrs_access_enabled = false
  }

  ip_allocation_policy {
    stack_type                    = "IPV4"
    cluster_secondary_range_name  = "pods-range"
    services_secondary_range_name = "services-range"
  }

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = true
    master_ipv4_cidr_block  = "10.0.0.0/28"
  }

  secret_manager_config {
    enabled = true
  }

  # Release channel is configurable (RAPID / REGULAR / STABLE / EXTENDED / UNSPECIFIED).
  release_channel {
    channel = var.release_channel
  }

  min_master_version = var.cluster_version

  # Maintenance windows + exclusions (cluster-level) for scheduling/deferring upgrades
  maintenance_policy {
    recurring_window {
      start_time = var.maintenance_recurring_window_start_time
      end_time   = var.maintenance_recurring_window_end_time
      recurrence = var.maintenance_recurring_window_recurrence
    }

    dynamic "maintenance_exclusion" {
      for_each = var.maintenance_exclusions
      content {
        exclusion_name = maintenance_exclusion.value.exclusion_name
        start_time     = maintenance_exclusion.value.start_time
        end_time       = maintenance_exclusion.value.end_time
        exclusion_options {
          scope = maintenance_exclusion.value.scope
        }
      }
    }
  }

  # enable gateway api
  gateway_api_config {
    channel = "CHANNEL_STANDARD"
  }

  addons_config {
    network_policy_config {
      disabled = false
    }
    http_load_balancing {
      disabled = false
    }
    gcp_filestore_csi_driver_config {
      enabled = true
    }
    config_connector_config {
      enabled = true
    }
    # GKE Standard 1.34.1-gke.3720000+ enables NodeLocal DNSCache by default.
    # That intercepts kube-dns ClusterIP on the node, so existing NetworkPolicies
    # that only match kube-dns Pods drop DNS, deployment does not complete successfully.
    dns_cache_config {
      enabled = false
    }
  }

  # Enable network policy enforcement for pod-to-pod traffic restriction
  # Required for GCP 327 compliance: Kubernetes pod-to-pod traffic must be restricted
  # Fix for vulnerability #6 - Using Calico provider (legacy datapath)
  network_policy {
    enabled  = true
    provider = "CALICO"
  }

  # Enable autoscaling
  cluster_autoscaling {
    enabled             = false
    autoscaling_profile = "OPTIMIZE_UTILIZATION"
  }

  # monitoring configuration
  monitoring_config {
    enable_components = ["SYSTEM_COMPONENTS", "APISERVER", "SCHEDULER", "CONTROLLER_MANAGER", "CADVISOR", "KUBELET"]
    # DISABLED monitoring for Kube state metrics : STORAGE, POD, DEPLOYMENT, STATEFULSET, DAEMONSET, JOBSET

    # Control Plane Metrics enabled
    managed_prometheus {
      enabled = true
    }
  }

  # Enable intranode visibility for better network monitoring and security
  # Allows monitoring of pod-to-pod traffic within nodes for security analysis
  enable_intranode_visibility = true

  # Enable application-layer secrets encryption with Cloud KMS (Fix for vulnerability #8)
  # Encrypts Kubernetes secrets at rest using customer-managed encryption keys
  # NOTE: Once enabled, GKE database encryption cannot be disabled without recreating the cluster
  # When enable_kms_encryption = false, this explicitly sets state to DECRYPTED
  database_encryption {
    state    = var.enable_kms_encryption ? "ENCRYPTED" : "DECRYPTED"
    key_name = var.enable_kms_encryption ? var.kms_crypto_key_id : ""
  }
}

# Automatically enable flow logs on the GKE-auto-created master subnet
# GKE creates this subnet automatically for the private cluster control plane
# This null_resource runs after cluster and node pool creation to enable flow logs
resource "null_resource" "enable_gke_master_subnet_flow_logs" {
  # Trigger on cluster recreation
  triggers = {
    cluster_id = google_container_cluster.sdv_cluster.id
  }

  provisioner "local-exec" {
    interpreter = ["bash", "-c"]
    command     = <<-EOT
      set -euo pipefail

      retry_gcloud() {
        local description="$1"
        shift
        local max_attempts=5 attempt=1 delay=15
        while true; do
          if "$@"; then
            echo "OK: $description"
            return 0
          fi
          if [ "$attempt" -ge "$max_attempts" ]; then
            echo "ERROR: $description failed after $max_attempts attempts"
            return 1
          fi
          echo "WARN: $description failed (attempt $attempt/$max_attempts); retrying in $${delay}s..."
          sleep "$delay"
          attempt=$((attempt + 1))
          delay=$((delay * 2))
        done
      }

      discover_master_subnet() {
        local attempt=1 max_attempts=12 delay=10 master_subnet=""
        while [ "$attempt" -le "$max_attempts" ]; do
          master_subnet=$(gcloud compute networks subnets list \
            --project=${var.project_id} \
            --network=${var.network} \
            --filter="name~'gke-${var.cluster_name}-.*-pe-subnet' AND region:${var.location}" \
            --format="value(name)" \
            --limit=1)
          if [ -n "$master_subnet" ]; then
            echo "$master_subnet"
            return 0
          fi
          if [ "$attempt" -lt "$max_attempts" ]; then
            echo "WARN: GKE master subnet not found yet (attempt $attempt/$max_attempts); retrying in $${delay}s..." >&2
            sleep "$delay"
          fi
          attempt=$((attempt + 1))
        done
        echo "ERROR: GKE master subnet not found after $max_attempts attempts. This may indicate the cluster is not a private cluster." >&2
        return 1
      }

      MASTER_SUBNET=$(discover_master_subnet)
      echo "Found GKE master subnet: $MASTER_SUBNET"

      FLOW_LOGS_ENABLED=$(gcloud compute networks subnets describe "$MASTER_SUBNET" \
        --project="${var.project_id}" \
        --region="${var.location}" \
        --format="value(enableFlowLogs)" 2>/dev/null || echo "False")

      if [ "$FLOW_LOGS_ENABLED" = "True" ]; then
        echo "Flow logs are already enabled on $MASTER_SUBNET"
        exit 0
      fi

      echo "Enabling flow logs on GKE master subnet: $MASTER_SUBNET"

      retry_gcloud "enable flow logs on GKE master subnet" \
        gcloud compute networks subnets update "$MASTER_SUBNET" \
        --project="${var.project_id}" \
        --region="${var.location}" \
        --enable-flow-logs \
        --logging-aggregation-interval=interval-5-min \
        --logging-flow-sampling=0.5 \
        --logging-metadata=include-all

      echo "Flow logs successfully enabled on GKE master subnet"
    EOT
  }

  depends_on = [
    google_container_cluster.sdv_cluster,
    google_container_node_pool.sdv_main_node_pool,
    google_container_node_pool.sdv_build_node_pool,
    google_container_node_pool.sdv_abfs_build_node_pool,
    google_container_node_pool.sdv_openbsw_build_node_pool,
    google_container_node_pool.sdv_utility_node_pool,
  ]
}

# Automatically enable Private Google Access on the GKE-auto-created master subnet
resource "null_resource" "enable_gke_master_subnet_private_google_access" {
  # Trigger on cluster recreation
  triggers = {
    cluster_id = google_container_cluster.sdv_cluster.id
  }

  provisioner "local-exec" {
    interpreter = ["bash", "-c"]
    command     = <<-EOT
      set -euo pipefail

      retry_gcloud() {
        local description="$1"
        shift
        local max_attempts=5 attempt=1 delay=15
        while true; do
          if "$@"; then
            echo "OK: $description"
            return 0
          fi
          if [ "$attempt" -ge "$max_attempts" ]; then
            echo "ERROR: $description failed after $max_attempts attempts"
            return 1
          fi
          echo "WARN: $description failed (attempt $attempt/$max_attempts); retrying in $${delay}s..."
          sleep "$delay"
          attempt=$((attempt + 1))
          delay=$((delay * 2))
        done
      }

      discover_master_subnet() {
        local attempt=1 max_attempts=12 delay=10 master_subnet=""
        while [ "$attempt" -le "$max_attempts" ]; do
          master_subnet=$(gcloud compute networks subnets list \
            --project=${var.project_id} \
            --network=${var.network} \
            --filter="name~'gke-${var.cluster_name}-.*-pe-subnet' AND region:${var.location}" \
            --format="value(name)" \
            --limit=1)
          if [ -n "$master_subnet" ]; then
            echo "$master_subnet"
            return 0
          fi
          if [ "$attempt" -lt "$max_attempts" ]; then
            echo "WARN: GKE master subnet not found yet (attempt $attempt/$max_attempts); retrying in $${delay}s..." >&2
            sleep "$delay"
          fi
          attempt=$((attempt + 1))
        done
        echo "ERROR: GKE master subnet not found after $max_attempts attempts. This may indicate the cluster is not a private cluster." >&2
        return 1
      }

      MASTER_SUBNET=$(discover_master_subnet)
      echo "Found GKE master subnet: $MASTER_SUBNET"

      PGA_ENABLED=$(gcloud compute networks subnets describe "$MASTER_SUBNET" \
        --project="${var.project_id}" \
        --region="${var.location}" \
        --format="value(privateIpGoogleAccess)" 2>/dev/null || echo "False")

      if [ "$PGA_ENABLED" = "True" ]; then
        echo "Private Google Access is already enabled on $MASTER_SUBNET"
        exit 0
      fi

      echo "Enabling Private Google Access on GKE master subnet: $MASTER_SUBNET"

      retry_gcloud "enable Private Google Access on GKE master subnet" \
        gcloud compute networks subnets update "$MASTER_SUBNET" \
        --project="${var.project_id}" \
        --region="${var.location}" \
        --enable-private-ip-google-access

      echo "Private Google Access successfully enabled on GKE master subnet"
    EOT
  }

  depends_on = [
    google_container_cluster.sdv_cluster,
    google_container_node_pool.sdv_main_node_pool,
    google_container_node_pool.sdv_build_node_pool,
    google_container_node_pool.sdv_abfs_build_node_pool,
    google_container_node_pool.sdv_openbsw_build_node_pool,
    google_container_node_pool.sdv_utility_node_pool,
    null_resource.enable_gke_master_subnet_flow_logs,
  ]
}


resource "google_container_node_pool" "sdv_main_node_pool" {
  name           = var.node_pool_name
  location       = var.location
  cluster        = google_container_cluster.sdv_cluster.name
  node_count     = var.node_count
  node_locations = var.node_locations
  node_config {
    preemptible  = false
    machine_type = var.machine_type

    # Google recommends custom service accounts that have cloud-platform
    # scope and permissions granted via IAM Roles.
    service_account = var.service_account
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  autoscaling {
    min_node_count = var.node_pool_min_node_count
    max_node_count = var.node_pool_max_node_count
  }

}

resource "google_container_node_pool" "sdv_build_node_pool" {
  name           = var.build_node_pool_name
  location       = var.location
  cluster        = google_container_cluster.sdv_cluster.name
  node_count     = var.build_node_pool_node_count
  node_locations = var.node_locations
  node_config {
    preemptible  = false
    machine_type = var.build_node_pool_machine_type
    disk_size_gb = 500
    image_type   = "UBUNTU_CONTAINERD"

    # Google recommends custom service accounts that have cloud-platform
    # scope and permissions granted via IAM Roles.
    service_account = var.service_account
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    labels = {
      workloadLabel = "android"
    }

    taint {
      key    = "workloadType"
      value  = "android"
      effect = "NO_SCHEDULE"
    }

    metadata = {
      disable-legacy-endpoints = "true"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  autoscaling {
    min_node_count = var.build_node_pool_min_node_count
    max_node_count = var.build_node_pool_max_node_count
  }

}

resource "google_container_node_pool" "sdv_abfs_build_node_pool" {
  name           = var.abfs_build_node_pool_name
  location       = var.location
  cluster        = google_container_cluster.sdv_cluster.name
  version        = var.abfs_build_node_pool_version
  node_count     = var.abfs_build_node_pool_node_count
  node_locations = var.node_locations
  node_config {
    preemptible  = false
    machine_type = var.abfs_build_node_pool_machine_type
    disk_size_gb = 500
    image_type   = "UBUNTU_CONTAINERD"

    # Google recommends custom service accounts that have cloud-platform
    # scope and permissions granted via IAM Roles.
    service_account = var.service_account
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    labels = {
      workloadLabel = "android-abfs"
    }

    taint {
      key    = "workloadType"
      value  = "android-abfs"
      effect = "NO_SCHEDULE"
    }

    metadata = {
      disable-legacy-endpoints = "true"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  autoscaling {
    min_node_count = var.abfs_build_node_pool_min_node_count
    max_node_count = var.abfs_build_node_pool_max_node_count
  }

  # ABFS pool management policy is coupled to the cluster release channel:
  #
  # - release_channel = "UNSPECIFIED": auto_upgrade = false. Pins this pool to
  #   var.abfs_build_node_pool_version for CASFS kernel-module compatibility.
  #   Use cluster maintenance windows / exclusions (NO_UPGRADES scope) to defer
  #   any other cluster maintenance.
  # - release_channel = RAPID / REGULAR / STABLE / EXTENDED: auto_upgrade = true is REQUIRED
  #   by the GKE API on every node pool of a channel-enrolled cluster
  #   (auto_upgrade = false is rejected at apply time). The CASFS version pin is
  #   therefore not enforced; rely on NO_MINOR_OR_NODE_UPGRADES exclusions to
  #   defer minor/node upgrades during CASFS-sensitive periods.
  management {
    auto_repair  = true
    auto_upgrade = var.release_channel != "UNSPECIFIED"
  }
}

resource "google_container_node_pool" "sdv_openbsw_build_node_pool" {
  name           = var.openbsw_build_node_pool_name
  location       = var.location
  cluster        = google_container_cluster.sdv_cluster.name
  node_count     = var.openbsw_build_node_pool_node_count
  node_locations = var.node_locations
  node_config {
    preemptible  = false
    machine_type = var.openbsw_build_node_pool_machine_type
    disk_size_gb = 500
    image_type   = "UBUNTU_CONTAINERD"

    # Google recommends custom service accounts that have cloud-platform
    # scope and permissions granted via IAM Roles.
    service_account = var.service_account
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    labels = {
      workloadLabel = "openbsw"
    }

    taint {
      key    = "workloadType"
      value  = "openbsw"
      effect = "NO_SCHEDULE"
    }

    metadata = {
      disable-legacy-endpoints = "true"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  autoscaling {
    min_node_count = var.openbsw_build_node_pool_min_node_count
    max_node_count = var.openbsw_build_node_pool_max_node_count
  }

}

resource "google_container_node_pool" "sdv_utility_node_pool" {
  name           = var.utility_node_pool_name
  location       = var.location
  cluster        = google_container_cluster.sdv_cluster.name
  node_count     = var.utility_node_pool_node_count
  node_locations = var.node_locations
  node_config {
    preemptible  = false
    machine_type = var.utility_node_pool_machine_type
    disk_size_gb = 500
    image_type   = "UBUNTU_CONTAINERD"

    service_account = var.service_account
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    labels = {
      workloadLabel = "utility"
    }

    taint {
      key    = "workloadType"
      value  = "utility"
      effect = "NO_SCHEDULE"
    }

    metadata = {
      disable-legacy-endpoints = "true"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  autoscaling {
    min_node_count = var.utility_node_pool_min_node_count
    max_node_count = var.utility_node_pool_max_node_count
  }

}

