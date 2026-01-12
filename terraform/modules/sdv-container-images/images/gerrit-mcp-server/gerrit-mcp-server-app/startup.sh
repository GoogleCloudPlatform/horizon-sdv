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

set -e

# Paths
CONFIG_TEMPLATE="/app/gerrit-mcp-server/gerrit_config.json" # ConfigMap-mount
CONFIG_FINAL="/usr/local/lib/python3.12/site-packages/gerrit_mcp_server/gerrit_config.json" # Final config file path

# Check required env vars from Secret
if [[ -z "${GERRIT_USERNAME}" || -z "${GERRIT_PASSWORD}" ]]; then
  echo "ERROR: GERRIT_USERNAME and/or GERRIT_PASSWORD not set"
  exit 1
fi

# Confirm template exists
if [[ ! -f "${CONFIG_TEMPLATE}" ]]; then
  echo "ERROR: Config template not found at ${CONFIG_TEMPLATE}"
  exit 1
fi

# Replace placeholders in config template and write to final config location
sed \
  -e "s|##USERNAME##|${GERRIT_USERNAME}|g" \
  -e "s|##PASSWORD##|${GERRIT_PASSWORD}|g" \
  "${CONFIG_TEMPLATE}" > "${CONFIG_FINAL}"

echo "Config file written to ${CONFIG_FINAL}"

# Launch the server
exec /usr/local/bin/uvicorn gerrit_mcp_server.main:app --host 0.0.0.0 --port 6322
