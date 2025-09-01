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
#   This script defines common environment variables, functions, and build
#   configurations for the OpenBSW builder pipeline. It is intended to be
#   sourced by other scripts in the build process to ensure consistent
#   environment setup, artifact management, and build command definitions.
#
# Features:
#   - Sets default values for key environment variables such as git repository
#     URL, branch, build directories, and artifact storage solutions.
#   - Defines command lines for cloning the repository and building various
#     targets (unit tests, POSIX, NXP S32K148).
#   - Manages artifact lists and post-build commands for copying and storing
#     build outputs and test results.
#   - Supports code coverage collection and reporting if enabled.
#   - Handles workspace setup to avoid issues with spaces in Jenkins
#     workspaces.
#   - Outputs environment variable values for debugging and traceability.
#   - Provides a utility function to create and switch to the build workspace.
#
# Usage:
#   Source this script in your build, test, or storage scripts to inherit the
#   environment and functions. The script automatically sets up the workspace
#   and outputs build information to a file for later reference.
#
# Note:
#   - Some variables and arrays are conditionally populated based on build
#     flags.
#   - The script is designed to be compatible with Jenkins and local
#     environments.

# Common environment functions and variables for OpenBSW builder.
JOB_NAME=${JOB_NAME:-BSW_BUILD}

# Download URL and clone commands.
OPENBSW_GIT_URL=$(echo "${OPENBSW_GIT_URL}" | xargs)
OPENBSW_GIT_URL=${OPENBSW_GIT_URL:-https://github.com/eclipse-openbsw/openbsw.git}
OPENBSW_GIT_BRANCH=$(echo "${OPENBSW_GIT_BRANCH}" | xargs)
OPENBSW_GIT_BRANCH=${OPENBSW_GIT_BRANCH:-main}
OPENBSW_GIT_DIR=${OPENBSW_GIT_DIR:-openbsw}
OPENBSW_CLONE_CMDLINE="git clone ${OPENBSW_GIT_URL} -b ${OPENBSW_GIT_BRANCH} ${OPENBSW_GIT_DIR}"

# CMAKE commands
CMAKE_SYNC_JOBS=${CMAKE_SYNC_JOBS:-}

# Build number and job name for artifact storage.
# shellcheck disable=SC2034
OPENBSW_BUILD_NUMBER=${OPENBSW_BUILD_NUMBER:-${BUILD_NUMBER}}
unset BUILD_NUMBER
JOB_NAME=${JOB_NAME:-openbsw}

# Define artifact storage strategy and functions.
OPENBSW_ARTIFACT_STORAGE_SOLUTION=${OPENBSW_ARTIFACT_STORAGE_SOLUTION:-"GCS_BUCKET"}
OPENBSW_ARTIFACT_STORAGE_SOLUTION=$(echo "${OPENBSW_ARTIFACT_STORAGE_SOLUTION}" | xargs)

# Artifact storage bucket
OPENBSW_ARTIFACT_ROOT_NAME=${OPENBSW_ARTIFACT_ROOT_NAME:-sdva-2108202401-openbsw}

# Workspace or local.
# Store original workspace for use later.
if [ -z "${WORKSPACE}" ]; then
    ORIG_WORKSPACE="${HOME}"
else
    # shellcheck disable=SC2034
    ORIG_WORKSPACE="${WORKSPACE}"
fi
# Do not build in Jenkins workspace because these can contain spaces and will
# break the build tools.
WORKSPACE="${HOME}"/bsw-builds

# Build info file name
BUILD_INFO_FILE="${WORKSPACE}/build_info.txt"

# Test list and result artifacts.
UNIT_TESTS_RESULTS_FILE="${WORKSPACE}/unit_test_results.txt"
UNIT_TESTS_LIST_FILE="${ORIG_WORKSPACE}/unit_test_list.txt"

# Post git clone commands
# shellcheck disable=SC2034
declare -a POST_GIT_CLONE_COMMANDS_LIST

if [ -n "${POST_GIT_CLONE_COMMAND}" ]; then
    # shellcheck disable=SC2034
    POST_GIT_CLONE_COMMANDS_LIST=("${POST_GIT_CLONE_COMMAND}")
fi

# Build commands
BUILD_UNIT_TESTS=${BUILD_UNIT_TESTS:-true}
RUN_UNIT_TESTS=${RUN_UNIT_TESTS:-true}
LIST_UNIT_TESTS=${LIST_UNIT_TESTS:-true}
BUILD_POSIX=${BUILD_POSIX:-true}
BUILD_NXP_S32K148=${BUILD_NXP_S32K148:-true}
UNIT_TEST_TARGET=${UNIT_TEST_TARGET:-all}
CODE_COVERAGE=${CODE_COVERAGE:-false}

# Configure and generate the build systems before building.
UNIT_TESTS_CMDLINE=${UNIT_TESTS_CMDLINE:-cmake -DBUILD_UNIT_TESTS=ON -DCMAKE_BUILD_TYPE=Debug -B cmake-build-unit-tests -S executables/unitTest && cmake --build cmake-build-unit-tests -j${CMAKE_SYNC_JOBS} --target ${UNIT_TEST_TARGET}}
LIST_UNIT_TESTS_CMDLINE=${LIST_UNIT_TESTS_CMDLINE:-cmake -DBUILD_UNIT_TESTS=ON -DCMAKE_BUILD_TYPE=Debug -B cmake-build-unit-tests -S executables/unitTest && cmake --build cmake-build-unit-tests --target help -j${CMAKE_SYNC_JOBS}}
RUN_UNIT_TESTS_CMDLINE=${RUN_UNIT_TESTS_CMDLINE:-ctest --test-dir cmake-build-unit-tests -j${CMAKE_SYNC_JOBS}}
POSIX_BUILD_CMDLINE=${POSIX_BUILD_CMDLINE:-cmake -B cmake-build-posix -S executables/referenceApp && cmake --build cmake-build-posix --target app.referenceApp -j${CMAKE_SYNC_JOBS}}
NXP_S32K148_BUILD_CMDLINE=${NXP_S32K148_BUILD_CMDLINE:-cmake -B cmake-build-s32k148 -S executables/referenceApp -DBUILD_TARGET_PLATFORM='S32K148EVB' --toolchain ../../admin/cmake/ArmNoneEabi-gcc.cmake && cmake --build cmake-build-s32k148 --target app.referenceApp -j${CMAKE_SYNC_JOBS}}

# Artifacts
POSIX_ARTIFACT=${POSIX_ARTIFACT:-"cmake-build-posix/app.referenceApp.elf"}
NXP_S32K148_ARTIFACT=${NXP_S32K148_ARTIFACT:-"cmake-build-s32k148/app.referenceApp.elf"}

# Post build commands
declare -a POST_BUILD_COMMANDS

# Declare artifact array.
# shellcheck disable=SC2034
declare -a OPENBSW_ARTIFACT_LIST=(
    "${BUILD_INFO_FILE}"
)

# Ensure artifacts are accessible for storage and Jenkins workspace
# has access for those stored with jobs.
if ${RUN_UNIT_TESTS}; then
    OPENBSW_ARTIFACT_LIST+=(
        "${UNIT_TESTS_RESULTS_FILE}"
    )
    POST_BUILD_COMMANDS+=(
        "cp -f ${UNIT_TESTS_RESULTS_FILE} \"${ORIG_WORKSPACE}\""
    )
fi

if ${BUILD_POSIX}; then
    OPENBSW_ARTIFACT_LIST+=(
        "${OPENBSW_GIT_DIR}/artifacts/posix"
    )
    POST_BUILD_COMMANDS+=(
        "mkdir -p artifacts/posix"
        "cp -f ${POSIX_ARTIFACT} artifacts/posix || true"
    )
fi

if ${BUILD_NXP_S32K148}; then
    OPENBSW_ARTIFACT_LIST+=(
        "${OPENBSW_GIT_DIR}/artifacts/s32k148"
    )
    POST_BUILD_COMMANDS+=(
        "mkdir -p artifacts/s32k148"
        "cp -f ${NXP_S32K148_ARTIFACT} artifacts/s32k148 || true"
        "cp -f cmake-build-s32k148/application/application.map artifacts/s32k148 || true"
    )
fi

# Code coverage metrics.
if ${CODE_COVERAGE}; then
    POST_BUILD_COMMANDS+=(
        "lcov --capture --directory . --output-file ${WORKSPACE}/coverage_unfiltered.info"
        "lcov --remove ${WORKSPACE}/coverage_unfiltered.info '*libs/3rdparty/googletest/*' '*/mock/*' '*/gmock/*' --output-file ${WORKSPACE}/coverage.info"
        "genhtml ${WORKSPACE}/coverage.info --output-directory cmake-build-unit-tests/coverage"
        "cd cmake-build-unit-tests && cp -rf coverage ${WORKSPACE} && cd -"
        "cd ${WORKSPACE} && tar -zcf coverage.html.tgz coverage && cd -"
        "cp -rf ${WORKSPACE}/coverage \"${ORIG_WORKSPACE}\""
        "cp -f ${WORKSPACE}/coverage.html.tgz \"${ORIG_WORKSPACE}\""
    )
    OPENBSW_ARTIFACT_LIST+=(
        "${WORKSPACE}/coverage_unfiltered.info"
        "${WORKSPACE}/coverage.info"
        "${WORKSPACE}/coverage.html.tgz"
    )
fi

# Show variables.
VARIABLES="Environment:"

case "$0" in
    *initialise.sh)
        VARIABLES+="
        OPENBSW_GIT_URL=${OPENBSW_GIT_URL}
        OPENBSW_GIT_BRANCH=${OPENBSW_GIT_BRANCH}
        OPENBSW_GIT_DIR=${OPENBSW_GIT_DIR}

        OPENBSW_CLONE_CMDLINE=${OPENBSW_CLONE_CMDLINE}

        POST_GIT_CLONE_COMMAND=${POST_GIT_CLONE_COMMAND}
        "
        ;;
    *build.sh)
        VARIABLES+="
        OPENBSW_GIT_DIR=${OPENBSW_GIT_DIR}

        LIST_UNIT_TESTS=${LIST_UNIT_TESTS}
        BUILD_UNIT_TESTS=${BUILD_UNIT_TESTS}
        RUN_UNIT_TESTS=${RUN_UNIT_TESTS}
        BUILD_POSIX=${BUILD_POSIX}
        BUILD_NXP_S32K148=${BUILD_NXP_S32K148}

        LIST_UNIT_TESTS_CMDLINE=${LIST_UNIT_TESTS_CMDLINE}
        UNIT_TESTS_CMDLINE=${UNIT_TESTS_CMDLINE}
        RUN_UNIT_TESTS_CMDLINE=${RUN_UNIT_TESTS_CMDLINE}
        POSIX_BUILD_CMDLINE=${POSIX_BUILD_CMDLINE}
        NXP_S32K148_BUILD_CMDLINE=${NXP_S32K148_BUILD_CMDLINE}

        UNIT_TESTS_LIST_FILE=${UNIT_TESTS_LIST_FILE}
        UNIT_TESTS_RESULTS_FILE=${UNIT_TESTS_RESULTS_FILE}
        "
        ;;
    *storage.sh)
        VARIABLES+="
        OPENBSW_GIT_DIR=${OPENBSW_GIT_DIR}
        OPENBSW_BUILD_NUMBER=${OPENBSW_BUILD_NUMBER}
        OPENBSW_ARTIFACT_ROOT_NAME=${OPENBSW_ARTIFACT_ROOT_NAME}
        OPENBSW_ARTIFACT_STORAGE_SOLUTION_FUNCTION=${OPENBSW_ARTIFACT_STORAGE_SOLUTION_FUNCTION}
        "
        ;;
    *)
        ;;
esac

VARIABLES+="
        WORKSPACE=${WORKSPACE}
        ORIG_WORKSPACE=\"${ORIG_WORKSPACE}\"
"

# Add to build info for storage.
echo "$0 Build Info:" | tee -a "${BUILD_INFO_FILE}"
echo "${VARIABLES}" | tee -a "${BUILD_INFO_FILE}"

function create_workspace() {
    mkdir -p "${WORKSPACE}" > /dev/null 2>&1
    cd "${WORKSPACE}" || exit
}

create_workspace
