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
# Report what is present or missing for Connect Gateway + kubectl against GKE.
# Exit 0 only if nothing blocking is missing. Exit 1 if connection cannot work yet.
#
# Usage: check-prerequisites.sh [--account EMAIL]
#   --account checks that account's credentials instead of the active one's.
set -uo pipefail

case "${1:-}" in
  --account)
    [[ -n "${2:-}" ]] || { echo "Usage: $(basename "$0") [--account EMAIL]" >&2; exit 1; }
    export CLOUDSDK_CORE_ACCOUNT="$2" ;;
  "") ;;
  *) echo "Usage: $(basename "$0") [--account EMAIL]" >&2; exit 1 ;;
esac

BLOCKING=0
WARNINGS=0

pass() { echo "[OK]   $*"; }
fail() { echo "[FAIL] $*"; BLOCKING=$((BLOCKING + 1)); }
warn() { echo "[WARN] $*"; WARNINGS=$((WARNINGS + 1)); }

echo "=== Prerequisite check (Horizon cluster / Connect Gateway) ==="
echo ""

# --- gcloud CLI ---
if ! command -v gcloud >/dev/null 2>&1; then
  fail "gcloud not found in PATH. Install Google Cloud SDK: https://cloud.google.com/sdk/docs/install"
else
  pass "gcloud: $(command -v gcloud)"
  if ! gcloud version >/dev/null 2>&1; then
    fail "gcloud is present but 'gcloud version' failed — repair or reinstall Cloud SDK"
  fi
fi

# --- kubectl ---
if ! command -v kubectl >/dev/null 2>&1; then
  fail "kubectl not found in PATH. Install with: gcloud components install kubectl"
else
  pass "kubectl: $(command -v kubectl)"
  if ! kubectl version --client >/dev/null 2>&1; then
    fail "kubectl is present but 'kubectl version --client' failed"
  fi
fi

# --- GKE auth plugin (required for Connect Gateway kubeconfig exec) ---
if ! command -v gke-gcloud-auth-plugin >/dev/null 2>&1; then
  fail "gke-gcloud-auth-plugin not in PATH. Install with: gcloud components install gke-gcloud-auth-plugin"
else
  pass "gke-gcloud-auth-plugin: $(command -v gke-gcloud-auth-plugin)"
fi

# --- dig (used by resolve-target.sh tiebreaker; optional) ---
if command -v dig >/dev/null 2>&1; then
  pass "dig: $(command -v dig)"
else
  warn "dig not found; URL lookups cannot use the IP tiebreaker when several projects match"
fi

# --- User credentials (not ADC) ---
if command -v gcloud >/dev/null 2>&1; then
  if ! gcloud auth print-access-token >/dev/null 2>&1; then
    fail "gcloud user credentials invalid or expired. Run: gcloud auth login"
  else
    pass "gcloud user token: valid (gcloud auth print-access-token)"
  fi

  active_acct="$(gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | head -1)"
  if [[ -z "${active_acct:-}" ]]; then
    fail "No ACTIVE gcloud account. Run: gcloud auth login   then: gcloud config set account YOUR_EMAIL"
  else
    pass "Active gcloud account: $active_acct"
  fi

  proj="$(gcloud config get-value project 2>/dev/null || true)"
  if [[ -z "${proj:-}" ]]; then
    warn "No default gcloud project (core/project). Set with: gcloud config set project PROJECT_ID"
  else
    pass "Default gcloud project: $proj"
  fi
fi

# --- Optional: ADC (kubectl via GKE exec uses the gcloud user, not ADC) ---
if command -v gcloud >/dev/null 2>&1; then
  if gcloud auth application-default print-access-token >/dev/null 2>&1; then
    pass "Application Default Credentials: valid (optional for this kubectl path)"
  else
    warn "Application Default Credentials missing or expired. Optional here; fix with: gcloud auth application-default login"
  fi
fi

echo ""
echo "--- Summary ---"
if [[ "$BLOCKING" -gt 0 ]]; then
  echo "Status: NOT READY — $BLOCKING blocking issue(s). Fix [FAIL] items above before connecting."
  [[ "$WARNINGS" -gt 0 ]] && echo "Also $WARNINGS warning(s) ([WARN])."
  exit 1
fi
echo "Status: READY for kubeconfig merge (gcloud auth + CLI tools OK)."
[[ "$WARNINGS" -gt 0 ]] && echo "Note: $WARNINGS non-blocking warning(s) ([WARN]) — connection may still work."
exit 0
