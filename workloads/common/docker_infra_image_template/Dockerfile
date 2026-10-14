# Copyright (c) 2025-2026 Accenture, All Rights Reserved.
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
# A common template for usage in ABFS server, uploader creation and
# networking utilities.

# Latest stable.
ARG LINUX_DISTRIBUTION=debian:12
FROM ${LINUX_DISTRIBUTION}

ARG TERRAFORM_VERSION=""

# Gcloud CLI and kubectl version.
ARG GCLOUD_CLI_VERSION="latest"
ARG KUBECTL_VERSION="latest"

# NODE required for gemini
ARG NODEJS_VERSION

# Install gemini-cli if enabled.
ARG ENABLE_GEMINI_AI_ASSISTANT="false"
ARG GEMINI_CLI_VERSION="latest"
ARG ANTIGRAVITY_SDK_VERSION="0.1.16"

# Install common packages. python3, python3-yaml (PyYAML) for Gemini skills conversion at runtime;
# python3-numpy for SSH IAP tunnelling. libsecret-1-0 for clients that use the Secret Service API
# (avoids missing libsecret-1.so.0 when running gemini-cli / IDE credential helpers in the image).
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends apt-utils && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates \
    gnupg software-properties-common curl git sudo jq openssh-client procps libsecret-1-0 \
    zip unzip python3 python3-yaml python3-numpy python3-pip && \
    apt-get update -y &&  apt install -y npm && \
    rm -rf /var/lib/apt/lists/*

# The following installs gcloud sdk including gcloud storage required for uploading
# artifacts to Google Cloud Storage. Remove if using alternative storage.
# https://cloud.google.com/sdk/gcloud/reference/storage
# GCS:START
RUN curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | \
    gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | \
    tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && \
    apt-get update -y && \
    [ "$GCLOUD_CLI_VERSION" = "latest" ] && apt-get install -y google-cloud-cli || \
      apt-get install -y google-cloud-cli="$GCLOUD_CLI_VERSION" google-cloud-cli-anthoscli="$GCLOUD_CLI_VERSION" && \
    [ "$KUBECTL_VERSION" = "latest" ] &&  apt-get install -y kubectl || apt-get install -y kubectl="$KUBECTL_VERSION" && \
    apt-get -y clean
# GCS:END

# Install Terraform command tool
RUN if [ -n "${TERRAFORM_VERSION}" ]; then \
        curl -fsSL https://apt.releases.hashicorp.com/gpg | apt-key add - && \
        apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) ${TERRAFORM_VERSION}" && \
        apt-get update -y && apt-get install -y terraform && \
        apt-get -y clean; \
    fi


# Add USER and provide sudo access.
ARG USER=builder
RUN useradd -u 1000 -ms /bin/bash ${USER} && \
    passwd -d ${USER} && \
    usermod -aG sudo ${USER} && \
    echo "${USER} ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers

# Switch to user
USER ${USER}
WORKDIR /home/${USER}

# Configure git config (fake details) for Google repo.
RUN git config --global user.email "abfs.jenkins@dummy.com" && \
    git config --global user.name "ABFS Jenkins" && \
    git config --global color.ui false

# Node version manager
RUN if [ "$ENABLE_GEMINI_AI_ASSISTANT" = "true" ]; then \
      echo "\033[1;32mInstalling gemini-cli@${GEMINI_CLI_VERSION}\033[0m" && \
      sudo npm cache clean -f && \
      sudo npm install -g n && \
      sudo n "${NODEJS_VERSION}" && \
      sudo ln -sf /usr/local/bin/node /usr/local/bin/nodejs || true && \
      sudo npm install -g @google/gemini-cli@${GEMINI_CLI_VERSION} && \
      sudo rm -f /usr/lib/python3*/EXTERNALLY-MANAGED && \
      echo "\033[1;32mInstalling google-antigravity@${ANTIGRAVITY_SDK_VERSION}\033[0m" && \
      sudo pip3 install "google-antigravity==${ANTIGRAVITY_SDK_VERSION}" && \
      sudo pip install "google-cloud-logging"; \
    fi
