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

# Allow user to choose distribution, e.g. ubuntu:22.04
ARG LINUX_DISTRIBUTION
FROM --platform=linux/amd64 ${LINUX_DISTRIBUTION}

# Overrides
ARG NODEJS_VERSION
# Create image for standard builds or those using ABFS.
ARG ABFS
# ABFS specific registry and category/versions.
ARG GOOGLE_DISTRIBUTION_REGISTRY
ARG ABFS_DISTRIBUTION_REGISTRY

# Install packages required for building AAOSP targets.
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends apt-utils && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates \
    git git-core gnupg flex bison build-essential zip unzip zlib1g-dev \
    libc6-dev libxml2-utils xsltproc unzip fontconfig gcc-multilib \
    gettext gettext-base g++-multilib libncurses5 libssl-dev python3 python3-setuptools \
    python3-pip python3-requests python3-venv python3-virtualenv procps \
    meson bc coreutils dosfstools e2fsprogs fdisk kpartx mtools ninja-build \
    pkg-config rsync less curl sudo vim ssh findutils gawk jq htop \
    libcap2-bin psmisc && \
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
    apt-get update -y && apt-get install -y google-cloud-cli kubectl && \
    apt-get -y clean
# GCS:END

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

RUN if [ "$ABFS" = "true" ]; then \
        mkdir /src && chown ${USER}:${USER} /src; \
    fi

# Switch to user
USER ${USER}
WORKDIR /home/${USER}

# Configure git config (fake details) for Google repo.
RUN git config --global user.email "aaosp.jenkins@dummy.com" && \
    git config --global user.name "AAOSP Jenkins" && \
    git config --global color.ui false

# Create virtualenv ready for RPi builds (include-system-site-packages=true)
# Install the required packages.
RUN virtualenv --system-site-packages venv && \
    . ${HOME}/venv/bin/activate && pip3 install --user meson==1.3.0 mako \
    jinja2 ply pyyaml dataclasses && deactivate

ENV PATH="/root/.local/bin:/usr/local/bin:/usr/bin:$PATH"

# Node version manager
RUN sudo npm cache clean -f && \
    sudo npm install -g n && \
    sudo npm install -g wait-on && \
    sudo n "${NODEJS_VERSION}" && \
    sudo ln -sf /usr/local/bin/node /usr/local/bin/nodejs || true
