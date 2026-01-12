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
MIRROR_DIR_TO_DELETE="$2"
DELETE_ENTIRE_MIRROR_SETUP="$3"
TF_BACKEND_BUCKET="$4"
MIRROR_TFVARS_JSON_FILE_PATH="$5"

METADATA_FILE_NAME="metadata.yaml"
METADATA_FILE_FULL_PATH="${MIRROR_ROOT_SUBDIR_PATH}/${METADATA_FILE_NAME}"
MIRROR_DIR_TO_DELETE_FULL_PATH="${MIRROR_ROOT_SUBDIR_PATH}/${MIRROR_DIR_TO_DELETE}"

METADATA_FILE_ROOT_KEY=$(basename "${MIRROR_ROOT_SUBDIR_PATH}")

# Import shared utils
# shellcheck disable=SC1091
source "$(dirname "$0")/../utils/utils.sh"

log_info "Storage info for Mirror PVC before deletion:"
get_aosp_mirror_pvc_storage_info "${MIRROR_ROOT_SUBDIR_PATH}"

# ------Delete Specific Mirror Directory------

# Check if only a specific mirror directory is to be deleted
if [[ "${DELETE_ENTIRE_MIRROR_SETUP}" != "true" && -n "${MIRROR_DIR_TO_DELETE}" ]]; then
  print_header "MIRROR SETUP: DELETE SPECIFIC MIRROR DIRECTORY"

  # Check if the specified mirror directory to delete exists
  if ! check_directory_exists "${MIRROR_DIR_TO_DELETE_FULL_PATH}"; then
    log_error "Specified mirror directory to delete: '${MIRROR_DIR_TO_DELETE_FULL_PATH}' does not exist. Aborting..."
  fi

  # Delete the specific mirror directory
  delete_mirror_directory "${MIRROR_DIR_TO_DELETE_FULL_PATH}"

  # Delete the mirror entry from metadata file
  delete_mirror_entry_from_metadata "${METADATA_FILE_FULL_PATH}" "${METADATA_FILE_ROOT_KEY}" "${MIRROR_DIR_TO_DELETE}"

  log_info "Storage info for Mirror PVC after deletion:"
  get_aosp_mirror_pvc_storage_info "${MIRROR_ROOT_SUBDIR_PATH}"

  exit 0

elif [[ "${DELETE_ENTIRE_MIRROR_SETUP}" != "true" && -z "${MIRROR_DIR_TO_DELETE}" ]]; then
  log_error "Neither DELETE_ENTIRE_MIRROR_SETUP is set to true nor a specific MIRROR_DIR_TO_DELETE is provided. Aborting..."
fi

# ------Delete Entire Mirror Setup------

# Warn user about deleting entire mirror setup
log_warning "Deleting entire Mirror setup including all mirror directories under ${MIRROR_ROOT_SUBDIR_PATH}..."
log_info "Ignoring input MIRROR_DIR_TO_DELETE: ${MIRROR_DIR_TO_DELETE}"
log_warning "If you intended to delete only a specific mirror directory, YOU HAVE 10 SECONDS TO ABORT..."
sleep 10
log_info "Proceeding with deletion of entire Mirror setup..."

validate_bucket_and_tfvars_args "$TF_BACKEND_BUCKET" "$MIRROR_TFVARS_JSON_FILE_PATH"

# Extract Mirror terraform directory path
MIRROR_TF_DIR=$(dirname "${MIRROR_TFVARS_JSON_FILE_PATH}")
# Extract Mirror tfvars file name
MIRROR_TFVARS_JSON_FILE=$(basename "$MIRROR_TFVARS_JSON_FILE_PATH")

# Change directory temporarily to Mirror terraform
pushd "$MIRROR_TF_DIR" > /dev/null || log_error "Cannot cd to ${MIRROR_TF_DIR}"


# ------Terraform workflow begins to delete the entire mirror setup------

print_header "MIRROR SETUP: DELETE ENTIRE MIRROR SETUP"

run_terraform_init "${TF_BACKEND_BUCKET}"

run_terraform_destroy "${MIRROR_TFVARS_JSON_FILE}"

log_success "Mirror entire setup deleted successfully."

# Exit Mirror terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."
exit 0