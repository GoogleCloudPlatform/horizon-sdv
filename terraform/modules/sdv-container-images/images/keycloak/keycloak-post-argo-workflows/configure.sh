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

set -euo pipefail

APISERVER=https://kubernetes.default.svc
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt
TARGET_NAMESPACE=${NAMESPACE_PREFIX}argo-workflows

# @keycloak/keycloak-admin-client 26+ runs a postinstall that invokes pnpm (not in image).
# Published tarballs are complete; lifecycle scripts are not needed at runtime.
npm install --ignore-scripts
node keycloak.mjs

if [[ ! -s client-argo-workflows.json ]]; then
  echo "client-argo-workflows.json was not generated or is empty" >&2
  exit 1
fi

CLIENT_SECRET=$(jq -er '.secret' client-argo-workflows.json)
sed -i "s/##SECRET##/${CLIENT_SECRET}/g" ./secret.json
sed -i "s/##NAMESPACE##/${TARGET_NAMESPACE}/g" ./secret.json

# Recreate Argo Workflows SSO secret with current client credentials
curl -sS -o /dev/null -w 'DELETE argo-workflows-sso: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -X DELETE "${APISERVER}/api/v1/namespaces/${TARGET_NAMESPACE}/secrets/argo-workflows-sso" || true
curl -fsS -o /dev/null -w 'POST argo-workflows-sso: %{http_code}\n' --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST "${APISERVER}/api/v1/namespaces/${TARGET_NAMESPACE}/secrets" -d @secret.json

echo "Argo Workflows Keycloak configuration completed successfully"
