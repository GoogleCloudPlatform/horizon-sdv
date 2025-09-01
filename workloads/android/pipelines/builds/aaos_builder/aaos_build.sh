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
# Build AAOS targets.
#
# This script will build the AAOS image(s) for the given target. The
# target is determined by the AAOS_LUNCH_TARGET environment variable.
#
# The following variables must be set before running this script:
#  - AAOS_LUNCH_TARGET: the target device.
#
# Optional variables:
#  - OVERRIDE_MAKE_COMMAND: the make command line to use
#
# Example usage:
# AAOS_LUNCH_TARGET=sdk_car_x86_64-ap1a-userdebug \
# ./workloads/android/pipelines/builds/aaos_builder/aaos_build.sh

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/aaos_environment.sh "$0"

if [ -n "${AAOS_MAKE_CMDLINE}" ]; then
    # Set environment variables and build target
    # shellcheck disable=SC1091
    source build/envsetup.sh
    if ! lunch "${AAOS_LUNCH_TARGET}"
    then
        echo "ERROR: lunch ${AAOS_LUNCH_TARGET}"
        exit 1
    fi

    echo "Building: $AAOS_MAKE_CMDLINE"

    # Run the build.
    eval "${AAOS_MAKE_CMDLINE}"
    RESULT=$?
else
    echo "Error: make command line undefined!"
    exit 1
fi

if (( RESULT == 0 )); then
    echo "Post build commands:"
    for command in "${POST_BUILD_COMMANDS[@]}"; do
        echo "${command}"
        eval "${command}"
    done
fi

# Return result
exit "$RESULT"
