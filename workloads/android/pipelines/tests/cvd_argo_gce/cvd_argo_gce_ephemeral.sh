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
#   Argo workflow pod driver for ephemeral Cuttlefish GCE (Path B).
#
#   This script runs inside the run-*-ephemeral-gce step. It does not compile Android
#   or run CTS itself — it provisions a VM, waits for the guest to finish, and collects
#   outputs. Guest work is cvd_argo_guest_startup.sh → cvd_argo_remote_entry.sh.
#
#   Steps (simplified):
#     1. Upload job env + script bundle to GCS (ephemeral-input/)
#     2. Apply KCC ComputeInstance; guest startup runs from instance metadata
#     3. Poll GCS status.json; stream logs (serial port 2 or Cloud Logging)
#     4. Download cvd-argo-artifacts.tgz to /tmp/cvd-argo-artifacts for later Gemini prep
#     5. Delete the ComputeInstance CR on exit (success or failure)
#
#   See cvd_argo_gce/README.md for the full flow and environment variables.

set -euo pipefail

# -----------------------------------------------------------------------------
# Required env and staging URIs
# -----------------------------------------------------------------------------
# Validates workflow env; derives ephemeral-input/ and ephemeral-output/ GCS prefixes.

: "${WORKSPACE:?WORKSPACE must be set (repo root, e.g. /workspace)}"
: "${CVD_ARGO_LOCAL_ARTIFACT_ROOT:=/tmp/cvd-argo-artifacts}"
mkdir -p "${CVD_ARGO_LOCAL_ARTIFACT_ROOT}"
mkdir -p "${CVD_ARGO_LOCAL_ARTIFACT_ROOT}/.meta"
printf 'false' >"${CVD_ARGO_LOCAL_ARTIFACT_ROOT}/.meta/mtk_connect_stage_failed"

: "${CLOUD_PROJECT:?}"
: "${CLOUD_ZONE:?}"
: "${CLOUD_REGION:?}"
: "${CVD_ARGO_INSTANCE_TEMPLATE:?}"
: "${CVD_ARGO_VM_NAME:?}"
: "${CVD_ARGO_MODE:?cvd or cts}"
: "${K8S_WORKFLOWS_NAMESPACE:?}"
: "${GEMINI_TEST_RESULTS_STAGING_URI:?}"
: "${WORKFLOW_UID:?}"

: "${CVD_ARGO_GUEST_POLL_SEC:=45}"
: "${CVD_ARGO_GUEST_POLL_FIRST_SEC:=30}"
: "${CVD_ARGO_GUEST_WALL_TIMEOUT_SEC:=0}"
: "${CVD_ARGO_KCC_WAIT_TIMEOUT:=20m}"
: "${CVD_ARGO_GCP_WAIT_TIMEOUT:=15m}"
# Set to 1 after the VM is visible in Compute API (serial/cloud log reads).
CVD_ARGO_GCP_VM_VISIBLE=0
# serial = Compute getSerialPortOutput; cloud = Cloud Logging app filter; both = debug.
: "${CVD_ARGO_LOG_SINK:=serial}"
: "${CVD_ARGO_CLOUD_LOG_PAGE_SIZE:=100}"
# GCE serial port for app stdout/stderr (2 = /dev/ttyS1; port 1 is kernel/systemd).
: "${CVD_ARGO_SERIAL_PORT:=2}"

CVD_ARGO_INPUT_URI="${GEMINI_TEST_RESULTS_STAGING_URI%/}/ephemeral-input"
CVD_ARGO_OUTPUT_URI="${GEMINI_TEST_RESULTS_STAGING_URI%/}/ephemeral-output"
CVD_ARGO_K8S_NAME="cvd-${WORKFLOW_UID}"
[[ "${CVD_ARGO_MODE}" == "cts" ]] && CVD_ARGO_K8S_NAME="cts-${WORKFLOW_UID}"

# -----------------------------------------------------------------------------
# Derived paths and driver state
# -----------------------------------------------------------------------------
# GCP REST script paths, cloud-log cursor file, and REMOTE_RC for the guest poll.

GUEST_STARTUP="${WORKSPACE}/workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_guest_startup.sh"
CVD_ARGO_GCP_COMMON="${WORKSPACE}/workloads/android/pipelines/common/gcp"
GCP_COMPUTE_REST_PY="${GCP_COMPUTE_REST_PY:-${CVD_ARGO_GCP_COMMON}/gcp_compute_rest.py}"
GCP_LOGGING_REST_PY="${GCP_LOGGING_REST_PY:-${CVD_ARGO_GCP_COMMON}/gcp_logging_rest.py}"
CVD_ARGO_CLOUD_LOG_AFTER_FILE="/tmp/cvd-argo-cloud-log-after-${WORKFLOW_UID}"
REMOTE_RC=0

# -----------------------------------------------------------------------------
# _cvd_argo_gcs_object_exists
# -----------------------------------------------------------------------------
# Return success when gcloud storage ls finds the object (e.g. status.json).

function _cvd_argo_gcs_object_exists() {
  gcloud storage ls "${1:?}" >/dev/null 2>&1
}

# -----------------------------------------------------------------------------
# write_job_env_for_vm
# -----------------------------------------------------------------------------
# Write a shell script of exported workflow env vars for the guest to source.

function write_job_env_for_vm() {
  local out="${1:?}"
  local k
  local -a keys=(
    CLOUD_PROJECT CLOUD_ZONE CLOUD_REGION HORIZON_DOMAIN
    BUILD_NUMBER JOB_NAME BUILD_USER BUILD_USER_ID
    CUTTLEFISH_DOWNLOAD_URL CUTTLEFISH_INSTALL_WIFI CUTTLEFISH_MAX_BOOT_TIME CUTTLEFISH_KEEP_ALIVE_TIME
    NUM_INSTANCES VM_CPUS VM_MEMORY_MB CVD_COMMAND_LINE
    MTK_CONNECT_USERNAME MTK_CONNECT_PASSWORD MTK_CONNECT_PUBLIC MTK_CONNECT_TUNNEL_PORT MTK_CONNECT_ENABLE
    ANDROID_VERSION CTS_DOWNLOAD_URL CTS_TESTPLAN CTS_MODULE CTS_RETRY_STRATEGY
    CTS_MAX_TESTCASE_RUN_COUNT CTS_TIMEOUT SHARD_COUNT
    ENABLE_GEMINI_AI_ASSISTANT GEMINI_ANALYSE_ON_SUCCESS GEMINI_COMMAND_LINE
    CTS_ARTIFACT_STORAGE_SOLUTION STORAGE_LABELS GEMINI_TEST_RESULTS_STAGING_URI
    GEMINI_PREVIEW_FEATURES GEMINI_LOCATION_GLOBAL
    CVD_ARGO_QUIET_GUEST_EXTRACT
    CVD_ARGO_APP_SERIAL_DEV
  )
  : >"${out}"
  for k in "${keys[@]}"; do
    if printenv "${k}" >/dev/null 2>&1; then
      printf 'export %s=%q\n' "$k" "$(printenv "${k}")" >>"${out}"
    fi
  done
}

# -----------------------------------------------------------------------------
# _cvd_argo_guest_tar_excludes
# -----------------------------------------------------------------------------
# Paths to omit from the guest workloads tarball (Helm, Jenkins, large docs).

function _cvd_argo_guest_tar_excludes() {
  printf '%s\n' \
    '*/helm/*' '*/groovy/*' '*/prompt/*' '*/hooks/*' '*/Jenkinsfile' '*/README.md' \
    '*.swp' '*~' \
    'workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_gce_ephemeral.sh' \
    'workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_sync_vm_artifacts_to_staging.sh'
}

# -----------------------------------------------------------------------------
# _cvd_argo_guest_bundle_roots
# -----------------------------------------------------------------------------
# Repo subtrees the guest needs (mtk-connect, cvd_launcher, cts_execution, scripts).

function _cvd_argo_guest_bundle_roots() {
  printf '%s\n' \
    workloads/common/mtk-connect \
    workloads/android/pipelines/tests/cvd_launcher \
    workloads/android/pipelines/tests/cts_execution \
    workloads/android/pipelines/tests/cvd_argo_gce
}

# -----------------------------------------------------------------------------
# cvd_argo_package_guest_scripts
# -----------------------------------------------------------------------------
# Build cvd-argo-workloads.tgz from bundle roots and tar excludes.

function cvd_argo_package_guest_scripts() {
  local out="${1:?}"
  local -a tar_excludes=() paths=() missing=0 x p
  while IFS= read -r x; do [[ -n "${x}" ]] && tar_excludes+=(--exclude="${x}"); done < <(_cvd_argo_guest_tar_excludes)
  while IFS= read -r p; do
    [[ -n "${p}" ]] || continue
    if [[ -e "${WORKSPACE}/${p}" ]]; then paths+=("${p}")
    else echo "[cvd-argo] ERROR: guest bundle root missing: ${WORKSPACE}/${p}" >&2; missing=1; fi
  done < <(_cvd_argo_guest_bundle_roots)
  [[ "${missing}" -eq 0 ]] || return 1
  local -a tar_noderef=()
  if tar --help 2>&1 | grep -q -- '--no-dereference'; then tar_noderef=(--no-dereference); fi
  tar czf "${out}" -C "${WORKSPACE}" "${tar_excludes[@]}" "${tar_noderef[@]}" "${paths[@]}"
}

# -----------------------------------------------------------------------------
# _cvd_argo_export_app_serial_dev
# -----------------------------------------------------------------------------
# Map CVD_ARGO_SERIAL_PORT (1–4) to /dev/ttyS0–ttyS3 for guest metadata.

function _cvd_argo_export_app_serial_dev() {
  if ! [[ "${CVD_ARGO_SERIAL_PORT}" =~ ^[1-4]$ ]]; then
    echo "[cvd-argo] ERROR: CVD_ARGO_SERIAL_PORT must be 1-4 (got ${CVD_ARGO_SERIAL_PORT})" >&2
    return 1
  fi
  export CVD_ARGO_APP_SERIAL_DEV="/dev/ttyS$((CVD_ARGO_SERIAL_PORT - 1))"
}

# -----------------------------------------------------------------------------
# cvd_argo_upload_inputs_to_gcs
# -----------------------------------------------------------------------------
# Upload job-env, workloads tarball, guest startup, and guest common to ephemeral-input/.

function cvd_argo_upload_inputs_to_gcs() {
  local job_env="/tmp/cvd-argo-job-env.sh" bundle="/tmp/cvd-argo-workloads.tgz"
  _cvd_argo_export_app_serial_dev
  write_job_env_for_vm "${job_env}"
  cvd_argo_package_guest_scripts "${bundle}"
  echo "[cvd-argo] uploading inputs to ${CVD_ARGO_INPUT_URI}/" >&2
  local guest_common="${WORKSPACE}/workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_guest_common.sh"
  gcloud storage cp "${job_env}" "${CVD_ARGO_INPUT_URI}/cvd-argo-job-env.sh"
  gcloud storage cp "${bundle}" "${CVD_ARGO_INPUT_URI}/cvd-argo-workloads.tgz"
  gcloud storage cp "${GUEST_STARTUP}" "${CVD_ARGO_INPUT_URI}/cvd_argo_guest_startup.sh"
  gcloud storage cp "${guest_common}" "${CVD_ARGO_INPUT_URI}/cvd_argo_guest_common.sh"
}

# -----------------------------------------------------------------------------
# cvd_argo_kcc_metadata_startup_wrapper
# -----------------------------------------------------------------------------
# Minimal metadata startup script: download cvd_argo_guest_startup.sh from GCS and exec.

function cvd_argo_kcc_metadata_startup_wrapper() {
  cat <<'BOOT'
#!/bin/bash
set -euo pipefail
INPUT=$(curl -fsS -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/cvd-argo-input-uri)
gcloud storage cp "${INPUT}/cvd_argo_guest_startup.sh" /tmp/cvd_argo_guest_startup.sh
chmod +x /tmp/cvd_argo_guest_startup.sh
exec bash /tmp/cvd_argo_guest_startup.sh
BOOT
}

# -----------------------------------------------------------------------------
# cvd_argo_apply_kcc_instance
# -----------------------------------------------------------------------------
# kubectl apply ComputeInstance CR with GCS URIs, serial logging, and startup script.

function cvd_argo_apply_kcc_instance() {
  local template_url="https://www.googleapis.com/compute/v1/projects/${CLOUD_PROJECT}/global/instanceTemplates/${CVD_ARGO_INSTANCE_TEMPLATE}"
  local startup
  startup="$(cvd_argo_kcc_metadata_startup_wrapper)"
  echo "[cvd-argo] applying KCC ComputeInstance ${CVD_ARGO_K8S_NAME} (GCP name ${CVD_ARGO_VM_NAME})" >&2
  if ! kubectl apply -f - <<EOF
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeInstance
metadata:
  name: ${CVD_ARGO_K8S_NAME}
  namespace: ${K8S_WORKFLOWS_NAMESPACE}
  labels:
    horizon-sdv.io/ephemeral-gce: "true"
    horizon-sdv.io/workflow-uid: "${WORKFLOW_UID}"
    horizon-sdv.io/mode: "${CVD_ARGO_MODE}"
  annotations:
    cnrm.cloud.google.com/project-id: "${CLOUD_PROJECT}"
spec:
  resourceID: "${CVD_ARGO_VM_NAME}"
  zone: ${CLOUD_ZONE}
  instanceTemplateRef:
    external: "${template_url}"
  metadata:
    - key: serial-port-logging-enable
      value: "true"
    - key: cvd-argo-input-uri
      value: "${CVD_ARGO_INPUT_URI}"
    - key: cvd-argo-output-uri
      value: "${CVD_ARGO_OUTPUT_URI}"
    - key: cvd-argo-mode
      value: "${CVD_ARGO_MODE}"
    - key: cvd-argo-vm-name
      value: "${CVD_ARGO_VM_NAME}"
    - key: cvd-argo-app-serial-dev
      value: "${CVD_ARGO_APP_SERIAL_DEV}"
  metadataStartupScript: |
$(printf '%s\n' "${startup}" | sed 's/^/    /')
  serviceAccount:
    scopes:
      - https://www.googleapis.com/auth/devstorage.read_write
      - https://www.googleapis.com/auth/logging.write
      - https://www.googleapis.com/auth/monitoring.write
EOF
  then
    echo "[cvd-argo] ERROR: kubectl apply ComputeInstance failed (check computeinstances RBAC on workflow-executor-elevated in namespace ${K8S_WORKFLOWS_NAMESPACE})" >&2
    return 1
  fi
}

# -----------------------------------------------------------------------------
# cvd_argo_wait_kcc_instance
# -----------------------------------------------------------------------------
# Block until Config Connector reports the VM Ready.

function cvd_argo_wait_kcc_instance() {
  echo "[cvd-argo] waiting for KCC ComputeInstance ${CVD_ARGO_K8S_NAME} (timeout ${CVD_ARGO_KCC_WAIT_TIMEOUT})" >&2
  kubectl wait --for=condition=Ready "computeinstance.compute.cnrm.cloud.google.com/${CVD_ARGO_K8S_NAME}" \
    -n "${K8S_WORKFLOWS_NAMESPACE}" --timeout="${CVD_ARGO_KCC_WAIT_TIMEOUT}"
}

# -----------------------------------------------------------------------------
# cvd_argo_apply_arm64_cloud_placement
# -----------------------------------------------------------------------------
# ARM64 Cuttlefish templates: ARM64_* zone/region from horizon-workflow-cloud-env,
# and Cloud Logging for live guest logs (Ubuntu arm64 serial port 2 does not carry app console).

function cvd_argo_apply_arm64_cloud_placement() {
  [[ "${CVD_ARGO_INSTANCE_TEMPLATE}" == *-arm64 ]] || return 0
  local z="${ARM64_ZONE:-}"
  local r="${ARM64_REGION:-}"
  if [[ -z "${z}" ]]; then
    echo "[cvd-argo] WARN: ARM64 instance template but ARM64_ZONE unset; using CLOUD_ZONE=${CLOUD_ZONE}" >&2
  else
    export CLOUD_ZONE="${z}"
    if [[ -n "${r}" ]]; then
      export CLOUD_REGION="${r}"
    fi
    echo "[cvd-argo] ARM64 ephemeral GCE placement: CLOUD_REGION=${CLOUD_REGION} CLOUD_ZONE=${CLOUD_ZONE}" >&2
  fi
  if [[ "${CVD_ARGO_LOG_SINK}" != "cloud" ]]; then
    echo "[cvd-argo] ARM64: overriding log_sink=${CVD_ARGO_LOG_SINK} → cloud (serial port 2 lacks app console on Ubuntu arm64)" >&2
    export CVD_ARGO_LOG_SINK=cloud
  fi
}

# -----------------------------------------------------------------------------
# _cvd_argo_export_compute_rest_token
# -----------------------------------------------------------------------------
# Metadata-server cloud-platform token for Compute REST (same identity as serial logs).

function _cvd_argo_export_compute_rest_token() {
  local token=""

  # shellcheck source=/dev/null
  source "${CVD_ARGO_GCP_COMMON}/gcp_metadata_access_token.sh"
  token="$(gcp_metadata_access_token)" || return 1
  export CF_COMPUTE_REST_TOKEN="${token}"
}

# -----------------------------------------------------------------------------
# _cvd_argo_gcp_instance_status
# -----------------------------------------------------------------------------
# Print GCE instance status (RUNNING, PROVISIONING, …) or empty when not found.

function _cvd_argo_gcp_instance_status() {
  _cvd_argo_export_compute_rest_token || return 1
  python3 "${GCP_COMPUTE_REST_PY}" get-instance-status \
    "${CLOUD_PROJECT}" "${CLOUD_ZONE}" "${CVD_ARGO_VM_NAME}" 2>/dev/null || true
}

# -----------------------------------------------------------------------------
# cvd_argo_log_kcc_instance_status
# -----------------------------------------------------------------------------
# On GCP wait failure, surface CNRM conditions from the ComputeInstance CR.

function cvd_argo_log_kcc_instance_status() {
  kubectl get "computeinstance.compute.cnrm.cloud.google.com/${CVD_ARGO_K8S_NAME}" \
    -n "${K8S_WORKFLOWS_NAMESPACE}" -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.message}{"\n"}{end}' 2>/dev/null \
    | sed '/^$/d' >&2 || true
}

# -----------------------------------------------------------------------------
# cvd_argo_wait_gcp_instance_visible
# -----------------------------------------------------------------------------
# KCC Ready can be set before the zonal instance exists in Compute API.

function cvd_argo_wait_gcp_instance_visible() {
  local poll_sec=15 start_sec="${SECONDS}"
  local timeout_raw="${CVD_ARGO_GCP_WAIT_TIMEOUT}"
  local timeout_sec=900
  if [[ "${timeout_raw}" =~ ^[0-9]+m$ ]]; then
    timeout_sec=$(( ${timeout_raw%m} * 60 ))
  elif [[ "${timeout_raw}" =~ ^[0-9]+s$ ]]; then
    timeout_sec=$(( ${timeout_raw%s} ))
  elif [[ "${timeout_raw}" =~ ^[0-9]+$ ]]; then
    timeout_sec="${timeout_raw}"
  fi
  local deadline=$((start_sec + timeout_sec))
  echo "[cvd-argo] waiting for GCP VM ${CVD_ARGO_VM_NAME} in ${CLOUD_ZONE} (timeout ${timeout_raw})" >&2
  while (( SECONDS < deadline )); do
    local st
    st="$(_cvd_argo_gcp_instance_status)"
    if [[ -n "${st}" ]]; then
      CVD_ARGO_GCP_VM_VISIBLE=1
      echo "[cvd-argo] GCP VM visible status=${st} zone=${CLOUD_ZONE}" >&2
      return 0
    fi
    sleep "${poll_sec}"
  done
  echo "[cvd-argo] ERROR: GCP VM ${CVD_ARGO_VM_NAME} not found in ${CLOUD_ZONE} after ${timeout_raw}" >&2
  cvd_argo_log_kcc_instance_status
  return 1
}

# -----------------------------------------------------------------------------
# cvd_argo_delete_kcc_instance
# -----------------------------------------------------------------------------
# Delete the ephemeral ComputeInstance CR (also called from EXIT trap).

# shellcheck disable=SC2329
function cvd_argo_delete_kcc_instance() {
  echo "[cvd-argo] deleting KCC ComputeInstance ${CVD_ARGO_K8S_NAME}" >&2
  kubectl delete "computeinstance.compute.cnrm.cloud.google.com/${CVD_ARGO_K8S_NAME}" \
    -n "${K8S_WORKFLOWS_NAMESPACE}" --ignore-not-found=true --wait=true --timeout=15m || true
}

# -----------------------------------------------------------------------------
# cvd_argo_compute_wall_timeout_sec
# -----------------------------------------------------------------------------
# Max seconds to wait for guest status.json (boot + keep-alive + CTS + MTK).

function cvd_argo_compute_wall_timeout_sec() {
  if [[ "${CVD_ARGO_GUEST_WALL_TIMEOUT_SEC}" =~ ^[0-9]+$ ]] && [[ "${CVD_ARGO_GUEST_WALL_TIMEOUT_SEC}" -gt 0 ]]; then
    echo "${CVD_ARGO_GUEST_WALL_TIMEOUT_SEC}"
    return 0
  fi
  local boot_max="${CUTTLEFISH_MAX_BOOT_TIME:-180}"
  boot_max="$(echo "${boot_max}" | tr -d '[:space:]')"
  [[ "${boot_max}" =~ ^[0-9]+$ ]] || boot_max=180
  local boot=$((boot_max * 4 + 1200))
  local keep=0 keep_mins="${CUTTLEFISH_KEEP_ALIVE_TIME:-0}"
  keep_mins="$(echo "${keep_mins}" | tr -d '[:space:]')"
  [[ -n "${keep_mins}" && "${keep_mins}" =~ ^[0-9]+$ && "${keep_mins}" -gt 0 ]] && keep=$((keep_mins * 60))
  local cts_sec=0
  if [[ "${CVD_ARGO_MODE}" == "cts" ]]; then
    local cts_min="${CTS_TIMEOUT:-600}"
    cts_min="$(echo "${cts_min}" | tr -d '[:space:]')"
    [[ "${cts_min}" =~ ^[0-9]+$ ]] || cts_min=600
    cts_sec=$((cts_min * 60))
  fi
  local mtk=0
  [[ "${MTK_CONNECT_ENABLE:-false}" == "true" ]] && mtk=900
  echo $((boot + keep + cts_sec + mtk))
}

# -----------------------------------------------------------------------------
# _cvd_argo_log_sink_uses_serial
# -----------------------------------------------------------------------------
# True when CVD_ARGO_LOG_SINK is serial or both.

function _cvd_argo_log_sink_uses_serial() {
  [[ "${CVD_ARGO_LOG_SINK}" == "serial" || "${CVD_ARGO_LOG_SINK}" == "both" ]]
}

# -----------------------------------------------------------------------------
# _cvd_argo_log_sink_uses_cloud
# -----------------------------------------------------------------------------
# True when CVD_ARGO_LOG_SINK is cloud or both.

function _cvd_argo_log_sink_uses_cloud() {
  [[ "${CVD_ARGO_LOG_SINK}" == "cloud" || "${CVD_ARGO_LOG_SINK}" == "both" ]]
}

# -----------------------------------------------------------------------------
# _cvd_argo_cloud_log_after_iso
# -----------------------------------------------------------------------------
# Timestamp cursor for incremental entries:list (stored in a temp file).

function _cvd_argo_cloud_log_after_iso() {
  if [[ -f "${CVD_ARGO_CLOUD_LOG_AFTER_FILE}" ]]; then
    cat "${CVD_ARGO_CLOUD_LOG_AFTER_FILE}"
    return 0
  fi
  printf ''
}

# -----------------------------------------------------------------------------
# _cvd_argo_emit_cloud_log_entries
# -----------------------------------------------------------------------------
# Fetch new guest app log lines from Cloud Logging and print to pod stderr.

function _cvd_argo_emit_cloud_log_entries() {
  local flush_all="${1:-0}"
  local token after_iso json last_ts="" extra=()

  _cvd_argo_log_sink_uses_cloud || return 0

  # shellcheck source=/dev/null
  source "${CVD_ARGO_GCP_COMMON}/gcp_metadata_access_token.sh"
  token="$(gcp_metadata_access_token)" || return 1
  export CF_COMPUTE_REST_TOKEN="${token}"
  after_iso="$(_cvd_argo_cloud_log_after_iso)"
  [[ "${flush_all}" == "1" ]] && extra+=(--all-pages)

  local log_err log_err_lines=0
  log_err="$(mktemp)"
  json="$(python3 "${GCP_LOGGING_REST_PY}" list-guest-app-logs \
    "${CLOUD_PROJECT}" "${CLOUD_ZONE}" "${CVD_ARGO_VM_NAME}" \
    --after="${after_iso}" --page-size="${CVD_ARGO_CLOUD_LOG_PAGE_SIZE}" \
    "${extra[@]}" 2>"${log_err}" || true)"
  if [[ -s "${log_err}" ]]; then
    log_err_lines=$(wc -l <"${log_err}" | tr -d '[:space:]')
    sed 's/^/[cvd-argo] Cloud Logging API: /' "${log_err}" >&2
  fi
  rm -f "${log_err}"
  if [[ -z "${json}" ]]; then
    [[ "${log_err_lines}" -eq 0 ]] && echo "[cvd-argo] Cloud Logging: no matching entries yet (vm=${CVD_ARGO_VM_NAME} zone=${CLOUD_ZONE}; Ops Agent + logging.logWriter on VM SA?)" >&2
    return 0
  fi

  # decode prints [guest] lines and a final CVD_ARGO_CLOUD_LOG_CURSOR= line on stdout.
  last_ts=""
  while IFS= read -r line || [[ -n "${line}" ]]; do
    case "${line}" in
      CVD_ARGO_CLOUD_LOG_CURSOR=*)
        last_ts="${line#CVD_ARGO_CLOUD_LOG_CURSOR=}"
        ;;
      *)
        echo "${line}" >&2
        ;;
    esac
  done < <(printf '%s\n' "${json}" | python3 "${GCP_LOGGING_REST_PY}" decode-guest-log-lines || true)
  if [[ -n "${last_ts}" ]]; then
    printf '%s' "${last_ts}" >"${CVD_ARGO_CLOUD_LOG_AFTER_FILE}"
  fi
  return 0
}

# -----------------------------------------------------------------------------
# _cvd_argo_flush_cloud_logs_to_stderr
# -----------------------------------------------------------------------------
# Drain all remaining Cloud Logging pages at end of poll.

function _cvd_argo_flush_cloud_logs_to_stderr() {
  _cvd_argo_emit_cloud_log_entries 1 || true
}

# -----------------------------------------------------------------------------
# _cvd_argo_get_serial_port_json
# -----------------------------------------------------------------------------
# One getSerialPortOutput REST call for CVD_ARGO_SERIAL_PORT.

function _cvd_argo_get_serial_port_json() {
  local start_byte="${1:?}"

  if [[ "${CVD_ARGO_GCP_VM_VISIBLE}" != "1" ]]; then
    return 1
  fi

  _cvd_argo_export_compute_rest_token || return 1
  python3 "${GCP_COMPUTE_REST_PY}" get-serial-port-output \
    "${CLOUD_PROJECT}" "${CLOUD_ZONE}" "${CVD_ARGO_VM_NAME}" \
    --port="${CVD_ARGO_SERIAL_PORT}" --start="${start_byte}"
}

# -----------------------------------------------------------------------------
# _cvd_argo_fetch_guest_serial_chunk
# -----------------------------------------------------------------------------
# Fetch one serial chunk; update start-byte nameref for paging.

function _cvd_argo_fetch_guest_serial_chunk() {
  local -n _start_byte_ref="${1:?}"
  local -n _contents_out="${2:?}"
  local json old_start

  old_start="${_start_byte_ref}"
  json="$(_cvd_argo_get_serial_port_json "${old_start}")" || return 1
  _contents_out="$(printf '%s' "${json}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("contents",""), end="")')"
  _start_byte_ref="$(printf '%s' "${json}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("next",""))')"
  [[ -n "${_start_byte_ref}" ]] || _start_byte_ref="${old_start}"
  return 0
}

# -----------------------------------------------------------------------------
# _cvd_argo_emit_guest_serial_log_chunk
# -----------------------------------------------------------------------------
# Print one serial chunk to pod stderr with [guest] prefix.

function _cvd_argo_emit_guest_serial_log_chunk() {
  local -n _start_byte_ref="${1}"
  local chunk_raw old_start nbytes

  old_start="${_start_byte_ref}"
  _cvd_argo_fetch_guest_serial_chunk "${1}" chunk_raw || return 1
  [[ -n "${chunk_raw}" ]] || return 1

  nbytes=$(printf '%s' "${chunk_raw}" | wc -c | tr -d '[:space:]')
  echo "[cvd-argo] --- serial port ${CVD_ARGO_SERIAL_PORT} bytes ${old_start}-$((_start_byte_ref - 1)) (${nbytes} new, next=${_start_byte_ref}) ---" >&2
  while IFS= read -r line || [[ -n "${line}" ]]; do
    printf '[guest] %s\n' "${line}" >&2
  done < <(printf '%s\n' "${chunk_raw}")
  printf '[cvd-argo] --- end serial port chunk ---\n' >&2
  return 0
}

# -----------------------------------------------------------------------------
# _cvd_argo_flush_guest_serial_log_to_stderr
# -----------------------------------------------------------------------------
# Drain serial port output until no new bytes.

function _cvd_argo_flush_guest_serial_log_to_stderr() {
  local -n _start_byte_ref="${1:?}"
  while _cvd_argo_emit_guest_serial_log_chunk "${1}"; do
    :
  done
}

# -----------------------------------------------------------------------------
# cvd_argo_poll_guest_status
# -----------------------------------------------------------------------------
# Poll status.json on GCS; stream serial/cloud logs until success, failed, or timeout.

function cvd_argo_poll_guest_status() {
  local timeout_sec="${1:?}"
  local start_sec="${SECONDS}"
  local deadline=$((start_sec + timeout_sec))
  local first="${CVD_ARGO_GUEST_POLL_FIRST_SEC}" poll="${CVD_ARGO_GUEST_POLL_SEC}"
  # shellcheck disable=SC2034 # updated via nameref in _cvd_argo_emit_guest_serial_log_chunk
  local guest_serial_start=0
  local status_uri="${CVD_ARGO_OUTPUT_URI}/status.json"
  local poll_log_ctx="log_sink=${CVD_ARGO_LOG_SINK}"
  _cvd_argo_log_sink_uses_serial && poll_log_ctx+=", serial_port=${CVD_ARGO_SERIAL_PORT}"
  echo "[cvd-argo] polling guest status at ${status_uri} (first ${first}s, every ${poll}s, wall ${timeout_sec}s; ${poll_log_ctx})" >&2
  _cvd_argo_log_sink_uses_cloud && rm -f "${CVD_ARGO_CLOUD_LOG_AFTER_FILE}"
  sleep "${first}"
  while (( SECONDS < deadline )); do
    if _cvd_argo_gcs_object_exists "${status_uri}"; then
      local body phase rc parsed
      body="$(gcloud storage cat "${status_uri}" 2>/dev/null || true)"
      echo "[cvd-argo] guest status: ${body}" >&2
      parsed="$(printf '%s' "${body}" | python3 -c 'import json,sys
try:
  d=json.load(sys.stdin)
  print(d.get("phase",""), d.get("rc",1))
except Exception:
  print("", "")' 2>/dev/null || true)"
      phase="${parsed%% *}"
      rc="${parsed#* }"
      case "${phase}" in
        success)
          _cvd_argo_log_sink_uses_serial && _cvd_argo_flush_guest_serial_log_to_stderr guest_serial_start || true
          _cvd_argo_log_sink_uses_cloud && _cvd_argo_flush_cloud_logs_to_stderr || true
          REMOTE_RC="${rc:-0}"
          return 0
          ;;
        failed)
          _cvd_argo_log_sink_uses_serial && _cvd_argo_flush_guest_serial_log_to_stderr guest_serial_start || true
          _cvd_argo_log_sink_uses_cloud && _cvd_argo_flush_cloud_logs_to_stderr || true
          REMOTE_RC="${rc:-1}"
          [[ "${REMOTE_RC}" -eq 0 ]] && REMOTE_RC=1
          return 0
          ;;
        running) ;;
        *)
          ;;
      esac
    else
      echo "[cvd-argo] guest status not yet at ${status_uri} (elapsed $((SECONDS - start_sec))s)" >&2
    fi
    _cvd_argo_log_sink_uses_serial && _cvd_argo_emit_guest_serial_log_chunk guest_serial_start || true
    _cvd_argo_log_sink_uses_cloud && _cvd_argo_emit_cloud_log_entries || true
    sleep "${poll}"
  done
  _cvd_argo_log_sink_uses_serial && _cvd_argo_flush_guest_serial_log_to_stderr guest_serial_start || true
  _cvd_argo_log_sink_uses_cloud && _cvd_argo_flush_cloud_logs_to_stderr || true
  echo "[cvd-argo] ERROR: guest wall timeout (${timeout_sec}s)" >&2
  REMOTE_RC=124
  return 0
}

# -----------------------------------------------------------------------------
# cvd_argo_download_guest_outputs
# -----------------------------------------------------------------------------
# Download cvd-argo-artifacts.tgz and marker from GCS into CVD_ARGO_LOCAL_ARTIFACT_ROOT.

function cvd_argo_download_guest_outputs() {
  local tgz="${CVD_ARGO_OUTPUT_URI}/cvd-argo-artifacts.tgz"
  if _cvd_argo_gcs_object_exists "${tgz}"; then
    echo "[cvd-argo] downloading ${tgz}" >&2
    gcloud storage cp "${tgz}" /tmp/cvd-argo-artifacts.tgz
    rm -rf "${CVD_ARGO_LOCAL_ARTIFACT_ROOT}/cvd-argo-out"
    mkdir -p "${CVD_ARGO_LOCAL_ARTIFACT_ROOT}"
    tar xzf /tmp/cvd-argo-artifacts.tgz -C "${CVD_ARGO_LOCAL_ARTIFACT_ROOT}"
  fi
  if _cvd_argo_gcs_object_exists "${CVD_ARGO_OUTPUT_URI}/cvd-argo.marker"; then
    gcloud storage cp "${CVD_ARGO_OUTPUT_URI}/cvd-argo.marker" /tmp/cvd-argo.marker || true
  fi
}

# -----------------------------------------------------------------------------
# write_mtk_connect_stage_failed_meta
# -----------------------------------------------------------------------------
# Write .meta/mtk_connect_stage_failed for gemini_argo_prepare_staging.sh.

function write_mtk_connect_stage_failed_meta() {
  local out="${CVD_ARGO_LOCAL_ARTIFACT_ROOT}/.meta/mtk_connect_stage_failed"
  mkdir -p "${CVD_ARGO_LOCAL_ARTIFACT_ROOT}/.meta"
  local val=false mk="${CVD_ARGO_LOCAL_ARTIFACT_ROOT}/cvd-argo-out/cvd-argo.marker"
  if [[ -f "${mk}" ]]; then
    # shellcheck disable=SC1090,SC1091
    source "${mk}" 2>/dev/null || true
    [[ "${MTK_CONNECT_STAGE_FAILED:-}" == "true" ]] && val=true
  fi
  printf '%s' "${val}" >"${out}"
}

# -----------------------------------------------------------------------------
# mtk_pod_offline_delete_if_needed
# -----------------------------------------------------------------------------
# Delete offline MTK testbench from the workflow pod when guest remote stage failed.

# shellcheck disable=SC2329
function mtk_pod_offline_delete_if_needed() {
  [[ -f /tmp/cvd-argo.marker ]] || return 0
  # shellcheck disable=SC1091
  source /tmp/cvd-argo.marker
  if [[ "${MTK_RAN:-false}" != "true" ]] || [[ "${REMOTE_RC}" == "0" ]]; then
    return 0
  fi
  if ! printenv MTK_CONNECT_USERNAME >/dev/null 2>&1 || ! printenv MTK_CONNECT_PASSWORD >/dev/null 2>&1; then
    return 0
  fi
  echo "[cvd-argo] MTK offline testbench cleanup (remote stage failed)" >&2
  pushd "${WORKSPACE}/workloads/common/mtk-connect" >/dev/null || return 0
  sudo \
    MTK_CONNECT_TUNNEL_PORT="${MTK_CONNECT_TUNNEL_PORT:-8555}" \
    MTK_CONNECT_DOMAIN="${HORIZON_DOMAIN:?}" \
    MTK_CONNECT_USERNAME="${MTK_CONNECT_USERNAME}" \
    MTK_CONNECT_PASSWORD="${MTK_CONNECT_PASSWORD}" \
    MTK_CONNECT_TESTBENCH="${JOB_NAME}-${BUILD_NUMBER}" \
    MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES=true \
    MTK_CONNECT_CONTAINER_ONLY=true \
    timeout 15m bash ./mtk_connect.sh --delete || true
  popd >/dev/null || true
}

# -----------------------------------------------------------------------------
# cleanup_vm
# -----------------------------------------------------------------------------
# EXIT trap: MTK offline cleanup then delete the ephemeral ComputeInstance CR.

# shellcheck disable=SC2329
function cleanup_vm() {
  mtk_pod_offline_delete_if_needed || true
  cvd_argo_delete_kcc_instance
}

trap cleanup_vm EXIT

# -----------------------------------------------------------------------------
# Prerequisites and pipeline driver (main)
# -----------------------------------------------------------------------------
# Upload inputs, apply KCC VM, poll guest status, download artifacts, optional GCS staging.

export CLOUDSDK_CORE_PROJECT="${CLOUD_PROJECT}"
export GOOGLE_CLOUD_PROJECT="${CLOUD_PROJECT}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "[cvd-argo] ERROR: kubectl required for KCC ephemeral VMs" >&2
  exit 125
fi
if ! command -v gcloud >/dev/null 2>&1; then
  echo "[cvd-argo] ERROR: gcloud required for Path B GCS I/O (gcloud storage)" >&2
  exit 125
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "[cvd-argo] ERROR: python3 required for GCP REST log helpers" >&2
  exit 125
fi
if [[ ! -f "${GCP_COMPUTE_REST_PY}" ]]; then
  echo "[cvd-argo] ERROR: missing ${GCP_COMPUTE_REST_PY}" >&2
  exit 125
fi

cvd_argo_apply_arm64_cloud_placement

if _cvd_argo_log_sink_uses_cloud && [[ ! -f "${GCP_LOGGING_REST_PY}" ]]; then
  echo "[cvd-argo] ERROR: CVD_ARGO_LOG_SINK=cloud requires ${GCP_LOGGING_REST_PY}" >&2
  exit 125
fi

echo "[cvd-argo] Path B: KCC ComputeInstance + GCS (no IAP ssh/scp)" >&2
echo "[cvd-argo] template=${CVD_ARGO_INSTANCE_TEMPLATE} vm=${CVD_ARGO_VM_NAME} mode=${CVD_ARGO_MODE} log_sink=${CVD_ARGO_LOG_SINK} serial_port=${CVD_ARGO_SERIAL_PORT}" >&2

cvd_argo_upload_inputs_to_gcs
echo "[cvd-argo] guest app serial device=${CVD_ARGO_APP_SERIAL_DEV}" >&2
cvd_argo_apply_kcc_instance
cvd_argo_wait_kcc_instance
cvd_argo_wait_gcp_instance_visible

_wall="$(cvd_argo_compute_wall_timeout_sec)"
cvd_argo_poll_guest_status "${_wall}"
cvd_argo_download_guest_outputs
write_mtk_connect_stage_failed_meta

if [[ "${REMOTE_RC}" -ne 0 ]]; then
  echo "[cvd-argo] guest run failed effective=${REMOTE_RC}" >&2
  exit "${REMOTE_RC}"
fi

# -----------------------------------------------------------------------------
# VM artifact staging (Gemini off)
# -----------------------------------------------------------------------------
# When Gemini is on, sync-vm-staging or gemini_storage.sh uploads (not this script).

if [[ "${ENABLE_GEMINI_AI_ASSISTANT:-false}" == "true" ]]; then
  echo "[cvd-argo] skipping VM GCS staging (Gemini enabled: sync-vm-staging or gemini_storage uploads)" >&2
else
  export GEMINI_TEST_RESULTS_STAGING_URI
  bash "${WORKSPACE}/workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_sync_vm_artifacts_to_staging.sh" || true
fi

echo "[cvd-argo] ephemeral GCE complete (REMOTE_RC=${REMOTE_RC})" >&2
exit 0
