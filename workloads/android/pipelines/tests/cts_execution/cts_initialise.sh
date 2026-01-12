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
# Initialise Android CTS for use when testing Cuttlefish Virtual Device
# (CVD) on host.

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/cts_environment.sh "$0"

# Install the CTS packages.
function cts_initialise() {
    if [ -n "${CTS_DOWNLOAD_URL}" ]; then
        echo "Installing Android CTS from ${CTS_DOWNLOAD_URL}."
        case "${CTS_DOWNLOAD_URL}" in
            gs://*)
                gcloud storage cp "${CTS_DOWNLOAD_URL}" android-cts.zip
                ;;
            *)
                wget -nv "${CTS_DOWNLOAD_URL}" -O android-cts.zip > /dev/null 2>&1
                ;;
        esac

        unzip android-cts.zip > /dev/null 2>&1
        rm -f android-cts.zip
        echo "Installed Android CTS from ${CTS_DOWNLOAD_URL}."
    else
        # Create symlink
        ln -sf android-cts_"${ANDROID_VERSION}"/android-cts "${CTS_PATHNAME}"
    fi
    # Setup JDK path
    echo "export PATH=${PATH}:${HOME}/android-cts/jdk/bin" >> "${HOME}/.bashrc"

    # Double check CTS Java version was created for correct architecture
    echo "Java file type:"
    file "${HOME}"/android-cts/jdk/bin/java || true
}

# Main program
cd "${HOME}" || exit
cts_initialise
