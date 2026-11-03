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

FROM --platform=linux/amd64 alpine:3.24
ARG KUBECTL_VERSION=v1.34.12
RUN apk upgrade --no-cache && \
    apk add --no-cache bash && \
    apk add --no-cache --virtual .build-deps curl && \
    curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256" | \
      awk '{print $1"  /usr/local/bin/kubectl"}' | sha256sum -c - && \
    chmod +x /usr/local/bin/kubectl && \
    apk del .build-deps && \
    addgroup -g 1000 node && \
    adduser -u 1000 -G node -s /bin/bash -D node
WORKDIR /home/node
COPY --chown=node:node configure.sh ./
# Legacy builder (Terraform docker provider) does not support COPY --chmod.
RUN chmod +x configure.sh
USER node
CMD ["/bin/bash", "-c", "/home/node/configure.sh"]
