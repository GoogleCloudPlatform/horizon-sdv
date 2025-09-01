#!/bin/bash

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
WORKSTATIONS_TFVARS_JSON_FILE_PATH="$2"
CURRENT_USER="$3"

# Import shared utils
source "$(dirname "$0")/../../utils/terraform-utils.sh"

# Temporary file used to store tfstate JSON of Workstations
WORKSTATIONS_TFSTATE_JSON_FILE="workstations_tfstate.json"
# Temporary file used to store extracted workstations for specified user as JSON
EXISTING_WORKSTATIONS_FOR_USER_JSON_FILE="existing_workstations_for_user.json"


# ------Functions------

# Function to get names and urls of all Workstations from existing Workstations data for current user
get_workstation_names_and_urls() {
  local existing_workstations_for_user_json_file="$1"

  [[ -z "$existing_workstations_for_user_json_file" ]] && log_error "Existing workstations for current user JSON file NOT provided as argument."
  [[ ! -f "$existing_workstations_for_user_json_file" ]] && log_error "File ${existing_workstations_for_user_json_file} does not exist."

  log_info "Filtering Workstation names and their urls from existing Workstations data for current user '${CURRENT_USER}' from JSON file '${existing_workstations_for_user_json_file}'..."

  jq '
    to_entries
    | map({
        workstation_name: .value.ws_name,
        workstation_url: .value.ws_url
      })
  ' "$existing_workstations_for_user_json_file" || log_error "Failed to run jq: invalid JSON while extracting workstation names and URLs."
}


# ------Initial Checks and Setup------

validate_bucket_and_tfvars_args "$TF_BACKEND_BUCKET" "$WORKSTATIONS_TFVARS_JSON_FILE_PATH"

# Extract workstations terraform directory path
WORKSTATIONS_TF_DIR=$(dirname "${WORKSTATIONS_TFVARS_JSON_FILE_PATH}")
# Extract workstations tfvars file name
WORKSTATIONS_TFVARS_JSON_FILE=$(basename "$WORKSTATIONS_TFVARS_JSON_FILE_PATH")

# ---Check WS Cluster exists before proceeding---
# Extract Workstation Cluster terraform directory path
WS_CLUSTER_TF_DIR="${WORKSTATIONS_TF_DIR}/../cluster"
if ! check_ws_cluster_exists "$WS_CLUSTER_TF_DIR" "$TF_BACKEND_BUCKET"; then
  log_error "Workstation Cluster must exist before any operation of Workstations. Please ask your admin to run 'Create Cluster' job and then 'Create Configuration' job."
fi

# Change to workstations terraform directory temporarily
pushd "$WORKSTATIONS_TF_DIR" > /dev/null || log_error "Cannot cd to ${WORKSTATIONS_TF_DIR}"

print_header "CLOUD WORKSTATION: LIST WORKSTATIONS"

run_terraform_init "$TF_BACKEND_BUCKET"


# ------Extract Workstations------

# Store WS tfstate in a file
export_tfstate_to_file "$WORKSTATIONS_TFSTATE_JSON_FILE"
log_info "Exported WS Workstations tfstate JSON to file: '${WORKSTATIONS_TFSTATE_JSON_FILE}'."

# Extract existing Workstations for the current user
get_existing_workstations_for_user "$WORKSTATIONS_TFSTATE_JSON_FILE" "$CURRENT_USER" > "$EXISTING_WORKSTATIONS_FOR_USER_JSON_FILE" || log_error "Failed exporting existing Workstations for current user '${CURRENT_USER}' as JSON to file $EXISTING_WORKSTATIONS_FOR_USER_JSON_FILE"
log_info "Exported existing Workstations data for current user '${CURRENT_USER}' to file: '${EXISTING_WORKSTATIONS_FOR_USER_JSON_FILE}' - will now be used for further operations."

# Extract Workstation names and their urls from existing Workstations data for the current user
workstation_names_and_urls=$(get_workstation_names_and_urls "$EXISTING_WORKSTATIONS_FOR_USER_JSON_FILE")

# ------Show Details of Workstation------

if [[ -z "$workstation_names_and_urls" || "$workstation_names_and_urls" == "{}" || "$workstation_names_and_urls" == "null" ]]; then
  log_warning "NO Workstations found."
else
  log_success "Workstations found. --- List of Workstations:"
  echo "$workstation_names_and_urls" | jq -C | print_result
fi

# Exit workstation terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."
exit 0