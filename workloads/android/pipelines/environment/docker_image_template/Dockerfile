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

# Allow user to choose distribution, e.g. ubuntu:22.04, ubuntu:jammy-20251203
ARG LINUX_DISTRIBUTION=ubuntu:22.04
FROM ${LINUX_DISTRIBUTION}

# Overrides
ARG NODEJS_VERSION
# Create image for standard builds or those using ABFS.
ARG ABFS
# ABFS specific registry and category/versions.
ARG GOOGLE_DISTRIBUTION_REGISTRY
ARG ABFS_DISTRIBUTION_REGISTRY

# Gcloud CLI and kubectl version.
ARG GCLOUD_CLI_VERSION="latest"
ARG KUBECTL_VERSION="latest"
ARG PACKER_VERSION="1.15.0"

# Install gemini-cli if enabled.
ARG ENABLE_GEMINI_AI_ASSISTANT="false"
ARG GEMINI_CLI_VERSION="latest"
ARG ANTIGRAVITY_SDK_VERSION="0.1.16"

# Install packages required for building AAOSP targets.
# libsecret-1-0: Secret Service API (libsecret-1.so.0) for gemini-cli / credential helpers in-container.
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends apt-utils && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates \
    git git-core gnupg flex bison build-essential zip unzip zlib1g-dev \
    libc6-dev libxml2-utils xsltproc unzip fontconfig gcc-multilib \
    gettext gettext-base g++-multilib libncurses5 libsecret-1-0 libssl-dev python3 python3-setuptools \
    python3-pip python3-requests python3-venv python3-virtualenv procps \
    meson bc coreutils dosfstools e2fsprogs fdisk kpartx mtools ninja-build \
    pkg-config rsync less curl sudo vim ssh findutils gawk jq htop wget \
    libcap2-bin psmisc lsof && \
    apt-get update -y && apt install -y npm nodejs && \
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

# Install Packer for VM image baking via HashiCorp APT repo.
RUN curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(grep -oP '(?<=UBUNTU_CODENAME=).*' /etc/os-release || lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list && \
    apt-get update -y && \
    [ "$PACKER_VERSION" = "latest" ] && apt-get install -y packer || apt-get install -y "packer=${PACKER_VERSION}-1" && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/* && \
    packer version

# Install Google repo command tool
RUN export REPO=$(mktemp /tmp/repo.XXXXXXXXX)  && \
    curl -o ${REPO} https://storage.googleapis.com/git-repo-downloads/repo && \
    gpg --recv-key 8BB9AD793E8E6153AF0F9A4416530D5E920F5C65 && \
    curl -s https://storage.googleapis.com/git-repo-downloads/repo.asc | \
       gpg --verify - ${REPO} && install -m 755 ${REPO} /usr/bin/repo;

RUN if [ "$ABFS" = "true" ]; then \
        echo "deb ${GOOGLE_DISTRIBUTION_REGISTRY}" | tee -a /etc/apt/sources.list.d/artifact-registry.list && \
        curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
        apt update -y && apt install -y apt-transport-artifact-registry && \
        echo "deb ${ABFS_DISTRIBUTION_REGISTRY}" | tee -a /etc/apt/sources.list.d/artifact-registry.list && \
        apt update -y && apt install -y kmod; \
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
RUN git config --global user.email "aaosp.jenkins@dummy.com" && \
    git config --global user.name "AAOSP Jenkins" && \
    git config --global color.ui false

# Create virtualenv ready for RPi builds (include-system-site-packages=true)
# Install the required packages. pyyaml required for Gemini skills conversion.
RUN virtualenv --system-site-packages venv && \
    . ${HOME}/venv/bin/activate && pip3 install --user meson==1.3.0 mako \
    jinja2 ply pyyaml dataclasses && deactivate && \
    python3 -m pip install pyyaml

ENV PATH="/root/.local/bin:/usr/local/bin:/usr/bin:$PATH"

# Node version manager
RUN sudo npm cache clean -f && \
    sudo npm install -g n && \
    sudo n "${NODEJS_VERSION}" && \
    sudo ln -sf /usr/local/bin/node /usr/local/bin/nodejs || true && \
    if [ "$ENABLE_GEMINI_AI_ASSISTANT" = "true" ]; then \
      echo "\033[1;32mInstalling gemini-cli@${GEMINI_CLI_VERSION}\033[0m" && \
      sudo npm install -g @google/gemini-cli@${GEMINI_CLI_VERSION} && \
      sudo rm -f /usr/lib/python3*/EXTERNALLY-MANAGED && \
      echo "\033[1;32mInstalling google-antigravity@${ANTIGRAVITY_SDK_VERSION}\033[0m" && \
      sudo pip3 install "google-antigravity==${ANTIGRAVITY_SDK_VERSION}" && \
      sudo pip install "google-cloud-logging"; \
    fi
