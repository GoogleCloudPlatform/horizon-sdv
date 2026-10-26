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
#   Argo workflow pod driver for the ephemeral remotive-topology (RemotiveTopology)
#   GCE VM — the remotive analogue of cvd_argo_gce/cvd_argo_gce_ephemeral.sh 
#
#   Steps:
#     1. Upload job env + guest script bundle to GCS (ephemeral-input/)
#     2. Apply KCC ComputeInstance; guest startup runs from instance metadata
#     3. Poll GCS status.json; stream guest logs from serial port 2
#     4. Download remotive-argo-artifacts.tgz to REMOTIVE_ARGO_LOCAL_ARTIFACT_ROOT
#     5. Delete the ComputeInstance CR on exit (success or failure)
#
#   While the topology is up, the guest writes access.json (VM name + forward ports);
#   this driver prints a ready-to-run gcloud IAP SSH port-forward command.

set -euo pipefail

# -----------------------------------------------------------------------------
# Required env and staging URIs
# -----------------------------------------------------------------------------
: "${WORKSPACE:?WORKSPACE must be set (repo root, e.g. /workspace)}"
: "${REMOTIVE_ARGO_LOCAL_ARTIFACT_ROOT:=/tmp/remotive-argo-artifacts}"
mkdir -p "${REMOTIVE_ARGO_LOCAL_ARTIFACT_ROOT}"

: "${CLOUD_PROJECT:?}"
: "${CLOUD_ZONE:?}"
: "${CLOUD_REGION:?}"
: "${REMOTIVE_ARGO_INSTANCE_TEMPLATE:?}"
: "${REMOTIVE_ARGO_VM_NAME:?}"
: "${K8S_WORKFLOWS_NAMESPACE:?}"
: "${REMOTIVE_TEST_RESULTS_STAGING_URI:?}"
: "${WORKFLOW_UID:?}"

: "${REMOTIVE_ARGO_GUEST_POLL_SEC:=30}"
: "${REMOTIVE_ARGO_GUEST_POLL_FIRST_SEC:=30}"
: "${REMOTIVE_ARGO_GUEST_WALL_TIMEOUT_SEC:=0}"
: "${REMOTIVE_ARGO_KCC_WAIT_TIMEOUT:=20m}"
: "${REMOTIVE_ARGO_GCP_WAIT_TIMEOUT:=15m}"
# GCE serial port for app stdout/stderr (2 = /dev/ttyS1; port 1 is kernel/systemd).
: "${REMOTIVE_ARGO_SERIAL_PORT:=2}"
REMOTIVE_ARGO_GCP_VM_VISIBLE=0

REMOTIVE_ARGO_INPUT_URI="${REMOTIVE_TEST_RESULTS_STAGING_URI%/}/ephemeral-input"
REMOTIVE_ARGO_OUTPUT_URI="${REMOTIVE_TEST_RESULTS_STAGING_URI%/}/ephemeral-output"
REMOTIVE_ARGO_K8S_NAME="remotive-${WORKFLOW_UID}"

GUEST_STARTUP="${WORKSPACE}/workloads/remotive/pipelines/tests/remotive_launcher/remotive_argo_gce/remotive_argo_guest_startup.sh"
GUEST_COMMON="${WORKSPACE}/workloads/remotive/pipelines/tests/remotive_launcher/remotive_argo_gce/remotive_argo_guest_common.sh"
GCP_COMMON="${WORKSPACE}/workloads/android/pipelines/common/gcp"
GCP_COMPUTE_REST_PY="${GCP_COMMON}/gcp_compute_rest.py"

# -----------------------------------------------------------------------------
# GCS helpers
# -----------------------------------------------------------------------------
function _remotive_argo_gcs_object_exists() {
  gcloud storage ls "${1:?}" >/dev/null 2>&1
}

# -----------------------------------------------------------------------------
# write_job_env_for_vm
# -----------------------------------------------------------------------------
# Shell script of exported workflow env vars for the guest to source. Includes the
# RemotiveCloud token (like MTK credentials in cvd) — the staging bucket is
# project-internal; use a revocable least-privilege service-account token.

function write_job_env_for_vm() {
  local out="${1:?}"
  local k
  local -a keys=(
    CLOUD_PROJECT CLOUD_ZONE CLOUD_REGION HORIZON_DOMAIN
    BUILD_NUMBER JOB_NAME BUILD_USER BUILD_USER_ID
    TOPOLOGY_NAME TOPOLOGY_DOWNLOAD_URL REMOTIVE_LAUNCHER_FILE RUN_TOPOLOGY_TESTS REMOTIVE_KEEP_ALIVE_TIME
    REMOTIVE_CLOUD_AUTH_TOKEN REMOTIVE_CLOUD_ORGANIZATION
    MTK_CONNECT_USERNAME MTK_CONNECT_PASSWORD MTK_CONNECT_PUBLIC MTK_CONNECT_TUNNEL_PORT
    STORAGE_LABELS REMOTIVE_TEST_RESULTS_STAGING_URI
    REMOTIVE_ARGO_APP_SERIAL_DEV
  )
  : >"${out}"
  for k in "${keys[@]}"; do
    if printenv "${k}" >/dev/null 2>&1; then
      printf 'export %s=%q\n' "$k" "$(printenv "${k}")" >>"${out}"
    fi
  done
}

# -----------------------------------------------------------------------------
# remotive_argo_package_guest_scripts
# -----------------------------------------------------------------------------
# Bundle the guest-needed repo subtrees: remotive_launcher scripts and mtk-connect.
# Helm content is excluded. The topology project itself is not part of the bundle:
# the guest downloads it from TOPOLOGY_DOWNLOAD_URL (the repo ships no topologies).

function remotive_argo_package_guest_scripts() {
  local out="${1:?}"
  local -a paths=(
    workloads/common/mtk-connect
    workloads/remotive/pipelines/tests/remotive_launcher
  )
  local p
  for p in "${paths[@]}"; do
    if [[ ! -e "${WORKSPACE}/${p}" ]]; then
      echo "[remotive-argo] ERROR: guest bundle root missing: ${WORKSPACE}/${p}" >&2
      return 1
    fi
  done
  local -a tar_noderef=()
  if tar --help 2>&1 | grep -q -- '--no-dereference'; then tar_noderef=(--no-dereference); fi
  tar czf "${out}" -C "${WORKSPACE}" \
    --exclude='*/helm/*' --exclude='*.swp' --exclude='*~' \
    "${tar_noderef[@]}" "${paths[@]}"
}

function remotive_argo_upload_inputs_to_gcs() {
  local job_env="/tmp/remotive-argo-job-env.sh" bundle="/tmp/remotive-argo-workloads.tgz"
  if ! [[ "${REMOTIVE_ARGO_SERIAL_PORT}" =~ ^[1-4]$ ]]; then
    echo "[remotive-argo] ERROR: REMOTIVE_ARGO_SERIAL_PORT must be 1-4 (got ${REMOTIVE_ARGO_SERIAL_PORT})" >&2
    return 1
  fi
  export REMOTIVE_ARGO_APP_SERIAL_DEV="/dev/ttyS$((REMOTIVE_ARGO_SERIAL_PORT - 1))"
  write_job_env_for_vm "${job_env}"
  remotive_argo_package_guest_scripts "${bundle}"
  echo "[remotive-argo] uploading inputs to ${REMOTIVE_ARGO_INPUT_URI}/" >&2
  gcloud storage cp "${job_env}" "${REMOTIVE_ARGO_INPUT_URI}/remotive-argo-job-env.sh"
  gcloud storage cp "${bundle}" "${REMOTIVE_ARGO_INPUT_URI}/remotive-argo-workloads.tgz"
  gcloud storage cp "${GUEST_STARTUP}" "${REMOTIVE_ARGO_INPUT_URI}/remotive_argo_guest_startup.sh"
  gcloud storage cp "${GUEST_COMMON}" "${REMOTIVE_ARGO_INPUT_URI}/remotive_argo_guest_common.sh"
}

# -----------------------------------------------------------------------------
# KCC ComputeInstance
# -----------------------------------------------------------------------------
function remotive_argo_kcc_metadata_startup_wrapper() {
  cat <<'BOOT'
#!/bin/bash
set -euo pipefail
INPUT=$(curl -fsS -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/remotive-argo-input-uri)
gcloud storage cp "${INPUT}/remotive_argo_guest_startup.sh" /tmp/remotive_argo_guest_startup.sh
chmod +x /tmp/remotive_argo_guest_startup.sh
exec bash /tmp/remotive_argo_guest_startup.sh
BOOT
}

function remotive_argo_apply_kcc_instance() {
  local template_url="https://www.googleapis.com/compute/v1/projects/${CLOUD_PROJECT}/global/instanceTemplates/${REMOTIVE_ARGO_INSTANCE_TEMPLATE}"
  local startup
  startup="$(remotive_argo_kcc_metadata_startup_wrapper)"
  echo "[remotive-argo] applying KCC ComputeInstance ${REMOTIVE_ARGO_K8S_NAME} (GCP name ${REMOTIVE_ARGO_VM_NAME})" >&2
  if ! kubectl apply -f - <<EOF
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeInstance
metadata:
  name: ${REMOTIVE_ARGO_K8S_NAME}
  namespace: ${K8S_WORKFLOWS_NAMESPACE}
  labels:
    horizon-sdv.io/ephemeral-gce: "true"
    horizon-sdv.io/workflow-uid: "${WORKFLOW_UID}"
    horizon-sdv.io/mode: "remotive"
  annotations:
    cnrm.cloud.google.com/project-id: "${CLOUD_PROJECT}"
spec:
  resourceID: "${REMOTIVE_ARGO_VM_NAME}"
  zone: ${CLOUD_ZONE}
  instanceTemplateRef:
    external: "${template_url}"
  metadata:
    - key: serial-port-logging-enable
      value: "true"
    - key: remotive-argo-input-uri
      value: "${REMOTIVE_ARGO_INPUT_URI}"
    - key: remotive-argo-output-uri
      value: "${REMOTIVE_ARGO_OUTPUT_URI}"
    - key: remotive-argo-vm-name
      value: "${REMOTIVE_ARGO_VM_NAME}"
    - key: remotive-argo-app-serial-dev
      value: "${REMOTIVE_ARGO_APP_SERIAL_DEV}"
  metadataStartupScript: |
$(printf '%s\n' "${startup}" | sed 's/^/    /')
  serviceAccount:
    scopes:
      - https://www.googleapis.com/auth/devstorage.read_write
      - https://www.googleapis.com/auth/logging.write
      - https://www.googleapis.com/auth/monitoring.write
EOF
  then
    echo "[remotive-argo] ERROR: kubectl apply ComputeInstance failed (check computeinstances RBAC on workflow-executor-elevated in namespace ${K8S_WORKFLOWS_NAMESPACE})" >&2
    return 1
  fi
}

function remotive_argo_wait_kcc_instance() {
  echo "[remotive-argo] waiting for KCC ComputeInstance ${REMOTIVE_ARGO_K8S_NAME} (timeout ${REMOTIVE_ARGO_KCC_WAIT_TIMEOUT})" >&2
  kubectl wait --for=condition=Ready "computeinstance.compute.cnrm.cloud.google.com/${REMOTIVE_ARGO_K8S_NAME}" \
    -n "${K8S_WORKFLOWS_NAMESPACE}" --timeout="${REMOTIVE_ARGO_KCC_WAIT_TIMEOUT}"
}

function remotive_argo_log_kcc_instance_status() {
  kubectl get "computeinstance.compute.cnrm.cloud.google.com/${REMOTIVE_ARGO_K8S_NAME}" \
    -n "${K8S_WORKFLOWS_NAMESPACE}" -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.message}{"\n"}{end}' 2>/dev/null \
    | sed '/^$/d' >&2 || true
}

# -----------------------------------------------------------------------------
# Compute REST (token, instance status, serial log)
# -----------------------------------------------------------------------------
function _remotive_argo_export_compute_rest_token() {
  local token=""
  # shellcheck source=/dev/null
  source "${GCP_COMMON}/gcp_metadata_access_token.sh"
  token="$(gcp_metadata_access_token)" || return 1
  export CF_COMPUTE_REST_TOKEN="${token}"
}

function _remotive_argo_gcp_instance_status() {
  _remotive_argo_export_compute_rest_token || return 1
  python3 "${GCP_COMPUTE_REST_PY}" get-instance-status \
    "${CLOUD_PROJECT}" "${CLOUD_ZONE}" "${REMOTIVE_ARGO_VM_NAME}" 2>/dev/null || true
}

function remotive_argo_wait_gcp_instance_visible() {
  local poll_sec=15 start_sec="${SECONDS}"
  local timeout_raw="${REMOTIVE_ARGO_GCP_WAIT_TIMEOUT}"
  local timeout_sec=900
  if [[ "${timeout_raw}" =~ ^[0-9]+m$ ]]; then
    timeout_sec=$(( ${timeout_raw%m} * 60 ))
  elif [[ "${timeout_raw}" =~ ^[0-9]+s$ ]]; then
    timeout_sec=$(( ${timeout_raw%s} ))
  elif [[ "${timeout_raw}" =~ ^[0-9]+$ ]]; then
    timeout_sec="${timeout_raw}"
  fi
  local deadline=$((start_sec + timeout_sec))
  echo "[remotive-argo] waiting for GCP VM ${REMOTIVE_ARGO_VM_NAME} in ${CLOUD_ZONE} (timeout ${timeout_raw})" >&2
  while (( SECONDS < deadline )); do
    local st
    st="$(_remotive_argo_gcp_instance_status)"
    if [[ -n "${st}" ]]; then
      REMOTIVE_ARGO_GCP_VM_VISIBLE=1
      echo "[remotive-argo] GCP VM visible status=${st} zone=${CLOUD_ZONE}" >&2
      return 0
    fi
    sleep "${poll_sec}"
  done
  echo "[remotive-argo] ERROR: GCP VM ${REMOTIVE_ARGO_VM_NAME} not found in ${CLOUD_ZONE} after ${timeout_raw}" >&2
  remotive_argo_log_kcc_instance_status
  return 1
}

# shellcheck disable=SC2329
function remotive_argo_delete_kcc_instance() {
  echo "[remotive-argo] deleting KCC ComputeInstance ${REMOTIVE_ARGO_K8S_NAME}" >&2
  kubectl delete "computeinstance.compute.cnrm.cloud.google.com/${REMOTIVE_ARGO_K8S_NAME}" \
    -n "${K8S_WORKFLOWS_NAMESPACE}" --ignore-not-found=true --wait=true --timeout=15m || true
}
trap remotive_argo_delete_kcc_instance EXIT

# -----------------------------------------------------------------------------
# Serial log streaming
# -----------------------------------------------------------------------------
function _remotive_argo_get_serial_port_json() {
  local start_byte="${1:?}"
  if [[ "${REMOTIVE_ARGO_GCP_VM_VISIBLE}" != "1" ]]; then
    return 1
  fi
  _remotive_argo_export_compute_rest_token || return 1
  python3 "${GCP_COMPUTE_REST_PY}" get-serial-port-output \
    "${CLOUD_PROJECT}" "${CLOUD_ZONE}" "${REMOTIVE_ARGO_VM_NAME}" \
    --port="${REMOTIVE_ARGO_SERIAL_PORT}" --start="${start_byte}"
}

function _remotive_argo_emit_guest_serial_log_chunk() {
  local -n _start_byte_ref="${1}"
  local json chunk_raw old_start
  old_start="${_start_byte_ref}"
  json="$(_remotive_argo_get_serial_port_json "${old_start}")" || return 1
  chunk_raw="$(printf '%s' "${json}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("contents",""), end="")')"
  _start_byte_ref="$(printf '%s' "${json}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("next",""))')"
  [[ -n "${_start_byte_ref}" ]] || _start_byte_ref="${old_start}"
  [[ -n "${chunk_raw}" ]] || return 1
  while IFS= read -r line || [[ -n "${line}" ]]; do
    printf '[guest] %s\n' "${line}" >&2
  done < <(printf '%s\n' "${chunk_raw}")
  return 0
}

function _remotive_argo_flush_guest_serial_log_to_stderr() {
  while _remotive_argo_emit_guest_serial_log_chunk "${1:?}"; do
    :
  done
}

# -----------------------------------------------------------------------------
# Access instructions (IAP SSH port-forward)
# -----------------------------------------------------------------------------
# The guest uploads access.json {"forward_ports":[{"name":...,"port":...},...]}
# once the topology is up; print the ready-to-run command exactly once.

REMOTIVE_ARGO_ACCESS_PRINTED=0
function remotive_argo_maybe_print_access_instructions() {
  [[ "${REMOTIVE_ARGO_ACCESS_PRINTED}" == "1" ]] && return 0
  local access_uri="${REMOTIVE_ARGO_OUTPUT_URI}/access.json"
  _remotive_argo_gcs_object_exists "${access_uri}" || return 0
  local body entries fwd="" named=""
  body="$(gcloud storage cat "${access_uri}" 2>/dev/null || true)"
  [[ -n "${body}" ]] || return 0
  entries="$(printf '%s' "${body}" | python3 -c 'import json,sys
try:
  for e in json.load(sys.stdin).get("forward_ports", []):
    print("{}:{}".format(e["name"], int(e["port"])))
except Exception:
  pass' 2>/dev/null || true)"
  local entry name port
  while IFS= read -r entry; do
    [[ -n "${entry}" ]] || continue
    name="${entry%:*}"
    port="${entry##*:}"
    fwd+=" -L ${port}:localhost:${port}"
    named+="${named:+, }${name} (${port})"
  done <<<"${entries}"
  REMOTIVE_ARGO_ACCESS_PRINTED=1
  {
    echo "[remotive-argo] ============================================================"
    echo "[remotive-argo] Topology is up on VM ${REMOTIVE_ARGO_VM_NAME}."
    echo "[remotive-argo] Forwarded ports: ${named}"
    echo "[remotive-argo] Reach its UIs via IAP SSH port-forwarding (until keep-alive expires):"
    echo "[remotive-argo]   gcloud compute ssh ${REMOTIVE_ARGO_VM_NAME} --project ${CLOUD_PROJECT} --zone ${CLOUD_ZONE} --tunnel-through-iap --${fwd}"
    echo "[remotive-argo] ============================================================"
  } >&2
  printf '%s\n' "${body}" >"${REMOTIVE_ARGO_LOCAL_ARTIFACT_ROOT}/access.json" || true
}

# -----------------------------------------------------------------------------
# Guest status poll
# -----------------------------------------------------------------------------
function remotive_argo_compute_wall_timeout_sec() {
  if [[ "${REMOTIVE_ARGO_GUEST_WALL_TIMEOUT_SEC}" =~ ^[0-9]+$ ]] && [[ "${REMOTIVE_ARGO_GUEST_WALL_TIMEOUT_SEC}" -gt 0 ]]; then
    echo "${REMOTIVE_ARGO_GUEST_WALL_TIMEOUT_SEC}"
    return 0
  fi
  # base: boot + apt-free topology build + compose pulls; tests; keep-alive; MTK.
  local base=2400 tests=0 keep=0 mtk=0
  [[ "${RUN_TOPOLOGY_TESTS:-false}" == "true" ]] && tests=1800
  local keep_mins="${REMOTIVE_KEEP_ALIVE_TIME:-0}"
  keep_mins="$(echo "${keep_mins}" | tr -d '[:space:]')"
  [[ -n "${keep_mins}" && "${keep_mins}" =~ ^[0-9]+$ && "${keep_mins}" -gt 0 ]] && keep=$((keep_mins * 60))
  [[ -n "${MTK_CONNECT_USERNAME:-}" ]] && mtk=900
  echo $((base + tests + keep + mtk))
}

REMOTE_RC=1
function remotive_argo_poll_guest_status() {
  local timeout_sec="${1:?}"
  local start_sec="${SECONDS}"
  local deadline=$((start_sec + timeout_sec))
  # shellcheck disable=SC2034 # updated via nameref in _remotive_argo_emit_guest_serial_log_chunk
  local guest_serial_start=0
  local status_uri="${REMOTIVE_ARGO_OUTPUT_URI}/status.json"
  echo "[remotive-argo] polling guest status at ${status_uri} (first ${REMOTIVE_ARGO_GUEST_POLL_FIRST_SEC}s, every ${REMOTIVE_ARGO_GUEST_POLL_SEC}s, wall ${timeout_sec}s)" >&2
  sleep "${REMOTIVE_ARGO_GUEST_POLL_FIRST_SEC}"
  while (( SECONDS < deadline )); do
    remotive_argo_maybe_print_access_instructions || true
    if _remotive_argo_gcs_object_exists "${status_uri}"; then
      local body phase rc parsed
      body="$(gcloud storage cat "${status_uri}" 2>/dev/null || true)"
      echo "[remotive-argo] guest status: ${body}" >&2
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
          _remotive_argo_flush_guest_serial_log_to_stderr guest_serial_start || true
          REMOTE_RC="${rc:-0}"
          return 0
          ;;
        failed)
          _remotive_argo_flush_guest_serial_log_to_stderr guest_serial_start || true
          REMOTE_RC="${rc:-1}"
          [[ "${REMOTE_RC}" -eq 0 ]] && REMOTE_RC=1
          return 0
          ;;
        *) ;;
      esac
    else
      echo "[remotive-argo] guest status not yet at ${status_uri} (elapsed $((SECONDS - start_sec))s)" >&2
    fi
    _remotive_argo_emit_guest_serial_log_chunk guest_serial_start || true
    sleep "${REMOTIVE_ARGO_GUEST_POLL_SEC}"
  done
  _remotive_argo_flush_guest_serial_log_to_stderr guest_serial_start || true
  echo "[remotive-argo] ERROR: guest wall timeout (${timeout_sec}s)" >&2
  REMOTE_RC=124
  return 0
}

function remotive_argo_download_guest_outputs() {
  local tgz="${REMOTIVE_ARGO_OUTPUT_URI}/remotive-argo-artifacts.tgz"
  if _remotive_argo_gcs_object_exists "${tgz}"; then
    echo "[remotive-argo] downloading ${tgz}" >&2
    gcloud storage cp "${tgz}" /tmp/remotive-argo-artifacts.tgz
    mkdir -p "${REMOTIVE_ARGO_LOCAL_ARTIFACT_ROOT}"
    tar xzf /tmp/remotive-argo-artifacts.tgz -C "${REMOTIVE_ARGO_LOCAL_ARTIFACT_ROOT}"
  fi
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
export CLOUDSDK_CORE_PROJECT="${CLOUD_PROJECT}"
export GOOGLE_CLOUD_PROJECT="${CLOUD_PROJECT}"

for tool in kubectl gcloud python3; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    echo "[remotive-argo] ERROR: ${tool} required" >&2
    exit 125
  fi
done
if [[ ! -f "${GCP_COMPUTE_REST_PY}" ]]; then
  echo "[remotive-argo] ERROR: missing ${GCP_COMPUTE_REST_PY}" >&2
  exit 125
fi

echo "[remotive-argo] Path B: KCC ComputeInstance + GCS (no IAP ssh/scp from the pod)" >&2
echo "[remotive-argo] template=${REMOTIVE_ARGO_INSTANCE_TEMPLATE} vm=${REMOTIVE_ARGO_VM_NAME} topology=${TOPOLOGY_NAME:-<unset>} serial_port=${REMOTIVE_ARGO_SERIAL_PORT}" >&2

remotive_argo_upload_inputs_to_gcs
remotive_argo_apply_kcc_instance
remotive_argo_wait_kcc_instance
remotive_argo_wait_gcp_instance_visible

_wall="$(remotive_argo_compute_wall_timeout_sec)"
remotive_argo_poll_guest_status "${_wall}"
remotive_argo_download_guest_outputs

if [[ "${REMOTE_RC}" -ne 0 ]]; then
  echo "[remotive-argo] guest run failed effective=${REMOTE_RC}" >&2
  exit "${REMOTE_RC}"
fi

echo "[remotive-argo] ephemeral GCE complete (REMOTE_RC=${REMOTE_RC})" >&2
exit 0
