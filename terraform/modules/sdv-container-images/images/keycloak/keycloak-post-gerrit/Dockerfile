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

FROM --platform=linux/amd64 node:22-alpine
RUN apk update && apk upgrade && apk add --no-cache bash vim curl jq
# ISD requires openssh >= 10.3, not yet in Alpine 3.23 stable; pull it from the edge branch.
RUN apk add --no-cache --upgrade \
    --repository=https://dl-cdn.alpinelinux.org/alpine/edge/main \
    openssh-client
WORKDIR /home/node
ADD --chown=node:node package.json ./
ADD --chown=node:node keycloak.mjs ./
ADD --chown=node:node configure.sh ./
ADD --chown=node:node secret.json ./
ADD --chown=node:node secure.config ./
RUN chmod +x configure.sh
USER node
CMD ["/bin/bash", "-c", "/home/node/configure.sh"]
