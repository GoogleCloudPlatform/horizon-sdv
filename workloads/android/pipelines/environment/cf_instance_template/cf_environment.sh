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

# Percent-encode credentials for embedding in https://user:pass@host/path (required when
# username is an email, or password contains @ : # % etc.). Trailing slashes on CUTTLEFISH_URL
# are stripped so git/libcurl accept the URL.
function _cuttlefish_urlencode_userinfo() {
    python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""), end="")' "$1"
}

# Android Cuttlefish Repository that holds supporting tools to prepare host
# to boot Cuttlefish.
CUTTLEFISH_URL=$(echo "${CUTTLEFISH_URL}" | xargs)
CUTTLEFISH_URL=${CUTTLEFISH_URL:-https://github.com/google/android-cuttlefish.git}
while [[ "${CUTTLEFISH_URL}" == */ ]]; do
    CUTTLEFISH_URL="${CUTTLEFISH_URL%/}"
done
CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION:-main}
REPO_USERNAME=${REPO_USERNAME:-}
REPO_PASSWORD=${REPO_PASSWORD:-}
CUTTLEFISH_REPO_NAME=$(basename "${CUTTLEFISH_URL}" .git)

# Create the repo url with credentials, when required but ensure the URL is never echoed.
if [ -n "${REPO_USERNAME}" ] && [ -n "${REPO_PASSWORD}" ]; then
    if ! command -v python3 >/dev/null 2>&1; then
        echo "ERROR: python3 is required to build authenticated Git HTTPS URLs for private repos." >&2
        exit 1
    fi
    # shellcheck disable=SC2034
    CUTTLEFISH_REPO_URL="${CUTTLEFISH_URL%%://*}://$(_cuttlefish_urlencode_userinfo "${REPO_USERNAME}"):$(_cuttlefish_urlencode_userinfo "${REPO_PASSWORD}")@${CUTTLEFISH_URL#*://}"
else
    # shellcheck disable=SC2034
    CUTTLEFISH_REPO_URL=${CUTTLEFISH_URL}
fi

# Must use flag because there is inconsistency between tag/branch and dpkg
# version number, eg main = 1.0.0.
CUTTLEFISH_UPDATE=${CUTTLEFISH_UPDATE:-false}

# Command tin run in android-cuttlefish repo.
CUTTLEFISH_POST_COMMAND=${CUTTLEFISH_POST_COMMAND:-}

# OS Version
OS_VERSION=${OS_VERSION:-}
# Android CTS test harness URLs, installed on host.
# Allow override - users may install their own. Defaults set in Groovy.
# https://source.android.com/docs/compatibility/cts/downloads
CTS_ANDROID_17_URL=${CTS_ANDROID_17_URL:-}
CTS_ANDROID_16_URL=${CTS_ANDROID_16_URL:-}
CTS_ANDROID_15_URL=${CTS_ANDROID_15_URL:-}
CTS_ANDROID_14_URL=${CTS_ANDROID_14_URL:-}

# Curl upgrade command
CURL_UPDATE_COMMAND=${CURL_UPDATE_COMMAND:-}

# NodeJS Version
NODEJS_VERSION=${NODEJS_VERSION:-20.9.0}

# Absolute path to this directory (cf_create_instance_template.sh, Packer, Helm, cf_compute_rest.py).
# Do not derive CF_SCRIPT_PATH from WORKSPACE alone: Argo sets WORKSPACE=/workspace and the workflow
# already cd's into workloads/android/pipelines/environment/cf_instance_template before exec; a
# WORKSPACE-relative path would wrongly resolve to …/cf_instance_template/workloads/…/packer (missing files).
CF_SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUTTLEFISH_LATEST_SHA1_FILENAME="android-cuttlefish-sha1.txt"

# Show variables.
VARIABLES="Environment:
        OS_VERSION=${OS_VERSION}
        CTS_ANDROID_17_URL=${CTS_ANDROID_17_URL}
        CTS_ANDROID_16_URL=${CTS_ANDROID_16_URL}
        CTS_ANDROID_15_URL=${CTS_ANDROID_15_URL}
        CTS_ANDROID_14_URL=${CTS_ANDROID_14_URL}
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
        CUTTLEFISH_URL=${CUTTLEFISH_URL}
        CUTTLEFISH_REPO_NAME=${CUTTLEFISH_REPO_NAME}
        REPO_USERNAME=${REPO_USERNAME}
        CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION}
        CUTTLEFISH_UPDATE=${CUTTLEFISH_UPDATE}

        CURL_UPDATE_COMMAND=${CURL_UPDATE_COMMAND}

        CUTTLEFISH_LATEST_SHA1_FILENAME=${CUTTLEFISH_LATEST_SHA1_FILENAME}

        CUTTLEFISH_POST_COMMAND=${CUTTLEFISH_POST_COMMAND}
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
