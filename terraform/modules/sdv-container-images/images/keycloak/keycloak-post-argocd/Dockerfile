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

FROM --platform=linux/amd64 node:22-alpine
RUN apk update && apk upgrade && apk add --no-cache bash vim curl jq
WORKDIR /home/node
ADD --chown=node:node package.json ./
ADD --chown=node:node keycloak.mjs ./
ADD --chown=node:node configure.sh ./
RUN chmod +x configure.sh
# Pin to the GKE 1.34 client already used by the Cloud Workstations workflow image.
ARG KUBECTL_VERSION=v1.34.12
RUN curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256" | \
      awk '{print $1"  /usr/local/bin/kubectl"}' | sha256sum -c - && \
    chmod +x /usr/local/bin/kubectl
USER node
CMD ["/bin/bash", "-c", "/home/node/configure.sh"]
