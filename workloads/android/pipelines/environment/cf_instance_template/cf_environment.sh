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
# Common environment functions and variables for Cuttlefish Instance
# Template creation.

# Android Cuttlefish Repository that holds supporting tools to prepare host
# to boot Cuttlefish.
CUTTLEFISH_REPO_URL=$(echo "${CUTTLEFISH_REPO_URL}" | xargs)
CUTTLEFISH_REPO_URL=${CUTTLEFISH_REPO_URL:-https://github.com/google/android-cuttlefish.git}
CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION:-main}
CUTTLEFISH_REPO_NAME=$(basename "${CUTTLEFISH_REPO_URL}" .git)
# Must use flag because there is inconsistency between tag/branch and dpkg
# version number, eg main = 1.0.0.
CUTTLEFISH_UPDATE=${CUTTLEFISH_UPDATE:-false}

# Android CTS test harness URLs, installed on host.
# https://source.android.com/docs/compatibility/cts/downloads
CTS_ANDROID_15_URL="https://dl.google.com/dl/android/cts/android-cts-15_r5-linux_x86-x86.zip"
CTS_ANDROID_14_URL="https://dl.google.com/dl/android/cts/android-cts-14_r9-linux_x86-x86.zip"
# NodeJS Version
NODEJS_VERSION=${NODEJS_VERSION:-20.9.0}

# Architecture x86_64 is only supported at this time.
ARCHITECTURE=${ARCHITECTURE:-x86_64}

# Support local vs Jenkins.
if [ -z "${WORKSPACE}" ]; then
    CF_SCRIPT_PATH=.
else
    CF_SCRIPT_PATH=workloads/android/pipelines/environment/cf_instance_template
fi

# Show variables.
VARIABLES="Environment:
        CTS_ANDROID_15_URL=${CTS_ANDROID_15_URL}
        CTS_ANDROID_14_URL=${CTS_ANDROID_14_URL}

        ARCHITECTURE=${ARCHITECTURE}
"

case "$0" in
    *create_instance_template.sh)
        VARIABLES+="
        CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION}

        CF_SCRIPT_PATH=${CF_SCRIPT_PATH}
        "
        ;;
    *initialise.sh)
        VARIABLES+="
        CUTTLEFISH_REPO_URL=${CUTTLEFISH_REPO_URL}
        CUTTLEFISH_REPO_NAME=${CUTTLEFISH_REPO_NAME}
        CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION}
        CUTTLEFISH_UPDATE=${CUTTLEFISH_UPDATE}
        "
        ;;
    *)
        ;;
esac

VARIABLES+="
        WORKSPACE=${WORKSPACE}

        /proc/cpuproc vmx: $(grep -cw vmx /proc/cpuinfo)
"

echo "${VARIABLES}"
