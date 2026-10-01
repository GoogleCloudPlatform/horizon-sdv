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
# Promotes a verified candidate snapshot and publishes a ready-to-use
# workstation configuration built from it.
#
# Two things happen here:
#   1. The "is-latest" label is moved from whatever previously held it onto the
#      candidate, so downstream consumers can always resolve the current image
#      by label rather than by name.
#   2. A long-lived workstation config is created (or updated) that boots from
#      the promoted snapshot with the freshly built container image. Without
#      this the pipeline would produce a snapshot that nobody can easily launch.
#
# Why gcloud and not Config Connector (KCC): the cluster runs the GKE
# Config Connector add-on at v1.126.0, which has no WorkstationConfig CRD
# (upstream KCC only promoted it to v1beta1 in 1.132.0). A declarative CR would
# genuinely be the better fit for this particular long-lived resource, so this
# is worth revisiting if the cluster ever moves to a newer Config Connector.
#
# Inputs (environment):
#   PROJECT           GCP project id
#   REGION            GCP region
#   CLUSTER           Cloud Workstations cluster name
#   CANDIDATE_SNAP    Verified snapshot to promote
#   PROMOTED_CONFIG   Name of the long-lived workstation config to publish
#   CONTAINER_IMAGE   Workstation container image to publish
#   MACHINE_TYPE      Machine type for the published config
#   ACCELERATOR_TYPE  GPU type for the published config
#   ACCELERATOR_COUNT GPU count for the published config

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

SNAPSHOT_URI="projects/${PROJECT}/global/snapshots/${CANDIDATE_SNAP}"

echo "Publishing workstation config ${PROMOTED_CONFIG}..."
if gcloud workstations configs describe "${PROMOTED_CONFIG}" \
    --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
    >/dev/null 2>&1; then
  echo "  config exists; updating it in place"
  # Existing workstations keep their own disks; only newly created ones pick up
  # the new snapshot. Updating rather than recreating avoids disrupting anyone
  # who already has a workstation on this config.
  if ! gcloud workstations configs update "${PROMOTED_CONFIG}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --enable-nested-virtualization \
      --container-custom-image="${CONTAINER_IMAGE}" \
      --pd-source-snapshot="${SNAPSHOT_URI}"; then
    echo "ERROR: could not update ${PROMOTED_CONFIG}." >&2
    echo "The snapshot has still been promoted and is available as ${CANDIDATE_SNAP}." >&2
    echo "The config may need to be recreated manually if its disk source is immutable." >&2
    exit 1
  fi
else
  echo "  config does not exist; creating it"
  gcloud workstations configs create "${PROMOTED_CONFIG}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --machine-type="${MACHINE_TYPE}" \
      --enable-nested-virtualization \
      --accelerator-type="${ACCELERATOR_TYPE}" \
      --accelerator-count="${ACCELERATOR_COUNT}" \
      --container-custom-image="${CONTAINER_IMAGE}" \
      --pd-disk-type="pd-ssd" \
      --pd-source-snapshot="${SNAPSHOT_URI}"
fi

echo "Promoted ${CANDIDATE_SNAP} and published workstation config ${PROMOTED_CONFIG}."
