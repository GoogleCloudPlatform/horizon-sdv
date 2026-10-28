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

# Define variables
CURL_OPTS="${CURL_OPTS}"
GCP_PROJECT="${GCP_PROJECT:-}"
SQUISH_DOWNLOAD_URL="${SQUISH_DOWNLOAD_URL}"
SQUISH_GCP_LICENSE_SECRET="${SQUISH_GCP_LICENSE_SECRET}"
SQUISH_SHA256="${SQUISH_SHA256}"

_INSTALLER_NAME="squish-android-linux64.run"
_QT_DIR="/opt/qt"
_SQUISH_GCP_LICENSE_SECRET_NAME=".squish-gcp-license-secret"
_SQUISH_TARGET_DIR="${_QT_DIR}/squish"

umask 077
_WORK_DIR="$(mktemp -d)"
_LICENSE_KEY_PATH="${_WORK_DIR}/squish-license-key"


# Download a file and verify its SHA-256 checksum (the Horizon GNOME base has no fetch_assets.sh helper).
download_and_validate() {
  local url="$1"
  local sha256="$2"
  local output="$3"
  # shellcheck disable=SC2086
  curl ${CURL_OPTS} -o "${output}" "${url}"
  echo "${sha256}  ${output}" | sha256sum -c -
}


fetch_installer() {
  log "Fetching Squish installer..."
  download_and_validate "${SQUISH_DOWNLOAD_URL}" "${SQUISH_SHA256}" "${_WORK_DIR}/${_INSTALLER_NAME}"
  chmod 700 "${_WORK_DIR}/${_INSTALLER_NAME}"
}


fetch_license_key() {
  if [[ -z "${GCP_PROJECT}" ]]; then
    log "ERROR: GCP_PROJECT is not set; cannot read the Squish license from Secret Manager."
    return 1
  fi
  log "Fetching license key from GCP Secret Manager (project: ${GCP_PROJECT}, secret: ${SQUISH_GCP_LICENSE_SECRET})..."
  gcloud \
    secrets \
    versions \
    access \
    latest \
    --secret="${SQUISH_GCP_LICENSE_SECRET}" \
    --project="${GCP_PROJECT}" \
    --quiet \
    > "${_LICENSE_KEY_PATH}"
  if [[ ! -s "${_LICENSE_KEY_PATH}" ]]; then
    log "ERROR: Secret Manager secret ${SQUISH_GCP_LICENSE_SECRET} is empty."
    return 1
  fi
  mkdir -p "${_QT_DIR}"
  (
    umask 022
    printf "%s" \
      "${SQUISH_GCP_LICENSE_SECRET}" \
      > "${_QT_DIR}/${_SQUISH_GCP_LICENSE_SECRET_NAME}"
  )
}


install() {
  log "Running Squish installer..."
  (
    umask 022
    mkdir -p "${_SQUISH_TARGET_DIR}" "${_WORK_DIR}/home"
    cd "${_WORK_DIR}"
    HOME="${_WORK_DIR}/home" "./${_INSTALLER_NAME}" \
        unattended=1 \
        targetdir="${_SQUISH_TARGET_DIR}" \
        licensekey="$(cat "${_LICENSE_KEY_PATH}")"
  )
}


# Runs on every exit (also on failure) so the license key never stays in the image layer.
cleanup() {
  log "Removing installer and license key..."
  rm -rf "${_WORK_DIR}" /root/.squish-license /root/.squish /tmp/squish-license-key
}


main() {
  log "Begin Squish installation..."
  trap cleanup EXIT
  fetch_installer
  fetch_license_key
  install
}


if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
