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

FROM node:22.13.0
RUN apt update -y && apt install vim -y
WORKDIR /home/node
ADD package.json ./
ADD keycloak.mjs ./
ADD secret.json ./
ADD configure.sh ./
RUN chmod +x configure.sh
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
RUN mv ./kubectl /usr/local/bin && chmod +x /usr/local/bin/kubectl
USER node
CMD ["/bin/bash", "-c", "/home/node/configure.sh"]
