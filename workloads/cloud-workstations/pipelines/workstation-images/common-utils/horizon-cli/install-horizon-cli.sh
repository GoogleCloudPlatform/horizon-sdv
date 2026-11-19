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
#   Downloads the Horizon CLI binary and installs it on the default PATH
#   (/usr/local/bin/horizon) during Cloud Workstation image build.
#
# Usage:
#   INSTALL_HORIZON_CLI=true \
#   HORIZON_DOMAIN=<cluster domain> \
#   HORIZON_CLI_URL=<optional direct binary URL> \
#   HORIZON_CLI_SHA256=<optional hex> \
#   HORIZON_CLI_BEARER=<optional OIDC token for portal /api/cli> \
#   install-horizon-cli.sh
#
# Output:
#   /usr/local/bin/horizon

set -euo pipefail

INSTALL_HORIZON_CLI="${INSTALL_HORIZON_CLI:-true}"
TARGET_BIN="/usr/local/bin/horizon"

log() { echo "[install-horizon-cli] $*"; }
die() { echo "[install-horizon-cli] ERROR: $*" >&2; exit 1; }

install_horizon_cli() {
  if [[ "${INSTALL_HORIZON_CLI}" != "true" ]]; then
    log "Horizon CLI installation skipped."
    return 0
  fi

  command -v curl >/dev/null 2>&1 || die "curl is required"
  command -v install >/dev/null 2>&1 || die "install is required"

  local os="linux"
  local arch
  arch="$(uname -m)"
  case "${arch}" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "unsupported architecture ${arch}" ;;
  esac

  local url="${HORIZON_CLI_URL:-}"
  if [[ -z "${url}" ]]; then
    local domain="${HORIZON_DOMAIN:-}"
    [[ -n "${domain}" ]] || die "Set HORIZON_CLI_URL or HORIZON_DOMAIN to download Horizon CLI"
    url="https://${domain}/developer-portal/api/cli/v1/download/${os}/${arch}"
  fi

  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' EXIT

  local dest="${tmpdir}/horizon"
  log "Downloading Horizon CLI from ${url}"
  local curl_args=(-fsSL --retry 3 --connect-timeout 10 --max-time 300)
  if [[ -n "${HORIZON_CLI_BEARER:-}" ]]; then
    curl_args+=(-H "Authorization: Bearer ${HORIZON_CLI_BEARER}")
  fi
  curl "${curl_args[@]}" "${url}" -o "${dest}"

  if [[ -n "${HORIZON_CLI_SHA256:-}" ]]; then
    command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required when HORIZON_CLI_SHA256 is set"
    local expected
    expected="$(echo "${HORIZON_CLI_SHA256}" | sed 's/^sha256://I' | tr 'A-F' 'a-f')"
    local actual
    actual="$(sha256sum "${dest}" | awk '{print $1}')"
    [[ "${actual}" == "${expected}" ]] || die "SHA256 mismatch: expected ${expected}, got ${actual}"
  fi

  install -d "$(dirname "${TARGET_BIN}")"
  install -m 0755 "${dest}" "${TARGET_BIN}"

  command -v horizon >/dev/null 2>&1 || die "horizon is not on PATH after install"
  log "Installed $(command -v horizon)"
  horizon version
}

main() {
  install_horizon_cli
}

main "$@"
