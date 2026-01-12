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

APISERVER=https://kubernetes.default.svc
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
NAMESPACE=$(cat ${SERVICEACCOUNT}/namespace)
TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt

npm install
node keycloak.mjs
WEB_CLIENT_SECRET=$(cat client-mcp-gateway-registry-web.json | jq -r ".secret")
sed -i "s/##SECRET##/${WEB_CLIENT_SECRET}/g" ./web-client-secret.json
CLI_CLIENT_SECRET=$(cat client-mcp-gateway-registry-cli.json | jq -r ".secret")
sed -i "s/##SECRET##/${CLI_CLIENT_SECRET}/g" ./cli-client-secret.json

# Update the web client k8s secret
curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/api/v1/namespaces/mcp-gateway-registry/secrets/mcp-gateway-registry-web-keycloak-secret
curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST ${APISERVER}/api/v1/namespaces/mcp-gateway-registry/secrets -d @web-client-secret.json

# Update the cli client k8s secret
curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/api/v1/namespaces/mcp-gateway-registry/secrets/mcp-gateway-registry-cli-keycloak-secret
curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST ${APISERVER}/api/v1/namespaces/mcp-gateway-registry/secrets -d @cli-client-secret.json