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
# Temporary file used to store extracted workstation configs and their corresponding IAM bindings (user emails) JSON
EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE="existing_ws_configs_with_ws_admins.json"


# ------Functions------

# Function to update param value of WS_REPLICA_ZONES in the input tfvars json file, with value from existing config data in order to retain its original value on update operation. (This is done as WS_REPLICA_ZONES cannot be updated)
retain_existing_ws_config_replica_zones() {
  local ws_configs_tfvars_json_file="$1"
  local input_ws_config_name="$2"
  local existing_ws_configs_with_ws_admins="$3"

  log_info "Updating file '${ws_configs_tfvars_json_file}' to retain existing WS Replica Zones..."

  # Get existing WS_REPLICA_ZONES JSON string
  local existing_ws_replica_zones_json
  existing_ws_replica_zones_json=$(get_json_value_by_key_at_path "$existing_ws_configs_with_ws_admins" ".${input_ws_config_name}" "ws_replica_zones")

  # Initialize bash array
  local existing_ws_replica_zones=()
  # Convert JSON array string into bash array
  readarray -t existing_ws_replica_zones < <(jq -r '.[]' <<< "$existing_ws_replica_zones_json")

  # Convert bash array back to JSON string before passing
  local replica_zones_json
  replica_zones_json=$(printf '%s\n' "${existing_ws_replica_zones[@]}" | jq -R . | jq -s .)

  # Update zones in input tfvars json file (retain original value)
  update_json_value_by_key_at_path "$ws_configs_tfvars_json_file" ".sdv_cloud_ws_configs.${input_ws_config_name}" "ws_replica_zones" "$replica_zones_json"
}


# ------Initial Checks and Setup------

validate_bucket_and_tfvars_args "$TF_BACKEND_BUCKET" "$WS_CONFIGS_TFVARS_JSON_FILE_PATH"

# Extract terraform directory path
WS_CONFIGS_TF_DIR=$(dirname "${WS_CONFIGS_TFVARS_JSON_FILE_PATH}")
# Extract Config tfvars file name
WS_CONFIGS_TFVARS_JSON_FILE=$(basename "$WS_CONFIGS_TFVARS_JSON_FILE_PATH")

# ---Check WS Cluster exists before proceeding---
# Extract Workstation Cluster terraform directory path
WS_CLUSTER_TF_DIR="${WS_CONFIGS_TF_DIR}/../cluster"
if ! check_ws_cluster_exists "$WS_CLUSTER_TF_DIR" "$TF_BACKEND_BUCKET"; then
  log_error "Workstation Cluster must exist before any operation of Workstation Config. Please run 'Create Cluster' job first."
fi

# Change directory temporarily to WS Config terraform
pushd "$WS_CONFIGS_TF_DIR" > /dev/null || log_error "Cannot cd to ${WS_CONFIGS_TF_DIR}"

print_header "CLOUD WORKSTATION: UPDATE CONFIGURATION"

run_terraform_init "$TF_BACKEND_BUCKET"


# ------Prepare final tfvars.json file------

# Store WS Config tfstate in a file
export_tfstate_to_file "$WS_CONFIGS_TFSTATE_JSON_FILE"
log_info "Exported WS Config tfstate JSON to file: '${WS_CONFIGS_TFSTATE_JSON_FILE}'."

# Extract existing WS Configs
get_existing_ws_configs_with_ws_admins "$WS_CONFIGS_TFSTATE_JSON_FILE" > "$EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE" || log_error "Failed exporting existing workstation Configs and WS Admins as JSON to file $EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE"
log_info "Exported existing WS Configs data to file: '${EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE}' - will now be used for further operations."

# Extract input WS Config name from tfvars file
input_ws_config_name=$(get_json_value_by_key_at_path "$WS_CONFIGS_TFVARS_JSON_FILE" "." "sdv_cloud_ws_input_config_name")

# Prevent update for non-existent input WS Config among existing WS Configs
if ! check_key_exists_in_json_at_path "$EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE" "." "${input_ws_config_name}"; then
  log_error "Please enter a workstation Configuration name that exists. Aborting..."
fi
log_info "Input WS Config: '${input_ws_config_name}' found in existing WS Configs."

# Retain existing list of WS_REPLICA_ZONES
retain_existing_ws_config_replica_zones "$WS_CONFIGS_TFVARS_JSON_FILE" "$input_ws_config_name" "$EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE"

# Remove existing (older) WS Config object from existing WS Configs file
remove_key_from_json_at_path "$EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE" "." "$input_ws_config_name"
log_info "Removed original WS Config key '${input_ws_config_name}' from existing WS Configs - to be replaced by new input WS Config key and data."

# Create a combined tfvars.json file with new input and existing configs data
merge_json_into_path "$WS_CONFIGS_TFVARS_JSON_FILE" ".sdv_cloud_ws_configs" "$EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE"
log_info "Final '${WS_CONFIGS_TFVARS_JSON_FILE}' file containing updated WS config + existing WS Configs along with their corresponding WS Admins is READY!"


# ------Terraform apply------

run_terraform_apply "$WS_CONFIGS_TFVARS_JSON_FILE"
log_success "Updated Workstation Configuration: '${input_ws_config_name}' with new input data."

# Exit Config terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."
exit 0
