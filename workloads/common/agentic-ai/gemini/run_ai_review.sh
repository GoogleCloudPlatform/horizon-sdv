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

# Description:
#   Headless Gemini review orchestrator for Argo Workflows, Jenkins, and any shell
#   where the monorepo root is known. Callers set GEMINI_* and CLOUD_* (see
#   gemini_environment.sh).
#
#   Sources gemini_environment.sh early (after policy defaults above). Sourcing
#   before setting those defaults would let gemini_environment's :-false win when
#   those vars are unset. gemini_initialise.sh sources the same file again (harmless);
#   you may see the environment diagnostic block twice in logs.
#
# Flow (high level):
#   1. Resolve REPO_ROOT (PIPELINE_REPO_ROOT → REPO_ROOT → WORKSPACE → /workspace).
#   1b. Set GEMINI_LOCATION_GLOBAL / GEMINI_PREVIEW_FEATURES (this script's policy defaults),
#       then source gemini_environment.sh (colours, move_gemini_artifacts, shared CLI env).
#   2. Optional GEMINI_ARTIFACTS_COMMAND (e.g. unzip logs into analysis dir).
#   3. Optional review-pre hook (GEMINI_HOOK_DIR + GEMINI_HOOK_PROFILE).
#   4. Rewrite /workspace-prefixed prompt paths to REPO_ROOT when Argo uses /workspace.
#   5. Analysis runtime (AI_REVIEW_RUNTIME):
#        gemini-cli (default) → gemini_initialise.sh + gemini_analysis.sh
#        agy → workloads/common/agentic-ai/antigravity/harness.py
#   6. Optional review-post hook.
#   7. Optional move_gemini_artifacts (skip if GEMINI_SKIP_MOVE_ARTIFACTS=1).
#   8. Trim empty headless JSON; optional GEMINI_POST_REVIEW_COMMAND.
#   9. Fail if gemini-client-error.zip exists; exit with analysis exit code.
#
# Optional env (caller):
#   GEMINI_HOOK_PROFILE (default generic)
#   GEMINI_HOOK_DIR — relative to REPO_ROOT; hooks:
#       ${REPO_ROOT}/${GEMINI_HOOK_DIR}/review-pre-${GEMINI_HOOK_PROFILE}.sh
#       ${REPO_ROOT}/${GEMINI_HOOK_DIR}/review-post-${GEMINI_HOOK_PROFILE}.sh
#   GEMINI_ANALYSIS_PATH — working directory for analysis (default /workspace)
#   GEMINI_ARTIFACTS_COMMAND — eval once after REPO_ROOT is set
#   GEMINI_POST_REVIEW_COMMAND — eval after move / empty-json cleanup
#   GEMINI_SKIP_MOVE_ARTIFACTS=1 — skip move_gemini_artifacts (e.g. CVD/CTS)

set +e

# Log banners: basename only (script may be renamed or invoked via a symlink path).
_SELF_BASH_NAME="$(basename "${BASH_SOURCE[0]}")"

# ---------------------------------------------------------------------------
# Repo root, policy defaults, gemini_environment.sh
# ---------------------------------------------------------------------------

REPO_ROOT="${PIPELINE_REPO_ROOT:-${REPO_ROOT:-${WORKSPACE:-/workspace}}}"
export REPO_ROOT

# Defaults for this entrypoint (must be set before sourcing gemini_environment.sh,
# which uses GEMINI_PREVIEW_FEATURES=${GEMINI_PREVIEW_FEATURES:-false} etc.).
GEMINI_LOCATION_GLOBAL="${GEMINI_LOCATION_GLOBAL:-true}"
GEMINI_PREVIEW_FEATURES="${GEMINI_PREVIEW_FEATURES:-true}"
export GEMINI_LOCATION_GLOBAL GEMINI_PREVIEW_FEATURES

_GEMINI_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${_GEMINI_SCRIPT_DIR}/gemini_environment.sh"

echo -e "${GREEN}=== Gemini AI review (${_SELF_BASH_NAME}) ===${NC}"
echo -e "${GREEN}REPO_ROOT${NC}=${REPO_ROOT}"

# ---------------------------------------------------------------------------
# Optional staging (GEMINI_ARTIFACTS_COMMAND) before hooks
# ---------------------------------------------------------------------------

if [[ -n "${GEMINI_ARTIFACTS_COMMAND:-}" ]]; then
  echo -e "${GREEN}Running GEMINI_ARTIFACTS_COMMAND${NC}"
  eval "${GEMINI_ARTIFACTS_COMMAND}"
fi

GEMINI_ANALYSIS_PATH="${GEMINI_ANALYSIS_PATH:-/workspace}"
export GEMINI_ANALYSIS_PATH
echo -e "${GREEN}GEMINI_ANALYSIS_PATH${NC}=${GEMINI_ANALYSIS_PATH}"

GEMINI_HOOK_PROFILE="${GEMINI_HOOK_PROFILE:-generic}"
export GEMINI_HOOK_PROFILE

HOOK_REL="${GEMINI_HOOK_DIR:-}"
if [[ -n "${HOOK_REL}" ]]; then
  HOOK_DIR="${REPO_ROOT}/${HOOK_REL#/}"
else
  HOOK_DIR=""
fi

# ---------------------------------------------------------------------------
# Optional pre-hook (review-pre-${GEMINI_HOOK_PROFILE}.sh)
# ---------------------------------------------------------------------------

cd "${REPO_ROOT}" || true
mkdir -p "${GEMINI_ANALYSIS_PATH}" 2>/dev/null || true
PRE_HOOK=""
if [[ -n "${HOOK_DIR}" ]]; then
  PRE_HOOK="${HOOK_DIR}/review-pre-${GEMINI_HOOK_PROFILE}.sh"
fi
if [[ -n "${PRE_HOOK}" && -f "${PRE_HOOK}" ]]; then
  echo -e "${GREEN}Pre-hook${NC}: ${PRE_HOOK}"
  bash "${PRE_HOOK}" || true
fi

cd "${GEMINI_ANALYSIS_PATH}" || true

# ---------------------------------------------------------------------------
# Map Argo /workspace paths to monorepo root (git artifact layout)
# ---------------------------------------------------------------------------

GEMINI_PROMPT_FILE="${GEMINI_PROMPT_FILE:-}"
case "${GEMINI_PROMPT_FILE}" in
  /workspace/*) export GEMINI_PROMPT_FILE="${REPO_ROOT}${GEMINI_PROMPT_FILE#/workspace}" ;;
esac
GEMINI_PROMPT_FILE_2="${GEMINI_PROMPT_FILE_2:-}"
GEMINI_PROMPT_FILE_3="${GEMINI_PROMPT_FILE_3:-}"
case "${GEMINI_PROMPT_FILE_2}" in /workspace/*) GEMINI_PROMPT_FILE_2="${REPO_ROOT}${GEMINI_PROMPT_FILE_2#/workspace}" ;; esac
case "${GEMINI_PROMPT_FILE_3}" in /workspace/*) GEMINI_PROMPT_FILE_3="${REPO_ROOT}${GEMINI_PROMPT_FILE_3#/workspace}" ;; esac
export GEMINI_PROMPT_FILE_2 GEMINI_PROMPT_FILE_3
case "${GEMINI_SKILLS_YAML}" in
  /workspace/*) export GEMINI_SKILLS_YAML="${REPO_ROOT}${GEMINI_SKILLS_YAML#/workspace}" ;;
esac

# ---------------------------------------------------------------------------
# Vertex / project (CLOUD_* → GOOGLE_*) and gemini_initialise + analysis
# ---------------------------------------------------------------------------

if [[ "${GEMINI_LOCATION_GLOBAL}" == "true" ]]; then
  export GOOGLE_CLOUD_LOCATION="global"
else
  export GOOGLE_CLOUD_LOCATION="${CLOUD_REGION}"
fi
export GOOGLE_CLOUD_PROJECT="${CLOUD_PROJECT}"
export GEMINI_MODEL="${GEMINI_MODEL:-}"
export GEMINI_COMMAND_LINE="${GEMINI_COMMAND_LINE}"

GEMINI_AI_EXECUTION_TIMEOUT_MINUTES="${GEMINI_AI_EXECUTION_TIMEOUT_MINUTES:-60}"
export GEMINI_AI_EXECUTION_TIMEOUT_MINUTES

AI_REVIEW_RUNTIME="${AI_REVIEW_RUNTIME:-agy}"
export AI_REVIEW_RUNTIME
echo -e "${GREEN}Analyse runtime${NC}=${AI_REVIEW_RUNTIME} (timeout ${GEMINI_AI_EXECUTION_TIMEOUT_MINUTES}m)…"

case "${AI_REVIEW_RUNTIME}" in
  agy|AGY)
    timeout --kill-after=1m "${GEMINI_AI_EXECUTION_TIMEOUT_MINUTES}m" \
    bash "${REPO_ROOT}/workloads/common/agentic-ai/antigravity/run_agy_ai_review.sh"
    GEMINI_EXIT_CODE="$?"
    if [[ "${GEMINI_EXIT_CODE}" -eq 0 ]]; then
      echo -e "${GREEN}AGY AI review finished OK (exit 0)${NC}"
    else
      echo -e "${ORANGE}AGY AI review exit code ${GEMINI_EXIT_CODE}${NC}"
    fi
    ;;
  gemini-cli)
    "${REPO_ROOT}"/workloads/common/agentic-ai/gemini/gemini_initialise.sh
    timeout --kill-after=1m "${GEMINI_AI_EXECUTION_TIMEOUT_MINUTES}m" "${REPO_ROOT}"/workloads/common/agentic-ai/gemini/gemini_analysis.sh
    GEMINI_EXIT_CODE="$?"
    if [[ "${GEMINI_EXIT_CODE}" -eq 0 ]]; then
      echo -e "${GREEN}gemini_analysis.sh finished OK (exit 0)${NC}"
    else
      echo -e "${ORANGE}gemini_analysis.sh exit code ${GEMINI_EXIT_CODE}${NC}"
    fi
    ;;
  *)
    echo -e "${RED}ERROR: unsupported AI_REVIEW_RUNTIME=${AI_REVIEW_RUNTIME} (use gemini-cli|agy)${NC}"
    GEMINI_EXIT_CODE=1
    ;;
esac

# ---------------------------------------------------------------------------
# Optional post-hook (review-post-${GEMINI_HOOK_PROFILE}.sh)
# ---------------------------------------------------------------------------

POST_HOOK=""
if [[ -n "${HOOK_DIR}" ]]; then
  POST_HOOK="${HOOK_DIR}/review-post-${GEMINI_HOOK_PROFILE}.sh"
fi
if [[ -n "${POST_HOOK}" && -f "${POST_HOOK}" ]]; then
  echo -e "${GREEN}Post-hook${NC}: ${POST_HOOK}"
  bash "${POST_HOOK}" || true
fi

# ---------------------------------------------------------------------------
# Move artifacts, post-review command, client-error check, exit
# ---------------------------------------------------------------------------

cd "${REPO_ROOT}" || true
if [[ "${GEMINI_SKIP_MOVE_ARTIFACTS:-}" != "1" ]]; then
  echo -e "${GREEN}Moving Gemini artifacts${NC} → ${REPO_ROOT}"
  move_gemini_artifacts "${GEMINI_ANALYSIS_PATH%/}" "${REPO_ROOT}" || true
else
  echo -e "${ORANGE}GEMINI_SKIP_MOVE_ARTIFACTS=1 — skipping move_gemini_artifacts${NC}"
fi

find . -type f -name "headless*.json" -size 0 -delete || true
if [[ -n "${GEMINI_POST_REVIEW_COMMAND:-}" ]]; then
  echo -e "${GREEN}Running GEMINI_POST_REVIEW_COMMAND${NC}"
  eval "${GEMINI_POST_REVIEW_COMMAND}"
fi

_wr_err="${GEMINI_ARTIFACT_WRITE_ROOT:-${GEMINI_ANALYSIS_PATH%/}}"
if [[ -f "${GEMINI_ANALYSIS_PATH%/}/gemini-client-error.zip" ]] \
  || [[ -f "${REPO_ROOT}/gemini-client-error.zip" ]] \
  || [[ -n "${_wr_err}" && -f "${_wr_err}/gemini-client-error.zip" ]]; then
  echo -e "${RED}ERROR: gemini-client-error.zip present — marking run failed${NC}"
  GEMINI_EXIT_CODE=1
fi

set -e
echo -e "${GREEN}=== Gemini AI review finished (${_SELF_BASH_NAME}, exit ${GEMINI_EXIT_CODE}) ===${NC}"
exit "${GEMINI_EXIT_CODE}"
