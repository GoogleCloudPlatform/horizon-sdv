#!/usr/bin/env bash

# Copyright (c) 2025 Accenture, All Rights Reserved.
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
# Common environment functions and variables for POSIX target tests.

POSIX_KEEP_ALIVE_TIME=$(echo "${POSIX_KEEP_ALIVE_TIME}" | xargs)
POSIX_KEEP_ALIVE_TIME=${POSIX_KEEP_ALIVE_TIME:-20}

JOB_NAME=${JOB_NAME:-BSW_POSIX}

# Download URL for artifacts.
OPENBSW_DOWNLOAD_URL=$(echo "${OPENBSW_DOWNLOAD_URL}" | xargs)
OPENBSW_DOWNLOAD_URL=${OPENBSW_DOWNLOAD_URL:-gs://sdva-2108202401-openbsw/OpenBSW/Builds/BSW_Builder/01}
# Strip any trailing slashes as this can impact on the download URL.
OPENBSW_DOWNLOAD_URL=${OPENBSW_DOWNLOAD_URL%/}

WORKSPACE=${WORKSPACE:-$(pwd)}

# Show variables.
VARIABLES="Environment:"

case "$0" in
    *install.sh)
        VARIABLES+="
        POSIX_KEEP_ALIVE_TIME=${POSIX_KEEP_ALIVE_TIME}

        OPENBSW_DOWNLOAD_URL=${OPENBSW_DOWNLOAD_URL}
        "
        ;;
    *)
        ;;
esac

VARIABLES+="
        WORKSPACE=${WORKSPACE}
"

echo "${VARIABLES}"
