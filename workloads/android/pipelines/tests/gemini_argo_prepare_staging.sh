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
#   Prepares the Gemini AI review step for CVD/CTS Argo workflows.
#
#   Earlier in the workflow, the ephemeral-GCE pod downloaded the guest tarball into
#   /tmp/cvd-argo-artifacts/cvd-argo-out. This script copies that tree into
#   /workspace/test-results on the gemini-test-results PVC, unpacks Cuttlefish zip
#   logs, and (for CTS) mirrors a few report files to the paths Gemini prompts expect.
#
#   If MTK Connect failed on the guest, we skip Gemini and optionally upload artifacts
#   to GCS instead (same path as when AI review is disabled).
#
#   Next step in the DAG: gemini-review runs run_ai_review.sh with GEMINI_SKIP_MOVE_ARTIFACTS=1.
#
# Environment:
#   WORKSPACE                     Pipeline repo root (may be /horizon with sharedPipelineWorkspace).
#   GEMINI_TEST_RESULTS_DIR       Must match gemini-review PVC mount (default /workspace/test-results).
#   TEST_RESULTS_STAGING_GCS_URI  Per-run GCS prefix; used for MTK skip upload and artifact fallback.
#   BUILD_NUMBER                  Matches workflow.uid in Argo (cuttlefish_logs zip name).
#   GEMINI_PREPARE_MODE           cvd | cts
#   CLOUD_PROJECT                 Required for MTK-skip staging upload path.

set -euo pipefail

export WORKSPACE="${WORKSPACE:-/workspace}"

readonly GEMINI_SKIP_FLAG=/tmp/gemini-skip.flag
readonly CVD_ARGO_ARTIFACT_ROOT=/tmp/cvd-argo-artifacts
readonly CVD_ARGO_BUNDLE_DIR="${CVD_ARGO_ARTIFACT_ROOT}/cvd-argo-out"
readonly MTK_FAILED_META="${CVD_ARGO_ARTIFACT_ROOT}/.meta/mtk_connect_stage_failed"
# gemini-test-results PVC is always mounted at /workspace/test-results (not ${WORKSPACE}/test-results).
readonly GEMINI_TEST_RESULTS_DIR="${GEMINI_TEST_RESULTS_DIR:-/workspace/test-results}"
readonly GEMINI_CVD_LOGS_DIR="${GEMINI_TEST_RESULTS_DIR}/cvd"

# -----------------------------------------------------------------------------
# MTK skip (no gemini-review)
# -----------------------------------------------------------------------------
# Guest writes .meta/mtk_connect_stage_failed when MTK --start fails; do not call Gemini.

function _gemini_exit_if_mtk_stage_failed() {
  if [[ ! -f "${MTK_FAILED_META}" ]]; then
    return 1
  fi
  if [[ "$(tr -d '[:space:]' < "${MTK_FAILED_META}")" != "true" ]]; then
    return 1
  fi

  echo "[gemini] skipping AI review: MTK Connect stage failed on guest" >&2
  echo -n "true" >"${GEMINI_SKIP_FLAG}"

  if [[ -n "${TEST_RESULTS_STAGING_GCS_URI:-}" ]] && [[ -n "${CLOUD_PROJECT:-}" ]]; then
    export GEMINI_TEST_RESULTS_STAGING_URI="${TEST_RESULTS_STAGING_GCS_URI}"
    export CVD_ARGO_LOCAL_ARTIFACT_ROOT="${CVD_ARGO_ARTIFACT_ROOT}"
    bash "${WORKSPACE}/workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_sync_vm_artifacts_to_staging.sh" \
      || echo "[gemini] WARN: MTK skip staging upload failed" >&2
  fi
  exit 0
}

# -----------------------------------------------------------------------------
# _gemini_fetch_artifacts_from_gcs
# -----------------------------------------------------------------------------
# When the Argo cvd-argo-artifacts input is empty, download the guest tarball from GCS.

function _gemini_fetch_artifacts_from_gcs() {
  local tgz_uri staging="${TEST_RESULTS_STAGING_GCS_URI:-}"

  [[ -n "${staging}" ]] || return 1
  [[ -d "${CVD_ARGO_BUNDLE_DIR}" ]] && return 0

  tgz_uri="${staging%/}/ephemeral-output/cvd-argo-artifacts.tgz"
  if ! gcloud storage ls "${tgz_uri}" >/dev/null 2>&1; then
    echo "[gemini] WARN: no guest tarball at ${tgz_uri}" >&2
    return 1
  fi

  echo "[gemini] fetching guest bundle from ${tgz_uri}" >&2
  mkdir -p "${CVD_ARGO_ARTIFACT_ROOT}"
  gcloud storage cp "${tgz_uri}" /tmp/cvd-argo-artifacts.tgz
  rm -rf "${CVD_ARGO_BUNDLE_DIR}"
  tar xzf /tmp/cvd-argo-artifacts.tgz -C "${CVD_ARGO_ARTIFACT_ROOT}"
  return 0
}

# -----------------------------------------------------------------------------
# Merge VM artifact bundle into test-results
# -----------------------------------------------------------------------------
# Flatten cvd-argo-out into test-results so run_ai_review.sh sees the same layout as Jenkins.

function _gemini_copy_vm_bundle() {
  mkdir -p "${GEMINI_TEST_RESULTS_DIR}"

  if [[ ! -d "${CVD_ARGO_BUNDLE_DIR}" ]]; then
    echo "[gemini] WARN: missing ${CVD_ARGO_BUNDLE_DIR} (guest gather_artifacts may have failed)" >&2
    return 0
  fi

  cp -r "${CVD_ARGO_BUNDLE_DIR}/." "${GEMINI_TEST_RESULTS_DIR}/" || true
  rm -rf "${GEMINI_TEST_RESULTS_DIR}/.meta"
  find "${GEMINI_TEST_RESULTS_DIR}" -name 'cvd-argo.marker' -delete 2>/dev/null || true
}

# -----------------------------------------------------------------------------
# Unpack Cuttlefish logs for Gemini triage
# -----------------------------------------------------------------------------
# cuttlefish_logs-${BUILD_NUMBER}.zip from the guest lands under test-results/cvd/.

function _gemini_unpack_cuttlefish_zips() {
  mkdir -p "${GEMINI_CVD_LOGS_DIR}"
  shopt -s nullglob
  local zip_path
  for zip_path in "${GEMINI_TEST_RESULTS_DIR}"/cuttlefish_logs*.zip; do
    echo "[gemini] unzip ${zip_path} → ${GEMINI_CVD_LOGS_DIR}/" >&2
    unzip -q -o "${zip_path}" -d "${GEMINI_CVD_LOGS_DIR}" 2>/dev/null \
      || echo "[gemini] WARN: unzip failed: ${zip_path}" >&2
  done
  shopt -u nullglob
}

# -----------------------------------------------------------------------------
# Staged log inventory (stderr diagnostics)
# -----------------------------------------------------------------------------
# Warn when kernel.log and host cvd-*.log are both missing (common gather_artifacts failure).

function _gemini_has_cts_phase0_artifacts() {
  [[ -f "${GEMINI_TEST_RESULTS_DIR}/invocation_summary.txt" ]] \
    || [[ -f "${GEMINI_TEST_RESULTS_DIR}/test_result_failures_suite.html" ]] \
    || [[ -f "${GEMINI_TEST_RESULTS_DIR}/android-cts-results/invocation_summary.txt" ]] \
    || [[ -f "${GEMINI_TEST_RESULTS_DIR}/android-cts-results-html/test_result_failures_suite.html" ]]
}

function _gemini_report_log_inventory() {
  local kernel_logs host_logs zips cts_phase0=false
  kernel_logs="$(find "${GEMINI_TEST_RESULTS_DIR}" -path '*/cvd*/logs/kernel.log' 2>/dev/null | wc -l | tr -d '[:space:]')"
  host_logs="$(find "${GEMINI_TEST_RESULTS_DIR}" -maxdepth 1 -name 'cvd-*.log' 2>/dev/null | wc -l | tr -d '[:space:]')"
  zips="$(find "${GEMINI_TEST_RESULTS_DIR}" -maxdepth 1 -name 'cuttlefish_logs*.zip' 2>/dev/null | wc -l | tr -d '[:space:]')"
  if _gemini_has_cts_phase0_artifacts; then
    cts_phase0=true
  fi

  echo "[gemini] inventory: kernel.log=${kernel_logs:-0} host cvd-*.log=${host_logs:-0} cuttlefish_zip=${zips:-0} cts_phase0=${cts_phase0} dir=${GEMINI_TEST_RESULTS_DIR} workspace=${WORKSPACE}" >&2

  if [[ "${kernel_logs:-0}" -eq 0 && "${host_logs:-0}" -eq 0 ]]; then
    if [[ "${GEMINI_PREPARE_MODE:-cvd}" == "cts" && "${cts_phase0}" == true ]]; then
      echo "[gemini] CTS Phase 0 suite artifacts present (CVD kernel/host logs optional for triage)" >&2
      return 0
    fi
    echo "[gemini] WARN: no CVD logs for triage — check guest gather_artifacts, Argo artifact, or GCS ephemeral-output" >&2
    return 1
  fi
  return 0
}

# -----------------------------------------------------------------------------
# _gemini_skip_review_no_logs
# -----------------------------------------------------------------------------
# Avoid gemini-review on an empty PVC (would only see prompt/json noise).

function _gemini_skip_review_no_logs() {
  if [[ "${GEMINI_PREPARE_MODE:-cvd}" == "cts" ]]; then
    echo "[gemini] skipping AI review: no CTS suite artifacts or CVD logs under ${GEMINI_TEST_RESULTS_DIR}" >&2
  else
    echo "[gemini] skipping AI review: no CVD logs staged under ${GEMINI_TEST_RESULTS_DIR}" >&2
  fi
  echo -n "true" >"${GEMINI_SKIP_FLAG}"
  exit 0
}

# -----------------------------------------------------------------------------
# CTS Phase 0 file mirror
# -----------------------------------------------------------------------------
# CTS prompts expect invocation_summary.txt and test_result_failures_suite.html at test-results root.

function _gemini_mirror_cts_phase0_files() {
  local mirrored=false

  if [[ -f "${GEMINI_TEST_RESULTS_DIR}/android-cts-results/invocation_summary.txt" ]] \
    && [[ ! -f "${GEMINI_TEST_RESULTS_DIR}/invocation_summary.txt" ]]; then
    cp -f "${GEMINI_TEST_RESULTS_DIR}/android-cts-results/invocation_summary.txt" \
      "${GEMINI_TEST_RESULTS_DIR}/invocation_summary.txt"
    mirrored=true
  fi

  if [[ -f "${GEMINI_TEST_RESULTS_DIR}/android-cts-results-html/test_result_failures_suite.html" ]] \
    && [[ ! -f "${GEMINI_TEST_RESULTS_DIR}/test_result_failures_suite.html" ]]; then
    cp -f "${GEMINI_TEST_RESULTS_DIR}/android-cts-results-html/test_result_failures_suite.html" \
      "${GEMINI_TEST_RESULTS_DIR}/test_result_failures_suite.html"
    mirrored=true
  fi

  [[ "${mirrored}" == true ]] && echo "[gemini] CTS Phase 0 files mirrored to test-results root" >&2
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------

function gemini_prepare_staging_main() {
  _gemini_exit_if_mtk_stage_failed || true
  echo -n "false" >"${GEMINI_SKIP_FLAG}"

  _gemini_fetch_artifacts_from_gcs || true
  _gemini_copy_vm_bundle
  _gemini_unpack_cuttlefish_zips
  if [[ "${GEMINI_PREPARE_MODE:-cvd}" == "cts" ]]; then
    _gemini_mirror_cts_phase0_files
  fi
  if ! _gemini_report_log_inventory; then
    _gemini_unpack_cuttlefish_zips
    if [[ "${GEMINI_PREPARE_MODE:-cvd}" == "cts" ]]; then
      _gemini_mirror_cts_phase0_files
    fi
    _gemini_report_log_inventory || _gemini_skip_review_no_logs
  fi
}

gemini_prepare_staging_main
