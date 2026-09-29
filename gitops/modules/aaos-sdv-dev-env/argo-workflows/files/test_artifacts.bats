#!/usr/bin/env bats
# Copyright (c) 2026 Accenture, All Rights Reserved.
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
#
# Acceptance tests for the aaos-sdv-dev-env workstation image and its persistent
# disk snapshot. These run ON a temporary Cloud Workstation booted from the
# candidate snapshot, before that snapshot is promoted to "latest".
#
# They deliberately exercise the workstation the way a developer would — via an
# interactive shell that sources ~/.bashrc — because the aliases and helper
# functions under test (launch_2vm, clear_2vm) are only defined there.
#
# Ported from sdv-setup/pipelines/tests/test_artifacts.bats.

setup() {
    export CARLA_DIR="${CARLA_DIR:-${HOME}/Workspace/carla-installation}"
    export ANDROID_BUILD_TOP="${ANDROID_BUILD_TOP:-${HOME}/Workspace/aaos-26q2}"
}

@test "CARLA 0.9.15 simulator starts and binds RPC socket on port 2000" {
    [ -d "${CARLA_DIR}" ]
    [ -x "${CARLA_DIR}/CarlaUE4.sh" ]

    # Launch CARLA in background headless offscreen mode (with GPU)
    "${CARLA_DIR}/CarlaUE4.sh" -RenderOffScreen -nosound -carla-server -carla-rpc-port=2000 </dev/null >/dev/null 2>&1 &
    local carla_pid=$!

    # Wait up to 30s for port 2000 to open
    local port_open=0
    for i in {1..30}; do
        if timeout 1 bash -c "</dev/tcp/127.0.0.1/2000" 2>/dev/null; then
            port_open=1
            break
        fi
        sleep 1
    done

    # Clean up process
    pkill -9 -f CarlaUE4 2>/dev/null || true
    kill -9 "${carla_pid}" 2>/dev/null || true
    wait "${carla_pid}" 2>/dev/null || true

    [ "$port_open" -eq 1 ]
}

@test "launch_2vm alias executes and launches CVD instances successfully" {
    [ -d "${ANDROID_BUILD_TOP}" ]

    cd "${ANDROID_BUILD_TOP}"

    # Clean up any lingering instances before starting
    bash -ic "clear_2vm" || true

    # Execute launch_2vm via an interactive shell that sources ~/.bashrc,
    # matching the real user experience on the workstation.
    run bash -ic "launch_2vm"
    local exit_status=$status
    echo "launch_2vm status: ${exit_status}"
    echo "launch_2vm output: ${output}"

    [ "$exit_status" -eq 0 ]

    # Verify cvd fleet lists running instances vm0 and vm1
    run bash -ic "cvd fleet"
    echo "cvd fleet status: ${status}"
    echo "cvd fleet output: ${output}"
    [ "$status" -eq 0 ]
    [[ "$output" =~ "vm0" ]]
    [[ "$output" =~ "vm1" ]]

    # Clean up instances after test
    bash -ic "clear_2vm" || true
}
