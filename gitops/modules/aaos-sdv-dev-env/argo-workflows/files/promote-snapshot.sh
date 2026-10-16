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
# Promotes a verified candidate snapshot and publishes ready-to-use workstation
# configurations built from it across both primary (europe-west3) and secondary
# (us-west4) Cloud Workstations clusters.
#
# Inputs (environment):
#   PROJECT                   GCP project id
#   REGION                    Primary GCP region
#   CLUSTER                   Primary Cloud Workstations cluster name
#   CANDIDATE_SNAP            Verified snapshot to promote
#   PROMOTED_CONFIG           Default long-lived workstation config name
#   PRIMARY_REGIONAL_CONFIG   Optional explicit primary regional config name
#   SECONDARY_REGION          Optional secondary GCP region (e.g. us-west4)
#   SECONDARY_CLUSTER         Optional secondary Cloud Workstations cluster name
#   SECONDARY_REGIONAL_CONFIG Optional secondary regional config name
#   CONTAINER_IMAGE           Workstation container image to publish
#   MACHINE_TYPE              Machine type for the published configs
#   ACCELERATOR_TYPE          GPU type for the published configs
#   ACCELERATOR_COUNT         GPU count for the published configs
#   HOST_SA_EMAIL             Host VM service account email

set -eu

: "${PROJECT:?PROJECT is required}"
: "${REGION:?REGION is required}"
: "${CLUSTER:?CLUSTER is required}"
: "${CANDIDATE_SNAP:?CANDIDATE_SNAP is required}"
: "${PROMOTED_CONFIG:?PROMOTED_CONFIG is required}"
: "${CONTAINER_IMAGE:?CONTAINER_IMAGE is required}"
: "${MACHINE_TYPE:?MACHINE_TYPE is required}"
: "${ACCELERATOR_TYPE:?ACCELERATOR_TYPE is required}"
: "${ACCELERATOR_COUNT:?ACCELERATOR_COUNT is required}"
: "${HOST_SA_EMAIL:?HOST_SA_EMAIL is required}"

PRIMARY_REGIONAL_CONFIG="${PRIMARY_REGIONAL_CONFIG:-}"
SECONDARY_REGION="${SECONDARY_REGION:-}"
SECONDARY_CLUSTER="${SECONDARY_CLUSTER:-}"
SECONDARY_REGIONAL_CONFIG="${SECONDARY_REGIONAL_CONFIG:-}"

SNAPSHOT_URI="projects/${PROJECT}/global/snapshots/${CANDIDATE_SNAP}"

upsert_promoted_config() {
  TARGET_REGION="$1"
  TARGET_CLUSTER="$2"
  TARGET_CONFIG="$3"

  echo "Publishing workstation config ${TARGET_CONFIG} in ${TARGET_REGION}/${TARGET_CLUSTER}..."
  if gcloud workstations configs describe "${TARGET_CONFIG}" \
      --project="${PROJECT}" --region="${TARGET_REGION}" --cluster="${TARGET_CLUSTER}" \
      >/dev/null 2>&1; then
    echo "  config ${TARGET_CONFIG} exists; updating it in place"
    if ! gcloud workstations configs update "${TARGET_CONFIG}" \
        --project="${PROJECT}" --region="${TARGET_REGION}" --cluster="${TARGET_CLUSTER}" \
        --enable-nested-virtualization \
        --container-custom-image="${CONTAINER_IMAGE}" \
        --pd-source-snapshot="${SNAPSHOT_URI}" \
        --quiet; then
      echo "ERROR: could not update ${TARGET_CONFIG} in ${TARGET_REGION}/${TARGET_CLUSTER}." >&2
      echo "The snapshot has still been promoted and is available as ${CANDIDATE_SNAP}." >&2
      exit 1
    fi
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
      --pd-reclaim-policy="retain" \
      --pd-source-snapshot="${SNAPSHOT_URI}" \
      --quiet
}

echo "Demoting any previously promoted snapshot..."
OLD_LATEST=$(gcloud compute snapshots list \
    --project="${PROJECT}" \
    --filter="labels.is-latest=true" \
    --format="value(name)" 2>/dev/null || true)
for old_snap in ${OLD_LATEST}; do
  if [ "${old_snap}" = "${CANDIDATE_SNAP}" ]; then
    continue
  fi
  echo "  removing is-latest from ${old_snap}"
  gcloud compute snapshots remove-labels "${old_snap}" \
      --labels="is-latest" --project="${PROJECT}" 2>/dev/null || true
done

echo "Promoting ${CANDIDATE_SNAP}..."
gcloud compute snapshots add-labels "${CANDIDATE_SNAP}" \
    --labels="is-latest=true,env=production" --project="${PROJECT}"

upsert_promoted_config "${REGION}" "${CLUSTER}" "${PROMOTED_CONFIG}"

if [ -n "${PRIMARY_REGIONAL_CONFIG}" ] && [ "${PRIMARY_REGIONAL_CONFIG}" != "${PROMOTED_CONFIG}" ]; then
  upsert_promoted_config "${REGION}" "${CLUSTER}" "${PRIMARY_REGIONAL_CONFIG}"
fi

if [ -n "${SECONDARY_REGION}" ] && [ -n "${SECONDARY_CLUSTER}" ] && [ -n "${SECONDARY_REGIONAL_CONFIG}" ]; then
  upsert_promoted_config "${SECONDARY_REGION}" "${SECONDARY_CLUSTER}" "${SECONDARY_REGIONAL_CONFIG}"
fi

echo "Promoted ${CANDIDATE_SNAP} and published workstation configs."
