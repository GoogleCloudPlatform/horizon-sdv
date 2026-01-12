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
# Main configuration file for the "sdv-ssh-keypair" module.
# Create a ssh keys for horizon eg. cuttlefish and gerrit ssh private keys.


# 1) Generate key pair
resource "tls_private_key" "this" {
  algorithm   = var.algorithm
  rsa_bits    = var.algorithm == "RSA" ? var.rsa_bits : null
  ecdsa_curve = var.algorithm == "ECDSA" ? var.ecdsa_curve : null
}

# 2) Ensure directory exists (only when writing files)
resource "null_resource" "mkdir" {
  count    = var.write_files ? 1 : 0
  triggers = { dir = var.dir }

  provisioner "local-exec" {
    command     = "mkdir -p ${var.dir}"
    interpreter = ["bash", "-c"]
  }
}

# 3) Save private key (PEM)
resource "local_sensitive_file" "private_pem" {
  count           = var.write_files ? 1 : 0
  depends_on      = [null_resource.mkdir]
  content         = tls_private_key.this.private_key_pem
  filename        = local.private_path
  file_permission = "0600"
}


# 4) Convert that private key file to OpenSSH format (-o) IN PLACE
resource "null_resource" "to_openssh" {
  count      = var.write_files && var.convert_to_openssh ? 1 : 0
  depends_on = [local_sensitive_file.private_pem]

  provisioner "local-exec" {
    #command = "ssh-keygen -p -f ./cuttlefish_vm_keys/my_cuttlefish_vm_ssh_key -N \"\" -o"
    command     = "ssh-keygen -p -f ${local.private_path} -N \"\" -o"
    interpreter = ["bash", "-c"]
  }
}


# 5) Save public key (OpenSSH) to disk (0644)
resource "local_file" "public_openssh" {
  count           = var.write_files ? 1 : 0
  depends_on      = [tls_private_key.this]
  content         = tls_private_key.this.public_key_openssh
  filename        = local.public_path
  file_permission = "0644"
}
