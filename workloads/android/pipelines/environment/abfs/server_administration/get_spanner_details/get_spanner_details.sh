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
# Retrieve ABFS (GCE) spanner details.
# - List the spanner database instance, backups, schedules and buckets.
#   Leave variables empty to use defaults.
#
# Variables:
#   ZONE:      The GCP project zone
#   ABFS_BUCKET_NAME:   Bucket name
#   ABFS_DB_NAME:       DB name
set -euo pipefail

ABFS_DB_NAME=${ABFS_DB_NAME:-abfs}

function artifact() {
    local file="${WORKSPACE}/${1}-status.txt"
    tee >(sed -E 's/\\x1b\\[[0-9;]*[A-Za-z]//g' >> "$file")
}

function list_buckets() {
    # List instances using gcloud's native filter and iterate safely.
    # Filter buckets: if ABFS_DB_BUCKET_NAME is set use fixed-string grep (-F),
    # otherwise match default ABFS pattern with extended regex (-E).
    if [[ -n "${ABFS_DB_BUCKET_NAME:-}" ]]; then
        bucket_filter="${ABFS_DB_BUCKET_NAME}"
        bucket_grep_opts="-F"
    else
        bucket_filter='^abfs-[0-9a-fA-F]{4}$'
        bucket_grep_opts="-E"
    fi

    # List buckets and apply filter.
    echo -e "\033[1;32mListing buckets...\033[0m"
    buckets=$(gcloud storage buckets list --format="value(name)" | { grep ${bucket_grep_opts} "${bucket_filter}" || true; })
    if [[ -z "$buckets" ]]; then
        echo -e "\033[1;33mNo buckets found.\033[0m"
    else
        while IFS= read -r bucket; do
            # Use while/read to handle names safely; always quote variables.
            [[ -z "$bucket" ]] && continue
            { echo -e "\033[1;32mBucket $bucket details:\033[0m"; echo; } | artifact "$bucket"
            size=$(gcloud storage du --readable-sizes "gs://${bucket}/" --summarize)
            { echo -e "Size: ${size}"; echo;} | artifact "$bucket"
        done <<< "$buckets"
    fi
}

function list_instances() {
    # Default database hint; may be overwritten from backups SOURCE_DATABASE below.
    # If an instance has no backups, this may remain "abfs" and schedules may be empty.
    db="${ABFS_DB_NAME}"
    instances=$(gcloud spanner instances list --format="value(name)" || true)
    found=0
    while IFS= read -r instance; do
        [[ -z "$instance" ]] && continue
        # If ABFS_DB_NAME is set, only keep instances whose name contains it (safe under set -u)
        if [[ -n "${ABFS_DB_NAME:-}" && "$instance" != *"${ABFS_DB_NAME}"* ]]; then
            echo -e "\033[1;33mSkipping instance $instance != ${ABFS_DB_NAME}\033[0m"
            continue
        fi
        found=1
        { echo -e "\033[1;32mInstance $instance details:\033[0m"; echo; } | artifact "$instance"
        { gcloud spanner instances describe "$instance"; echo; } | artifact "$instance"
        echo -e "\033[1;32mBackup list:\033[0m\033[0m"; echo
        # Enumerate backups safely; derive SOURCE_DATABASE for schedules below.
        backups=$(gcloud spanner backups list --instance="$instance" --format="value(name)" || true)
        if [[ -z "$backups" ]]; then
            echo -e "\033[1;33mNo backups found.\033[0m"
        else
            while IFS= read -r backup; do
                [[ -z "$backup" ]] && continue
                echo -e "\033[1;32mBackup $backup\033[0m"; echo
                { gcloud spanner backups describe "$backup" --instance="$instance"; echo; } | artifact "$instance"
                db=$(gcloud spanner backups list --instance="$instance" --filter="$backup" --format="value(SOURCE_DATABASE)")
            done <<< "$backups"
        fi

        echo; echo -e "\033[1;32mBackup Schedule:\033[0m"; echo
        # Query backup schedules for the most recently set database (if any).
        schedules=$(gcloud spanner backup-schedules list --instance="$instance" --database="$db" --format="value(name)" || true)
        if [[ -z "$schedules" ]]; then
            echo -e "\033[1;33mNo backup schedules found.\033[0m"
        else
            while IFS= read -r schedule; do
                [[ -z "$schedule" ]] && continue
                { echo -e "\033[1;32mBackup Schedule $schedule\033[0m"; echo; } | artifact "$instance"
                { gcloud spanner backup-schedules describe "$schedule" --instance="$instance" --database="$db"; echo; } | artifact "$instance";
            done <<< "$schedules"
        fi
        # shellcheck disable=SC2086
        { echo; echo -e "\033[1;32mInstance $instance state: $(gcloud spanner instances describe $instance --format='value(instanceType)')\033[0m"; } | artifact "$instance"
    done <<< "$instances"

    if [[ "$found" -eq 0 ]]; then
        echo -e "\033[1;33mNo instances found, may have been destroyed.\033[0m"
    fi
}

function main() {
    list_buckets
    list_instances
}

main
