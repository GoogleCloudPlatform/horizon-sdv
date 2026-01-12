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

SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
NAMESPACE=$(cat ${SERVICEACCOUNT}/namespace)
TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt

# script steps:
# define temporary files used  for file operation - grafana configmap
# get and parse grafana-keycloak-secret from secret store, prepare format for replace
# get current grafana configmap to file
# replace 1st occurence of client_secret and save it as new output configmap file
# apply new grafana configmap into kubernetes
# parse pod name and reset grafana pod. New configuration will be applied into grafana
# clean up temp files

echo "POSTJOB GRAFANA: GET AND UPDATE KEYCLOAK SECRET FOR GRAFANA"

# temporary files used in operation
GRAFANA_TEMP_FILE="temp_grafana_cm.yaml"
UPDATED_GRAFANA_TEMP_FILE="updated_grafana_cm.yaml"

#get grafana-keycloak-secret and prepare NEW_SECRET string
NEW_SECRET="client_secret = "$(kubectl get secret --namespace monitoring  grafana-keycloak-secret -o jsonpath="{.data.client_secret}" | base64 --decode)


if [[ "${DEBUG:-0}" == "1" ]]; then
  echo "NEW_SECRET: "
  echo $NEW_SECRET
fi

#save current grafana configMap
kubectl get cm -n monitoring grafana -o yaml > $GRAFANA_TEMP_FILE

if [[ "${DEBUG:-0}" == "1" ]]; then
  echo "GRAFANA_TEMP_FILE with current secret: "
  cat $GRAFANA_TEMP_FILE | grep client
fi

#replace client_secret with NEW_SECRET
#eg. line :
#"client_secret = oldsecret_1234556545643"
# will be replaced with eg:
#"client_secret = iUTc644gHMUNgYVi18Fj8kkOzgfj6AQdkihhj7dhKg1BBh8a"

awk -v new_value="$NEW_SECRET" '
  BEGIN { replaced = 0 }
  {
    if (replaced == 0 && $0 ~ /^[[:space:]]*client_secret[[:space:]]*=/) {
      sub(/(client_secret[[:space:]]*=[[:space:]]*).*/,  new_value)
      replaced = 1
    }
    print
  }
' $GRAFANA_TEMP_FILE > $UPDATED_GRAFANA_TEMP_FILE

if [[ "${DEBUG:-0}" == "1" ]]; then
  echo "UPDATED_GRAFANA_TEMP_FILE: "
  cat $UPDATED_GRAFANA_TEMP_FILE | grep client
fi

# apply configMap with new secret into K8s
kubectl apply -f $UPDATED_GRAFANA_TEMP_FILE

 # reset grafana pod to apply changes
POD_NAME=$(kubectl get pods -n monitoring | grep grafana | grep -v post | awk '{print $1}')
kubectl delete pod $POD_NAME -n monitoring

 #clean up files
rm $GRAFANA_TEMP_FILE $UPDATED_GRAFANA_TEMP_FILE

