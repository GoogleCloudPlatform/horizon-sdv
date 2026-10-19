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

# Shared helpers for license WorkflowTemplates.
# After updating Secret abfs-license, optionally recreate the ABFS data
# ContainerNodePools (server and uploader) so create-time Helm lookup can
# stamp GCE metadata on each of them.
# Never patch KCC nodeConfig.metadata (immutable). Never call
# `gcloud container node-pools update --metadata` (flag does not exist).

{{- define "license.ensureKubectl" -}}
# google-cloud-cli images no longer ship kubectl (unlike legacy cloud-sdk tags).
if ! command -v kubectl >/dev/null 2>&1; then
  echo "Installing kubectl (gcloud components)..."
  gcloud components install kubectl --quiet
fi
{{- end -}}

{{- define "license.recreatePoolEnv" -}}
- name: ABFS_NAMESPACE
  value: {{ .Values.abfsNamespace | quote }}
- name: NODE_POOL_NAMES
  value: {{ .Values.dataNodePoolNames | default list | join " " | quote }}
- name: PROJECT_ID
  value: {{ .Values.projectID | quote }}
- name: SA_ID
  value: {{ .Values.serviceAccountId | quote }}
- name: IDENTITY_CM
  value: {{ .Values.identityConfigMapName | quote }}
{{- end -}}

{{- define "license.recreatePoolBash" -}}
{{ include "license.ensureKubectl" . }}
# Delete every configured ContainerNodePool that is present so Argo/KCC
# recreate it with the current Secret lookup in containernodepool.yaml.
# Pools are deleted together and waited on in one call. A missing pool is
# skipped; no pool at all → SKIP (exit 0).
recreate_data_node_pool_for_license() {
  if [[ -z "${ABFS_NAMESPACE}" || -z "${NODE_POOL_NAMES}" ]]; then
    echo "SKIP: abfsNamespace/dataNodePoolNames not configured for pool recreate."
    return 0
  fi
  local pool present=()
  for pool in ${NODE_POOL_NAMES}; do
    if kubectl get containernodepool "${pool}" -n "${ABFS_NAMESPACE}" >/dev/null 2>&1; then
      present+=("${pool}")
    else
      echo "SKIP: ContainerNodePool ${ABFS_NAMESPACE}/${pool} not found (enable abfs-service; PreSync will align on next sync)."
    fi
  done
  if [[ ${#present[@]} -eq 0 ]]; then
    echo "SKIP: none of the configured ContainerNodePools exist in ${ABFS_NAMESPACE}."
    return 0
  fi
  echo "Deleting ContainerNodePool(s) ${present[*]} in ${ABFS_NAMESPACE} so Argo recreates them with licence metadata..."
  kubectl delete containernodepool "${present[@]}" -n "${ABFS_NAMESPACE}" \
    --wait=true --timeout=45m
  echo "Deleted ${present[*]}. Argo automated sync will recreate the pool(s)."
}

# Run after Apply, before the Secret is written. A failed check exits
# without importing the payload.
verify_abfs_license() {
  if [[ -z "${PROJECT_ID}" || -z "${SA_ID}" ]]; then
    echo "ERROR: PROJECT_ID and SA_ID are required to verify the license" >&2
    exit 1
  fi
  local email="${SA_ID}@${PROJECT_ID}.iam.gserviceaccount.com"
  local unique_id=""
  unique_id="$(kubectl get configmap "${IDENTITY_CM}" -n "${HORIZON_NAMESPACE}" \
    -o jsonpath='{.data.uniqueId}' 2>/dev/null || true)"
  if [[ -z "${unique_id}" ]]; then
    unique_id="$(gcloud iam service-accounts describe "${email}" \
      --project="${PROJECT_ID}" --format='value(uniqueId)' 2>/dev/null || true)"
  fi
  if [[ -z "${unique_id}" ]]; then
    echo "ERROR: could not read UniqueID for ${email}" >&2
    exit 1
  fi
  local project_number=""
  project_number="$(gcloud projects describe "${PROJECT_ID}" \
    --format='value(projectNumber)' 2>/dev/null || true)"
  if [[ -z "${project_number}" ]]; then
    echo "ERROR: could not read project number for ${PROJECT_ID}" >&2
    exit 1
  fi
  LICENSE_PAYLOAD="${LICENSE_PAYLOAD}" \
    EXPECTED_UNIQUE_ID="${unique_id}" \
    EXPECTED_PROJECT_NUMBER="${project_number}" \
    python3 /etc/abfs/verify_license.py
}
{{- end -}}
