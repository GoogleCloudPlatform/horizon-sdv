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
# Description
# Main configuration file for the "sdv-container-images" module.
# Builds and pushes container images to Google Artifact Registry.

# Build container images
resource "docker_image" "sdv-container-images" {
  for_each = var.images

  name = "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/${each.key}:${each.value.version}"
  build {
    no_cache = true

    context = "${path.module}/images/${each.value.directory}/${each.key}"
    tag     = ["${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/${each.key}:${each.value.version}"]

    build_args = each.value.build_args
  }

  triggers = {
    dir_sha1 = sha1(join("", [
      for f in fileset("${path.module}/images/${each.value.directory}/${each.key}", "**") :
      filesha1("${path.module}/images/${each.value.directory}/${each.key}/${f}")
    ]))

    build_args_sha = sha1(jsonencode(each.value.build_args))
  }
}

# Push container images to Google Artifact Registry
resource "docker_registry_image" "sdv-container-images" {
  for_each = docker_image.sdv-container-images

  name          = each.value.name
  keep_remotely = true

  # Push container image to Google Artifact Registry when changes are detected.
  triggers = {
    image_id = each.value.id
  }
}