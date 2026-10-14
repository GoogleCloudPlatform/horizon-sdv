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
#   Shared post-hook for CVD/CTS Argo gemini-review (review-post-cvd.sh / review-post-cts.sh).
#   Uploads Gemini outputs and test-results via gemini_storage.sh after run_ai_review.sh.

set +e
REPO_ROOT="${PIPELINE_REPO_ROOT:-${REPO_ROOT:-/workspace}}"
rm -rf "${REPO_ROOT}/test-results/cvd" 2>/dev/null || true

if [[ -z "${GEMINI_ARTIFACT_ROOT_NAME:-}" ]] && [[ -n "${ANDROID_BUILD_BUCKET_ROOT_NAME:-}" ]]; then
  export GEMINI_ARTIFACT_ROOT_NAME="${ANDROID_BUILD_BUCKET_ROOT_NAME}"
fi

if [[ -n "${GEMINI_ARTIFACT_ROOT_NAME:-}" ]]; then
  unset STORAGE_BUCKET_DESTINATION 2>/dev/null || true
  if [[ -n "${GEMINI_STORAGE_BUCKET_DESTINATION:-}" ]]; then
    export STORAGE_BUCKET_DESTINATION="${GEMINI_STORAGE_BUCKET_DESTINATION}"
  fi
  _have_bucket=""
  [[ -n "${STORAGE_BUCKET_DESTINATION:-}" ]] && _have_bucket=1
  _have_jenkins_job=""
  if [[ -n "${GEMINI_STORAGE_JOB_NAME:-}" ]] && [[ -n "${GEMINI_STORAGE_BUILD_NUMBER:-}" ]]; then
    _have_jenkins_job=1
    export JOB_NAME="${GEMINI_STORAGE_JOB_NAME}"
    export BUILD_NUMBER="${GEMINI_STORAGE_BUILD_NUMBER}"
  fi
  if [[ -n "${_have_bucket}" ]] || [[ -n "${_have_jenkins_job}" ]]; then
  # Review writes gemini-assist/ and traces/ into the test-results PVC.
  # Move them out before upload so storage keeps one copy beside test-results.
  _results="${GEMINI_ANALYSIS_PATH:-/workspace/test-results}"
  _results="${_results%/}"
  _stage="$(mktemp -d "${TMPDIR:-/tmp}/gemini-artifacts.XXXXXX")"

  for _name in "${GEMINI_ARTIFACT_PATH:-gemini-assist}" traces; do
    if [[ -e "${_results}/${_name}" ]]; then
      mv "${_results}/${_name}" "${_stage}/"
    fi
  done

  shopt -s nullglob
  for _json in "${_results}"/headless_output*.json "${_results}"/gemini-client-error.zip; do
    mv "${_json}" "${_stage}/"
  done
  shopt -u nullglob

  export GEMINI_ARTIFACT_WRITE_ROOT="${_stage}"

  # shellcheck disable=SC1091
  source "${REPO_ROOT}/workloads/common/agentic-ai/gemini/gemini_environment.sh"
  "${REPO_ROOT}/workloads/common/agentic-ai/gemini/gemini_storage.sh" || true
  else
    echo "WARN: GEMINI_ARTIFACT_ROOT_NAME set but no GEMINI_STORAGE_BUCKET_DESTINATION or Jenkins job/build; skipping gemini_storage.sh" >&2
  fi
fi
