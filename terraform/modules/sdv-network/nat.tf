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
# Configuration file containing details for the NAT resource.

resource "google_compute_router" "vpc_nat_router" {
  project = data.google_project.project.project_id
  name    = "${var.network}-${var.region}-nat-router"
  region  = var.region
  network = module.vpc.network_self_link
}

resource "google_compute_address" "vpc_nat_ip" {
  project = data.google_project.project.project_id
  name    = "${var.network}-${var.region}-egress-nat-ip"
  region  = var.region
}

resource "google_compute_router_nat" "vpc_nat" {
  project = data.google_project.project.project_id
  name    = "${var.network}-${var.region}-egress-nat"
  region  = var.region
  router  = google_compute_router.vpc_nat_router.name

  nat_ip_allocate_option = "AUTO_ONLY"

  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"

  subnetwork {
    name                    = module.vpc.subnets["${var.region}/${var.subnetwork}"].self_link
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }

  log_config {
    filter = "TRANSLATIONS_ONLY"
    enable = true
  }
}

resource "google_compute_router" "arm64_router" {
  count   = var.enable_arm64 ? 1 : 0
  project = data.google_project.project.project_id
  name    = "${var.network}-${var.arm64_region}-arm64-nat-router"
  region  = var.arm64_region
  network = module.vpc.network_self_link
}

resource "google_compute_router_nat" "arm64_nat" {
  count   = var.enable_arm64 ? 1 : 0
  project = data.google_project.project.project_id
  name    = "${var.network}-${var.arm64_region}-arm64-egress-nat"
  region  = var.arm64_region
  router  = google_compute_router.arm64_router[0].name

  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"

  subnetwork {
    name                    = module.vpc.subnets["${var.arm64_region}/${var.arm64_subnetwork}"].self_link
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }

  log_config {
    filter = "TRANSLATIONS_ONLY"
    enable = true
  }
}