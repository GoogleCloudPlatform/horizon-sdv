#!/usr/bin/env bash

# Copyright (c) 2026 Accenture, All Rights Reserved.
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

# ------Global Variables------
GIT_LOCK_ERR_PATTERN="cannot lock ref 'refs/heads/.*.lock': File exists" # Pattern to identify git lock file errors
RETRY_DELAY_SECONDS=60 # Delay between retries in seconds

# ------Logging helper functions-------

# Function to print a header message
# Returns void
print_header() {
  echo
  echo "┌────────────────────────────────────────────────────────────────┐"
  printf "│ %-62s │\n" "$1"
  echo "└────────────────────────────────────────────────────────────────┘"
}

# Function to print result messages with indentation
# Returns void
print_result() {
  echo "┌─RESULT"
  # Read and indent lines from stdin preserving colors and whitespace
  while IFS= read -r line || [[ -n $line ]]; do
    printf '    %s\n' "$line"
  done
  echo "└─"
}

# Logging functions with different severity levels
log_info() { echo -e "\n [INFO] $1 \n" >&2; }
log_success() { echo -e "\n\u001B[32m [SUCCESS] $1 \u001B[0m\n" >&2; }
log_warning() { echo -e "\n\u001B[33m [WARNING] $1 \u001B[0m\n" >&2; }
log_error() { echo -e "\n\u001B[31m [ERROR] $1\u001B[0m" >&2; exit 1; } # Exits with status 1

# Function to check for missing function arguments
# Exits with error if any argument is missing
check_missing_func_args() {
  local missing_args=()
  for arg_name in "$@"; do
    if [[ -z "${!arg_name}" || "${!arg_name}" == "null" ]]; then
      missing_args+=("$arg_name")
    fi
  done

  if (( ${#missing_args[@]} )); then
    log_error "Missing required function arguments: ${missing_args[*]}"
  fi
}

# Function to calculate and return formatted elapsed time
# Returns formatted elapsed time string
get_formatted_elapsed_time() {
  local start_time_in_seconds=$1
  local end_time_in_seconds=$2
  check_missing_func_args start_time_in_seconds end_time_in_seconds

  local elapsed_time_in_seconds=$((end_time_in_seconds - start_time_in_seconds))
  local hours=$((elapsed_time_in_seconds / 3600))
  local remaining_seconds=$((elapsed_time_in_seconds % 3600))
  local minutes=$((remaining_seconds / 60))
  local seconds=$((remaining_seconds % 60))
  local formatted_elapsed_time
  formatted_elapsed_time=$(printf '%02dh %02dm %02ds' $hours $minutes $seconds)

  echo "$formatted_elapsed_time"
}

# ------Kubernetes helper functions-------

# Function to check if the Mirror PVC exists
# Returns boolean
check_aosp_mirror_pvc_exists() {
  local pvc_name=$1
  local namespace=$2
  check_missing_func_args pvc_name namespace

  log_info "Checking if PVC '${pvc_name}' exists in namespace '${namespace}'..."
  
  if ! kubectl get pvc "$pvc_name" -n "$namespace" &> /dev/null; then
    log_warning "PVC '${pvc_name}' not found in namespace '${namespace}'."
    return 1
  fi

  return 0
}

# Function to get storage info for Mirror PVC
# Returns void; exit 1 on failure
get_aosp_mirror_pvc_storage_info() {
  local mirror_pvc_mount_path_in_container=$1
  check_missing_func_args mirror_pvc_mount_path_in_container

  log_info "Fetching storage info for mirror PVC mounted at '${mirror_pvc_mount_path_in_container}'..."

  df "${mirror_pvc_mount_path_in_container}" -h || log_warning "Failed to fetch storage info for mirror PVC mounted at '${mirror_pvc_mount_path_in_container}'."
}

# ------Validation and Metadata Functions------

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

# Function to check if given file exists
# Returns boolean
check_file_exists() {
  local file_path=$1
  check_missing_func_args file_path

  local file_name
  file_name=$(basename "$file_path")

  log_info "Checking if file '${file_name}' exists at path '${file_path}'..."

  if [[ ! -f "$file_path" ]]; then
    log_warning "File '${file_name}' NOT found at path '${file_path}'."
    return 1
  fi

  return 0
}

# Function to check if given directory exists
# Returns boolean
check_directory_exists() {
  local dir_path=$1
  check_missing_func_args dir_path

  local dir_name
  dir_name=$(basename "$dir_path")

  log_info "Checking if directory '${dir_name}' exists at path '${dir_path}'..."

  if [[ ! -d "$dir_path" ]]; then
    log_warning "Directory '${dir_name}' NOT found at path '${dir_path}'."
    return 1
  fi

  return 0
}

# Function to create new directory
# Returns void; exit 1 on failure
create_directory() {
  local dir_path=$1
  check_missing_func_args dir_path

  local dir_name
  dir_name=$(basename "$dir_path")

  log_info "Creating new directory '${dir_name}' at path '${dir_path}'..."

  mkdir -p "${dir_path}" || log_error "Failed to create directory at path '${dir_path}'."

  log_success "Created new directory '${dir_name}' at path '${dir_path}'."
}

# Function to create a new metadata file with just root key
# Returns void; exit 1 on failure
create_metadata_file_with_root_key() {
  local metadata_file_path=$1
  local root_key=$2
  check_missing_func_args metadata_file_path root_key

  log_info "Creating new metadata file at path '${metadata_file_path}'..."

  echo "${root_key}: {}" > "${metadata_file_path}" || log_error "Failed to create metadata file at path '${metadata_file_path}'."

  log_success "Created new metadata file at path '${metadata_file_path}'."
}

# Function to check if nested key for mirror exists in metadata file
# Returns boolean
check_mirror_metadata_entry_exists() {
  local metadata_file_path=$1
  local root_key=$2
  local mirror_dir_name=$3
  check_missing_func_args metadata_file_path root_key mirror_dir_name

  log_info "Checking if metadata entry exists for mirror directory '$mirror_dir_name'..."

  local exists
  exists=$(yq ".${root_key} | has(\"${mirror_dir_name}\")" "${metadata_file_path}")
  if [[ "$exists" != "true" ]]; then
    log_warning "Metadata entry NOT found for mirror directory '$mirror_dir_name'."
    return 1
  fi

  return 0
}

# Function to get all mirror directory names from metadata file
# Returns a space-separated list of mirror directory names
get_mirror_list_from_metadata() {
  local metadata_file_path=$1
  local root_key=$2
  check_missing_func_args metadata_file_path root_key

  log_info "Fetching mirror directory names from metadata file '${metadata_file_path}'..."

  local mirror_list
  mirror_list=$(yq ".${root_key} | keys | .[]" "${metadata_file_path}") || log_error "Failed to fetch mirror directory names from metadata file '${metadata_file_path}'."

  echo "$mirror_list"
}

# Function to get a nested value from metadata file
# Returns the value or an error message
get_value_from_metadata() {
  local metadata_file_path=$1
  local root_key=$2
  local mirror_dir_name=$3
  local nested_key=$4
  check_missing_func_args metadata_file_path root_key mirror_dir_name nested_key

  log_info "Fetching value for '${nested_key}' from mirror directory '${mirror_dir_name}' in metadata file '${metadata_file_path}'..."

  local value
  value=$(yq ".${root_key}.${mirror_dir_name}.${nested_key}" "${metadata_file_path}") || log_error "Failed to fetch value for '${nested_key}' from mirror directory '${mirror_dir_name}' in metadata file '${metadata_file_path}'."

  echo "$value"
}

# Function to get entire metadata file contents
# Returns the contents or an error message
get_metadata_file_contents() {
  local metadata_file_path=$1
  check_missing_func_args metadata_file_path

  log_info "Fetching entire contents of metadata file '${metadata_file_path}'..."

  yq -C . "${metadata_file_path}" || log_error "Failed to fetch entire contents of metadata file '${metadata_file_path}'."
}


# Function to insert new mirror details into metadata file
# Returns void; exit 1 on failure
insert_new_mirror_metadata_entry() {
  local metadata_file_path=$1
  local root_key=$2
  local mirror_dir_name=$3
  local manifest_url=$4
  local manifest_ref=$5
  local manifest_file=$6
  local repo_sync_jobs=$7
  local build_user=$8
  check_missing_func_args metadata_file_path root_key mirror_dir_name manifest_url manifest_ref manifest_file repo_sync_jobs build_user

  log_info "Inserting new metadata entry for mirror directory '${mirror_dir_name}'..."

  # Use yq to insert new mirror details under the root key
  yq "
    .${root_key}.${mirror_dir_name}.name = \"${mirror_dir_name}\" |
    .${root_key}.${mirror_dir_name}.type = \"repo\" |
    .${root_key}.${mirror_dir_name}.manifest_url = \"${manifest_url}\" |
    .${root_key}.${mirror_dir_name}.manifest_ref = \"${manifest_ref}\" |
    .${root_key}.${mirror_dir_name}.manifest_file = \"${manifest_file}\" |
    .${root_key}.${mirror_dir_name}.repo_sync_jobs = \"${repo_sync_jobs}\" |
    .${root_key}.${mirror_dir_name}.status = \"uninitialized\" |
    .${root_key}.${mirror_dir_name}.last_successful_sync_time = \"\" |
    .${root_key}.${mirror_dir_name}.created_by = \"${build_user}\"
  " --inplace "$metadata_file_path" || log_error "Failed to insert new mirror metadata entry into file '${metadata_file_path}'."

  log_success "Inserted new mirror metadata entry for mirror directory '${mirror_dir_name}' into file '${metadata_file_path}'."
}

# Function to check if input manifest URL matches existing manifest URL in metadata file
# Returns boolean
match_mirror_manifest_url_in_metadata() {
  local metadata_file_path=$1
  local root_key=$2
  local mirror_dir_name=$3
  local input_manifest_url=$4
  local existing_manifest_url=""
  check_missing_func_args metadata_file_path root_key mirror_dir_name input_manifest_url

  log_info "Checking if input manifest URL '$input_manifest_url' matches the existing manifest URL for mirror directory '${mirror_dir_name}'..."

  existing_manifest_url=$(yq ".${root_key}.${mirror_dir_name}.manifest_url" "${metadata_file_path}") || log_error "Failed to read manifest URL from metadata file '${metadata_file_path}'."

  if [[ "$existing_manifest_url" != "$input_manifest_url" ]]; then
    log_warning "Input Manifest URL '${input_manifest_url}' does NOT match with Existing Manifest URL '${existing_manifest_url}' for mirror directory '${mirror_dir_name}'."
    return 1
  fi

  log_info "Input Manifest URL '${input_manifest_url}' matches Existing Manifest URL '${existing_manifest_url}' for mirror directory '${mirror_dir_name}'."
  return 0
}

# Function to update nested value in metadata file
# Returns void; exit 1 on failure
update_mirror_metadata_value() {
  local metadata_file_path=$1
  local root_key=$2
  local mirror_dir_name=$3
  local nested_key=$4
  local new_value=$5
  check_missing_func_args metadata_file_path root_key mirror_dir_name nested_key new_value

  log_info "Updating value for '${nested_key}' to '${new_value}' for mirror directory '${mirror_dir_name}' in metadata file '${metadata_file_path}'..."

  # Check if given mirror directory exists in metadata file
  if ! check_mirror_metadata_entry_exists "${metadata_file_path}" "${root_key}" "${mirror_dir_name}"; then
    log_error "Cannot update value. Mirror directory '${mirror_dir_name}' does not exist in metadata file '${metadata_file_path}'."
  fi

  # Update the nested key with new value in metadata file
  yq "
    .${root_key}.${mirror_dir_name}.${nested_key} = \"$new_value\"
  " --inplace "${metadata_file_path}" || log_error "Failed to update value for '${nested_key}' in metadata file '${metadata_file_path}'."

  log_info "Updated value for '${nested_key}' to '${new_value}' for mirror directory '${mirror_dir_name}' in metadata file '${metadata_file_path}'."
}

# Function to delete mirror directory
# Returns void; exit 1 on failure
delete_mirror_directory() {
  local mirror_dir_full_path=$1
  check_missing_func_args mirror_dir_full_path

  local mirror_dir_name
  mirror_dir_name=$(basename "$mirror_dir_full_path")

  log_info "Deleting mirror directory '${mirror_dir_name}' at path '${mirror_dir_full_path}'..."

  rm -rf "${mirror_dir_full_path}" || log_error "Failed to delete mirror directory: '${mirror_dir_full_path}'"

  log_success "Successfully deleted mirror directory: '${mirror_dir_full_path}'"
}

# Function to delete mirror entry from metadata file
# Returns void; exit 1 on failure
delete_mirror_entry_from_metadata() {
  local metadata_file_path=$1
  local root_key=$2
  local mirror_dir_name=$3
  check_missing_func_args metadata_file_path root_key mirror_dir_name

  # Check if given mirror directory exists in metadata file
  if ! check_mirror_metadata_entry_exists "${metadata_file_path}" "${root_key}" "${mirror_dir_name}"; then
    log_error "Cannot delete mirror entry. Mirror directory '${mirror_dir_name}' does not exist in metadata file '${metadata_file_path}'."
  fi

  log_info "Deleting entry for directory '${mirror_dir_name}' from metadata file '${metadata_file_path}'..."

  yq "
    del(.${root_key}.${mirror_dir_name})
  " --inplace "${metadata_file_path}" || log_error "Failed to delete entry for directory '${mirror_dir_name}' from metadata file '${metadata_file_path}'."

  log_success "Deleted entry for directory '${mirror_dir_name}' from metadata file '${metadata_file_path}'."
}

# ------Repo helper functions-------

# Function to remove stale git lock files
# Returns void; exit 1 on failure
remove_stale_git_locks() {
  local mirror_path=${1:-"."} # Default to current directory if no argument is provided

  log_info "Starting git lock file cleanup from any previous runs..."

  find "${mirror_path}" -name "*.lock" -print -delete || log_warning "Failed to cleanup some lock files, proceeding with sync."
}

# Function to initialize a new repo on local with mirror manifest
# Returns void; exit 1 on failure
initialise_new_repo() {
  local mirror_path=${1:-"."} # Default to current directory if no argument is provided
  local manifest_url=$2
  local manifest_ref=$3
  local manifest_file=$4
  check_missing_func_args mirror_path manifest_url manifest_ref manifest_file

  log_info "Initializing new repo inside '${mirror_path}' with details:\n Manifest URL:'${manifest_url}'\n Manifest Ref:'${manifest_ref}'\n Manifest File:'${manifest_file}'..."

  cd "${mirror_path}" || log_error "Failed to cd into mirror path '${mirror_path}'."

  repo init \
  -u "${manifest_url}" \
  -b "${manifest_ref}" \
  -m "${manifest_file}" \
  --mirror || log_error "Failed to initialize repo with manifest at '${manifest_url}'."

  log_info "Removing unnecessary refs..."
  repo forall -c "git for-each-ref --format '%(refname)' refs/changes/ | xargs -n1 git update-ref -d" || log_error "Failed to remove unnecessary refs"
}

# Function to perform repo sync with Google's source
# Returns 0 on success; 1 on failure (exits on critical errors only)
sync_mirror() {
  local mirror_path=${1:-"."} # Default to current directory if no argument is provided
  local manifest_url=$2
  local manifest_ref=$3
  local manifest_file=$4
  local repo_sync_jobs=$5
  local operation_type=${6:-"updated"} # Default to "updated" if no argument is provided
  check_missing_func_args mirror_path manifest_url manifest_ref manifest_file repo_sync_jobs operation_type

  local start_time_in_seconds
  start_time_in_seconds=$(date +%s)
  local end_time_in_seconds
  local formatted_elapsed_time

  # Ensure parallel sync jobs are at least 1, and not more than nproc value
  local jobs=$repo_sync_jobs
  local max_jobs
  max_jobs=$(nproc)
  if (( jobs < 1 )); then
    jobs=1
  elif (( jobs > max_jobs )); then
    log_info "Input 'repo_sync_jobs' is higher than the machine's ${max_jobs} cores. Reducing it to ${max_jobs} value for optimal performance."
    jobs=$max_jobs
  fi

  log_info "Starting repo sync inside '${mirror_path}' with details:\n Manifest URL:'${manifest_url}'\n Manifest Ref:'${manifest_ref}'\n Manifest File:'${manifest_file}'\n Parallel jobs: ${jobs}\n Sync started at [$(date)]..."

  cd "${mirror_path}" || log_error "Failed to cd into mirror path '${mirror_path}'."

  repo sync \
    -j"${jobs}" \
    --optimized-fetch \
    --prune \
    --retry-fetches=3 \
    --auto-gc \
    --no-clone-bundle

  local sync_status=$?

  end_time_in_seconds=$(date +%s)
  formatted_elapsed_time=$(get_formatted_elapsed_time "$start_time_in_seconds" "$end_time_in_seconds")

  if [[ $sync_status -ne 0 ]]; then
    log_warning "Failed to perform repo sync. Time elapsed: [${formatted_elapsed_time}]"
    return 1
  fi

  log_success "Repo sync completed at [$(date)].\n Time elapsed: [${formatted_elapsed_time}].\n Local mirror ${operation_type}."

  return 0

  log_info "Performing aggressive garbage collection..."
  repo forall -c "git gc --aggressive --prune=all" || log_error "Failed to perform garbage collection post repo sync."
}

# Function to sync mirror with retries (and handling git lock errors)
# Returns 0 on success; 1 on failure (exits on critical errors only)
sync_mirror_with_retries() {
  local mirror_path=${1:-"."} # Default to current directory if no argument is provided
  local manifest_url=$2
  local manifest_ref=$3
  local manifest_file=$4
  local repo_sync_jobs=$5
  local operation_type=${6:-"updated"} # created/updated; Default to "updated" if no argument is provided
  local metadata_file_path="$7"
  local metadata_root_key="$8"
  check_missing_func_args mirror_path manifest_url manifest_ref manifest_file repo_sync_jobs operation_type metadata_file_path metadata_root_key

  local mirror_dir_name
  mirror_dir_name=$(basename "$mirror_path")
  local log_file="/tmp/sync_mirror_${mirror_dir_name}.log"
  local attempt=1
  local max_retries=3

  # Update metadata file with latest mirror details before starting sync
  update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "manifest_ref" "${manifest_ref}"
  update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "manifest_file" "${manifest_file}"
  update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "repo_sync_jobs" "${repo_sync_jobs}"

  while [[ $attempt -le $max_retries ]]; do
    log_info "Repo sync attempt ${attempt} of ${max_retries}..."
    # Before starting sync, update status to 'syncing' in metadata file
    update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "status" "syncing"

    # Tee the output to the log file for post-mortem analysis
    if sync_mirror "${mirror_path}" "${manifest_url}" "${manifest_ref}" "${manifest_file}" "${repo_sync_jobs}" "${operation_type}" | tee "${log_file}"; then
      # SUCCESS: Update metadata and return successfully
      update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "status" "ready"
      update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "last_successful_sync_time" "$(date)"

      log_success "Repo sync succeeded for '${mirror_dir_name}' on attempt ${attempt}."
      return 0
    else
      log_warning "Repo sync failed on attempt ${attempt}."
      update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "status" "error"

      # Check if this is the final attempt
      if (( attempt >= max_retries )); then
        log_warning "Max retries reached. Final sync failure.\n Repo sync failed after ${max_retries} attempts."
        return 1
      fi

      # Potential remediation
      # Check for Git lock error
      if grep -qE "${GIT_LOCK_ERR_PATTERN}" "${log_file}"; then
        log_info "Detected Git lock error. Removing stale git lock files (expected to take 20-30 mins)..."
        remove_stale_git_locks "${mirror_path}"
        ((attempt+=1))
        continue
      fi

      # Reduce parallel jobs to 3 for second attempt to avoid potential rate-limiting (when syncing AOSP from Google)
      if (( attempt + 1 == 2 )); then
        log_info "Reducing parallel sync jobs to 3 for second attempt."
        repo_sync_jobs=3
        update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "repo_sync_jobs" "${repo_sync_jobs}"
      # Reduce parallel jobs to 1 for third and last attempt to avoid potential rate-limiting (when syncing AOSP from Google)
      elif (( attempt + 1 == max_retries )); then
        log_info "Reducing parallel sync jobs to 1 for last attempt."
        repo_sync_jobs=1
        update_mirror_metadata_value "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "repo_sync_jobs" "${repo_sync_jobs}"
      fi

      # Standard Delay and Retry
      log_info "Waiting ${RETRY_DELAY_SECONDS} seconds before next retry..."
      sleep "${RETRY_DELAY_SECONDS}"

      ((attempt+=1))
    fi
  done
}


# ------Terraform workflow Functions-------

export TF_IN_AUTOMATION=1

# Function to initialize terraform
# Returns void; exit 1 on failure
run_terraform_init() {
  local backend_bucket=$1
  check_missing_func_args backend_bucket

  log_info "Initializing Terraform..."
  terraform init -backend-config="bucket=${backend_bucket}" || log_error "Terraform init failed"
}

# Function to apply terraform changes
# Returns void; exit 1 on failure
run_terraform_apply() {
  local tfvars_file=$1
  check_missing_func_args tfvars_file

  log_info "Applying changes..."
  terraform apply -auto-approve -var-file="${tfvars_file}" || log_error "Terraform apply failed."
}

# Function to destroy terraform-managed infrastructure
# Returns void; exit 1 on failure
run_terraform_destroy() {
  local tfvars_file=$1
  check_missing_func_args tfvars_file

  log_info "Running Terraform destroy..."
  terraform destroy -auto-approve -var-file="${tfvars_file}" || log_error "Terraform destroy failed."
}
