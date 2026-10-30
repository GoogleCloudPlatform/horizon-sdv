# Copyright (c) 2026 Accenture, All Rights Reserved.
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
#
# Slim Cloud Workstations workflow image: gcloud, kubectl, and jq.
# Replaces the Jenkins environment image (no Terraform, git, or sudo).
# USER 65534 matches the workflow step securityContext (nobody).
# kubectl is pinned to a GKE 1.34 patch; keep it within one minor of the control plane.
# Do not install kubectl from the Cloud SDK apt repo: that tracks latest, not GKE.
ARG DEBIAN_IMAGE=debian:12.12-slim
FROM ${DEBIAN_IMAGE}

ARG GCLOUD_CLI_VERSION=545.0.0-0
ARG KUBECTL_VERSION=v1.34.12
ARG KUBECTL_SHA256=90b7b9058ffeb5c10710bb1c73f541eaf426fb1b4e87df435db669de9a2564a2

RUN apt-get update -y \
  && apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates curl gnupg jq \
  && curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg \
    | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
  && echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" \
    > /etc/apt/sources.list.d/google-cloud-sdk.list \
  && apt-get update -y \
  && apt-get install -y --no-install-recommends "google-cloud-cli=${GCLOUD_CLI_VERSION}" \
  && curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
  && echo "${KUBECTL_SHA256}  /usr/local/bin/kubectl" | sha256sum -c - \
  && chmod 0755 /usr/local/bin/kubectl \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

ENV HOME=/tmp \
    CLOUDSDK_CONFIG=/tmp/gcloud
USER 65534:65534
WORKDIR /tmp
