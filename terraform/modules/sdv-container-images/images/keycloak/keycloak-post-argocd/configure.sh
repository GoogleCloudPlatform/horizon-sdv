#!/usr/bin/env bash

# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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

SECRET=$(cat client-argocd.json | jq -r ".secret")

kubectl -n argocd patch secret argocd-secret \
  --patch="{\"stringData\": { \"oidc.keycloak.clientSecret\": \"${SECRET}\" }}"

kubectl -n argocd patch configmap argocd-cm --patch="
{
  \"data\": {
    \"url\": \"${DOMAIN}/argocd\",
    \"oidc.config\": \"name: Keycloak\nissuer: ${DOMAIN}/auth/realms/horizon\nclientID: argocd\nclientSecret: \$oidc.keycloak.clientSecret\nrequestedScopes: [\\\"openid\\\", \\\"profile\\\", \\\"email\\\", \\\"groups\\\"]\"
  }
}"

kubectl -n argocd patch configmap argocd-rbac-cm --patch='
{
  "data": {
    "policy.csv": "g, horizon-argocd-administrators, role:admin"
  }
}'

kubectl rollout restart deployment/argocd-server -n argocd
