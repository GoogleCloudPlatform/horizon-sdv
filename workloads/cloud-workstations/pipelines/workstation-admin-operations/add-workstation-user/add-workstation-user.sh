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

set -eo pipefail

# Capture the arguments passed to the script
TF_BACKEND_BUCKET="$1"
TFVARS_JSON_FILE_PATH="$2"
REMOTE_TFVARS_JSON="output.tfvars.json"

# Import shared utils
source "$(dirname "$0")/../../utils/terraform-utils.sh"

# ------Initial Checks and Setup------

validate_bucket_and_tfvars_args "$TF_BACKEND_BUCKET" "$TFVARS_JSON_FILE_PATH"

# Extract terraform directory path
TF_DIR=$(dirname "${TFVARS_JSON_FILE_PATH}")
# Extract tfvars file name
TFVARS_JSON_FILE=$(basename "$TFVARS_JSON_FILE_PATH")

# Change directory temporarily (for terraform)
pushd "$TF_DIR" > /dev/null || log_error "Cannot cd to ${TF_DIR}"

# ------ JSON Helpers ------

input_cloud_ws_workstation_name=$(get_json_value_by_key_at_path "$TFVARS_JSON_FILE" "." "input_sdv_cloud_ws_workstation_name")

remove_key_from_json_at_path "$TFVARS_JSON_FILE" "." "input_sdv_cloud_ws_workstation_name"

input_cloud_ws_workstation_users=$(get_json_value_by_key_at_path "$TFVARS_JSON_FILE" "." "input_sdv_cloud_ws_workstation_users")

remove_key_from_json_at_path "$TFVARS_JSON_FILE" "." "input_sdv_cloud_ws_workstation_users"

# Add user emails for the specified Cloud Workstation
add_users_to_workstation() {
  local json_file="$1"
  local ws_id="$2"
  local csv_users="$3"

  # Validate input
  [[ -z "$json_file" || ! -f "$json_file" ]] && log_error "JSON file not found: $json_file"
  [[ -z "$ws_id" ]] && log_error "Workstation ID not provided."
  [[ -z "$csv_users" ]] && log_error "No users provided."

  # Convert comma-separated users into array
  IFS=',' read -r -a all_users <<< "$csv_users"

  local new_users=()

  # Check if each user already exists in the array
  for u in "${all_users[@]}"; do
    if jq -e --arg ws "$ws_id" --arg u "$u" \
      '.[$ws].sdv_cloud_ws_user_emails | index($u)' \
      "$json_file" >/dev/null; then
      log_warning "[SKIP] User ${u} already exists in workstation ${ws_id}"
    else
      new_users+=("$u")
    fi
  done

  # If no new users, exit early
  if [[ ${#new_users[@]} -eq 0 ]]; then
    log_error "No new users to add for workstation ${ws_id}"
  fi

  # Convert new_users array to JSON
  local users_json
  users_json=$(printf '%s\n' "${new_users[@]}" | jq -R . | jq -s .)

  # Update JSON in-place
  local tmp_file
  tmp_file=$(mktemp)

  jq --arg ws "$ws_id" --argjson add "$users_json" '
    .[$ws].sdv_cloud_ws_user_emails += $add
  ' "$json_file" > "$tmp_file" && mv "$tmp_file" "$json_file"

  log_info "Added new users to workstation ${ws_id}: ${new_users[*]}"
}


# ------Terraform workflow begins------

run_terraform_init "${TF_BACKEND_BUCKET}"
get_existing_workstations > "${REMOTE_TFVARS_JSON}"

if ! check_key_exists_in_json_at_path "${REMOTE_TFVARS_JSON}" "." "$input_cloud_ws_workstation_name"; then
  log_error "Workstation not found."
fi

validate_workstation_state "$input_cloud_ws_workstation_name"
add_users_to_workstation "$REMOTE_TFVARS_JSON" "$input_cloud_ws_workstation_name" "$input_cloud_ws_workstation_users"
merge_json_into_path "$TFVARS_JSON_FILE" ".workstations" "$REMOTE_TFVARS_JSON"

run_terraform_apply "${TFVARS_JSON_FILE}"

# Exit terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."
exit 0
