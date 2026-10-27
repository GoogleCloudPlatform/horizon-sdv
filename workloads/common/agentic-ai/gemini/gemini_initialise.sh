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
#   One-time setup before running Gemini analysis. Run from job workspace after
#   GEMINI_* env vars are set. Does: cleanup → install/upgrade CLI → auth →
#   skills setup. Used by Gemini AI Assistant, CTS, and Builder jobs.
#
# Flow:
#   1. gemini_cleanup     Remove previous run artifacts (.gemini, step*.*, etc.)
#   2. gemini_install    Optional: install/upgrade gemini-cli (if GEMINI_CLI_VERSION set)
#   3. gemini_authentication  Write .gemini/settings.json for Vertex AI; in CI copy
#      policies/*.toml → .gemini/policies/ (TOML; see
#      https://geminicli.com/docs/reference/policy-engine )
#   4. gemini_install_workspace_skills  Convert skills.yaml → .gemini/skills/*/SKILL.md
#
# Skills resolution (gemini_install_workspace_skills):
#   - If GEMINI_SKILLS_YAML is set: use that path, or decode from base64 (file upload).
#   - Else: look for skills.yaml in the directory of each prompt file
#     (GEMINI_PROMPT_FILE, _2, _3). First found wins. Requires python3 + PyYAML.
#
# Note: Vertex AI non-interactive mode only.
#

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/gemini_environment.sh "$0"
cd "${GEMINI_ARTIFACT_WRITE_ROOT}" || exit 1

RESULT=0

# --- Install / upgrade gemini-cli (optional) ---
function gemini_install() {
    if [ -n "$GEMINI_CLI_VERSION" ]; then
        echo -e "${GREEN}Installing gemini-cli@${GEMINI_CLI_VERSION}${NC}"
        if sudo npm install -g @google/gemini-cli@"${GEMINI_CLI_VERSION}"; then
            echo -e "${GREEN}Installed gemini-cli@${GEMINI_CLI_VERSION}${NC}"
        else
            RESULT=$?
            echo -e "${RED}Installing gemini-cli@${GEMINI_CLI_VERSION} failed ($RESULT)${NC}"
        fi
    fi
}

# --- Remove previous run artifacts so each run starts clean ---
function gemini_cleanup() {
    echo -e "${GREEN}Cleaning up old artifacts...${NC}"
    rm -rf "${GEMINI_ARTIFACT_PATH}" traces
    rm -rf .gemini
    rm -f step*.*
    rm -f gemini-client-error.zip
    local files=()
    for f in $GEMINI_ARTIFACT_FILES_WILDCARD; do
        [[ -e "$f" ]] && files+=("$f")
    done
    # shellcheck disable=SC2086
    rm -f ${GEMINI_OUTPUT_FILE_WILDCARD}
    rm -f "${GEMINI_PROMPT_FILENAME}"
    echo -e "${GREEN}Cleaned up old artifacts.${NC}"
}

# --- Write .gemini/settings.json for Vertex AI (non-interactive) ---
function gemini_authentication() {
    if [[ "${GOOGLE_GENAI_USE_VERTEXAI}" == "True" ]]; then
        echo -e "${GREEN}Setup Vertex AI in non-interactive mode${NC}"
        export GOOGLE_GENAI_USE_VERTEXAI="${GOOGLE_GENAI_USE_VERTEXAI}"
        # Gemini 1st looks to current directory before ${HOME} etc. This benefits Jenkins when running
        # from within a workspace.
        mkdir -p .gemini && echo '{ "security": { "auth": { "selectedType": "vertex-ai" } }, "general": { "previewFeatures": '"${GEMINI_PREVIEW_FEATURES,,}"' } }' > .gemini/settings.json
        # Workspace policies are TOML files under .gemini/policies/ (schema: Gemini CLI policy engine).
        # https://geminicli.com/docs/reference/policy-engine
        #
        # Headless automation: default policy uses ask_user for run_shell_command; without a TTY that is deny.
        # Even with --yolo on GEMINI_COMMAND_LINE (e.g. gemini --model ... --yolo --output-format json), the
        # CLI can still deny shell in non-interactive runs; an explicit allow rule for interactive=false fixes that.
        if [[ -n "${JENKINS_URL:-}" ]] || [[ "${CI:-}" == "true" ]] || [[ -n "${ARGO_WORKFLOW_NAME:-}" ]]; then
            mkdir -p .gemini/policies
            local policy_dir installed
            policy_dir="$(dirname "${BASH_SOURCE[0]}")/policies"
            installed=0
            if [[ ! -d "${policy_dir}" ]]; then
                echo -e "${ORANGE}Missing policy directory ${policy_dir}; skipping workspace policies.${NC}" >&2
            else
                for f in "${policy_dir}"/*.toml; do
                    [[ -f "${f}" ]] || continue
                    cp -f "${f}" .gemini/policies/
                    installed=$((installed + 1))
                done
                if [[ "${installed}" -gt 0 ]]; then
                    echo -e "${GREEN}Installed ${installed} workspace policy file(s) from ${policy_dir}${NC}"
                else
                    echo -e "${ORANGE}No *.toml files in ${policy_dir}; skipping workspace policies.${NC}" >&2
                fi
            fi
        fi
    fi
}

# --- Populate .gemini/skills/ from skills.yaml via gemini_skills_from_yaml.py ---
# Skills file is always named skills.yaml. Source: GEMINI_SKILLS_YAML (path or base64)
# or skills.yaml next to any of the prompt file paths. Requires python3 and PyYAML.
function gemini_install_workspace_skills() {
    [[ ! -d .gemini ]] && return 0
    local script_dir prompt_dir skills_out p yaml_file resolved_skills_yaml
    script_dir="$(dirname "${BASH_SOURCE[0]}")"
    skills_out=".gemini/skills"

    # GEMINI_SKILLS_YAML: either a filesystem path or base64-encoded content (Jenkins file param)
    resolved_skills_yaml=""
    if [[ -n "${GEMINI_SKILLS_YAML:-}" ]]; then
        if [[ -f "${GEMINI_SKILLS_YAML}" ]]; then
            resolved_skills_yaml="${GEMINI_SKILLS_YAML}"
        else
            if echo "${GEMINI_SKILLS_YAML}" | base64 -d > .gemini/skills_uploaded.yaml 2>/dev/null && [[ -s .gemini/skills_uploaded.yaml ]]; then
                resolved_skills_yaml=".gemini/skills_uploaded.yaml"
            fi
        fi
    fi

    # Helper: convert one skills.yaml to .gemini/skills/<name>/SKILL.md (requires PyYAML)
    _gemini_skills_yaml_to_dir() {
        local yaml_path="$1"
        [[ -z "$yaml_path" ]] || [[ ! -f "$yaml_path" ]] && return 1
        if python3 "${script_dir}/gemini_skills_from_yaml.py" "$yaml_path" "$skills_out" 2>/dev/null; then
            echo -e "${GREEN}Skills set up in ${skills_out} (source: ${yaml_path})${NC}"
            return 0
        fi
        echo -e "${ORANGE}Could not convert skills.yaml (missing PyYAML?). pip install pyyaml${NC}" >&2
        return 1
    }

    # Prefer explicit GEMINI_SKILLS_YAML (path or decoded upload)
    if [[ -n "$resolved_skills_yaml" ]] && [[ -f "$resolved_skills_yaml" ]]; then
        _gemini_skills_yaml_to_dir "$resolved_skills_yaml" && return 0
    fi

    # Fallback: skills.yaml in same directory as any prompt file (absolute path when needed)
    for p in "${GEMINI_PROMPT_FILE:-}" "${GEMINI_PROMPT_FILE_2:-}" "${GEMINI_PROMPT_FILE_3:-}"; do
        [[ -z "$p" ]] && continue
        [[ -f "$p" ]] || continue
        prompt_dir="$(dirname "$p")"
        if [[ "$prompt_dir" != /* ]]; then
            [[ -n "${WORKSPACE:-}" ]] && prompt_dir="${WORKSPACE}/${prompt_dir}"
            [[ "$prompt_dir" != /* ]] && prompt_dir="${PWD}/${prompt_dir}"
        fi
        yaml_file="${prompt_dir}/skills.yaml"
        if [[ -f "$yaml_file" ]] && _gemini_skills_yaml_to_dir "$yaml_file"; then return 0; fi
    done
    return 0
}

# --- Main sequence (order matters) ---
gemini_cleanup
gemini_install
gemini_authentication
gemini_install_workspace_skills
exit "$RESULT"
