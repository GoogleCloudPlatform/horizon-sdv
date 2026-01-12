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
# Start(Launch) and Stop Cuttlefish Virtual Device (CVD) host.
#
# References:
# * https://github.com/google/android-cuttlefish
# * https://source.android.com/docs/devices/cuttlefish/multi-tenancy
# * https://source.android.com/docs/devices/cuttlefish/get-started
#
# Notes:
# Cuttlefish multi-tenancy allows for your host machine to launch multiple
# virtual guest devices with a single launch invocation. TCP sockets start
# at port 6520 and increment. The cuttlefish-base debian package, preallocates
# resources for 10 instances.
#
# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/cvd_environment.sh "$0"

declare BOOTED_INSTANCES=0

# CVD log file.
declare -r logfile="${HOME}"/cvd-"${BUILD_NUMBER}".log
# WiFi device status
declare -r wifilogfile="${WORKSPACE}"/wifi_connection_status.log

# Download CVD host package and Cuttlefish AVD artifacts
function cuttlefish_extract_artifacts() {
    sudo rm -rf "${HOME}"/cf
    mkdir -p "${HOME}"/cf
    cd "${HOME}"/cf || exit

    case "${CUTTLEFISH_DOWNLOAD_URL}" in
        gs://*)
            gcloud storage cp "${CUTTLEFISH_DOWNLOAD_URL}"/cvd-host_package.tar.gz .
            gcloud storage cp "${CUTTLEFISH_DOWNLOAD_URL}"/aosp_cf_"${ARCHITECTURE}"_auto-img*.zip .
            # Allow this to fail.
            gcloud storage cp "${CUTTLEFISH_DOWNLOAD_URL}/${WIFI_APK_NAME}" . >/dev/null 2>&1 || true
            ;;
        *)
            wget -nv "${CUTTLEFISH_DOWNLOAD_URL}"/cvd-host_package.tar.gz .
            wget -r -nd -nv --no-parent -A "aosp_cf_${ARCHITECTURE}_auto-img*.zip" "${CUTTLEFISH_DOWNLOAD_URL}"/
            # Allow this to fail.
            wget -nv "${CUTTLEFISH_DOWNLOAD_URL}/${WIFI_APK_NAME}" . > /dev/null 2>&1 || true
            ;;
    esac

    # Unpack the host packages.
    if ! tar -xvf cvd-host_package.tar.gz
    then
        echo "Failed to extract cvd-host_package.tar.gz"
        exit 1
    fi

    # Unpack the Cuttlefish device images.
    if ! unzip aosp_cf_"${ARCHITECTURE}"_auto-img*.zip
    then
        echo "Failed to extract aosp_cf_${ARCHITECTURE}_auto-img*.zip"
        exit 1
    fi

    # Clean up
    rm -f aosp_cf_"${ARCHITECTURE}"_auto-img*.zip
    rm -f cvd-host_package.tar.gz
}

# Adjust cuttlefish resources
function cuttlefish_adjust_resources() {
    if (( NUM_INSTANCES > 10 )); then
        # Modify the resource file to support > 10 devices
        sudo echo num_cvd_accounts="${NUM_INSTANCES}" | sudo tee -a /etc/default/cuttlefish-host-resources
        # Restart resource
        sudo systemctl restart cuttlefish-host-resources

        # Check how many are available
        INTERFACES=$(ip -c a | grep -c cvd-wtap)
        if (( NUM_INSTANCES == INTERFACES )); then
            echo "Cuttlefish updated for $NUM_INSTANCES instances"
        else
            echo "Warning: resources $INTERFACES != $NUM_INSTANCES"
        fi
    fi
}

# Start Cuttlefish Virtual Device (CVD) host.
function cuttlefish_start() {
    echo "cuttlefish_start"

    cd "${HOME}"/cf || exit

    # Remove log file.
    rm -f "${logfile}"

    # Start the CF devices (must be run as sudo)
    # Options:
    # resume: do not resume using the disk from the last session.
    # config: default to auto
    # report_anonymous_usage_stats: default to no, avoids user input.
    # num_instances: number of guest instances to launch.
    # cpus: virtual CPU count.
    # memory_mb: total memory available to guest.
    # console: enable serial console
    CVD_CMD="sudo HOME=\"${PWD}\" /usr/bin/cvd create --noresume -config=auto \
        -report_anonymous_usage_stats=no --num_instances=\"${NUM_INSTANCES}\" \
        --cpus=\"${VM_CPUS}\" --memory_mb=\"${VM_MEMORY_MB}\" --console=true \
        ${CVD_ADDITIONAL_FLAGS} >> \"${logfile}\" 2>&1 &"
    echo "Running ${CVD_CMD} in background."
    if ! eval "${CVD_CMD}"
    then
        echo "ERROR: command ${CVD_CMD} failed, exit!"
        exit 1
    fi
}

# Install WiFi
function cuttlefish_install_wifi() {
    cd "${HOME}"/cf || exit

    if [ -f "${WIFI_APK_NAME}" ]; then

        echo "WiFi Device Summary:" | tee "${wifilogfile}"

        echo "Start adb server in readiness to install Wifi"
        sudo adb kill-server || true
        sleep 10
        sudo adb start-server || true
        sleep 20

        # shellcheck disable=SC2207
        DEVICES=($(adb devices | grep -E '0.+device$' | cut -f1))
        for device in "${DEVICES[@]}"; do
            echo "Installing ${WIFI_APK_NAME} on ${device}"
            adb -s "${device}" install -g -r "${WIFI_APK_NAME}"

            echo "Enabling WiFi service on ${device}"
            adb -s "${device}" shell su root svc wifi enable

            echo "Connecting WiFi to Network on ${device}"
            adb -s "${device}" shell am instrument -e method "connectToNetwork" -e scan_ssid "false" -e ssid "VirtWifi" -w com.android.tradefed.utils.wifi/.WifiUtil | tee connection_result.log
            if ! grep -E -q "INSTRUMENTATION_RESULT.*result=true" connection_result.log
            then
                echo "${device}: Failed to connect to wifi" | tee -a "${wifilogfile}"
                connection_result=$(grep "INSTRUMENTATION_RESULT:" connection_result.log)
                if [ -n "${connection_result}" ]; then
                    echo "    ${connection_result}" | tee -a "${wifilogfile}"
                fi
            else
                echo "${device}: Successfully connected to wifi" | tee -a "${wifilogfile}"
            fi

            echo "WiFi status on ${device}"
            echo "================================================="
            adb -s "${device}" shell su root dumpsys wifi | grep "current SSID"
            echo "================================================="
        done
    else
        echo "Unable to find ${WIFI_APK_NAME}"
    fi
}

# Wait for device to boot (VIRTUAL_DEVICE_BOOT_COMPLETED) or timeout.
function cuttlefish_wait_for_device_booted() {
    local -r timeout="${SECONDS}"+"${CUTTLEFISH_MAX_BOOT_TIME}"
    echo "Wait for boot: ${CUTTLEFISH_MAX_BOOT_TIME} seconds"
    while (( "${SECONDS}" < "${timeout}" )); do
        BOOTED_INSTANCES=$(grep -c VIRTUAL_DEVICE_BOOT_COMPLETED "${logfile}")
        if (( BOOTED_INSTANCES == NUM_INSTANCES )); then
            echo "Boot completed."
            break
        fi
        echo "Waiting on boot, sleep 20s ..."
        sleep 20
    done
}

# Cleanup cuttlefish directory.
function cuttlefish_cleanup() {
    echo "cuttlefish_cleanup"
    cd "${HOME}" || exit
    sudo rm -rf cf > /dev/null 2>&1
}

function cuttlefish_nuclear() {
    # dnsmasq process can remain and block a new start. Kill all CVD.
    # Brute force so we can stop/start repeatedly on the same instance.
    sudo pkill -9 -f cvd
}

# Stop CVD.
function cuttlefish_stop() {
    echo "cuttlefish_stop"
    adb reboot
    sudo adb kill-server || true
    sudo /usr/bin/cvd stop > /dev/null 2>&1
    sudo /usr/bin/cvd remove > /dev/null 2>&1
    sudo /usr/bin/cvd reset -y --clean-runtime-dir >/dev/null 2>&1
}

# Archive logs
function cuttlefish_archive_logs() {
    cp -f "${logfile}" "${WORKSPACE}"
    cd "${HOME}"/cf/cuttlefish/instances/ || true
    zip -r "${WORKSPACE}"/cuttlefish_logs-"${BUILD_NUMBER}".zip cvd*/logs/ || true
    cd - || true
}

case "${1}" in
    --stop)
        # Stop
        cuttlefish_archive_logs
        cuttlefish_stop
        cuttlefish_cleanup
        cuttlefish_nuclear
        ;;
    --start|*)
        # Adjust resources based on instances requested
        cuttlefish_adjust_resources
        # Start
        cuttlefish_cleanup
        cuttlefish_extract_artifacts
        # This works around CVD issues.
        # CVD can fail to boot any devices, so we retry start.
        # Refer to Google for the reasons why!
        NUM_RETRIES=4
        for (( i = 1; i <= NUM_RETRIES; ++i )); do
            echo "Attempt ${i} of ${NUM_RETRIES} ..."
            cuttlefish_start
            cuttlefish_wait_for_device_booted
            echo "Booted ${BOOTED_INSTANCES} instances of ${NUM_INSTANCES}"
            if (( BOOTED_INSTANCES == NUM_INSTANCES )); then
                sudo /usr/bin/cvd status
                break;
            else
                cuttlefish_stop
            fi
        done

        if (( BOOTED_INSTANCES == 0 )); then
            echo "Error: android guest instances/devices not booted."
            # Stop and clean up
            cuttlefish_archive_logs
            cuttlefish_stop
            cuttlefish_cleanup
            exit 1
        elif (( BOOTED_INSTANCES != NUM_INSTANCES )); then
            echo "ERROR: Only booted ${BOOTED_INSTANCES} of requested ${NUM_INSTANCES}!"
            echo "       Terminating."
            exit 1
        fi
        if [[ "${CUTTLEFISH_INSTALL_WIFI}" == "true" ]]; then
            cuttlefish_install_wifi
        fi
        ;;
esac
