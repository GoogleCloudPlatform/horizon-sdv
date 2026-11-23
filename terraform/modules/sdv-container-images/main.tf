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

locals {
  # Effective Docker build context per image (default: folder under images/).
  docker_context = {
    for name, img in var.images : name => coalesce(try(img.context_path, null), "${path.module}/images/${img.directory}/${name}")
  }

  horizon_cli_root = abspath("${path.module}/../../../tools/horizon")
  horizon_cli_files = toset([
    for f in fileset(local.horizon_cli_root, "**") : f
    if !endswith(f, "_test.go") && (
    endswith(f, ".go") || endswith(f, ".mod") || endswith(f, ".sum") || endswith(f, ".json")
    )
  ])
  portal_image_key = "horizon-dev-portal"

  # Shared Go auth library. Each image below replaces it in go.mod with ../../shared/horizonauth,
  # so it is copied into the image's build context as shared/horizonauth/.
  horizonauth_root = abspath("${path.module}/images/shared/horizonauth")
  horizonauth_files = sort([
    for f in fileset(local.horizonauth_root, "**") : f
    if !endswith(f, "_test.go") && (endswith(f, ".go") || endswith(f, ".mod") || endswith(f, ".sum"))
  ])
  horizonauth_image_keys = toset(["horizon-api-app", "module-manager-app"])

  # The portal's cli/ is written during apply by local_file.horizon_cli_docker_src, so it must never be listed:
  # Terraform fails when fileset() returns a different result at apply than at plan.
  # Add any new top-level portal source directory here, or changes to it will not trigger a rebuild.
  portal_context_patterns = ["*", "proxy/**", "public/**", "src/**"]

  # When context_path is set, skip hashing paths that match local npm/build artifacts (like .dockerignore).
  docker_context_files = {
    for name, img in var.images : name => sort([
      for f in setunion([
        for p in(name == local.portal_image_key ? local.portal_context_patterns : ["**"]) : fileset(local.docker_context[name], p)
      ]...) : f
      # shared/ is written during apply by local_file.horizonauth_docker_src; hashed via horizonauth_files instead.
      if !(contains(local.horizonauth_image_keys, name) && startswith(f, "shared/")) && (try(img.context_path, null) == null ? true : !(
        startswith(f, "node_modules/") || startswith(f, "dist/") || startswith(f, ".git/") || startswith(f, "cli/")
      ))
    ])
  }
}

resource "local_file" "horizonauth_docker_src" {
  for_each = {
    for pair in setproduct(setintersection(local.horizonauth_image_keys, keys(var.images)), local.horizonauth_files) :
    "${pair[0]}/${pair[1]}" => { image = pair[0], file = pair[1] }
  }

  content  = file("${local.horizonauth_root}/${each.value.file}")
  filename = "${local.docker_context[each.value.image]}/shared/horizonauth/${each.value.file}"
}

# Copy Horizon CLI sources into the portal Docker context (legacy builder cannot use additional_contexts).
resource "local_file" "horizon_cli_docker_src" {
  for_each = contains(keys(var.images), local.portal_image_key) ? {
    for f in local.horizon_cli_files : f => f
  } : {}

  content  = file("${local.horizon_cli_root}/${each.value}")
  filename = "${local.docker_context[local.portal_image_key]}/cli/${each.value}"
}

resource "docker_image" "sdv-container-images" {
  for_each = var.images

  depends_on = [local_file.horizon_cli_docker_src, local_file.horizonauth_docker_src]

  name = "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/${each.key}:${each.value.version}"
  build {
    no_cache = true

    context    = local.docker_context[each.key]
    tag        = ["${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/${var.gcp_registry_id}/${each.key}:${each.value.version}"]
    build_args = each.value.build_args

    # Default "Dockerfile" in context; override with absolute path when context is external.
    dockerfile = coalesce(try(each.value.dockerfile_path, null), "Dockerfile")
    platform   = try(each.value.platform, null)
  }

  triggers = {
    dir_sha1 = sha1(join("", concat(
      [for f in local.docker_context_files[each.key] : filesha1("${local.docker_context[each.key]}/${f}")],
      contains(local.horizonauth_image_keys, each.key) ? [for f in local.horizonauth_files : filesha1("${local.horizonauth_root}/${f}")] : [],
    )))

    build_args_sha = sha1(jsonencode(each.value.build_args))

    dockerfile_sha = try(each.value.dockerfile_path, null) != null ? filesha1(each.value.dockerfile_path) : ""

    platform_sha = try(each.value.platform, null) != null ? each.value.platform : ""

    horizon_cli_sha = each.key == local.portal_image_key ? sha1(join("", [
      for f in sort(tolist(local.horizon_cli_files)) :
      filesha1("${local.horizon_cli_root}/${f}")
    ])) : ""
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
