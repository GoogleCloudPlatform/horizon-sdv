#!/usr/bin/env bash
# Copyright (c) 2025 Accenture, All Rights Reserved.
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
# Retrieve ABFS (GCE) instance details.
# - Lists instances in ZONE filtered by INSTANCE_PREFIX and prints
#   their status.
# - Requires env: ZONE, INSTANCE_PREFIX. Uses gcloud CLI to retrieve
#   details.
#
# Variables:
#   ZONE:      The GCP project zone
#   INSTANCE_PREFIX: The prefix name of the ABFS instance to retrieve details.
#   LIST_BRANCHES:   boolean to indicate whether to dump ABFS branches being
#                    uploaded.
set -euo pipefail

LIST_BRANCHES=${LIST_BRANCHES:-false}
RESULT=0

function artifact() {
    local file="${WORKSPACE}/${1}-status.txt"
    tee >(sed -E 's/\\x1b\\[[0-9;]*[A-Za-z]//g' >> "$file")
}

function abfs_liveness() {
    [[ "${2}" != "RUNNING" ]] && return
    gcloud compute ssh --quiet --zone="${ZONE}" "$1" --tunnel-through-iap --command="" > /dev/null 2>&1 || true

    { echo; echo -e "\033[1;32mABFS Process Liveness $1\033[0m"; } | artifact "${1}"
    PROCESS=$(gcloud compute ssh --quiet --zone="${ZONE}" "${1}" --tunnel-through-iap --command="pgrep -a abfs" || true)
    if [ -n "${PROCESS}" ]; then
        { echo; echo -e "\033[1;32mABFS Alive:\033[0m ${PROCESS}"; echo; } | artifact "${1}"

        # Uploaders have branch
        if [[ "${LIST_BRANCHES}" == "true" ]]; then
            BR_TAG=$(echo "${PROCESS}" | awk '{
                grab=0; out=""
                    for (i=1;i<=NF;i++) {
                        if ($i ~ /^--branch(=|$)/) {
                            grab=1
                            if ($i ~ /^--branch=/) { sub(/^--branch=/,"",$i); out=$i }
                                continue
                         }
                         if (grab && $i ~ /^--project(\b|[-=])/) { print out; exit }
                         if (grab) { out = out (out?" ":"") $i }
                    }
                }')
            { echo; echo -e "\033[1;32mBranches/tags:\033[0m ${BR_TAG}"; echo; } | artifact "${1}"
        fi
    else
        { echo; echo -e "\033[1;31mABFS Not running, consider DESTROY and APPLY\033[0m"; echo; } | artifact "${1}"

        # Dump useful logs
        gcloud compute ssh --quiet --zone="${ZONE}" "${1}" --tunnel-through-iap --command="sudo cat /var/log/cloud-init.log" > "${WORKSPACE}"/"${1}"-cloud-init.log 2>&1 || true
        gcloud compute ssh --quiet --zone="${ZONE}" "${1}" --tunnel-through-iap --command="sudo cat /var/log/cloud-init-output.log" > "${WORKSPACE}"/"${1}"-cloud-init-output.log 2>&1 || true
        gcloud compute ssh --quiet --zone="${ZONE}" "${1}" --tunnel-through-iap --command="sudo journalctl" > "${WORKSPACE}"/"${1}"-journalctl.log 2>&1 || true

        RESULT=1
    fi
}

function main() {
    # List instances using gcloud's native filter and iterate safely.
    # shellcheck disable=SC2207
    instances=($(gcloud compute instances list --zones="${ZONE}" --format="value(name)" --filter="name~'${INSTANCE_PREFIX}'" || true))
    if [ "${#instances[@]}" -gt 0 ]; then
        for instance in "${instances[@]}"; do
            [[ -z "$instance" ]] && continue
            { echo; echo -e "\033[1;32mInstance $instance\033[0m"; } | artifact "$instance"; echo

            gcloud compute instances describe "$instance" --zone="${ZONE}" | artifact "$instance"
            state=$(gcloud compute instances describe "$instance" --zone="${ZONE}" --format="value(status)")
            { echo; echo -e "\033[1;32mInstance $instance state: $state\033[0m"; } | artifact "$instance"
            # shellcheck disable=SC2086
            abfs_liveness "$instance" $state
        done
        echo "RESULT = $RESULT"
        exit "${RESULT}"
    else
        echo -e "\033[1;33mNo instances matching ${INSTANCE_PREFIX}\033[0m"
    fi
}

main
