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
#
# Description:
# Sync a local repo directory into a PVC using a helper pod.
# On GKE, RWO volume attach to the node can take many minutes; the default wait
# for the pod to become Ready is 30m (override with --pod-ready-timeout or
# REPO_SYNC_POD_READY_TIMEOUT).
#
# StorageClasses with volumeBindingMode: WaitForFirstConsumer do not bind the PVC
# until a pod that uses the claim is scheduled. This script creates the helper pod
# first, then waits for the PVC to become Bound, then for the pod to be Ready.

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  repo-sync-pvc.sh -n <namespace> -p <pvc-name> -s <local-path> [-m <mount-path>] [--kubectl-timeout <duration>] [--pod-ready-timeout <duration>] [--clean] [--exclude-git] [--exclude-terraform]

Options:
  -n  Kubernetes namespace (e.g., jenkins)
  -p  PVC name (e.g., workloads-repo-pvc)
  -s  Local path to repo (absolute path)
  -m  Mount path in pod (default: /repo)
  --kubectl-timeout  kubectl request timeout (default: 0, no timeout)
  --pod-ready-timeout  Max time to wait for the helper pod to be Ready (default: 30m). RWO attach on GKE can exceed 2m.
  --clean  Delete existing contents in the mount path before copy
  --exclude-git  Skip .git directory
  --exclude-terraform  Skip terraform directory

Environment:
  REPO_SYNC_POD_READY_TIMEOUT  Same as --pod-ready-timeout (default: 30m)

Example:
  tools/scripts/repo-sync-pvc.sh \
    -n jenkins \
    -p workloads-repo-pvc \
    -s "/Users/dave.m.smith/Horizon/source/sdv/acn-horizon-sdv"
EOF
}

NAMESPACE=""
PVC_NAME=""
LOCAL_PATH=""
MOUNT_PATH="/repo"
KUBECTL_TIMEOUT="0"
POD_READY_TIMEOUT="${REPO_SYNC_POD_READY_TIMEOUT:-30m}"
EXCLUDE_ARGS=()
CLEAN_MOUNT="false"
ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --clean)
      CLEAN_MOUNT="true"
      shift
      ;;
    --exclude-git)
      EXCLUDE_ARGS+=("--exclude=.git")
      shift
      ;;
    --exclude-terraform)
      EXCLUDE_ARGS+=("--exclude=terraform")
      shift
      ;;
    --kubectl-timeout)
      KUBECTL_TIMEOUT="${2:-}"
      if [ -z "${KUBECTL_TIMEOUT}" ]; then
        echo "Missing value for --kubectl-timeout" >&2
        usage
        exit 1
      fi
      shift 2
      ;;
    --pod-ready-timeout)
      POD_READY_TIMEOUT="${2:-}"
      if [ -z "${POD_READY_TIMEOUT}" ]; then
        echo "Missing value for --pod-ready-timeout" >&2
        usage
        exit 1
      fi
      shift 2
      ;;
    *)
      ARGS+=("$1")
      shift
      ;;
  esac
done

set -- "${ARGS[@]}"
OPTIND=1
while getopts ":n:p:s:m:h" opt; do
  case "${opt}" in
    n) NAMESPACE="${OPTARG}" ;;
    p) PVC_NAME="${OPTARG}" ;;
    s) LOCAL_PATH="${OPTARG}" ;;
    m) MOUNT_PATH="${OPTARG}" ;;
    h)
      usage
      exit 0
      ;;
    \?)
      echo "Unknown option: -${OPTARG}" >&2
      usage
      exit 1
      ;;
    :)
      echo "Missing argument for -${OPTARG}" >&2
      usage
      exit 1
      ;;
  esac
done

if [ -z "${NAMESPACE}" ] || [ -z "${LOCAL_PATH}" ]; then
  echo "Missing required arguments." >&2
  usage
  exit 1
fi
if [ -z "${PVC_NAME}" ]; then
  PVC_NAME="workloads-repo-pvc"
fi

if ! kubectl -n "${NAMESPACE}" get pvc "${PVC_NAME}" >/dev/null 2>&1; then
  kubectl -n "${NAMESPACE}" apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${PVC_NAME}
  namespace: ${NAMESPACE}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
EOF
fi

if [ ! -d "${LOCAL_PATH}" ]; then
  echo "Local path does not exist or is not a directory: ${LOCAL_PATH}" >&2
  exit 1
fi
LOCAL_PATH="${LOCAL_PATH%/}"

POD_NAME="repo-sync-${PVC_NAME}"

kubectl -n "${NAMESPACE}" apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
spec:
  restartPolicy: Never
  containers:
    - name: repo-sync
      image: alpine:3.24
      command: ["sleep", "3600"]
      volumeMounts:
        - name: repo
          mountPath: ${MOUNT_PATH}
  volumes:
    - name: repo
      persistentVolumeClaim:
        claimName: ${PVC_NAME}
EOF

# WaitForFirstConsumer: binding happens after the pod above is scheduled as first consumer.
echo "Waiting for pvc/${PVC_NAME} to reach Bound (provisioner may need the scheduled pod)..."
if ! kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Bound "pvc/${PVC_NAME}" --timeout=600s; then
  echo "PVC ${PVC_NAME} did not reach Bound." >&2
  kubectl -n "${NAMESPACE}" describe pvc "${PVC_NAME}" >&2
  kubectl -n "${NAMESPACE}" describe pod "${POD_NAME}" >&2
  exit 1
fi

if ! kubectl -n "${NAMESPACE}" wait --for=condition=Ready "pod/${POD_NAME}" --timeout="${POD_READY_TIMEOUT}"; then
  echo "Timed out waiting for pod/${POD_NAME} to be Ready after ${POD_READY_TIMEOUT} (volume attach on GKE can take many minutes)." >&2
  kubectl -n "${NAMESPACE}" describe pod "${POD_NAME}" >&2
  exit 1
fi

if [ "${CLEAN_MOUNT}" = "true" ]; then
  kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- sh -c "rm -rf ${MOUNT_PATH}/* ${MOUNT_PATH}/.[!.]* ${MOUNT_PATH}/..?* || true"
fi

# Copy repo contents into the mount root so /workspace-local/workloads/... exists.
kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- sh -c "mkdir -p ${MOUNT_PATH} && touch ${MOUNT_PATH}/.sync-test && rm -f ${MOUNT_PATH}/.sync-test"
TOTAL_KB=$(du -sk "${LOCAL_PATH}" | awk '{print $1}')
TOTAL_BYTES=$((TOTAL_KB * 1024))
echo "Syncing ${TOTAL_KB} KB from ${LOCAL_PATH} to ${NAMESPACE}/${PVC_NAME}:${MOUNT_PATH}"

TAR_CMD=(tar "${EXCLUDE_ARGS[@]}" -C "${LOCAL_PATH}" -cf - .)
KUBECTL_EXEC=(kubectl -n "${NAMESPACE}" exec -i --request-timeout="${KUBECTL_TIMEOUT}" "${POD_NAME}" -- tar -C "${MOUNT_PATH}" --no-same-owner --no-same-permissions -xf -)

if command -v pv >/dev/null 2>&1; then
  "${TAR_CMD[@]}" | pv -pterb -s "${TOTAL_BYTES}" | "${KUBECTL_EXEC[@]}"
else
  echo "pv not found; streaming without progress (install pv for progress output)."
  "${TAR_CMD[@]}" | "${KUBECTL_EXEC[@]}"
fi
kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- sh -c "ls -la ${MOUNT_PATH} | head -50"

kubectl -n "${NAMESPACE}" delete pod "${POD_NAME}" --ignore-not-found
