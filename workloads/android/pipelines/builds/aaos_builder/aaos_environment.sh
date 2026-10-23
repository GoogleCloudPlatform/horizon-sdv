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
# Common functions and variables for use with AAOS build scripts.
#
# The following variables must be set before this script is referenced by
# the calling scripts.
#
#  - AAOS_GERRIT_MANIFEST_URL: the URL of the AAOS manifest.
#  - AAOS_REVISION: the branch or tag/version of the AAOS manifest.
#  - AAOS_LUNCH_TARGET: the target device.
#
# Optional variables:
#  - AAOS_CLEAN: whether to clean before building.
#  - AAOS_MANIFEST_NAME: default manifest filename.
#  - AAOS_ARTIFACT_STORAGE_SOLUTION: the persistent storage location for
#        artifacts (GCS_BUCKET default).
#  - AAOS_ARTIFACT_ROOT_NAME: the name of the bucket to store artifacts.
#  - AAOS_BUILD_STAGE_FAILED: when "true", OUT_DIR artifacts are not added to
#        AAOS_ARTIFACT_LIST (used by storage when build failed but AI Review ran).
#  - ANDROID_VERSION: the Android version (default: 14).
#  - REPO_SYNC_JOBS: the number of parallel repo sync jobs to use Default: 3).
#  - MAX_REPO_SYNC_JOBS: the maximum number of parallel repo sync jobs
#        supported. (Default: 14).
#  - OVERRIDE_MAKE_COMMAND: the make command line to use
#  - POST_REPO_INITIALISE_COMMAND: additional vendor commands for repo initialisation.
#  - POST_REPO_COMMAND: additional vendor commands initialisation post repo sync.
#  - DISK_SPACE_WATERMARK: percentage watermark to clean out old buids
#        to retain space for current build.
#  - AAOS_PARALLEL_BUILD_JOBS: define the number of parallel build jobs. Default
#        let the build run as many jobs in parallel,  otherwise define max number.
#  - USE_LOCAL_AOSP_MIRROR: Use the AOSP Gerrit mirror to repo sync from.
#  - AOSP_MIRROR_DIR_NAME: The mirror directory.
#  - MIRROR_DIR_FULL_PATH: The full mirror path.
#
# For Gerrit review change sets:
#  - GERRIT_SERVER_URL: URL of Gerrit server.
#  - GERRIT_PROJECT: the name of the project to download.
#  - GERRIT_CHANGE_NUMBER: the change number of the changeset to download.
#  - GERRIT_PATCHSET_NUMBER: the patchset number of the changeset to download.
#  - GERRIT_TOPIC: the topic identifying the changes to fetch.
#
# If running standalone, only AAOS_CLEAN and AAOS_LUNCH_TARGET apply, eg.
#
# AAOS_CLEAN=CLEAN_BUILD \
# AAOS_LUNCH_TARGET=aosp_cf_x86_64_auto-ap1a-userdebug \
# ./workloads/android/pipelines/builds/aaos_builder/aaos_environment.sh
#
# AAOS_CLEAN=CLEAN_ALL \
# ./workloads/android/pipelines/builds/aaos_builder/aaos_environment.sh

# Store BUILD_NUMBER for path in aaos_storage.sh
# shellcheck disable=SC2034
AAOS_BUILD_NUMBER=${AAOS_BUILD_NUMBER:-${BUILD_NUMBER}}
JOB_NAME=${JOB_NAME:-aaos}

# Android rebuilds if Jenkins BUILD_NUMBER / HOSTNAME change.
# New job will always have new number and agent name changes.
# unset Jenkins BUILD_NUMBER and BUILD_HOSTNAME to keep rebuilds
# minimal.
unset BUILD_NUMBER
# BUILD_HOSTNAME is defined by Android from hostname. Jenkinsfile now
# defines a fixed hostname for the agent rather than using default
# agent hostname which changes per build and thus forcing Android
# rebuild. See:
# hostname: jenkins-aaos-build-pod

AAOS_DEFAULT_REVISION=$(echo "${AAOS_DEFAULT_REVISION}" | xargs)
AAOS_DEFAULT_REVISION=${AAOS_DEFAULT_REVISION:-android-16.0.0_r3}

# Android branch/tag:
AAOS_REVISION=${AAOS_REVISION:-${AAOS_DEFAULT_REVISION}}
AAOS_REVISION=$(echo "${AAOS_REVISION}" | xargs)

# Android manifest filename:
AAOS_MANIFEST_NAME=${AAOS_MANIFEST_NAME:-}

# Gerrit AAOS and RPi manifest URLs.
AAOS_GERRIT_MANIFEST_URL=$(echo "${AAOS_GERRIT_MANIFEST_URL}" | xargs)
AAOS_GERRIT_MANIFEST_URL=${AAOS_GERRIT_MANIFEST_URL:-https://android.googlesource.com/platform/manifest}
AAOS_GERRIT_RPI_MANIFEST_URL=$(echo "${AAOS_GERRIT_RPI_MANIFEST_URL}" | xargs)
AAOS_GERRIT_RPI_MANIFEST_URL=${AAOS_GERRIT_RPI_MANIFEST_URL:-https://raw.githubusercontent.com/raspberry-vanilla/android_local_manifest}

# ABFS Flag
ABFS_BUILDER=${ABFS_BUILDER:-false}
ABFS_CACHED_BUILD=${ABFS_CACHED_BUILD:-false}
ABFS_CACHEMAN_DIRECTORY=${ABFS_CACHEMAN_DIRECTORY:-}
ABFS_MOUNT_POINT="abfs"
# ABFS server endpoint for 'abfs config -w'. Build pods run in the jenkins
# namespace, where the short Service name abfs-server does not resolve, so
# default to the FQDN of the Service in the abfs namespace.
ABFS_SERVER=${ABFS_SERVER:-abfs-server.abfs.svc.cluster.local:50051}
ABFS_SERVER=$(echo "${ABFS_SERVER}" | xargs)
UPLOADER_MANIFEST_SERVER=${UPLOADER_MANIFEST_SERVER:-android.googlesource.com}

# Google Repo Sync parallel jobs value
REPO_SYNC_JOBS=${REPO_SYNC_JOBS:-3}
MAX_REPO_SYNC_JOBS=${MAX_REPO_SYNC_JOBS:-14}
# Set up the parallel sync job argument based on value.
# Min 1, Max 14.
REPO_SYNC_JOBS_ARG="-j$(( REPO_SYNC_JOBS < 1 ? 1 : REPO_SYNC_JOBS > MAX_REPO_SYNC_JOBS ? MAX_REPO_SYNC_JOBS : REPO_SYNC_JOBS ))"
# If empty let the build system decide otherwise override with -j<NUMBER>, e.g. -j64
AAOS_PARALLEL_BUILD_JOBS=${AAOS_PARALLEL_BUILD_JOBS:-}

# AOSP Mirror parameters
USE_LOCAL_AOSP_MIRROR=${USE_LOCAL_AOSP_MIRROR:-false}
AOSP_MIRROR_DIR_NAME=${AOSP_MIRROR_DIR_NAME:-}

# Check we have a target defined.
AAOS_LUNCH_TARGET=$(echo "${AAOS_LUNCH_TARGET}" | xargs)
# Default if not defined (important for initial pipeline build)
AAOS_LUNCH_TARGET=${AAOS_LUNCH_TARGET:-sdk_car_x86_64-ap1a-userdebug}
if [ -z "${AAOS_LUNCH_TARGET}" ]; then
    echo -e "\033[1;31mError: please define AAOS_LUNCH_TARGET\033[0m"
    exit 1
fi

# Android Version
ANDROID_VERSION=${ANDROID_VERSION:-14}
case "${ANDROID_VERSION}" in
    16)
        ANDROID_API_LEVEL=36
        ;;
    15)
        ANDROID_API_LEVEL=35
        ;;
    *)
        # Deliberate fallthrough, 14 thus API level 34 minimum.
        ANDROID_API_LEVEL=34
        ;;
esac

# Adjust stat command for platform.
if [ "$(uname -s)" = "Darwin" ]; then
    STAT_CMD="stat -f%z "
else
    STAT_CMD="stat -c%s "
fi

# Android SDK addon file.
AAOS_SDK_ADDON_FILE=${AAOS_SDK_ADDON_FILE:-horizon-sdv-aaos-sys-img2-1.xml}
AAOS_SDK_SYSTEM_IMAGE_PREFIX=${AAOS_SDK_SYSTEM_IMAGE_PREFIX:-sdk-repo-linux-system-images}

# Cache directory
AAOS_CACHE_DIRECTORY=${AAOS_CACHE_DIRECTORY:-/aaos-cache}
AAOS_BUILDS_DIRECTORY="aaos_builds"

# AAOS workspace and artifact storage paths
# Store original workspace for use later.
if [ -z "${WORKSPACE}" ]; then
    ORIG_WORKSPACE="${HOME}"
else
    ORIG_WORKSPACE="${WORKSPACE}"
fi

# Disk space ceiling, remove older build targets if insufficient space.
DISK_SPACE_WATERMARK=${DISK_SPACE_WATERMARK:-85}
if [[ "${AAOS_LUNCH_TARGET}" =~ "rpi" ]]; then
    DISK_SPACE_WATERMARK=78
fi

if [ -d "${AAOS_CACHE_DIRECTORY}" ]; then
    # Ensure PVC has correct privileges.
    # Note: builder Dockerfile defines USER name
    sudo chown builder:builder /"${AAOS_CACHE_DIRECTORY}"
    sudo chmod g+s /"${AAOS_CACHE_DIRECTORY}"

    if [[ "${ABFS_BUILDER}" == "true" ]]; then
        if [[ "${ABFS_CACHED_BUILD}" == "true" ]]; then
            ABFS_CMD_FLAGS="--cache-dir ${AAOS_CACHE_DIRECTORY}/cache"
            mkdir -p "${AAOS_CACHE_DIRECTORY}/cache"
            mkdir -p "${AAOS_CACHE_DIRECTORY}/${ABFS_MOUNT_POINT}"
        fi
    fi
    case "$0" in
        *initialise.sh | *build.sh)
            if [[ "${ABFS_BUILDER}" == "true" ]]; then
                if [[ "${ABFS_CACHED_BUILD}" == "true" ]]; then
                    USAGE=$(df -h "${AAOS_CACHE_DIRECTORY}" | tail -1 | awk '{print "Used " $3 " of " $2}')
                    USED_PERCENTAGE=$(df "${AAOS_CACHE_DIRECTORY}" | tail -1 | awk '{print ($3/$2)*100}' | cut -d '.' -f 1)
                    if [ "${USED_PERCENTAGE}" -lt "${DISK_SPACE_WATERMARK}" ]; then
                        echo "Disk space - ${USED_PERCENTAGE}% (${USAGE})"
                    else
                        echo "WARNING: Insufficient disk space - ${USED_PERCENTAGE}% (${USAGE})"
                        echo "WARNING: Removing ${AAOS_CACHE_DIRECTORY}/cache} ..."
                        find "${AAOS_CACHE_DIRECTORY}/cache" -delete
                    fi
                fi
            else
                # Remove unwanted directories that may have been created for dev.
                # Retain the official cache directories.
                find "${AAOS_CACHE_DIRECTORY}" -mindepth 1 -maxdepth 1 -type d ! -name "${AAOS_BUILDS_DIRECTORY}" ! \
                    -name 'lost+found' -exec rm -rf {} + || true

                # Remove oldest target directory if disk space is limited.
                while true; do
                    USED_PERCENTAGE=$(df "${AAOS_CACHE_DIRECTORY}" | tail -1 | awk '{print ($3/$2)*100}' | cut -d '.' -f 1)
                    if [ "${USED_PERCENTAGE}" -lt "${DISK_SPACE_WATERMARK}" ]; then
                        break
                    fi
                    USAGE=$(df -h "${AAOS_CACHE_DIRECTORY}" | tail -1 | awk '{print "Used " $3 " of " $2}')
                    echo "WARNING: Insufficient disk space - ${USED_PERCENTAGE}% (${USAGE})"

                    # List the oldest target directory (exclude current lunch target)
                    OLDEST_DIR=$(find "${AAOS_CACHE_DIRECTORY}"/aaos_builds* -mindepth 1 -maxdepth 1 -type d -name 'out_sdv*' ! -name "out_sdv-${AAOS_LUNCH_TARGET}" -exec ls -drt {} + | head -1)
                    if [ -z "${OLDEST_DIR}" ]; then
                        echo "No further target directories to clean up."
                        break
                    fi
                    echo "WARNING: Removing ${OLDEST_DIR} ..."
                    find "${OLDEST_DIR}" -delete
                done
            fi
            ;;
        *)
            ;;
    esac
else
    AAOS_CACHE_DIRECTORY="${HOME}"
fi

EMPTY_DIR="${AAOS_CACHE_DIRECTORY}"/empty_dir

declare -a DIRECTORY_LIST
WORKSPACE="${AAOS_CACHE_DIRECTORY}"/"${AAOS_BUILDS_DIRECTORY}"
if [[ "${ABFS_BUILDER}" == "false" ]]; then
    DIRECTORY_LIST+=(
        "${AAOS_CACHE_DIRECTORY}"/"${AAOS_BUILDS_DIRECTORY}"
    )
else
    if [[ "${ABFS_CACHED_BUILD}" == "true" ]]; then
        DIRECTORY_LIST+=(
            "${AAOS_CACHE_DIRECTORY}/cache"
        )
        WORKSPACE="${AAOS_CACHE_DIRECTORY}/${ABFS_MOUNT_POINT}"
    else
        WORKSPACE="/${ABFS_MOUNT_POINT}"
    fi
fi

# Build records directory: aaos-build-info.txt + aaos-build.log (after WORKSPACE is finalized).
# Jenkins: WORKSPACE is reassigned to the AAOS cache tree, but ORIG_WORKSPACE is still the job workspace —
# archiveArtifacts / storage expecting workspace paths need records there. Argo keeps them on the PVC
# build WORKSPACE. Set AAOS_BUILD_LOGS_USE_ORIG_WORKSPACE=true|false to force either behaviour.
if [[ "${AAOS_BUILD_LOGS_USE_ORIG_WORKSPACE:-}" == "true" ]]; then
  AAOS_BUILD_RECORDS_DIR="${ORIG_WORKSPACE}"
elif [[ "${AAOS_BUILD_LOGS_USE_ORIG_WORKSPACE:-}" == "false" ]]; then
  AAOS_BUILD_RECORDS_DIR="${WORKSPACE}"
elif [[ -n "${JENKINS_URL:-}" ]] || [[ -n "${HUDSON_URL:-}" ]]; then
  AAOS_BUILD_RECORDS_DIR="${ORIG_WORKSPACE}"
else
  AAOS_BUILD_RECORDS_DIR="${WORKSPACE}"
fi
AAOS_BUILD_INFO_FILE="${AAOS_BUILD_RECORDS_DIR}/aaos-build-info.txt"
AAOS_BUILD_LOG_FILE="${AAOS_BUILD_RECORDS_DIR}/aaos-build.log"
export AAOS_BUILD_RECORDS_DIR AAOS_BUILD_INFO_FILE AAOS_BUILD_LOG_FILE

# Clean commands
AAOS_CLEAN=${AAOS_CLEAN:-NO_CLEAN}

# ABFS Cache clean
ABFS_CLEAN_CACHE=${ABFS_CLEAN_CACHE:-false}
if [[ "${ABFS_CLEAN_CACHE}" == "true" ]]; then
    AAOS_CLEAN=CLEAN_ALL
fi

# Override build output directory to keep builds
# separate from each other.
if [[ "${ABFS_BUILDER}" == "false" ]]; then
    export OUT_DIR="out_sdv-${AAOS_LUNCH_TARGET}"
else
    export OUT_DIR="out"
fi

# Architecture:
AAOS_ARCH=""
AAOS_ARCH_ABI=""
if [[ "${AAOS_LUNCH_TARGET}" =~ "arm64" ]]; then
    AAOS_ARCH="arm64"
    AAOS_ARCH_ABI="-v8a"
elif [[ "${AAOS_LUNCH_TARGET}" =~ "x86_64" ]]; then
    AAOS_ARCH="x86_64"
elif [[ "${AAOS_LUNCH_TARGET}" =~ "rpi4" ]]; then
    AAOS_ARCH="rpi4"
elif [[ "${AAOS_LUNCH_TARGET}" =~ "rpi5" ]]; then
    AAOS_ARCH="rpi5"
elif [[ "${AAOS_LUNCH_TARGET}" =~ "tangor" ]]; then
    AAOS_ARCH="arm64"
fi
# SDV CF targets omit x86_64/arm64 in the lunch name; default to x86_64 Cuttlefish.
if [[ "${AAOS_LUNCH_TARGET}" =~ ^sdv_.*_cf ]] && [ -z "${AAOS_ARCH}" ]; then
    AAOS_ARCH="x86_64"
fi

# If Jenkins, or local, the artifacts differ so update.
USER=$(whoami)

# Post repo init commands
declare -a POST_REPO_INITIALISE_COMMANDS_LIST

# Post repo sync commands
declare -a POST_REPO_COMMAND_LIST

# Define the make command line for given target
AAOS_MAKE_CMDLINE=""
# Post build commands
declare -a POST_BUILD_COMMANDS

# Declare artifact array.
declare -a AAOS_ARTIFACT_LIST=(
    "${AAOS_BUILD_INFO_FILE}"
    "${AAOS_BUILD_LOG_FILE}"
)

# Gemini AI assistant: only upload Gemini artifacts when build failed (AI Review ran)
ENABLE_GEMINI_AI_ASSISTANT=${ENABLE_GEMINI_AI_ASSISTANT:-false}

if [[ "${ENABLE_GEMINI_AI_ASSISTANT}" == "true" ]] && [[ "${AAOS_BUILD_STAGE_FAILED}" == "true" ]]; then
    AAOS_ARTIFACT_LIST+=(
      "${ORIG_WORKSPACE}/gemini-assist/"
      "${ORIG_WORKSPACE}/*.json"
      "${ORIG_WORKSPACE}/traces/"
      "/aaos-cache/aaos_builds/gemini-assist/"
      "/aaos-cache/aaos_builds/traces/"
      "/aaos-cache/aaos_builds/headless_output*.json"
      "/aaos-cache/aaos_builds/gemini-client-error.zip"
    )
fi

# Post storage commands
declare -a POST_STORAGE_COMMANDS=(
    "rm -f ${AAOS_BUILD_INFO_FILE}"
    "rm -f ${AAOS_BUILD_LOG_FILE}"
    "rm -rf vendor"
)

# This is a dictionary mapping the target names to the command line
# to build the image.
# SDV Cuttlefish lunch targets (glob sdv_*_cf*, e.g. sdv_core_cf-trunk_staging-userdebug) share
# the same make/artifact path as aosp_cf*; OUT_DIR is still out_sdv-${AAOS_LUNCH_TARGET}.
case "${AAOS_LUNCH_TARGET}" in
    aosp_rpi*)
        AAOS_BUILD_CTS="false"
        AAOS_MAKE_CMDLINE="m bootimage systemimage vendorimage -j${AAOS_PARALLEL_BUILD_JOBS}"
        # FIXME: we can build full flashable image but may require special
        # permissions, for now host the individual parts.
        # ${VERSION}-${DATE}-rpi5.img # rpi5-mkimg.sh
        if [[ "${AAOS_BUILD_STAGE_FAILED}" != "true" ]]; then
            AAOS_ARTIFACT_LIST+=(
                "${OUT_DIR}/target/product/${AAOS_ARCH}/boot.img"
                "${OUT_DIR}/target/product/${AAOS_ARCH}/system.img"
                "${OUT_DIR}/target/product/${AAOS_ARCH}/vendor.img"
            )
        fi

        case "${AAOS_LUNCH_TARGET}" in
            # Download the RPi manifest if we are building for an RPi device.
            # Note: if versions change then the previous manifests must be removed, eg. see POST_REPO_INITIALISE_COMMANDS_LIST above
            *ap1a*)
                POST_REPO_INITIALISE_COMMANDS_LIST=(
                    "curl -o .repo/local_manifests/manifest_brcm_rpi.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-14.0.0_r34/manifest_brcm_rpi.xml --create-dirs"
                    "curl -o .repo/local_manifests/remove_projects.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-14.0.0_r34/remove_projects.xml"
                )
                ;;
            *ap3a*)
                POST_REPO_INITIALISE_COMMANDS_LIST=(
                    "curl -o .repo/local_manifests/manifest_brcm_rpi.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-15.0.0_r4/manifest_brcm_rpi.xml --create-dirs"
                    "curl -o .repo/local_manifests/remove_projects.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-15.0.0_r4/remove_projects.xml"
                )
                ;;
            *bp1a*)
                # bp1a fallthrough: android-15.0.0_r36 / android-15.0.0_r32 / android-15.0.0_r20
                POST_REPO_INITIALISE_COMMANDS_LIST=(
                    "curl -o .repo/local_manifests/manifest_brcm_rpi.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-15.0/manifest_brcm_rpi.xml --create-dirs"
                    "curl -o .repo/local_manifests/remove_projects.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-15.0/remove_projects.xml"
                )
                ;;
            *bp3a*)
                # bp3a fallthrough: android-16.0.0_r3
                POST_REPO_INITIALISE_COMMANDS_LIST=(
                    "curl -o .repo/local_manifests/manifest_brcm_rpi.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-16.0.0_r3/manifest_brcm_rpi.xml --create-dirs"
                    "curl -o .repo/local_manifests/remove_projects.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-16.0.0_r3/remove_projects.xml"
                )
                ;;
            *bp4a*)
                # bp3a fallthrough: android-16.0.0_r4
                POST_REPO_INITIALISE_COMMANDS_LIST=(
                    "curl -o .repo/local_manifests/manifest_brcm_rpi.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-16.0/manifest_brcm_rpi.xml --create-dirs"
                    "curl -o .repo/local_manifests/remove_projects.xml -L ${AAOS_GERRIT_RPI_MANIFEST_URL}/android-16.0/remove_projects.xml"
                )
        esac

        # Clean up the manifests to avoid issues when versions change.
        POST_REPO_COMMAND_LIST=(
            "rm .repo/local_manifests/manifest_brcm_rpi.xml > /dev/null 2>&1"
            "rm .repo/local_manifests/remove_projects.xml > /dev/null 2>&1"
        )
        ;;
    sdk_car*)
        AAOS_BUILD_CTS="false"
        AAOS_MAKE_CMDLINE="m -j${AAOS_PARALLEL_BUILD_JOBS}&& m emu_img_zip -j${AAOS_PARALLEL_BUILD_JOBS}&& m sbom -j${AAOS_PARALLEL_BUILD_JOBS}"
        # Newer versions, sbom is under SOONG
        if [[ "${AAOS_BUILD_STAGE_FAILED}" != "true" ]]; then
            AAOS_ARTIFACT_LIST+=(
                "${OUT_DIR}/soong/sbom/sdk_car_${AAOS_ARCH}/sbom.spdx.json"
                "${OUT_DIR}/target/product/emulator_car64_${AAOS_ARCH}/sbom.spdx.json"
                "${OUT_DIR}/target/product/emulator_car64_${AAOS_ARCH}/${AAOS_SDK_SYSTEM_IMAGE_PREFIX}*.zip"
                "${OUT_DIR}/target/product/emulator_car64_${AAOS_ARCH}/${AAOS_SDK_ADDON_FILE}"
            )
        fi
        POST_STORAGE_COMMANDS+=(
            "rm -f devices.xml"
            "rm -f ${AAOS_SDK_ADDON_FILE}"
        )
        ;;
    aosp_cf*|sdv_*_cf*)
        AAOS_MAKE_CMDLINE="m dist -j${AAOS_PARALLEL_BUILD_JOBS}"

        WIFI_APK_NAME="WifiUtil.apk"

        # Fallback Wifi APK
        WIFI_APK_FALLBACK_CMD="git clone https://android.googlesource.com/platform/tools/tradefederation -b android-14.0.0_r30 ${HOME}/horizon_wifi && cp -f ${HOME}/horizon_wifi/res/apks/wifiutil/${WIFI_APK_NAME} . ; rm -rf ${HOME}/horizon_wifi"
        # Trade Federation Wifi APK from repo.
        WIFI_APK_PATH_NAME="tools/tradefederation/core/res/apks/wifiutil/${WIFI_APK_NAME}"

        POST_BUILD_COMMANDS=(
            "[ -f ${WIFI_APK_PATH_NAME} ] && cp -f ${WIFI_APK_PATH_NAME} . || ${WIFI_APK_FALLBACK_CMD}"
        )

        # If the AAOS_BUILD_CTS variable is set, build CTS only.
        if [[ "${AAOS_BUILD_CTS}" == "true" ]]; then
            # CTS causes OOMs if too may threads are used when building, more threads more memory it requires!
            # Reduce by half to ensure builds succeed.
            threads=$(( $(nproc) / 2 ))
            threads=$(( threads < 1 ? 1 : threads ))
  
            # Build only the CTS test suite.
            AAOS_MAKE_CMDLINE="m cts -j ${threads}"
            if [[ "${AAOS_BUILD_STAGE_FAILED}" != "true" ]]; then
                AAOS_ARTIFACT_LIST+=("${OUT_DIR}/host/linux-x86/cts/android-cts.zip")
            fi
        else
            if [[ "${AAOS_BUILD_STAGE_FAILED}" != "true" ]]; then
                AAOS_ARTIFACT_LIST+=(
                    "${OUT_DIR}/dist/cvd-host_package.tar.gz"
                    "${OUT_DIR}/dist/sbom/sbom.spdx.json"
                    "${OUT_DIR}/dist/aosp_cf_${AAOS_ARCH}_auto-img*.zip"
                    "${OUT_DIR}/dist/sdv_*_cf*-img*.zip"
                    "${OUT_DIR}/host/linux-x86/bin/sdv-cf"
                    "${WIFI_APK_NAME}"
                )
            fi
        fi
        POST_STORAGE_COMMANDS+=(
            "rm -f ${WIFI_APK_NAME}"
            "rm -f ${OUT_DIR}/dist/aosp_cf_${AAOS_ARCH}_auto-img*.zip"
        )
        ;;
    *tangorpro_car*)
        AAOS_BUILD_CTS="false"
        if [[ "${AAOS_BUILD_STAGE_FAILED}" != "true" ]]; then
            AAOS_ARTIFACT_LIST+=(
                "${OUT_DIR}.tgz"
            )
        fi
        AAOS_MAKE_CMDLINE="m -j${AAOS_PARALLEL_BUILD_JOBS} && m android.hardware.automotive.vehicle@2.0-default-service android.hardware.automotive.audiocontrol-service.example -j${AAOS_PARALLEL_BUILD_JOBS}"
        # Pixel Tablet binaries for Android ap1a/ap2a/ap3a/ap4a/bp1a
        case "${AAOS_LUNCH_TARGET}" in
            *ap1a*)
                POST_REPO_COMMAND_LIST=(
                    "curl --output - https://dl.google.com/dl/android/aosp/google_devices-tangorpro-ap1a.240405.002-8d141153.tgz | tar -xzvf - "
                    "tail -n +315 extract-google_devices-tangorpro.sh | tar -zxvf -"
                )
                ;;
            *ap2a*)
                POST_REPO_COMMAND_LIST=(
                    "curl --output - https://dl.google.com/dl/android/aosp/google_devices-tangorpro-ap2a.240805.005-7e95f619.tgz | tar -xzvf - "
                    "tail -n +315 extract-google_devices-tangorpro.sh | tar -zxvf -"
                )
                ;;
            *ap3a*)
                POST_REPO_COMMAND_LIST=(
                    "curl --output - https://dl.google.com/dl/android/aosp/google_devices-tangorpro-ap3a.241105.007-2bf56572.tgz | tar -xzvf - "
                    "tail -n +315 extract-google_devices-tangorpro.sh | tar -zxvf -"
                )
                ;;
            *ap4a*)
                POST_REPO_COMMAND_LIST=(
                    "curl --output - https://dl.google.com/dl/android/aosp/google_devices-tangorpro-ap4a.250205.002-6474e704.tgz | tar -xzvf - "
                    "tail -n +315 extract-google_devices-tangorpro.sh | tar -zxvf -"
                )
                ;;
            *bp1a*)
                POST_REPO_COMMAND_LIST=(
                    "curl --output - https://dl.google.com/dl/android/aosp/google_devices-tangorpro-bp1a.250505.005-fb23c626.tgz | tar -xzvf - "
                    "tail -n +315 extract-google_devices-tangorpro.sh | tar -zxvf -"
                )
                ;;
            *bp3a*)
                echo -e "\033[1;31mTAA-1094: ${AAOS_LUNCH_TARGET} is not currently supported on ${AAOS_REVISION}!\033[0m"
                exit 1
                ;;
            *)
                # android-16.0.0_r2/r3: https://developers.google.com/android/drivers (same as bp1a above)
                POST_REPO_COMMAND_LIST=(
                    "curl --output - https://dl.google.com/dl/android/aosp/google_devices-tangorpro-bp1a.250505.005-fb23c626.tgz | tar -xzvf - "
                    "tail -n +315 extract-google_devices-tangorpro.sh | tar -zxvf -"
                )
                ;;
        esac
        POST_BUILD_COMMANDS=(
            "tar -zcf ${OUT_DIR}.tgz \
                ${OUT_DIR}/target/product/tangorpro/android-info.txt \
                ${OUT_DIR}/target/product/tangorpro/fastboot-info.txt \
                ${OUT_DIR}/target/product/tangorpro/boot.img \
                ${OUT_DIR}/target/product/tangorpro/bootloader.img \
                ${OUT_DIR}/target/product/tangorpro/init_boot.img \
                ${OUT_DIR}/target/product/tangorpro/dtbo.img \
                ${OUT_DIR}/target/product/tangorpro/vendor_kernel_boot.img \
                ${OUT_DIR}/target/product/tangorpro/pvmfw.img \
                ${OUT_DIR}/target/product/tangorpro/vendor_boot.img \
                ${OUT_DIR}/target/product/tangorpro/vbmeta.img \
                ${OUT_DIR}/target/product/tangorpro/vbmeta_system.img \
                ${OUT_DIR}/target/product/tangorpro/vbmeta_vendor.img \
                ${OUT_DIR}/target/product/tangorpro/system.img \
                ${OUT_DIR}/target/product/tangorpro/system_dlkm.img \
                ${OUT_DIR}/target/product/tangorpro/system_ext.img \
                ${OUT_DIR}/target/product/tangorpro/product.img \
                ${OUT_DIR}/target/product/tangorpro/vendor.img \
                ${OUT_DIR}/target/product/tangorpro/vendor_dlkm.img \
                ${OUT_DIR}/target/product/tangorpro/system_other.img \
                ${OUT_DIR}/target/product/tangorpro/super_empty.img \
                ${OUT_DIR}/target/product/tangorpro/vendor"
        )
        POST_STORAGE_COMMANDS+=(
            "rm -f ${OUT_DIR}.tgz"
            "rm -f extract-google_devices-tangorpro.sh"
        )
        ;;
    *)
        # If the target is not one of the above, print an error message
        # but continue as best so people can play with builds.
        echo "WARNING: unknown target ${AAOS_LUNCH_TARGET}"
        AAOS_MAKE_CMDLINE="m -j${AAOS_PARALLEL_BUILD_JOBS}"
        echo "Artifacts will not be stored!"
        ;;
esac

# Additional repo init/sync commands.
if [ -n "${POST_REPO_INITIALISE_COMMAND}" ]; then
    POST_REPO_INITIALISE_COMMANDS_LIST+=("${POST_REPO_INITIALISE_COMMAND}")
fi

# ABFS: Soong runs soong_build in an nsjail sandbox where the ABFS mount is
# read-only and the out directory is only writable at /src/out. The fix needed
# so that Soong writes under the sandbox path differs per Android version, so
# it is selected from the lunch target instead of being a user default.
#   - Android 14 (ap1a) / 15.0.0_r4 (ap3a): nothing needed.
#   - Android 15 (bp1a): upstream build/soong sandbox changes 3619490-3619492
#     (abandoned on main, still fetchable).
#   - Android 16 (bp3a/bp4a): pass TOP and LOG_DIR through sandboxPath in
#     ui/build/soong.go; without it soong_build panics with
#     "open /abfs/out/soong/.temp/rawN: read-only file system".
if [[ "${ABFS_BUILDER}" == "true" ]]; then
    case "${AAOS_LUNCH_TARGET}" in
        *bp1a*)
            POST_REPO_COMMAND_LIST+=(
                "cd build/soong ; git fetch https://android.googlesource.com/platform/build/soong refs/changes/90/3619490/1 && git cherry-pick FETCH_HEAD; git fetch https://android.googlesource.com/platform/build/soong refs/changes/91/3619491/1 && git cherry-pick FETCH_HEAD; git fetch https://android.googlesource.com/platform/build/soong refs/changes/92/3619492/1 && git cherry-pick FETCH_HEAD; cd -"
            )
            ;;
        *bp3a*|*bp4a*)
            POST_REPO_COMMAND_LIST+=(
                "cd build/soong && sed -i -e 's|Set(\"TOP\", os.Getenv(\"TOP\"))|Set(\"TOP\", config.sandboxPath(os.Getenv(\"TOP\"), os.Getenv(\"TOP\")))|' -e 's|Set(\"LOG_DIR\", config.LogsDir())|Set(\"LOG_DIR\", config.sandboxPath(os.Getenv(\"TOP\"), config.LogsDir()))|' ui/build/soong.go && grep -q 'sandboxPath(os.Getenv(\"TOP\")' ui/build/soong.go; cd -"
            )
            ;;
    esac
fi

if [ -n "${POST_REPO_COMMAND}" ]; then
    POST_REPO_COMMAND_LIST+=("${POST_REPO_COMMAND}")
fi

# Additional build commands
if [ -n "${OVERRIDE_MAKE_COMMAND}" ]; then
    AAOS_MAKE_CMDLINE="${OVERRIDE_MAKE_COMMAND}"
fi

# Gerrit Review environment variables: remove leading and trailing slashes.
GERRIT_SERVER_URL=$(echo "${GERRIT_SERVER_URL}" | xargs)
GERRIT_SERVER_URL=${GERRIT_SERVER_URL:-https://android.googlesource.com}
# Strip any trailing slashes as this can impact on the download URL.
GERRIT_SERVER_URL=${GERRIT_SERVER_URL%/}
GERRIT_PROJECT=$(echo "${GERRIT_PROJECT}" | xargs)
GERRIT_CHANGE_NUMBER=$(echo "${GERRIT_CHANGE_NUMBER}" | xargs)
GERRIT_PATCHSET_NUMBER=$(echo "${GERRIT_PATCHSET_NUMBER}" | xargs)
GERRIT_TOPIC=$(echo "${GERRIT_TOPIC}" | xargs)
GERRIT_USERNAME=$(echo "${GERRIT_USERNAME:-}" | xargs)
GERRIT_PASSWORD=$(echo "${GERRIT_PASSWORD:-}" | xargs)
# Gerrit REST (topic query, curl_gerrit_rest_get): HTTP basic auth uses GERRIT_USERNAME + GERRIT_PASSWORD.
GERRIT_HTTP_USERNAME="${GERRIT_USERNAME}"
# Holds changes that will be used to provide vote on verified label.
GERRIT_CHANGES_FILE="${ORIG_WORKSPACE}/gerrit-changes.txt"

# Define artifact storage strategy and functions.
AAOS_ARTIFACT_STORAGE_SOLUTION=${AAOS_ARTIFACT_STORAGE_SOLUTION:-"GCS_BUCKET"}
AAOS_ARTIFACT_STORAGE_SOLUTION=$(echo "${AAOS_ARTIFACT_STORAGE_SOLUTION}" | xargs)

# Artifact storage bucket
AAOS_ARTIFACT_ROOT_NAME=${AAOS_ARTIFACT_ROOT_NAME:-sdva-2108202401-aaos}

# Show variables that are applicable to each script.
VARIABLES="Environment:
        AAOS_LUNCH_TARGET=${AAOS_LUNCH_TARGET}
"

case "$0" in
    *environment.sh)
        VARIABLES+="
        AAOS_CLEAN=${AAOS_CLEAN}
        "
        ;;
    *initialise.sh)
        if [[ "${ABFS_BUILDER}" == "false" ]]; then
            VARIABLES+="
            AAOS_GERRIT_MANIFEST_URL=${AAOS_GERRIT_MANIFEST_URL}
            AAOS_GERRIT_RPI_MANIFEST_URL=${AAOS_GERRIT_RPI_MANIFEST_URL}

            AAOS_REVISION=${AAOS_REVISION}
            AAOS_MANIFEST_NAME=${AAOS_MANIFEST_NAME}

            POST_REPO_INITIALISE_COMMAND=${POST_REPO_INITIALISE_COMMAND}
            POST_REPO_COMMAND=${POST_REPO_COMMAND}

            REPO_SYNC_JOBS_ARG=${REPO_SYNC_JOBS_ARG}

            GERRIT_SERVER_URL=${GERRIT_SERVER_URL}
            GERRIT_PROJECT=${GERRIT_PROJECT}
            GERRIT_CHANGE_NUMBER=${GERRIT_CHANGE_NUMBER}
            GERRIT_PATCHSET_NUMBER=${GERRIT_PATCHSET_NUMBER}
            GERRIT_TOPIC=${GERRIT_TOPIC}
            GERRIT_HTTP_USERNAME=${GERRIT_HTTP_USERNAME}

            USE_LOCAL_AOSP_MIRROR=${USE_LOCAL_AOSP_MIRROR}
            AOSP_MIRROR_DIR_NAME=${AOSP_MIRROR_DIR_NAME}
            MIRROR_DIR_FULL_PATH=${MIRROR_DIR_FULL_PATH}
            "
        else
            VARIABLES+="
            AAOS_REVISION=${AAOS_REVISION}
            AAOS_MANIFEST_NAME=${AAOS_MANIFEST_NAME}
            AAOS_CLEAN=${AAOS_CLEAN}
            ABFS_CACHED_BUILD=${ABFS_CACHED_BUILD}
            ABFS_CMD_FLAGS=${ABFS_CMD_FLAGS}
            ABFS_SERVER=${ABFS_SERVER}
            ABFS_CLIENT_FLAGS=${ABFS_CLIENT_FLAGS}

            UPLOADER_MANIFEST_SERVER=${UPLOADER_MANIFEST_SERVER}

            GERRIT_SERVER_URL=${GERRIT_SERVER_URL}
            GERRIT_PROJECT=${GERRIT_PROJECT}
            GERRIT_CHANGE_NUMBER=${GERRIT_CHANGE_NUMBER}
            GERRIT_PATCHSET_NUMBER=${GERRIT_PATCHSET_NUMBER}

            POST_REPO_COMMAND=${POST_REPO_COMMAND}
            "
        fi
        ;;
    *build.sh)
        # Do not clean cache in build for ABFS cacheman cache.
        if [[ "${ABFS_BUILDER}" == "true" ]]; then
            AAOS_CLEAN=NO_CLEAN
        else
            # Only allow cleaning the build, ensure override.
            if [[ "${AAOS_CLEAN}" != "NO_CLEAN" ]]; then
                AAOS_CLEAN=CLEAN_BUILD
            fi
        fi
        VARIABLES+="
        AAOS_MAKE_CMDLINE=${AAOS_MAKE_CMDLINE}
        AAOS_CLEAN=${AAOS_CLEAN}

        AAOS_BUILD_CTS=${AAOS_BUILD_CTS}

        AAOS_PARALLEL_BUILD_JOBS=${AAOS_PARALLEL_BUILD_JOBS}
        "
        ;;
    *avd_sdk.sh)
        AAOS_CLEAN=NO_CLEAN
        VARIABLES+="
        ANDROID_VERSION=${ANDROID_VERSION}
        ANDROID_API_LEVEL=${ANDROID_API_LEVEL}

        AAOS_SDK_SYSTEM_IMAGE_PREFIX=${AAOS_SDK_SYSTEM_IMAGE_PREFIX}
        AAOS_SDK_ADDON_FILE=${AAOS_SDK_ADDON_FILE}
        "
        ;;

    *storage.sh)
        AAOS_CLEAN=NO_CLEAN
        VARIABLES+="
        AAOS_BUILD_NUMBER=${AAOS_BUILD_NUMBER}

        AAOS_ARCH=${AAOS_ARCH}

        AAOS_ARTIFACT_STORAGE_SOLUTION=${AAOS_ARTIFACT_STORAGE_SOLUTION}
        AAOS_ARTIFACT_ROOT_NAME=${AAOS_ARTIFACT_ROOT_NAME}

        AAOS_BUILD_CTS=${AAOS_BUILD_CTS}

        ENABLE_GEMINI_AI_ASSISTANT=${ENABLE_GEMINI_AI_ASSISTANT}
        "
        ;;
    *)
        ;;
esac

VARIABLES+="
        ORIG_WORKSPACE=${ORIG_WORKSPACE}
        WORKSPACE=${WORKSPACE}
        AAOS_BUILD_RECORDS_DIR=${AAOS_BUILD_RECORDS_DIR}
        AAOS_BUILD_INFO_FILE=${AAOS_BUILD_INFO_FILE}
        AAOS_BUILD_LOG_FILE=${AAOS_BUILD_LOG_FILE}
        hostname=$(hostname)

        Storage Usage (${AAOS_CACHE_DIRECTORY}): $(df -h "${AAOS_CACHE_DIRECTORY}" | tail -1 | awk '{print "Used " $3 " of " $2}')
        Kernel Revision: $(uname -r)
"
# Add to build info for storage.
echo "$0 Build Info:" | tee -a "${AAOS_BUILD_INFO_FILE}"
echo "${VARIABLES}" | tee -a "${AAOS_BUILD_INFO_FILE}"

# Remove directories if requested.
RSYNC_DELETE=${RSYNC_DELETE:-false}
function remove_directory() {
    echo "Remove directory ${1} ..."
    if [[ "${RSYNC_DELETE}" == "true" ]]; then
        echo "Delete with rsync ..."
        mkdir -p "${EMPTY_DIR}"
        # Faster than rm -rf
        time rsync --max-alloc=0 -aq --delete "${EMPTY_DIR}"/ "${1}"/ || true
        # Final, remove directories.
        rm -rf "${EMPTY_DIR}"
        rm -rf "${1}"
    else
        echo "Delete with find ..."
        time find "${1}" -delete
    fi
    echo "Removed directory ${1}."
}

# Clean Workspace or specific build target directory.
case "${AAOS_CLEAN}" in
    CLEAN_ALL)
        for directory in "${DIRECTORY_LIST[@]}"; do
            remove_directory "${directory}"
        done
        ;;
    CLEAN_BUILD)
        remove_directory "${WORKSPACE}"/"${OUT_DIR}"
        ;;
    NO_CLEAN)
        echo "Reusing existing ${WORKSPACE}..."
        ;;
    *)
        ;;
esac

function create_workspace() {
    # ABFS will mount, don't create.
    if [[ "${ABFS_BUILDER}" == "false" ]]; then
        mkdir -p "${WORKSPACE}" > /dev/null 2>&1
    else
        if [[ "${ABFS_CACHED_BUILD}" == "false" ]]; then
            sudo mkdir -p "/${ABFS_MOUNT_POINT}"
            sudo chown builder:builder "/${ABFS_MOUNT_POINT}"
            sudo chmod 0755 "/${ABFS_MOUNT_POINT}"
        fi
    fi
    cd "${WORKSPACE}" || true

    if [[ "${ABFS_BUILDER}" == "false" ]]; then
        if [[ "${AAOS_LUNCH_TARGET}" =~ "aosp_cf_arm64" ]]; then
            # TAA-1095 workaround
            BUG_FIX="rm -rf out && ln -sf ${OUT_DIR} out"
            echo -e "\033[1;31mTAA-1095: workaround for Android 16 OUT_DIR issue:\033[0m"
            echo "${BUG_FIX}"
            eval "${BUG_FIX}"
        fi
    fi
}


function recreate_workspace() {
    remove_directory "${WORKSPACE}"
    create_workspace
}

create_workspace
