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
# Store targets to bucket area.
#
#
# If the bucket does not exist, it is created.
#
# Enable nullglob to handle non-matching globs gracefully
shopt -s nullglob

# Convert string back to a list.
readarray -t ARTIFACT_LIST <<< "$ARTIFACT_LIST"
IFS=$'\n' read -r -d '' -a POST_CLEANUP_COMMANDS <<< "$POST_CLEANUP_STRING"

# shellcheck disable=SC2329
function gcs_bucket() {
    local failed=0
    # A new destination has nothing to remove, so a not-found here is expected.
    # A copy that fails is what fails the store.
    gcloud storage rm -r "${STORAGE_BUCKET_DESTINATION}" || true

    # Wait for old artifacts to be removed.
    # Note: belts and braces because removal used to take time and appear to run in background. Now rm finishes cleanly.
    local -i attempts=0
    local -i max_attempts=5
    while gcloud storage ls "${STORAGE_BUCKET_DESTINATION}" &> /dev/null; do
        sleep 1.0
        ((attempts++))
        if [ "${attempts}" -gt "${max_attempts}" ]; then
            echo "ERROR: ${STORAGE_BUCKET_DESTINATION} still exists after ${max_attempts}s." >&2
            # Brute force just let it continue.
            break
        fi
    done

    rm -f "${ARTIFACT_SUMMARY}"

    # Print download URL links in console log and file..
    echo ""
    echo "Artifacts stored in ${STORAGE_BUCKET_DESTINATION}" | tee -a "${ARTIFACT_SUMMARY}"
    echo "Bucket URL: ${STORAGE_CLOUD_URL}" | tee -a "${ARTIFACT_SUMMARY}"
    echo "" | tee -a "${ARTIFACT_SUMMARY}"

    # Copy artifacts to Google Cloud Storage bucket
    echo "Storing artifacts to bucket."
    for artifact in "${ARTIFACT_LIST[@]}"; do
        # Check for wildcards (* or ?)
        if [[ "$artifact" == *[*?]* ]]; then
            # Expand wildcards using find (handles spaces safely)
            base=$(dirname "$artifact")
            pattern=$(basename "$artifact")
            files=()
            while IFS= read -r -d '' f; do
                files+=("$f")
            done < <(find "$base" -maxdepth 1 -name "$pattern" -print0 2>/dev/null)
            if [ ${#files[@]} -eq 0 ]; then
                files=("$artifact")  # Fallback if no matches
            fi
        else
            # No wildcards: treat as literal path
            files=("$artifact")
        fi

        for file in "${files[@]}"; do
            if [ -f "$file" ] || [ -d "$file" ]; then
                # Check whether we have created a contents from a directory.
                # Remove 0-byte (empty) files so they are not uploaded
                if [ -f "$file" ] && [ ! -s "$file" ]; then
                    rm -f "$file" && echo "Removed empty artifact (not uploaded): $file"
                    continue
                fi
                if [ -d "${file}" ]; then
                    # gcloud fails on a directory that holds no files, so skip it like an empty file.
                    if [ -z "$(find -L "${file}" -type f -print -quit 2>/dev/null)" ]; then
                        echo "Skipped empty directory (not uploaded): ${file}"
                        continue
                    fi
                    copycmd="cp -r"
                    directory=$(basename "$file")
                    gcloud storage ls "${STORAGE_BUCKET_DESTINATION}/${directory}" 2>/dev/null | grep -q "/$"
                    if [ "${PIPESTATUS[0]}" -eq 0 ]; then
                        echo "Folder ${STORAGE_BUCKET_DESTINATION}/${directory} exists and is not empty, skip."
                        continue;
                    fi
                else
                    copycmd="cp"
                fi

                # Copy the artifact to the bucket (do not use quotes for cp!)
                # shellcheck disable=SC2086
                if gcloud storage ${copycmd} "${file}" "${STORAGE_BUCKET_DESTINATION}"/; then
                    echo "Copied ${file} to ${STORAGE_BUCKET_DESTINATION}"
                    echo "    gcloud storage ${copycmd} ${STORAGE_BUCKET_DESTINATION}/$(basename "$file") ." | tee -a "${ARTIFACT_SUMMARY}"
                else
                    echo "ERROR: failed to copy ${file} to ${STORAGE_BUCKET_DESTINATION}" >&2
                    failed=1
                fi
            fi
        done
    done
    return "${failed}"
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

RESULT=0

# Store artifacts to artifact storage.
if [ -n "${ARTIFACT_STORAGE_SOLUTION}" ] && [ -n "${STORAGE_BUCKET_DESTINATION}" ]; then
    if [ "${#ARTIFACT_LIST[@]}" -gt 0 ]; then
        "${ARTIFACT_STORAGE_SOLUTION_FUNCTION}" || RESULT=1
    else
        echo "No artifacts to store to ${ARTIFACT_STORAGE_SOLUTION}, ignored."
    fi
else
    # If not running from Jenkins, just NOOP!
    noop
fi

# Post storage commands. A successful command must not clear a store failure.
echo "Post storage commands:"
for command in "${POST_CLEANUP_COMMANDS[@]}"; do
    echo "${command}"
    eval "${command}" || RESULT=$?
done

if [ -f "${ARTIFACT_SUMMARY}" ]; then
    echo
    echo
    printf '%b\n' '\033[1;36m*** Build artifacts are ready — see below for location and access ***\033[0m'
    echo
    printf '%b\n' '\033[1;34m================================================================================\033[0m'
    printf '%b\n' '\033[1;34m                             ARTIFACT SUMMARY\033[0m'
    printf '%b\n' '\033[1;34m================================================================================\033[0m'
    echo
    while IFS= read -r line; do
        # Lines with URLs must stay plain: printf '%b' + trailing \\033[0m on the same line is
        # picked up by Jenkins / Argo log linkifiers as part of the href (%1B[0m in Console).
        if [[ "$line" =~ https?:// ]]; then
            echo "$line"
        else
            printf '%b%s%b\n' '\033[1;32m' "$line" '\033[0m'
        fi
    done < "${ARTIFACT_SUMMARY}"
    echo
    printf '%b\n' '\033[1;34m================================================================================\033[0m'
    echo
    echo
fi
# Return result
exit "${RESULT}"
