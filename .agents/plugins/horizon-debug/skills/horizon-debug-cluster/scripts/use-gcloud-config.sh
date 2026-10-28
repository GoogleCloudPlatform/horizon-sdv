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
# Point gcloud at a Horizon project, optionally as a different account.
#
# Usage: use-gcloud-config.sh --project PROJECT_ID [--account EMAIL]
#
# If a configuration named horizon-<PROJECT_ID> exists, activates it (setting
# EMAIL on it when given). Otherwise, without --account or when EMAIL is
# already the active account, sets the project on the active gcloud
# configuration. Otherwise creates horizon-<PROJECT_ID> holding EMAIL and the
# project, and activates it, leaving the previous configuration untouched.
# kubectl's Connect Gateway auth uses the active configuration's account, so
# it must match the project being connected to.
#
# Prints CONFIG=<active configuration> and PREVIOUS_CONFIG=<configuration
# before the change>. Exit codes: 0 done, 1 usage/auth error.
set -uo pipefail

PROJECT=""
ACCOUNT=""

usage() {
  echo "Usage: $(basename "$0") --project PROJECT_ID [--account EMAIL]" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --project) [[ $# -ge 2 ]] || usage; PROJECT="$2"; shift 2 ;;
    --account) [[ $# -ge 2 ]] || usage; ACCOUNT="$2"; shift 2 ;;
    *) usage ;;
  esac
done
[[ -n "$PROJECT" ]] || usage

PREVIOUS_CONFIG="$(gcloud config configurations list --filter=is_active=true --format="value(name)" 2>/dev/null)"
ACTIVE_ACCOUNT="$(gcloud config get-value account 2>/dev/null)"

# Configuration names allow lowercase letters, digits and hyphens, starting with a letter.
CONFIG="horizon-$(echo "$PROJECT" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9-\n' '-')"

if [[ -n "$ACCOUNT" && -z "$(gcloud auth list --filter="account:$ACCOUNT" --format="value(account)" 2>/dev/null)" ]]; then
  echo "Account $ACCOUNT is not signed in to gcloud. Run: gcloud auth login $ACCOUNT" >&2
  exit 1
fi

if gcloud config configurations describe "$CONFIG" >/dev/null 2>&1; then
  gcloud config configurations activate "$CONFIG" >/dev/null 2>&1 \
    || { echo "Could not activate gcloud configuration $CONFIG" >&2; exit 1; }
  if [[ -n "$ACCOUNT" ]]; then
    gcloud config set account "$ACCOUNT" >/dev/null 2>&1 \
      || { echo "Could not set account on $CONFIG" >&2; exit 1; }
  fi
  gcloud config set project "$PROJECT" >/dev/null 2>&1 \
    || { echo "Could not set project on $CONFIG" >&2; exit 1; }
  echo "CONFIG=$CONFIG"
  echo "PREVIOUS_CONFIG=$PREVIOUS_CONFIG"
  exit 0
fi

if [[ -z "$ACCOUNT" || "$ACCOUNT" == "$ACTIVE_ACCOUNT" ]]; then
  gcloud config set project "$PROJECT" >/dev/null 2>&1 || { echo "Could not set project $PROJECT" >&2; exit 1; }
  echo "CONFIG=$PREVIOUS_CONFIG"
  echo "PREVIOUS_CONFIG=$PREVIOUS_CONFIG"
  exit 0
fi

gcloud config configurations create "$CONFIG" >/dev/null 2>&1 \
  || { echo "Could not create gcloud configuration $CONFIG" >&2; exit 1; }
gcloud config set account "$ACCOUNT" >/dev/null 2>&1 \
  && gcloud config set project "$PROJECT" >/dev/null 2>&1 \
  || { echo "Could not set account/project on $CONFIG" >&2; exit 1; }

echo "CONFIG=$CONFIG"
echo "PREVIOUS_CONFIG=$PREVIOUS_CONFIG"
exit 0
