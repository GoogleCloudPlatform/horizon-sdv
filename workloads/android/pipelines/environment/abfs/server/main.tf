# Copyright (c) 2025 Accenture, All Rights Reserved.
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
data "google_project" "project" {
  project_id = var.project_id
}

module "abfs-server" {
  source                                = "git::https://github.com/terraform-google-modules/terraform-google-abfs.git//modules/server?ref=961f5aa3c3be87a242597cbd4bc08821f28a7085"
  project_id                            = var.project_id
  zone                                  = var.zone
  service_account_email                 = "abfs-server@${var.project_id}.iam.gserviceaccount.com"
  subnetwork                            = "sdv-subnet"
  abfs_bucket_location                  = var.region
  abfs_spanner_instance_config          = "regional-${var.region}"
  abfs_docker_image_uri                 = var.abfs_docker_image_uri
  abfs_license                          = var.abfs_license
  abfs_server_machine_type              = var.abfs_server_machine_type
  abfs_server_name                      = "abfs-server"
  abfs_server_allow_stopping_for_update = true
}

resource "google_compute_firewall" "abfs-server-allow-all-from-internal" {
  name    = "abfs-server-allow-all-from-internal"
  network = var.sdv_network

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "icmp"
  }

  source_ranges = ["0.0.0.0/0"]

  target_service_accounts = ["abfs-server@${var.project_id}.iam.gserviceaccount.com"]
}

resource "google_logging_project_bucket_config" "basic" {
  project        = var.project_id
  location       = "global"
  retention_days = 1
  bucket_id      = "_Default"
}

resource "google_logging_project_sink" "log-bucket" {
  name        = "_Default"
  destination = "logging.googleapis.com/projects/${var.project_id}/locations/global/buckets/_Default"

  exclusions {
    name   = "no-spanner"
    filter = "resource.type=\"spanner_instance\" OR resource.type=\"spanner_database\" OR logName:(\"cloudaudit.googleapis.com\" OR \"spanner.googleapis.com\")"
  }

  unique_writer_identity = true
}
