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

# Storage Mirror end-to-end harness on a dedicated kind cluster. See README.md.
#
#   tools/storage-mirror-e2e/run.sh [--keep] [--reuse] [--skip-build] [--render-only] [--apiserver-only]
#                                   [--only SCENARIO...]

set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${HERE}/../.." && pwd)"
# shellcheck source=lib.sh
source "${HERE}/lib.sh"

KEEP=0
REUSE=0
SKIP_BUILD=0
RENDER_ONLY=0
APISERVER_ONLY=0
ONLY=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --keep) KEEP=1 ;;
    --reuse) REUSE=1; KEEP=1 ;;
    --skip-build) SKIP_BUILD=1 ;;
    --render-only) RENDER_ONLY=1 ;;
    --apiserver-only) APISERVER_ONLY=1 ;;
    --only) shift; ONLY+=("$1") ;;
    -h|--help) sed -n '17,23p' "$0"; exit 0 ;;
    *) die "unknown flag $1" ;;
  esac
  shift
done
export ONLY_SCENARIOS="${ONLY[*]:-}"

ARGO_WORKFLOWS_CHART_VERSION="0.47.4" # gitops/values.yaml chartVersions.argo_workflows (Argo Workflows v3.7.10)
ARGO_EVENTS_CHART_VERSION="2.3.2"     # gitops/values.yaml chartVersions.argoEvents (Argo Events v1.7.6)
ARGO_HELM_REPO="https://argoproj.github.io/argo-helm"
MODULE_CHART="${REPO_ROOT}/gitops/modules/storage-mirror-module"
IMAGES="${REPO_ROOT}/terraform/modules/sdv-container-images/images/storage-mirror-module"
RENDER="${WORK_DIR}/render"
mkdir -p "${WORK_DIR}" "${RENDER}"

preflight() {
  for t in python3 jq curl "${KUBECTL}" "${HELM}"; do
    command -v "${t}" >/dev/null || die "${t} not found"
  done
  python3 -c 'import yaml' 2>/dev/null || die "python3 PyYAML not installed"
  if [[ "${RENDER_ONLY}" == 1 ]]; then
    return
  fi
  if [[ "${APISERVER_ONLY}" == 1 ]]; then
    command -v go >/dev/null || die "go not found"
    return
  fi
  command -v "${KIND}" >/dev/null || die "kind not found at ${KIND} (GOBIN=${TOOLS_BIN} go install sigs.k8s.io/kind@v0.27.0)"
  command -v docker >/dev/null || die "docker not found"
  docker info >/dev/null 2>&1 || die "docker is not usable by $(id -un): $(docker info 2>&1 | tail -1)"
  if ! grep -qw nfsd /proc/filesystems && ! modinfo nfsd >/dev/null 2>&1; then
    die "the host kernel has no nfsd module (the in-cluster NFS server needs it; try: sudo modprobe nfsd)"
  fi
}

# render NFS_IP: helm template of the module chart and its child chart, then the test-only patches.
render() {
  local nfs_ip=$1
  log "rendering charts (NFS server ${nfs_ip})"
  "${HELM}" template storage-mirror "${MODULE_CHART}" -f "${HERE}/values/storage-mirror.yaml" >"${RENDER}/module-raw.yaml"
  python3 "${HERE}/render.py" child-values "${RENDER}/module-raw.yaml" "${RENDER}/child-values.yaml"
  "${HELM}" template storage-mirror-argo-workflows "${MODULE_CHART}/argo-workflows" -f "${RENDER}/child-values.yaml" >"${RENDER}/workflows-raw.yaml"
  python3 "${HERE}/render.py" module "${RENDER}/module-raw.yaml" "${RENDER}/module.yaml" --nfs-server "${nfs_ip}"
  python3 "${HERE}/render.py" workflows "${RENDER}/workflows-raw.yaml" "${RENDER}/workflows.yaml"
}

cleanup() {
  local rc=$?
  if [[ -n "${PF_PID:-}" ]]; then
    kill "${PF_PID}" 2>/dev/null || true
    pkill -P "${PF_PID}" 2>/dev/null || true
  fi
  if [[ "${KEEP}" == 1 ]]; then
    log "keeping cluster ${CLUSTER_NAME} (kubeconfig ${KCFG}); delete with: ${KIND} delete cluster --name ${CLUSTER_NAME}"
  elif [[ "${CREATED:-0}" == 1 ]]; then
    log "deleting kind cluster ${CLUSTER_NAME}"
    "${KIND}" delete cluster --name "${CLUSTER_NAME}" --kubeconfig "${KCFG}" || true
  fi
  exit "${rc}"
}

create_cluster() {
  if [[ "${REUSE}" == 1 ]] && "${KIND}" get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
    log "reusing kind cluster ${CLUSTER_NAME}"
    "${KIND}" export kubeconfig --name "${CLUSTER_NAME}" --kubeconfig "${KCFG}"
  else
    "${KIND}" get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}" && die "cluster ${CLUSTER_NAME} exists; delete it or pass --reuse"
    log "creating kind cluster ${CLUSTER_NAME}"
    CREATED=1
    "${KIND}" create cluster --name "${CLUSTER_NAME}" --kubeconfig "${KCFG}" --config "${HERE}/kind-config.yaml" --wait 120s
  fi
  assert_local_cluster
  # kubelet mounts the NFS PVs; kind node images do not ship mount.nfs.
  for node in $("${KIND}" get nodes --name "${CLUSTER_NAME}"); do
    docker exec "${node}" sh -c 'command -v mount.nfs >/dev/null || (apt-get update -qq && apt-get install -y -qq nfs-common >/dev/null)'
  done
}

build_images() {
  if [[ "${SKIP_BUILD}" == 0 ]]; then
    local go_version
    go_version="$(python3 -c 'import sys, yaml; print(yaml.safe_load(open(sys.argv[1]))["tools"]["go"]["version"])' "${REPO_ROOT}/versions.yaml")"
    log "building images (GO_VERSION=${go_version})"
    docker build --build-arg GO_VERSION="${go_version}" -t storage-mirror-module-app:e2e "${IMAGES}/storage-mirror-module-app"
    docker build --build-arg GO_VERSION="${go_version}" -t storage-mirror-sync:e2e "${IMAGES}/storage-mirror-sync"
  fi
  "${KIND}" load docker-image storage-mirror-module-app:e2e storage-mirror-sync:e2e --name "${CLUSTER_NAME}"
}

install_infra() {
  log "installing NFS server and Gitea"
  k apply -f "${HERE}/manifests/infra.yaml"
  k -n e2e rollout status deploy/nfs-server --timeout=300s
  k -n e2e rollout status deploy/gitea --timeout=300s
  TIMEOUT=120 wait_for k -n e2e exec deploy/gitea -- gitea admin user list
  k -n e2e exec deploy/gitea -- gitea admin user create --admin --username e2e --password "${GITEA_PASSWORD}" \
    --email e2e@example.com --must-change-password=false >/dev/null 2>&1 || true
}

seed() {
  local mode=$1 job="seed-${1}"
  k -n e2e create configmap seed-script --from-file=seed.sh="${HERE}/manifests/seed.sh" --dry-run=client -o yaml | k apply -f -
  k -n e2e delete job "${job}" --ignore-not-found
  k -n e2e apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata: {name: ${job}, namespace: e2e}
spec:
  backoffLimit: 2
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: seed
          image: storage-mirror-sync:e2e
          imagePullPolicy: IfNotPresent
          command: [bash, /seed/seed.sh]
          env:
            - {name: MODE, value: ${mode}}
            - {name: GITEA_USER, value: e2e}
            - {name: GITEA_PASSWORD, value: "${GITEA_PASSWORD}"}
          volumeMounts: [{name: seed, mountPath: /seed}]
      volumes: [{name: seed, configMap: {name: seed-script}}]
EOF
  k -n e2e wait --for=condition=complete "job/${job}" --timeout=300s || { k -n e2e logs "job/${job}" || true; die "seed ${mode} failed"; }
}

install_argo() {
  log "installing Argo Workflows ${ARGO_WORKFLOWS_CHART_VERSION} and Argo Events ${ARGO_EVENTS_CHART_VERSION}"
  # The Argo Workflows chart binds roles in the managed namespace; create the platform namespaces first.
  for ns in horizon horizon-api workflows jenkins argo-events; do
    k create namespace "${ns}" --dry-run=client -o yaml | k apply -f - >/dev/null
  done
  h upgrade --install argo-workflows argo-workflows --repo "${ARGO_HELM_REPO}" --version "${ARGO_WORKFLOWS_CHART_VERSION}" \
    -n argo --create-namespace -f "${HERE}/values/argo-workflows.yaml" --wait --timeout 10m
  h upgrade --install argo-events argo-events --repo "${ARGO_HELM_REPO}" --version "${ARGO_EVENTS_CHART_VERSION}" \
    -n argo-events --create-namespace --wait --timeout 10m
  k apply -f "${HERE}/manifests/platform.yaml"
  TIMEOUT=300 wait_for sh -c "[ \"\$(${KUBECTL} --kubeconfig ${KCFG} --context ${KCTX} -n argo-events get pods -l eventsource-name=webhook -o jsonpath='{.items[0].status.containerStatuses[0].ready}')\" = true ]" \
    || die "webhook EventSource not ready"
}

# platform_policies applies the platform's own NetworkPolicies for the namespaces the module uses, rendered from the
# root chart, plus manifests/harness-policies.yaml for the kind differences.
PLATFORM_POLICY_NAMESPACES="horizon workflows argo-events jenkins"
platform_policies() {
  log "applying platform NetworkPolicies (${PLATFORM_POLICY_NAMESPACES})"
  "${HELM}" template platform "${REPO_ROOT}/gitops" -f "${HERE}/values/platform-root.yaml" \
    | python3 -c "import sys,yaml
keep = set('${PLATFORM_POLICY_NAMESPACES}'.split())
docs = [d for d in yaml.safe_load_all(sys.stdin) if d and d.get('kind') == 'NetworkPolicy' and d['metadata'].get('namespace') in keep]
for d in docs:
    d['metadata'].pop('annotations', None)
yaml.safe_dump_all(docs, sys.stdout, sort_keys=False)" >"${RENDER}/platform-policies.yaml"
  k apply -f "${RENDER}/platform-policies.yaml"
  k apply -f "${HERE}/manifests/harness-policies.yaml"
}

install_module() {
  log "installing the storage-mirror module"
  k apply --server-side -f "${MODULE_CHART}/crds/"
  k wait --for=condition=Established crd/storagemirrors.storagemirror.horizon-sdv.io crd/storagemirrorconfigs.storagemirror.horizon-sdv.io --timeout=60s
  k apply -f "${RENDER}/module.yaml"
  k apply -f "${RENDER}/workflows.yaml"
  k -n horizon rollout status deploy/storage-mirror-module --timeout=300s
}

# port_forward keeps a port-forward to the controller Service, restarting it when the pod changes (restart scenario).
port_forward() {
  (
    while true; do
      k -n horizon port-forward svc/storage-mirror-module 18443:8443 >>"${WORK_DIR}/port-forward.log" 2>&1 || true
      sleep 1
    done
  ) &
  PF_PID=$!
  export PF_PID
  k -n horizon get secret storage-mirror-module-api-tls -o jsonpath='{.data.ca\.crt}' | base64 -d >"${WORK_DIR}/ca.crt"
  k -n horizon get secret storage-mirror-module-api-token -o jsonpath='{.data.default}' | base64 -d >"${WORK_DIR}/token"
  TIMEOUT=60 wait_for curl -fsS --cacert "${WORK_DIR}/ca.crt" -H "Authorization: Bearer $(cat "${WORK_DIR}/token")" \
    --resolve storage-mirror-module.horizon.svc.cluster.local:18443:127.0.0.1 \
    https://storage-mirror-module.horizon.svc.cluster.local:18443/api/v1/config || die "controller API not reachable through port-forward"
}

ARGO_WORKFLOWS_VERSION="v3.7.10"

# apiserver_cleanup stops the local controller, the fake webhook and the envtest control plane (and its children).
apiserver_cleanup() {
  local rc=$? p
  for p in "${APISERVER_PIDS[@]}"; do
    pkill -TERM -P "${p}" 2>/dev/null || true
    kill -TERM "${p}" 2>/dev/null || true
  done
  for _ in $(seq 1 20); do
    local alive=0
    for p in "${APISERVER_PIDS[@]}"; do kill -0 "${p}" 2>/dev/null && alive=1; done
    [[ "${alive}" == 0 ]] && break
    sleep 0.5
  done
  for p in "${APISERVER_PIDS[@]}"; do
    pkill -KILL -P "${p}" 2>/dev/null || true
    kill -KILL "${p}" 2>/dev/null || true
  done
  exit "${rc}"
}

# apiserver_main: no container runtime. A throwaway kube-apiserver + etcd (envtest) with the module, Argo Workflows
# (full schema) and Argo Events CRDs; the controller runs locally against it; Argo is simulated by the scenarios.
apiserver_main() {
  local assets="${KUBEBUILDER_ASSETS:-}" argo_dir pids=()
  if [[ -z "${assets}" ]]; then
    [[ -x "${TOOLS_BIN}/setup-envtest" ]] || die "KUBEBUILDER_ASSETS unset and ${TOOLS_BIN}/setup-envtest missing (GOBIN=${TOOLS_BIN} go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.23)"
    assets="$("${TOOLS_BIN}/setup-envtest" use 1.35.x --bin-dir "${TOOLS_BIN}/envtest" -p path)"
  fi
  argo_dir="$(cd /tmp && go mod download -json "github.com/argoproj/argo-workflows/v3@${ARGO_WORKFLOWS_VERSION}" | jq -r .Dir)/manifests/base/crds/full"
  mkdir -p "${WORK_DIR}/crds/argo-events" "${WORK_DIR}/bin"
  "${HELM}" template ae argo-events --repo "${ARGO_HELM_REPO}" --version "${ARGO_EVENTS_CHART_VERSION}" --include-crds \
    | python3 -c "import sys,yaml
for d in yaml.safe_load_all(sys.stdin):
    if d and d.get('kind') == 'CustomResourceDefinition':
        open('${WORK_DIR}/crds/argo-events/' + d['metadata']['name'] + '.yaml', 'w').write(yaml.safe_dump(d))"
  (cd "${HERE}/apiserver" && go build -o "${WORK_DIR}/bin/apiserver" .)
  (cd "${IMAGES}/storage-mirror-module-app" && go build -o "${WORK_DIR}/bin/manager" ./cmd/manager)

  KCFG="${WORK_DIR}/apiserver.kubeconfig"
  rm -f "${KCFG}" "${WORK_DIR}/events.jsonl"
  KUBEBUILDER_ASSETS="${assets}" "${WORK_DIR}/bin/apiserver" "${KCFG}" "${MODULE_CHART}/crds" "${argo_dir}" "${WORK_DIR}/crds/argo-events" \
    >"${WORK_DIR}/apiserver.log" 2>&1 &
  pids+=($!)
  APISERVER_PIDS=("${pids[@]}")
  trap apiserver_cleanup EXIT
  TIMEOUT=120 POLL=1 wait_for test -s "${KCFG}" || die "apiserver did not start: $(tail -3 "${WORK_DIR}/apiserver.log")"
  KCTX="$("${KUBECTL}" --kubeconfig "${KCFG}" config current-context)"
  export KCFG KCTX
  assert_local_cluster

  render 10.96.0.250
  k create namespace argo-events >/dev/null
  k apply --validate=strict -f "${HERE}/manifests/platform.yaml" >/dev/null
  k apply --validate=strict -f "${RENDER}/module.yaml" >/dev/null
  k apply --validate=strict -f "${RENDER}/workflows.yaml" >/dev/null
  log "module, WorkflowTemplates, Sensors and platform objects accepted with strict validation"

  python3 "${HERE}/fake-webhook.py" 18999 "${WORK_DIR}/events.jsonl" &
  APISERVER_PIDS+=($!)
  KUBECONFIG="${KCFG}" MANAGER_NAMESPACE=horizon WORKFLOW_NAMESPACE=workflows JENKINS_NAMESPACE=jenkins POD_NAMESPACE=horizon \
    GOOGLE_CLOUD_PROJECT=e2e-project EVENTS_WEBHOOK_URL=http://127.0.0.1:18999/events/workflow \
    API_TLS_ADDR=127.0.0.1:18443 API_HTTP_ADDR=127.0.0.1:18080 \
    TEST_NFS_SERVER=10.96.0.250 "${WORK_DIR}/bin/manager" >"${WORK_DIR}/manager.log" 2>&1 &
  APISERVER_PIDS+=($!)
  # The controller creates the API Secrets on start.
  TIMEOUT=60 POLL=1 wait_for k -n horizon-api get secret storage-mirror-module-api-tls || die "controller did not create its Secrets: $(tail -3 "${WORK_DIR}/manager.log")"
  k -n horizon get secret storage-mirror-module-api-tls -o jsonpath='{.data.ca\.crt}' | base64 -d >"${WORK_DIR}/ca.crt"
  k -n horizon get secret storage-mirror-module-api-token -o jsonpath='{.data.default}' | base64 -d >"${WORK_DIR}/token"
  TIMEOUT=60 POLL=1 wait_for curl -fsS --cacert "${WORK_DIR}/ca.crt" -H "Authorization: Bearer $(cat "${WORK_DIR}/token")" \
    --resolve storage-mirror-module.horizon.svc.cluster.local:18443:127.0.0.1 \
    https://storage-mirror-module.horizon.svc.cluster.local:18443/api/v1/config || die "controller API not reachable: $(tail -3 "${WORK_DIR}/manager.log")"

  FAILED=0
  # shellcheck source=scenarios.sh
  source "${HERE}/scenarios.sh"
  run_apiserver_scenarios
  summary
  [[ "${FAILED}" == 0 ]]
}

main() {
  preflight
  if [[ "${APISERVER_ONLY}" == 1 ]]; then
    apiserver_main
    return
  fi
  # Test-only credential of the throwaway in-cluster Gitea; fixed so --reuse keeps working.
  GITEA_PASSWORD="${GITEA_PASSWORD:-e2e-local-Passw0rd}"
  export GITEA_PASSWORD
  if [[ "${RENDER_ONLY}" == 1 ]]; then
    render 10.96.0.250
    python3 -c "import yaml,sys; [list(yaml.safe_load_all(open(f))) for f in sys.argv[1:]]" \
      "${RENDER}/module.yaml" "${RENDER}/workflows.yaml" "${HERE}/manifests/infra.yaml" "${HERE}/manifests/platform.yaml"
    log "render-only: manifests in ${RENDER}"
    return
  fi
  trap cleanup EXIT
  create_cluster
  build_images
  install_infra
  seed create
  install_argo
  local nfs_ip
  nfs_ip="$(k -n e2e get svc nfs-server -o jsonpath='{.spec.clusterIP}')"
  render "${nfs_ip}"
  platform_policies
  install_module
  port_forward
  export -f k h log die wait_for pass fail skip check
  export KCFG KCTX KUBECTL HELM WORK_DIR HERE
  FAILED=0
  # shellcheck source=scenarios.sh
  source "${HERE}/scenarios.sh"
  run_scenarios
  summary
  [[ "${FAILED}" == 0 ]]
}

main "$@"
