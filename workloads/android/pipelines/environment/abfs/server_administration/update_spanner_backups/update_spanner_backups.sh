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
# Update Spanner backup schedules.
#
# Variables:
#   BACKUP_SCHEDULE_ACTION: .The action to perform.
#   BACKUP_SCHEDULE_ID:      The backup schedule identifier.
#   CRON:                    Backup schedule (crontab)
#   DATABASE:                Database name (default: abfs)
#   RETENTION_DURATION:      Time in seconds to retain backups.
set -euo pipefail

BACKUP_SCHEDULE_ACTION=${BACKUP_SCHEDULE_ACTION:-DETAILS}

function update_backup_schedule() {
    # Build instance stream and (optionally) filter by ABFS_DB_NAME without using eval.
    # Use grep -F for fixed string matching and tolerate no matches under strict mode.
    gcloud spanner instances list --format="value(name)" | while IFS= read -r instance; do
        [[ -z "$instance" ]] && continue

        db="${DATABASE}"
        # Derive DATABASE name: default to "abfs", override from backups' SOURCE_DATABASE when present.
        while IFS= read -r backup; do
            [[ -z $backup ]] && continue
            db=$(gcloud spanner backups list --instance="$instance" --filter="name=$backup" --format="value(SOURCE_DATABASE)")
        done < <(gcloud spanner backups list --instance="$instance" --format="value(name)")

        # If DATABASE is still empty (no backups), you may choose a fallback such as the first database.
        if [[ -z "${db:-}" ]]; then
            db=$(gcloud spanner databases list --instance="$instance" --format="value(name)" | head -n1 || true)
        fi

        # Switch on the requested backup schedule action.
        case "${BACKUP_SCHEDULE_ACTION:-}" in
            "DETAILS")
                # List schedules and show details for the actual schedule being iterated.
                gcloud spanner backup-schedules list --instance="$instance" --database="$DATABASE" --format="value(name)" \
                    | while IFS= read -r schedule; do
                          [[ -z "$schedule" ]] && continue
                          echo -e "\033[1;32m"
                          echo "Backup Schedule $schedule"
                          echo "Cron: $(gcloud spanner backup-schedules describe "$schedule" --instance="$instance" --database="$db" --format="value(spec.cronSpec.text)")"
                          echo "Retention Duration: $(gcloud spanner backup-schedules describe "$schedule" --instance="$instance" --database="$db" --format="value(retentionDuration)")"
                          echo -e "\033[0m"
                      done
                 ;;
             "CREATE")
                 # Create a new schedule with provided CRON and RETENTION_DURATION (seconds).
                 gcloud spanner backup-schedules create "${BACKUP_SCHEDULE_ID}" \
                     --instance="$instance" \
                     --database="$db" \
                     --cron="${CRON}" \
                     --retention-duration="${RETENTION_DURATION}s" \
                     --encryption-type=USE_DATABASE_ENCRYPTION \
                     --backup-type=full-backup
                 echo -e "\033[1;32mCreated ${BACKUP_SCHEDULE_ID}\033[0m"
                 ;;
             "DELETE")
                 # Delete the specified schedule quietly.
                 gcloud spanner backup-schedules delete "${BACKUP_SCHEDULE_ID}" \
                     --instance="$instance" \
                     --database="$db" \
                     --quiet
                 echo -e "\033[1;32mDeleted ${BACKUP_SCHEDULE_ID}\033[0m"
                 ;;
             "UPDATE")
                 # Update schedule cron and retention.
                 gcloud spanner backup-schedules update "${BACKUP_SCHEDULE_ID}" \
                     --instance="$instance" \
                     --database="$db" \
                     --cron="${CRON}" \
                     --retention-duration="${RETENTION_DURATION}s" \
                     --encryption-type=USE_DATABASE_ENCRYPTION
                 echo -e "\033[1;32mUpdated ${BACKUP_SCHEDULE_ID}\033[0m"
                 ;;
             *)
                 echo -e "\033[1;33mError: unsupported option ${BACKUP_SCHEDULE_ACTION:-<unset>}\033[0m"
                 ;;
        esac
    done
}

update_backup_schedule
