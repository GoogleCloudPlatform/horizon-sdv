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

ES_NS=external-secrets
ES_NAME=external-secrets
ES_CHART=external-secrets
ES_PATH=external-secrets
ES_VERSION=0.10.4
ES_VALUES=external-secrets-values.yaml

ARGOCD_NS=argocd
ARGOCD_NAME=argocd
ARGOCD_CHART=argo-cd
ARGOCD_PATH=argocd
ARGOCD_VERSION=7.6.12
ARGOCD_VALUES=argocd-values.yaml

REPOSITORY=https://github.com/${GITHUB_REPO_NAME}

helm repo add argocd https://argoproj.github.io/argo-helm
helm repo add external-secrets https://charts.external-secrets.io
helm repo update

deploy() {
  D_NS=$1
  D_NAME=$2
  D_CHART=$3
  D_PATH=$4
  D_VERSION=$5
  D_VALUES=$6

  D_STATUS=$(helm list -n $D_NS | grep $D_CHART-$D_VERSION | awk '{print $1 " " $2 " " $8 " " $9}')
  if [ "$D_STATUS" == "$D_NAME $D_NS deployed $D_CHART-$D_VERSION" ]; then
    D_DIFF=$(helm diff upgrade $D_NAME $D_PATH/$D_CHART --namespace $D_NS --version $D_VERSION)
    if [ "$D_DIFF" != "" ]; then
      helm upgrade $D_NAME $D_PATH/$D_CHART -n $D_NS --create-namespace --values $D_VALUES --version $D_VERSION --wait
      sleep 10
    fi
  else
    helm install $D_NAME $D_PATH/$D_CHART -n $D_NS --create-namespace --values $D_VALUES --version $D_VERSION --wait
    sleep 10
  fi
}

deploy $ES_NS $ES_NAME $ES_CHART $ES_PATH $ES_VERSION $ES_VALUES

sed -i "s,##REPO_URL##,${REPOSITORY},g" ./argocd-secrets.yaml
sed -i "s,##PROJECT_ID##,${GCP_PROJECT_ID},g" ./argocd-secrets.yaml
sed -i "s,##CLOUD_REGION##,${GCP_CLOUD_REGION},g" ./argocd-secrets.yaml

kubectl apply -f argocd-secrets.yaml

sed -i "s,##SUBDOMAIN##,${GITHUB_ENV_NAME},g" ./argocd-values.yaml
sed -i "s,##DOMAIN##,${GITHUB_DOMAIN_NAME},g" ./argocd-values.yaml

deploy $ARGOCD_NS $ARGOCD_NAME $ARGOCD_CHART $ARGOCD_PATH $ARGOCD_VERSION $ARGOCD_VALUES

sed -i "s,##REPO_URL##,${REPOSITORY},g" ./argocd-config.yaml
sed -i "s,##REPO_BRANCH##,env/${GITHUB_ENV_NAME},g" ./argocd-config.yaml
sed -i "s,##DOMAIN##,${GITHUB_ENV_NAME}.${GITHUB_DOMAIN_NAME},g" ./argocd-config.yaml
sed -i "s,##PROJECT_ID##,${GCP_PROJECT_ID},g" ./argocd-config.yaml
sed -i "s,##CLOUD_REGION##,${GCP_CLOUD_REGION},g" ./argocd-config.yaml
sed -i "s,##CLOUD_ZONE##,${GCP_CLOUD_ZONE},g" ./argocd-config.yaml
sed -i "s,##CLOUD_BACKEND_BUCKET##,${GCP_BACKEND_BUCKET_NAME},g" ./argocd-config.yaml

kubectl apply -f argocd-config.yaml
