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
# Configuration file containing outputs for the "env" module.
# Outputs can be used by other modules or resources.

# Outputs (optional, helpful for local debugging)
output "gerrit_admin_private_key_openssh" {
  value     = module.gerrit_admin_key.private_key_openssh
  sensitive = true
}

output "cuttlefish_private_key_openssh" {
  value     = module.cuttlefish_key.private_key_openssh
  sensitive = true
}