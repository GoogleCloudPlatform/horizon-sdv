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

locals {
  deleted_aosp_mirror_filestore_instance_output = "[DELETED MIRROR FILESTORE INSTANCE] ${var.sdv_aosp_mirror_filestore_instance_name}"

  deleted_aosp_mirror_filestore_pv_output = "[DELETED MIRROR FILESTORE PV] ${var.sdv_aosp_mirror_filestore_pv_name}"

  deleted_aosp_mirror_filestore_pvc_output = "[DELETED MIRROR FILESTORE PVC] ${var.sdv_aosp_mirror_filestore_pvc_name}"
}

output "filestore_instance_name" {
  description = "Name of the created Mirror Filestore instance"
  value       = try(google_filestore_instance.sdv_aosp_mirror_filestore_instance.name, local.deleted_aosp_mirror_filestore_instance_output)
}

output "filestore_pv_name" {
  description = "Name of the created Persistent Volume for Mirror Filestore"
  value       = try(kubernetes_persistent_volume.sdv_aosp_mirror_filestore_pv.metadata[0].name, local.deleted_aosp_mirror_filestore_pv_output)
}

output "filestore_pvc_name" {
  description = "Name of the created Persistent Volume Claim for Mirror Filestore"
  value       = try(kubernetes_persistent_volume_claim.sdv_aosp_mirror_filestore_pvc.metadata[0].name, local.deleted_aosp_mirror_filestore_pvc_output)
}

output "filestore_pvc_size" {
  description = "Size of the created Persistent Volume Claim for Mirror Filestore"
  value       = try(kubernetes_persistent_volume_claim.sdv_aosp_mirror_filestore_pvc.spec[0].resources[0].requests.storage, local.deleted_aosp_mirror_filestore_pvc_output)
}

output "project_id" {
  description = "GCP Project ID where the Mirror is deployed"
  value       = var.sdv_aosp_mirror_project_id
}

output "location" {
  description = "Region (Location) of the Mirror Filestore instance"
  value       = var.sdv_aosp_mirror_region
}

output "network_name" {
  description = "GCP network (VPC) name where Mirror is deployed."
  value       = var.sdv_aosp_mirror_network_name
}

output "subnetwork_name" {
  description = "GCP subnetwork (VPC subnet) name where Mirror is deployed."
  value       = var.sdv_aosp_mirror_subnetwork_name
}
