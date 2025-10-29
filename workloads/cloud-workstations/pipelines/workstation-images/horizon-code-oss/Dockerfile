# Copyright (c) 2024-2025 Accenture and Google LLC, All Rights Reserved.
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

FROM us-central1-docker.pkg.dev/cloud-workstations-images/predefined/code-oss:latest

ARG ADDITIONAL_PACKAGES=""
ARG INSTALL_ABFS="false"

RUN apt-get update -y && \
    apt-get install -y --no-install-recommends \
      curl \
      gnupg && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN gpg_file=/usr/share/keyrings/helm.gpg && \
    curl -sSL https://packages.buildkite.com/helm-linux/helm-debian/gpgkey \
      | gpg --dearmor -o $gpg_file && \
    echo "deb [signed-by=$gpg_file] \
      https://packages.buildkite.com/helm-linux/helm-debian/any/ any main" \
      > /etc/apt/sources.list.d/helm-stable-debian.list && \
    apt-get update -y && \
    if [ -n "${ADDITIONAL_PACKAGES}" ]; then \
      apt-get install -y --no-install-recommends ${ADDITIONAL_PACKAGES} ; \
    fi && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN if [ "${INSTALL_ABFS}" = "true" ]; then \
      gpg_file=/usr/share/keyrings/cloud.google.gpg && \
      curl -sSL https://packages.cloud.google.com/apt/doc/apt-key.gpg \
        | gpg --dearmor -o $gpg_file && \
      echo "deb [signed-by=$gpg_file] https://packages.cloud.google.com/apt apt-transport-artifact-registry-stable main" \
        > /etc/apt/sources.list.d/artifact-registry.list && \
      apt-get update -y && \
      apt-get install -y --no-install-recommends apt-transport-artifact-registry && \
      apt-get clean && \
      rm -rf /var/lib/apt/lists/* ; \
    fi

RUN if [ "${INSTALL_ABFS}" = "true" ]; then \
      gpg_file=/usr/share/keyrings/cloud.google.gpg && \
      echo "deb [signed-by=$gpg_file] ar+https://us-apt.pkg.dev/projects/abfs-binaries abfs-apt-alpha-public main" \
        > /etc/apt/sources.list.d/abfs.list && \
      apt-get update -y && \
      apt-get install -y --no-install-recommends \
        abfs-client \
        git \
        psmisc \
        rsync \
        unzip \
        zip && \
      apt-get clean && \
      rm -rf /var/lib/apt/lists/* ; \
    fi

COPY ./assets/. /
