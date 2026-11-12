#!/usr/bin/env bash

# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
set -e

log() {
  echo "$(date -u +'%Y-%m-%dT%H:%M:%SZ') [mtk-connect-post] $*"
}

log "Script start"

APISERVER=https://kubernetes.default.svc
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
NAMESPACE=$(cat ${SERVICEACCOUNT}/namespace)
TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt

cd /root
log "Running in namespace=${NAMESPACE}, NAMESPACE_PREFIX=${NAMESPACE_PREFIX}"

MTK_CONNECT_POD=$(kubectl get pod -l app.kubernetes.io/name=mtk-connect -n "${NAMESPACE}" -o name | sed 's@^pod/@@')
log "Found mtk-connect pod: ${MTK_CONNECT_POD}"

log "Creating service account API key for mtk-connect-admin"
MTKC_APIKEY=$(kubectl exec "${MTK_CONNECT_POD}" -n "${NAMESPACE}" -c authenticator -- node createServiceAccount.js mtk-connect-admin)
log "Service account API key created (length=${#MTKC_APIKEY})"

log "Preparing secret manifests"
sed -i "s/##MTKC_APIKEY##/${MTKC_APIKEY}/g" ./secret-jenkins.json
sed -i "s/##NAMESPACE##/${NAMESPACE_PREFIX}jenkins/g" ./secret-jenkins.json
sed -i "s/##MTKC_APIKEY##/${MTKC_APIKEY}/g" ./secret-mtk-connect.json
sed -i "s/##NAMESPACE##/${NAMESPACE_PREFIX}mtk-connect/g" ./secret-mtk-connect.json
sed -i "s/##MTKC_APIKEY##/${MTKC_APIKEY}/g" ./secret-workflows.json
sed -i "s/##NAMESPACE##/${NAMESPACE_PREFIX}workflows/g" ./secret-workflows.json

# DELETE may 404 on first run when secrets do not exist yet
log "Creating jenkins secret in namespace=${NAMESPACE_PREFIX}jenkins"
curl -sS -o /dev/null -w 'DELETE jenkins-mtk-connect-apikey: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -X DELETE "${APISERVER}/api/v1/namespaces/${NAMESPACE_PREFIX}jenkins/secrets/jenkins-mtk-connect-apikey" || true
curl -fsS -o /dev/null -w 'POST jenkins-mtk-connect-apikey: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST "${APISERVER}/api/v1/namespaces/${NAMESPACE_PREFIX}jenkins/secrets" -d @secret-jenkins.json
log "Jenkins secret created"

log "Creating mtk-connect secret in namespace=${NAMESPACE_PREFIX}mtk-connect"
curl -sS -o /dev/null -w 'DELETE mtk-connect-apikey: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -X DELETE "${APISERVER}/api/v1/namespaces/${NAMESPACE_PREFIX}mtk-connect/secrets/mtk-connect-apikey" || true
curl -fsS -o /dev/null -w 'POST mtk-connect-apikey: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST "${APISERVER}/api/v1/namespaces/${NAMESPACE_PREFIX}mtk-connect/secrets" -d @secret-mtk-connect.json
log "mtk-connect secret created"

log "Creating workflows secret in namespace=${NAMESPACE_PREFIX}workflows"
curl -sS -o /dev/null -w 'DELETE workflow-mtk-connect-apikey: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -X DELETE "${APISERVER}/api/v1/namespaces/${NAMESPACE_PREFIX}workflows/secrets/workflow-mtk-connect-apikey" || true
curl -fsS -o /dev/null -w 'POST workflow-mtk-connect-apikey: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST "${APISERVER}/api/v1/namespaces/${NAMESPACE_PREFIX}workflows/secrets" -d @secret-workflows.json
log "Workflows secret created"

log "Script end"
