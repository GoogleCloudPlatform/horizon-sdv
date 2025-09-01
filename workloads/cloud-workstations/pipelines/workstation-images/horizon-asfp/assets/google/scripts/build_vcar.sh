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
#
# This script downloads the Android Automotive OS 15 branch and builds
# the Cuttlefish Virtual Device and the Compatibility Test Suite (CTS).

# References
# Manifest: https://android.googlesource.com/platform/manifest/+/refs/heads/android15-automotiveos-dev
# Changes: https://android-review.googlesource.com/q/branch:android15-automotiveos-dev

CODEBASE_DIR="${HOME}/aaos/vcar"
LOG="${HOME}/aaos/build-$(date +"%Y%m%d-%I%M%S").log"

# Ensure the directory exists for logging.
mkdir -p "${HOME}/aaos"

echo "Syncing & building aosp_cf_x86_64_auto-ap4a-userdebug" | tee ${LOG}
df -h | tee -a ${LOG}
time /google/scripts/build_aosp.sh -o ${CODEBASE_DIR} \
  -b android15-automotiveos-dev \
  -t aosp_cf_x86_64_auto-ap4a-userdebug 2>&1 | tee -a ${LOG}
df -h | tee -a ${LOG}

echo
echo "Building CTS" | tee -a ${LOG}
pushd ${CODEBASE_DIR} > /dev/null 2>&1
source build/envsetup.sh
time m cts TARGET_PRODUCT=aosp_cf_x86_64_auto \
    TARGET_RELEASE=ap4a \
    TARGET_BUILD_VARIANT=userdebug 2>&1 | tee -a ${LOG}
df -h | tee -a ${LOG}
popd > /dev/null 2>&1
