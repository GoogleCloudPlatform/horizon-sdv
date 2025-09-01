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

# Description:
# Store AAOS targets to artifact area.
#
# This script will store the specified AAOS target to the artifact area.
# The target is determined by the AAOS_LUNCH_TARGET environment variable.
#
# The following variables must be set before running this script:
#
#  - AAOS_LUNCH_TARGET: the target device.
#
# Optional variables:
#  - AAOS_ARTIFACT_STORAGE_SOLUTION: the persistent storage location for
#        artifacts (GCS_BUCKET default).
#  - AAOS_ARTIFACT_ROOT_NAME: the name of the bucket to store artifacts.
#
# Example usage:
# AAOS_LUNCH_TARGET=sdk_car_x86_64-ap1a-userdebug \
# ./workloads/android/pipelines/builds/aaos_builder/aaos_storage.sh

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/aaos_environment.sh "$0"

export ARTIFACT_LIST="${AAOS_ARTIFACT_LIST[*]}"
export ARTIFACT_ROOT_NAME="${AAOS_ARTIFACT_ROOT_NAME}"
export ARTIFACT_SUMMARY="${ORIG_WORKSPACE}/${AAOS_LUNCH_TARGET}-artifacts.txt"
export BUILD_NUMBER="${AAOS_BUILD_NUMBER}"
export JOB_NAME="${JOB_NAME}"
POST_CLEANUP_STRING=""
export POST_CLEANUP_STRING
POST_CLEANUP_STRING="$(printf "%s\n" "${POST_STORAGE_COMMANDS[@]}")"
export ARTIFACT_STORAGE_SOLUTION="${AAOS_ARTIFACT_STORAGE_SOLUTION}"
export ARTIFACT_STORAGE_SOLUTION_FUNCTION="${AAOS_ARTIFACT_STORAGE_SOLUTION_FUNCTION}"
export WORKSPACE="${ORIG_WORKSPACE}"
"${ORIG_WORKSPACE}"/workloads/common/storage/storage.sh
exit "$?"
