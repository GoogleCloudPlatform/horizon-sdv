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

# Description:
# Dockerfile for building a container image that can be used to build
# the Eclipse Foundation OpenBSW project. This image includes all the
# necessary tools and dependencies required for building OpenBSW targets,
# such as the ARM toolchain, CMake, Clang tools, and other utilities.

# Allow user to choose distribution, e.g. ubuntu:22.04, ubuntu:jammy-20251203
ARG LINUX_DISTRIBUTION=22.04
FROM ${LINUX_DISTRIBUTION}

# Overrides
ARG ARM_TOOLCHAIN_URL
ARG GCLOUD_CLI_VERSION="latest"
ARG KUBECTL_VERSION="latest"
ARG LLVM_ARM_TOOLCHAIN_URL
ARG NODEJS_VERSION
ARG OPENBSW_GIT_URL
ARG OPENBSW_GIT_BRANCH
ARG PLANTUML_URL
ARG POST_GIT_CLONE_COMMAND
ARG PYTHON_VERSION
ARG TREEFMT_URL

# Install gemini-cli if enabled.
ARG ENABLE_GEMINI_AI_ASSISTANT="false"
ARG GEMINI_CLI_VERSION="latest"
ARG ANTIGRAVITY_SDK_VERSION="0.1.16"

# Rust (posix-rust, s32k148-rust-gcc): defaults align with eclipse-openbsw/openbsw docker/development/Dockerfile
ARG RUST_DEFAULT_TOOLCHAIN=1.90.0
ARG RUSTUP_PROFILE=minimal
ARG RUST_EMBEDDED_TARGET=thumbv7em-none-eabihf
ARG CBINDGEN_VERSION=0.27.0

# Install packages required for building OpenBSW targets.
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends apt-utils && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates \
    build-essential bzip2 clang-17 clang-format-17 clang-tidy-17 \
    cmake curl default-jdk-headless \
    doxygen file gawk gcc-11 g++11 gettext gettext-base git \
    git-core gnupg graphviz htop iproute2 iputils-ping jq kmod less \
    lcov ninja-build python"${PYTHON_VERSION}" python3-full \
    python3-setuptools python3-pip python3-requests python3-venv \
    python3-virtualenv procps rsync ruby-rubygems ssh sudo tar tmux unzip \
    vim wget xz-utils zip && \
    apt-get update -y && apt install -y npm && \
    rm -rf /var/lib/apt/lists/*

# Download and install OpenBSW dependent tools.
RUN echo "\033[1;32mInstall ${ARM_TOOLCHAIN_URL}\033[0m" \
    && curl -L "${ARM_TOOLCHAIN_URL}" -o arm-gnu-toolchain.tar.xz \
    && mkdir -p /opt/arm-gnu-toolchain \
    && case $(file -b arm-gnu-toolchain.tar.xz) in \
       *"bzip2"*) tar -xjf arm-gnu-toolchain.tar.xz --strip-components=1 -C /opt/arm-gnu-toolchain ;; \
       *"gzip"*) tar -xvzf arm-gnu-toolchain.tar.xz --strip-components=1 -C /opt/arm-gnu-toolchain ;; \
       *"XZ"* ) xz -d arm-gnu-toolchain.tar.xz && tar xf arm-gnu-toolchain.tar --strip-components=1 -C /opt/arm-gnu-toolchain ;; \
       esac \
    && rm -f arm-gnu-toolchain.tar* \
    && echo "\033[1;32mInstall ${LLVM_ARM_TOOLCHAIN_URL}\033[0m" \
    && curl -L "${LLVM_ARM_TOOLCHAIN_URL}" -o LLVM-ET-Arm.tar.xz \
    && mkdir -p /opt/llvm-et-arm \
    && tar -xf LLVM-ET-Arm.tar.xz -C /opt/llvm-et-arm --strip-components 1 \
    && rm -f LLVM-ET-Arm.tar.xz \
    && echo "\033[1;32mInstall ${TREEFMT_URL}\033[0m" \
    && curl -L "${TREEFMT_URL}" -o treefmt.tar.gz \
    && tar -xvzf treefmt.tar.gz \
    && install -m 755 treefmt /usr/bin/treefmt\
    && rm LICENSE README.md treefmt treefmt.tar.gz \
    && echo "\033[1;32mInstall ${PLANTUML_URL}\033[0m" \
    && curl -L "${PLANTUML_URL}" -o /usr/local/bin/plantuml.jar \
    && echo '#!/bin/sh\njava -jar /usr/local/bin/plantuml.jar "$@"' > /usr/local/bin/plantuml \
    && chmod +x /usr/local/bin/plantuml \
    && echo 'export HISTFILE=/root/.bash_history/history' >> /root/.bashrc

RUN gem install esr-rim

ENV RUSTUP_HOME=/opt/rustup
ENV CARGO_HOME=/opt/cargo
ENV PATH="/opt/cargo/bin:/opt/arm-gnu-toolchain/bin:/opt/llvm-et-arm/bin:${PATH}"
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y \
    --default-toolchain "${RUST_DEFAULT_TOOLCHAIN}" \
    --profile "${RUSTUP_PROFILE}" \
    && rustup target add "${RUST_EMBEDDED_TARGET}" \
    && cargo install cbindgen --version "${CBINDGEN_VERSION}" --locked

# The following installs gcloud sdk required for uploading
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

# Add USER and provide sudo access.
ARG USER=builder
RUN userdel $(awk -F: '$3==1000{print $1}' /etc/passwd) || true && \
    useradd -u 1000 -ms /bin/bash ${USER} && \
    passwd -d ${USER} && \
    usermod -aG sudo ${USER} && \
    echo "${USER} ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers

# Switch to user
USER ${USER}
WORKDIR /home/${USER}

# Paths
ENV PATH="/home/${USER}/.local/bin:/home/${USER}/bin:/home/${USER}/venv/bin:/usr/local/bin:/usr/bin:/opt/cargo/bin:/opt/arm-gnu-toolchain/bin:/opt/llvm-et-arm/bin:${PATH}"
RUN echo "PATH=$PATH" >> /home/${USER}/.bash_aliases

# Configure git config (fake details)
RUN git config --global user.email "bsw.jenkins@dummy.com" && \
    git config --global user.name "BSW Jenkins" && \
    git config --global color.ui false

# Configure python packages together with OpenBSW pyTest and UdsTool. pyyaml for Gemini skills conversion.
RUN echo "\033[1;32mInstall python packages\033[0m" \
    && python3 -m venv /home/${USER}/venv \
    && pip3 install coverxygen pyyaml \
    && git clone "${OPENBSW_GIT_URL}" -b "${OPENBSW_GIT_BRANCH}" openbsw \
    && cd openbsw \
    && if [ ! -z "${POST_GIT_CLONE_COMMAND}" ]; then eval "${POST_GIT_CLONE_COMMAND}"; fi \
    && echo "\033[1;32mInstall python files\033[0m" \
    && pip3 install -r test/pyTest/requirements.txt \
    && cd tools/UdsTool/ \
    && pip install -r requirements.txt \
    && cd ../safety/ \
    && pip install -r requirements.txt \
    && cd ../../docker/development \
    && pip3 install -r files/requirements.lock \
    && cd ../../doc \
    && pip3 install -r dev/requirements.txt \
    && cd ../../ \
    && rm -rf openbsw \
    && echo "\033[1;32mReinstall poetry\033[0m" \
    && pip install --upgrade packaging \
    && pip3 uninstall poetry -y \
    && pip3 install pipx \
    && pipx install poetry \
    && pipx ensurepath

# Node version manager
RUN echo "\033[1;32mInstall nodejs ${NODEJS_VERSION}\033[0m" && \
    sudo npm cache clean -f && \
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
