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
# Destroy the spanner instance, backups and associated bucket.
# You 1st destroy the INSTANCE and then the BUCKET.
#
# Variables:
#   DESTROY_ACTION: .      The action to perform.
#   ABFS_DB_BUCKET_NAME:   The bucket to remove.
#   ABFS_DB_NAME:          The database instance to destroy.
set -euo pipefail

function destroy_spanner() {

    # Validate DESTROY_ACTION parameter early to avoid accidental destructive actions.
    # Valid values are BUCKET or INSTANCE.
    if [[ -z "${DESTROY_ACTION:-}" ]]; then
        echo -e "\033[1;31mError: DESTROY_ACTION parameter is required.\033[0m"
        exit 1
    fi

    # Branch based on the requested destroy action.
    case "${DESTROY_ACTION}" in
        "BUCKET")
            # Bucket deletion path. Requires ABFS_DB_BUCKET_NAME to be set.
            if [[ -n "${ABFS_DB_BUCKET_NAME:-}" ]]; then
                echo -e "\033[1;32mDestroying bucket gs://${ABFS_DB_BUCKET_NAME}... please wait as this can take a while!\033[0m"
                gcloud storage rm --recursive "gs://${ABFS_DB_BUCKET_NAME}/"
                echo -e "\033[1;32mDestroyed bucket gs://${ABFS_DB_BUCKET_NAME} successfully.\033[0m"
                echo -e "\033[1;32mJob completed successfully.\033[0m"
            else
                echo -e "\033[1;31mError: ABFS_DB_BUCKET_NAME parameter is required for BUCKET destruction.\033[0m"
                exit 1
            fi
            ;;
        "INSTANCE")
            # Instance deletion path. Requires ABFS_DB_NAME to be set.
            if [[ -n "${ABFS_DB_NAME:-}" ]]; then
                echo -e "\033[1;32mStarting instance destruction...\033[0m"

                # Delete all backup schedules
                echo -e "\033[1;32mListing and deleting backup schedules...\033[0m"
                schedules=$(gcloud spanner backup-schedules list --instance="$ABFS_DB_NAME" --database="$ABFS_DB_NAME" --format="value(name)" || true)
                if [[ -z "$schedules" ]]; then
                    echo -e "\033[1;33No backup schedules found.\033[0m"
                else
                    while IFS= read -r schedule; do
                        [[ -z "$schedule" ]] && continue
                        echo -e "\033[1;32mDestroying schedule $schedule... please wait as this can take a while!\033[0m"
                        gcloud spanner backup-schedules list --instance="${ABFS_DB_NAME}" --database="${ABFS_DB_NAME}" --format="value(name)"
                    done <<< "$schedules"
                fi
                echo -e "\033[1;32mAll backup schedules destroyed.\033[0m"

                # Delete all backups first to satisfy dependency constraints.
                echo -e "\033[1;32mListing and deleting backups...\033[0m"
                backups=$(gcloud spanner backups list --instance="${ABFS_DB_NAME}" --format="value(name)")
                if [[ -z "$backups" ]]; then
                    echo -e "\033[1;33No backups found.\033[0m"
                else
                    while IFS= read -r backup; do
                        [[ -z "$backup" ]] && continue
                        echo -e "\033[1;32mDestroying backup $backup... please wait as this can take a while!\033[0m"
                        gcloud spanner backups delete "$backup" --instance="${ABFS_DB_NAME}" --quiet
                    done <<< "$backups"
                fi
                echo -e "\033[1;32mAll backups destroyed.\033[0m"

                # Finally delete the instance itself.
                echo -e "\033[1;32mDestroying instance... please wait as this can take a while!\033[0m"
                gcloud spanner instances delete "${ABFS_DB_NAME}" --quiet
                echo -e "\033[1;32mInstance destroyed successfully.\033[0m"
                echo -e "\033[1;32mJob completed successfully.\033[0m"
            else
                echo -e "\033[1;31mError: ABFS_DB_NAME parameter is required for INSTANCE destruction.\033[0m"
                exit 1
            fi
            ;;
        *)
            # Unknown/unsupported action guard.
            echo -e "\033[1;31mError: Invalid DESTROY_ACTION '${DESTROY_ACTION}'. Valid values are 'BUCKET' or 'INSTANCE'.\033[0m"
            exit 1
            ;;
    esac
}

destroy_spanner
