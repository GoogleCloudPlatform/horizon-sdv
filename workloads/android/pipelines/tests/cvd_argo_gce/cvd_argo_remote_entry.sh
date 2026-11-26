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
#   Guest-side CVD or CTS work for one step of an Argo ephemeral-GCE run.
#
#   cvd_argo_guest_startup.sh calls this script twice on the same VM:
#     1. CVD_ARGO_REMOTE_PHASE=main     — start Cuttlefish, optional MTK Connect, run CTS
#     2. CVD_ARGO_REMOTE_PHASE=teardown — stop services, gather logs into a tarball
#
#   The workflow pod never SSHs here; it only polls GCS status.json and downloads
#   cvd-argo-artifacts.tgz after teardown.
#
#   Behaviour matches the Jenkins cvdPipeline / CTS hooks for the paths we support.
#
# Required environment (set by guest startup from job env):
#   CVD_ARGO_REMOTE_ROOT   — unpacked repo workspace on the VM
#   CVD_ARGO_MODE          — cvd | cts
#   CVD_ARGO_REMOTE_PHASE  — main | teardown
#   REMOTE_ARTIFACT_TGZ    — path for gather_artifacts output (uploaded to GCS after teardown)

set -euo pipefail

# -----------------------------------------------------------------------------
# Bootstrap (required env and workspace)
# -----------------------------------------------------------------------------
# Validate inputs, cd into the pipeline checkout, load shared guest helpers.

: "${CVD_ARGO_REMOTE_ROOT:?}"
: "${CVD_ARGO_MODE:?}"
: "${REMOTE_ARTIFACT_TGZ:?}"
: "${CVD_ARGO_REMOTE_PHASE:?}"

export WORKSPACE="${CVD_ARGO_REMOTE_ROOT}"
cd "${WORKSPACE}" || {
  echo "[cvd-argo-remote] ERROR: cd WORKSPACE failed — path missing or unreachable: ${WORKSPACE}" >&2
  exit 1
}

# shellcheck source=/dev/null
source "${WORKSPACE}/workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_guest_common.sh"
_cvd_argo_ensure_home
_trace "[cvd-argo-remote] start mode=${CVD_ARGO_MODE} phase=${CVD_ARGO_REMOTE_PHASE} workspace=${WORKSPACE}"

# -----------------------------------------------------------------------------
# Job identity (Jenkins-compatible; Argo sets BUILD_NUMBER from workflow.uid)
# -----------------------------------------------------------------------------
# MTK testbench names and log zip names use JOB_NAME + BUILD_NUMBER like Jenkins jobs.

BUILD_NUMBER="${BUILD_NUMBER:-0}"
JOB_NAME="${JOB_NAME:-cvd-argo}"
BUILD_USER="${BUILD_USER:-jenkins}"
# BUILD_USER_ID: Keycloak preferred_username from workflow submittedBy / Jenkins BUILD_USER_ID; empty → jenkins.
BUILD_USER_ID="${BUILD_USER_ID:-jenkins}"
export BUILD_NUMBER JOB_NAME BUILD_USER BUILD_USER_ID

MTK_RAN=false
MTK_FAILED=false

# -----------------------------------------------------------------------------
# Guest artifact gather
# -----------------------------------------------------------------------------
# Helpers below build /tmp/cvd-argo-out; gather_artifacts tars it for GCS (and Gemini).

# -----------------------------------------------------------------------------
# _gather_cuttlefish_instance_logs
# -----------------------------------------------------------------------------
# Cuttlefish writes per-VM logs under ~/cf/cuttlefish/instances/cvd-*/logs/ (kernel.log, etc.).

function _gather_cuttlefish_instance_logs() {
  local bundle_dir="${1:?}"
  local instances_root="${HOME}/cf/cuttlefish/instances"
  local inst_dir inst_name

  [[ -d "${instances_root}" ]] || return 0

  for inst_dir in "${instances_root}"/cvd-*; do
    [[ -d "${inst_dir}/logs" ]] || continue
    inst_name="$(basename "${inst_dir}")"
    mkdir -p "${bundle_dir}/cvd/${inst_name}"
    cp -a "${inst_dir}/logs" "${bundle_dir}/cvd/${inst_name}/" 2>/dev/null || true
  done
}

# -----------------------------------------------------------------------------
# _gather_workspace_and_host_logs
# -----------------------------------------------------------------------------
# Top-level host logs and zips from $HOME and WORKSPACE (cvd-*.log, cuttlefish_logs*.zip).

function _gather_workspace_and_host_logs() {
  local bundle_dir="${1:?}"

  cp -f "${HOME}"/cvd-*.log "${bundle_dir}/" 2>/dev/null || true
  cp -f "${WORKSPACE}"/cvd-*.log "${bundle_dir}/" 2>/dev/null || true
  cp -f "${WORKSPACE}"/wifi*.log "${bundle_dir}/" 2>/dev/null || true
  cp -f "${WORKSPACE}"/cuttlefish*.zip "${bundle_dir}/" 2>/dev/null || true
  cp -f "${WORKSPACE}"/cts*.txt "${bundle_dir}/" 2>/dev/null || true
}

# -----------------------------------------------------------------------------
# _gather_cts_result_trees
# -----------------------------------------------------------------------------
# Full CTS output directories when this run executed CTS (not used for CVD-only jobs).

function _gather_cts_result_trees() {
  local bundle_dir="${1:?}"

  if [[ -d "${WORKSPACE}/android-cts-results" ]]; then
    cp -a "${WORKSPACE}/android-cts-results" "${bundle_dir}/" 2>/dev/null || true
  fi
  if [[ -d "${WORKSPACE}/android-cts-results-html" ]]; then
    cp -a "${WORKSPACE}/android-cts-results-html" "${bundle_dir}/" 2>/dev/null || true
  fi
}

# -----------------------------------------------------------------------------
# _strip_artifact_symlinks
# -----------------------------------------------------------------------------
# Argo workflow artifact init rejects symlinks whose target lies outside the bundle
# (e.g. Tradefed results/latest -> /opt/android-cts/...). Drop all symlinks before tar.

function _strip_artifact_symlinks() {
  local bundle_dir="${1:?}"
  find "${bundle_dir}" -type l -print -delete 2>/dev/null || true
}

# -----------------------------------------------------------------------------
# gather_artifacts
# -----------------------------------------------------------------------------
# Orchestrates the helpers above, adds cvd-argo.marker, writes REMOTE_ARTIFACT_TGZ for GCS upload.

function gather_artifacts() {
  local bundle_dir="/tmp/cvd-argo-out"
  local artifact_tgz="${REMOTE_ARTIFACT_TGZ}"

  _trace "[cvd-argo-remote] gather_artifacts → ${artifact_tgz}"
  rm -rf "${bundle_dir}" 2>/dev/null || true
  mkdir -p "${bundle_dir}" || return 1

  shopt -s nullglob
  _cvd_argo_ensure_home
  _gather_workspace_and_host_logs "${bundle_dir}"
  _gather_cuttlefish_instance_logs "${bundle_dir}"
  _gather_cts_result_trees "${bundle_dir}"
  _strip_artifact_symlinks "${bundle_dir}"
  cp -f /tmp/cvd-argo.marker "${bundle_dir}/cvd-argo.marker" 2>/dev/null || true
  shopt -u nullglob

  tar czf "${artifact_tgz}" -C /tmp cvd-argo-out || return 1
  _trace "[cvd-argo-remote] gather_artifacts ok"
}

# -----------------------------------------------------------------------------
# MTK Connect
# -----------------------------------------------------------------------------
# Optional remote device farm tunnel; /tmp/cvd-argo.marker records state for teardown.

# -----------------------------------------------------------------------------
# _mtk_testbench_user
# -----------------------------------------------------------------------------
# Return MTK testbench owner: everyone (public) or BUILD_USER_ID.

function _mtk_testbench_user() {
  if [[ "${MTK_CONNECT_PUBLIC:-false}" == "true" ]]; then
    echo everyone
  else
    echo "${BUILD_USER_ID:-jenkins}"
  fi
}

# -----------------------------------------------------------------------------
# run_mtk_start
# -----------------------------------------------------------------------------
# Start MTK Connect tunnel; record success or failure in /tmp/cvd-argo.marker.

function run_mtk_start() {
  if [[ -z "${MTK_CONNECT_USERNAME:-}" ]] || [[ -z "${MTK_CONNECT_PASSWORD:-}" ]]; then
    echo "[cvd-argo-remote] MTK credentials not set; skipping MTK Connect"
    return 0
  fi
  local _mtk_user
  _mtk_user="$(_mtk_testbench_user)"
  _trace "[cvd-argo-remote] MTK Connect --start: MTK_CONNECT_PUBLIC=${MTK_CONNECT_PUBLIC:-false} BUILD_USER_ID=${BUILD_USER_ID:-<unset>} → MTK_CONNECT_TESTBENCH_USER=${_mtk_user} TESTBENCH=${JOB_NAME}-${BUILD_NUMBER}"
  MTK_RAN=true
  cat >/tmp/cvd-argo.marker <<'EOF'
export MTK_RAN=true
export MTK_CONNECT_STAGE_FAILED=false
:
EOF
  pushd "${WORKSPACE}/workloads/common/mtk-connect" >/dev/null
  set +e
  sudo \
    MTK_CONNECT_TUNNEL_PORT="${MTK_CONNECT_TUNNEL_PORT:-8555}" \
    MTK_CONNECT_DOMAIN="${HORIZON_DOMAIN:?}" \
    MTK_CONNECT_USERNAME="${MTK_CONNECT_USERNAME}" \
    MTK_CONNECT_PASSWORD="${MTK_CONNECT_PASSWORD}" \
    MTK_CONNECTED_DEVICES="${NUM_INSTANCES:?}" \
    MTK_CONNECT_TEST_ARTIFACT="${CUTTLEFISH_DOWNLOAD_URL:?}" \
    MTK_CONNECT_TESTBENCH="${JOB_NAME}-${BUILD_NUMBER}" \
    MTK_CONNECT_TESTBENCH_USER="${_mtk_user}" \
    timeout 15m bash ./mtk_connect.sh --start
  local st=$?
  # Do not let popd failure mark MTK as failed: mtk_connect may alter cwd/stack; summary can succeed then teardown=1.
  popd >/dev/null || true
  if [[ "${st}" -ne 0 ]]; then
    MTK_FAILED=true
    cat >/tmp/cvd-argo.marker <<'EOF'
export MTK_RAN=true
export MTK_CONNECT_STAGE_FAILED=true
:
EOF
    echo "[cvd-argo-remote] MTK Connect --start failed (${st})" >&2
    return "${st}"
  fi
  return 0
}

# -----------------------------------------------------------------------------
# run_mtk_stop
# -----------------------------------------------------------------------------
# Stop MTK Connect if main started it; errors are non-fatal (|| true).

function run_mtk_stop() {
  [[ "${MTK_RAN}" == "true" ]] || return 0
  pushd "${WORKSPACE}/workloads/common/mtk-connect" >/dev/null || return 0
  set +e
  sudo \
    MTK_CONNECT_TUNNEL_PORT="${MTK_CONNECT_TUNNEL_PORT:-8555}" \
    MTK_CONNECT_DOMAIN="${HORIZON_DOMAIN:-}" \
    MTK_CONNECT_USERNAME="${MTK_CONNECT_USERNAME:-}" \
    MTK_CONNECT_PASSWORD="${MTK_CONNECT_PASSWORD:-}" \
    MTK_CONNECTED_DEVICES="${NUM_INSTANCES:-1}" \
    MTK_CONNECT_TESTBENCH="${JOB_NAME}-${BUILD_NUMBER}" \
    timeout 10m bash ./mtk_connect.sh --stop || true
  set -e
  popd >/dev/null || true
}

# -----------------------------------------------------------------------------
# CVD lifecycle
# -----------------------------------------------------------------------------
# Start/stop Cuttlefish and optional keep-alive; main and teardown entry points below.

# -----------------------------------------------------------------------------
# run_cvd_stop
# -----------------------------------------------------------------------------
# Wraps cvd_start_stop.sh --stop so Cuttlefish releases resources before gather.

function run_cvd_stop() {
  set +e
  if [[ -n "${CUTTLEFISH_DOWNLOAD_URL:-}" ]]; then
    (
      cd "${WORKSPACE}" || exit 1
      bash ./workloads/android/pipelines/tests/cvd_launcher/cvd_start_stop.sh --stop || true
    )
  fi
  set -e
}

# -----------------------------------------------------------------------------
# CTS execution
# -----------------------------------------------------------------------------
# Full plan runs on the guest after Cuttlefish is up (list-only stays on Jenkins).

# -----------------------------------------------------------------------------
# cts_full_run
# -----------------------------------------------------------------------------
# Run CTS initialise + full test plan after Cuttlefish is up (run_cts_main).

function cts_full_run() {
  (
    cd "${WORKSPACE}" || exit 1
    ANDROID_VERSION="${ANDROID_VERSION:?}" \
      CTS_DOWNLOAD_URL="${CTS_DOWNLOAD_URL:-}" \
      bash ./workloads/android/pipelines/tests/cts_execution/cts_initialise.sh
    CTS_TESTPLAN="${CTS_TESTPLAN:-cts-virtual-device-stable}" \
      CTS_MODULE="${CTS_MODULE:-}" \
      CTS_TIMEOUT="${CTS_TIMEOUT:-600}" \
      CTS_RETRY_STRATEGY="${CTS_RETRY_STRATEGY:-RETRY_ANY_FAILURE}" \
      CTS_MAX_TESTCASE_RUN_COUNT="${CTS_MAX_TESTCASE_RUN_COUNT:-2}" \
      SHARD_COUNT="${SHARD_COUNT:-${NUM_INSTANCES:-1}}" \
      bash ./workloads/android/pipelines/tests/cts_execution/cts_execution.sh
  )
}

# -----------------------------------------------------------------------------
# keep_alive_sleep
# -----------------------------------------------------------------------------
# Optional delay after main work so engineers can attach before teardown stops the VM.

function keep_alive_sleep() {
  local mins="${CUTTLEFISH_KEEP_ALIVE_TIME:-0}"
  mins="$(echo "${mins}" | tr -d '[:space:]')"
  if [[ -z "${mins}" || "${mins}" == "0" ]]; then
    return 0
  fi
  echo "[cvd-argo-remote] keep-alive ${mins} minutes"
  sleep "$((mins * 60))"
}

# -----------------------------------------------------------------------------
# _restore_mtk_state_for_teardown
# -----------------------------------------------------------------------------
# Re-load MTK_RAN / MTK_FAILED from marker files (main and teardown are separate processes).

function _restore_mtk_state_for_teardown() {
  MTK_RAN=false
  MTK_FAILED=false
  _trace "[cvd-argo-remote] teardown: sourcing /tmp/cvd-argo.marker and mtk-failed flag"
  if [[ -f /tmp/cvd-argo.marker ]]; then
    set +e
    # shellcheck disable=SC1091
    source /tmp/cvd-argo.marker
    local _marker_rc=$?
    set -e
    if [[ "${_marker_rc}" -ne 0 ]]; then
      echo "[cvd-argo-remote] ERROR: source /tmp/cvd-argo.marker returned ${_marker_rc} (under set -e this used to exit silently). Marker contents:" >&2
      sed -n '1,40p' /tmp/cvd-argo.marker >&2 || true
      exit 1
    fi
  fi
  if [[ -f /tmp/cvd-argo-mtk-failed.flag ]]; then
    local raw
    raw="$(tr -d '[:space:]' < /tmp/cvd-argo-mtk-failed.flag || true)"
    # Do not use [[ ... ]] && MTK_FAILED=true under set -e: a false test exits the shell (last command in && chain).
    if [[ "${raw}" == "true" ]]; then
      MTK_FAILED=true
    fi
  fi
}

# -----------------------------------------------------------------------------
# _write_mtk_failed_flag
# -----------------------------------------------------------------------------
# Persist MTK failure for teardown to read after main exits.

function _write_mtk_failed_flag() {
  if [[ "${MTK_FAILED}" == "true" ]]; then
    printf 'true\n' >/tmp/cvd-argo-mtk-failed.flag
  else
    printf 'false\n' >/tmp/cvd-argo-mtk-failed.flag
  fi
}

# -----------------------------------------------------------------------------
# run_cvd_main
# -----------------------------------------------------------------------------
# Start Cuttlefish, optional MTK, keep-alive; exit non-zero if MTK --start failed.

function run_cvd_main() {
  cat >/tmp/cvd-argo.marker <<'EOF'
export MTK_RAN=false
:
EOF
  rm -f /tmp/cvd-argo-mtk-failed.flag 2>/dev/null || true
  MTK_RAN=false
  MTK_FAILED=false

  [[ -n "${CUTTLEFISH_DOWNLOAD_URL:-}" ]] || {
    echo "[cvd-argo-remote] CUTTLEFISH_DOWNLOAD_URL is required" >&2
    exit 2
  }

  bash ./workloads/android/pipelines/tests/cvd_launcher/cvd_start_stop.sh --start

  MTK_FAILED=false
  run_mtk_start || MTK_FAILED=true

  keep_alive_sleep

  _write_mtk_failed_flag
  if [[ "${MTK_FAILED}" == "true" ]]; then
    echo "[cvd-argo-remote] main(cvd): exiting 1 — MTK Connect --start failed (see earlier logs)" >&2
    exit 1
  fi
  exit 0
}

# -----------------------------------------------------------------------------
# run_cvd_teardown
# -----------------------------------------------------------------------------
# Stop MTK/CVD, gather artifacts; exit non-zero if MTK_FAILED was set during main.

function run_cvd_teardown() {
  _trace "[cvd-argo-remote] teardown(cvd): enter"

  _restore_mtk_state_for_teardown

  _trace "[cvd-argo-remote] teardown(cvd): MTK_RAN=${MTK_RAN:-} MTK_FAILED=${MTK_FAILED:-} (mtk_connect --stop is non-fatal: || true)"
  run_mtk_stop
  _trace "[cvd-argo-remote] teardown(cvd): run_mtk_stop finished"
  run_cvd_stop
  _trace "[cvd-argo-remote] teardown(cvd): run_cvd_stop finished (next: gather_artifacts)"
  if ! gather_artifacts; then
    echo "[cvd-argo-remote] ERROR: gather_artifacts failed" >&2
    exit 1
  fi

  if [[ "${MTK_FAILED}" == "true" ]]; then
    echo "[cvd-argo-remote] teardown(cvd): exiting 1 because MTK_FAILED flag was true after main (see /tmp/cvd-argo-mtk-failed.flag)" >&2
    exit 1
  fi
  _trace "[cvd-argo-remote] teardown(cvd): complete exit 0"
  exit 0
}

# -----------------------------------------------------------------------------
# CTS mode (main and teardown)
# -----------------------------------------------------------------------------
# Same two-phase pattern as CVD: main runs tests, teardown stops services and gathers logs.

# -----------------------------------------------------------------------------
# run_cts_main
# -----------------------------------------------------------------------------
# Start Cuttlefish, optional MTK, run cts_full_run, keep-alive when MTK enabled.

function run_cts_main() {
  cat >/tmp/cvd-argo.marker <<'EOF'
export MTK_RAN=false
:
EOF
  rm -f /tmp/cvd-argo-mtk-failed.flag 2>/dev/null || true
  MTK_RAN=false
  MTK_FAILED=false

  [[ -n "${CUTTLEFISH_DOWNLOAD_URL:-}" ]] || {
    echo "[cvd-argo-remote] CUTTLEFISH_DOWNLOAD_URL is required for CTS full run" >&2
    exit 2
  }

  bash ./workloads/android/pipelines/tests/cvd_launcher/cvd_start_stop.sh --start

  MTK_FAILED=false
  if [[ "${MTK_CONNECT_ENABLE:-false}" == "true" ]]; then
    run_mtk_start || MTK_FAILED=true
  fi

  if [[ "${MTK_FAILED}" != "true" ]]; then
    cts_full_run
  else
    echo "[cvd-argo-remote] skipping CTS because MTK Connect failed" >&2
  fi

  if [[ "${MTK_CONNECT_ENABLE:-false}" == "true" ]]; then
    keep_alive_sleep
  fi

  _write_mtk_failed_flag
  if [[ "${MTK_FAILED}" == "true" ]] && [[ "${MTK_CONNECT_ENABLE:-false}" == "true" ]]; then
    echo "[cvd-argo-remote] main(cts): exiting 1 — MTK Connect --start failed" >&2
    exit 1
  fi
  exit 0
}

# -----------------------------------------------------------------------------
# run_cts_teardown
# -----------------------------------------------------------------------------
# Stop MTK/CVD, gather artifacts; exit non-zero if MTK_FAILED was set during main.

function run_cts_teardown() {
  _trace "[cvd-argo-remote] teardown(cts): enter"

  _restore_mtk_state_for_teardown

  _trace "[cvd-argo-remote] teardown(cts): MTK_RAN=${MTK_RAN:-} MTK_FAILED=${MTK_FAILED:-} (mtk_connect --stop is non-fatal: || true)"
  run_mtk_stop
  _trace "[cvd-argo-remote] teardown(cts): run_mtk_stop finished"
  run_cvd_stop
  _trace "[cvd-argo-remote] teardown(cts): run_cvd_stop finished (next: gather_artifacts)"
  if ! gather_artifacts; then
    echo "[cvd-argo-remote] ERROR: gather_artifacts failed" >&2
    exit 1
  fi

  if [[ "${MTK_FAILED}" == "true" ]]; then
    echo "[cvd-argo-remote] teardown(cts): exiting 1 because MTK_FAILED flag was true after main" >&2
    exit 1
  fi
  _trace "[cvd-argo-remote] teardown(cts): complete exit 0"
  exit 0
}

# -----------------------------------------------------------------------------
# Dispatch (mode and phase)
# -----------------------------------------------------------------------------
# Guest startup sets CVD_ARGO_MODE and CVD_ARGO_REMOTE_PHASE, then execs this script.

case "${CVD_ARGO_MODE}:${CVD_ARGO_REMOTE_PHASE}" in
  cvd:main) run_cvd_main ;;
  cvd:teardown) run_cvd_teardown ;;
  cts:main) run_cts_main ;;
  cts:teardown) run_cts_teardown ;;
  *)
    echo "[cvd-argo-remote] Unknown CVD_ARGO_MODE=${CVD_ARGO_MODE} CVD_ARGO_REMOTE_PHASE=${CVD_ARGO_REMOTE_PHASE}" >&2
    exit 2
    ;;
esac
