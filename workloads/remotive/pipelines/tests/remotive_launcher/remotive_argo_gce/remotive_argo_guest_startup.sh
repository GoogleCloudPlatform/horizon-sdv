#!/usr/bin/env bash
#
# Copyright (c) 2026 RemotiveLabs, All Rights Reserved.
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
#   First script that runs on the ephemeral remotive (RemotiveTopology) VM (GCE
#   metadata startup) — same shape as cvd_argo_gce/cvd_argo_guest_startup.sh.
#
#   Flow:
#     1. Read GCS URIs from instance metadata.
#     2. Download shared helpers and job inputs from GCS.
#     3. Unpack the pipeline tarball into a workspace directory.
#     4. Run remotive_argo_remote_entry.sh twice: main, then teardown.
#     5. Upload status.json and remotive-argo-artifacts.tgz back to GCS.
#
#   The Argo workflow pod creates the VM, uploads inputs, polls status.json, and
#   never SSHs to the guest. Live logs come from serial port 2.

set -euo pipefail

function _metadata_attr() {
  curl -fsS -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/${1}"
}

if [[ -z "${REMOTIVE_ARGO_INPUT_URI:-}" ]]; then
  REMOTIVE_ARGO_INPUT_URI="$(_metadata_attr remotive-argo-input-uri)"
fi
if [[ -z "${REMOTIVE_ARGO_OUTPUT_URI:-}" ]]; then
  REMOTIVE_ARGO_OUTPUT_URI="$(_metadata_attr remotive-argo-output-uri)"
fi
if [[ -z "${REMOTIVE_ARGO_VM_NAME:-}" ]]; then
  REMOTIVE_ARGO_VM_NAME="$(_metadata_attr remotive-argo-vm-name)"
fi
if [[ -z "${REMOTIVE_ARGO_APP_SERIAL_DEV:-}" ]]; then
  REMOTIVE_ARGO_APP_SERIAL_DEV="$(_metadata_attr remotive-argo-app-serial-dev 2>/dev/null || true)"
fi

: "${REMOTIVE_ARGO_INPUT_URI:?}"
: "${REMOTIVE_ARGO_OUTPUT_URI:?}"
: "${REMOTIVE_ARGO_VM_NAME:?}"

REMOTE_STAGING="/tmp/remotive-argo-ws-${REMOTIVE_ARGO_VM_NAME}"
REMOTE_ARTIFACT_TGZ="/tmp/remotive-argo-artifacts.tgz"
STATUS_LOCAL="/tmp/remotive-argo-guest-status.json"
LOG_LOCAL="/tmp/remotive-argo-guest-startup.log"

gcloud storage cp "${REMOTIVE_ARGO_INPUT_URI}/remotive_argo_guest_common.sh" /tmp/remotive_argo_guest_common.sh
# shellcheck source=/dev/null
source /tmp/remotive_argo_guest_common.sh

function _write_status() {
  local phase="${1:?}" rc="${2:?}" msg="${3:-}"
  python3 -c 'import json,sys; json.dump({"phase":sys.argv[1],"rc":int(sys.argv[2]),"message":sys.argv[3]}, open(sys.argv[4],"w"), separators=(",",":"))' \
    "${phase}" "${rc}" "${msg}" "${STATUS_LOCAL}"
  gcloud storage cp "${STATUS_LOCAL}" "${REMOTIVE_ARGO_OUTPUT_URI}/status.json" >/dev/null 2>&1 || true
}

function _upload_guest_outputs_to_gcs() {
  if [[ -f "${REMOTE_ARTIFACT_TGZ}" ]]; then
    gcloud storage cp "${REMOTE_ARTIFACT_TGZ}" "${REMOTIVE_ARGO_OUTPUT_URI}/remotive-argo-artifacts.tgz"
  fi
}

# shellcheck disable=SC2329
function _on_err() {
  local ec=$?
  _write_status failed "${ec}" "guest startup error (see serial port 2 or ${LOG_LOCAL})"
  exit "${ec}"
}

APP_SERIAL_DEV="${REMOTIVE_ARGO_APP_SERIAL_DEV:-/dev/ttyS1}"
_remotive_argo_setup_stdio_redirect "${APP_SERIAL_DEV}" "${LOG_LOCAL}" remotive-argo-guest
_trace "[remotive-argo-guest] startup serial=${APP_SERIAL_DEV}"

trap _on_err ERR

gcloud storage cp "${REMOTIVE_ARGO_INPUT_URI}/remotive-argo-job-env.sh" /tmp/remotive-argo-job-env.sh
gcloud storage cp "${REMOTIVE_ARGO_INPUT_URI}/remotive-argo-workloads.tgz" /tmp/remotive-argo-workloads.tgz

sudo rm -rf "${REMOTE_STAGING}"
sudo mkdir -p "${REMOTE_STAGING}"
sudo tar xzf /tmp/remotive-argo-workloads.tgz -C "${REMOTE_STAGING}"
sudo chown -R "$(id -un):$(id -gn)" "${REMOTE_STAGING}"

# shellcheck disable=SC1091
set -a && source /tmp/remotive-argo-job-env.sh && set +a
_remotive_argo_ensure_home

export WORKSPACE="${REMOTE_STAGING}"
export REMOTIVE_ARGO_OUTPUT_URI
export REMOTE_ARTIFACT_TGZ

REMOTE_ENTRY="${REMOTE_STAGING}/workloads/remotive/pipelines/tests/remotive_launcher/remotive_argo_gce/remotive_argo_remote_entry.sh"
cd "${REMOTE_STAGING}"

# Always run teardown after main, even when main fails, so we stop the topology
# and gather compose logs.
_write_status running 0 "main phase"
export REMOTIVE_ARGO_REMOTE_PHASE=main
# Use "cmd || MAIN_RC=$?" — $? inside "if ! cmd; then" is 0 (if-test success), not cmd's exit code.
MAIN_RC=0
bash "${REMOTE_ENTRY}" || MAIN_RC=$?
if [[ "${MAIN_RC}" -ne 0 ]]; then
  # Keep phase=running until teardown + artifact upload finish so the workflow pod
  # does not delete the VM while logs are still being gathered.
  _write_status running 0 "teardown after main failure"
  export REMOTIVE_ARGO_REMOTE_PHASE=teardown
  TEARDOWN_RC=0
  bash "${REMOTE_ENTRY}" || TEARDOWN_RC=$?
  _upload_guest_outputs_to_gcs
  _write_status failed "${MAIN_RC}" "remote main failed (teardown=${TEARDOWN_RC})"
  exit "${MAIN_RC}"
fi

_write_status running 0 "teardown phase"
export REMOTIVE_ARGO_REMOTE_PHASE=teardown
TEARDOWN_RC=0
bash "${REMOTE_ENTRY}" || TEARDOWN_RC=$?

_upload_guest_outputs_to_gcs

if [[ "${TEARDOWN_RC}" -eq 0 ]]; then
  _write_status success 0 "complete"
else
  _write_status failed "${TEARDOWN_RC}" "main=0 teardown=${TEARDOWN_RC}"
fi
exit "${TEARDOWN_RC}"
