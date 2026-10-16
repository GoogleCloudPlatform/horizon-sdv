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
# Builds a candidate persistent-disk snapshot for the aaos-sdv-dev-env workstation
# image, or slots in an existing snapshot when EXISTING_SNAPSHOT_NAME is set.
#
# A workstation is created from the builder config and started once, which is
# what materialises and populates its regional persistent disk (the CARLA
# installation and the AAOS tree live on that disk, not in the container
# image). The workstation is then stopped so the disk is quiesced, snapshotted,
# and the workstation torn down again.
#
# Inputs (environment):
#   PROJECT                 GCP project id
#   REGION                  GCP region
#   CLUSTER                 Cloud Workstations cluster name
#   CONFIG                  Cloud Workstations builder config name
#   WORKFLOW_UID            Argo workflow UID, used to name per-run resources
#   CONTAINER_IMAGE         Freshly built workstation container image
#   BUILDER_MACHINE_TYPE    Machine type for the builder config (default: n2-standard-64)
#   BUILDER_PD_SIZE_GB      Persistent disk size in GB for builder (default: 1000)
#   HOST_SA_EMAIL           Host VM service account email
#   EXISTING_SNAPSHOT_NAME  Optional pre-existing snapshot name to slot in directly
#
# Output:
#   Writes the candidate snapshot name to /tmp/candidate_snapshot_name.

set -eu

: "${PROJECT:?PROJECT is required}"
: "${REGION:?REGION is required}"
: "${CLUSTER:?CLUSTER is required}"
: "${CONFIG:?CONFIG is required}"
: "${WORKFLOW_UID:?WORKFLOW_UID is required}"
: "${CONTAINER_IMAGE:?CONTAINER_IMAGE is required}"
: "${HOST_SA_EMAIL:?HOST_SA_EMAIL is required}"

BUILDER_MACHINE_TYPE="${BUILDER_MACHINE_TYPE:-n2-standard-64}"
BUILDER_PD_SIZE_GB="${BUILDER_PD_SIZE_GB:-1000}"
EXISTING_SNAPSHOT_NAME="${EXISTING_SNAPSHOT_NAME:-}"

if [ -n "${EXISTING_SNAPSHOT_NAME}" ]; then
  echo "Existing snapshot parameter provided: ${EXISTING_SNAPSHOT_NAME}"
  echo "Verifying snapshot ${EXISTING_SNAPSHOT_NAME} exists in project ${PROJECT}..."
  gcloud compute snapshots describe "${EXISTING_SNAPSHOT_NAME}" \
      --project="${PROJECT}" >/dev/null
  printf '%s' "${EXISTING_SNAPSHOT_NAME}" > /tmp/candidate_snapshot_name
  echo "Using existing snapshot as candidate: ${EXISTING_SNAPSHOT_NAME}"
  exit 0
fi

BUILD_WS="aaos-sdv-builder-${WORKFLOW_UID}"
CANDIDATE_SNAP="aaos-sdv-dev-env-pd-snap-${WORKFLOW_UID}"

# gcloud workstations ssh wraps ssh inside a local TCP tunnel and can return
# exit status 0 even when the remote command exits non-zero. Stream the remote
# output and capture a sentinel exit code file so any build failure aborts the
# step immediately.
ws_ssh() {
  RC_FILE="/tmp/ws_ssh_rc.$$"
  rm -f "${RC_FILE}"
  gcloud workstations ssh "${BUILD_WS}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --config="${CONFIG}" --user=user --quiet \
      --ssh-flag="-o ServerAliveInterval=30" \
      --ssh-flag="-o ServerAliveCountMax=20" \
      --command="( $1 ); printf '\n__WS_SSH_RC__:%d\n' \$?" | while IFS= read -r line; do
    case "${line}" in
      __WS_SSH_RC__:*)
        printf '%s' "${line#__WS_SSH_RC__:}" > "${RC_FILE}"
        ;;
      *)
        printf '%s\n' "${line}"
        ;;
    esac
  done
  RC=$(cat "${RC_FILE}" 2>/dev/null || echo 1)
  rm -f "${RC_FILE}"
  return "${RC}"
}

wait_for_ssh_ready() {
  echo "Waiting for SSH readiness on ${BUILD_WS}..."
  for attempt in $(seq 1 30); do
    if gcloud workstations ssh "${BUILD_WS}" \
        --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
        --config="${CONFIG}" --user=user --quiet \
        --command="echo SSH_READY" 2>/dev/null | grep -q "SSH_READY"; then
      echo "SSH connection established on ${BUILD_WS}."
      return 0
    fi
    sleep 5
  done
  echo "ERROR: failed to establish SSH connection to ${BUILD_WS} after 30 attempts." >&2
  return 1
}

ensure_builder_config() {
  if gcloud workstations configs describe "${CONFIG}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      >/dev/null 2>&1; then
    echo "Updating existing builder config ${CONFIG} to use image ${CONTAINER_IMAGE}..."
    gcloud workstations configs update "${CONFIG}" \
        --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
        --enable-nested-virtualization \
        --idle-timeout=43200s \
        --running-timeout=43200s \
        --container-custom-image="${CONTAINER_IMAGE}" \
        --quiet
    return 0
  fi

  echo "Creating builder config ${CONFIG} (${BUILDER_MACHINE_TYPE}, ${BUILDER_PD_SIZE_GB} GB pd-ssd)..."
  gcloud workstations configs create "${CONFIG}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --machine-type="${BUILDER_MACHINE_TYPE}" \
      --enable-nested-virtualization \
      --service-account="${HOST_SA_EMAIL}" \
      --disable-public-ip-addresses \
      --pool-size=0 \
      --idle-timeout=43200s \
      --running-timeout=43200s \
      --container-custom-image="${CONTAINER_IMAGE}" \
      --pd-disk-type="pd-ssd" \
      --pd-disk-size="${BUILDER_PD_SIZE_GB}" \
      --pd-reclaim-policy="delete" \
      --quiet
}

# Always try to remove the temporary builder workstation, including on failure,
# so a failed run does not leave a billable workstation behind.
cleanup() {
  echo "Cleaning up temporary builder workstation ${BUILD_WS}..."
  gcloud workstations delete "${BUILD_WS}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --config="${CONFIG}" --quiet || true
}

ensure_builder_config
trap cleanup EXIT

echo "Creating temporary builder workstation: ${BUILD_WS}..."
gcloud workstations create "${BUILD_WS}" \
    --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
    --config="${CONFIG}"

echo "Starting builder workstation..."
STARTED=false
for attempt in 1 2 3 4 5; do
  if gcloud workstations start "${BUILD_WS}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --config="${CONFIG}"; then
    STARTED=true
    break
  fi
  echo "Retrying builder workstation start in 20s (attempt ${attempt}/5)..."
  sleep 20
done
if [ "${STARTED}" != "true" ]; then
  echo "ERROR: failed to start builder workstation after 5 attempts." >&2
  exit 1
fi

wait_for_ssh_ready

echo "Step 1/2: Installing CARLA 0.9.15 and extra maps (Town15) onto persistent disk..."
ws_ssh "cd \"\${HOME}\" && . /google/recipe/base.sh && bash /google/carla915-utils/setup_carla.sh -y"

echo "Step 2/2: Checking out, patching, and building AAOS 26Q2 onto persistent disk..."
ws_ssh "cd \"\${HOME}\" && . /google/recipe/base.sh && bash -eo pipefail /google/aaos-utils/deploy_aaos_26q2_entry_point.sh"

echo "Stopping builder to quiesce its persistent disk before snapshotting..."
gcloud workstations stop "${BUILD_WS}" \
    --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
    --config="${CONFIG}" --quiet

echo "Looking up persistent disk for ${BUILD_WS}..."
SHORT_DISK_NAME=$(gcloud compute disks list \
    --project="${PROJECT}" \
    --filter="labels.workstation_id=${BUILD_WS}" \
    --format="value(name)" | head -n 1)

if [ -z "${SHORT_DISK_NAME}" ]; then
  echo "ERROR: could not find a persistent disk for workstation ${BUILD_WS}." >&2
  exit 1
fi
echo "Found persistent disk: ${SHORT_DISK_NAME}"

echo "Creating snapshot ${CANDIDATE_SNAP} from regional disk ${SHORT_DISK_NAME}..."
gcloud compute snapshots create "${CANDIDATE_SNAP}" \
    --source-disk="${SHORT_DISK_NAME}" \
    --source-disk-region="${REGION}" \
    --project="${PROJECT}"

printf '%s' "${CANDIDATE_SNAP}" > /tmp/candidate_snapshot_name
echo "Candidate snapshot ready: ${CANDIDATE_SNAP}"
