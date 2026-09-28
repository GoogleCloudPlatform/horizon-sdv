// Copyright (c) 2025-2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

# NOTE: the packer { required_plugins { googlecompute { ... } } } block is NOT declared here.
# It is generated at build time into plugins.pkr.hcl next to a copy of this template
# (see generate_packer_plugins_fragment in cf_create_instance_template.sh), so the version
# bounds are configurable via env / Helm / Jenkins params instead of hard-coded here.
#
# Bounds and their justification (defaults live in cf_create_instance_template.sh):
#   >= PACKER_GOOGLECOMPUTE_VERSION_MIN (default 1.2.3)
#       max_run_duration_in_seconds / instance_termination_action need a recent 1.2.x plugin.
#       Registry releases for this source only go through v1.2.x; there is no v1.5.0 line here.
#   <  PACKER_GOOGLECOMPUTE_VERSION_MAX (default 1.2.5)
#       Argo read-only /workspace hit SIGSEGV in StepImportOSLoginSSHKey with v1.2.5; cap below it
#       until upstream fixes it, then raise the max via param (no code change needed).

variable "project_id" {
  type = string
}

variable "zone" {
  type = string
}

variable "region" {
  type = string
}

variable "network" {
  type = string
}

variable "subnetwork" {
  type = string
}

variable "source_image_project_id" {
  type = string
}

variable "source_image" {
  type = string
}

variable "machine_type" {
  type = string
}

variable "disk_size_gb" {
  type = number
}

variable "disk_type" {
  type = string
}

variable "image_name" {
  type = string
}

variable "image_description" {
  type = string
}

variable "ssh_username" {
  type = string
}

variable "default_user" {
  type = string
}

variable "cf_script_path" {
  type = string
}

variable "android_cuttlefish_revision" {
  type = string
}

variable "cuttlefish_url" {
  type = string
}

variable "cuttlefish_post_command" {
  type = string
}

variable "repo_username" {
  type = string
  default = ""
}

variable "repo_password" {
  type = string
  default   = ""
  sensitive = true
}

variable "java_version" {
  type = string
}

variable "nodejs_version" {
  type = string
}

variable "curl_update_command" {
  type = string
}

variable "os_version" {
  type = string
}

variable "cts_android_16_url" {
  type = string
  default = ""
}

variable "cts_android_15_url" {
  type = string
  default = ""
}

variable "cts_android_14_url" {
  type = string
  default = ""
}

variable "ssh_public_key_b64" {
  type      = string
  sensitive = true
}

# GCE limit VM runtime on the ephemeral Packer builder only (not the baked image / instance template).
# Prevents orphaned builders if Jenkins/Argo kills the client or Packer hangs. Requires a recent
# hashicorp/googlecompute Packer plugin (see packer { required_plugins }).
variable "packer_max_run_duration_seconds" {
  type        = number
  description = "Maximum wall-clock seconds the Packer builder instance may run before GCE deletes it."
}

# When Packer runs outside the builder VPC (e.g. Argo pod on GKE), plain SSH to the RFC1918 address
# often never completes; IAP-tunneled SSH matches `gcloud compute ssh --tunnel-through-iap`.
# Builder SA needs IAP-secured Tunnel User (e.g. roles/iap.tunnelResourceAccessor on the project).
variable "use_iap" {
  type    = bool
  default = true
}

variable "ssh_timeout" {
  type        = string
  default     = "15m"
  description = "How long to wait for SSH (boot + IAP tunnel ready). Default 15m for slow metal images."
}

variable "iap_tunnel_launch_wait" {
  type        = number
  default     = 300
  description = "Seconds to wait for IAP tunnel before treating launch as failed (plugin default is shorter)."
}

variable "state_timeout" {
  type        = string
  default     = "20m"
  description = "How long to wait for packer image registration. System default of 5m may not be enough"
}

source "googlecompute" "cuttlefish" {
  project_id              = var.project_id
  source_image_project_id = [var.source_image_project_id]
  source_image            = var.source_image
  zone                    = var.zone
  machine_type            = var.machine_type
  # Required for bare-metal machine families (for example c4a-*-metal).
  on_host_maintenance     = "TERMINATE"
  disk_size               = var.disk_size_gb
  disk_type               = var.disk_type
  network                 = var.network
  subnetwork              = var.subnetwork
  omit_external_ip        = true
  use_internal_ip         = true
  use_iap                 = var.use_iap
  ssh_timeout             = var.ssh_timeout
  state_timeout           = var.state_timeout
  iap_tunnel_launch_wait  = var.iap_tunnel_launch_wait
  ssh_username            = var.ssh_username
  image_name              = var.image_name
  image_description       = var.image_description
  image_storage_locations = [var.region]
  # If the job fails or the Packer client disconnects, the builder can otherwise keep running until manual cleanup.
  max_run_duration_in_seconds = var.packer_max_run_duration_seconds
  instance_termination_action = "DELETE"
  metadata = {
    enable-oslogin = "true"
  }
}

build {
  sources = ["source.googlecompute.cuttlefish"]

  provisioner "file" {
    source      = var.cf_script_path
    destination = "/tmp/cf"
  }

  provisioner "shell" {
    execute_command = "chmod +x {{ .Path }}; {{ .Vars }} sudo -E bash '{{ .Path }}'"
    environment_vars = [
      "CUTTLEFISH_REVISION=${var.android_cuttlefish_revision}",
      "CUTTLEFISH_URL=${var.cuttlefish_url}",
      "CUTTLEFISH_POST_COMMAND=${var.cuttlefish_post_command}",
      "REPO_USERNAME=${var.repo_username}",
      "REPO_PASSWORD=${var.repo_password}",
      "JAVA_VERSION=${var.java_version}",
      "NODEJS_VERSION=${var.nodejs_version}",
      "CURL_UPDATE_COMMAND=${var.curl_update_command}",
      "DEFAULT_USER=${var.default_user}",
      "OS_VERSION=${var.os_version}",
      "CTS_ANDROID_16_URL=${var.cts_android_16_url}",
      "CTS_ANDROID_15_URL=${var.cts_android_15_url}",
      "CTS_ANDROID_14_URL=${var.cts_android_14_url}",
      "SSH_PUBLIC_KEY_B64=${var.ssh_public_key_b64}"
    ]
    script = "${path.root}/provision_cf_host.sh"
  }
}
