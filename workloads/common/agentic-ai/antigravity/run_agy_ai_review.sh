#!/usr/bin/env bash

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

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${PIPELINE_REPO_ROOT:-${REPO_ROOT:-${WORKSPACE:-/workspace}}}"

# Shared write-root / wildcard defaults
_GEMINI_ENV="${REPO_ROOT}/workloads/common/agentic-ai/gemini/gemini_environment.sh"
if [[ -f "${_GEMINI_ENV}" ]]; then
  # shellcheck disable=SC1091
  source "${_GEMINI_ENV}"
fi

echo "====== AGY AI review ======"
cd "${GEMINI_ARTIFACT_WRITE_ROOT:-${GEMINI_ANALYSIS_PATH:-${REPO_ROOT}}}" || cd "${REPO_ROOT}" || true

# Same cleanup as gemini_initialise.sh gemini_cleanup (AGY skips full initialise).
# Prevents stale gemini-client-error.zip / step*_output.md on persistent PVCs from
# failing or poisoning the next AGY run.
cleanup_agy_artifacts() {
  local dir="$1"
  [[ -d "${dir}" ]] || return 0
  echo "Cleaning old AI Review artifacts in ${dir}..."
  (
    cd "${dir}" || exit 0
    rm -rf "${GEMINI_ARTIFACT_PATH:-gemini-assist}" traces .gemini
    rm -f step*.*
    rm -f gemini-client-error.zip agentic-step-error.txt
    # shellcheck disable=SC2086
    rm -f ${GEMINI_OUTPUT_FILE_WILDCARD:-headless_output*.json}
    rm -f "${GEMINI_PROMPT_FILENAME:-prompt_file.txt}"
  )
  echo "Cleaned old AI Review artifacts in ${dir}."
}

cleanup_agy_artifacts "${GEMINI_ANALYSIS_PATH:-${WORKSPACE:-${REPO_ROOT}}}"
if [[ "${GEMINI_ARTIFACT_WRITE_ROOT:-}" != "${GEMINI_ANALYSIS_PATH:-}" ]]; then
  cleanup_agy_artifacts "${GEMINI_ARTIFACT_WRITE_ROOT}"
fi

_agy_dir="${SCRIPT_DIR}"
_agy_workspace="${GEMINI_ANALYSIS_PATH:-${WORKSPACE:-${REPO_ROOT}}}"
_agy_workspace="${_agy_workspace%/}"


_agy_log="$(find "${_agy_workspace}" -maxdepth 3 -type f \
  \( -name 'aaos-build.log.tail' -o -name 'aaos-build.log' -o -name 'bsw-build.log' \
     -o -name 'launcher.log' \) -print -quit)"

_agy_args=(
  "${_agy_dir}/harness.py"
  "--workspace=${_agy_workspace}"
  "--mcp-config=${_agy_dir}/mcp_config.json"
  "--markdown-output=gemini-assist/agy_analysis_and_proposed_fix.md"
)
[[ -n "${_agy_log}" ]] && _agy_args+=("--log-file=${_agy_log}")

python3 "${_agy_args[@]}"
exit "$?"