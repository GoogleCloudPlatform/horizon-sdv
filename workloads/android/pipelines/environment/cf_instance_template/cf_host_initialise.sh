#!/usr/bin/env bash

# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
# create the CF instance template. On Debian/Ubuntu (x86_64 and arm64),
# installs linux-modules-extra for the running kernel before Cuttlefish debs
# when the distro publishes that package for the booted kernel.
#
# Also installs Google Cloud Ops Agent (journald receiver) so ephemeral CVD/CTS
# workflows can use Helm spec.guestLogSink: cloud. Live Argo logs default to
# serial port 2 (getSerialPortOutput); Ops Agent is not required for serial.

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/cf_environment.sh "$0"

# Apt package name for the JDK (e.g. temurin-21-jdk, openjdk-21-jdk-headless). Default matches cf_create / x86 Jenkins job.
JAVA_VERSION=${JAVA_VERSION:-temurin-21-jdk}

DEFAULT_USER=${DEFAULT_USER:-jenkins}
declare -r sha1File="${HOME}/${CUTTLEFISH_LATEST_SHA1_FILENAME}"

# Colours for logging.
GREEN='\033[1;32m'
ORANGE='\033[1;33m'
RED='\033[1;31m'
NC='\033[0m'

# Eclipse Temurin packages (temurin-*-jdk) are published on Adoptium apt, not Debian/Ubuntu main.
function ensure_adoptium_apt_for_temurin_package() {
    if [[ "${JAVA_VERSION}" != temurin-* ]]; then
        return 0
    fi
    echo -e "${GREEN}Adding Adoptium apt (required for ${JAVA_VERSION}).${NC}"
    sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates wget gnupg
    sudo mkdir -p /etc/apt/keyrings
    wget -qO - https://packages.adoptium.net/artifactory/api/gpg/key/public | sudo gpg --dearmor -o /etc/apt/keyrings/adoptium.gpg
    # shellcheck source=/dev/null
    . /etc/os-release
    echo "deb [signed-by=/etc/apt/keyrings/adoptium.gpg] https://packages.adoptium.net/artifactory/deb ${VERSION_CODENAME} main" | sudo tee /etc/apt/sources.list.d/adoptium.list > /dev/null
}

function install_jdk_from_apt() {
    # shellcheck source=/dev/null
    [ -f /etc/os-release ] && . /etc/os-release
    echo -e "${GREEN}JDK install: package=${JAVA_VERSION} (ID=${ID:-} VERSION_CODENAME=${VERSION_CODENAME:-})${NC}"
    sudo env DEBIAN_FRONTEND=noninteractive apt-get update -y
    ensure_adoptium_apt_for_temurin_package
    sudo env DEBIAN_FRONTEND=noninteractive apt-get update -y
    sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y "${JAVA_VERSION}" || {
        echo -e "${RED}ERROR: apt install failed for ${JAVA_VERSION}${NC}" >&2
        return 1
    }
}

function verify_jdk() {
    if ! command -v java >/dev/null 2>&1; then
        echo -e "${RED}ERROR: java not on PATH after installing ${JAVA_VERSION}${NC}" >&2
        exit 1
    fi
    local java_bin jdk_home
    java_bin=$(readlink -f "$(command -v java)")
    jdk_home=$(dirname "$(dirname "${java_bin}")")
    echo -e "${GREEN}JDK verify: package=${JAVA_VERSION}${NC}"
    /usr/bin/java --version
    /usr/bin/java -fullversion
    echo -e "${GREEN}JDK resolved: JAVA_HOME=${jdk_home} java=${java_bin}${NC}"
}

# Extra kernel modules package for the *running* kernel (Debian/Ubuntu on GCE images).
# Some Cuttlefish packaging expects optional modules from linux-modules-extra-$(uname -r).
# When that exact apt name is absent, we skip install (no candidate after apt-get update).
function ensure_linux_modules_extra_for_kernel() {
    # shellcheck source=/dev/null
    [ -f /etc/os-release ] && . /etc/os-release
    if [[ "${ID:-}" != "debian" && "${ID:-}" != "ubuntu" ]]; then
        return 0
    fi
    local kver pkg
    kver="$(uname -r)"
    pkg="linux-modules-extra-${kver}"
    echo -e "${GREEN}Checking apt for ${pkg}.${NC}"
    if ! sudo env DEBIAN_FRONTEND=noninteractive apt-get update -y; then
        echo -e "${ORANGE}WARNING: apt-get update failed before linux-modules-extra check; continuing.${NC}" >&2
    fi
    if ! apt-cache show "${pkg}" >/dev/null 2>&1; then
        echo -e "${ORANGE}Skipping ${pkg}: not in apt indexes.${NC}" >&2
        return 0
    fi
    if sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y "${pkg}"; then
        echo -e "${GREEN}${pkg} installed or already satisfied.${NC}"
    else
        echo -e "${ORANGE}WARNING: apt install failed for ${pkg}; continuing.${NC}" >&2
    fi
}

# Mesa EGL loader, Vulkan loader, and Mesa Vulkan drivers (e.g. lavapipe) for host-side
# graphics checks. assemble_cvd still probes EGL/Vulkan with gpu_mode=guest_swiftshader.
function ensure_apt_headless_graphics_for_cvd() {
    echo -e "${GREEN}Installing Mesa EGL/Vulkan host libraries for Cuttlefish (Debian/Ubuntu apt).${NC}"
    sudo env DEBIAN_FRONTEND=noninteractive apt-get update -y
    if ! sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y libegl1-mesa libvulkan1 mesa-vulkan-drivers; then
        echo -e "${ORANGE}WARNING: could not install libegl1-mesa, libvulkan1, and/or mesa-vulkan-drivers; continuing without them.${NC}" >&2
    fi
}

# Check virtualization enabled.
function cuttlefish_virtualization() {
    if ! sudo find /dev -name kvm > /dev/null 2>&1; then
        echo -e "${RED}Error: virtualization not enabled${NC}"
        exit 1
    fi
}

# Install additional packages.
function cuttlefish_install_additional_packages() {
    # Do not install default-jdk: on Debian 12 it pins OpenJDK 17 alternatives even when another JAVA_VERSION is also installed.
    local -a package_list=("adb" "git" "npm" "aapt" "htop" "zip" "unzip" "lsof")

    echo -e "${GREEN}Installing additional packages.${NC}"

    echo -e "${ORANGE}Install JDK: ${JAVA_VERSION}${NC}"
    install_jdk_from_apt

    for package in "${package_list[@]}"; do
        if ! dpkg -s "${package}" > /dev/null 2>&1; then
            echo -e "${GREEN}Installing ${package}${NC}"
            sudo env DEBIAN_FRONTEND=noninteractive apt install -y "${package}"
        else
            echo -e "${GREEN}${package} already installed${NC}"
        fi
    done

    verify_jdk

    ensure_apt_headless_graphics_for_cvd

    # Install Node version manager and nodejs.
    echo -e "${GREEN}Installing nodejs ${NODEJS_VERSION}${NC}"
    npm cache clean -f
    sudo npm install -g n
    sudo n "${NODEJS_VERSION}"
    sudo ln -sf /usr/local/bin/node  /usr/local/bin/nodejs || true

    # Show node version and path.
    which node
    node -v
    echo -e "${ORANGE}npx version and path:${NC}"
    command -v npx || true
    npx -v || true

    echo -e "${GREEN}Installing additional packages completed.${NC}"
}

function update_curl() {
    if [ -n "${CURL_UPDATE_COMMAND}" ]; then
        echo -e "${GREEN}Curl update: ${CURL_UPDATE_COMMAND}.${NC}"
        sudo env DEBIAN_FRONTEND=noninteractive apt update -y
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

# -----------------------------------------------------------------------------
# ensure_google_cloud_ops_agent
# -----------------------------------------------------------------------------
# Optional Argo live logs via Cloud Logging (spec.guestLogSink: cloud | both).
# Guest scripts pipe stdout through systemd-cat -t cvd-argo-guest; this agent
# ships that journal stream to the project. Not used when guestLogSink is serial
# (default). VM SA needs roles/logging.logWriter at runtime.
# https://cloud.google.com/logging/docs/agent/ops-agent/installation

function ensure_google_cloud_ops_agent() {
    local ops_cfg="/etc/google-cloud-ops-agent/config.yaml"
    local install_script="https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh"

    echo -e "${GREEN}Ensuring Google Cloud Ops Agent (optional guestLogSink: cloud).${NC}"

    if ! dpkg -s google-cloud-ops-agent >/dev/null 2>&1; then
        local tmp=""
        tmp="$(mktemp -d)"
        if ! curl -fsSL -o "${tmp}/add-google-cloud-ops-agent-repo.sh" "${install_script}"; then
            echo -e "${RED}ERROR: could not download Ops Agent install script from ${install_script}${NC}" >&2
            rm -rf "${tmp}"
            exit 1
        fi
        if ! sudo bash "${tmp}/add-google-cloud-ops-agent-repo.sh" --also-install; then
            echo -e "${RED}ERROR: Ops Agent install script failed.${NC}" >&2
            rm -rf "${tmp}"
            exit 1
        fi
        rm -rf "${tmp}"
    else
        echo -e "${GREEN}google-cloud-ops-agent package already installed.${NC}"
    fi

    # Journald only: guest stdout uses systemd-cat -t cvd-argo-guest (one line per message).
    # Avoid tailing /var/log/syslog or /tmp/cvd-argo-guest-startup.log — duplicates cloud live logs.
    sudo tee "${ops_cfg}" > /dev/null <<'EOF'
logging:
  receivers:
    cvd_argo_journal:
      type: systemd_journald
  service:
    pipelines:
      cvd_argo_pipeline:
        receivers:
          - cvd_argo_journal
EOF

    sudo systemctl enable google-cloud-ops-agent
    if ! sudo systemctl restart google-cloud-ops-agent; then
        echo -e "${ORANGE}WARNING: Ops Agent restart failed; may start on GCE with instance SA.${NC}" >&2
    fi
    if ! systemctl is-active --quiet google-cloud-ops-agent; then
        echo -e "${ORANGE}WARNING: google-cloud-ops-agent not active during image build (expected until GCE runtime).${NC}" >&2
    else
        echo -e "${GREEN}google-cloud-ops-agent is active.${NC}"
    fi
}

# Disable unattended-upgrades
function disable_unattended_upgrades() {
    sudo systemctl status unattended-upgrades || true
    sudo env DEBIAN_FRONTEND=noninteractive apt remove -y --purge unattended-upgrades
    sudo env DEBIAN_FRONTEND=noninteractive apt autoremove -y
    sudo rm -rf /var/log/unattended-upgrades
}

# ---------------------------------------------------------------------------
# CTS harness install under /opt/android-cts_<ver>
# ---------------------------------------------------------------------------

# Download from local storage of official (http)
function download_cts() {
    local url="$1"
    local dest="$2"
    if [[ "${url}" == gs://* ]]; then
        CMD="gcloud storage cp ${url} ${dest}"
        echo "Download $CMD"
        su -l "${DEFAULT_USER}" -c "eval $CMD"
    elif [[ "${url}" == http*  ]]; then
        CMD="wget -nv ${url} -O ${dest}"
        echo "Download $CMD"
        su -l "${DEFAULT_USER}" -c "eval $CMD"
    else
        echo "echo 'Unknown URL scheme (${url})."
        exit 1
    fi
    su -l "${DEFAULT_USER}" -c "du -sh ${dest}"
}

# Install one CTS pack under /opt/android-cts_<ver>/android-cts (runtime symlinks /opt/android-cts in cts_initialise.sh — not here).
function cuttlefish_install_cts_version() {
    local ver="$1"
    local url="$2"
    local opt_root="/opt/android-cts_${ver}"
    local zip_path="/tmp/android-cts_${ver}.zip"

    echo -e "${GREEN}CTS ${ver}: ${opt_root}${NC}"
    sudo mkdir -p "${opt_root}"
    echo -e "${GREEN}Downloading.${NC} ${url}. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
    download_cts "${url}" "${zip_path}"
    echo -e "${GREEN}Unpacking.${NC} ${zip_path} -> ${opt_root}. ${ORANGE}This can take several minutes to complete, please wait!${NC}"
    sudo unzip -q "${zip_path}" -d "${opt_root}"
    sudo rm -f "${zip_path}"
    if [[ ! -d "${opt_root}/android-cts/tools" ]]; then
        echo -e "${RED}CTS ${ver}: expected ${opt_root}/android-cts/tools missing after unzip.${NC}"
        exit 1
    fi
    sudo chown -R "${DEFAULT_USER}:${DEFAULT_USER}" "${opt_root}"
    sudo chmod -R a+rX "${opt_root}"
    sudo find "${opt_root}/android-cts/tools" -type f \( -name '*.sh' -o -name 'cts-tradefed' \) -exec chmod a+x {} \; 2>/dev/null || true
    if [[ -d "${opt_root}/android-cts/jdk/bin" ]]; then
        sudo chmod a+x "${opt_root}/android-cts/jdk/bin/"* 2>/dev/null || true
    fi
    sudo mkdir -p "${opt_root}/android-cts/results"
    sudo chmod 1777 "${opt_root}/android-cts/results"
}

# Install CTS test harness on instance to avoid lengthy CTS runs.
function cuttlefish_install_cts() {
    if [ "$(uname -s)" = "Darwin" ]; then
        echo -e "${ORANGE}This script is only supported on Linux${NC}"
        echo -e "${ORANGE}   Ignore CTS download and install${NC}"
        return 0;
    fi

    echo -e "${GREEN}Installing CTS test harness under /opt/android-cts_<ver> ... ${NC}"
    local start=$SECONDS

    if [ ! -z "${CTS_ANDROID_17_URL}" ]; then
        cuttlefish_install_cts_version 17 "${CTS_ANDROID_17_URL}"
    else
        echo -e "${ORANGE} Skipped Android 17 CTS, nothing to install.${NC}"
    fi

    if [ ! -z "${CTS_ANDROID_16_URL}" ]; then
        cuttlefish_install_cts_version 16 "${CTS_ANDROID_16_URL}"
    else
        echo -e "${ORANGE} Skipped Android 16 CTS, nothing to install.${NC}"
    fi

    if [ ! -z "${CTS_ANDROID_15_URL}" ]; then
        cuttlefish_install_cts_version 15 "${CTS_ANDROID_15_URL}"
    else
        echo -e "${ORANGE} Skipped Android 15 CTS, nothing to install.${NC}"
    fi

    if [ ! -z "${CTS_ANDROID_14_URL}" ]; then
        cuttlefish_install_cts_version 14 "${CTS_ANDROID_14_URL}"
    else
        echo -e "${ORANGE} Skipped Android 14 CTS, nothing to install.${NC}"
    fi

    local elapsed=$(( SECONDS - start ))
    m=$(( elapsed / 60 ))
    s=$(( elapsed % 60 ))
    echo -e "${GREEN}Installing CTS test harness completed in ${m}m${s}s.${NC}"
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
        # users can't access the instance without being added to the standard
        # sudoers file. Referred to Google but workaround appears to resolve this
        # regression.
        echo -e "${ORANGE}Group google-sudoers missing, use sudoers instead for user $1.${NC}"
        sudo echo "$1 ALL=(ALL:ALL) NOPASSWD: ALL" | sudo tee -a /etc/sudoers
    else
        echo -e "${GREEN}Group google-sudoers exists, add user $1 to group.${NC}"
        sudo usermod -aG google-sudoers "$1" > /dev/null 2>&1 || true
    fi
}

function cuttlefish_default_user() {
    if [[ "$OS_VERSION" == *ubuntu* ]]; then
        # Delete any ubuntu default user (1000)
        # shellcheck disable=SC2046
        sudo userdel $(awk -F: '$3==1000{print $1}' /etc/passwd) > /dev/null 2>&1 || true
    fi
    sudo useradd -u 1000 -ms /bin/bash "${DEFAULT_USER}" > /dev/null 2>&1
    sudo passwd -d "${DEFAULT_USER}" > /dev/null 2>&1
    update_sudoers "${DEFAULT_USER}"
}

function cuttlefish_cleanup() {
    # Clean up (clone lives next to this script: provision_cf_host.sh runs from /tmp/cf, not $HOME).
    cd ..
    sudo rm -rf "./${CUTTLEFISH_REPO_NAME}"
    # Remove bazel cache to save space before disk image is created.
    sudo rm -rf "${HOME}"/.cache/bazel/
}

# Build Cuttlefish
function cuttlefish_build() {
    # Cuttlefish will request package restart, override mode.
    export NEEDRESTART_MODE=a
    echo -e "${GREEN}Cuttlefish Building from ${CUTTLEFISH_URL} ${CUTTLEFISH_REVISION}.${NC}"; echo
    echo -e "${GREEN}Cloning into ${CUTTLEFISH_REPO_NAME} (from URL host path; see cf_environment.sh).${NC}"
    if ! git clone "${CUTTLEFISH_REPO_URL}" "${CUTTLEFISH_REPO_NAME}"; then
        echo -e "${RED}git clone failed. Check HTTPS access to the repo from this VM, TLS trust, and for private Gerrit/Git HTTPS that REPO_USERNAME and REPO_PASSWORD are set in the Jenkins job.${NC}"
        exit 1
    fi
    chown -R "$(whoami):$(whoami)" "${CUTTLEFISH_REPO_NAME}"
    cd "${CUTTLEFISH_REPO_NAME}" || exit
    if ! git checkout "${CUTTLEFISH_REVISION}"; then
        echo -e "${RED}git checkout ${CUTTLEFISH_REVISION} failed (branch/tag missing or fetch incomplete).${NC}"
        cuttlefish_cleanup
        exit 1
    fi

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
        if ! sudo env DEBIAN_FRONTEND=noninteractive apt install -y ./cuttlefish-base_*.deb ./cuttlefish-user_*.deb ./cuttlefish-orchestration*.deb; then
            echo -e "${RED}Error: ${CUTTLEFISH_REVISION} failed to install packages.${NC}"
            cuttlefish_cleanup
            exit 1
        fi

        # Clean up
        cuttlefish_cleanup
    fi

    # Add groups to the user and also root.
    declare -a cf_ids=("$(whoami)" "${DEFAULT_USER}" "root")
    for username in "${cf_ids[@]}"; do
        cuttlefish_user_groups "${username}"
    done

    echo -e "${GREEN}Cuttlefish Build process complete.${NC}"
}

# Install the Cuttlefish packages.
function cuttlefish_install() {
    # Disable unattended-upgrades
    disable_unattended_upgrades

    # Kernel extras before Cuttlefish debs (Debian/Ubuntu x86_64 and arm64 Packer images).
    ensure_linux_modules_extra_for_kernel

    # Install additional packages
    cuttlefish_install_additional_packages

    # Add default user
    cuttlefish_default_user

    # Build cuttlefish
    cuttlefish_build

    # Install CTS
    cuttlefish_install_cts

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
            sudo env DEBIAN_FRONTEND=noninteractive apt remove -y cuttlefish-* > /dev/null 2>&1
            sudo env DEBIAN_FRONTEND=noninteractive apt autoremove -y > /dev/null 2>&1
            sudo dpkg --purge cuttlefish-base cuttlefish-user cuttlefish-orchestration > /dev/null 2>&1
            cuttlefish_install
        fi
    fi
    ensure_google_cloud_ops_agent
    echo -e "${GREEN}Installing Cuttlefish revision ${CUTTLEFISH_REVISION} completed${NC}"
}

# Main program
cuttlefish_initialise
