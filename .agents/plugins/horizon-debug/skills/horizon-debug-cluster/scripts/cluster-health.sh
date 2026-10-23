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
# Read-only summary of problems on a Horizon SDV cluster in the current kubectl
# context: nodes, pods, workloads, PVCs, failed jobs, Argo CD apps, Gateways,
# HTTPRoutes, Config Connector (KCC) resources, Argo Workflows, ModuleCatalog,
# ExternalSecrets and recent Warning events. Read-only. Hard limit: 60 seconds.
#
# Usage: cluster-health.sh [--restarts N] [--max-items N]
#
# Exit codes: 0 no issues, 1 usage error, 4 issues found, 5 timed out or
# cluster unreadable.
set -uo pipefail

RESTART_THRESHOLD=10
MAX_ITEMS=10
TIME_LIMIT=60
KUBE_TIMEOUT="30s"

usage() {
  echo "Usage: $(basename "$0") [--restarts N] [--max-items N]" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --restarts) [[ $# -ge 2 ]] || usage; RESTART_THRESHOLD="$2"; shift 2 ;;
    --max-items) [[ $# -ge 2 ]] || usage; MAX_ITEMS="$2"; shift 2 ;;
    *) usage ;;
  esac
done

ISSUES=0
CATEGORIES=0
UNREADABLE=0

WORKDIR="$(mktemp -d)"
on_timeout() {
  echo ""
  echo "Timed out after ${TIME_LIMIT}s; results above are partial."
  echo "Verdict: UNKNOWN"
  exit 5
}
trap on_timeout USR1
( sleep "$TIME_LIMIT"; kill -USR1 $$ 2>/dev/null ) >/dev/null 2>&1 &
WATCHDOG=$!
trap 'kill $WATCHDOG 2>/dev/null; rm -rf "$WORKDIR"' EXIT

k() { kubectl --request-timeout="$KUBE_TIMEOUT" "$@"; }

# Fetch everything in parallel; each result lands in $WORKDIR/<name>, and a
# matching <name>.err file marks a failed or unsupported query.
fetch() {
  local name="$1"; shift
  ( k "$@" > "$WORKDIR/$name" 2>/dev/null || touch "$WORKDIR/$name.err" ) &
}

fetch nodes get nodes --no-headers
fetch pods get pods -A --no-headers
fetch workloads get deployments,statefulsets -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.kind}{" "}{.metadata.name}{" "}{.spec.replicas}{" "}{.status.readyReplicas}{"\n"}{end}'
fetch pvcs get pvc -A --no-headers
fetch jobs get jobs -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.conditions[?(@.type=="Failed")].status}{" "}{.status.conditions[?(@.type=="Failed")].reason}{"\n"}{end}'
fetch apps get applications -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.sync.status}{" "}{.status.health.status}{"\n"}{end}'
fetch gateways get gateways -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.conditions[?(@.type=="Programmed")].status}{"\n"}{end}'
fetch externalsecrets get externalsecrets -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.conditions[?(@.type=="Ready")].status}{" "}{.status.conditions[?(@.type=="Ready")].reason}{"\n"}{end}'
fetch events get events -A --field-selector type=Warning \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.reason}{" "}{.involvedObject.kind}{"/"}{.involvedObject.name}{" "}{.count}{"\n"}{end}'
fetch httproutes get httproutes -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.parents[0].conditions[?(@.type=="Accepted")].status}{" "}{.status.parents[0].conditions[?(@.type=="ResolvedRefs")].status}{"\n"}{end}'
# Config Connector (KCC) resources across the common Horizon CNRM kinds.
fetch kcc get pubsubtopics,storagebuckets,bigquerydatasets,iampolicymembers,iamserviceaccounts,iampartialpolicies -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.kind}{"/"}{.metadata.name}{" "}{.status.conditions[?(@.type=="Ready")].status}{" "}{.status.conditions[?(@.type=="Ready")].reason}{"\n"}{end}'
# Argo Workflows that did not complete successfully.
fetch workflows get workflows.argoproj.io -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.phase}{"\n"}{end}'
# Module Manager catalog: per-module enablement and reconcile state.
fetch modulecatalog get modulecatalog -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.status.conditions[?(@.type=="Ready")].status}{" "}{.status.conditions[?(@.type=="Ready")].reason}{"\n"}{end}'
wait_for_fetches() {
  local pids
  pids="$(jobs -p | grep -v "^$WATCHDOG\$")"
  for p in $pids; do wait "$p" 2>/dev/null; done
}
wait_for_fetches

# Prints one check: "[OK] title: ok_msg", or the issue count and up to
# MAX_ITEMS of the newline-separated items.
report() {
  local title="$1" ok_msg="$2" items="$3"
  CATEGORIES=$((CATEGORIES + 1))
  if [[ -z "$items" ]]; then
    echo "[OK]    $title: $ok_msg"
    return
  fi
  local n
  n="$(echo "$items" | wc -l | tr -d ' ')"
  ISSUES=$((ISSUES + n))
  echo "[ISSUE] $title: $n"
  echo "$items" | head -"$MAX_ITEMS" | sed 's/^/          /'
  [[ "$n" -gt "$MAX_ITEMS" ]] && echo "          ... and $((n - MAX_ITEMS)) more"
}

# Like report, but for optional resource types that may not exist on the cluster.
section() {
  local name="$1" title="$2" ok_msg="$3" items="$4"
  if [[ -f "$WORKDIR/$name.err" ]]; then
    echo "[SKIP]  $title: could not be read (CRD missing or no RBAC)"
    UNREADABLE=$((UNREADABLE + 1))
    return
  fi
  report "$title" "$ok_msg" "$items"
}

echo "=== Cluster health: $(kubectl config current-context 2>/dev/null) ==="
echo ""

if [[ -f "$WORKDIR/nodes.err" && -f "$WORKDIR/pods.err" ]]; then
  echo "Could not read nodes or pods; check access with 'kubectl get ns'." >&2
  exit 5
fi

section nodes "Nodes" "all Ready" \
  "$(awk '$2 != "Ready" {printf "%s (%s)\n", $1, $2}' "$WORKDIR/nodes" 2>/dev/null)"

section pods "Pods" "all Running and ready, none restarting heavily" \
  "$(awk -v t="$RESTART_THRESHOLD" '
    # Strips generated pod-name suffixes so replicas of one workload group together.
    function workload(n) {
      sub(/-[a-z0-9]{5}$/, "", n)
      sub(/-([0-9a-f]{8,10}|[0-9]+)$/, "", n)
      return n
    }
    {
      ns = $1; name = $2; ready = $3; status = $4; restarts = $5 + 0
      split(ready, r, "/")
      if (status == "Completed" || status == "Succeeded") next
      if (status == "Running" && r[1] != r[2]) status = "Running but not ready"
      if (status != "Running") { k = ns "/" workload(name) " " status; c[k]++; if (!(k in o)) o[k] = ++order; next }
      if (restarts >= t) { k = ns "/" name " " restarts " restarts"; c[k] = 1; o[k] = ++order }
    }
    END { for (k in c) printf "%d\t%s%s\n", o[k], k, (c[k] > 1 ? " x" c[k] : "") }
  ' "$WORKDIR/pods" 2>/dev/null | sort -n | cut -f2-)"

section workloads "Deployments/StatefulSets" "all at desired replicas" \
  "$(awk '{ want = $4 + 0; got = ($5 == "" ? 0 : $5 + 0); if (got < want) printf "%s/%s %s %d/%d ready\n", $1, $3, $2, got, want }' "$WORKDIR/workloads" 2>/dev/null)"

section pvcs "PersistentVolumeClaims" "all Bound" \
  "$(awk '$3 != "Bound" {printf "%s/%s %s\n", $1, $2, $3}' "$WORKDIR/pvcs" 2>/dev/null)"

section jobs "Jobs" "none failed" \
  "$(awk '$3 == "True" {printf "%s/%s %s\n", $1, $2, $4}' "$WORKDIR/jobs" 2>/dev/null)"

section apps "Argo CD applications" "all Synced and Healthy" \
  "$(awk '$3 != "Synced" || $4 != "Healthy" {printf "%s/%s %s / %s\n", $1, $2, ($3 == "" ? "Unknown" : $3), ($4 == "" ? "Unknown" : $4)}' "$WORKDIR/apps" 2>/dev/null)"

section gateways "Gateways" "all Programmed" \
  "$(awk '$3 != "True" {printf "%s/%s Programmed=%s\n", $1, $2, ($3 == "" ? "Unknown" : $3)}' "$WORKDIR/gateways" 2>/dev/null)"

section httproutes "HTTPRoutes" "all Accepted with resolved refs" \
  "$(awk '$3 != "True" || $4 != "True" {printf "%s/%s Accepted=%s ResolvedRefs=%s\n", $1, $2, ($3 == "" ? "Unknown" : $3), ($4 == "" ? "Unknown" : $4)}' "$WORKDIR/httproutes" 2>/dev/null)"

section kcc "Config Connector resources" "all Ready" \
  "$(awk '$3 != "True" {printf "%s/%s %s\n", $1, $2, ($4 == "" ? ($3 == "" ? "no status" : $3) : $4)}' "$WORKDIR/kcc" 2>/dev/null)"

section workflows "Argo Workflows" "none Failed or in Error" \
  "$(awk '$3 == "Failed" || $3 == "Error" {printf "%s/%s %s\n", $1, $2, $3}' "$WORKDIR/workflows" 2>/dev/null)"

section modulecatalog "Module Manager catalog" "Ready" \
  "$(awk '$3 != "True" {printf "%s/%s %s\n", $1, $2, ($4 == "" ? ($3 == "" ? "no status" : $3) : $4)}' "$WORKDIR/modulecatalog" 2>/dev/null)"

section externalsecrets "ExternalSecrets" "all Ready" \
  "$(awk '$3 != "True" {printf "%s/%s %s\n", $1, $2, ($4 == "" ? "Unknown" : $4)}' "$WORKDIR/externalsecrets" 2>/dev/null)"

if [[ -f "$WORKDIR/events.err" ]]; then
  echo "[SKIP]  Warning events: could not be read"
  UNREADABLE=$((UNREADABLE + 1))
else
  events="$(awk '{c[$1" "$2] += ($4 == "" ? 1 : $4)} END {for (k in c) printf "%6d  %s\n", c[k], k}' "$WORKDIR/events" | sort -rn)"
  if [[ -z "$events" ]]; then
    echo "[OK]    Warning events: none in the last ~1h"
  else
    echo "[INFO]  Warning events in the last ~1h, by namespace and reason:"
    echo "$events" | head -"$MAX_ITEMS" | sed 's/^/          /'
  fi
fi

echo ""
echo "--- Summary ---"
if [[ "$ISSUES" -gt 0 ]]; then
  echo "Verdict: ISSUES — $ISSUES problem(s) found across $CATEGORIES checks."
  [[ "$UNREADABLE" -gt 0 ]] && echo "Also $UNREADABLE check(s) skipped."
  exit 4
fi
if [[ "$UNREADABLE" -gt 0 ]]; then
  echo "Verdict: HEALTHY (partial) — no problems found; $UNREADABLE check(s) skipped."
  exit 0
fi
echo "Verdict: HEALTHY — no problems found across $CATEGORIES checks."
exit 0
