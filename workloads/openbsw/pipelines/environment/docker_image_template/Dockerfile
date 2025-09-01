# Copyright (c) 2025 Accenture, All Rights Reserved.
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

# Allow user to choose distribution, e.g. ubuntu:22.04
ARG LINUX_DISTRIBUTION
FROM --platform=linux/amd64 ${LINUX_DISTRIBUTION}

# Overrides
ARG ARM_TOOLCHAIN_URL
ARG CLANG_TOOLS_URL
ARG CMAKE_URL
ARG TREEFMT_URL
ARG NODEJS_VERSION


# Install packages required for building OpenBSW targets.
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends apt-utils && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates \
    clang-tidy gcc-11 g++11 build-essential file \
    gnupg git git-core lcov  bzip2 zip unzip tar xz-utils \
    python3 python3-setuptools python3-pip python3-requests \
    python3-venv python3-virtualenv gettext gettext-base \
    rsync less curl wget sudo vim ssh gawk htop && \
    apt-get update -y && apt install -y npm && \
    rm -rf /var/lib/apt/lists/*

# Download and install OpenBSW dependent tools.
RUN curl -L "${CMAKE_URL}" -o cmake-linux-x86_64.sh \
    && chmod +x cmake-linux-x86_64.sh \
    && ./cmake-linux-x86_64.sh --skip-license --prefix=/usr/local \
    && rm cmake-linux-x86_64.sh \
    && curl -L "${ARM_TOOLCHAIN_URL}" -o arm-gnu-toolchain.tar.xz \
    && case $(file -b arm-gnu-toolchain.tar.xz) in \
       *"bzip2"*) tar -xjf arm-gnu-toolchain.tar.xz --strip-components=1 -C /usr/local ;; \
       *"gzip"*) tar -xvzf arm-gnu-toolchain.tar.xz --strip-components=1 -C /usr/local ;; \
       *"XZ"* ) xz -d arm-gnu-toolchain.tar.xz && tar xf arm-gnu-toolchain.tar --strip-components=1 -C /usr/local ;; \
       esac \
    && rm -f arm-gnu-toolchain.tar* \
    && ln -s /usr/local/gcc-arm-none-eabi-*/bin/* /usr/local/bin/  \
    && mkdir -p /root/.local/bin \
    && curl -L "${TREEFMT_URL}" -o treefmt.tar.gz \
    && tar -xvzf treefmt.tar.gz \
    && install -m 755 treefmt /root/.local/bin/treefmt \
    && rm LICENSE README.md treefmt treefmt.tar.gz \
    && curl -L "${CLANG_TOOLS_URL}" -o /root/.local/bin/clang-format \
    && chmod +x /root/.local/bin/clang-format \
    && echo 'export HISTFILE=/root/.bash_history/history' >> /root/.bashrc

ENV PATH="/root/.local/bin:/usr/bin:$PATH"

# The following installs gcloud sdk required for uploading
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

# Add USER and provide sudo access.
ARG USER=builder
RUN useradd -u 1000 -ms /bin/bash ${USER} && \
    passwd -d ${USER} && \
    usermod -aG sudo ${USER} && \
    echo "${USER} ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers

# Switch to user
USER ${USER}
WORKDIR /home/${USER}

# Configure git config (fake details)
RUN git config --global user.email "bsw.jenkins@dummy.com" && \
    git config --global user.name "BSW Jenkins" && \
    git config --global color.ui false && \
    virtualenv --system-site-packages venv && \
    . ${HOME}/venv/bin/activate && pip3 install --user cmakelang && deactivate

ENV PATH="/root/.local/bin:/usr/local/bin:/usr/bin:$PATH"

# Node version manager
RUN sudo npm cache clean -f && \
    sudo npm install -g n && \
    sudo npm install -g wait-on && \
    sudo n "${NODEJS_VERSION}" && \
    sudo ln -sf /usr/local/bin/node /usr/local/bin/nodejs || true
