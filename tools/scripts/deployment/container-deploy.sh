#!/usr/bin/env bash

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
# Wrapper script to prepare and run containerized platform deployment.

set -euo pipefail

# Configuration
IMAGE_NAME="horizon-sdv-deployer:latest"
CONTAINER_HOSTNAME="horizon-deployer"
GCLOUD_CONFIG_VOLUME="horizon-deployer-gcloud-config"

REPO_ROOT="$(git rev-parse --show-toplevel)"
LOCAL_TFVARS="${REPO_ROOT}/terraform/env/terraform.tfvars"
DOCKER_DIR="./container"

if [[ ! -f "$LOCAL_TFVARS" ]]; then
  echo "Error: Configuration file not found at $LOCAL_TFVARS"
  exit 1
fi

# Build & Volume Setup
if [[ "$(docker images -q $IMAGE_NAME 2> /dev/null)" == "" ]]; then
  echo "Building deployer image..."
  docker build --no-cache -t $IMAGE_NAME -f $DOCKER_DIR/Dockerfile .
fi

if [[ "$(docker volume ls -q -f name=$GCLOUD_CONFIG_VOLUME)" == "" ]]; then
  docker volume create $GCLOUD_CONFIG_VOLUME
fi

echo "Starting deployment shell..."

docker run --rm -it \
  --hostname $CONTAINER_HOSTNAME \
  -u 0 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$GCLOUD_CONFIG_VOLUME:/root/.config/gcloud" \
  -v "$LOCAL_TFVARS:/tmp/terraform.tfvars" \
  -w /workspace \
  $IMAGE_NAME \
  /usr/local/bin/deploy.sh "$@"