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
# Install Android CTS for use when testing Cuttlefish Virtual Device (CVD).
#
# References:
# https://source.android.com/docs/devices/cuttlefish/cts

CTS_DOWNLOAD_URL=$(echo "${CTS_DOWNLOAD_URL}" | xargs)
CTS_DOWNLOAD_URL=${CTS_DOWNLOAD_URL:-}
# Strip any trailing slashes as this can impact on the download URL.
CTS_DOWNLOAD_URL=${CTS_DOWNLOAD_URL%/}
CTS_PATHNAME=$(echo "${CTS_PATHNAME}" | xargs)
CTS_PATHNAME=${CTS_PATHNAME:-android-cts}
CTS_TESTPLAN=$(echo "${CTS_TESTPLAN}" | xargs)
CTS_TESTPLAN=${CTS_TESTPLAN:-cts-system-virtual}
CTS_MODULE=$(echo "${CTS_MODULE}" | xargs)
CTS_MODULE=${CTS_MODULE:-}
CTS_TEST=$(echo "${CTS_TEST}" | xargs)
CTS_TEST=${CTS_TEST:-}
CTS_TEST_LISTS_ONLY=${CTS_TEST_LISTS_ONLY:-false}
CTS_TIMEOUT=$(echo "${CTS_TIMEOUT}" | xargs)
CTS_TIMEOUT=${CTS_TIMEOUT:-600}
ANDROID_VERSION=${ANDROID_VERSION:-14}

# Architecture x86_64 is only supported at this time.
ARCHITECTURE=${ARCHITECTURE:-x86_64}

# Shards should match CVD --num_instances (NUM_INSTANCES).
SHARD_COUNT=$(echo "${SHARD_COUNT}" | xargs)
SHARD_COUNT=${SHARD_COUNT:-8}

if [ -z "${WORKSPACE}" ]; then
    WORKSPACE="${HOME}"
fi


# Don't risk downloading CTS locally!
if [ "$(uname -s)" = "Darwin" ]; then
    echo "This script is only supported on Linux"
    exit 1
fi

# Show variables.
VARIABLES="Environment:
        ARCHITECTURE=${ARCHITECTURE}
"

case "$0" in
    *initialise.sh)
        VARIABLES+="
        CTS_DOWNLOAD_URL=${CTS_DOWNLOAD_URL}
        ANDROID_VERSION=${ANDROID_VERSION}
        CTS_PATHNAME=${CTS_PATHNAME}
        "
        ;;
    *execution.sh)
        VARIABLES+="
        CTS_TEST_LISTS_ONLY=${CTS_TEST_LISTS_ONLY}
        CTS_TESTPLAN=${CTS_TESTPLAN}
        CTS_MODULE=${CTS_MODULE}
        CTS_TEST=${CTS_TEST}
        CTS_TIMEOUT=${CTS_TIMEOUT}

        SHARD_COUNT=${SHARD_COUNT} (--shard-count ${SHARD_COUNT})
        "
        ;;
    *)
        ;;
esac

VARIABLES+="
        WORKSPACE=${WORKSPACE}
"

echo "${VARIABLES}"
