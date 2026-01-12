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

// from jenkins env; same as horizon-sdv cluster value
variable "sdv_aosp_mirror_project_id" {
  description = "GCP Project ID (existing) where Mirror is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv cluster value
variable "sdv_aosp_mirror_region" {
  description = "GCP region where Mirror is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv cluster value
variable "sdv_aosp_mirror_zone" {
  description = "GCP zone where Mirror is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv cluster value
variable "sdv_aosp_mirror_network_name" {
  description = "GCP network (VPC) name where Mirror is deployed."
  type        = string
}

// from jenkins env; same as horizon-sdv cluster value
variable "sdv_aosp_mirror_subnetwork_name" {
  description = "GCP subnetwork (VPC subnet) name where Mirror is deployed."
  type        = string
}

// new resource
variable "sdv_aosp_mirror_filestore_instance_name" {
  description = "Name of the Mirror Filestore instance."
  type        = string
  default     = "sdv-aosp-mirror-filestore-instance"
}

// new resource
variable "sdv_aosp_mirror_filestore_share_name" {
  description = "Name of the Mirror Filestore share, part of the Filestore instance."
  type        = string
  default     = "sdv_aosp_mirror_filestore_share"
}

// new resource
variable "sdv_aosp_mirror_filestore_share_capacity_gb" {
  description = "Capacity (in GB) of the Mirror Filestore share."
  type        = number
  default     = 2048
}

// new resource
variable "sdv_aosp_mirror_filestore_pv_name" {
  description = "Name of the Persistent Volume for Mirror Filestore."
  type        = string
  default     = "sdv-aosp-mirror-filestore-pv"
}

// from jenkins env; same NS as android build pods in horizon-sdv cluster
variable "sdv_aosp_mirror_filestore_pvc_namespace" {
  description = "Namespace where the Persistent Volume Claim for Mirror Filestore is created."
  type        = string
  default     = "jenkins"
}

// new resource
variable "sdv_aosp_mirror_filestore_pvc_name" {
  description = "Name of the Persistent Volume Claim for Mirror Filestore."
  type        = string
  default     = "sdv-aosp-mirror-filestore-pvc"
}