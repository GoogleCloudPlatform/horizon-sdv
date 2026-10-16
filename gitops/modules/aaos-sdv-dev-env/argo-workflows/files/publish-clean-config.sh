#!/bin/sh
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
# Publishes clean persistent-disk Cloud Workstation configurations (no source
# snapshot, fresh 1000 GB pd-ssd volume) immediately after the container image
# build completes. This allows partners and developers to launch a GPU-enabled
# Selkies + Openbox + Antigravity 2.0 workstation right away while the multi-hour
# CARLA + AAOS 26Q2 snapshot build runs in parallel.

set -eu

: "${PROJECT:?PROJECT is required}"
: "${REGION:?REGION is required}"
: "${CLUSTER:?CLUSTER is required}"
: "${CLEAN_CONFIG:?CLEAN_CONFIG is required}"
: "${CLEAN_PD_SIZE_GB:?CLEAN_PD_SIZE_GB is required}"
: "${CONTAINER_IMAGE:?CONTAINER_IMAGE is required}"
: "${MACHINE_TYPE:?MACHINE_TYPE is required}"
: "${ACCELERATOR_TYPE:?ACCELERATOR_TYPE is required}"
: "${ACCELERATOR_COUNT:?ACCELERATOR_COUNT is required}"
: "${HOST_SA_EMAIL:?HOST_SA_EMAIL is required}"

SECONDARY_REGION="${SECONDARY_REGION:-}"
SECONDARY_CLUSTER="${SECONDARY_CLUSTER:-}"
SECONDARY_CLEAN_CONFIG="${SECONDARY_CLEAN_CONFIG:-}"

publish_clean_config_in_cluster() {
  TARGET_REGION="$1"
  TARGET_CLUSTER="$2"
  TARGET_CONFIG="$3"

  RECONCILING=$(gcloud workstations clusters describe "${TARGET_CLUSTER}" \
      --project="${PROJECT}" --region="${TARGET_REGION}" \
      --format="value(reconciling)" 2>/dev/null || echo "NOT_FOUND")

  if [ "${RECONCILING}" = "NOT_FOUND" ]; then
    echo "WARNING: cluster ${TARGET_CLUSTER} in ${TARGET_REGION} not found; skipping ${TARGET_CONFIG}."
    return 0
  fi

  if [ "${RECONCILING}" = "True" ] || [ "${RECONCILING}" = "true" ]; then
    echo "Waiting for cluster ${TARGET_CLUSTER} in ${TARGET_REGION} to finish reconciling..."
    for attempt in $(seq 1 40); do
      RECONCILING=$(gcloud workstations clusters describe "${TARGET_CLUSTER}" \
          --project="${PROJECT}" --region="${TARGET_REGION}" \
          --format="value(reconciling)" 2>/dev/null || echo "True")
      if [ "${RECONCILING}" != "True" ] && [ "${RECONCILING}" != "true" ]; then
        break
      fi
      sleep 15
    done
  fi

  echo "Publishing clean-PD workstation config ${TARGET_CONFIG} in ${TARGET_REGION}/${TARGET_CLUSTER}..."
  if gcloud workstations configs describe "${TARGET_CONFIG}" \
      --project="${PROJECT}" --region="${TARGET_REGION}" --cluster="${TARGET_CLUSTER}" \
      >/dev/null 2>&1; then
    echo "  config ${TARGET_CONFIG} exists; updating image to ${CONTAINER_IMAGE}"
    gcloud workstations configs update "${TARGET_CONFIG}" \
        --project="${PROJECT}" --region="${TARGET_REGION}" --cluster="${TARGET_CLUSTER}" \
        --enable-nested-virtualization \
        --container-custom-image="${CONTAINER_IMAGE}" \
        --quiet
    return 0
  fi

  echo "  config ${TARGET_CONFIG} does not exist; creating it"
  gcloud workstations configs create "${TARGET_CONFIG}" \
      --project="${PROJECT}" --region="${TARGET_REGION}" --cluster="${TARGET_CLUSTER}" \
      --machine-type="${MACHINE_TYPE}" \
      --enable-nested-virtualization \
      --accelerator-type="${ACCELERATOR_TYPE}" \
      --accelerator-count="${ACCELERATOR_COUNT}" \
      --service-account="${HOST_SA_EMAIL}" \
      --disable-public-ip-addresses \
      --pool-size=0 \
      --container-custom-image="${CONTAINER_IMAGE}" \
      --pd-disk-type="pd-ssd" \
      --pd-disk-size="${CLEAN_PD_SIZE_GB}" \
      --pd-reclaim-policy="retain" \
      --quiet
}

publish_clean_config_in_cluster "${REGION}" "${CLUSTER}" "${CLEAN_CONFIG}"

if [ -n "${SECONDARY_REGION}" ] && [ -n "${SECONDARY_CLUSTER}" ] && [ -n "${SECONDARY_CLEAN_CONFIG}" ]; then
  publish_clean_config_in_cluster "${SECONDARY_REGION}" "${SECONDARY_CLUSTER}" "${SECONDARY_CLEAN_CONFIG}"
fi

echo "Clean-PD workstation configs published."
