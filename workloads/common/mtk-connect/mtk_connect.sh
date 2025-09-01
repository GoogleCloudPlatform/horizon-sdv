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
# Script to start/stop MTK Connect agent and testbench to create devices.
#
# The following variables must be set before running this script:
#
#  - MTK_CONNECT_DOMAIN: the URL domain for mtk-connect.
#  - MTK_CONNECT_USERNAME: the MTK Connect API key username.
#  - MTK_CONNECT_PASSWORD: the MTK Connect API key password.
#
# Optional variables:
#  - MTK_CONNECT_TESTBENCH: the name of the testbench to create in mtk-connect.
#  - MTK_CONNECT_TEST_ARTIFACT: what is being tested.
#  - MTK_CONNECT_TESTBENCH_USER: users email address limiting access to test
#  - MTK_CONNECT_CONTAINER_ONLY: if true, then only container will be used
#  - MTK_CONNECT_HOST_ONLY: if true, then only HOST will be supported.
#  - MTK_CONNECT_DEVICE_PREFIX: prefix for device name.
#    and access only to the host machine will be allowed.
#
# Example Usage:
# sudo \
#   MTK_CONNECT_DOMAIN=${MTK_CONNECT_DOMAIN} \
#   MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME} \
#   MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD} ./mtk_connect.sh

# Environment
MTK_CONNECT_DOMAIN=${MTK_CONNECT_DOMAIN:-}
MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME:-}
MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD:-}
MTK_CONNECTED_DEVICES=${MTK_CONNECTED_DEVICES:-8}
MTK_CONNECTED_DEVICES=$(echo "${MTK_CONNECTED_DEVICES}" | xargs)
MTK_CONNECT_LAUNCH_APPLICATION_NAME=${MTK_CONNECT_LAUNCH_APPLICATION_NAME:-}
MTK_CONNECT_TESTBENCH=${MTK_CONNECT_TESTBENCH// /_}
MTK_CONNECT_TESTBENCH=$(echo "${MTK_CONNECT_TESTBENCH}" | xargs)
MTK_CONNECT_TESTBENCH_USER=${MTK_CONNECT_TESTBENCH_USER:-everyone}
MTK_CONNECT_TESTBENCH_USER=$(echo "${MTK_CONNECT_TESTBENCH_USER}" | xargs)
MTK_CONNECT_TEST_ARTIFACT=${MTK_CONNECT_TEST_ARTIFACT:-N/A}
MTK_CONNECT_TEST_ARTIFACT=$(echo "${MTK_CONNECT_TEST_ARTIFACT}" | xargs)
MTK_CONNECT_FILE_PATH="$(dirname "${BASH_SOURCE[0]}")"
MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES=${MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES:-false}
MTK_CONNECT_CONTAINER_ONLY=${MTK_CONNECT_CONTAINER_ONLY:-false}
MTK_CONNECT_HOST_ONLY=${MTK_CONNECT_HOST_ONLY:-false}
MTK_CONNECT_DEVICE_PREFIX=${MTK_CONNECT_DEVICE_PREFIX:-AAOS}
NODEJS_VERSION=${NODEJS_VERSION-20.9.0}

declare -r scripts_path="/usr/src/scripts"
declare -r app_path="/usr/src/app"
declare -r config_path="/usr/src/config"
declare -r mtkc_config_path="/opt/mtk-connect-agent/config"

# Get the host and port from adb if MTK Connect is using adb.
# If devices don't exist then the defaults will be used from
# the environment.
if dpkg -s adb > /dev/null 2>&1; then
    # Retrieve a list of the devices host ip and port numbers.
    adb start-server || true
    sleep 20
    MTK_CONNECT_HOST_LIST=$(adb devices | grep -E '0.+device$' | cut -d: -f1 | awk '{print $1","}' | tr -d '\n' | sed 's/,$//' | head -1)
    MTK_CONNECT_HOST_PORT_LIST=$(adb devices | grep -E '0.+device$' | cut -d: -f2 | awk '{print $1","}' | tr -d '\n' | sed 's/,$//' | head -1)
fi
MTK_CONNECT_HOST_LIST=${MTK_CONNECT_HOST_LIST:-$(hostname -I | sed 's/ .*//')}
MTK_CONNECT_HOST_PORT_LIST=${MTK_CONNECT_HOST_PORT_LIST:-6520}

# Start MTK Connect agent and create testbench.
function mtkc_start() {

    # Install the required packages.
    npm install -g wait-on pm2 >/dev/null 2>&1

    # Create the environment.
    mkdir -p "${app_path}" "${config_path}" "${scripts_path}" "${mtkc_config_path}"

    {
        echo "MTK_CONNECT_DOMAIN=${MTK_CONNECT_DOMAIN}"
        echo "MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME}"
        echo "MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD}"
        echo "MTK_CONNECT_DEVICES=${MTK_CONNECTED_DEVICES}"
        echo "MTK_CONNECT_TESTBENCH=${MTK_CONNECT_TESTBENCH}"
        echo "MTK_CONNECT_TESTBENCH_USER=${MTK_CONNECT_TESTBENCH_USER}"
        echo "MTK_CONNECT_HOST_LIST=${MTK_CONNECT_HOST_LIST}"
        echo "MTK_CONNECT_HOST_PORT_LIST=${MTK_CONNECT_HOST_PORT_LIST}"
        echo "MTK_CONNECT_DELETE_OFFLINE=${MTK_CONNECT_DELETE_OFFLINE}"
        echo "MTK_CONNECT_LAUNCH_APPLICATION_NAME=${MTK_CONNECT_LAUNCH_APPLICATION_NAME}"
        echo "MTK_CONNECT_HOST_ONLY=${MTK_CONNECT_HOST_ONLY}"
        echo "MTK_CONNECT_DEVICE_PREFIX=${MTK_CONNECT_DEVICE_PREFIX}"
    } >> "${scripts_path}"/.env

    {
        echo "agent__uri=https://${MTK_CONNECT_DOMAIN}/mtk-connect"
        echo "agent__log__appender=file"
    } >> "${app_path}"/.env

    local -a mtkc_files=(create-testbench.js package.json remove-testbench.js)

    if [ "${MTK_CONNECT_CONTAINER_ONLY}" == "true" ]; then
       mtkc_files+=(download-agent.js)
    fi

    # Copy over the MTKC files.
    for file in "${mtkc_files[@]}"; do
        cp -f "${MTK_CONNECT_FILE_PATH}"/"${file}" "${scripts_path}"
    done

    # Required for dotenv.
    cd "${scripts_path}" || exit # If fails, exit, don't continue!
    npm install

    if [ "${MTK_CONNECT_CONTAINER_ONLY}" == "false" ]; then
         # Local Linux host install.
        AUTH=$(echo -n "${MTK_CONNECT_USERNAME}:${MTK_CONNECT_PASSWORD}" | base64)
        curl -sSL https://"${MTK_CONNECT_DOMAIN}"/mtk-connect/get-agent?platform=linux | AUTH="${AUTH}" bash
        RESULT="$?"
        if (( RESULT != 0 )); then
            echo "Error Download/install returned ${RESULT}"
            exit "${RESULT}"
        fi

        rm -rf "${config_path}"
        ln -sf "${mtkc_config_path}" "${config_path}"
    else
        # Download the agent
        node download-agent.js
        tar -zxf mtk-connect-agent.node.tgz

        # Reorganise the files
        ln -sf "${mtkc_config_path}" "${config_path}"
        mv -f src/* "${app_path}"
        cd "${app_path}" || exit
        node index.js &

        cd "${scripts_path}" || exit # If fails, exit, don't continue!
    fi

    echo "Waiting on ${config_path}/registration.name complete."
    wait-on "${config_path}"/registration.name

}

function mtkc_create_testbench() {
    # Create the requisite testbench.
    node create-testbench.js
}


# Stop MTK Connect agent and remove testbench.
function mtkc_stop() {
    cd "${scripts_path}" || exit
    node remove-testbench.js
    if [ "${MTK_CONNECT_CONTAINER_ONLY}" == "false" ]; then
        # Clean up
        rm -rf /opt/mtk-connect-agent "${config_path}" "${app_path}" "${scripts_path}"
        pkill -9 -f runAgent.js
    fi
}

# Print a summary of the MTK Connect agent.
function mtkc_summary() {
    if (( "$1" == 0 )); then
        echo "===================================================================="
        echo "MTK Connect Summary:"
        echo "MTK Connect Test Artifact URL: ${MTK_CONNECT_TEST_ARTIFACT}"
        echo "MTK Connect URL: https://${MTK_CONNECT_DOMAIN}/mtk-connect"
        echo "MTK Connect Testbench: ${MTK_CONNECT_TESTBENCH}"
        echo "MTK Connect Testbench User: ${MTK_CONNECT_TESTBENCH_USER}"
        echo "===================================================================="
    fi
}

# Show variables.
VARIABLES="
Environment:
    MTK_CONNECT_DOMAIN=${MTK_CONNECT_DOMAIN}
    MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME}
    MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD}
    MTK_CONNECTED_DEVICES=${MTK_CONNECTED_DEVICES}
    MTK_CONNECT_TESTBENCH=${MTK_CONNECT_TESTBENCH}
    MTK_CONNECT_TESTBENCH_USER=${MTK_CONNECT_TESTBENCH_USER}
    MTK_CONNECT_HOST_LIST=${MTK_CONNECT_HOST_LIST}
    MTK_CONNECT_HOST_PORT_LIST=${MTK_CONNECT_HOST_PORT_LIST}
    MTK_CONNECT_LAUNCH_APPLICATION_NAME=${MTK_CONNECT_LAUNCH_APPLICATION_NAME}
    MTK_CONNECT_TEST_ARTIFACT=${MTK_CONNECT_TEST_ARTIFACT}
    MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES=${MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES}
    MTK_CONNECT_CONTAINER_ONLY=${MTK_CONNECT_CONTAINER_ONLY}
    MTK_CONNECT_HOST_ONLY=${MTK_CONNECT_HOST_ONLY}
    MTK_CONNECT_DEVICE_PREFIX=${MTK_CONNECT_DEVICE_PREFIX}
   "
echo "${VARIABLES}"

# Main
case "${1}" in
    --stop)
        # Stop
        mtkc_stop
        RESULT=0
        ;;
    --delete)
        mtkc_start
        mtkc_stop
        RESULT=0
        ;;
    --start|*)
        # Start
        mtkc_start
        mtkc_create_testbench
        RESULT="$?"
        if (( RESULT == 0 )); then
            mtkc_summary "${RESULT}"
        else
            # MTKC failures can lead to dangling jobs.
            mtkc_stop
        fi
esac

exit "${RESULT}"
