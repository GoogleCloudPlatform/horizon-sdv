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
#   Common environment variables for Gemini CLI (AI Assistant, CTS, Builder jobs).
#   Source this file from gemini_initialise.sh and gemini_analysis.sh; do not execute.
#
# Mandatory (set by Jenkins/Argo):
#   GOOGLE_CLOUD_PROJECT    Google Cloud project identifier
#   GOOGLE_CLOUD_LOCATION   Region (or global)
#   GEMINI_ARTIFACT_ROOT_NAME  Storage URL for artifacts (used by storage jobs)
#
# Optional:
#   GEMINI_ARTIFACT_WRITE_ROOT  Writable dir for .gemini/, CLI outputs, gemini-assist/ (default: WORKSPACE if
#                          writable; else WORKSPACE/test-results if that dir is writable;
#                          else /tmp/gemini-ai-review-<hostname>). Override when pipeline-repo mount is RO (Argo).
#   ANALYSIS_WORKING_DIRECTORY  When set (ai-review CWT), used as GEMINI_ARTIFACT_WRITE_ROOT if root unset.
#   GOOGLE_GENAI_USE_VERTEXAI  Use Vertex AI (default: True)
#   GEMINI_PROMPT_FILE     Step 1 prompt path or base64. Unset → use repo default.
#   GEMINI_PROMPT_FILE_2   Step 2 prompt (required for sequenced). Path or base64.
#   GEMINI_PROMPT_FILE_3   Step 3 prompt (optional). Path or base64.
#   GEMINI_COMMAND_LINE    Full CLI invocation (default: gemini --yolo --output-format json).
#                          To pin model: add --model <name> (e.g. --model gemini-2.5-pro).
#   GEMINI_CLI_VERSION     If set, gemini_initialise.sh installs/upgrades to this version.
#   GEMINI_OUTPUT_FILE_NAME  JSON output file (default: headless_output_<timestamp>.json).
#   GEMINI_ARTIFACT_PATH   Dir for analysis output (default: gemini-assist).
#   GEMINI_ARTIFACT_FILES_WILDCARD  Fallback glob if CLI writes to cwd (default: gemini*.md).
#   GEMINI_ADDITIONAL_ARTIFACTS  Extra paths to archive (e.g. test-results).
#   GEMINI_PREVIEW_FEATURES  Passed to .gemini/settings.json (default: false).
#   GEMINI_LOCATION_GLOBAL  If true, GOOGLE_CLOUD_LOCATION=global.
#   GEMINI_SKILLS_YAML      Path to skills.yaml, or base64-encoded content (file upload).
#                          gemini_initialise.sh decodes if base64 and converts to .gemini/skills/*/SKILL.md.
#   GEMINI_STEP2_PRIOR_CONTEXT_BYTES  If set to a positive integer, max bytes of step1 output appended into step2 composed prompt (head -c).
#                          Unset = no cap (full step1 text). AAOS Builder sets 131072 in Argo gemini-review / Jenkinsfile. Use 0 for explicit no cap.
#   GEMINI_STEP3_PRIOR_STEP1_BYTES  Max bytes of step1_output.md inlined into step 3 composed prompt (gemini_analysis.sh). Unset/0 = full file.
#   GEMINI_STEP3_PRIOR_STEP2_BYTES  Max bytes of step2_output.md inlined into step 3 composed prompt. Unset/0 = full file.
#   GEMINI_FORCE_FILE_STORAGE  Default true: skip OS keychain (required in headless containers without D-Bus).
#   GEMINI_CLI_TRUST_WORKSPACE  Default true: bypass the headless folder-trust check so the CLI loads our
#                          workspace .gemini/settings.json (selectedType=vertex-ai) and skills/policies.
#                          Without this, recent CLI versions run in restricted "safe mode" and ignore
#                          workspace settings, producing: "Please set an Auth method ... GEMINI_API_KEY,
#                          GOOGLE_GENAI_USE_VERTEXAI, GOOGLE_GENAI_USE_GCA". See
#                          https://geminicli.com/docs/cli/trusted-folders/ (Headless and automated environments).
#   TERM                   Forced to xterm-256color when unset or "dumb" (Jenkins sh steps export
#                          TERM=dumb, Argo leaves it unset). Silences the CLI's "Basic terminal
#                          detected (TERM=dumb)..." and "256-color support not detected..."
#                          advisories. Set to any other non-dumb value in the caller to override.
#   COLORTERM              Default truecolor: standard advertisement for 24-bit colour; silences
#                          the CLI's "True color (24-bit) support not detected..." warning.
#   NO_COLOR               Default 1: standard opt-out (https://no-color.org) that suppresses ANSI
#                          styling in the CLI's stderr/stdout, keeping JSON output clean for jq.
#                          Set to empty ("") in the caller to re-enable colour.
GEMINI_PROMPT_FILENAME=${GEMINI_PROMPT_FILENAME:-prompt_file.txt}
GEMINI_PROMPT_FILE=${GEMINI_PROMPT_FILE:-${GEMINI_PROMPT_FILENAME}}
GEMINI_PROMPT_FILE_2=${GEMINI_PROMPT_FILE_2:-}
GEMINI_PROMPT_FILE_3=${GEMINI_PROMPT_FILE_3:-}
GEMINI_COMMAND_LINE=${GEMINI_COMMAND_LINE:-gemini --yolo --output-format json}
GEMINI_CLI_VERSION=${GEMINI_CLI_VERSION:-}
GEMINI_OUTPUT_FILE_NAME=${GEMINI_OUTPUT_FILE_NAME:-headless_output_$(date +%Y%m%d_%H%M%S).json}
GEMINI_OUTPUT_FILE_WILDCARD=${GEMINI_OUTPUT_FILE_WILDCARD:-headless_output*.json}
GEMINI_ARTIFACT_STORAGE_SOLUTION=${GEMINI_ARTIFACT_STORAGE_SOLUTION:-GCS_BUCKET}
GEMINI_ARTIFACT_PATH=${GEMINI_ARTIFACT_PATH:-gemini-assist}
GEMINI_ARTIFACT_FILES_WILDCARD=${GEMINI_ARTIFACT_FILES_WILDCARD:-"gemini*.md"}
GEMINI_ADDITIONAL_ARTIFACTS=${GEMINI_ADDITIONAL_ARTIFACTS:-}
GEMINI_PREVIEW_FEATURES=${GEMINI_PREVIEW_FEATURES:-false}
GEMINI_LOCATION_GLOBAL=${GEMINI_LOCATION_GLOBAL:-false}
GEMINI_SKILLS_YAML=${GEMINI_SKILLS_YAML:-}

# Workspace and artifact storage paths
if [ -z "${WORKSPACE:-}" ]; then
    WORKSPACE="${HOME}"
fi

# Directory for mutable Gemini outputs (.gemini/, decoded prompts, gemini-assist/, *.json).
# Argo ai-review mounts pipeline-repo at WORKSPACE read-only; Jenkins workspace is usually writable.
if [ -z "${GEMINI_ARTIFACT_WRITE_ROOT:-}" ]; then
    if [ -n "${ANALYSIS_WORKING_DIRECTORY:-}" ]; then
        GEMINI_ARTIFACT_WRITE_ROOT="${ANALYSIS_WORKING_DIRECTORY}"
    elif [ -n "${WORKSPACE:-}" ] && [ -w "${WORKSPACE}" ]; then
        GEMINI_ARTIFACT_WRITE_ROOT="${WORKSPACE}"
    elif [ -n "${WORKSPACE:-}" ]; then
        mkdir -p "${WORKSPACE}/test-results" 2>/dev/null || true
        if [ -d "${WORKSPACE}/test-results" ] && [ -w "${WORKSPACE}/test-results" ]; then
            GEMINI_ARTIFACT_WRITE_ROOT="${WORKSPACE}/test-results"
        fi
    fi
    if [ -z "${GEMINI_ARTIFACT_WRITE_ROOT:-}" ]; then
        GEMINI_ARTIFACT_WRITE_ROOT="${TMPDIR:-/tmp}/gemini-ai-review-${HOSTNAME:-run}"
    fi
fi
export GEMINI_ARTIFACT_WRITE_ROOT
mkdir -p "${GEMINI_ARTIFACT_WRITE_ROOT}" 2>/dev/null || true

# Colours for logging (define before first use so sourced under set -u is safe).
GREEN='\033[1;32m'
# shellcheck disable=SC2034
ORANGE='\033[1;33m'
# shellcheck disable=SC2034
RED='\033[1;31m'
NC='\033[0m'

# Options that must be env variables for gemini cli to operate.
export GOOGLE_GENAI_USE_VERTEXAI=${GOOGLE_GENAI_USE_VERTEXAI:-True}
export GOOGLE_CLOUD_PROJECT=${GOOGLE_CLOUD_PROJECT:-}
export GOOGLE_CLOUD_LOCATION=${GOOGLE_CLOUD_LOCATION:-}
# Headless pods (Argo/Jenkins): do not use libsecret/keytar → D-Bus/gnome-keyring (needs DISPLAY).
# Upstream: GEMINI_FORCE_FILE_STORAGE=true uses file storage instead of KeychainTokenStorage.
export GEMINI_FORCE_FILE_STORAGE=${GEMINI_FORCE_FILE_STORAGE:-true}
# Headless pods (Argo/Jenkins): trust the current workspace so the CLI loads our workspace
# .gemini/settings.json (auth, policies, skills). Without this, recent CLI versions enable
# Folder Trust in safe mode and ignore workspace settings, surfacing as: "Please set an Auth
# method ... GEMINI_API_KEY, GOOGLE_GENAI_USE_VERTEXAI, GOOGLE_GENAI_USE_GCA" even though
# GOOGLE_GENAI_USE_VERTEXAI is exported. Equivalent to passing --skip-trust on the CLI.
export GEMINI_CLI_TRUST_WORKSPACE=${GEMINI_CLI_TRUST_WORKSPACE:-true}
# Headless pods (Argo/Jenkins) run without a full TTY. Jenkins sh steps force TERM=dumb and
# Argo leaves TERM unset, which makes the CLI emit three advisories:
#   "Warning: Basic terminal detected (TERM=dumb). Visual rendering will be limited..."
#   "Warning: 256-color support not detected..."
#   "Warning: True color (24-bit) support not detected..."
# TERM=xterm-256color silences the first two (terminfo capability check); COLORTERM=truecolor
# is the standard advertisement for 24-bit colour and silences the third. Overrides in the
# caller are respected; dumb/unset TERM is always replaced. NO_COLOR=1 is the standard opt-out
# (https://no-color.org) and suppresses ANSI styling in the CLI's stderr/stdout, keeping
# JSON output clean for jq.
if [ -z "${TERM:-}" ] || [ "${TERM}" = "dumb" ]; then
    export TERM="xterm-256color"
else
    export TERM
fi
export COLORTERM=${COLORTERM:-truecolor}
export NO_COLOR=${NO_COLOR:-1}

# Store BUILD_NUMBER and JOB_NAME for path in storage.
# shellcheck disable=SC2034
BUILD_NUMBER=${BUILD_NUMBER:-00}
JOB_NAME=${JOB_NAME:-aaos}

# Declare artifact array (absolute paths so gemini_storage.sh works after cd to read-only WORKSPACE).
declare -a GEMINI_ARTIFACT_LIST=()
GEMINI_ARTIFACT_LIST+=(
  "${GEMINI_ARTIFACT_WRITE_ROOT}/${GEMINI_ARTIFACT_PATH}/"
  "${GEMINI_ARTIFACT_WRITE_ROOT}/traces/"
  "${GEMINI_ARTIFACT_WRITE_ROOT}/*.json"
)

# Store additional artifacts.
if [ -n "${GEMINI_ADDITIONAL_ARTIFACTS}" ]; then
    GEMINI_ARTIFACT_LIST+=(
        "${GEMINI_ADDITIONAL_ARTIFACTS}"
    )
fi

# Move artifacts back into workspace. Only required
# if working outside workspaces.
function move_gemini_artifacts() {
    local src_dir="$1"
    local target_dir="$2"

    if [[ -z "$src_dir" || -z "$target_dir" ]]; then
        echo -e "${RED}ERROR: Usage: move_gemini_artifacts <src_dir> <target_dir>${NC}"
        return 1
    fi

    # Move Antigravity traces.
    if [[ -d "${src_dir}/traces" && "${src_dir%/}" != "${target_dir%/}" ]]; then
        mkdir -p "${target_dir}/traces"
        mv "${src_dir}/traces"/* "${target_dir}/traces/" >/dev/null 2>&1 || true
        rmdir "${src_dir}/traces" >/dev/null 2>&1 || true
    fi

    echo -e "${GREEN}Transferring artifacts from ${src_dir} to ${target_dir}${NC}"
    mv "${src_dir}"/"${GEMINI_ARTIFACT_PATH}" "${target_dir}/" >/dev/null 2>&1 || true

    # shellcheck disable=SC2086
    mv "${src_dir}"/${GEMINI_OUTPUT_FILE_WILDCARD} "${target_dir}" >/dev/null 2>&1 || true

    # Sequenced step outputs (so Jenkins archiveArtifacts can find them)
    for f in step1_output.md step2_output.md step3_output.md; do
        [ -f "${src_dir}/${f}" ] && mv "${src_dir}/${f}" "${target_dir}/" >/dev/null 2>&1 || true
    done
}

# Show variables that are applicable to each script.
VARIABLES="Environment:
"

case "$0" in
    *environment.sh)
        VARIABLES+="
        GOOGLE_CLOUD_PROJECT=${GOOGLE_CLOUD_PROJECT}
        GOOGLE_CLOUD_LOCATION=${GOOGLE_CLOUD_LOCATION}
        "
        ;;
    *initialise.sh)
        VARIABLES+="
        GOOGLE_CLOUD_PROJECT=${GOOGLE_CLOUD_PROJECT}
        GOOGLE_CLOUD_LOCATION=${GOOGLE_CLOUD_LOCATION}
        GEMINI_CLI_VERSION=${GEMINI_CLI_VERSION}
        GOOGLE_GENAI_USE_VERTEXAI=${GOOGLE_GENAI_USE_VERTEXAI}

        GEMINI_PROMPT_FILENAME=${GEMINI_PROMPT_FILENAME}
        GEMINI_ARTIFACT_PATH=${GEMINI_ARTIFACT_PATH}
        GEMINI_OUTPUT_FILE_WILDCARD=${GEMINI_OUTPUT_FILE_WILDCARD}

        GEMINI_PREVIEW_FEATURES=${GEMINI_PREVIEW_FEATURES}
        GEMINI_LOCATION_GLOBAL=${GEMINI_LOCATION_GLOBAL}
        GEMINI_CLI_TRUST_WORKSPACE=${GEMINI_CLI_TRUST_WORKSPACE}
        TERM=${TERM}
        COLORTERM=${COLORTERM}
        NO_COLOR=${NO_COLOR}
        "
        ;;
    *analysis*.sh)
        VARIABLES+="
        GEMINI_PROMPT_FILE=${GEMINI_PROMPT_FILE}
        GEMINI_PROMPT_FILE_2=${GEMINI_PROMPT_FILE_2}
        GEMINI_PROMPT_FILE_3=${GEMINI_PROMPT_FILE_3}
        GEMINI_PROMPT_FILENAME=${GEMINI_PROMPT_FILENAME}
        GEMINI_COMMAND_LINE=${GEMINI_COMMAND_LINE}
        GEMINI_OUTPUT_FILE_NAME=${GEMINI_OUTPUT_FILE_NAME}
        GEMINI_CLI_TRUST_WORKSPACE=${GEMINI_CLI_TRUST_WORKSPACE}
        TERM=${TERM}
        COLORTERM=${COLORTERM}
        NO_COLOR=${NO_COLOR}
        "
        ;;
    *storage.sh)
        VARIABLES+="
        GEMINI_ARTIFACT_ROOT_NAME=${GEMINI_ARTIFACT_ROOT_NAME}
        GEMINI_ARTIFACT_STORAGE_SOLUTION=${GEMINI_ARTIFACT_STORAGE_SOLUTION}

        GEMINI_ARTIFACT_PATH=${GEMINI_ARTIFACT_PATH}
        GEMINI_ARTIFACT_FILES_WILDCARD=${GEMINI_ARTIFACT_FILES_WILDCARD}
        GEMINI_ADDITIONAL_ARTIFACTS=${GEMINI_ADDITIONAL_ARTIFACTS}

        BUILD_NUMBER=${BUILD_NUMBER}
        JOB_NAME=${JOB_NAME}
        "
        ;;
    *)
        ;;
esac

VARIABLES+="
        WORKSPACE=${WORKSPACE}
        hostname=$(hostname)

        Kernel Revision: $(uname -r)
"

echo "${VARIABLES}"
