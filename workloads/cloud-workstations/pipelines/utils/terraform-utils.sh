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

set -euo pipefail

# ------Logging helper functions-------

print_header() {
  echo
  echo "┌────────────────────────────────────────────────────────────────┐"
  printf "│ %-62s │\n" "$1"
  echo "└────────────────────────────────────────────────────────────────┘"
}

print_result() {
  echo "┌─RESULT"
  # Read and indent lines from stdin preserving colors and whitespace
  while IFS= read -r line || [[ -n $line ]]; do
    printf '    %s\n' "$line"
  done
  echo "└─"
}

log_info() { echo -e "\n [INFO] $1 \n" >&2; }
log_success() { echo -e "\n\u001B[32m [SUCCESS] $1 \u001B[0m\n" >&2; }
log_warning() { echo -e "\n\u001B[33m [WARNING] $1 \u001B[0m\n" >&2; }
log_error() { echo -e "\n\u001B[31m [ERROR] $1\u001B[0m" >&2; exit 1; }


# ------Generic JSON handling functions------

convert_json_object_path_to_jq_path_array() {
  local json_object_path="$1"

  [[ -z "$json_object_path" ]] && log_error "Input JSON object path not provided as argument"

  local jq_path_array
  if [[ "$json_object_path" == "." ]]; then
    jq_path_array="[]"
  else
    json_object_path="${json_object_path#.}"
    jq_path_array=$(jq -nc --arg p "$json_object_path" '$p | split(".")')
  fi

  echo "${jq_path_array}"
}

# Function to check if a key exists at a given object path in input json file
# Returns boolean
check_key_exists_in_json_at_path() {
  local json_file="$1"
  local json_object_path="$2"
  local key="$3"

  # Validate all args
  [[ -z "$json_file" ]] && log_error "Input JSON file not provided as argument"
  [[ ! -f "$json_file" ]] && log_error "File ${json_file} does not exist."
  [[ -z "$json_object_path" ]] && log_error "JSON object path of key not provided as argument."
  [[ -z "$key" ]] && log_error "JSON Key to get value for not provided as an argument."

  log_info "Checking if key: '${key}' exists at path '${json_object_path}' in JSON file '${json_file}'..."

  jq_path_array=$(convert_json_object_path_to_jq_path_array "$json_object_path")

  local exists
  exists=$(
    jq -r --argjson path_array "${jq_path_array}" --arg key "$key" '
      getpath($path_array) | has($key)
    ' "$json_file" 2>/dev/null
  )

  if [[ "$exists" == "true" ]]; then
    return 0
  else
    return 1
  fi
}

# Function to extract value for given key at a given object path in input json file
# Returns (output to stdout) a JSON object value. If value is string, then its returned without quotes
get_json_value_by_key_at_path() {
  local json_file="$1"
  local json_object_path="$2"
  local key="$3"

  # Validate all args
  [[ -z "$json_file" ]] && log_error "Input JSON file not provided as argument"
  [[ ! -f "$json_file" ]] && log_error "File ${json_file} does not exist."
  [[ -z "$json_object_path" ]] && log_error "JSON object path of key not provided as argument."
  [[ -z "$key" ]] && log_error "JSON Key to get value for not provided as an argument."
  
  # Check existence of key in given json file at given path
  if ! check_key_exists_in_json_at_path "$json_file" "$json_object_path" "$key"; then
    log_error "Key '$key' does NOT exist at path '$json_object_path' in file '$json_file'."
  fi

  log_info "Extracting JSON value for key: '${key}' at path '${json_object_path}' in JSON file '${json_file}'..."

  jq_path_array=$(convert_json_object_path_to_jq_path_array "$json_object_path")

  # Extract value safely
  value=$(jq -re --argjson path_array "$jq_path_array" --arg key "$key" '
    getpath($path_array) | .[$key]
  ' "$json_file") || log_error "Failed to run jq: invalid JSON while extracting JSON value for key: '${key}' at path '${json_object_path}' in JSON file '${json_file}'..."

  echo "$value"
}

# Function to update a value of a specified key at a given object path from input json file
# Returns nothing, just modifies input json_file
update_json_value_by_key_at_path() {
  local json_file="$1"
  local json_object_path="$2"
  local key_to_update="$3"
  local new_value="$4"

  # Validate all args
  [[ -z "$json_file" ]] && log_error "Input JSON file not provided as argument."
  [[ ! -f "$json_file" ]] && log_error "File ${json_file} does not exist."
  [[ -z "$json_object_path" ]] && log_error "JSON object path of key not provided as argument."
  [[ -z "$key_to_update" ]] && log_error "Key who's value need to be updated in JSON was not provided as argument."
  [[ -z ${new_value+x} ]] && log_error "New value to be updated not provided as argument."

  log_info "Updating value for key: '${key_to_update}' at path '${json_object_path}' in JSON file '${json_file}'..."

  local tmp_json_file
  tmp_json_file=$(mktemp) || log_error "Failed to create temp file for operation function '${FUNCNAME[0]}'."

  trap "rm -f '$tmp_json_file'" EXIT
  
  jq_path_array=$(convert_json_object_path_to_jq_path_array "$json_object_path")

  jq --argjson path_array "$jq_path_array" --arg key "$key_to_update" --argjson val "$new_value" '
    (. as $root
    | getpath($path_array) as $parent
    | $root
    | setpath($path_array; $parent | .[$key] = $val)
    )
  ' "$json_file" > "$tmp_json_file"

  if [[ $? -ne 0 ]]; then
    log_error "Failed to update ${json_file} while updating value for key: ${key_to_update}"
  fi
  
  mv "$tmp_json_file" "$json_file" || log_error "Failed moving ${tmp_json_file} file contents into original ${json_file} post operation of function '${FUNCNAME[0]}'."
}

# Function to remove a key at a given object path from input json file
# Returns nothing, just modifies input json_file
remove_key_from_json_at_path() {
  local json_file="$1"
  local json_object_path="$2"
  local key_to_remove="$3"

  # Validate all args
  [[ -z "$json_file" ]] && log_error "Input JSON file not provided as argument."
  [[ ! -f "$json_file" ]] && log_error "File ${json_file} does not exist."
  [[ -z "$json_object_path" ]] && log_error "JSON object path of key not provided as argument."
  [[ -z "$key_to_remove" ]] && log_error "Key to remove from JSON not provided as argument."

  log_info "Removing key: '${key_to_remove}' at path '${json_object_path}' in JSON file '${json_file}'..."


  local tmp_json_file
  tmp_json_file=$(mktemp) || log_error "Failed to create temp file for operation function '${FUNCNAME[0]}'."

  trap "rm -f '$tmp_json_file'" EXIT

  jq_path_array=$(convert_json_object_path_to_jq_path_array "$json_object_path")

  jq --argjson path_array "$jq_path_array" --arg key "$key_to_remove" '
    getpath($path_array) |= del(.[$key])
  ' "$json_file" > "$tmp_json_file"

  if [[ $? -ne 0 ]]; then
    log_error "Failed to update ${json_file} while removing key: ${key_to_remove}"
  fi
  
  mv "$tmp_json_file" "$json_file" || log_error "Failed moving '${tmp_json_file}' file contents into original '${json_file}' post operation of function '${FUNCNAME[0]}'."
}

# Function to merge a JSON file (object) into another JSON file (object) at a given JSON object path.
# Merge attempts only if both are objects.
# Returns nothing, just modifies input target_json_file
merge_json_into_path() {
  local target_json_file="$1"
  local json_object_path="$2"
  local source_json_file="$3"

  [[ -z "$target_json_file" ]] && log_error "Target JSON file not provided as argument."
  [[ ! -f "$target_json_file" ]] && log_error "File ${target_json_file} not found."
  [[ -z "$json_object_path" ]] && log_error "JSON object path of key not provided as argument."
  [[ -z "$source_json_file" ]] && log_error "Source JSON file not provided as argument."
  [[ ! -f "$source_json_file" ]] && log_error "File ${source_json_file} not found."

  log_info "Updating file ${target_json_file} at object path ${json_object_path} with JSON data from file ${source_json_file}..."

  local tmp_target_json_file
  tmp_target_json_file=$(mktemp) || log_error "Failed to create temp file for operation function '${FUNCNAME[0]}'."

  trap "rm -f '$tmp_target_json_file'" EXIT

  jq_path_array=$(convert_json_object_path_to_jq_path_array "$json_object_path")

  local source_json
  source_json=$(jq '.' "$source_json_file") || log_error "Failed to parse source JSON"

  jq --argjson path_array "$jq_path_array" --argjson src "$source_json" '
    (
      if (getpath($path_array) | type) == "object" and ($src | type) == "object" then
        getpath($path_array) + $src
      else
        error("Merge failed: both target and source must be JSON objects")
      end
    ) as $merged_objects
    | setpath($path_array; $merged_objects)
  ' "$target_json_file" > "$tmp_target_json_file" || \
  log_error "Failed merger of JSON file ${source_json_file} into file ${target_json_file} at object path ${json_object_path}"

  # Replace original file with merged content
  mv "$tmp_target_json_file" "$target_json_file" || \
  log_error "Failed moving ${tmp_target_json_file} file contents into original ${target_json_file} post operation of function '${FUNCNAME[0]}'"
}


# ------Validation Functions------

# Function to validate common args for all scripts
# Returns boolean
validate_bucket_and_tfvars_args() {
  local tf_backend_bucket="$1"
  local tfvars_json_file_path="$2"

  log_info "Validating arguments: TF Backend bucket and .tfvars file..."

  # Check if bucket name is provided as argument
  [[ -z "$tf_backend_bucket" ]] && log_error "TF Backend bucket name was not provided as an argument."
  # Check if .tfvars file name is provided as argument
  [[ -z "$tfvars_json_file_path" ]] && log_error ".tfvars file path was not provided as an argument."
  # Check if .tfvars file exists
  [[ ! -f "$tfvars_json_file_path" ]] && log_error "File ${tfvars_json_file_path} not found."

  return 0
}

# ------Common tfstate functions------

# Function to fetch the entire tfstate and store it in a file
# Returns nothing
export_tfstate_to_file() {
  local output_tfstate_file="$1"

  [[ -z "$output_tfstate_file" ]] && log_error "Output file not provided to as argument."

  log_info "Fetching current Terraform state as JSON and exporting to file: '${output_tfstate_file}'..."
  terraform show -json > "$output_tfstate_file" || log_error "Failed to fetch terraform state as JSON."
}

# ------Workstation CLUSTER Functions------

# Function to filter existing workstation cluster details from tfstate
# Returns (output to stdout) a JSON object
get_existing_ws_cluster() {
  local ws_cluster_tfstate_json_file="$1"

  [[ -z "$ws_cluster_tfstate_json_file" ]] && log_error "WS Configs tfstate JSON file not provided to as argument."
  [[ ! -f "$ws_cluster_tfstate_json_file" ]] && log_error "File ${ws_cluster_tfstate_json_file} does not exist."

  log_info "Filtering existing WS Cluster details from tfstate JSON file '${ws_cluster_tfstate_json_file}'..."

  jq '
    .values.root_module.resources // []
    | map(select(.type == "google_workstations_workstation_cluster" and .mode == "managed"))
    | map({
        key: .name,
        value: (
          .values as $cluster |
          {
            project: ($cluster.project // null),
            location: ($cluster.location // null),
            network: ($cluster.network // null),
            sub_network: ($cluster.subnetwork // null),
          }
        )
      })
    | from_entries
  ' "${ws_cluster_tfstate_json_file}" || log_error "Failed to run jq: invalid JSON while filtering existing WS Cluster details from tfstate JSON file '${ws_cluster_tfstate_json_file}'...."
}

# Function to check if the WS Cluster exists
# Returns boolean
check_ws_cluster_exists() {
  local ws_cluster_tf_dir="$1"
  local tf_backend_bucket="$2"

  log_info "Checking if Workstation Cluster exists..."

  # Change directory temporarily to WS Cluster terraform
  pushd "$ws_cluster_tf_dir" > /dev/null || log_error "Cannot cd to ${ws_cluster_tf_dir}"

  run_terraform_init "$tf_backend_bucket"

  local ws_cluster_tfstate_json_file="ws_cluster_tfstate.json"
  local existing_ws_cluster_json

  # Store WS Cluster tfstate in a file
  export_tfstate_to_file "$ws_cluster_tfstate_json_file"
  log_info "Exported WS Cluster tfstate JSON to file: '${ws_cluster_tfstate_json_file}'."

  # Extract existing WS Cluster details
  existing_ws_cluster_json=$(get_existing_ws_cluster "$ws_cluster_tfstate_json_file")

  # Exit WS Cluster terraform directory
  popd > /dev/null || log_error "Failed to return to the original working directory."

  if [[ -z "$existing_ws_cluster_json" || "$existing_ws_cluster_json" == "{}" ]]; then
    log_warning "Workstation Cluster does NOT exist."
    return 1
  else
    log_info "Workstation Cluster exists. Proceeding..."
    return 0
  fi
}

# ------Workstation CONFIG Functions-------

# Function to filter and combine - existing ws configs and their corresponding ws admins from tfstate
# Returns (output to stdout) a JSON object
get_existing_ws_configs_with_ws_admins() {
  local ws_configs_tfstate_json_file="$1"

  [[ -z "$ws_configs_tfstate_json_file" ]] && log_error "WS Configs tfstate JSON file not provided to as argument."
  [[ ! -f "$ws_configs_tfstate_json_file" ]] && log_error "File ${ws_configs_tfstate_json_file} does not exist."

  log_info "Filtering existing WS Configs and their corresponding WS Admin members from tfstate JSON file '${ws_configs_tfstate_json_file}'..."

  jq '
    (.values.root_module.resources // []) as $resources
    | (
        $resources
        | map(select(.type == "google_workstations_workstation_config_iam_binding"))
        | map({
            key: .index,
            value: (
              (.values.members // [])
              | map(sub("^user:";""))
            )
          })
        | from_entries
      ) as $iam_map
    | $resources
    | map(select(.type == "google_workstations_workstation_config" and .mode == "managed"))
    | map({
        key: .index,
        value: (
          .values as $config |
          {
            ws_idle_timeout: ($config.idle_timeout | sub("s$"; "") | tonumber),
            ws_running_timeout: ($config.running_timeout | sub("s$"; "") | tonumber),
            ws_replica_zones: ($config.replica_zones // []),
            host_machine_type: ($config.host[0]?.gce_instance[0]?.machine_type // null),
            host_quickstart_pool_size: ($config.host[0]?.gce_instance[0]?.pool_size // null),
            host_boot_disk_size_gb: ($config.host[0]?.gce_instance[0]?.boot_disk_size_gb // null),
            host_disable_public_ip_addresses: ($config.host[0]?.gce_instance[0]?.disable_public_ip_addresses // null),
            host_disable_ssh: ($config.host[0]?.gce_instance[0]?.disable_ssh // null),
            host_enable_nested_virtualization: ($config.host[0]?.gce_instance[0]?.enable_nested_virtualization // null),
            pd_required: ((($config.persistent_directories // []) | length) > 0),
            pd_mount_path: (($config.persistent_directories // [])[0]?.mount_path // null),
            pd_fs_type: (($config.persistent_directories // [])[0]?.gce_pd[0]?.fs_type // null),
            pd_disk_type: (($config.persistent_directories // [])[0]?.gce_pd[0]?.disk_type // null),
            pd_size_gb: (($config.persistent_directories // [])[0]?.gce_pd[0]?.size_gb // null),
            pd_reclaim_policy: (($config.persistent_directories // [])[0]?.gce_pd[0]?.reclaim_policy // null),
            pd_source_snapshot: (
              (($config.persistent_directories // [])[0]?.gce_pd[0]?.source_snapshot) as $snap
              | if $snap and $snap != "" then ($snap | sub("^.*snapshots/"; "")) else null end
            ),
            ed_required: ((($config.ephemeral_directories // []) | length) > 0),
            ed_mount_path: (($config.ephemeral_directories // [])[0]?.mount_path // null),
            ed_disk_type: (($config.ephemeral_directories // [])[0]?.gce_pd[0]?.disk_type // null),
            ed_source_snapshot: (
              (($config.ephemeral_directories // [])[0]?.gce_pd[0]?.source_snapshot) as $esnap
              | if $esnap and $esnap != "" then $esnap else null end
            ),
            ed_source_image: (
              (($config.ephemeral_directories // [])[0]?.gce_pd[0]?.source_image) as $eimg
              | if $eimg and $eimg != "" then $eimg else null end
            ),
            ed_read_only: (($config.ephemeral_directories // [])[0]?.gce_pd[0]?.read_only // null),
            container_image: ($config.container[0]?.image // null),
            container_entrypoint_commands: ($config.container[0]?.command // []),
            container_entrypoint_args: ($config.container[0]?.args // []),
            container_working_dir: (
              $config.container[0]?.working_dir as $wd
              | if $wd and $wd != "" then $wd else null end
            ),
            container_env_vars: ($config.container[0]?.env // {}),
            container_user: (
              $config.container[0]?.run_as_user as $u | if $u == 0 then null else $u end
            ),
            ws_allowed_ports: ($config.allowed_ports // []),
            ws_admin_iam_members: (
              ($iam_map[.index] // [])
              | unique
            )
          }
        )
      })
    | from_entries
  ' "${ws_configs_tfstate_json_file}" || log_error "Failed to run jq: invalid JSON while filtering existing WS Configs and their corresponding WS Admin members from tfstate JSON file '${ws_configs_tfstate_json_file}'..."
}


# ------WORKSTATION Functions-------

# Function to filter list of existing workstations from tfstate
# Returns (output to stdout) a JSON object
get_existing_workstations_with_ws_users() {
  local workstations_tfstate_json_file="$1"

  [[ -z "$workstations_tfstate_json_file" ]] && log_error "Workstations tfstate JSON file not provided to as argument."
  [[ ! -f "$workstations_tfstate_json_file" ]] && log_error "File ${workstations_tfstate_json_file} does not exist."

  log_info "Filtering list of existing Workstations and their corresponding WS User members from tfstate JSON file '${workstations_tfstate_json_file}'..."

  jq '
    (.values.root_module.resources // []) as $resources
    | (
        $resources
        | map(select(.type == "google_workstations_workstation_iam_binding"))
        | map({
            key: .index,
            value: (
              (.values.members // [])
              | map(sub("^user:";""))
            )
          })
        | from_entries
      ) as $iam_map
    | $resources
    | map(select(.type == "google_workstations_workstation" and .mode == "managed"))
    | map({
        key: .index,
        value: (
          .values as $workstation |
          {
            ws_config_name: ($workstation.workstation_config_id // null),
            ws_name: ($workstation.workstation_id // null),
            ws_display_name: ($workstation.display_name // null),
            ws_url: (if $workstation.host == null then null else "https://80-\($workstation.host)" end),
            ws_user_iam_members: (
              ($iam_map[.index] // [])
              | unique
            )
          }
        )
      })
    | from_entries
  ' "${workstations_tfstate_json_file}" || log_error "Failed to run jq: invalid JSON while filtering list of existing Workstations and their corresponding WS User members from tfstate JSON file '${workstations_tfstate_json_file}'..."
}

# Function to filter list of existing workstations for a specific user from existing workstations
# Returns (output to stdout) a JSON object
get_existing_workstations_for_user() {
  local workstations_tfstate_json_file="$1"
  local workstation_user="$2"

  [[ -z "$workstations_tfstate_json_file" ]] && log_error "Workstations tfstate JSON file not provided to as argument."
  [[ ! -f "$workstations_tfstate_json_file" ]] && log_error "File ${workstations_tfstate_json_file} does not exist."
  [[ -z "$workstation_user" ]] && log_error "Workstation user email not provided to as argument."

  # Fetch all existing workstations and their corresponding ws users
  local existing_workstations_with_ws_users_json_file
  existing_workstations_with_ws_users_json_file=$(mktemp) || log_error "Failed to create temp file for existing workstations"

  trap "rm -f '$existing_workstations_with_ws_users_json_file'" EXIT

  get_existing_workstations_with_ws_users "$workstations_tfstate_json_file" > "$existing_workstations_with_ws_users_json_file"

  log_info "Filtering list of existing Workstations for a specific user from tfstate JSON file '${workstations_tfstate_json_file}'..."

  # Filter workstations only for the input workstation_user
  jq --arg workstation_user "$workstation_user" '
    to_entries
    | map(select(.value.ws_user_iam_members | index($workstation_user)))
    | from_entries
  ' "$existing_workstations_with_ws_users_json_file" || log_error "Failed to run jq: invalid JSON while filtering list of existing Workstations for specific user '${workstation_user}' from tfstate JSON file '${workstations_tfstate_json_file}'..."
}

# Function to fetch current workstation state via gcloud
# Returns (output to stdout) a string
get_current_workstation_state() {
  local workstation="$1"
  local workstation_config="$2"
  local workstation_cluster="$3"
  local workstation_region="$4"

  log_info "Fetching current state of Workstation '${workstation}' from GCP using gcloud..."

  state=$(
    gcloud workstations describe "${workstation}"\
      --config="${workstation_config}"\
      --cluster="${workstation_cluster}"\
      --region="${workstation_region}"\
      --format="value(state)"
  )

  if [[ -z "$state" || "$state" == "null" ]]; then
    log_error "Failed to fetch state of Workstation '${workstation}' from GCP using gcloud."
  fi

  echo "$state"
}

# Function to fetch URL of the workstation via gcloud
# Returns (output to stdout) a string
get_workstation_url() {
  local workstation="$1"
  local workstation_config="$2"
  local workstation_cluster="$3"
  local workstation_region="$4"

  log_info "Fetching URL of Workstation '${workstation}' from GCP using gcloud..."

  workstation_url=$(
    gcloud workstations describe "${workstation}"\
      --config="${workstation_config}"\
      --cluster="${workstation_cluster}"\
      --region="${workstation_region}"\
      --format="value(host)"
  )

  if [[ -z "$workstation_url" || "$workstation_url" == "null" ]]; then
    log_error "Failed to fetch the URL of the Workstation $workstation_url from GCP using gcloud."
  fi

  echo "$workstation_url"
}

validate_workstation_state() {
    local target_workstation="$1"

    # Extract state directly from Terraform state
    state=$(terraform show -json | jq -r --arg ws "$target_workstation" '
    .values.root_module.resources[]
    | select(.type == "google_workstations_workstation" and .mode == "managed" and .index == $ws)
    | .values.state
    ')

    if [[ -z "$state" || "$state" == "null" ]]; then
        log_error "Workstation $target_workstation not found in Terraform state."
    fi

    if [[ "$state" != "STATE_STOPPED" ]]; then
        log_error "Workstation $target_workstation is in state $state; expected STATE_STOPPED"
    fi

    log_success "Workstation $target_workstation is in expected state: $state"
}

# Fetch remote Terraform state and construct a tfvars JSON
get_existing_workstations() {
  terraform show -json | jq '
    (.values.root_module.resources // []) as $resources
    | (
        $resources
        | map(select(.type == "google_workstations_workstation_iam_binding"))
        | map({
            key: .index,
            value: (
              (.values.members // [])
              | map(sub("^user:";""))
            )
          })
        | from_entries
      ) as $iam_map
    | $resources
    | map(select(.type == "google_workstations_workstation" and .mode == "managed"))
    | map({
        key: .index,
        value: (
          .values as $workstation |
          {
            sdv_cloud_ws_workstation_config_id: ($workstation.workstation_config_id // null),
            sdv_cloud_ws_workstation_id: ($workstation.workstation_id // null),
            sdv_cloud_ws_display_name: ($workstation.display_name // null),
            sdv_cloud_ws_user_emails: (
              ($iam_map[.index] // [])
              | unique
            )
          }
        )
      })
    | from_entries
  '
}

# List detailed workstations - returns plain JSON
# Usage: list_detailed_workstations [ws_regex] [config_regex] [user_regex]
list_detailed_workstations() {
    local ws_regex="${1:-.*}"
    local config_regex="${2:-.*}"
    local user_regex="${3:-.*}"

    # Get base workstation data from terraform
    local base_cloud_ws_workstation_data
    base_cloud_ws_workstation_data=$(terraform show -json | jq --arg ws_re "$ws_regex" --arg cfg_re "$config_regex" --arg usr_re "$user_regex" '
        (.values.root_module.resources // []) as $resources
        |
        (
            $resources
            | map(select(.type == "google_workstations_workstation_iam_binding"))
            | map({
                key: .index,
                value: ((.values.members // []) | map(sub("^user:";"")) | unique)
            })
            | from_entries
        ) as $iam_map
        |
        $resources
        | map(select(.type == "google_workstations_workstation" and .mode == "managed"))
        | map(select(.index | test($ws_re)))
        | map(select(.values.workstation_config_id | test($cfg_re)))
        | map(select(
            ($iam_map[.index] // []) as $users
            | if ($usr_re == "" or $usr_re == ".*") then
                true
              else
                ($users | length > 0) and ($users | any(test($usr_re)))
              end
        ))
        | map({
            workstation_id: .index,
            display_name: .values.display_name,
            workstation_config_id: .values.workstation_config_id,
            project: .values.project,
            location: .values.location,
            workstation_cluster_id: .values.workstation_cluster_id,
            state: .values.state,
            create_time: .values.create_time,
            host: .values.host,
            uid: .values.uid,
            user_emails: ($iam_map[.index] // [])
        })
    ')

    if [[ $(echo "$base_cloud_ws_workstation_data" | jq 'if type=="array" then length else 0 end') -eq 0 ]]; then
      echo "[]"
      return 0
    fi

    # If base_cloud_ws_workstation_data is empty, return empty array
    [[ -z "$base_cloud_ws_workstation_data" ]] && echo "[]" && return 0

    # Fetch timeout data for unique configs (build list of unique keys)
    local tmp_timeout
    tmp_timeout=$(mktemp) || { log_error "Failed to create temp file"; return 1; }

    while IFS=$'\t' read -r project location cluster config; do
        # call gcloud and protect jq in case gcloud produced nothing
        local gcloud_cloud_ws_config_json_output
        gcloud_cloud_ws_config_json_output=$(gcloud workstations configs describe "$config" \
            --project="$project" \
            --region="$location" \
            --cluster="$cluster" \
            --format='json' 2>/dev/null || true)

        if [[ -z "$gcloud_cloud_ws_config_json_output" ]]; then
            # produce default object when describe fails or returns empty
            jq -n --arg key "${project}_${location}_${cluster}_${config}" '{
                config_key: $key,
                idle_timeout: "0s",
                running_timeout: "0s"
            }' >> "$tmp_timeout"
        else
            # parse actual gcloud JSON
            echo "$gcloud_cloud_ws_config_json_output" | jq --arg key "${project}_${location}_${cluster}_${config}" '{
                config_key: $key,
                idle_timeout: (.idleTimeout // "0s"),
                running_timeout: (.runningTimeout // "0s")
            }' >> "$tmp_timeout"
        fi
    done < <(echo "$base_cloud_ws_workstation_data" | jq -r '.[] | [.project, .location, .workstation_cluster_id, .workstation_config_id] | @tsv' | sort -u)

    # Combine timeout entries into a single JSON object
    jq -s 'map({(.config_key): {idle_timeout, running_timeout}}) | add' "$tmp_timeout" > "${tmp_timeout}.json"

    # Merge timeout data with base data (use empty defaults if not found)
    jq --slurpfile timeouts "${tmp_timeout}.json" '
        map(. + ($timeouts[0][(.project + "_" + .location + "_" + .workstation_cluster_id + "_" + .workstation_config_id)] // {idle_timeout: "0s", running_timeout: "0s"}))
    ' <(echo "$base_cloud_ws_workstation_data")

    rm -f "$tmp_timeout" "${tmp_timeout}.json"
}


# ------Terraform workflow Functions-------

export TF_IN_AUTOMATION=1

run_terraform_init() {
  local backend_bucket=$1

  log_info "Initializing Terraform..."
  terraform init -backend-config="bucket=${backend_bucket}" || log_error "Terraform init failed"
}

# Check if the cluster is already deleted
run_terraform_empty_state_check() {
    log_info "Checking current Terraform state contents..."

    if [ -z "$(terraform state list)" ]; then
      log_error "No resources found in state. Nothing to destroy."
    fi
}

run_terraform_apply() {
  local tfvars_file=$1

  log_info "Applying changes..."
  terraform apply -auto-approve -var-file="${tfvars_file}" || log_error "Terraform apply failed."
}

run_terraform_destroy() {
  local tfvars_file=$1
  log_info "Running Terraform destroy..."
  terraform destroy -auto-approve -var-file="${tfvars_file}" || log_error "Terraform destroy failed."
}
