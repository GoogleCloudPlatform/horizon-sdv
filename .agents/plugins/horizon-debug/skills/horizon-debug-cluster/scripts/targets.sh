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
# Remembers which GCP project, account and fleet membership serve each
# Horizon hostname, so later connections can skip the project scan.
#
# Usage:
#   targets.sh lookup <URL|HOSTNAME|PROJECT_ID>
#   targets.sh save <HOSTNAME> <PROJECT_ID> <ACCOUNT> <MEMBERSHIP> <LOCATION>
#   targets.sh remove <HOSTNAME>
#   targets.sh list
#   targets.sh path
#
# Entries live outside the skill, in
# ${XDG_CONFIG_HOME:-$HOME/.config}/horizon-cluster-connect/targets.tsv
# (override with HORIZON_TARGETS_FILE), one tab-separated line per hostname:
#   hostname  project_id  account  membership  location
# On Windows this is the WSL home directory. The file is created mode 600
# because it holds account emails.
#
# lookup prints one block of HOSTNAME= PROJECT_ID= ACCOUNT= MEMBERSHIP=
# LOCATION= CONTEXT= lines per matching entry (a project ID can match several
# hostnames). Exit codes: 0 found/done, 1 usage error, 2 not found.
set -uo pipefail

TARGETS_FILE="${HORIZON_TARGETS_FILE:-${XDG_CONFIG_HOME:-$HOME/.config}/horizon-cluster-connect/targets.tsv}"
TAB="$(printf '\t')"

usage() {
  sed -n '5,10p' "$0" | sed 's/^# \{0,1\}//' >&2
  exit 1
}

# Reduces a URL to its lowercase hostname, matching resolve-target.sh.
normalize_host() {
  local h="$1"
  h="${h#*://}"
  h="${h%%/*}"
  h="${h%%\?*}"
  h="${h##*@}"
  h="${h%%:*}"
  h="${h%.}"
  echo "$h" | tr '[:upper:]' '[:lower:]'
}

# Rewrites the file without lines whose hostname is $1, via a temp file in
# the same directory so an interrupted write never leaves a partial file.
write_without() {
  local host="$1" tmp
  [[ -f "$TARGETS_FILE" ]] || return 0
  tmp="$(mktemp "$TARGETS_FILE.XXXXXX")" || return 1
  awk -F'\t' -v h="$host" '$1 != h' "$TARGETS_FILE" > "$tmp" \
    && chmod 600 "$tmp" && mv "$tmp" "$TARGETS_FILE" \
    || { rm -f "$tmp"; return 1; }
}

cmd="${1:-}"
[[ -n "$cmd" ]] || usage
shift

case "$cmd" in
  path)
    echo "$TARGETS_FILE"
    ;;

  list)
    [[ -f "$TARGETS_FILE" ]] || exit 2
    grep -v '^#' "$TARGETS_FILE"
    ;;

  lookup)
    [[ $# -eq 1 ]] || usage
    [[ -f "$TARGETS_FILE" ]] || exit 2
    # GCP project IDs never contain '.' or '/'; URLs and hostnames always do.
    if [[ "$1" != *.* && "$1" != */* ]]; then
      col=2 key="$1"
    else
      col=1 key="$(normalize_host "$1")"
    fi
    out="$(awk -F'\t' -v c="$col" -v k="$key" '
      /^#/ || NF < 5 { next }
      $c == k {
        printf "HOSTNAME=%s\nPROJECT_ID=%s\nACCOUNT=%s\nMEMBERSHIP=%s\nLOCATION=%s\nCONTEXT=connectgateway_%s_%s_%s\n",
          $1, $2, $3, $4, $5, $2, $5, $4
      }' "$TARGETS_FILE")"
    [[ -n "$out" ]] || exit 2
    echo "$out"
    ;;

  save)
    [[ $# -eq 5 ]] || usage
    host="$(normalize_host "$1")"
    for v in "$host" "$2" "$3" "$4" "$5"; do
      if [[ -z "$v" || "$v" == *"$TAB"* || "$v" == *$'\n'* ]]; then
        echo "Values must be non-empty and contain no tabs or newlines." >&2
        exit 1
      fi
    done
    umask 077
    mkdir -p "$(dirname "$TARGETS_FILE")" || exit 1
    if [[ ! -f "$TARGETS_FILE" ]]; then
      printf '# hostname\tproject_id\taccount\tmembership\tlocation\n' > "$TARGETS_FILE" || exit 1
      chmod 600 "$TARGETS_FILE"
    fi
    write_without "$host" || { echo "Could not update $TARGETS_FILE" >&2; exit 1; }
    printf '%s\t%s\t%s\t%s\t%s\n' "$host" "$2" "$3" "$4" "$5" >> "$TARGETS_FILE" || exit 1
    echo "SAVED=$host"
    ;;

  remove)
    [[ $# -eq 1 ]] || usage
    host="$(normalize_host "$1")"
    write_without "$host" || { echo "Could not update $TARGETS_FILE" >&2; exit 1; }
    echo "REMOVED=$host"
    ;;

  *)
    usage
    ;;
esac
