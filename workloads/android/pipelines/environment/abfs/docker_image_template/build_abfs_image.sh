#!/bin/sh
# Copyright (c) 2025-2026 Accenture, All Rights Reserved.
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

# Shared BuildKit invocation for ABFS Docker Image Template (Ubuntu or COS-tagged).
# Required env: DESTINATION, PUSH, EFFECTIVE_IMAGE_TAG, EFFECTIVE_LINUX_DISTRIBUTION,
# TEMPLATE_FLAVOR, plus ABFS/Google registry and tool version vars from the Jenkins job.
# POSIX sh only: this runs in the moby/buildkit container (BusyBox, no bash).
set -eu

: "${DESTINATION:?DESTINATION is required}"
: "${PUSH:?PUSH is required}"
: "${EFFECTIVE_IMAGE_TAG:?EFFECTIVE_IMAGE_TAG is required}"
: "${EFFECTIVE_LINUX_DISTRIBUTION:?EFFECTIVE_LINUX_DISTRIBUTION is required}"
: "${TEMPLATE_FLAVOR:?TEMPLATE_FLAVOR is required}"
: "${WORKSPACE:?WORKSPACE is required}"

if [ -z "${EFFECTIVE_IMAGE_TAG}" ]; then
  echo "ERROR: image tag for flavor ${TEMPLATE_FLAVOR} is empty."
  exit 1
fi

if [ -n "${ABFS_DISTRIBUTION_REGISTRY:-}" ]; then
  EFFECTIVE_ABFS_DISTRIBUTION_REGISTRY="${ABFS_DISTRIBUTION_REGISTRY}"
else
  if [ -z "${ABFS_REPOSITORY:-}" ]; then
    echo "ERROR: ABFS_REPOSITORY is required when ABFS_DISTRIBUTION_REGISTRY is empty."
    exit 1
  fi
  EFFECTIVE_ABFS_DISTRIBUTION_REGISTRY="ar+https://us-apt.pkg.dev/projects/abfs-binaries ${ABFS_REPOSITORY} main"
fi

echo "Building ABFS template flavor=${TEMPLATE_FLAVOR}"
echo "  tag=${EFFECTIVE_IMAGE_TAG}"
echo "  LINUX_DISTRIBUTION=${EFFECTIVE_LINUX_DISTRIBUTION}"
echo "  ABFS_DISTRIBUTION_REGISTRY=${EFFECTIVE_ABFS_DISTRIBUTION_REGISTRY}"
echo "  destination=${DESTINATION}:${EFFECTIVE_IMAGE_TAG} push=${PUSH}"

cd "${WORKSPACE}/workloads/android/pipelines/environment/docker_image_template/"
buildctl build \
  --frontend=dockerfile.v0 \
  --local=dockerfile=. \
  --local context=. \
  --opt build-arg:ABFS="true" \
  --opt build-arg:LINUX_DISTRIBUTION="${EFFECTIVE_LINUX_DISTRIBUTION}" \
  --opt build-arg:NODEJS_VERSION="${NODEJS_VERSION}" \
  --opt build-arg:GOOGLE_DISTRIBUTION_REGISTRY="${GOOGLE_DISTRIBUTION_REGISTRY}" \
  --opt build-arg:ABFS_DISTRIBUTION_REGISTRY="${EFFECTIVE_ABFS_DISTRIBUTION_REGISTRY}" \
  --opt build-arg:GCLOUD_CLI_VERSION="${GCLOUD_CLI_VERSION}" \
  --opt build-arg:KUBECTL_VERSION="${KUBECTL_VERSION}" \
  --opt build-arg:PACKER_VERSION="${PACKER_VERSION}" \
  --opt build-arg:ENABLE_GEMINI_AI_ASSISTANT="${ENABLE_GEMINI_AI_ASSISTANT}" \
  --opt build-arg:GEMINI_CLI_VERSION="${GEMINI_CLI_VERSION}" \
  --opt build-arg:GOOGLE_ADK_VERSION="${GOOGLE_ADK_VERSION}" \
  --opt platform=linux/amd64 \
  --registry-auth-tlscontext "host=${REGISTRY},insecure=true" \
  --output="type=image,name=${DESTINATION}:${EFFECTIVE_IMAGE_TAG},push=${PUSH}"

echo "Finished ABFS template flavor=${TEMPLATE_FLAVOR} tag=${EFFECTIVE_IMAGE_TAG}"
