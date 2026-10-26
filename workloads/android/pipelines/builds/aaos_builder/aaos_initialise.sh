#!/usr/bin/env bash

# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
# Initialise the AAOSP repositories.
#
# This script does the following:
#
#  1. Initialises the repository checkout using the given manifest.
#  2. Supports post initialise and sync commands to setup repo.
#  3. Downloads the given changeset if the build is from an open review.
#
# The following variables must be set before running this script:
#
#  - AAOS_GERRIT_MANIFEST_URL: the URL of the AAOS manifest.
#  - AAOS_REVISION: the branch or version of the AAOS manifest.
#  - AAOS_LUNCH_TARGET: the target device.
#
# Optional variables:
#  - AAOS_CLEAN: whether to clean before building. Only CLEAN_BUILD or
#        NO_CLEAN are applicable.
#  - AAOS_MANIFEST_NAME: the name of default manifest to use.
#  - REPO_SYNC_JOBS: the number of parallel repo sync jobs to use.
#  - MAX_REPO_SYNC_JOBS: the maximum number of parallel repo sync jobs
#         supported. (Default: 24).
#  - POST_REPO_INITIALISE_COMMAND: additional vendor commands for repo initialisation.
#  - POST_REPO_COMMAND: additional vendor commands initialisation post repo sync.
#
# For Gerrit review change sets:
#  - GERRIT_SERVER_URL: URL of Gerrit server.
#  - GERRIT_AUTH_MODE: optional — auto | username_password | gitcookies | none (REST topic query only).
#        auto: use username+password if both set; else cookie file if present; else no auth.
#        username_password: require GERRIT_USERNAME and GERRIT_PASSWORD.
#        gitcookies: require git config http.cookiefile or ~/.gitcookies.
#        none: unauthenticated curl (public Gerrit API only).
#  - GERRIT_USERNAME / GERRIT_PASSWORD: used when mode is username_password (or auto and both set).
#  - GERRIT_PROJECT: the name of the project to download.
#  - GERRIT_CHANGE_NUMBER: the change number of the changeset to download.
#  - GERRIT_PATCHSET_NUMBER: the patchset number of the changeset to download.
#  - GERRIT_TOPIC: the topic identifying the changes to fetch.
#
# Example usage:
# AAOS_GERRIT_MANIFEST_URL=https://dev.horizon-sdv.scpmtk.com/android/platform/manifest \
# AAOS_REVISION=horizon/android-14.0.0_r30 \
# AAOS_LUNCH_TARGET=aosp_cf_x86_64_auto-ap1a-userdebug \
# ./workloads/android/pipelines/builds/aaos_builder/aaos_initialise.sh
#
# AAOS_GERRIT_MANIFEST_URL=https://dev.horizon-sdv.scpmtk.com/android/platform/manifest \
# AAOS_REVISION=horizon/android-14.0.0_r30 \
# AAOS_MANIFEST_NAME=default.xml \
# AAOS_LUNCH_TARGET=aosp_tangorpro_car-ap1a-userdebug \
# GERRIT_SERVER_URL=https://dev.horizon-sdv.com/gerrit \
# GERRIT_CHANGE_NUMBER=82 \
# GERRIT_PATCHSET_NUMBER=1 \
# GERRIT_PROJECT=android/platform/packages/services/Car \
# ./workloads/android/pipelines/builds/aaos_builder/aaos_initialise.sh

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/aaos_environment.sh "$0"
declare PROJECT_PATH=""

# Tune Git HTTPS for the manifest host (e.g. partner *.googlesource.com) before repo init/sync.
# Reduces HTTP/2 / TLS edge cases for large fetches.
function configure_git_googlesource_https_host() {
    if [[ -z "${AAOS_GERRIT_MANIFEST_URL:-}" ]]; then
        return 0
    fi
    local mhost
    mhost=$(echo "${AAOS_GERRIT_MANIFEST_URL}" | sed -e 's|^https\?://||' -e 's|/.*||')
    if [[ -z "${mhost}" ]] || [[ "${mhost}" != *googlesource.com* ]]; then
        return 0
    fi
    git config --global "http.https://${mhost}/.version" "HTTP/1.1"
    git config --global "http.https://${mhost}/.extraHeader" "Connection: keep-alive"
    git config --global "http.https://${mhost}/.sslVersion" "tlsv1.2"
    echo "Git: HTTP/1.1, keep-alive, TLS 1.2 for manifest host ${mhost}"
}

# Initialise the repository
function initialise_repo() {
    local LOCAL_MIRROR_REFERENCE=""
    if [[ "${USE_LOCAL_AOSP_MIRROR}" == "true" ]]; then
        if [[ -d "${MIRROR_DIR_FULL_PATH}/.repo" ]]; then
            LOCAL_MIRROR_REFERENCE="--reference ${MIRROR_DIR_FULL_PATH}"
            echo "Using AOSP mirror: '${MIRROR_DIR_FULL_PATH}'."
        else
            echo -e "\033[1;31mERROR: AOSP mirror not found at path: '${MIRROR_DIR_FULL_PATH}', ensure AOSP Mirror has been setup..\033[0m"
            exit 1
        fi
    fi
    configure_git_googlesource_https_host
    # Retry 4 times, on 3rd fail, clean workspace and retry once more.
    MAX_RETRIES=4
    for ((i=1; i<="${MAX_RETRIES}"; i++)); do
        # Initialise repo checkout.
        local manifest_name="default.xml"
        local init_cmd="repo init -u ${AAOS_GERRIT_MANIFEST_URL} -b ${AAOS_REVISION}"
        if [[ -n "${AAOS_MANIFEST_NAME}" ]]; then
            manifest_name="${AAOS_MANIFEST_NAME}"
        fi
        init_cmd+=" -m ${manifest_name} --depth=1 ${LOCAL_MIRROR_REFERENCE}"

        echo "${init_cmd}"
        if ! eval "${init_cmd}"
        then
            echo -e "\033[1;31mERROR: repo init failed, exit!\033[0m"
            exit 1
        fi

        for command in "${POST_REPO_INITIALISE_COMMANDS_LIST[@]}"; do
            echo "${command}"
            if ! eval "${command}"
            then
                echo -e "\033[1;31mERROR: command ${command} failed, exit!\033[0m"
                exit 1
            fi
        done

        local repo_sync_jobs="${REPO_SYNC_JOBS_ARG}"
        # This will automatically clean any previous staged/fetched/downloaded changes.
        if ! repo sync --no-tags --optimized-fetch --prune --retry-fetches=3 --auto-gc --no-clone-bundle --fail-fast --force-sync "${repo_sync_jobs}"
        then
            # reduce parallel jobs to a reasonable level because mirror failures with high job
            # value result in Google remote repo failures (HTTP 429 errors - rate limits).
            repo_sync_jobs="-j3"
            echo "WARNING: repo sync failed, sleep 60s and retrying with $repo_sync_jobs..."
            sleep 60
            if [ "$i" -eq 3 ]; then
                echo "WARNING: clean workspace and retry."
                recreate_workspace
            fi
            if [ "$i" -eq 4 ]; then
                echo -e "\033[1;31mERROR: repo sync retry failed, giving up.\033[0m"
                exit 1
            fi
        else
            # Remove any unstaged changes others may have left in place on PV.
            if ! repo forall -c 'git checkout -- .; git clean -fdx'
            then
                echo -e "\033[1;31mERROR: git clean failed, giving up.\033[0m"
                exit 1
            fi
            break
        fi
    done

    echo "SUCCESS: repo sync complete."
}

# generate list of manifest files based on top level manifest
find_all_manifest_files()
{
    local manifest="${1}"
    local base_dir="${2}"
    local list_file="${3}"
    local full_path="${base_dir}/${manifest}"

    # note processed manifest files
    if [[ ! -f "${list_file}" ]]; then
        touch "${list_file}"
    fi

    # return if current already processed
    if grep -qx "${full_path}" "${list_file}"; then
        return
    fi

    # add manifest to the list of processed
    echo "${full_path}" >> "${list_file}"

    # collect
    echo "${full_path}"

    # recurse through next level includes
    grep -oP '(?<=<include name=")[^"]+' "$full_path" | while read -r include; do
        find_all_manifest_files "$include" "$base_dir" "$list_file"
    done
}

get_all_manifest_files()
{
    local top_manifest="${1}"
    local tmpfile="/tmp/processed.$$"
    local list

    list=$(find_all_manifest_files "${top_manifest}" ".repo/manifests" "${tmpfile}")
    if [ -f "${tmpfile}" ]; then
        rm "${tmpfile}"
    fi

    echo "${list}"
}

find_prj_path_in_manifest()
{
    local manifest="${1}"
    local project="${2}"
    local manifest_list=""
    local project_path=""

    manifest_list=$(get_all_manifest_files "${manifest}")
    if [[ -z "${manifest_list}" ]]; then
        echo ""
        exit 1
    fi

    for i in ${manifest_list}; do
        project_path=$(grep "name=\"${project}\"" "$i" | sed -r 's/.*path="([^"]+)".*/\1/')
        if [[ -n "${project_path}" ]]; then
            break
        fi
    done

    echo "${project_path}"
}

# Determine the true project path
function set_repo_path() {
    local project="${1}"
    local init_cmd="repo init -u ${AAOS_GERRIT_MANIFEST_URL} -b ${AAOS_REVISION}"
    local manifest_name="default.xml"
    if [[ -n "${AAOS_MANIFEST_NAME}" ]]; then
        manifest_name="${AAOS_MANIFEST_NAME}"
    fi
    init_cmd+=" -m ${manifest_name} --depth=1"

    if [[ "${ABFS_BUILDER}" == "false" ]]; then
        # Use standard git fetch to retrieve the change.
        # Find the project name from the manifest.
        if [[ -n "${GERRIT_TOPIC}" ]]; then
            PROJECT_PATH=$(repo list -p "${project}")
        else
            PROJECT_PATH=$(find_prj_path_in_manifest "${manifest_name}" "${project}")
        fi
    else

        # Make this common to topic.
        # Find the path from manifest
        mkdir -p "${HOME}"/manifest
        cd "${HOME}"/manifest || exit

        # FIXME: will use clone in future but for now this is just convenience for commonality.
        if ! eval "${init_cmd}"
        then
            echo -e "\033[1;31mERROR: repo init failed, exit!\033[0m"
            exit 1
        fi

        PROJECT_PATH=$(find_prj_path_in_manifest "${manifest_name}" "${project}")
        rm -rf  "${HOME}"/manifest
        cd - || exit
    fi

    # If path was not found in manifest, check if it is not the manifest repository itself
    if [[ -z "${PROJECT_PATH}" ]]; then
        local origin=$(cd .repo/manifests; git remote -v |grep "${project}"|grep fetch)
        if [[ -n "${origin}" ]]; then
            PROJECT_PATH=".repo/manifests"
        fi
    fi

    echo "Repo path: $PROJECT_PATH"
}

# Configure Git for more reliable fetches from Gerrit (avoids HTTP/2 INTERNAL_ERROR,
# early EOF, GnuTLS recv error, unpack-objects failed). Call once before any Gerrit fetch.
function configure_git_gerrit_fetch() {
    # Larger buffer for big fetches; reduces "RPC failed" / "body still expected" errors.
    git config --global http.postBuffer 524288000
    if [[ -n "${GERRIT_SERVER_URL:-}" ]]; then
        local host
        host=$(echo "${GERRIT_SERVER_URL}" | sed -e 's|^https\?://||' -e 's|/.*||')
        if [[ -n "${host}" ]]; then
            git config --global "http.https://${host}/.extraHeader" "Connection: keep-alive"
            # Force HTTP/1.1 for this host to avoid HTTP/2 stream errors (INTERNAL_ERROR, stream not closed cleanly).
            git config --global "http.https://${host}/.version" "HTTP/1.1"
            # Force TLS 1.2 for this host to avoid GnuTLS "Error decoding the received TLS packet" (curl 56).
            git config --global "http.https://${host}/.sslVersion" "tlsv1.2"
            echo "Git: configured postBuffer, keep-alive, HTTP/1.1 and TLS 1.2 for ${host}"
        fi
    fi
}

# Fetch ref from URL and cherry-pick with retries (transient HTTP/2, GnuTLS, network errors).
# Uses a timeout so a stuck fetch fails and retries instead of hanging. Set GERRIT_FETCH_TIMEOUT_SEC to override (default 300).
# Returns 0 on success, 1 after all retries failed.
function fetch_and_cherry_pick_with_retry() {
    local project_path="$1"
    local fetch_url="$2"
    local ref="$3"
    local max_attempts=5
    local retry_delay_sec=20
    local fetch_timeout_sec="${GERRIT_FETCH_TIMEOUT_SEC:-300}"
    local attempt=1
    local saved_pwd
    saved_pwd=$(pwd)

    while (( attempt <= max_attempts )); do
        echo "Attempt ${attempt}/${max_attempts}: git fetch ${fetch_url} ${ref} (timeout ${fetch_timeout_sec}s) && git cherry-pick FETCH_HEAD"
        local fetch_exit=0
        ( cd "${project_path}" && timeout "${fetch_timeout_sec}" git fetch "${fetch_url}" "${ref}" && git cherry-pick FETCH_HEAD ) || fetch_exit=$?
        if (( fetch_exit == 0 )); then
            cd "${saved_pwd}" || true
            return 0
        fi
        if (( fetch_exit == 124 )); then
            echo "WARNING: git fetch timed out after ${fetch_timeout_sec}s (attempt ${attempt}/${max_attempts})."
        else
            echo "WARNING: fetch or cherry-pick failed with exit ${fetch_exit} (attempt ${attempt}/${max_attempts})."
        fi
        cd "${saved_pwd}" || true
        git -C "${project_path}" cherry-pick --abort 2>/dev/null || true
        git -C "${project_path}" reset --hard HEAD 2>/dev/null || true
        if (( attempt < max_attempts )); then
            echo "Retrying in ${retry_delay_sec} seconds..."
            sleep "${retry_delay_sec}"
        fi
        (( attempt++ )) || true
    done
    return 1
}

# Path to Git cookie jar if it exists (http.cookiefile or ~/.gitcookies); else exit 1.
function gerrit_cookie_jar_path() {
    local cookiejar
    cookiejar=$(git config --global --get http.cookiefile 2>/dev/null || true)
    if [[ -n "${cookiejar}" ]] && [[ "${cookiejar}" =~ ^~ ]]; then
        cookiejar="${cookiejar/#\~/${HOME}}"
    fi
    if [[ -n "${cookiejar}" ]] && [[ -f "${cookiejar}" ]]; then
        echo "${cookiejar}"
        return 0
    fi
    cookiejar="${HOME}/.gitcookies"
    if [[ -f "${cookiejar}" ]]; then
        echo "${cookiejar}"
        return 0
    fi
    return 1
}

# Resolve GERRIT_AUTH_MODE: auto | username_password | gitcookies | none
function resolve_gerrit_auth_mode() {
    local raw
    raw=$(echo "${GERRIT_AUTH_MODE:-}" | xargs | tr '[:upper:]' '[:lower:]')
    case "${raw}" in
        username_password|userpass|basic)
            echo "username_password"
            ;;
        gitcookies|cookie|cookies)
            echo "gitcookies"
            ;;
        none|anonymous|public)
            echo "none"
            ;;
        ""|auto)
            echo "auto"
            ;;
        *)
            echo -e "\033[1;31mERROR: GERRIT_AUTH_MODE must be auto, username_password, gitcookies, or none (got: ${GERRIT_AUTH_MODE})\033[0m" >&2
            return 1
            ;;
    esac
}

# decode user and pass from .git-credentials
function urldecode()
{
    local input="${1}"
    local output=""

    # replace + with space
    input="${input//+/ }"

    # decode %xx sequences
    output=$(printf '%b' "${input//%/\\x}")
    echo "${output}"
}

# encode user and pass into .git-credentials
function urlencode()
{
    local input="${1}"
    local len="${#input}"

    local output=""
    local curr_char

    for (( i=0; i<len; i++ )); do
    curr_char="${input:i:1}"
    case "${curr_char}" in
        [a-zA-Z0-9.~_-])
            output+="${curr_char}"
            ;;
        *)
            printf -v hex_value '%%%02x' "'${curr_char}"
            output+="${hex_value}"
            ;;
        esac
    done

    echo "${output}"
}

# encode user pass proto and url into token
function create_credentials()
{
    local user
    local pass
    local proto="${3}"
    local url="${4}"
    user=$(urlencode "${1}")
    pass=$(urlencode "${2}")

    local output=""
    output+="${proto}://${user}:${pass}@${url}"
    echo "${output}"
}

# store token in .git-credentials
function store_credentials()
{
    local file
    local credentials
    file="${HOME}/.git-credentials"
    credentials=$(create_credentials "${GERRIT_HTTP_USERNAME}" "${GERRIT_PASSWORD}" \
                  "https" "${GERRIT_SERVER_URL}")

    if [[ ! -f "${file}" ]]; then
        # create file
        echo "${credentials}" > "${file}"
        chmod 0600 "${file}"
    else
        # or update if only entry for our domain missing
        local token
        token=$(grep "${GERRIT_SERVER_URL}" "${file}"|head -n1)
        if [ -z "${token}" ]; then
            echo "${credentials}" >> "${file}"
        fi
    fi
    git config --global credential.helper store
}

# get part of token
function get_string_from_token()
{
    local input="${1}"
    local output

    local d1="${2}"
    local f1="${3}"
    local d2="${4}"
    local f2="${5}"

    output=$(echo "${input}" | \
             cut -d "${d1}" -f "${f1}"|cut -d "${d2}" -f "${f2}")
    echo "${output}"
}

# get user from token
function get_user_from_token()
{
    local token="${1}"
    local userurl
    local user

    # token format: http(s)://userurl:passurl@domain
    userurl=$(get_string_from_token "${token}" ':' '2' '/' '3')
    user=$(urldecode "${userurl}")
    echo "${user}"
}

# get pass from token
function get_pass_from_token()
{
    local token="${1}"
    local passurl
    local pass

    passurl=$(get_string_from_token "${token}" ':' '3' '@' '1')
    pass=$(urldecode "${passurl}")
    echo "${pass}"
}

# GET a Gerrit REST URL using GERRIT_AUTH_MODE (see script header).
function curl_gerrit_rest_get() {
    local url="${1}"
    local mode

    mode=$(resolve_gerrit_auth_mode) || return 1
    case "${mode}" in
        auto)
            local cmd
            # try userpass
            echo "trying userpass" >&2
            if [[ -n "${GERRIT_HTTP_USERNAME}" ]] && [[ -n "${GERRIT_PASSWORD}" ]]; then
                if curl -fsS -u "${GERRIT_HTTP_USERNAME}:${GERRIT_PASSWORD}" "${url}"; then
                    # echo "userpass success" >&2;
                    # if credentials not stored, subsequent opearations on fetching changes by topic will fail
                    # or ask for password
                    store_credentials
                    return
                fi
            else
                echo "user and/or pass missing" >&2
            fi

            # then try .git-credentials
            echo "trying .git-credentials" >&2
            if [[ -f "${HOME}/.git-credentials" ]]; then
                local token
                local user
                local pass
                token=$(grep "${GERRIT_SERVER_URL}" "${HOME}/.git-credentials"|head -n1)
                user=$(get_user_from_token "${token}")
                pass=$(get_pass_from_token "${token}")
                if curl -fsS -u "${user}:${pass}" "${url}"; then
                    # echo ".git-credentials success" >&2;
                    return
                fi
            else
                echo "${HOME}/.git-credentials missing" >&2
            fi

            # then try .gitcookies
            echo "trying .gitcookies" >&2
            local cookiejar
            if cookiejar=$(gerrit_cookie_jar_path); then
                if curl -fsS -b "${cookiejar}" "${url}"; then
                    # echo ".gitcookies success" >&2
                    return
                fi
            else
                echo "neither .gitcookies nor http.cookiefile present" >&2
            fi

            # then try none
            echo "trying none" >&2
            curl -fsS "${url}"
            ;;
        username_password)
            if [[ -z "${GERRIT_HTTP_USERNAME}" || -z "${GERRIT_PASSWORD}" ]]; then
                echo -e "\033[1;31mERROR: GERRIT_AUTH_MODE=username_password requires GERRIT_HTTP_USERNAME (or GERRIT_USERNAME) and GERRIT_PASSWORD\033[0m" >&2
                return 1
            fi
            curl -fsS -u "${GERRIT_HTTP_USERNAME}:${GERRIT_PASSWORD}" "$url"
            ;;
        gitcookies)
            local cookiejar
            if ! cookiejar=$(gerrit_cookie_jar_path); then
                echo -e "\033[1;31mERROR: GERRIT_AUTH_MODE=gitcookies requires git config http.cookiefile or ~/.gitcookies\033[0m" >&2
                return 1
            fi
            curl -fsS -b "${cookiejar}" "$url"
            ;;
        none)
            curl -fsS "$url"
            ;;
    esac
}

# Fetch and apply all changes based on GERRIT_TOPIC
function fetch_from_topic() {
    echo "Fetching topic: ${GERRIT_TOPIC}"

    local createChangesFile=1
    if [ -f "${GERRIT_CHANGES_FILE}" ]; then
        echo "${GERRIT_CHANGES_FILE} exists, so do not replace."
        createChangesFile=0
    fi

    local topic_json
    topic_json=$(curl_gerrit_rest_get "${GERRIT_SERVER_URL}/a/changes/?q=topic:${GERRIT_TOPIC}+status:open&o=CURRENT_REVISION") || exit 1

    while IFS=$'\t' read -r project url ref rev; do
        echo "Fetch Project: ${project}"
        echo "Fetch HTTP URL    : $url"
        echo "Fetch HTTP Ref    : $ref"
        echo "Current Revision  | $rev"

        # Derive project path from manifest
        set_repo_path "${project}"

        if [[ "${ABFS_BUILDER}" != "false" ]]; then
            # ABFS: fetch and cherry-pick with retries (HTTP/2, GnuTLS, network).
            if ! fetch_and_cherry_pick_with_retry "${PROJECT_PATH}" "${url}" "${ref}"
            then
                echo -e "\033[1;31mERROR: git fetch failed after retries, exit!\033[0m"
                git -C "${PROJECT_PATH}" cherry-pick --abort 2>/dev/null || true
                git -C "${PROJECT_PATH}" reset --hard HEAD 2>/dev/null || true
                exit 1
            fi
        else
            REPO_CMD="cd ${PROJECT_PATH} && git fetch ${url} ${ref} && git cherry-pick FETCH_HEAD && cd -"
            echo "Running: ${REPO_CMD}"
            if ! eval "${REPO_CMD}"
            then
                echo -e "\033[1;31mERROR: git fetch failed, exit!\033[0m"
                git -C "${PROJECT_PATH}" cherry-pick --abort 2>/dev/null || true
                git -C "${PROJECT_PATH}" reset --hard HEAD 2>/dev/null || true
                exit 1
            fi
        fi
        if (( createChangesFile == 1 )); then
            echo "$rev" | tee -a  "${GERRIT_CHANGES_FILE}"
        fi
    done < <(echo "$topic_json" | sed '1d' | jq -r ' .[] |
            .project as $project |
            (if .current_revision != null
             then .revisions[.current_revision].fetch.http?
             else (.revisions | to_entries | first.value.fetch.http?)
             end) as $http |
                 [$project, ($http.url // ""), ($http.ref // ""), .current_revision] | @tsv')
    echo "Changes ->"
    cat "${GERRIT_CHANGES_FILE}"
    echo "<-"
}

# Pull in change set from Gerrit.
function fetch_patchset() {
    # ABFS only: configure Git for Gerrit (postBuffer, HTTP/1.1, TLS 1.2, retry on transient errors).
    if [[ "${ABFS_BUILDER}" != "false" ]]; then
        configure_git_gerrit_fetch
    fi

    if [[ -n "${GERRIT_TOPIC}" ]]; then
        fetch_from_topic
    elif [[ -n "${GERRIT_PROJECT}" && -n "${GERRIT_CHANGE_NUMBER}" && -n "${GERRIT_PATCHSET_NUMBER}" ]]; then

        echo "Fetching patchset: project: ${GERRIT_PROJECT}, change: ${GERRIT_CHANGE_NUMBER}, patchset: ${GERRIT_PATCHSET_NUMBER}"
        set_repo_path "${GERRIT_PROJECT}"

        # Derive the Gerrit URL from the manifest URL.
        #   Horizon SDV uses path based URL whereas Google Android does not.
        PROJECT_URL=$(echo "${AAOS_GERRIT_MANIFEST_URL}" | cut -d'/' -f1-3)/"${GERRIT_PROJECT}"
        if ! curl -s -f -o /dev/null "${PROJECT_URL}"; then
            # Use default.
            PROJECT_URL="${GERRIT_SERVER_URL}/${GERRIT_PROJECT}"
        fi

        # Extract the last two digits of the change number.
        if (( ${#GERRIT_CHANGE_NUMBER} > 2 )); then
            LAST_TWO_DIGITS=${GERRIT_CHANGE_NUMBER: -2}
        else
            if (( ${#GERRIT_CHANGE_NUMBER} == 1 )); then
                LAST_TWO_DIGITS=0${GERRIT_CHANGE_NUMBER}
            else
                LAST_TWO_DIGITS=${GERRIT_CHANGE_NUMBER}
            fi
        fi

        FETCHED_REFS="refs/changes/${LAST_TWO_DIGITS}"/"${GERRIT_CHANGE_NUMBER}"/"${GERRIT_PATCHSET_NUMBER}"
        if [[ "${ABFS_BUILDER}" != "false" ]]; then
            echo "Running: cd ${PROJECT_PATH} && git fetch ${PROJECT_URL} ${FETCHED_REFS} && git cherry-pick FETCH_HEAD"
            if ! fetch_and_cherry_pick_with_retry "${PROJECT_PATH}" "${PROJECT_URL}" "${FETCHED_REFS}"
            then
                echo -e "\033[1;31mERROR: git fetch failed after retries, exit!\033[0m"
                exit 1
            fi
        else
            REPO_CMD="cd ${PROJECT_PATH} && git fetch ${PROJECT_URL} ${FETCHED_REFS} && git cherry-pick FETCH_HEAD && cd -"
            echo "Running: ${REPO_CMD}"
            if ! eval "${REPO_CMD}"
            then
                echo -e "\033[1;31mERROR: git fetch failed, exit!\033[0m"
                exit 1
            fi
        fi

        if [ -f "${GERRIT_CHANGES_FILE}" ]; then
            echo "${GERRIT_CHANGES_FILE} exists, so do not replace."
        else
            echo "$GERRIT_CHANGE_ID" | tee -a  "${GERRIT_CHANGES_FILE}"
        fi
    fi
}

# ABFS: requires systemd and thus systemctl, simply stub.
function fake_systemd() {
    echo "fake_systemd"
    cat >systemctl <<EOL
#!/bin/bash
echo \$0 \$@
exit 0
EOL

    sudo mv systemctl /usr/bin
    sudo chmod +x /usr/bin/systemctl
}

# ABFS: install aptitude binaries for abfs
function abfs_install() {
    echo "abfs_install."
    sudo apt update -y
    declare -r abfs_artifacts="${ORIG_WORKSPACE}"/abfs_repository_list.txt
    rm -f "${abfs_artifacts}"

    {
        echo "Build Parameters:"
        echo "Kernel Version: $(uname -r)"
        echo "ABFS_VERSION: ${ABFS_VERSION}"
        echo "ABFS_CASFS_VERSION: ${ABFS_CASFS_VERSION}"
    } >> "${abfs_artifacts}"

    ABFS_CLIENT_FILE="abfs-client_${ABFS_VERSION}"
    ABFS_CASFS_FILE="casfs-kmod-$(uname -r)_${ABFS_CASFS_VERSION}"

    ABFS_FILES=(
        "${ABFS_CLIENT_FILE}"
        "${ABFS_CASFS_FILE}"
      )

    gcloud artifacts files list --project=abfs-binaries --location=us --repository="${ABFS_REPOSITORY}" >> "${abfs_artifacts}" 2>&1
    grep -e "pool/${ABFS_CLIENT_FILE}" -e "pool/${ABFS_CASFS_FILE}" "${abfs_artifacts}" | awk '{print $1}' | while read -r a; do gcloud artifacts files download --project=abfs-binaries --location=us --repository="${ABFS_REPOSITORY}" --destination=. "${a}"; done
    for f in "${ABFS_FILES[@]}"; do
        if ! ls ./*"${f}"* 1> /dev/null 2>&1; then
            echo -e "\033[1;31mERROR: $f does not exist. Review ${abfs_artifacts} for supported versions.\033[0m"
            exit 1
        fi
    done

    CMD="find . -maxdepth 1 -type f -name \"pool*\" -exec sudo apt install \"./{}\" \\;"
    echo "Command: ${CMD}"
    eval "${CMD}"
    sudo depmod -a
    sudo modprobe casfs
    CMD="find . -maxdepth 1 -type f -name \"pool*\" -exec sudo rm -rf \"./{}\" \\;"
    echo "Command: ${CMD}"
    eval "${CMD}"
}

# ABFS: initialise
function abfs_initialise() {
    echo "abfs_initialise."
    # shellcheck disable=SC2086
    if ! abfs ${ABFS_CMD_FLAGS} init -c
    then
        echo -e "\033[1;31mERROR: failed on abfs init\033[0m"
        exit 1
    fi
    # shellcheck disable=SC2086
    abfs ${ABFS_CMD_FLAGS} --remote-servers abfs-server:50051 --tunnel-ports 0 --manifest-server ${UPLOADER_MANIFEST_SERVER} config -w
    # shellcheck disable=SC2086
    abfs ${ABFS_CMD_FLAGS} cacheman run -l /home/builder/.abfs/logs/cacheman &
    sleep 5

    # shellcheck disable=SC2086
    if ! abfs ${ABFS_CMD_FLAGS} mount -b "${AAOS_REVISION}" "${WORKSPACE}"
    then
        echo -e "\033[1;31mERROR: failed on abfs mount\033[0m"
        exit 1
    fi
    cd "${WORKSPACE}" || exit 1
    # shellcheck disable=SC2086
    if ! abfs ${ABFS_CMD_FLAGS} setup .
    then
        echo -e "\033[1;31mERROR: failed on abfs setup\033[0m"
        exit 1
    fi
}

# Additional commands to run after repo sync or git clone.
function post_repo_commands() {
    for command in "${POST_REPO_COMMAND_LIST[@]}"; do
        echo "${command}"
        if ! eval "${command}"
        then
            echo -e "\033[1;31mERROR: command ${command} failed, exit!"
            exit 1
        fi
    done
}

fake_systemd
if [[ "${ABFS_BUILDER}" == "false" ]]; then
    initialise_repo
else
    abfs_install
    abfs_initialise
fi
fetch_patchset
post_repo_commands
exit 0
