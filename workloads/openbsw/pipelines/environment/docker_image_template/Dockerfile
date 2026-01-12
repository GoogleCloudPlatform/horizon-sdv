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

# Allow user to choose distribution, e.g. ubuntu:22.04, ubuntu:jammy-20251203
ARG LINUX_DISTRIBUTION=22.04
FROM ${LINUX_DISTRIBUTION}

# Overrides
ARG ARM_TOOLCHAIN_URL
ARG CLANG_TOOLS_URL
ARG CMAKE_URL
ARG GCLOUD_CLI_VERSION="latest"
ARG KUBECTL_VERSION="latest"
ARG LLVM_ARM_TOOLCHAIN_URL
ARG LLVM_PROJECT_URL
ARG NODEJS_VERSION
ARG OPENBSW_GIT_URL
ARG OPENBSW_GIT_BRANCH
ARG PLANTUML_URL
ARG POST_GIT_CLONE_COMMAND
ARG PYTHON_VERSION
ARG PYELFTOOLS_VERSION
ARG SSCACHE_URL
ARG TREEFMT_URL

# Install packages required for building OpenBSW targets.
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends apt-utils && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates \
    build-essential bzip2 clang-tidy curl default-jdk-headless \
    doxygen file gawk gcc-11 g++11 gettext gettext-base git \
    git-core gnupg graphviz htop iproute2 iputils-ping kmod less \
    lcov ninja-build python"${PYTHON_VERSION}" python3-setuptools \
    python3-pip python3-requests python3-venv python3-virtualenv \
    rsync ruby-rubygems ssh sudo tar tmux unzip vim wget \
    xz-utils zip && \
    apt-get update -y && apt install -y npm && \
    rm -rf /var/lib/apt/lists/*

# Download and install OpenBSW dependent tools.
RUN echo "\033[1;32mInstall ${CMAKE_URL}\033[0m" \
    && curl -L "${CMAKE_URL}" -o cmake-linux-x86_64.sh \
    && chmod +x cmake-linux-x86_64.sh \
    && ./cmake-linux-x86_64.sh --skip-license --prefix=/usr/local \
    && rm cmake-linux-x86_64.sh \
    && echo "\033[1;32mInstall ${ARM_TOOLCHAIN_URL}\033[0m" \
    && curl -L "${ARM_TOOLCHAIN_URL}" -o arm-gnu-toolchain.tar.xz \
    && case $(file -b arm-gnu-toolchain.tar.xz) in \
       *"bzip2"*) tar -xjf arm-gnu-toolchain.tar.xz --strip-components=1 -C /usr/local ;; \
       *"gzip"*) tar -xvzf arm-gnu-toolchain.tar.xz --strip-components=1 -C /usr/local ;; \
       *"XZ"* ) xz -d arm-gnu-toolchain.tar.xz && tar xf arm-gnu-toolchain.tar --strip-components=1 -C /usr/local ;; \
       esac \
    && rm -f arm-gnu-toolchain.tar* \
    && ln -s /usr/local/gcc-arm-none-eabi-*/bin/* /usr/local/bin/  \
    && mkdir -p /root/.local/bin \
    && echo "\033[1;32mInstall ${LLVM_PROJECT_URL}\033[0m" \
    && curl -L "${LLVM_PROJECT_URL}" -o clang+llvm.tar.xz \
    && tar -xf clang+llvm.tar.xz \
    && mkdir -p /usr/bin/llvm \
    && mv clang+llvm-*-x86_64-linux-gnu-ubuntu* /usr/bin/llvm \
    && rm -f clang+llvm.tar.xz \
    && echo "\033[1;32mInstall ${LLVM_ARM_TOOLCHAIN_URL}\033[0m" \
    && curl -L "${LLVM_ARM_TOOLCHAIN_URL}" -o LLVM-ET-Arm.tar.xz \
    && tar -xf LLVM-ET-Arm.tar.xz \
    && mkdir -p /usr/bin/llvm-arm \
    && mv LLVM-ET-Arm-*-Linux-x86_64 /usr/bin/llvm-arm \
    && rm -f LLVM-ET-Arm.tar.xz \
    && echo "\033[1;32mInstall ${TREEFMT_URL}\033[0m" \
    && curl -L "${TREEFMT_URL}" -o treefmt.tar.gz \
    && tar -xvzf treefmt.tar.gz \
    && install -m 755 treefmt /usr/bin/treefmt\
    && rm LICENSE README.md treefmt treefmt.tar.gz \
    && echo "\033[1;32mInstall ${CLANG_TOOLS_URL}\033[0m" \
    && curl -L "${CLANG_TOOLS_URL}" -o usr/bin/clang-format-17 \
    && chmod +x usr/bin/clang-format-17 \
    && echo "\033[1;32mInstall ${SSCACHE_URL}\033[0m" \
    && curl -L "${SSCACHE_URL}" -o sccache.tar.gz \
    && tar -zxf sccache.tar.gz \
    && mkdir -p /usr/local/bin/sccache \
    && mv sccache-*-x86_64-unknown-linux-musl/sccache /usr/local/bin/sccache \
    && rm -f sccache.tar.gz \
    && chmod a+x /usr/local/bin/sccache \
    && rm -rf sccache-*-x86_64-unknown-linux-musl \
    && echo "\033[1;32mInstall ${CPLANTUML_URL}\033[0m" \
    && curl -L "${PLANTUML_URL}" -o /usr/local/bin/plantuml.jar \
    && echo '#!/bin/sh\njava -jar /usr/local/bin/plantuml.jar "$@"' > /usr/local/bin/plantuml \
    && chmod +x /usr/local/bin/plantuml \
    && echo 'export HISTFILE=/root/.bash_history/history' >> /root/.bashrc

RUN gem install esr-rim

ENV PATH="/usr/bin/gcc-arm-none-eabi/bin:/usr/bin/llvm/bin:/usr/bin/llvm-arm/bin:${PATH}"

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
    git config --global color.ui false

# Paths for pyTest
ENV PATH="/home/${USER}/.local/bin:/usr/local/bin:/usr/bin:${PATH}"
RUN echo "PATH=$PATH" >> /home/${USER}/.bash_aliases

# Configure python packages together with OpenBSW pyTest and UdsTool
RUN echo "\033[1;32mInstall python packages\033[0m" \
    && pip3 install cmakelang coverxygen pytest pyelftools=="${PYELFTOOLS_VERSION}" rich \
    && git clone "${OPENBSW_GIT_URL}" -b "${OPENBSW_GIT_BRANCH}" openbsw \
    && cd openbsw \
    && if [ ! -z "${POST_GIT_CLONE_COMMAND}" ]; then eval "${POST_GIT_CLONE_COMMAND}"; fi \
    && echo "\033[1;32mInstall pyTest\033[0m" \
    && pip3 install -r test/pyTest/requirements.txt \
    && echo "\033[1;32mInstall UdsTool\033[0m" \
    && cd tools/UdsTool \
    && pip install . \
    && cd ../../../ \
    && rm -rf openbsw

# Node version manager
RUN echo "\033[1;32mInstall nodejs ${NODEJS_VERSION}\033[0m" && \
    sudo npm cache clean -f && \
    sudo npm install -g n && \
    sudo n "${NODEJS_VERSION}" && \
    sudo npm install -g wait-on && \
    sudo ln -sf /usr/local/bin/node /usr/local/bin/nodejs || true
