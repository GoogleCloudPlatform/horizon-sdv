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
# Store Gemini outputs to artifact area.
#
# Parameters:
#   Mandatory:
#     GEMINI_ARTIFACT_ROOT_NAME: GCS bucket URL.
#     GEMINI_ARTIFACT_STORAGE_SOLUTION: storage type, eg GCS_BUCKET
#
# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/gemini_environment.sh "$0"

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
STORAGE_PATH=$(pad_first_number_if_numeric 2 "${BUILD_NUMBER}")
# Use default or use your own
export STORAGE_BUCKET_DESTINATION=${STORAGE_BUCKET_DESTINATION:-gs://${GEMINI_ARTIFACT_ROOT_NAME}/${BUCKET_FOLDER}/${STORAGE_PATH}}
export BUCKET_RELATIVE_DESTINATION="${STORAGE_BUCKET_DESTINATION#gs://}"
export STORAGE_CLOUD_URL=${STORAGE_CLOUD_URL:-https://console.cloud.google.com/storage/browser/${BUCKET_RELATIVE_DESTINATION}}

ARTIFACT_LIST=$(printf '%s\n' "${GEMINI_ARTIFACT_LIST[@]}")
export ARTIFACT_LIST
export ARTIFACT_SUMMARY="${GEMINI_ARTIFACT_WRITE_ROOT}/gemini-artifacts.txt"
POST_CLEANUP_STRING=""
export POST_CLEANUP_STRING
POST_CLEANUP_STRING="$(printf "%s\n" "${POST_STORAGE_COMMANDS[@]}")"
export ARTIFACT_STORAGE_SOLUTION="${GEMINI_ARTIFACT_STORAGE_SOLUTION}"
"${WORKSPACE}"/workloads/common/storage/storage.sh
RESULT="$?"

export STORAGE_LABELS="${STORAGE_LABELS}"

if [ -n "${STORAGE_LABELS}" ]; then
    case "${ARTIFACT_STORAGE_SOLUTION}" in
        GCS_BUCKET)
            export URL_PATH="${STORAGE_BUCKET_DESTINATION}/"
            export KEYVALUE_PAIRS="${STORAGE_LABELS}"
            # Labels are best-effort and must not replace the upload result.
            "${WORKSPACE}"/workloads/common/storage/gcs_utilities.sh ADD_OBJECT_METADATA || true
            ;;
        *)
            echo "Utility to add metadata using $ARTIFACT_STORAGE_SOLUTION not available"
            ;;
    esac
else
    echo "STORAGE_LABELS empty, ignoring"
fi
exit "${RESULT}"
