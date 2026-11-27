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

# Scenarios of the Storage Mirror kind harness; sourced by run.sh (needs lib.sh helpers, a port-forward to the
# controller API on 18443, ${WORK_DIR}/ca.crt and ${WORK_DIR}/token).

API_HOST="storage-mirror-module.horizon.svc.cluster.local"
API="https://${API_HOST}:18443/api/v1"
GITEA="https://gitea.e2e.svc.cluster.local:3000/e2e"
ROOT="/mnt/mirror-filestore-pvc/horizon-mirrors"

# api METHOD PATH [BODY] prints the body; HTTP status in ${WORK_DIR}/last-status.
api() {
  local method=$1 path=$2 body=${3:-}
  local args=(-sS --cacert "${WORK_DIR}/ca.crt" -H "Authorization: Bearer $(cat "${WORK_DIR}/token")"
    -H "X-Horizon-User: e2e-user" -H "Content-Type: application/json"
    --resolve "${API_HOST}:18443:127.0.0.1" -X "${method}" -o "${WORK_DIR}/last-body" -w '%{http_code}')
  [[ -n "${body}" ]] && args+=(-d "${body}")
  if ! curl "${args[@]}" "${API}${path}" >"${WORK_DIR}/last-status"; then
    echo '{}' >"${WORK_DIR}/last-body"
    echo 000 >"${WORK_DIR}/last-status"
  fi
  cat "${WORK_DIR}/last-body"
}
status_code() { cat "${WORK_DIR}/last-status"; }
api_up() {
  curl -fsS -o /dev/null --cacert "${WORK_DIR}/ca.crt" -H "Authorization: Bearer $(cat "${WORK_DIR}/token")" \
    --resolve "${API_HOST}:18443:127.0.0.1" "${API}/config"
}
phase() { api GET "/mirrors/$1/status" | jq -r '.phase // empty'; }
phase_is() { [[ "$(phase "$1")" == "$2" ]]; }

# wait_phase NAME PHASE [TIMEOUT]: polls the status API, logging progress lines.
wait_phase() {
  local name=$1 want=$2 timeout=${3:-900} start
  start=$(date +%s)
  while (( $(date +%s) - start < timeout )); do
    local st
    st="$(api GET "/mirrors/${name}/status")"
    echo "${st}" >>"${WORK_DIR}/status-${name}.jsonl"
    local p
    p="$(jq -r '.phase // empty' <<<"${st}")"
    printf '  %s %s %s\n' "${name}" "${p}" "$(jq -c '.currentRun.progress // {} | {phase,state,projectsDone,projectsTotal,objectsReceived,refsUpdated,overallPercent}' <<<"${st}")" >&2
    if [[ "${p}" == "${want}" && "$(jq -r '.currentRun // empty | .workflowName' <<<"${st}")" == "" && "$(jq -r '.pendingRequestId // empty' <<<"${st}")" == "" ]]; then
      return 0
    fi
    sleep "${STATUS_POLL:-2}"
  done
  return 1
}

# seen_progress NAME FILTER: true when any recorded status line matches the jq filter.
seen_progress() { [[ -f "${WORK_DIR}/status-$1.jsonl" ]] && jq -e -s "map(select($2)) | length > 0" "${WORK_DIR}/status-$1.jsonl" >/dev/null; }

# onvol CMD runs CMD in a throwaway pod with the read-only mirror claim (any service account may mount it).
onvol() {
  local name="onvol-${RANDOM}${RANDOM}"
  local overrides
  overrides="$(jq -n --arg n "${name}" --arg cmd "$1" '{spec: {
    volumes: [{name: "m", persistentVolumeClaim: {claimName: "storage-mirror-ro", readOnly: true}}],
    containers: [{name: $n, image: "storage-mirror-sync:e2e", imagePullPolicy: "IfNotPresent",
      command: ["bash", "-c", $cmd], volumeMounts: [{name: "m", mountPath: "/mnt/mirror-filestore-pvc", readOnly: true}]}]}}')"
  k -n workflows run "${name}" --rm -i --quiet --restart=Never --image=storage-mirror-sync:e2e --overrides="${overrides}" 2>/dev/null
}

catalog_status_is() { onvol "grep -A12 '$1:' ${ROOT}/metadata.yaml" | grep -q "status: $2"; }
workflows_json() { k -n workflows get workflows -l horizon-sdv.io/module=storage-mirror -o json; }
running_count() { workflows_json | jq '[.items[] | select(.status.phase == "Running")] | length'; }

# post_event BODY posts straight to the Argo Events webhook (bypasses the controller), from the horizon-api namespace,
# which the platform webhook ingress policy admits.
post_event() {
  k -n horizon-api run "post-${RANDOM}${RANDOM}" --rm -i --quiet --restart=Never --image=curlimages/curl:8.11.1 -- \
    curl -fsS --max-time 30 -X POST -H 'Content-Type: application/json' -d "$1" \
    http://webhook-eventsource-svc.argo-events.svc.cluster.local:12000/events/workflow >/dev/null 2>&1
}

want() { [[ -z "${ONLY_SCENARIOS}" || " ${ONLY_SCENARIOS} " == *" $1 "* ]]; }

sc_cel() {
  local out
  out="$(k apply --dry-run=server -f - 2>&1 <<'EOF' || true
apiVersion: storagemirror.horizon-sdv.io/v1alpha1
kind: StorageMirrorConfig
metadata: {name: other, namespace: horizon}
spec: {volume: {capacityGb: 1024, zone: "e2e-zone", network: "e2e-network"}}
EOF
)"
  check "CEL: StorageMirrorConfig must be named default" grep -q "must be named default" <<<"${out}"
  out="$(k patch storagemirrorconfig default -n horizon --type merge -p '{"spec":{"volume":{"capacityGb":768}}}' --dry-run=server 2>&1 || true)"
  check "CEL: capacity cannot shrink / below minimum" grep -qiE "decreased|should be greater than or equal to 1024" <<<"${out}"
  out="$(k apply --dry-run=server -f - 2>&1 <<'EOF' || true
apiVersion: storagemirror.horizon-sdv.io/v1alpha1
kind: StorageMirror
metadata: {name: bad, namespace: horizon}
spec: {kind: Repo, git: {url: "https://x/y.git"}}
EOF
)"
  check "CEL: spec.git on a Repo mirror is rejected" grep -q "must match spec.kind" <<<"${out}"
}

sc_volume() {
  TIMEOUT=600 POLL=5 wait_for bash -c "${KUBECTL} --kubeconfig ${KCFG} --context ${KCTX} -n horizon get storagemirrorconfig default -o json | jq -e '.status.conditions[] | select(.type==\"VolumeInitialized\" and .status==\"True\")'"
  check "volume-init reported the catalog (VolumeInitialized=True)" test $? -eq 0
  check "metadata.yaml created on the NFS volume" eval "onvol 'cat ${ROOT}/metadata.yaml' | grep -q horizon-mirrors"
  check "Jenkins claim sdv-mirror-filestore-pvc is Bound" bash -c "[ \"\$(${KUBECTL} --kubeconfig ${KCFG} --context ${KCTX} -n jenkins get pvc sdv-mirror-filestore-pvc -o jsonpath='{.status.phase}')\" = Bound ]"
  check "Jenkins PV is read-only" bash -c "[ \"\$(${KUBECTL} --kubeconfig ${KCFG} --context ${KCTX} get pv storage-mirror-jenkins -o jsonpath='{.spec.nfs.readOnly}')\" = true ]"
}

sc_vap() {
  local out
  out="$(k -n workflows apply --dry-run=server -f - 2>&1 <<'EOF' || true
apiVersion: v1
kind: Pod
metadata: {name: rw-intruder, namespace: workflows}
spec:
  containers: [{name: c, image: busybox:stable, volumeMounts: [{name: m, mountPath: /m}]}]
  volumes: [{name: m, persistentVolumeClaim: {claimName: storage-mirror-rw}}]
EOF
)"
  check "admission: only storage-mirror-sync may mount storage-mirror-rw" grep -q "only the storage-mirror-sync service account" <<<"${out}"
}

sc_flock() {
  local name="flock-${RANDOM}" out
  out="$(k -n workflows run "${name}" --rm -i --quiet --restart=Never --image=storage-mirror-sync:e2e --overrides="$(jq -n --arg n "${name}" '{spec: {
    serviceAccountName: "storage-mirror-sync", securityContext: {runAsUser: 1000, runAsGroup: 1000},
    volumes: [{name: "m", persistentVolumeClaim: {claimName: "storage-mirror-rw"}}],
    containers: [{name: $n, image: "storage-mirror-sync:e2e", imagePullPolicy: "IfNotPresent", command: ["bash", "-c",
      "L=/mnt/mirror-filestore-pvc/horizon-mirrors/.storage-mirror/catalog.lock; flock -x $L sleep 15 & sleep 3; if flock -n -x $L true; then echo NOT-LOCKED; else echo LOCKED; fi; wait"],
      volumeMounts: [{name: "m", mountPath: "/mnt/mirror-filestore-pvc"}]}]}}')" 2>&1 || true)"
  check "flock on the NFS share blocks a second locker" grep -q "^LOCKED" <<<"${out}"
}

sc_create_repo() {
  local res req
  res="$(api POST /mirrors '{"name":"aosp-e2e","kind":"repo","repo":{"syncJobs":2}}')"
  check "create repo mirror: HTTP 201" test "$(status_code)" = 201
  req="$(jq -r .requestId <<<"${res}")"
  wait_phase aosp-e2e Ready 1200
  check "repo mirror reaches Ready" test $? -eq 0
  check "fetch progress reported project counts" seen_progress aosp-e2e '.currentRun.progress.phase == "fetch" and .currentRun.progress.projectsTotal >= 3'
  check "progress state running seen with Initializing" seen_progress aosp-e2e '.phase == "Initializing" and .currentRun.progress.state == "running"'
  check "Workflow carries labelsFrom request-id and mirror labels" eval "workflows_json | jq -e --arg r '${req}' '[.items[] | select(.metadata.labels[\"storagemirror.horizon-sdv.io/request-id\"] == \$r and .metadata.labels[\"storagemirror.horizon-sdv.io/mirror\"] == \"aosp-e2e\")] | length == 1'"
  local cat
  cat="$(onvol "cat ${ROOT}/metadata.yaml")"
  check "catalog entry in Jenkins format (type repo, status ready, created_by)" bash -c "grep -q 'aosp-e2e:' <<<'${cat}' && grep -q 'type: repo' <<<'${cat}' && grep -q 'status: ready' <<<'${cat}' && grep -q 'created_by: e2e-user' <<<'${cat}'"
  check "repo mirror tree (.repo and project repositories)" eval "onvol 'test -d ${ROOT}/aosp-e2e/.repo && ls -d ${ROOT}/aosp-e2e/p1.git ${ROOT}/aosp-e2e/p2.git'"
  check "prune-refs removed refs/changes/*" eval "[ -z \"\$(onvol 'git -C ${ROOT}/aosp-e2e/p1.git for-each-ref refs/changes')\" ]"
}

sc_create_git_lfs() {
  api POST /mirrors "{\"name\":\"lfs-e2e\",\"kind\":\"git\",\"git\":{\"url\":\"${GITEA}/lfsrepo.git\"},\"lfs\":\"auto\"}" >/dev/null
  check "create git mirror: HTTP 201" test "$(status_code)" = 201
  wait_phase lfs-e2e Ready 900
  check "git mirror reaches Ready" test $? -eq 0
  check "git-lfs auto detected" bash -c "[ \"\$(curl -sS --cacert ${WORK_DIR}/ca.crt -H 'Authorization: Bearer $(cat "${WORK_DIR}/token")' --resolve ${API_HOST}:18443:127.0.0.1 ${API}/mirrors/lfs-e2e | jq -r .status.lfsDetected)\" = true ]"
  check "LFS objects fetched into the mirror" eval "[ \"\$(onvol 'find ${ROOT}/lfs-e2e/lfs/objects -type f | wc -l')\" -ge 1 ]"
  check "git mirror is a bare repository with refs" eval "onvol 'git -C ${ROOT}/lfs-e2e rev-parse refs/heads/main'"
}

sc_create_git_credentials() {
  # Private upstream: the credentials Secret is mounted next to the API CA (both under /etc/storage-mirror).
  k -n workflows create secret generic e2e-git-credentials --from-literal=username=e2e \
    --from-literal=password="${GITEA_PASSWORD}" --dry-run=client -o yaml | k apply -f - >/dev/null
  api POST /mirrors "{\"name\":\"private-e2e\",\"kind\":\"git\",\"git\":{\"url\":\"${GITEA}/private-repo.git\",\"credentialsSecretName\":\"e2e-git-credentials\"}}" >/dev/null
  check "create git mirror with credentials: HTTP 201" test "$(status_code)" = 201
  wait_phase private-e2e Ready 600
  check "private git mirror reaches Ready (credentials Secret mounted)" test $? -eq 0
  check "private mirror has the upstream ref" eval "onvol 'git -C ${ROOT}/private-e2e rev-parse refs/heads/main'"
  check "credentials do not appear in metadata.yaml" eval "! onvol 'grep -q \"${GITEA_PASSWORD}\" ${ROOT}/metadata.yaml'"
}

sc_sync_progress() {
  seed update
  rm -f "${WORK_DIR}/status-aosp-e2e.jsonl"
  api POST /mirrors/aosp-e2e/sync >/dev/null
  check "sync request: HTTP 202" test "$(status_code)" = 202
  wait_phase aosp-e2e Ready 900
  check "sync finishes Ready" test $? -eq 0
  check "Syncing phase with fetch progress observed" seen_progress aosp-e2e '.phase == "Syncing" and .currentRun.progress.phase != null'
  check "lastRun is a successful storage-mirror-sync" bash -c "[ \"\$(curl -sS --cacert ${WORK_DIR}/ca.crt -H 'Authorization: Bearer $(cat "${WORK_DIR}/token")' --resolve ${API_HOST}:18443:127.0.0.1 ${API}/mirrors/aosp-e2e/status | jq -r '.lastRun.template + \"/\" + .lastRun.phase')\" = storage-mirror-sync/Succeeded ]"
}

sc_batch_semaphore() {
  api POST /syncs '{"all":true}' >/dev/null
  check "batch sync: HTTP 202" test "$(status_code)" = 202
  local max=0 n start
  start=$(date +%s)
  while (( $(date +%s) - start < 900 )); do
    n="$(running_count)"
    (( n > max )) && max=$n
    if [[ "$(phase aosp-e2e)" == Ready && "$(phase lfs-e2e)" == Ready ]] && \
       [[ "$(workflows_json | jq '[.items[] | select(.status.phase == "Running" or .status.phase == "Pending" or .status.phase == null)] | length')" == 0 ]]; then
      break
    fi
    sleep 2
  done
  check "semaphore: at most 1 mirror workflow Running at a time (max seen ${max})" test "${max}" -le 1
  check "batch: both mirrors Ready afterwards" bash -c "[ \"\$(curl -sS --cacert ${WORK_DIR}/ca.crt -H 'Authorization: Bearer $(cat "${WORK_DIR}/token")' --resolve ${API_HOST}:18443:127.0.0.1 ${API}/mirrors | jq '[.items[] | select(.managed and .status.phase == \"Ready\")] | length')\" -ge 2 ]"
}

sc_mutex() {
  # Two sync events for the same mirror straight to Argo Events: the per-directory mutex must serialise them.
  local body
  body="$(jq -nc '{workflowTemplateName: "storage-mirror-sync", workflowTemplateNamespace: "workflows", horizonModule: "storage-mirror",
    horizonSubmittedFrom: "api", horizonSubmittedBy: "e2e-mutex", mirror: "lfs-e2e", directory: "lfs-e2e", requestId: ("mutex" + (now|tostring|gsub("\\.";"")))}')"
  post_event "${body}"
  post_event "$(jq -c '.requestId += "b"' <<<"${body}")"
  local both=0 start
  start=$(date +%s)
  while (( $(date +%s) - start < 600 )); do
    n="$(workflows_json | jq '[.items[] | select(.metadata.labels["storagemirror.horizon-sdv.io/mirror"] == "lfs-e2e" and .status.phase == "Running")] | length')"
    (( n > 1 )) && both=1
    [[ "$(workflows_json | jq '[.items[] | select((.metadata.labels["storagemirror.horizon-sdv.io/request-id"] // "") | startswith("mutex")) | select(.status.phase == "Succeeded")] | length')" -ge 2 ]] && break
    sleep 2
  done
  check "mutex: two runs of one mirror never Running together" test "${both}" = 0
  wait_phase lfs-e2e Ready 300
  check "webhook-posted runs keep the mirror Ready" test $? -eq 0
}

# hold_catalog_lock NAME holds the metadata.yaml flock from pod NAME (writer identity) until the pod is deleted, so a
# run started meanwhile blocks at its first catalog update; this keeps the cancel scenario independent of how fast the
# seed syncs.
hold_catalog_lock() {
  k -n workflows run "$1" --rm -i --quiet --restart=Never --image=storage-mirror-sync:e2e --overrides="$(jq -n --arg n "$1" '{spec: {
    serviceAccountName: "storage-mirror-sync", securityContext: {runAsUser: 1000, runAsGroup: 1000},
    volumes: [{name: "m", persistentVolumeClaim: {claimName: "storage-mirror-rw"}}],
    containers: [{name: $n, image: "storage-mirror-sync:e2e", imagePullPolicy: "IfNotPresent", command: ["bash", "-c",
      "flock -x /mnt/mirror-filestore-pvc/horizon-mirrors/.storage-mirror/catalog.lock sleep 600"],
      volumeMounts: [{name: "m", mountPath: "/mnt/mirror-filestore-pvc"}]}]}}')" </dev/null >/dev/null 2>&1
}

sc_cancel() {
  local holder="catalog-lock-${RANDOM}"
  hold_catalog_lock "${holder}" &
  local holder_pid=$!
  TIMEOUT=60 POLL=1 wait_for k -n workflows wait --for=condition=Ready "pod/${holder}" --timeout=5s
  api POST /mirrors/aosp-e2e/sync >/dev/null
  TIMEOUT=120 POLL=1 wait_for phase_is aosp-e2e Syncing
  api POST /mirrors/aosp-e2e/sync/cancel >/dev/null
  check "cancel: HTTP 202" test "$(status_code)" = 202
  # Release the lock: the stopped agent then gets it and records the error status.
  k -n workflows delete pod "${holder}" --grace-period=0 --force --wait=false >/dev/null 2>&1
  wait_phase aosp-e2e Error 600
  check "cancelled run leaves phase Error" test $? -eq 0
  TIMEOUT=90 POLL=3 wait_for catalog_status_is aosp-e2e error
  check "catalog status error after cancel" test $? -eq 0
  wait "${holder_pid}" 2>/dev/null || true
  api POST /mirrors/aosp-e2e/sync >/dev/null
  wait_phase aosp-e2e Ready 900
  check "next sync after cancel recovers to Ready" test $? -eq 0
}

sc_restart() {
  api POST /mirrors/aosp-e2e/sync >/dev/null
  TIMEOUT=300 POLL=2 wait_for phase_is aosp-e2e Syncing
  local old secrets_before
  secrets_before="$(k -n horizon get secret storage-mirror-module-api-token storage-mirror-module-api-tls -o json | jq -c '[.items[].data]')"
  old="$(k -n horizon get pods -l app.kubernetes.io/name=storage-mirror-module -o jsonpath='{.items[0].metadata.name}')"
  k -n horizon delete pod "${old}" --wait=true --timeout=120s
  k -n horizon wait --for=condition=Ready pod -l app.kubernetes.io/name=storage-mirror-module --timeout=300s
  # run.sh's port-forward loop reconnects to the new pod.
  TIMEOUT=60 POLL=2 wait_for api_up
  wait_phase aosp-e2e Ready 900
  check "controller restart mid-sync: mirror still reaches Ready" test $? -eq 0
  check "restart reuses the API token and TLS Secrets" test "$(k -n horizon get secret storage-mirror-module-api-token storage-mirror-module-api-tls -o json | jq -c '[.items[].data]')" = "${secrets_before}"
  check "history keeps the run across the restart" bash -c "[ \"\$(curl -sS --cacert ${WORK_DIR}/ca.crt -H 'Authorization: Bearer $(cat "${WORK_DIR}/token")' --resolve ${API_HOST}:18443:127.0.0.1 ${API}/mirrors/aosp-e2e/runs | jq '[.items[] | select(.phase == \"Succeeded\")] | length')\" -ge 2 ]"
}

sc_delete_retain_import() {
  api DELETE /mirrors/lfs-e2e >/dev/null
  check "delete (retain): HTTP 202" test "$(status_code)" = 202
  TIMEOUT=120 wait_for bash -c "! ${KUBECTL} --kubeconfig ${KCFG} --context ${KCTX} -n horizon get storagemirror lfs-e2e"
  check "retain delete removes the CR" test $? -eq 0
  check "retain delete keeps the data" eval "onvol 'test -d ${ROOT}/lfs-e2e/objects'"
  check "retained mirror listed as unmanaged" bash -c "[ \"\$(curl -sS --cacert ${WORK_DIR}/ca.crt -H 'Authorization: Bearer $(cat "${WORK_DIR}/token")' --resolve ${API_HOST}:18443:127.0.0.1 ${API}/mirrors | jq -r '.items[] | select(.name==\"lfs-e2e\") | .managed')\" = false ]"
  api POST /import '{"mirrors":["lfs-e2e"]}' >/dev/null
  check "import brings it back under management" bash -c "${KUBECTL} --kubeconfig ${KCFG} --context ${KCTX} -n horizon get storagemirror lfs-e2e"
}

sc_delete_purge() {
  api DELETE "/mirrors/aosp-e2e?purge=true" >/dev/null
  check "purge without confirm is refused (HTTP 400)" test "$(status_code)" = 400
  api DELETE "/mirrors/aosp-e2e?purge=true&confirm=aosp-e2e" >/dev/null
  check "purge: HTTP 202" test "$(status_code)" = 202
  TIMEOUT=600 POLL=5 wait_for bash -c "! ${KUBECTL} --kubeconfig ${KCFG} --context ${KCTX} -n horizon get storagemirror aosp-e2e"
  check "purge: CR released after the delete run" test $? -eq 0
  check "purge: directory removed" eval "onvol 'test ! -e ${ROOT}/aosp-e2e'"
  check "purge: catalog entry removed" eval "! onvol 'cat ${ROOT}/metadata.yaml' | grep -q 'aosp-e2e:'"
}

run_scenarios() {
  # Scenarios record failures and carry on.
  set +e +o pipefail
  want cel && sc_cel
  want volume && sc_volume
  want vap && sc_vap
  want flock && sc_flock
  want create-repo && sc_create_repo
  want create-git-lfs && sc_create_git_lfs
  want create-git-credentials && sc_create_git_credentials
  want sync && sc_sync_progress
  want batch && sc_batch_semaphore
  want mutex && sc_mutex
  want cancel && sc_cancel
  want restart && sc_restart
  want retain && sc_delete_retain_import
  want purge && sc_delete_purge
  skip "Argo CD Delete=false on CRDs (no Argo CD in the harness; remains open)"
  skip "Horizon CLI end to end (needs Horizon API + Keycloak; CLI rendering is unit-tested)"
}

# --- --apiserver-only: real kube-apiserver, controller running locally, Argo replaced by simulated Workflows ---

EVENTS="${WORK_DIR}/events.jsonl"

# sim_workflow NAME TEMPLATE MIRROR REQUEST_ID PHASE: create or update a Workflow as Argo would.
sim_workflow() {
  local name=$1 tmpl=$2 mirror=$3 rid=$4 ph=$5 now patch
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if ! k -n workflows get workflow "${name}" >/dev/null 2>&1; then
    k create -f - >/dev/null <<WF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: ${name}
  namespace: workflows
  labels:
    horizon-sdv.io/module: storage-mirror
    storagemirror.horizon-sdv.io/mirror: "${mirror}"
    storagemirror.horizon-sdv.io/request-id: "${rid}"
    workflows.argoproj.io/workflow-template: ${tmpl}
spec:
  workflowTemplateRef: {name: ${tmpl}}
WF
  fi
  patch="$(jq -nc --arg p "${ph}" --arg t "${now}" 'if ($p == "Succeeded" or $p == "Failed") then {status: {phase: $p, finishedAt: $t}} else {status: {phase: $p, startedAt: $t}} end')"
  k -n workflows patch workflow "${name}" --type merge -p "${patch}" >/dev/null
}

agent_progress() {
  local run=$1 mirror=$2 json=$3 tok
  tok="$(k -n workflows get secret storage-mirror-module-api-token -o jsonpath='{.data.storage-mirror-agent}' | base64 -d)"
  curl -sS -o /dev/null -w '%{http_code}' --cacert "${WORK_DIR}/ca.crt" -H "Authorization: Bearer ${tok}" \
    --resolve "${API_HOST}:18443:127.0.0.1" -X PUT -d "$(jq -nc --arg m "${mirror}" --argjson p "${json}" '{mirror: $m, progress: $p}')" \
    "https://${API_HOST}:18443/internal/v1/runs/${run}/progress"
}

events_with() { [[ -f "${EVENTS}" ]] && grep -c -- "$1" "${EVENTS}" || echo 0; }
event_for() { grep -- "\"requestId\":\"$1\"" "${EVENTS}" | tail -1; }
has_event_for() { [[ -n "$(event_for "$1")" ]]; }
cr_json() { k -n horizon get storagemirror "$1" -o json; }
cr_absent() { ! k -n horizon get storagemirror "$1" >/dev/null 2>&1; }
volume_ready() { [[ "$(k -n horizon get smc default -o jsonpath='{.status.volume.phase}')" == Ready ]]; }

sc_sim_volume() {
  TIMEOUT=60 wait_for volume_ready
  check "sim: test volume Ready" test $? -eq 0
  check "sim: three static PVs on the test NFS server" test "$(k get pv -l horizon-sdv.io/module=storage-mirror -o json | jq '[.items[] | select(.spec.nfs.server != null)] | length')" = 3
  check "sim: RO and Jenkins PVs are read-only" test "$(k get pv storage-mirror-workflows-ro storage-mirror-jenkins -o json | jq '[.items[] | select(.spec.nfs.readOnly)] | length')" = 2
  check "sim: claims created in workflows and jenkins" k -n workflows get pvc storage-mirror-rw storage-mirror-ro
  check "sim: Jenkins claim created" k -n jenkins get pvc sdv-mirror-filestore-pvc
  check "sim: controller created the API Secrets; copies carry one caller token each" test \
    "$(k -n workflows get secret storage-mirror-module-api-token -o json | jq -c '.data | keys')/$(k -n horizon-api get secret storage-mirror-module-api-token -o json | jq -c '.data | keys')" = '["storage-mirror-agent"]/["horizon-api"]'
  TIMEOUT=60 POLL=1 wait_for test "$(events_with storage-mirror-volume-init)" -ge 1
  sleep 5
  check "sim: volume-init dispatched once" test "$(events_with storage-mirror-volume-init)" = 1
}

sc_sim_lifecycle() {
  local res rid ev st
  res="$(api POST /mirrors '{"name":"aosp","kind":"repo"}')"
  check "sim: create HTTP 201" test "$(status_code)" = 201
  rid="$(jq -r .requestId <<<"${res}")"
  TIMEOUT=30 POLL=1 wait_for has_event_for "${rid}"
  ev="$(event_for "${rid}")"
  check "sim: create event carries template, mirror, defaults and submitter" jq -e '.workflowTemplateName == "storage-mirror-create" and .mirror == "aosp" and .directory == "aosp" and .kind == "repo" and (.manifestUrl | length > 0) and .horizonSubmittedBy == "e2e-user" and .submittedBy == "e2e-user"' <<<"${ev}"
  check "sim: data finalizer added" jq -e '.metadata.finalizers | index("storagemirror.horizon-sdv.io/data") != null' <<<"$(cr_json aosp)"
  api POST /mirrors/aosp/sync >/dev/null
  check "sim: sync while a request is pending is folded into it" test "$(status_code)/$(events_with '"mirror":"aosp"')" = "202/1"

  sim_workflow storage-mirror-create-sim1 storage-mirror-create aosp "${rid}" Running
  check "sim: agent progress accepted (204)" test "$(agent_progress storage-mirror-create-sim1 aosp '{"phase":"fetch","state":"running","projectsDone":2,"projectsTotal":3,"overallPercent":54}')" = 204
  check "sim: progress for another mirror rejected (403)" test "$(agent_progress storage-mirror-create-sim1 other '{}')" = 403
  sleep 3
  st="$(api GET /mirrors/aosp/status)"
  check "sim: status Initializing with live progress 2/3 54%" jq -e '.phase == "Initializing" and .catalogStatus == "syncing" and .currentRun.progress.projectsDone == 2 and .currentRun.progress.projectsTotal == 3 and .currentRun.progress.overallPercent == 54 and (.pendingRequestId // "") == ""' <<<"${st}"
  sim_workflow storage-mirror-create-sim1 storage-mirror-create aosp "${rid}" Succeeded
  TIMEOUT=30 POLL=1 wait_for phase_is aosp Ready
  check "sim: Succeeded run makes the mirror Ready with history" jq -e '.status.lastSuccessfulSyncTime != null and (.status.history | length) == 1 and .status.catalogStatus == "ready"' <<<"$(cr_json aosp)"

  res="$(api POST /mirrors/aosp/sync)"
  rid="$(jq -r .requestId <<<"${res}")"
  TIMEOUT=30 POLL=1 wait_for has_event_for "${rid}"
  check "sim: sync event dispatched" jq -e '.workflowTemplateName == "storage-mirror-sync"' <<<"$(event_for "${rid}")"
  sim_workflow storage-mirror-sync-sim2 storage-mirror-sync aosp "${rid}" Running
  TIMEOUT=30 POLL=1 wait_for phase_is aosp Syncing
  check "sim: phase Syncing while the sync runs" test $? -eq 0
  sim_workflow storage-mirror-sync-sim2 storage-mirror-sync aosp "${rid}" Failed
  TIMEOUT=30 POLL=1 wait_for phase_is aosp Error
  check "sim: failed sync gives phase Error" test $? -eq 0

  # A create Workflow for a mirror without a CR (posted straight to the webhook) is not adopted.
  k create -f - >/dev/null <<'WF'
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: storage-mirror-create-direct
  namespace: workflows
  labels: {horizon-sdv.io/module: storage-mirror, storagemirror.horizon-sdv.io/mirror: direct, workflows.argoproj.io/workflow-template: storage-mirror-create}
spec:
  workflowTemplateRef: {name: storage-mirror-create}
  arguments:
    parameters:
      - {name: mirror, value: direct}
      - {name: kind, value: git}
      - {name: gitUrl, value: "https://example.com/direct.git"}
WF
  sleep 5
  check "sim: a create Workflow without a StorageMirror is not adopted" cr_absent direct
  api POST /mirrors '{"name":"keep","kind":"git","git":{"url":"https://example.com/keep.git"}}' >/dev/null
  check "sim: second mirror created (HTTP 201)" test "$(status_code)" = 201

  api DELETE "/mirrors/aosp?purge=true&confirm=aosp" >/dev/null
  check "sim: purge HTTP 202" test "$(status_code)" = 202
  TIMEOUT=30 POLL=1 wait_for test "$(events_with storage-mirror-delete)" -ge 1
  ev="$(grep storage-mirror-delete "${EVENTS}" | tail -1)"
  rid="$(jq -r .requestId <<<"${ev}")"
  check "sim: delete event has confirm = directory" jq -e '.confirm == "aosp" and .directory == "aosp"' <<<"${ev}"
  check "sim: CR kept until the delete run succeeds" k -n horizon get storagemirror aosp
  sim_workflow storage-mirror-delete-sim3 storage-mirror-delete aosp "${rid}" Succeeded
  TIMEOUT=30 POLL=1 wait_for cr_absent aosp
  check "sim: CR released after the delete run" test $? -eq 0
  api DELETE /mirrors/keep >/dev/null
  TIMEOUT=30 POLL=1 wait_for cr_absent keep
  check "sim: Retain delete releases the CR without a delete run" test "$?/$(events_with storage-mirror-delete)" = "0/1"
}

run_apiserver_scenarios() {
  set +e +o pipefail
  sc_cel
  sc_vap
  sc_sim_volume
  sc_sim_lifecycle
  skip "NFS, git, repo, git-lfs, Argo runtime (labelsFrom, semaphores, mutexes) and flock need the kind run"
}
