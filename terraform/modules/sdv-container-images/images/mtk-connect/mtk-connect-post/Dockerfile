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

FROM --platform=linux/amd64 debian:12.13
ADD configure.sh /root
ADD secret-jenkins.json /root
ADD secret-mtk-connect.json /root
ADD secret-workflows.json /root
# Add security repository and upgrade all packages to fix CVEs
RUN echo "deb http://security.debian.org/debian-security bookworm-security main" > /etc/apt/sources.list.d/security.list && \
    apt update -y && apt upgrade -y && \
    apt install vim curl skopeo jq -y && \
    apt clean && rm -rf /var/lib/apt/lists/*
# Pin to the GKE 1.34 client already used by the Cloud Workstations workflow image.
ARG KUBECTL_VERSION=v1.34.12
RUN curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256" | \
      awk '{print $1"  /usr/local/bin/kubectl"}' | sha256sum -c - && \
    chmod +x /usr/local/bin/kubectl && \
    chmod +x /root/configure.sh
CMD ["/bin/bash", "-c", "/root/configure.sh"]
