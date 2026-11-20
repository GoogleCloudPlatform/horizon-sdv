#!/usr/bin/env bash

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

# Description:
#   Installs Horizon CLI on the default PATH (/usr/local/bin/horizon) during
#   Cloud Workstation image build. Prefers a pre-built binary or tools/horizon
#   sources. Does not call Developer Portal /api/cli (that endpoint requires OIDC).
#
# Usage:
#   INSTALL_HORIZON_CLI=true \
#   HORIZON_CLI_BIN=<optional pre-built binary> \
#   HORIZON_CLI_SRC=<optional tools/horizon directory> \
#   HORIZON_CLI_URL=<optional direct binary URL> \
#   HORIZON_CLI_SHA256=<optional hex for URL downloads> \
#   install-horizon-cli.sh
#
# Output:
#   /usr/local/bin/horizon

set -euo pipefail

INSTALL_HORIZON_CLI="${INSTALL_HORIZON_CLI:-true}"
TARGET_BIN="/usr/local/bin/horizon"
tmpdir=""

log() { echo "[install-horizon-cli] $*"; }
die() { echo "[install-horizon-cli] ERROR: $*" >&2; exit 1; }

cleanup() { rm -rf "${tmpdir:-}"; }
trap cleanup EXIT

install_from_file() {
  local src="$1"
  [[ -f "${src}" ]] || die "binary not found: ${src}"
  install -d "$(dirname "${TARGET_BIN}")"
  install -m 0755 "${src}" "${TARGET_BIN}"
}

install_from_source() {
  local src="$1"
  [[ -f "${src}/go.mod" ]] || die "go.mod not found in ${src}"
  command -v go >/dev/null 2>&1 || die "go is required to build Horizon CLI from source"
  tmpdir="$(mktemp -d)"
  log "Building Horizon CLI from ${src}"
  (cd "${src}" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${tmpdir}/horizon" .)
  install_from_file "${tmpdir}/horizon"
}

install_from_url() {
  local url="$1"
  command -v curl >/dev/null 2>&1 || die "curl is required"
  tmpdir="$(mktemp -d)"
  local dest="${tmpdir}/horizon"
  log "Downloading Horizon CLI from ${url}"
  curl -fsSL --retry 3 --connect-timeout 10 --max-time 300 "${url}" -o "${dest}"
  if [[ -n "${HORIZON_CLI_SHA256:-}" ]]; then
    command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required when HORIZON_CLI_SHA256 is set"
    local expected actual
    expected="$(echo "${HORIZON_CLI_SHA256}" | sed 's/^sha256://I' | tr 'A-F' 'a-f')"
    actual="$(sha256sum "${dest}" | awk '{print $1}')"
    [[ "${actual}" == "${expected}" ]] || die "SHA256 mismatch: expected ${expected}, got ${actual}"
  fi
  install_from_file "${dest}"
}

install_horizon_cli() {
  if [[ "${INSTALL_HORIZON_CLI}" != "true" ]]; then
    log "Horizon CLI installation skipped."
    return 0
  fi

  command -v install >/dev/null 2>&1 || die "install is required"

  if [[ -n "${HORIZON_CLI_BIN:-}" ]]; then
    log "Installing Horizon CLI from ${HORIZON_CLI_BIN}"
    install_from_file "${HORIZON_CLI_BIN}"
  elif [[ -n "${HORIZON_CLI_SRC:-}" ]]; then
    install_from_source "${HORIZON_CLI_SRC}"
  elif [[ -n "${HORIZON_CLI_URL:-}" ]]; then
    install_from_url "${HORIZON_CLI_URL}"
  else
    die "Set HORIZON_CLI_BIN, HORIZON_CLI_SRC, or HORIZON_CLI_URL (do not use portal /api/cli from image build)"
  fi

  command -v horizon >/dev/null 2>&1 || die "horizon is not on PATH after install"
  log "Installed $(command -v horizon)"
  horizon version
}

main() {
  install_horizon_cli
}

main "$@"
