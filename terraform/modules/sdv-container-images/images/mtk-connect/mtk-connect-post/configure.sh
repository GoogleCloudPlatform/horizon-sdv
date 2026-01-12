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

cd /root
MTKC_APIKEY=$(kubectl exec "$(kubectl get pod -l app=mtk-connect -n ${NAMESPACE} -o name | sed 's@^pod/@@')" -n ${NAMESPACE} -c authenticator -- node createServiceAccount.js mtk-connect-admin)

if [ $? -eq 0 ]; then
  sed -i "s/##MTKC_APIKEY##/${MTKC_APIKEY}/g" ./secret-jenkins.json
  sed -i "s/##MTKC_APIKEY##/${MTKC_APIKEY}/g" ./secret-mtk-connect.json

  curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/api/v1/namespaces/jenkins/secrets/jenkins-mtk-connect-apikey
  curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST ${APISERVER}/api/v1/namespaces/jenkins/secrets -d @secret-jenkins.json

  curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/api/v1/namespaces/mtk-connect/secrets/mtk-connect-apikey
  curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST ${APISERVER}/api/v1/namespaces/mtk-connect/secrets -d @secret-mtk-connect.json
fi
