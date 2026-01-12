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
TF_BACKEND_BUCKET="$1"
AOSP_MIRROR_TFVARS_JSON_FILE_PATH="$2"

# Import shared utils
# shellcheck disable=SC1091
source "$(dirname "$0")/../utils/utils.sh"


# ------Initial Checks and Setup------

validate_bucket_and_tfvars_args "$TF_BACKEND_BUCKET" "$AOSP_MIRROR_TFVARS_JSON_FILE_PATH"

# Extract Mirror terraform directory path
AOSP_MIRROR_TF_DIR=$(dirname "${AOSP_MIRROR_TFVARS_JSON_FILE_PATH}")
# Extract Mirror tfvars file name
AOSP_MIRROR_TFVARS_JSON_FILE=$(basename "$AOSP_MIRROR_TFVARS_JSON_FILE_PATH")

# Change directory temporarily to Mirror terraform
pushd "$AOSP_MIRROR_TF_DIR" > /dev/null || log_error "Cannot cd to ${AOSP_MIRROR_TF_DIR}"


# ------Terraform workflow begins------

print_header "MIRROR SETUP: CREATE MIRROR INFRA"

run_terraform_init "${TF_BACKEND_BUCKET}"

run_terraform_apply "${AOSP_MIRROR_TFVARS_JSON_FILE}"


# Exit Mirror terraform directory
popd > /dev/null || log_error "Failed to return to the original working directory."
exit 0
