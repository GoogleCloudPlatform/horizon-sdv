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

# Capture the arguments passed to the script
MIRROR_ROOT_SUBDIR_PATH="$1"
SYNC_ALL_EXISTING_MIRRORS="$2"
MIRROR_DIR="$3"
MIRROR_MANIFEST_URL="$4"
MIRROR_MANIFEST_REF="$5"
MIRROR_MANIFEST_FILE="$6"
REPO_SYNC_JOBS="$7"
BUILD_USER="$8"

METADATA_FILE_NAME="metadata.yaml"
METADATA_FILE_FULL_PATH="${MIRROR_ROOT_SUBDIR_PATH}/${METADATA_FILE_NAME}"
MIRROR_DIR_FULL_PATH="${MIRROR_ROOT_SUBDIR_PATH}/${MIRROR_DIR}" # for single mirror sync

METADATA_FILE_ROOT_KEY=$(basename "${MIRROR_ROOT_SUBDIR_PATH}")

# Import shared utils
# shellcheck disable=SC1091
source "$(dirname "$0")/../utils/utils.sh"

# Function to process a single mirror sync (or creation)
# Returns 0 on success, return 1 on failure (exits on critical errors only)
process_single_mirror() {
  local mirror_dir_path="$1"
  local manifest_url="$2"
  local manifest_ref="$3"
  local manifest_file="$4"
  local repo_sync_jobs="$5"
  local build_user="$6"
  local metadata_file_path="$7"
  local metadata_root_key="$8"

  local sync_type="created"
  local mirror_dir_name
  mirror_dir_name=$(basename "${mirror_dir_path}")

  print_header "MIRROR SETUP: SYNCING MIRROR ${mirror_dir_name}"

  # Create new input mirror directory, if it does not exist
  if ! check_directory_exists "${mirror_dir_path}"; then
    create_directory "${mirror_dir_path}"
  fi

  # Add new mirror details to metadata file, if entry does not exist
  if ! check_mirror_metadata_entry_exists "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}"; then
    insert_new_mirror_metadata_entry \
    "${metadata_file_path}" \
    "${metadata_root_key}" \
    "${mirror_dir_name}" \
    "${manifest_url}" \
    "${manifest_ref}" \
    "${manifest_file}" \
    "${repo_sync_jobs}" \
    "${build_user}"
  fi

  # Change directory temporarily to input mirror directory
  pushd "${mirror_dir_path}" > /dev/null || log_error "Cannot cd to ${mirror_dir_path}"

  # ------Mirror workflow begins------

  # Check if .repo directory exists
  if ! check_directory_exists "${mirror_dir_path}/.repo"; then
    initialise_new_repo "${mirror_dir_path}" "${manifest_url}" "${manifest_ref}" "${manifest_file}"
  else
    log_info "'.repo' folder found in directory '${mirror_dir_path}'. Reusing existing repo. Updating mirror with manifest at '${manifest_url}'..."

    sync_type="updated"

    # Check if manifest URL has changed;
    if ! match_mirror_manifest_url_in_metadata "${metadata_file_path}" "${metadata_root_key}" "${mirror_dir_name}" "${manifest_url}"; then
      log_error "You cannot change the manifest URL of an existing mirror.\n Please create a new mirror instead (in different directory)."
    fi
  fi

  log_info "Mirror Metadata file contents before syncing mirror '${mirror_dir_name}':"
  metadata_file_contents=$(get_metadata_file_contents "${metadata_file_path}")
  echo "${metadata_file_contents}" | print_result

  log_info "Storage info for Mirror PVC before syncing '${mirror_name}':"
  get_aosp_mirror_pvc_storage_info "${MIRROR_ROOT_SUBDIR_PATH}"

  # Sync the mirror with retries; func handles updating metadata status and returns 1 on failure
  sync_mirror_with_retries \
    "${mirror_dir_path}" \
    "${manifest_url}" \
    "${manifest_ref}" \
    "${manifest_file}" \
    "${repo_sync_jobs}" \
    "${sync_type}" \
    "${metadata_file_path}" \
    "${metadata_root_key}"

  local sync_status=$?

  log_info "Mirror Metadata file contents after syncing mirror '${mirror_dir_name}':"
  metadata_file_contents=$(get_metadata_file_contents "${metadata_file_path}")
  echo "${metadata_file_contents}" | print_result

  log_info "Storage info for Mirror PVC after syncing '${mirror_name}':"
  get_aosp_mirror_pvc_storage_info "${MIRROR_ROOT_SUBDIR_PATH}"

  popd > /dev/null || log_error "Cannot return from directory ${mirror_dir_path}"

  return $sync_status
}

# ------Common Initial Checks and Setup------

# Create mirror root subdirectory, if it does not exist
if ! check_directory_exists "${MIRROR_ROOT_SUBDIR_PATH}"; then
  create_directory "${MIRROR_ROOT_SUBDIR_PATH}"
fi

# Create metadata file for storing mirror details, if it does not exist
if ! check_file_exists "${METADATA_FILE_FULL_PATH}"; then
  log_info "This is the first mirror setup..."
  create_metadata_file_with_root_key "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}"
fi

# Check if all mirrors have to be synced; ignores parameters for new mirror
if [[ "${SYNC_ALL_EXISTING_MIRRORS}" == "true" ]]; then
  log_info "SYNC_ALL_EXISTING_MIRRORS is set to true. Syncing all existing mirrors in volume..."

  # Get list of existing mirrors from metadata
  mirror_list=$(get_mirror_list_from_metadata "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}")
  succeeded_mirrors_list=()
  success_count=0
  failed_mirrors_list=()
  failure_count=0
  total_mirrors=$(echo "$mirror_list" | wc -w)

  if [[ $total_mirrors -eq 0 ]]; then
    log_error "No existing mirrors found in metadata file. You need to have at least one mirror set up before selecting SYNC_ALL_EXISTING_MIRRORS parameter.\n Aborting sync all operation."
  fi

  # Loop through each mirror and process sync
  for mirror_name in $mirror_list; do
    mirror_dir_path="${MIRROR_ROOT_SUBDIR_PATH}/${mirror_name}"
    manifest_url=$(get_value_from_metadata "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}" "${mirror_name}" "manifest_url")
    manifest_ref=$(get_value_from_metadata "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}" "${mirror_name}" "manifest_ref")
    manifest_file=$(get_value_from_metadata "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}" "${mirror_name}" "manifest_file")
    repo_sync_jobs=$(get_value_from_metadata "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}" "${mirror_name}" "repo_sync_jobs")
    build_user=$(get_value_from_metadata "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}" "${mirror_name}" "created_by")

    if process_single_mirror \
      "${mirror_dir_path}" \
      "${manifest_url}" \
      "${manifest_ref}" \
      "${manifest_file}" \
      "${repo_sync_jobs}" \
      "${build_user}" \
      "${METADATA_FILE_FULL_PATH}" \
      "${METADATA_FILE_ROOT_KEY}" ; then
      log_success "Successfully synced mirror: '${mirror_name}'"

      succeeded_mirrors_list+=("${mirror_name}") || log_error "Failed to add mirror to succeeded list: '${mirror_name}'"
      ((success_count+=1))
    else
      log_warning "Failed to sync mirror: '${mirror_name}'."

      failed_mirrors_list+=("${mirror_name}") || log_error "Failed to add mirror to failed list: '${mirror_name}'"
      ((failure_count+=1))
    fi

    remaining_count=$((total_mirrors - success_count - failure_count))
    log_info "Progress: ${success_count}/${total_mirrors} mirrors synced successfully.\n ${failure_count} failures so far.\n ${remaining_count} mirrors remaining to sync."
  done

  {
    log_info "-----Sync All Existing Mirrors Summary-----"
    echo "Total Mirrors: ${total_mirrors}"
    echo "Successful Syncs: ${success_count}"
    echo "Failed Syncs: ${failure_count}"
    echo ""
    echo "Succeeded Mirrors: ${succeeded_mirrors_list[*]}"
    echo "Failed Mirrors: ${failed_mirrors_list[*]}"
  } | print_result

  
  if [[ ${failure_count} -gt 0 ]]; then
    log_error "Some mirrors failed to sync. Please check the logs above for details.\n Setting overall batch sync status to 'error'."
  fi
else
  # Process single mirror sync with provided parameters
  if process_single_mirror \
    "${MIRROR_DIR_FULL_PATH}" \
    "${MIRROR_MANIFEST_URL}" \
    "${MIRROR_MANIFEST_REF}" \
    "${MIRROR_MANIFEST_FILE}" \
    "${REPO_SYNC_JOBS}" \
    "${BUILD_USER}" \
    "${METADATA_FILE_FULL_PATH}" \
    "${METADATA_FILE_ROOT_KEY}" ; then
    log_success "Sync completed successfully for mirror: '${MIRROR_DIR}'"
  else
    log_error "Sync failed for mirror: '${MIRROR_DIR}'"
  fi
fi

exit 0
