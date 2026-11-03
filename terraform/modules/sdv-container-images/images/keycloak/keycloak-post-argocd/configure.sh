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

APISERVER=https://kubernetes.default.svc
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
NAMESPACE=$(cat ${SERVICEACCOUNT}/namespace)
TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt

npm install --ignore-scripts
node keycloak.mjs

SECRET=$(cat client-argocd.json | jq -r ".secret")

kubectl -n ${NAMESPACE_PREFIX}argocd patch secret argocd-secret \
  --patch="{\"stringData\": { \"oidc.keycloak.clientSecret\": \"${SECRET}\" }}"

kubectl -n ${NAMESPACE_PREFIX}argocd patch configmap argocd-cm --patch="
{
  \"data\": {
    \"url\": \"${DOMAIN}/argocd\",
    \"oidc.groupsClaim\": \"roles\",
    \"oidc.config\": \"name: Keycloak\nissuer: ${DOMAIN}/auth/realms/horizon\nclientID: argocd\nclientSecret: \$oidc.keycloak.clientSecret\nrequestedScopes: [\\\"openid\\\", \\\"profile\\\", \\\"email\\\", \\\"roles\\\"]\"
  }
}"

# POLICY_CSV must match line-for-line: terraform/modules/sdv-gke-apps/argocd-values.yaml.tpl → configs.rbac.policy.csv (edit the .tpl first).
# This Job patches rbac after Keycloak; Helm upgrades still reapply from values — keep both copies equal or RBAC/UI diverges.
read -r -d '' POLICY_CSV << 'POLICY_END' || true
p, role:readonly, applications, get, */*, allow
p, role:readonly, applications, list, */*, allow
p, role:readonly, applicationsets, get, */*, allow
p, role:readonly, applicationsets, list, */*, allow
p, role:readonly, certificates, get, *, allow
p, role:readonly, clusters, get, *, allow
p, role:readonly, clusters, list, *, allow
p, role:readonly, repositories, get, *, allow
p, role:readonly, write-repositories, get, *, allow
p, role:readonly, projects, get, *, allow
p, role:readonly, projects, list, *, allow
p, role:readonly, accounts, get, *, allow
p, role:readonly, gpgkeys, get, *, allow
p, role:readonly, logs, get, */*, allow
p, role:admin, applications, create, */*, allow
p, role:admin, applications, update, */*, allow
p, role:admin, applications, update/*, */*, allow
p, role:admin, applications, delete, */*, allow
p, role:admin, applications, delete/*, */*, allow
p, role:admin, applications, sync, */*, allow
p, role:admin, applications, override, */*, allow
p, role:admin, applications, action/*, */*, allow
p, role:admin, applications, list, */*, allow
p, role:admin, applicationsets, get, */*, allow
p, role:admin, applicationsets, list, */*, allow
p, role:admin, applicationsets, create, */*, allow
p, role:admin, applicationsets, update, */*, allow
p, role:admin, applicationsets, delete, */*, allow
p, role:admin, certificates, create, *, allow
p, role:admin, certificates, update, *, allow
p, role:admin, certificates, delete, *, allow
p, role:admin, clusters, create, *, allow
p, role:admin, clusters, update, *, allow
p, role:admin, clusters, delete, *, allow
p, role:admin, clusters, list, *, allow
p, role:admin, repositories, create, *, allow
p, role:admin, repositories, update, *, allow
p, role:admin, repositories, delete, *, allow
p, role:admin, write-repositories, create, *, allow
p, role:admin, write-repositories, update, *, allow
p, role:admin, write-repositories, delete, *, allow
p, role:admin, projects, create, *, allow
p, role:admin, projects, update, *, allow
p, role:admin, projects, delete, *, allow
p, role:admin, projects, list, *, allow
p, role:admin, accounts, update, *, allow
p, role:admin, gpgkeys, create, *, allow
p, role:admin, gpgkeys, delete, *, allow
p, role:admin, exec, create, */*, allow
g, role:admin, role:readonly
g, administrators, role:admin
g, viewers, role:readonly
POLICY_END

POLICY_CSV_JSON=$(echo "$POLICY_CSV" | jq -Rs .)

kubectl -n ${NAMESPACE_PREFIX}argocd patch configmap argocd-rbac-cm --patch="{\"data\": {\"policy.csv\": $POLICY_CSV_JSON, \"scopes\": \"[roles]\"}}"

kubectl rollout restart deployment/${NAMESPACE_PREFIX}argocd-server -n ${NAMESPACE_PREFIX}argocd
