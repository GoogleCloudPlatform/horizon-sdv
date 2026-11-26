#!/usr/bin/env bash

# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
# Install Android CTS for use when testing Cuttlefish Virtual Device (CVD).
#
# References:
# https://source.android.com/docs/devices/cuttlefish/cts

# Colours for logging (same pattern as cf_host_initialise.sh).
GREEN='\033[1;32m'
ORANGE='\033[1;33m'
RED='\033[1;31m'
NC='\033[0m'

CTS_DOWNLOAD_URL=$(echo "${CTS_DOWNLOAD_URL}" | xargs)
CTS_DOWNLOAD_URL=${CTS_DOWNLOAD_URL:-}
# Strip any trailing slashes as this can impact on the download URL.
CTS_DOWNLOAD_URL=${CTS_DOWNLOAD_URL%/}
# CTS installs always use /opt/android-cts (symlink to /opt/android-cts_<ver> from cts_initialise; not a job parameter).
# Official Tradefed layout is ${CTS_ROOT}/android-cts/{tools,jdk,results,...}. Legacy images may symlink CTS_ROOT directly
# to the inner android-cts tree (${CTS_ROOT}/tools). Use _cts_tradefed_home() for the directory that contains tools/.
CTS_ROOT=/opt/android-cts
export CTS_ROOT

_cts_tradefed_home() {
    if [[ -d "${CTS_ROOT}/android-cts/tools" ]]; then
        echo "${CTS_ROOT}/android-cts"
    elif [[ -d "${CTS_ROOT}/tools" ]]; then
        echo "${CTS_ROOT}"
    else
        echo "${CTS_ROOT}/android-cts"
    fi
}
CTS_TESTPLAN=$(echo "${CTS_TESTPLAN}" | xargs)
CTS_TESTPLAN=${CTS_TESTPLAN:-cts-virtual-device-stable}
CTS_MAX_TESTCASE_RUN_COUNT=${CTS_MAX_TESTCASE_RUN_COUNT:-}
CTS_MODULE=$(echo "${CTS_MODULE}" | xargs)
CTS_MODULE=${CTS_MODULE:-}
CTS_RETRY_STRATEGY=${CTS_RETRY_STRATEGY:-NO_RETRY}
CTS_TEST=$(echo "${CTS_TEST}" | xargs)
CTS_TEST=${CTS_TEST:-}
CTS_TEST_LISTS_ONLY=${CTS_TEST_LISTS_ONLY:-false}
CTS_TIMEOUT=$(echo "${CTS_TIMEOUT}" | xargs)
CTS_TIMEOUT=${CTS_TIMEOUT:-600}
ANDROID_VERSION=${ANDROID_VERSION:-14}
CTS_BUILD_NUMBER=${CTS_BUILD_NUMBER:-${BUILD_NUMBER}}
JOB_NAME=${JOB_NAME:-cts}
CTS_ARTIFACT_STORAGE_SOLUTION=${CTS_ARTIFACT_STORAGE_SOLUTION:-"GCS_BUCKET"}

SUMMARY_FILE="${WORKSPACE}/cts_execution_parameters.txt"

# Shards should match CVD --num_instances (NUM_INSTANCES).
SHARD_COUNT=$(echo "${SHARD_COUNT}" | xargs)
SHARD_COUNT=${SHARD_COUNT:-8}

if [ -z "${WORKSPACE}" ]; then
    WORKSPACE="${HOME}"
fi

# Don't risk downloading CTS locally!
if [ "$(uname -s)" = "Darwin" ]; then
    echo -e "${RED}This script is only supported on Linux${NC}" >&2
    exit 1
fi

# Show variables.
VARIABLES="Environment:
        JAVA_VERSION=$(java --version)

        WORKSPACE=${WORKSPACE}

"

case "$0" in
    *initialise.sh)
        VARIABLES+="
        CUTTLEFISH_DOWNLOAD_URL=${CUTTLEFISH_DOWNLOAD_URL}

        ANDROID_VERSION=${ANDROID_VERSION}

        CTS_DOWNLOAD_URL=${CTS_DOWNLOAD_URL}
        CTS_ROOT=${CTS_ROOT}
        BUILD_USER=${BUILD_USER:-}

        "
        ;;
    *execution.sh)
        VARIABLES+="
        CTS_TEST_LISTS_ONLY=${CTS_TEST_LISTS_ONLY}
        CTS_TESTPLAN=${CTS_TESTPLAN}
        CTS_MODULE=${CTS_MODULE}
        CTS_MAX_TESTCASE_RUN_COUNT=${CTS_MAX_TESTCASE_RUN_COUNT}
        CTS_RETRY_STRATEGY=${CTS_RETRY_STRATEGY}
        CTS_TEST=${CTS_TEST}
        CTS_TIMEOUT=${CTS_TIMEOUT}

        CTS_ROOT=${CTS_ROOT}

        SHARD_COUNT=${SHARD_COUNT} (--shard-count ${SHARD_COUNT})

        "
        ;;
    *)
        ;;
esac

echo "$0 Test Info:" | tee -a "${SUMMARY_FILE}"
echo "${VARIABLES}" | tee -a "${SUMMARY_FILE}"
