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
WS_CONFIGS_TFVARS_JSON_FILE_PATH="$2"

# Import shared utils
source "$(dirname "$0")/../../utils/terraform-utils.sh"

# Temporary file used to store tfstate JSON of WS Configs
WS_CONFIGS_TFSTATE_JSON_FILE="ws_configs_tfstate.json"
# Temporary file used to store extracted workstation configs and their corresponding IAM bindings (user emails) JSON
EXISTING_WS_CONFIGS_WITH_WS_ADMINS_JSON_FILE="existing_ws_configs_with_ws_admins.json"


# ------Functions------

# Function to get list of WS Configs from tfstate that match given regex pattern
# Returns (output to stdout) each matching config name in its own line, as plain text (due to -r option in jq). If nothing matches, output is empty.
get_ws_configs_matching_regex() {
  local ws_configs_tfstate_json_file="$1"
  local ws_config_name_regex_pattern="$2"

  log_info "Filtering list of WS Configs from tfstate JSON file '${ws_configs_tfstate_json_file}' that match given regex pattern: '${ws_config_name_regex_pattern}'..."

  local matching_configs
  jq -r --arg reg "$ws_config_name_regex_pattern" '
    .values.root_module.resources // []
    | map(select(.type == "google_workstations_workstation_config" and .mode == "managed"))
    | map(.index)
    | map(select(test($reg)))
    | .[]
  ' "${ws_configs_tfstate_json_file}" || log_error "Failed to run jq: invalid JSON or invalid regex pattern."
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

# Change directory temporarily (for terraform)
pushd "$WS_CONFIGS_TF_DIR" > /dev/null || log_error "Cannot cd to ${WS_CONFIGS_TF_DIR}"

print_header "CLOUD WORKSTATION: LIST CONFIGURATIONS"

run_terraform_init "$TF_BACKEND_BUCKET"


# ------Extract input regex------

# Store WS Config tfstate in a file
export_tfstate_to_file "$WS_CONFIGS_TFSTATE_JSON_FILE"
log_info "Exported WS Config tfstate JSON to file: '${WS_CONFIGS_TFSTATE_JSON_FILE}'."

# Extract input WS Config name regex pattern from tfvars file
input_ws_config_name_regex_pattern=$(get_json_value_by_key_at_path "$WS_CONFIGS_TFVARS_JSON_FILE" "." "sdv_cloud_ws_input_config_name")

# Get list of WS Configs that match provided regex pattern
matching_configs_list=$(get_ws_configs_matching_regex "$WS_CONFIGS_TFSTATE_JSON_FILE" "$input_ws_config_name_regex_pattern")


# ------Show list of WS Configs that match------

if [[ -z "$matching_configs_list" ]]; then
  log_warning "NO matching workstation Configurations found for input regex pattern: '${input_ws_config_name_regex_pattern}'"
else
  log_success "List of matching workstation Configurations found for input regex pattern: '${input_ws_config_name_regex_pattern}'"
  echo "$matching_configs_list" | print_result
fi

# Exit Config terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."
exit 0
