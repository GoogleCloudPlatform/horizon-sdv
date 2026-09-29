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
# Verifies a candidate snapshot by booting a throwaway Cloud Workstation from
# it and running the BATS acceptance suite (files/test_artifacts.bats) on that
# workstation.
#
# Both the temporary config and the temporary workstation are removed on exit,
# success or failure.
#
# Why gcloud and not Config Connector (KCC): see build-snapshot.sh.
#
# Inputs (environment):
#   PROJECT           GCP project id
#   REGION            GCP region
#   CLUSTER           Cloud Workstations cluster name
#   CONFIG            Name to use for the *temporary* tester config
#   WORKFLOW_UID      Argo workflow UID, used to name per-run resources
#   CANDIDATE_SNAP    Snapshot to boot the tester from
#   CONTAINER_IMAGE   Workstation container image to test
#   MACHINE_TYPE      Tester machine type
#   ACCELERATOR_TYPE  Tester GPU type
#   ACCELERATOR_COUNT Tester GPU count
#   BATS_FILE         Path to the BATS suite to execute (mounted from ConfigMap)

set -eu

: "${PROJECT:?PROJECT is required}"
: "${REGION:?REGION is required}"
: "${CLUSTER:?CLUSTER is required}"
: "${CONFIG:?CONFIG is required}"
: "${WORKFLOW_UID:?WORKFLOW_UID is required}"
: "${CANDIDATE_SNAP:?CANDIDATE_SNAP is required}"
: "${CONTAINER_IMAGE:?CONTAINER_IMAGE is required}"
: "${MACHINE_TYPE:?MACHINE_TYPE is required}"
: "${ACCELERATOR_TYPE:?ACCELERATOR_TYPE is required}"
: "${ACCELERATOR_COUNT:?ACCELERATOR_COUNT is required}"
: "${BATS_FILE:?BATS_FILE is required}"

TEST_WS="aaos-sdv-tester-${WORKFLOW_UID}"
REMOTE_BATS="/tmp/test_artifacts.bats"

if [ ! -f "${BATS_FILE}" ]; then
  echo "ERROR: BATS suite not found at ${BATS_FILE}." >&2
  exit 1
fi

# gcloud workstations ssh wraps ssh inside a local TCP tunnel and has been
# observed to return exit status 0 even when the remote command exits non-zero.
# Emit an explicit exit-code sentinel from the remote shell and return that
# status so a failing bats run always fails the step.
ws_ssh() {
  OUT=$(gcloud workstations ssh "${TEST_WS}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --config="${CONFIG}" --user=user --quiet \
      --command="( $1 ); printf '\n__WS_SSH_RC__:%d\n' \$?")
  printf '%s\n' "${OUT}" | sed '/^__WS_SSH_RC__:[0-9]*$/d'
  RC=$(printf '%s\n' "${OUT}" | sed -n 's/^__WS_SSH_RC__:\([0-9]*\)$/\1/p' | tail -n 1)
  return "${RC:-1}"
}

cleanup() {
  echo "Cleaning up test workstation and temporary config..."
  gcloud workstations delete "${TEST_WS}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --config="${CONFIG}" --quiet || true
  gcloud workstations configs delete "${CONFIG}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --quiet || true
}

echo "Creating temporary tester config ${CONFIG} from snapshot ${CANDIDATE_SNAP}..."
gcloud workstations configs create "${CONFIG}" \
    --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
    --machine-type="${MACHINE_TYPE}" \
    --enable-nested-virtualization \
    --accelerator-type="${ACCELERATOR_TYPE}" \
    --accelerator-count="${ACCELERATOR_COUNT}" \
    --container-custom-image="${CONTAINER_IMAGE}" \
    --pd-disk-type="pd-ssd" \
    --pd-source-snapshot="projects/${PROJECT}/global/snapshots/${CANDIDATE_SNAP}"

# Only arm the trap once the config exists, so a failure to create it does not
# trigger a delete of something we never made.
trap cleanup EXIT

echo "Creating test workstation ${TEST_WS}..."
gcloud workstations create "${TEST_WS}" \
    --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
    --config="${CONFIG}"

echo "Starting test workstation..."
STARTED=false
for attempt in 1 2 3 4 5; do
  if gcloud workstations start "${TEST_WS}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --config="${CONFIG}"; then
    STARTED=true
    break
  fi
  echo "Retrying workstation start in 20s (attempt ${attempt}/5)..."
  sleep 20
done
if [ "${STARTED}" != "true" ]; then
  echo "ERROR: failed to start test workstation after 5 attempts." >&2
  exit 1
fi

echo "Waiting for SSH readiness on ${TEST_WS}..."
SSH_OK=false
for attempt in $(seq 1 30); do
  if gcloud workstations ssh "${TEST_WS}" \
      --project="${PROJECT}" --region="${REGION}" --cluster="${CLUSTER}" \
      --config="${CONFIG}" --user=user --quiet \
      --command="echo SSH_READY" 2>/dev/null | grep -q "SSH_READY"; then
    SSH_OK=true
    break
  fi
  sleep 5
done
if [ "${SSH_OK}" != "true" ]; then
  echo "ERROR: failed to establish SSH connection to ${TEST_WS} after 30 attempts." >&2
  exit 1
fi

# Copy the suite over as a real file rather than inlining it into the ssh
# command. base64 is used because its alphabet contains no shell metacharacters,
# so the payload survives the ssh command string untouched regardless of what
# the test content looks like.
echo "Uploading BATS suite to ${TEST_WS}:${REMOTE_BATS}..."
SUITE_B64=$(base64 < "${BATS_FILE}" | tr -d '\n')
ws_ssh "printf '%s' '${SUITE_B64}' | base64 -d > '${REMOTE_BATS}' && chmod 0644 '${REMOTE_BATS}'"

echo "Checking the bats harness is available on the workstation..."
if ! ws_ssh "command -v bats >/dev/null 2>&1"; then
  echo "ERROR: 'bats' is not installed in the workstation image." >&2
  echo "It is expected to be provided by the aaos-sdv-demos Dockerfile." >&2
  exit 1
fi

echo "Running acceptance suite..."
ws_ssh "bats --print-output-on-failure '${REMOTE_BATS}'"

echo "Artifacts verified successfully."
