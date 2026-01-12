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

# Enable debug mode by setting DEBUG=1
set -euo pipefail

DEBUG="${DEBUG:-0}"

debug_log() {
  if [ "${DEBUG}" -eq 1 ] 2>/dev/null; then
    echo "[DEBUG]" "$@"
  fi
}

error_exit() {
  local rc=$?
  # last_cmd captured by DEBUG-trap
  echo "[ERROR] Script failed at line ${BASH_LINENO[0]} - last command: '${last_cmd:-unknown}' (exit ${rc})" >&2
  if [ "${DEBUG}" -eq 1 ] 2>/dev/null; then
    echo "[DEBUG] Exiting with code ${rc}" >&2
  fi
  exit "${rc}"
}

trap 'last_cmd=$BASH_COMMAND' DEBUG
trap 'error_exit' ERR

# Variables and service account
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
APISERVER="https://kubernetes.default.svc"

debug_log "========================"
debug_log "DEBUG mode enabled"
debug_log "Starting post-job script"
debug_log "========================"

# Check SA files exist
if [ ! -f "${SERVICEACCOUNT}/namespace" ] || [ ! -f "${SERVICEACCOUNT}/token" ] || [ ! -f "${SERVICEACCOUNT}/ca.crt" ]; then
  echo "[ERROR] ServiceAccount files missing under ${SERVICEACCOUNT}" >&2
  exit 2
fi

NAMESPACE="$(cat "${SERVICEACCOUNT}/namespace")"
debug_log "NAMESPACE=${NAMESPACE}"

# Check kubectl can reach cluster
debug_log "Checking kubectl connectivity..."
if ! kubectl version >/dev/null 2>&1; then
  echo "[ERROR] kubectl cannot reach API server or is misconfigured" >&2
  exit 3
fi
debug_log "kubectl connectivity OK"

# Run node script
debug_log "Installing npm packages"
npm install --silent

debug_log "===================="
debug_log "Running keycloak.mjs"
debug_log "===================="
node keycloak.mjs
debug_log "==================="
debug_log "keycloak.mjs finish"
debug_log "==================="

# Client secret extraction
if [ ! -f client-headlamp.json ]; then
  echo "[ERROR] client-headlamp.json not found" >&2
  exit 4
fi
debug_log "client-headlamp.json found"

CLIENT_SECRET="$(jq -r '.secret // empty' client-headlamp.json || true)"
if [ -z "${CLIENT_SECRET}" ]; then
  echo "[ERROR] client-headlamp.json missing '.secret' field or it is empty" >&2
  exit 5
fi
debug_log "Client secret present (value redacted)"

# Cookie secret generation
debug_log "Generating cookie secret"
COOKIE_SECRET="$(openssl rand -base64 32 | tr -- '+/' '-_' )" || {
  echo "[ERROR] cookie secret generation failed (openssl failed)" >&2
  exit 6
}
# Cookie secret length check (base64 32 -> ~44 chars)
if [ "${#COOKIE_SECRET}" -lt 16 ]; then
  echo "[ERROR] cookie secret appears too short" >&2
  exit 6
fi
debug_log "Cookie secret generated (value redacted)"

# Check secret exists before patch
TARGET_NS="headlamp"
TARGET_SECRET="headlamp-oauth2-proxy"
debug_log "Ensuring target secret ${TARGET_SECRET} exists in ${TARGET_NS}"
if ! kubectl -n "${TARGET_NS}" get secret "${TARGET_SECRET}" >/dev/null 2>&1; then
  echo "[ERROR] target secret ${TARGET_SECRET} not found in namespace ${TARGET_NS}" >&2
  exit 7
fi
debug_log "Target secret exists"

# Patch cookie-secret
debug_log "Patching cookie-secret into ${TARGET_SECRET}"
if ! kubectl -n "${TARGET_NS}" patch secret "${TARGET_SECRET}" --patch "{\"stringData\": { \"cookie-secret\": \"${COOKIE_SECRET}\" }}" >/dev/null 2>&1; then
  echo "[ERROR] Failed to patch cookie-secret into secret/${TARGET_SECRET}" >&2
  exit 8
fi
debug_log "Patched cookie-secret"

# Patch client-secret
debug_log "Patching client-secret into ${TARGET_SECRET} (value redacted)"
if ! kubectl -n "${TARGET_NS}" patch secret "${TARGET_SECRET}" --patch "{\"stringData\": { \"client-secret\": \"${CLIENT_SECRET}\" }}" >/dev/null 2>&1; then
  echo "[ERROR] Failed to patch client-secret into secret/${TARGET_SECRET}" >&2
  exit 9
fi
debug_log "Patched client-secret"

# Verify secret keys exist
debug_log "Verifying secret keys present (not printing values)"
if ! kubectl -n "${TARGET_NS}" get secret "${TARGET_SECRET}" -o jsonpath='{.data}' >/dev/null 2>&1; then
  echo "[ERROR] Failed to verify secret/${TARGET_SECRET} after patch" >&2
  exit 10
fi
debug_log "Secret verification OK"

# Rollout restart
debug_log "Restarting deployment/headlamp-oauth2-proxy -n ${TARGET_NS}"
if ! kubectl rollout restart deployment/headlamp-oauth2-proxy -n "${TARGET_NS}" >/dev/null 2>&1; then
  echo "[ERROR] Failed to restart deployment/headlamp-oauth2-proxy in ${TARGET_NS}" >&2
  exit 11
fi
debug_log "Deployment rollout restart requested"

debug_log "======================"
debug_log "Post-job script Finish"
debug_log "======================"

echo "All steps finished successfully"
exit 0

