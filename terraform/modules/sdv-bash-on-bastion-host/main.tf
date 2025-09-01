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
# Main configuration file for the "sdv-bash-on-bastion-host" module.
# Contains a null resource that executes a command on the cluster's bastion
# host once it has been provisioned successfully.

data "google_project" "project" {}

#resource "terraform_data" "debug_google_project" {
#  input = data.google_project.project
#}

resource "null_resource" "execute_bash_commands" {
  triggers = {
    always_run = "${timestamp()}"
  }

  provisioner "local-exec" {
    command = <<EOT
      echo "Executing bash commands..."
      gcloud beta compute ssh ${var.bastion_host} --zone=${var.zone} --project=${data.google_project.project.project_id} --command="
      echo 'Executing commands on the bastion host...'
      ${var.command}
      "
    EOT
  }
}
