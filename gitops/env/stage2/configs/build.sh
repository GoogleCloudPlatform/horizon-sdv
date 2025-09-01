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

VERSION=1.0.0
#GCP_PROJECT_ID=<PROJECT_ID>
#GCP_CLOUD_REGION=<REGION_NAME>

declare -a configs=("landingpage-app")
substr="-app"
for config in "${configs[@]}"; do
  docker build -t ${GCP_CLOUD_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/horizon-sdv/${config}:${VERSION} ${config%$substr*}/${config}
  docker push ${GCP_CLOUD_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/horizon-sdv/${config}:${VERSION}
done

declare -a configs=("gerrit-post" "mtk-connect-post" "mtk-connect-post-key" "keycloak-post" "keycloak-post-gerrit" "keycloak-post-jenkins" "keycloak-post-argocd" "keycloak-post-mtk-connect")
substr="-post"
for config in "${configs[@]}"; do
  docker build -t ${GCP_CLOUD_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/horizon-sdv/${config}:${VERSION} ${config%$substr*}/${config}
  docker push ${GCP_CLOUD_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/horizon-sdv/${config}:${VERSION}
done
