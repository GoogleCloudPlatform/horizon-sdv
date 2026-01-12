#!/bin/bash
#
# Copyright 2024 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Updated entrypoint to run systemd on startup.

# Invoke startup scripts supplied in the image we are extending along with any
# scripts we choose to include.
/usr/bin/workstation-startup

log () {
  echo "$(date -u +'%b %d %H:%M:%S') $(hostname) $(basename $0)[$$]: $1"
}

# dump logs to stdout for cloud logging
{
  until journalctl -n0 >/dev/null 2>&1; do
    log "waiting for journalctl"
    sleep 1
  done
  log "following journalctl"
  journalctl -f
} &

# re-use machine id saved in home directory after initialized on first boot
# see also: /etc/workstation-startup.d/100_persist-machine-id
MACHINE_ID=$(cat /home/.workstation/machine-id 2>/dev/null)
if [ ! -z "$MACHINE_ID" ]; then
  log "starting systemd with machine id $MACHINE_ID"
  exec /sbin/init --system --unit=multi-user.target --machine-id "$MACHINE_ID"
else
  log "starting systemd"
  exec /sbin/init --system --unit=multi-user.target
fi
