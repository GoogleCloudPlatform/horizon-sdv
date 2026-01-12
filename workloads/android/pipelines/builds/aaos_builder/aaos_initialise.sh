#!/usr/bin/env bash

# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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
#  - REPO_SYNC_JOBS: the number of parallel repo sync jobs to use.
#  - MAX_REPO_SYNC_JOBS: the maximum number of parallel repo sync jobs
#         supported. (Default: 24).
#  - POST_REPO_INITIALISE_COMMAND: additional vendor commands for repo initialisation.
#  - POST_REPO_COMMAND: additional vendor commands initialisation post repo sync.
#
# For Gerrit review change sets:
#  - GERRIT_SERVER_URL: URL of Gerrit server.
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
# AAOS_LUNCH_TARGET=aosp_tangorpro_car-ap1a-userdebug \
# GERRIT_SERVER_URL=https://dev.horizon-sdv.com/gerrit \
# GERRIT_CHANGE_NUMBER=82 \
# GERRIT_PATCHSET_NUMBER=1 \
# GERRIT_PROJECT=android/platform/packages/services/Car \
# ./workloads/android/pipelines/builds/aaos_builder/aaos_initialise.sh

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/aaos_environment.sh "$0"

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
    # Retry 4 times, on 3rd fail, clean workspace and retry once more.
    MAX_RETRIES=4
    for ((i=1; i<="${MAX_RETRIES}"; i++)); do
        # Initialise repo checkout.
        # shellcheck disable=SC2086
        if ! repo init -u "${AAOS_GERRIT_MANIFEST_URL}" -b "${AAOS_REVISION}" --depth=1 ${LOCAL_MIRROR_REFERENCE}
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

# Fetch and apply all changes based on GERRIT_TOPIC
function fetch_from_topic() {
    echo "Fetching ${GERRIT_TOPIC}"

    local createChangesFile=1
    if [ -f "${GERRIT_CHANGES_FILE}" ]; then
        echo "${GERRIT_CHANGES_FILE} exists, so do not replace."
        createChangesFile=0
    fi

    while IFS=$'\t' read -r project url ref rev; do
        echo "Fetch Project: ${project}"
        echo "Fetch HTTP URL    : $url"
        echo "Fetch HTTP Ref    : $ref"
        echo "Current Revision  | $rev"

        # Derive project path from manifest
        PROJECT_PATH=$(grep "name=\"${project}\"" .repo/manifests/default.xml | sed -r 's/.*path="([^"]+)".*/\1/')
        # Create the command to apply the patchset from topic.
        REPO_CMD="cd ${PROJECT_PATH} && git fetch ${url} ${ref} && git cherry-pick FETCH_HEAD && cd -"
        echo "Running: ${REPO_CMD}"
        if ! eval "${REPO_CMD}"
        then
            echo -e "\033[1;31mERROR: git fetch failed, exit!\033[0m"
            # Clean up so pv is not left in limbo (and thus removed)
            git cherry-pick --abort || true  && git reset --hard HEAD || true
            exit 1
        else
            if (( createChangesFile == 1 )); then
                echo "$rev" | tee -a  "${GERRIT_CHANGES_FILE}"
            fi
        fi
    done < <(curl -sS -u "${GERRIT_USERNAME}:${GERRIT_PASSWORD}" \
        "${GERRIT_SERVER_URL}/a/changes/?q=topic:${GERRIT_TOPIC}+status:open&o=CURRENT_REVISION" \
        | sed '1d' | jq -r ' .[] |
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
    if [[ -n "${GERRIT_TOPIC}" ]]; then
        fetch_from_topic
    elif [[ -n "${GERRIT_PROJECT}" && -n "${GERRIT_CHANGE_NUMBER}" && -n "${GERRIT_PATCHSET_NUMBER}" ]]; then
        if [[ "${ABFS_BUILDER}" == "false" ]]; then
            # Use standard git fetch to retrieve the change.
            # Find the project name from the manifest.
            PROJECT_PATH=$(repo list -p "${GERRIT_PROJECT}")
        else
            # Find the path from manifest
            mkdir -p "${HOME}"/manifest
            cd "${HOME}"/manifest || exit

            # FIXME: fix branch (demo only)
            if [[ "${AAOS_GERRIT_MANIFEST_URL}" =~ "horizon" ]]; then
                if [[ ! "${AAOS_REVISION}" =~ "horizon" ]]; then
                    AAOS_REVISION=horizon/"${AAOS_REVISION}"
                fi
            fi

            # FIXME: will use clone in future but for now this is just convenience for commonality.
            if ! repo init -u "${AAOS_GERRIT_MANIFEST_URL}" -b "${AAOS_REVISION}" --depth=1
            then
                echo -e "\033[1;31mERROR: repo init failed, exit!\033[0m"
                exit 1
            fi

            PROJECT_PATH=$(grep "name=\"${GERRIT_PROJECT}\"" .repo/manifests/default.xml | sed -r 's/.*path="([^"]+)".*/\1/')
            rm -rf  "${HOME}"/manifest
            cd - || exit
        fi

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
        # shellcheck disable=SC2164
        REPO_CMD="cd ${PROJECT_PATH} && git fetch ${PROJECT_URL} ${FETCHED_REFS} && git cherry-pick FETCH_HEAD && cd -"

        echo "Running: ${REPO_CMD}"
        if ! eval "${REPO_CMD}"
        then
            echo -e "\033[1;31mERROR: git fetch failed, exit!\033[0m"
            exit 1
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
