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

set -euo pipefail

APISERVER=https://kubernetes.default.svc
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
TOKEN=$(cat "${SERVICEACCOUNT}/token")
CACERT=${SERVICEACCOUNT}/ca.crt

npm install
node keycloak.mjs

SECRET=$(tr -d '\n\r' <horizon-api-ci-client-secret)
JENKINS_NS="${NAMESPACE_PREFIX}jenkins"
SECRET_JSON=$(
  jq -n \
    --arg ns "$JENKINS_NS" \
    --arg secret "$SECRET" \
    '{
      apiVersion: "v1",
      kind: "Secret",
      metadata: {
        name: "jenkins-horizon-api-ci-secret",
        namespace: $ns,
        labels: {"jenkins.io/credentials-type": "secretText"},
        annotations: {"jenkins.io/credentials-description": "Keycloak horizon-api-ci client secret for Horizon API pipelines"}
      },
      type: "Opaque",
      stringData: {text: $secret}
    }'
)

curl -sS -o /dev/null -w 'DELETE jenkins-horizon-api-ci-secret: %{http_code}\n' \
  --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" \
  -X DELETE "${APISERVER}/api/v1/namespaces/${JENKINS_NS}/secrets/jenkins-horizon-api-ci-secret" || true

printf '%s' "${SECRET_JSON}" | curl -fsS -o /dev/null -w 'POST jenkins-horizon-api-ci-secret: %{http_code}\n' \
  --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" \
  -H 'Accept: application/json' -H 'Content-Type: application/json' \
  -X POST "${APISERVER}/api/v1/namespaces/${JENKINS_NS}/secrets" \
  --data-binary @-

# The Developer Portal proxy forwards the user's own token and needs no client secret.
# Delete the copy of this secret that earlier versions of this job wrote into its namespace.
DEVPORTAL_NS="${NAMESPACE_PREFIX}horizon-dev-portal"
SECRET_NAME="${NAMESPACE_PREFIX}horizon-dev-portal-secrets"
curl -sS -o /dev/null -w "DELETE ${SECRET_NAME}: %{http_code}\n" \
  --cacert "${CACERT}" --header "Authorization: Bearer ${TOKEN}" \
  -X DELETE "${APISERVER}/api/v1/namespaces/${DEVPORTAL_NS}/secrets/${SECRET_NAME}" || true
