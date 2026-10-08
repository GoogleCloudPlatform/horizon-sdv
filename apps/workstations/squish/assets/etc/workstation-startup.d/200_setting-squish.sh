#!/bin/bash

# Copyright (C) 2026 The Qt Company Ltd.
# All rights reserved.
#
# This file is provided AS IS with NO WARRANTY OF ANY KIND, INCLUDING THE
# WARRANTY OF DESIGN, MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE.

set -euo pipefail

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
      --secret=${SQUISH_GCP_LICENSE_SECRET}
  )
  jq \
    -n \
    --arg key "${SQUISH_LICENSE_KEY}" \
    '{format: "local", key: $key}' \
    > "${_HOME_DIR}/${_SQUISH_LICENSE_NAME}"
}

squish_make_symlink() {
    ln -s "${_SQUISH_DIR}" "${_HOME_DIR}"
}

main() {
  echo "Setting Squish..."
  squish_fetch_license_key
  squish_make_symlink
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
