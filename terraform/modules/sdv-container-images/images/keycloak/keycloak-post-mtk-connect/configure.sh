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

OLD_KEY=$(kubectl get secret -n mtk-connect mtk-connect-keycloak -o jsonpath='{.data.privateKey}' | base64 -d)
OLD_CERT=$(kubectl get secret -n mtk-connect mtk-connect-keycloak -o jsonpath='{.data.idpCert}' | base64 -d)

if [ "${OLD_KEY}" != "$(<privateKey.pem)" ] || [ "${OLD_CERT}" != "$(<idpCert.pem)" ]; then
  echo "Restarting MTK Connect ..."
  kubectl -n mtk-connect delete secret mtk-connect-keycloak
  kubectl -n mtk-connect create secret generic mtk-connect-keycloak --from-file=privateKey=./privateKey.pem --from-file=idpCert=./idpCert.pem
  kubectl -n mtk-connect delete pod -l app=mtk-connect,type=deploy
else
  echo "mtk-connect-keycloak secret unchanged => no need to restart MTK Connect"
fi
