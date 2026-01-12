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
# This script automates the build process for the OpenBSW project, supporting
# multiple targets and unit testing workflows. It sources the environment
# configuration, defines functions for building and testing, and executes
# steps based on the parameters/environment variables.
#
# Features:
# - Builds POSIX and NXP S32K148 targets.
# - Builds, lists, and runs unit tests.
# - Supports post-build command execution.
#
# Usage:
#   This script is intended to be invoked as part of a CI/CD pipeline or
#   manually to perform builds and tests for OpenBSW.
#
# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/bsw_environment.sh "$0"

# Function to build the POSIX target
function build_posix_target() {
    echo "Building POSIX target"
    if ! eval "${POSIX_BUILD_CMDLINE}"
    then
        echo "ERROR: ${POSIX_BUILD_CMDLINE} failed"
        exit 1
    fi
}

# Function to run pytest for POSIX target
function run_pytest_posix_target() {
    echo "Running POSIX pytest"
    eval "${POSIX_PYTEST_CMDLINE}" | tee -a "${PYTEST_RESULTS_FILE}"
    if [ "${PIPESTATUS[0]}" -ne 0 ]; then
        echo "ERROR: ${POSIX_PYTEST_CMDLINE} failed"
        # Don't exit, allow tests and storage to complete.
        exit 0
    fi
}

# Function to build unit tests
function build_unit_tests() {
    echo "Building unit tests"
    if ! eval "${UNIT_TESTS_CMDLINE}"
    then
        echo "ERROR: ${UNIT_TESTS_CMDLINE} failed"
        exit 1
    fi
}

# Function to list unit tests
function list_unit_tests() {
    echo "List unit tests"
    eval "${LIST_UNIT_TESTS_CMDLINE}" | tee -a "${UNIT_TESTS_LIST_FILE}"
    if [ "${PIPESTATUS[0]}" -ne 0 ]; then
        echo "ERROR: ${LIST_UNIT_TESTS_CMDLINE} failed"
        exit 1
    fi
}

# Function to generate documentation
function build_documentation() {
    echo "Building unit tests"
    if ! eval "${BUILD_DOCUMENTATION_CMDLINE}"
    then
        echo "ERROR: ${BUILD_DOCUMENTATION_CMDLINE} failed"
        exit 1
    fi
}

# Function to run unit tests
function run_unit_tests() {
    echo "Running unit tests"
    eval "${RUN_UNIT_TESTS_CMDLINE}" | tee -a "${UNIT_TESTS_RESULTS_FILE}"
    if [ "${PIPESTATUS[0]}" -ne 0 ]; then
        echo "ERROR: ${RUN_UNIT_TESTS_CMDLINE} failed"
        exit 1
    fi
}

# Function to build the NXP S32K148 target
function build_nxp_target() {
    echo "Building NXP S32K148 target"
    if ! eval "${NXP_S32K148_BUILD_CMDLINE}"
    then
        echo "ERROR: ${NXP_S32K148_BUILD_CMDLINE} failed"
        exit 1
    fi
}

# Change directory to the root of the OpenBSW repository
cd "${OPENBSW_GIT_DIR}" || exit

# List available tests.
if ${LIST_UNIT_TESTS}; then
    list_unit_tests
fi

# Create documentation
if ${BUILD_DOCUMENTATION}; then
    build_documentation
fi

# Build and run unit tests if enabled
if ${BUILD_UNIT_TESTS}; then
    build_unit_tests
fi

# Run unit tests (assumes Build was run)
if ${RUN_UNIT_TESTS}; then
    run_unit_tests
fi

# Build the POSIX target if enabled
if ${BUILD_POSIX}; then
    build_posix_target
fi

# Run POSIX pytest if enabled
if ${POSIX_PYTEST}; then
    run_pytest_posix_target
fi

# Build the NXP S32K148 target if enabled
if ${BUILD_NXP_S32K148}; then
    build_nxp_target
fi

# Execute post build commands if any
if [ "${#POST_BUILD_COMMANDS[@]}" -gt 0 ]; then
    echo "Post build commands:"
    for command in "${POST_BUILD_COMMANDS[@]}"; do
        echo "${command}"
        eval "${command}"
    done
fi

exit 0
