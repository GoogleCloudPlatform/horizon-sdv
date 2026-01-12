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
# Main configuration file for sdv-dns-zone to manage Cloud DNS Zone and
# records to be created within the zone.

data "google_project" "project" {}

resource "google_dns_managed_zone" "sdv-cloud-dns-zone" {
  name          = var.zone_name
  dns_name      = var.dns_name
  force_destroy = true
}

# Create Google certificate manager certificate CNAME record required for DNS Authz
resource "google_dns_record_set" "sdv_auth_cname" {
  project      = data.google_project.project.project_id
  managed_zone = google_dns_managed_zone.sdv-cloud-dns-zone.name

  name = var.dns_auth_record.name
  type = var.dns_auth_record.type
  ttl  = 300

  rrdatas = [
    var.dns_auth_record.data
  ]

  depends_on = [
    google_dns_managed_zone.sdv-cloud-dns-zone
  ]
}
