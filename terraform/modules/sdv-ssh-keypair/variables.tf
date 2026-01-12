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

variable "name" {
  description = "Base file name for the key pair (no extension)"
  type        = string
}

variable "dir" {
  description = "Directory where keys will be stored"
  type        = string
}

variable "algorithm" {
  description = "Key algorithm: RSA | ECDSA | ED25519"
  type        = string
  default     = "RSA"
}

variable "rsa_bits" {
  description = "RSA key strength (only for RSA)"
  type        = number
  default     = 4096
}

variable "ecdsa_curve" {
  description = "ECDSA curve: P256 | P384 | P521 (only for ECDSA)"
  type        = string
  default     = "P256"
}

variable "write_files" {
  description = "Whether to write keys to disk"
  type        = bool
  default     = true
}

variable "convert_to_openssh" {
  description = "Convert the private key file to new OpenSSH format (-o)"
  type        = bool
  default     = true
}

locals {
  private_path = "${var.dir}/${var.name}"
  public_path  = "${var.dir}/${var.name}.pub"
}