#!/bin/bash

# Copyright (c) 2026 The Qt Company Ltd., All Rights Reserved.
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

set -euo pipefail

# shellcheck disable=SC1091
source "/google/scripts/common.sh"

_HOME_DIR="/home/${WORKSTATION_USER}"
_QT_DIR="/opt/qt"
_SQUISH_DIR="${_QT_DIR}/squish"
_SQUISH_GCP_LICENSE_SECRET_NAME=".squish-gcp-license-secret"
_SQUISH_LICENSE_NAME=".squish-license"

squish_fetch_license_key() {
  SQUISH_GCP_LICENSE_SECRET=$(
    cat "${_QT_DIR}/${_SQUISH_GCP_LICENSE_SECRET_NAME}"
  )
  SQUISH_LICENSE_KEY=$(
    gcloud \
      secrets \
      versions \
      access \
      latest \
      --secret="${SQUISH_GCP_LICENSE_SECRET}"
  )
  (
    umask 077
    jq \
      -n \
      --arg key "${SQUISH_LICENSE_KEY}" \
      '{format: "local", key: $key}' \
      > "${_HOME_DIR}/${_SQUISH_LICENSE_NAME}"
  )
  chmod 600 "${_HOME_DIR}/${_SQUISH_LICENSE_NAME}"
  chown "${WORKSTATION_USER}:" "${_HOME_DIR}/${_SQUISH_LICENSE_NAME}"
}

# /home is a persistent disk: the link survives restarts, so (re)create it idempotently.
squish_make_symlink() {
    ln -sfn "${_SQUISH_DIR}" "${_HOME_DIR}/squish"
}

main() {
  echo "Setting Squish..."
  squish_fetch_license_key
  squish_make_symlink
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
