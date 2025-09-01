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
# Store OpenBSW targets to cloud artifact storage.
#
# Optional variables:
#  - OPENBSW_ARTIFACT_STORAGE_SOLUTION: the persistent storage location for
#        artifacts (GCS_BUCKET default).
#  - OPENBSW_ARTIFACT_ROOT_NAME: the name of the bucket to store artifacts.
#

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/bsw_environment.sh "$0"

export ARTIFACT_LIST="${OPENBSW_ARTIFACT_LIST[*]}"
export ARTIFACT_ROOT_NAME="${OPENBSW_ARTIFACT_ROOT_NAME}"
export BUILD_NUMBER="${OPENBSW_BUILD_NUMBER}"
export ARTIFACT_SUMMARY="${ORIG_WORKSPACE}/openbsw-${BUILD_NUMBER}-artifacts.txt"
export JOB_NAME="${JOB_NAME}"
export ARTIFACT_STORAGE_SOLUTION="${OPENBSW_ARTIFACT_STORAGE_SOLUTION}"
export ARTIFACT_STORAGE_SOLUTION_FUNCTION="${OPENBSW_ARTIFACT_STORAGE_SOLUTION_FUNCTION}"
export WORKSPACE="${ORIG_WORKSPACE}"
"${WORKSPACE}"/workloads/common/storage/storage.sh
exit "$?"
