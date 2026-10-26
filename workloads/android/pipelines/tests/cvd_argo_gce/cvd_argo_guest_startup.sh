#!/usr/bin/env bash
#
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
#   First script that runs on the ephemeral Cuttlefish VM (GCE metadata startup).
#
#   Flow:
#     1. Read GCS URIs and mode from instance metadata (or env if pre-set).
#     2. Download shared helpers and job inputs from GCS.
#     3. Unpack the pipeline tarball into a workspace directory.
#     4. Run cvd_argo_remote_entry.sh twice: main, then teardown.
#     5. Upload status.json and cvd-argo-artifacts.tgz back to GCS.
#
#   The Argo workflow pod creates the VM, uploads inputs, polls status.json, and
#   never SSHs to the guest. Live logs come from serial port 2 or Cloud Logging.

set -euo pipefail

# -----------------------------------------------------------------------------
# _metadata_attr
# -----------------------------------------------------------------------------
# Read one instance custom metadata key (set by the workflow pod on the VM).

function _metadata_attr() {
  curl -fsS -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/${1}"
}

# -----------------------------------------------------------------------------
# Bootstrap (metadata URIs, then shared guest helpers from GCS)
# -----------------------------------------------------------------------------
# Input/output prefixes live under the per-run GEMINI_TEST_RESULTS_STAGING_URI.

if [[ -z "${CVD_ARGO_INPUT_URI:-}" ]]; then
  CVD_ARGO_INPUT_URI="$(_metadata_attr cvd-argo-input-uri)"
fi
if [[ -z "${CVD_ARGO_OUTPUT_URI:-}" ]]; then
  CVD_ARGO_OUTPUT_URI="$(_metadata_attr cvd-argo-output-uri)"
fi
if [[ -z "${CVD_ARGO_MODE:-}" ]]; then
  CVD_ARGO_MODE="$(_metadata_attr cvd-argo-mode)"
fi
if [[ -z "${CVD_ARGO_VM_NAME:-}" ]]; then
  CVD_ARGO_VM_NAME="$(_metadata_attr cvd-argo-vm-name)"
fi
if [[ -z "${CVD_ARGO_APP_SERIAL_DEV:-}" ]]; then
  CVD_ARGO_APP_SERIAL_DEV="$(_metadata_attr cvd-argo-app-serial-dev 2>/dev/null || true)"
fi

: "${CVD_ARGO_INPUT_URI:?}"
: "${CVD_ARGO_OUTPUT_URI:?}"
: "${CVD_ARGO_MODE:?}"
: "${CVD_ARGO_VM_NAME:?}"

REMOTE_STAGING="/tmp/cvd-argo-ws-${CVD_ARGO_VM_NAME}"
REMOTE_ARTIFACT_TGZ="/tmp/cvd-argo-artifacts.tgz"
STATUS_LOCAL="/tmp/cvd-argo-guest-status.json"
LOG_LOCAL="/tmp/cvd-argo-guest-startup.log"

gcloud storage cp "${CVD_ARGO_INPUT_URI}/cvd_argo_guest_common.sh" /tmp/cvd_argo_guest_common.sh
# shellcheck source=/dev/null
source /tmp/cvd_argo_guest_common.sh

# -----------------------------------------------------------------------------
# _write_status
# -----------------------------------------------------------------------------
# Write phase/rc/message JSON to GCS; workflow pod polls status.json until done.

function _write_status() {
  local phase="${1:?}" rc="${2:?}" msg="${3:-}"
  python3 -c 'import json,sys; json.dump({"phase":sys.argv[1],"rc":int(sys.argv[2]),"message":sys.argv[3]}, open(sys.argv[4],"w"), separators=(",",":"))' \
    "${phase}" "${rc}" "${msg}" "${STATUS_LOCAL}"
  gcloud storage cp "${STATUS_LOCAL}" "${CVD_ARGO_OUTPUT_URI}/status.json" >/dev/null 2>&1 || true
}

# -----------------------------------------------------------------------------
# _upload_guest_outputs_to_gcs
# -----------------------------------------------------------------------------
# Upload artifact tarball and marker after teardown (success or main failure).

function _upload_guest_outputs_to_gcs() {
  if [[ -f "${REMOTE_ARTIFACT_TGZ}" ]]; then
    gcloud storage cp "${REMOTE_ARTIFACT_TGZ}" "${CVD_ARGO_OUTPUT_URI}/cvd-argo-artifacts.tgz"
  fi
  if [[ -f /tmp/cvd-argo.marker ]]; then
    gcloud storage cp /tmp/cvd-argo.marker "${CVD_ARGO_OUTPUT_URI}/cvd-argo.marker" >/dev/null 2>&1 || true
  fi
}

# -----------------------------------------------------------------------------
# _on_err
# -----------------------------------------------------------------------------
# ERR trap: mark guest startup failed in GCS before exiting.

# shellcheck disable=SC2329
function _on_err() {
  local ec=$?
  _write_status failed "${ec}" "guest startup error (see serial port 2 or ${LOG_LOCAL})"
  exit "${ec}"
}

# -----------------------------------------------------------------------------
# Guest stdio and ERR trap
# -----------------------------------------------------------------------------
# Tee logs to app serial device; trap writes failed status.json before exit.

APP_SERIAL_DEV="${CVD_ARGO_APP_SERIAL_DEV:-/dev/ttyS1}"
_cvd_argo_setup_stdio_redirect "${APP_SERIAL_DEV}" "${LOG_LOCAL}"
_trace "[cvd-argo-guest] startup mode=${CVD_ARGO_MODE} serial=${APP_SERIAL_DEV}"

trap _on_err ERR

# -----------------------------------------------------------------------------
# Load inputs and workspace
# -----------------------------------------------------------------------------
# job-env.sh exports workflow parameters; workloads.tgz is a trimmed repo checkout.

gcloud storage cp "${CVD_ARGO_INPUT_URI}/cvd-argo-job-env.sh" /tmp/cvd-argo-job-env.sh
gcloud storage cp "${CVD_ARGO_INPUT_URI}/cvd-argo-workloads.tgz" /tmp/cvd-argo-workloads.tgz

sudo rm -rf "${REMOTE_STAGING}"
sudo mkdir -p "${REMOTE_STAGING}"
sudo tar xzf /tmp/cvd-argo-workloads.tgz -C "${REMOTE_STAGING}"
sudo chown -R "$(id -un):$(id -gn)" "${REMOTE_STAGING}"

# shellcheck disable=SC1091
set -a && source /tmp/cvd-argo-job-env.sh && set +a
_cvd_argo_ensure_home

export WORKSPACE="${REMOTE_STAGING}"
export CVD_ARGO_REMOTE_ROOT="${REMOTE_STAGING}"
export REMOTE_ARTIFACT_TGZ="${REMOTE_ARTIFACT_TGZ}"
export CVD_ARGO_MODE="${CVD_ARGO_MODE}"

REMOTE_ENTRY="${REMOTE_STAGING}/workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_remote_entry.sh"
cd "${REMOTE_STAGING}"

# -----------------------------------------------------------------------------
# Remote entry and GCS outputs
# -----------------------------------------------------------------------------
# Always run teardown after main, even when main fails, so we stop Cuttlefish and gather logs.

_write_status running 0 "main phase"
export CVD_ARGO_REMOTE_PHASE=main
# Use "cmd || MAIN_RC=$?" — $? inside "if ! cmd; then" is 0 (if-test success), not cmd's exit code.
MAIN_RC=0
bash "${REMOTE_ENTRY}" || MAIN_RC=$?
if [[ "${MAIN_RC}" -ne 0 ]]; then
  # Keep phase=running until teardown + artifact upload finish so the workflow pod
  # does not delete the VM or skip Gemini staging while gather_artifacts runs.
  _write_status running 0 "teardown after main failure"
  export CVD_ARGO_REMOTE_PHASE=teardown
  TEARDOWN_RC=0
  bash "${REMOTE_ENTRY}" || TEARDOWN_RC=$?
  _upload_guest_outputs_to_gcs
  _write_status failed "${MAIN_RC}" "remote main failed (teardown=${TEARDOWN_RC})"
  exit "${MAIN_RC}"
fi
MAIN_RC=0

_write_status running 0 "teardown phase"
export CVD_ARGO_REMOTE_PHASE=teardown
TEARDOWN_RC=0
bash "${REMOTE_ENTRY}" || TEARDOWN_RC=$?

_upload_guest_outputs_to_gcs

EFFECTIVE=$((MAIN_RC != 0 ? MAIN_RC : TEARDOWN_RC))
if [[ "${EFFECTIVE}" -eq 0 ]]; then
  _write_status success 0 "complete"
else
  _write_status failed "${EFFECTIVE}" "main=${MAIN_RC} teardown=${TEARDOWN_RC}"
fi
exit "${EFFECTIVE}"
