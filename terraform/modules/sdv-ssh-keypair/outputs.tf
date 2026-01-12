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
# Configuration file containing outputs for the sdv-ssh-keypair" module.
# Outputs can be used by other modules or resources.

output "private_key_pem" {
  description = "Private key (PEM)"
  value       = tls_private_key.this.private_key_pem
  sensitive   = true
}

# If your TLS provider exposes private_key_openssh, this will return it.
# Otherwise, the on-disk conversion ensures the file at local.private_path is OpenSSH.
output "private_key_openssh" {
  description = "Private key (OpenSSH format)"
  value       = tls_private_key.this.private_key_openssh
  sensitive   = true
}

output "public_key_openssh" {
  description = "Public key (OpenSSH)"
  value       = tls_private_key.this.public_key_openssh
}

output "private_key_path" {
  description = "Private key file path"
  value       = var.write_files ? local_sensitive_file.private_pem[0].filename : null
}

output "public_key_path" {
  description = "Public key file path"
  value       = var.write_files ? local.public_path : null
}

output "summary" {
  value = {
    name               = var.name
    dir                = var.dir
    algorithm          = var.algorithm
    rsa_bits           = var.rsa_bits
    ecdsa_curve        = var.ecdsa_curve
    write_files        = var.write_files
    convert_to_openssh = var.convert_to_openssh
  }
}

