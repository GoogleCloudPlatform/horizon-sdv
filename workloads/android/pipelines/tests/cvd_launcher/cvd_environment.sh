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
# Common environment functions and variables for Cuttlefish Virtual Device
# (CVD).

# Time (seconds) to wait for Virtual Device to boot.
CUTTLEFISH_MAX_BOOT_TIME=$(echo "${CUTTLEFISH_MAX_BOOT_TIME}" | xargs)
CUTTLEFISH_MAX_BOOT_TIME=${CUTTLEFISH_MAX_BOOT_TIME:-180}
# Time (minutes) to keep device alive.
CUTTLEFISH_KEEP_ALIVE_TIME=$(echo "${CUTTLEFISH_KEEP_ALIVE_TIME}" | xargs)
CUTTLEFISH_KEEP_ALIVE_TIME=${CUTTLEFISH_KEEP_ALIVE_TIME:-20}

# Whether to install Wifi Utility on devices.
CUTTLEFISH_INSTALL_WIFI=${CUTTLEFISH_INSTALL_WIFI:-false}

# Wifi Utility to install on devices.
WIFI_APK_NAME="WifiUtil.apk"

JOB_NAME=${JOB_NAME:-AAOS_CVD}

# Derive architecture from instance.
ARCHITECTURE=$(uname -m)
case "${ARCHITECTURE}" in
  x86_64)  ARCHITECTURE="x86_64" ;;
  aarch64) ARCHITECTURE="arm64" ;;
  *)       echo "Error: ${ARCHITECTURE} is not supported!"; exit 1 ;;
esac

# Download URL for artifacts.
CUTTLEFISH_DOWNLOAD_URL=$(echo "${CUTTLEFISH_DOWNLOAD_URL}" | xargs)
CUTTLEFISH_DOWNLOAD_URL=${CUTTLEFISH_DOWNLOAD_URL:-gs://sdva-2108202401-aaos/Android/Builds/AAOS_Builder/5}
# Strip any trailing slashes as this can impact on the download URL.
CUTTLEFISH_DOWNLOAD_URL=${CUTTLEFISH_DOWNLOAD_URL%/}

# Specific Cuttlefish Virtual Device and CTS variables.
NUM_INSTANCES=$(echo "${NUM_INSTANCES}" | xargs)
NUM_INSTANCES=${NUM_INSTANCES:-10}
VM_CPUS=$(echo "${VM_CPUS}" | xargs)
VM_CPUS=${VM_CPUS:-3}
VM_MEMORY_MB=$(echo "${VM_MEMORY_MB}" | xargs)
VM_MEMORY_MB=${VM_MEMORY_MB:-8192}

# Resolutions
CVD_ADDITIONAL_FLAGS=${CVD_ADDITIONAL_FLAGS:-}

WORKSPACE=${WORKSPACE:-$(pwd)}

# Show variables.
VARIABLES="Environment:"

case "$0" in
    *start_stop.sh)
        VARIABLES+="
        CUTTLEFISH_MAX_BOOT_TIME=${CUTTLEFISH_MAX_BOOT_TIME}
        CUTTLEFISH_KEEP_ALIVE_TIME=${CUTTLEFISH_KEEP_ALIVE_TIME}

        CUTTLEFISH_DOWNLOAD_URL=${CUTTLEFISH_DOWNLOAD_URL}

        CUTTLEFISH_INSTALL_WIFI=${CUTTLEFISH_INSTALL_WIFI}

        WIFI_APK_NAME=${WIFI_APK_NAME}

        NUM_INSTANCES=${NUM_INSTANCES} (--num_instances=${NUM_INSTANCES})
        VM_CPUS=${VM_CPUS} (--cpu ${VM_CPUS})
        VM_MEMORY_MB=${VM_MEMORY_MB} (--memory_mb ${VM_MEMORY_MB})

        CVD_ADDITIONAL_FLAGS=${CVD_ADDITIONAL_FLAGS}

        ARCHITECTURE=${ARCHITECTURE}

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
