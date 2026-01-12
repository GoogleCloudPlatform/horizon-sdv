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
# Main configuration file for the "sdv-certificate-manager" module.
# Create Google certificate manager certificate required for DNS Authz. It is
# required to update the CNAME record within 30 minutes from the certificate
# creation.

data "google_project" "project" {}

resource "google_certificate_manager_certificate" "horizon_sdv_cert" {
  project = data.google_project.project.project_id
  name    = var.name
  scope   = "DEFAULT"
  managed {
    domains = [
      google_certificate_manager_dns_authorization.instance.domain,
      "*.${google_certificate_manager_dns_authorization.instance.domain}"
    ]
    dns_authorizations = [
      google_certificate_manager_dns_authorization.instance.id
    ]
  }
}

resource "google_certificate_manager_dns_authorization" "instance" {
  name   = "horizon-sdv-dns-auth"
  domain = var.domain
}

resource "google_certificate_manager_certificate_map" "horizon_sdv_map" {
  project     = data.google_project.project.project_id
  name        = "horizon-sdv-map"
  description = "Certificate Manager Map for Horizon SDV"
}

resource "google_certificate_manager_certificate_map_entry" "horizon_sdv_map_entry" {
  name         = "horizon-sdv-map-entry"
  description  = "Certificate Manager Map Entry for Horizon SDV"
  map          = google_certificate_manager_certificate_map.horizon_sdv_map.name
  certificates = [google_certificate_manager_certificate.horizon_sdv_cert.id]
  matcher      = "PRIMARY"
}

