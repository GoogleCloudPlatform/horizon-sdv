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
# Initialise Cuttlefish host instance.
#
# Script is only intended for use by cvd_create_instance_template.sh
# for installing host tools on the base VM instance which is used to
# create the CF instance template.

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/cf_environment.sh "$0"

declare -r JENKINS_USER="jenkins"
declare -r sha1File="${HOME}/${CUTTLEFISH_LATEST_SHA1_FILENAME}"

# Colours for logging.
GREEN='\033[1;32m'
ORANGE='\033[1;33m'
RED='\033[1;31m'
NC='\033[0m'

# Check virtualization enabled.
function cuttlefish_virtualization() {
    if ! sudo find /dev -name kvm > /dev/null 2>&1; then
        echo -e "${RED}Error: virtualization not enabled${NC}"
        exit 1
    fi
}

# Install additional packages.
function cuttlefish_install_additional_packages() {
    local -a package_list=("default-jdk" "adb" "git" "npm" "aapt" "htop" "zip" "unzip")

    echo -e "${GREEN}Installing additional packages.${NC}"

    # Ensure update to latest package list.
    sudo apt update -y
    for package in "${package_list[@]}"; do
        if ! dpkg -s "${package}" > /dev/null 2>&1; then
            echo -e "${GREEN}Installing ${package}${NC}"
            sudo apt install -y "${package}"
        else
            echo -e "${GREEN}${package} already installed${NC}"
        fi
    done

    echo -e "${ORANGE}Install version ${JAVA_VERSION}.${NC}"
    sudo apt-get update -y
    sudo apt-get install -y "${JAVA_VERSION}" || true

    echo -e "${GREEN}Java version:${NC}"
    java --version

    # Install Node version manager and nodejs.
    echo -e "${GREEN}Installing nodejs ${NODEJS_VERSION}${NC}"
    npm cache clean -f
    sudo npm install -g n
    sudo n "${NODEJS_VERSION}"
    sudo npm install -g wait-on
    sudo ln -sf /usr/local/bin/node  /usr/local/bin/nodejs || true

    # Show node version and path.
    which node
    node -v

    echo -e "${GREEN}Installing additional packages completed.${NC}"
}

function update_curl() {
    if [ -n "${CURL_UPDATE_COMMAND}" ]; then
        echo -e "${GREEN}Curl update: ${CURL_UPDATE_COMMAND}.${NC}"
        sudo apt update -y
        if ! eval "${CURL_UPDATE_COMMAND}"
        then
            echo -e "${RED}Curl update failed, exit!${NC}"
            exit 1
        else
            echo -e "${ORANGE}Curl version and path:${NC}"
            which curl && curl --version
        fi
        echo -e "${GREEN}Curl update complete.${NC}"
    fi
}

# Disable unattended-upgrades
function disable_unattended_upgrades() {
    sudo systemctl status unattended-upgrades || true
    sudo apt remove -y --purge unattended-upgrades
    sudo apt autoremove -y
    sudo rm -rf /var/log/unattended-upgrades
}

# Download from local storage of official (http)
function download_cts() {
    local url="$1"
    local dest="$2"
    if [[ "${url}" == gs://* ]]; then
        CMD="gcloud storage cp ${url} ${dest}"
        echo "Download $CMD"
        su -l "${JENKINS_USER}" -c "eval $CMD"
    elif [[ "${url}" == http*  ]]; then
        CMD="wget -nv ${url} -O ${dest}"
        echo "Download $CMD"
        su -l "${JENKINS_USER}" -c "eval $CMD"
    else
        echo "echo 'Unknown URL scheme (${url})."
        exit 1
    fi
    su -l "${JENKINS_USER}" -c "du -sh ${dest}"
}

# Install CTS test harness on instance to avoid lengthy CTS runs.
function cuttlefish_install_cts() {
    echo -e "${GREEN}Installing CTS test harness ... ${NC}"
    local start=$SECONDS

    if [ ! -z "${CTS_ANDROID_16_URL}" ]; then
        su -l "${JENKINS_USER}" -c "mkdir -p android-cts_16"
        echo -e "${GREEN}Downloading.${NC} ${CTS_ANDROID_16_URL}. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
        download_cts  "${CTS_ANDROID_16_URL}" android-cts_16.zip
        echo -e "${GREEN}Unpacking.${NC} android-cts_16.zip. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
        su -l "${JENKINS_USER}" -c "unzip -q android-cts_16.zip -d android-cts_16"
        su -l "${JENKINS_USER}" -c "rm -f android-cts_16.zip"
    else
        echo -e "${ORANGE} Skipped Android 16 CTS, nothing to install.${NC}"
    fi

    if [ ! -z "${CTS_ANDROID_15_URL}" ]; then
        su -l "${JENKINS_USER}" -c "mkdir -p android-cts_15"
        echo -e "${GREEN}Downloading.${NC} ${CTS_ANDROID_15_URL}. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
        download_cts  "${CTS_ANDROID_15_URL}" android-cts_15.zip
        echo -e "${GREEN}Unpacking.${NC} android-cts_15.zip. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
        su -l "${JENKINS_USER}" -c "unzip -q android-cts_15.zip -d android-cts_15"
        su -l "${JENKINS_USER}" -c "rm -f android-cts_15.zip"
    else
        echo -e "${ORANGE} Skipped Android 15 CTS, nothing to install.${NC}"
    fi

    if [ ! -z "${CTS_ANDROID_14_URL}" ]; then
        su -l "${JENKINS_USER}" -c "mkdir -p android-cts_14"
        echo -e "${GREEN}Downloading.${NC} ${CTS_ANDROID_14_URL}. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
        download_cts  "${CTS_ANDROID_14_URL}" android-cts_14.zip
        echo -e "${GREEN}Unpacking.${NC} android-cts_14.zip. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
        su -l "${JENKINS_USER}" -c "unzip -q android-cts_14.zip -d android-cts_14"
        su -l "${JENKINS_USER}" -c "rm -f android-cts_14.zip"
    else
        echo -e "${ORANGE} Skipped Android 14 CTS, nothing to install.${NC}"
    fi

    local elapsed=$(( SECONDS - start ))
    m=$(( elapsed / 60 ))
    s=$(( elapsed % 60 ))
    echo -e "${GREEN}Installing CTS test harness completed in ${m}m${s}s.${NC}"
}

# Install Cuttlefish prebuilts
function cuttlefish_install_prebuilt() {
    # Register the apt repository on Artifact Registry
    echo -e "${GREEN}Cuttlefish attempt prebuilt install on $1 ...${NC}"
    sudo curl -fsSL https://us-apt.pkg.dev/doc/repo-signing-key.gpg \
        -o /etc/apt/trusted.gpg.d/artifact-registry.asc
    sudo chmod a+r /etc/apt/trusted.gpg.d/artifact-registry.asc
    echo "deb https://us-apt.pkg.dev/projects/android-cuttlefish-artifacts android-cuttlefish $1" \
        | sudo tee -a /etc/apt/sources.list.d/artifact-registry.list
    sudo cat /etc/apt/sources.list.d/artifact-registry.list
    sudo apt update -y

    if ! sudo apt install -y cuttlefish-base cuttlefish-user cuttlefish-orchestration; then
        echo -e "${RED}Failed to install prebuilt for revision ${1}${NC}"
        return 1
    else
        echo -e "${GREEN}Installed prebuilt for revision ${1}${NC}"
        return 0
    fi
}

# Add the user to the CVD groups.
function cuttlefish_user_groups() {
    declare -a cf_gids=(cvdnetwork kvm render)
    local -r gids=$(id -nG "$1")

    for gid in "${cf_gids[@]}"; do
        # This is most reliable method to check if group is present.
        if ! echo "${gids}" | grep -qw "${gid}"; then
            echo -e "${ORANGE}Group ${gid} is missing from user: ${1}${NC}"
            sudo usermod -aG "${gid}" "$1"
        fi
        if ! getent group "${gid}" &>/dev/null; then
            echo -e "${ORANGE}Group $gid does not exist${NC}"
        fi
    done
}

function update_sudoers() {
    if ! getent group google-sudoers; then
        # TAA-1216: workaround for debian updates from 20251014, google-sudoers
        # group not created from gcloud compute instance create and as such
        # jenkins can't access the instance without being added to the standard
        # sudoers file. Referred to Google but workaround appears to resolve this
        # regression.
        echo -e "${ORANGE}Group google-sudoers missing, use sudoers instead for user $1.${NC}"
        sudo echo "$1 ALL=(ALL:ALL) NOPASSWD: ALL" | sudo tee -a /etc/sudoers
    else
        echo -e "${GREEN}Group google-sudoers exists, add user $1 to group.${NC}"
        sudo usermod -aG google-sudoers "$1" > /dev/null 2>&1 || true
    fi
}

function cuttlefish_jenkins_user() {
    if [[ "$OS_VERSION" == *ubuntu* ]]; then
        # Delete any ubuntu default user (1000)
        # shellcheck disable=SC2046
        sudo userdel $(awk -F: '$3==1000{print $1}' /etc/passwd) > /dev/null 2>&1 || true
    fi
    sudo useradd -u 1000 -ms /bin/bash ${JENKINS_USER} > /dev/null 2>&1
    sudo passwd -d ${JENKINS_USER} > /dev/null 2>&1
    update_sudoers ${JENKINS_USER}
}

function cuttlefish_cleanup() {
    # Clean up
    cd ..
    sudo rm -rf "${HOME}/${CUTTLEFISH_REPO_NAME}"
    # Remove bazel cache to save space before disk image is created.
    sudo rm -rf "${HOME}"/.cache/bazel/
}

# Install the Cuttlefish packages.
function cuttlefish_install() {
    # Disable unattended-upgrades
    disable_unattended_upgrades

    # Install additional packages
    cuttlefish_install_additional_packages

    # Cuttlefish will request package restart, override mode.
    export NEEDRESTART_MODE=a
    # Prebuilts are only supported on X86_64 and main currently, fall through to build on error.
    if [ "${ANDROID_CUTTLEFISH_PREBUILT}" != "true" ] || ! cuttlefish_install_prebuilt "${CUTTLEFISH_REVISION}"; then
        echo -e "${GREEN}Cuttlefish Building from ${CUTTLEFISH_URL} ${CUTTLEFISH_REVISION}.${NC}"; echo
        git clone "${CUTTLEFISH_REPO_URL}" >/dev/null 2>&1
        chown -R "$(whoami):$(whoami)" "${CUTTLEFISH_REPO_NAME}"
        cd "${CUTTLEFISH_REPO_NAME}" || exit
        git checkout "${CUTTLEFISH_REVISION}" > /dev/null 2>&1

        # Fake config ahead of post command
        git config --global user.email "android@example.com"
        git config --global user.name "Android Cuttlefish"

        # Store the sha1 and last commit to file for future reference (branches move).
        echo -e "${GREEN}android-cuttlefish:${CUTTLEFISH_REVISION} sha1:${NC}"
        { echo "android-cuttlefish:${CUTTLEFISH_REVISION} sha1:"; echo; } | tee "${sha1File}"
        git log -1 | tee -a "${sha1File}"

        if [ -n "${CUTTLEFISH_POST_COMMAND}" ]; then
            CMD="${CUTTLEFISH_POST_COMMAND};"
            echo -e "${ORANGE}Running ${CMD} in ${CUTTLEFISH_REPO_NAME}${NC}"
            if ! eval "${CMD}"
            then
                echo -e "${RED}Error: ${CUTTLEFISH_POST_COMMAND} failed,${NC}"
                cuttlefish_cleanup
                exit 1
            else
                echo -e "${GREEN}SUCCESS: ${CUTTLEFISH_POST_COMMAND}${NC}"
                echo -e "${GREEN}android-cuttlefish:${CUTTLEFISH_REVISION} sha1:${NC}"
                { echo ; echo "Post ${CUTTLEFISH_POST_COMMAND}"; echo; } |  tee -a "${sha1File}"
                git log -1 | tee -a "${sha1File}"

                if ! git diff --quiet; then
                    echo -e "${GREEN}android-cuttlefish diffs:${NC}"
                    { echo; echo; echo "android-cuttlefish diffs:"; echo; } | tee -a "${sha1File}"
                    git diff | tee -a "${sha1File}"
                fi
            fi
        fi

        declare -r BUILD_SCRIPT=./tools/buildutils/build_packages.sh

        # Build and install the cuttlefish packages
        if ! [ -f "${BUILD_SCRIPT}" ]; then
            echo -e "${RED}Error: ${CUTTLEFISH_REVISION} does not support ${BUILD_SCRIPT}${NC}"
            echo -e "${RED}       Please choose a compatible version.${NC}"
            cuttlefish_cleanup
            exit 1
        else
            echo -e "${GREEN}Cuttlefish build script: ${BUILD_SCRIPT}${NC}"
            # Build cuttlefish packages
            if ! yes Y | "${BUILD_SCRIPT}"; then
                echo -e "${RED}Error: ${CUTTLEFISH_REVISION} failed on: ${BUILD_SCRIPT}${NC}"
                cuttlefish_cleanup
                exit 1
            fi

            # Install the cuttlefish packages
            if ! sudo apt install -y ./cuttlefish-base_*.deb ./cuttlefish-user_*.deb ./cuttlefish-orchestration*.deb; then
                echo -e "${RED}Error: ${CUTTLEFISH_REVISION} failed to install packages.${NC}"
                cuttlefish_cleanup
                exit 1
            fi

            # Clean up
            cuttlefish_cleanup
        fi
        echo -e "${GREEN}Cuttlefish Build process complete.${NC}"
    fi

    # Add jenkins user
    cuttlefish_jenkins_user

    # Add groups to the user and also root.
    declare -a cf_ids=("$(whoami)" "${JENKINS_USER}" "root")
    for username in "${cf_ids[@]}"; do
        cuttlefish_user_groups "${username}"
    done

    # Install CTS
    if [ "$(uname -s)" = "Darwin" ]; then
        echo -e "${ORANGE}This script is only supported on Linux${NC}"
        echo -e "${ORANGE}   Ignore CTS download and install${NC}"
    else
        cuttlefish_install_cts
    fi

    # Update curl on debian
    update_curl

    # Force sync to ensure disk is updated.
    sync
}

# Initialise or update Cuttlefish.
function cuttlefish_initialise() {

    # Check if virtualization is enabled.
    cuttlefish_virtualization

    # Check if cuttlefish is already installed
    echo -e "${GREEN}Installing Cuttlefish revision ${CUTTLEFISH_REVISION}${NC}"

    if ! dpkg -s cuttlefish-base > /dev/null 2>&1; then
        cuttlefish_install
    else
        if [ "${CUTTLEFISH_UPDATE}" = "true" ]; then
            echo -e "${ORANGE}Cuttlefish upgrade required.${NC}"
            # Remove and purge previous install.
            # Note: base will remove user, but remove just in case
            sudo apt remove -y cuttlefish-* > /dev/null 2>&1
            sudo apt autoremove -y > /dev/null 2>&1
            sudo dpkg --purge cuttlefish-base cuttlefish-user cuttlefish-orchestration > /dev/null 2>&1
            cuttlefish_install
        fi
    fi
    echo -e "${GREEN}Installing Cuttlefish revision ${CUTTLEFISH_REVISION} completed${NC}"
}

# Main program
cuttlefish_initialise
