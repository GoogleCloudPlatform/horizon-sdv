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

# Shared helpers for run.sh and scenarios.sh. Every kubectl and helm call goes through k / h, which pin the dedicated
# kubeconfig and context; the default kubeconfig and the caller's KUBECONFIG are never used.

CLUSTER_NAME="${CLUSTER_NAME:-storage-mirror-e2e}"
WORK_DIR="${WORK_DIR:-/tmp/storage-mirror-e2e}"
KCFG="${WORK_DIR}/kubeconfig"
KCTX="kind-${CLUSTER_NAME}"
TOOLS_BIN="${TOOLS_BIN:-/tmp/cs-tools}"
KIND="${KIND:-${TOOLS_BIN}/kind}"
HELM="${HELM:-${TOOLS_BIN}/helm}"
KUBECTL="${KUBECTL:-kubectl}"
unset KUBECONFIG

log() { printf '\n[e2e %s] %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
die() { printf '\n[e2e] FATAL: %s\n' "$*" >&2; exit 1; }

k() { "${KUBECTL}" --kubeconfig "${KCFG}" --context "${KCTX}" "$@"; }
h() { "${HELM}" --kubeconfig "${KCFG}" --kube-context "${KCTX}" "$@"; }

# assert_local_cluster refuses to continue unless the dedicated kubeconfig targets a loopback API server.
assert_local_cluster() {
  [[ -f "${KCFG}" ]] || die "kubeconfig ${KCFG} missing"
  local server
  server="$("${KUBECTL}" --kubeconfig "${KCFG}" config view --minify --context "${KCTX}" -o jsonpath='{.clusters[0].cluster.server}')"
  [[ "${server}" =~ ^https://(127\.0\.0\.1|localhost|\[::1\]):[0-9]+$ ]] || die "refusing to use API server '${server}' (not loopback)"
}

# wait_for CMD... retries until CMD succeeds or TIMEOUT (seconds, default 300) passes.
wait_for() {
  local timeout="${TIMEOUT:-300}" start
  start=$(date +%s)
  until "$@" >/dev/null 2>&1; do
    if (( $(date +%s) - start > timeout )); then
      return 1
    fi
    sleep "${POLL:-3}"
  done
}

RESULTS=()
pass() { RESULTS+=("PASS  $*"); log "PASS: $*"; }
fail() { RESULTS+=("FAIL  $*"); log "FAIL: $*"; FAILED=1; }
skip() { RESULTS+=("SKIP  $*"); log "SKIP: $*"; }
check() {
  local name=$1
  shift
  if "$@"; then pass "${name}"; else fail "${name}"; fi
}
summary() {
  printf '\n==== storage-mirror e2e results ====\n'
  printf '%s\n' "${RESULTS[@]}"
  printf '====================================\n'
}
