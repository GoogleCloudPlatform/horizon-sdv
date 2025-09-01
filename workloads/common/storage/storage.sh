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
# Store targets to bucket area.
#
#
# If the bucket does not exist, it is created.
#
# Convert string back to a list.
read -r -a ARTIFACT_LIST <<< "${ARTIFACT_LIST}"
IFS=$'\n' read -r -d '' -a POST_CLEANUP_COMMANDS <<< "$POST_CLEANUP_STRING"

# shellcheck disable=SC2317
function gcs_bucket() {
    local -r bucket_name="gs://${ARTIFACT_ROOT_NAME}"
    # Replace spaces in Jenkins Job Name
    BUCKET_FOLDER="${JOB_NAME// /_}"
    local -r destination="${bucket_name}/${BUCKET_FOLDER}/${BUILD_NUMBER}"
    local -r cloud_url="https://console.cloud.google.com/storage/browser/${ARTIFACT_ROOT_NAME}/${BUCKET_FOLDER}/${BUILD_NUMBER}"

    # Remove the old artifacts
    gcloud storage rm -r "${destination}" || true

    # Wait for old artifacts to be removed.
    # Note: belts and braces because removal used to take time and appear to run in background. Now rm finishes cleanly.
    local -i attempts=0
    local -i max_attempts=5
    while gcloud storage ls "${destination}" &> /dev/null; do
        sleep 1.0
        ((attempts++))
        if [ "${attempts}" -gt "${max_attempts}" ]; then
            echo "ERROR: ${destination} still exists after ${max_attempts}s." >&2
            # Brute force just let it continue.
            break
        fi
    done

    rm -f "${ARTIFACT_SUMMARY}"

    # Print download URL links in console log and file..
    echo ""
    echo "Artifacts stored in ${destination}" | tee -a "${ARTIFACT_SUMMARY}"
    echo "Bucket URL: ${cloud_url}" | tee -a "${ARTIFACT_SUMMARY}"
    echo "" | tee -a "${ARTIFACT_SUMMARY}"

    # Copy artifacts to Google Cloud Storage bucket
    echo "Storing artifacts to bucket ${bucket_name}"
    for artifact in "${ARTIFACT_LIST[@]}"; do
        for file in ${artifact}; do
            # Look for wildcard files.
            if [ -e "${file}" ]; then
                [ -d "${file}" ] && copycmd="cp -r" || copycmd="cp"
                # Copy the artifact to the bucket (do not use quotes for cp!)
                # shellcheck disable=SC2086
                gcloud storage ${copycmd} "${file}" "${destination}"/ || true
                echo "Copied ${file} to ${destination}"
                # shellcheck disable=SC2086
                filename=$(echo ${file} | awk -F / '{print $NF}')
                echo "    gcloud storage ${copycmd} ${destination}/${filename} ." | tee -a "${ARTIFACT_SUMMARY}"
            else
                echo "WARNING: File $file ignored!"
            fi
        done
    done
    echo "Artifacts summary:"
    cat "${ARTIFACT_SUMMARY}"
}

#
# A noop function that does nothing.
#
# This function is used when the ARTIFACT_STORAGE_SOLUTION is not
# supported. It prints a message to indicate that the artifacts are not
# being stored to any storage solution.
# shellcheck disable=SC2317
function noop() {
    echo "Noop: skipping artifact stored to ${ARTIFACT_STORAGE_SOLUTION}" >&2
    for artifact in "${ARTIFACT_LIST[@]}"; do
        echo "Skipping copy of ${artifact}" >&2
    done
}

#
# Storage selection.
#
# This case statement sets the ARTIFACT_STORAGE_SOLUTION_FUNCTION
# variable to the appropriate function to call to store artifacts to
# the given storage solution.
case "${ARTIFACT_STORAGE_SOLUTION}" in
    GCS_BUCKET)
        ARTIFACT_STORAGE_SOLUTION_FUNCTION=gcs_bucket
        ;;
    *)
        ARTIFACT_STORAGE_SOLUTION_FUNCTION=noop
        ;;
esac

# Store artifacts to artifact storage.
if [ -n "${ARTIFACT_STORAGE_SOLUTION}" ] && [ -n "${BUILD_NUMBER}" ]; then
    if [ "${#ARTIFACT_LIST[@]}" -gt 0 ]; then
        "${ARTIFACT_STORAGE_SOLUTION_FUNCTION}"
    else
        echo "No artifacts to store to ${ARTIFACT_STORAGE_SOLUTION}, ignored."
    fi
else
    # If not running from Jenkins, just NOOP!
    noop
fi

# Post storage commands.
echo "Post storage commands:"
for command in "${POST_CLEANUP_COMMANDS[@]}"; do
    echo "${command}"
    eval "${command}"
done

# Return result
exit $?
