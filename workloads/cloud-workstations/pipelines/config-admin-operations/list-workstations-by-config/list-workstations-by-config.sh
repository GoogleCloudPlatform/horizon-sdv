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
WS_CONFIGS_TFVARS_JSON_FILE_PATH="$2"

# Import shared utils
source "$(dirname "$0")/../../utils/terraform-utils.sh"

# Temporary file used to store tfstate JSON of WS Configs
WS_CONFIGS_TFSTATE_JSON_FILE="ws_configs_tfstate.json"
# Temporary file used to store tfstate JSON of Workstations
WORKSTATIONS_TFSTATE_JSON_FILE="workstations_tfstate.json"
# Temporary file used to store extracted workstation configs and their corresponding IAM bindings (user emails) JSON
EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE="existing_ws_configs_with_ws_admins.json"


# ------Functions------

# Function to filter list of existing workstations created using a given config name from tfstate
get_workstations_by_config() {
  local workstations_tfstate_json_file="$1"
  local input_ws_config_name="$2"

  log_info "Filtering list of existing Workstations created using input config: '${input_ws_config_name}' from tfstate JSON file '${workstations_tfstate_json_file}'..."

  jq -r --arg config_name "$input_ws_config_name" '
    .values.root_module.resources // []
    | map(select(.type == "google_workstations_workstation" and .mode == "managed"))
    | map(.values)
    | map(select(.workstation_config_id == $config_name))
    | map(.workstation_id)
    | .[]
  ' "$workstations_tfstate_json_file" || log_error "Failed to run jq: invalid JSON."
}

# ------Initial Checks and Setup------

validate_bucket_and_tfvars_args "$TF_BACKEND_BUCKET" "$WS_CONFIGS_TFVARS_JSON_FILE_PATH"

# Extract WS Config terraform directory path
WS_CONFIGS_TF_DIR=$(dirname "${WS_CONFIGS_TFVARS_JSON_FILE_PATH}")
# Extract Config tfvars file name
WS_CONFIGS_TFVARS_JSON_FILE=$(basename "$WS_CONFIGS_TFVARS_JSON_FILE_PATH")

# ---Check WS Cluster exists before proceeding---
# Extract Workstation Cluster terraform directory path
WS_CLUSTER_TF_DIR="${WS_CONFIGS_TF_DIR}/../cluster"
if ! check_ws_cluster_exists "$WS_CLUSTER_TF_DIR" "$TF_BACKEND_BUCKET"; then
  log_error "Workstation Cluster must exist before any operation of Workstation Config. Please run 'Create Cluster' job first."
fi

print_header "CLOUD WORKSTATION: LIST WORKSTATIONS BY CONFIG"


# ------Extract WS Config------

log_info "Changing directory to WS Configs terraform..."
pushd "$WS_CONFIGS_TF_DIR" > /dev/null || log_error "Cannot cd to ${WS_CONFIGS_TF_DIR}"

run_terraform_init "$TF_BACKEND_BUCKET"

# Store WS Config tfstate in a file
export_tfstate_to_file "$WS_CONFIGS_TFSTATE_JSON_FILE"
log_info "Exported WS Config tfstate JSON to file: '${WS_CONFIGS_TFSTATE_JSON_FILE}'."

# Extract existing WS Configs data
get_existing_ws_configs_with_ws_admins "$WS_CONFIGS_TFSTATE_JSON_FILE" > "$EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE" || log_error "Failed exporting existing workstation Configs and WS Admins as JSON to file '${EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE}'."
log_info "Exported existing WS Configs and their WS Admins to file: '${EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE}'."

# Extract input WS Config name from tfvars file
input_ws_config_name=$(get_json_value_by_key_at_path "$WS_CONFIGS_TFVARS_JSON_FILE" "." "sdv_cloud_ws_input_config_name")

# Check for non-existent input WS Config among existing WS Configs
if ! check_key_exists_in_json_at_path "$EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE" "." "${input_ws_config_name}"; then
  log_error "Please enter a workstation Configuration name that exists. Aborting..."
fi
log_info "Input WS Config: '${input_ws_config_name}' found in existing WS Configs."

# Exit Config terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."


# ------Extract Workstations by config------

# Change to workstation terraform directory temporarily
WORKSTATIONS_TF_DIR="${WS_CONFIGS_TF_DIR}/../workstation"
log_info "Changing directory to Workstations terraform..."
pushd "$WORKSTATIONS_TF_DIR" > /dev/null || log_error "Cannot cd to ${WORKSTATIONS_TF_DIR}"

run_terraform_init "$TF_BACKEND_BUCKET"

# Store Workstations tfstate in a file
export_tfstate_to_file "$WORKSTATIONS_TFSTATE_JSON_FILE"
log_info "Exported Workstations tfstate JSON to file: '${WORKSTATIONS_TFSTATE_JSON_FILE}'."

# Get list of workstations created using input config name
workstations_list=$(get_workstations_by_config "$WORKSTATIONS_TFSTATE_JSON_FILE" "$input_ws_config_name")


# ------Show list of Workstations------

if [[ -z "$workstations_list" ]]; then
  log_warning "NO workstations found for input configuration: '${input_ws_config_name}'"
else
  log_success "List of Workstations created using input Configuration: '${input_ws_config_name}'"
  echo "$workstations_list" | print_result
fi

# Exit Workstation terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."
exit 0
