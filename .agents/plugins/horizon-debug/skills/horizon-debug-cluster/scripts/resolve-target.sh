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
# Resolve a Horizon target (cluster URL or GCP project ID) to its GCP project.
#
# Usage: resolve-target.sh [--account EMAIL] [--all] [--parallel N] <URL|PROJECT_ID>
#
# Results go to stdout as KEY=VALUE lines; progress and errors go to stderr.
#   Project ID input: PROJECT_ID=... plus one HOSTNAME=... per cert-map entry
#   URL input:        PROJECT_ID=... and HOSTNAME=...
#   Ambiguous URL:    one CANDIDATE=... per matching project
#   Always on success: ACCOUNT=... (the gcloud account used for the lookup)
#
# --account runs every gcloud call as that credentialed account without
# changing the active gcloud configuration.
#
# Exit codes: 0 resolved, 1 usage/tooling error, 2 no match, 3 ambiguous.
set -uo pipefail

CERT_MAP="horizon-sdv-map"
PARALLEL=16
SCAN_ALL=0

usage() {
  echo "Usage: $(basename "$0") [--account EMAIL] [--all] [--parallel N] <URL|PROJECT_ID>" >&2
  exit 1
}

# Prints the hostnames in a project's Horizon cert map; fails if the map is
# missing, the Certificate Manager API is disabled, or access is denied.
cert_map_hostnames() {
  gcloud certificate-manager maps entries list \
    --map="$CERT_MAP" --location=global --project="$1" \
    --format="value(hostname)" 2>/dev/null
}

# Echoes "exact" or "wildcard" if $1 (hostname) is served by entry $2.
host_matches_entry() {
  local host="$1" entry="$2"
  if [[ "$host" == "$entry" ]]; then
    echo exact
  elif [[ "$entry" == '*.'* ]]; then
    local suffix="${entry#\*.}"
    local label="${host%."$suffix"}"
    if [[ "$label" != "$host" && -n "$label" && "$label" != *.* ]]; then
      echo wildcard
    fi
  fi
}

# Internal worker invoked via xargs: checks one project for the hostname.
if [[ "${1:-}" == "--check-project" ]]; then
  project="$2" host="$3" workdir="$4"
  if [[ "$SCAN_ALL_FLAG" != 1 && -n "$(ls "$workdir/match" 2>/dev/null)" ]]; then
    touch "$workdir/done/$project"
    exit 0
  fi
  best=""
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    kind="$(host_matches_entry "$host" "$entry")"
    if [[ "$kind" == exact ]]; then best=exact; break; fi
    [[ "$kind" == wildcard ]] && best=wildcard
  done < <(cert_map_hostnames "$project")
  [[ -n "$best" ]] && echo "$best" > "$workdir/match/$project"
  touch "$workdir/done/$project"
  exit 0
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --all) SCAN_ALL=1; shift ;;
    --parallel) [[ $# -ge 2 ]] || usage; PARALLEL="$2"; shift 2 ;;
    --account) [[ $# -ge 2 ]] || usage; export CLOUDSDK_CORE_ACCOUNT="$2"; shift 2 ;;
    -h|--help) usage ;;
    -*) usage ;;
    *) break ;;
  esac
done
[[ $# -eq 1 ]] || usage
INPUT="$1"

command -v gcloud >/dev/null 2>&1 || { echo "gcloud not found in PATH" >&2; exit 1; }

ACCOUNT="${CLOUDSDK_CORE_ACCOUNT:-$(gcloud config get-value account 2>/dev/null)}"
if [[ -z "$ACCOUNT" ]]; then
  echo "No active gcloud account. Run: gcloud auth login" >&2
  exit 1
fi
if [[ -z "$(gcloud auth list --filter="account:$ACCOUNT" --format="value(account)" 2>/dev/null)" ]]; then
  echo "Account $ACCOUNT is not signed in to gcloud. Run: gcloud auth login $ACCOUNT" >&2
  exit 1
fi
if ! gcloud auth print-access-token >/dev/null 2>&1; then
  echo "Credentials for $ACCOUNT are expired. Run: gcloud auth login $ACCOUNT" >&2
  exit 1
fi

# Lists other signed-in accounts the user could retry with.
other_accounts_hint() {
  local others
  others="$(gcloud auth list --format="value(account)" 2>/dev/null | grep -vxF "$ACCOUNT")"
  [[ -z "$others" ]] && return
  echo "Other signed-in gcloud accounts you could retry with (--account EMAIL):" >&2
  echo "$others" | sed 's/^/  - /' >&2
}

# GCP project IDs never contain '.' or '/'; URLs and hostnames always do.
if [[ "$INPUT" != *.* && "$INPUT" != */* ]]; then
  PROJECT_ID="$INPUT"
  if ! gcloud projects describe "$PROJECT_ID" --format="value(projectId)" >/dev/null 2>&1; then
    echo "Project '$PROJECT_ID' not found or not accessible with $ACCOUNT." >&2
    other_accounts_hint
    exit 2
  fi
  echo "PROJECT_ID=$PROJECT_ID"
  echo "ACCOUNT=$ACCOUNT"
  hostnames="$(cert_map_hostnames "$PROJECT_ID" | grep -v '^\*\.' || true)"
  if [[ -z "$hostnames" ]]; then
    echo "No '$CERT_MAP' entries found in $PROJECT_ID; cannot show the cluster URL." >&2
    exit 0
  fi
  while IFS= read -r h; do echo "HOSTNAME=$h"; done <<< "$hostnames"
  exit 0
fi

HOST="$INPUT"
HOST="${HOST#*://}"
HOST="${HOST%%/*}"
HOST="${HOST%%\?*}"
HOST="${HOST##*@}"
HOST="${HOST%%:*}"
HOST="${HOST%.}"
HOST="$(echo "$HOST" | tr '[:upper:]' '[:lower:]')"
[[ -n "$HOST" ]] || { echo "Could not extract a hostname from '$INPUT'" >&2; exit 1; }

echo "Resolving https://$HOST ..." >&2
projects="$(gcloud projects list --format="value(projectId)" 2>/dev/null)"
if [[ -z "$projects" ]]; then
  echo "gcloud projects list returned nothing; check 'gcloud auth login'." >&2
  exit 1
fi
total="$(echo "$projects" | wc -l | tr -d ' ')"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT
mkdir -p "$WORKDIR/done" "$WORKDIR/match"

echo "$projects" | SCAN_ALL_FLAG="$SCAN_ALL" xargs -P "$PARALLEL" -I{} \
  bash "$0" --check-project {} "$HOST" "$WORKDIR" &
xargs_pid=$!

while kill -0 "$xargs_pid" 2>/dev/null; do
  done_count="$(ls "$WORKDIR/done" | wc -l | tr -d ' ')"
  printf '\r  scanned %s/%s projects' "$done_count" "$total" >&2
  sleep 1
done
wait "$xargs_pid" 2>/dev/null
printf '\r  scanned %s/%s projects\n' "$(ls "$WORKDIR/done" | wc -l | tr -d ' ')" "$total" >&2

exact="$(grep -l '^exact$' "$WORKDIR"/match/* 2>/dev/null | xargs -n1 basename 2>/dev/null)"
wild="$(grep -l '^wildcard$' "$WORKDIR"/match/* 2>/dev/null | xargs -n1 basename 2>/dev/null)"
matches="${exact:-$wild}"

if [[ -z "$matches" ]]; then
  cat >&2 <<EOF
No project $ACCOUNT can access serves $HOST in its '$CERT_MAP' cert map.
Likely causes:
  - your account has no IAM role on the owning project
  - the URL is mistyped
  - the environment has been destroyed
  - the project belongs to a different gcloud account
EOF
  other_accounts_hint
  exit 2
fi

count="$(echo "$matches" | wc -l | tr -d ' ')"
if [[ "$count" -gt 1 ]]; then
  echo "$count projects serve $HOST; checking which owns its IP ..." >&2
  ip=""
  if command -v dig >/dev/null 2>&1; then
    ip="$(dig +short "$HOST" A | grep -E '^[0-9.]+$' | tail -1)"
  fi
  if [[ -n "$ip" ]]; then
    owners=""
    while IFS= read -r p; do
      rule="$(gcloud compute forwarding-rules list --global --project="$p" \
        --filter="IPAddress=$ip" --format="value(name)" 2>/dev/null)"
      [[ -n "$rule" ]] && owners="${owners:+$owners$'\n'}$p"
    done <<< "$matches"
    if [[ -n "$owners" && "$(echo "$owners" | wc -l | tr -d ' ')" -eq 1 ]]; then
      matches="$owners"
      count=1
    fi
  else
    echo "Could not resolve $HOST to an IPv4 address; skipping IP tiebreaker." >&2
  fi
fi

if [[ "$count" -gt 1 ]]; then
  echo "Still ambiguous; ask the user to choose:" >&2
  while IFS= read -r p; do echo "CANDIDATE=$p"; done <<< "$matches"
  exit 3
fi

echo "PROJECT_ID=$matches"
echo "ACCOUNT=$ACCOUNT"
echo "HOSTNAME=$HOST"
exit 0
