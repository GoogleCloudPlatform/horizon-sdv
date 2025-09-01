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

# Check virtualization enabled.
function cuttlefish_virtualization() {
    if ! sudo find /dev -name kvm > /dev/null 2>&1; then
        echo "Error: virtualization not enabled"
        exit 1
    fi
}

# Install additional packages.
function cuttlefish_install_additional_packages() {
    local -a package_list=("default-jdk" "adb" "git" "npm" "aapt" "htop")

    echo "Installing additional packages."

    # Ensure update to latest package list.
    sudo apt update -y
    for package in "${package_list[@]}"; do
        if ! dpkg -s "${package}" > /dev/null 2>&1; then
            echo "Installing ${package}"
            sudo apt install -y "${package}"
        else
            echo "${package} already installed"
        fi
    done

    # Show Java version and path.
    which java
    java -version

    # Install Node version manager and nodejs.
    npm cache clean -f
    sudo npm install -g n
    sudo npm install -g wait-on
    sudo n "${NODEJS_VERSION}"
    sudo ln -sf /usr/local/bin/node  /usr/local/bin/nodejs || true

    # Show node version and path.
    which node
    node -v

    echo "Installing additional packages completed."
}

# Disable unattended-upgrades
function disable_unattended_upgrades() {
    sudo systemctl status unattended-upgrades || true
    sudo apt remove -y --purge unattended-upgrades
    sudo apt autoremove -y
    sudo rm -rf /var/log/unattended-upgrades
}

# Install CTS test harness on instance to avoid lengthy CTS runs.
function cuttlefish_install_cts() {
    echo "Installing CTS test harness"

    su -l "${JENKINS_USER}" -c "mkdir -p android-cts_15"
    su -l "${JENKINS_USER}" -c "wget -nv ${CTS_ANDROID_15_URL} -O android-cts_15.zip"
    su -l "${JENKINS_USER}" -c "unzip android-cts_15.zip -d android-cts_15"
    su -l "${JENKINS_USER}" -c "rm -f android-cts_15.zip"

    su -l "${JENKINS_USER}" -c "mkdir -p android-cts_14"
    su -l "${JENKINS_USER}" -c "wget -nv ${CTS_ANDROID_14_URL} -O android-cts_14.zip"
    su -l "${JENKINS_USER}" -c "unzip android-cts_14.zip -d android-cts_14"
    su -l "${JENKINS_USER}" -c "rm -f android-cts_14.zip"
    # Force sync to ensure disk is updated.
    sync

    echo "Installing CTS test harness completed."
}

# Add the user to the CVD groups.
function cuttlefish_user_groups() {
    declare -a cf_gids=(cvdnetwork kvm render)
    local -r gids=$(id -nG "$1")

    for gid in "${cf_gids[@]}"; do
        # This is most reliable method to check if group is present.
        if ! echo "${gids}" | grep -qw "${gid}"; then
            echo "Group ${gid} is missing from user: $1"
            sudo usermod -aG "${gid}" "$1"
        fi
    done
}

function cuttlefish_jenkins_user() {
    sudo useradd -u 1000 -ms /bin/bash ${JENKINS_USER} > /dev/null 2>&1
    sudo passwd -d ${JENKINS_USER} > /dev/null 2>&1
    sudo usermod -aG google-sudoers ${JENKINS_USER} > /dev/null 2>&1
    cuttlefish_user_groups ${JENKINS_USER}
}

function cuttlefish_cleanup() {
    # Clean up
    cd ..
    rm -rf "${CUTTLEFISH_REPO_NAME}"
}

# Install the Cuttlefish packages.
function cuttlefish_install() {
    # Disable unattended-upgrades
    disable_unattended_upgrades

    # Install additional packages
    cuttlefish_install_additional_packages

    git clone "${CUTTLEFISH_REPO_URL}" -b "${CUTTLEFISH_REVISION}" > /dev/null 2>&1
    cd "${CUTTLEFISH_REPO_NAME}" || exit

    declare -r BUILD_SCRIPT=./tools/buildutils/build_packages.sh

    # Build and install the cuttlefish packages
    if ! [ -f "${BUILD_SCRIPT}" ]; then
        echo "Error: ${CUTTLEFISH_REVISION} does not support ${BUILD_SCRIPT}"
        echo "       Please choose a compatible version."
        cuttlefish_cleanup
        exit 1
    else
        echo "Cuttlefish build script: ${BUILD_SCRIPT}"
        # Build cuttlefish packages
        yes Y | "${BUILD_SCRIPT}"

        # Install the cuttlefish packages
        sudo apt install -y ./cuttlefish-base_*.deb ./cuttlefish-user_*.deb

        # Clean up
        cuttlefish_cleanup

        # Add groups to the user.
        cuttlefish_user_groups "$(whoami)"

        # Add jenkins user
        cuttlefish_jenkins_user

        # Install CTS
        if [ "$(uname -s)" = "Darwin" ]; then
            echo "This script is only supported on Linux"
            echo "   Ignore CTS download and install"
        else
            cuttlefish_install_cts
        fi
    fi
}

# Initialise or update Cuttlefish.
function cuttlefish_initialise() {

    # Check if virtualization is enabled.
    cuttlefish_virtualization

    # Check if cuttlefish is already installed
    CUTTLEFISH_VERSION=$(dpkg -s cuttlefish-base | grep '^Version:' | cut -d' ' -f2)
    echo "Cuttlefish revision: ${CUTTLEFISH_REVISION}"
    echo "Cuttlefish installed version: ${CUTTLEFISH_VERSION}"

    if ! dpkg -s cuttlefish-base > /dev/null 2>&1; then
        cuttlefish_install
    else
        if [ "${CUTTLEFISH_UPDATE}" = "true" ]; then
            echo "Cuttlefish upgrade required."
            # Remove and purge previous install.
            # Note: base will remove user, but remove just in case
            sudo apt remove -y cuttlefish-base cuttlefish-user > /dev/null 2>&1
            sudo apt autoremove -y > /dev/null 2>&1
            sudo dpkg --purge cuttlefish-base cuttlefish-user > /dev/null 2>&1
            cuttlefish_install
        fi
    fi
}

# Main program
cuttlefish_initialise
