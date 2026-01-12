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
# Store OpenBSW targets to cloud artifact storage.
#
# Optional variables:
#  - OPENBSW_ARTIFACT_STORAGE_SOLUTION: the persistent storage location for
#        artifacts (GCS_BUCKET default).
#  - OPENBSW_ARTIFACT_ROOT_NAME: the name of the bucket to store artifacts.
#

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/bsw_environment.sh "$0"

# Format STORAGE_PATH as a zero-padded two-digit string (e.g. 7/aaa -> 07/aaa, 7 -> 07)
# shellcheck disable=SC2329
function pad_first_number_if_numeric() {
    local width=${1:-2} s=$2 head rest
    head=${s%%/*}
    [[ $s == */* ]] && rest="/${s#*/}" || rest=
    if [[ $head =~ ^[0-9]+$ ]]; then
        printf "%0*d%s\n" "$width" "$head" "$rest"
    else
        echo "$s" # Return the original.
    fi
}

# Replace spaces in Jenkins Job Name
BUCKET_FOLDER="${JOB_NAME// /_}"
STORAGE_PATH=$(pad_first_number_if_numeric 2 "${OPENBSW_BUILD_NUMBER}")

# Use default or use your own
export STORAGE_BUCKET_DESTINATION=${STORAGE_BUCKET_DESTINATION:-gs://${OPENBSW_ARTIFACT_ROOT_NAME}/${BUCKET_FOLDER}/${STORAGE_PATH}}
export BUCKET_RELATIVE_DESTINATION="${STORAGE_BUCKET_DESTINATION#gs://}"
export STORAGE_CLOUD_URL=${STORAGE_CLOUD_URL:-https://console.cloud.google.com/storage/browser/${BUCKET_RELATIVE_DESTINATION}}

export ARTIFACT_LIST="${OPENBSW_ARTIFACT_LIST[*]}"
export ARTIFACT_SUMMARY="${ORIG_WORKSPACE}/openbsw-${OPENBSW_BUILD_NUMBER}-artifacts.txt"
export ARTIFACT_STORAGE_SOLUTION="${OPENBSW_ARTIFACT_STORAGE_SOLUTION}"
export ARTIFACT_STORAGE_SOLUTION_FUNCTION="${OPENBSW_ARTIFACT_STORAGE_SOLUTION_FUNCTION}"
export WORKSPACE="${ORIG_WORKSPACE}"
"${WORKSPACE}"/workloads/common/storage/storage.sh

export STORAGE_LABELS="${STORAGE_LABELS}"
case "${ARTIFACT_STORAGE_SOLUTION}" in
    GCS_BUCKET)
        export URL_PATH="${STORAGE_BUCKET_DESTINATION}/"
        export KEYVALUE_PAIRS="${STORAGE_LABELS}"
        "${ORIG_WORKSPACE}"/workloads/common/storage/gcs_utilities.sh ADD_OBJECT_METADATA || true
        ;;
    *)
        echo "Utility to add metadata using $ARTIFACT_STORAGE_SOLUTION not available"
        ;;
esac
exit "$?"
