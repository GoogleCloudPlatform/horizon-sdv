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

FROM debian:12-slim

# Install necessary packages for Terraform, gcloud CLI, and common utilities.
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates curl gnupg unzip git jq lsb-release sudo python3 vim openssh-client && \
    # Install Go version of yq
    curl -Lo /usr/local/bin/yq https://github.com/mikefarah/yq/releases/download/v4.48.1/yq_linux_amd64 && \
    chmod +x /usr/local/bin/yq && \
    # Install Google Cloud SDK (gcloud CLI) and kubectl
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | \
    gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | \
    tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && \
    apt-get update -y && \
    apt-get install -y google-cloud-cli kubectl && \
    # Install Terraform
    curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list && \
    apt-get update -y && \
    apt-get install -y terraform && \
    apt-get clean && \
    # Install repo tool with signature verification
    export REPO=$(mktemp /tmp/repo.XXXXXXXXX) && \
    curl -o ${REPO} https://storage.googleapis.com/git-repo-downloads/repo && \
    gpg --recv-key 8BB9AD793E8E6153AF0F9A4416530D5E920F5C65 && \
    curl -s https://storage.googleapis.com/git-repo-downloads/repo.asc | gpg --verify - ${REPO} && \
    install -m 755 ${REPO} /usr/bin/repo && \
    rm -rf /var/lib/apt/lists/*

# Add a non-root user for security best practices.
# GID (1000) of this user will be granted write access to mounted NFS volume via initContainer in the Pod spec.
ARG USER=horizon-sdv-aosp-mirror-ops
RUN useradd -u 1000 -ms /bin/bash ${USER} && \
    passwd -d ${USER} && \
    # SSH Configuration for Git/Repo
    mkdir -p /home/${USER}/.ssh && \
    chown -R 1000:1000 /home/${USER}/.ssh && \
    chmod 700 /home/${USER}/.ssh

# Switch to the non-root user and set working directory.
USER ${USER}
WORKDIR /home/${USER}

# Configure git config (fake details)
RUN git config --global user.email "horizon-sdv-aosp-mirror-ops@example.com" && \
    git config --global user.name "horizon-sdv-aosp-mirror-ops" && \
    git config --global color.ui false
