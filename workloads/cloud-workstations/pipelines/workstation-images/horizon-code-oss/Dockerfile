# Copyright 2025-2026 Google LLC
#
# Modifications Copyright (c) 2026 Accenture, All Rights Reserved.
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

FROM us-central1-docker.pkg.dev/cloud-workstations-images/predefined/code-oss:code-oss-1.104.3

ARG ADDITIONAL_PACKAGES=""
ARG INSTALL_ABFS="false"
ARG HORIZON_DOMAIN
ARG GOOGLE_CLOUD_PROJECT
ARG NODE_MAJOR=20
ARG ANTIGRAVITY_CLI_VERSION=1.1.0
ARG ANTIGRAVITY_CLI_TARBALL_URL=https://github.com/google-antigravity/antigravity-cli/releases/download/1.1.0/agy_cli_linux_x64.tar.gz
ARG ANTIGRAVITY_CLI_SHA256=7ee512440af5ed0c819065cd7cc14eec90699214df4be32280ac346f0100577e

ENV GOOGLE_CLOUD_PROJECT="${GOOGLE_CLOUD_PROJECT}"
# Shared MCP Gateway core + Gemini and Antigravity MCP agents.
# Stable paths via ENV so bridge entries in client configs keep resolving.
ENV HORIZON_MCP_LIB="/usr/local/lib/horizon-mcp" \
    GEMINI_MCP_AGENT_PATH="/usr/local/bin/gemini-mcp-agent" \
    ANTIGRAVITY_MCP_AGENT_PATH="/usr/local/bin/antigravity-mcp-agent"

# Base tools, Yarn key fix, Node.js 20, Gemini CLI
RUN mkdir -p /etc/apt/keyrings && \
    curl -fsSL https://dl.yarnpkg.com/debian/pubkey.gpg -o /etc/apt/keyrings/yarn.asc && \
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | \
      gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_${NODE_MAJOR}.x nodistro main" \
      > /etc/apt/sources.list.d/nodesource.list && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      gnupg \
      fonts-dejavu-core \
      python3-pip \
      python3-requests \
      python3-dotenv \
      nodejs && \
    npm install -g @google/gemini-cli && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Helm repo and optional additional packages
RUN gpg_file=/usr/share/keyrings/helm.gpg && \
    curl -sSL https://packages.buildkite.com/helm-linux/helm-debian/gpgkey \
      | gpg --dearmor -o "${gpg_file}" && \
    echo "deb [signed-by=${gpg_file}] https://packages.buildkite.com/helm-linux/helm-debian/any/ any main" \
      > /etc/apt/sources.list.d/helm-stable-debian.list && \
    apt-get update -y && \
    if [ -n "${ADDITIONAL_PACKAGES}" ]; then \
      apt-get install -y --no-install-recommends ${ADDITIONAL_PACKAGES} ; \
    fi && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Optional ABFS support (Google Artifact Registry + ABFS repo & tools)
RUN if [ "${INSTALL_ABFS}" = "true" ]; then \
      gpg_file=/usr/share/keyrings/cloud.google.gpg && \
      curl -sSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | \
        gpg --dearmor -o "${gpg_file}" && \
      echo "deb [signed-by=${gpg_file}] https://packages.cloud.google.com/apt apt-transport-artifact-registry-stable main" \
        > /etc/apt/sources.list.d/artifact-registry.list && \
      echo "deb [signed-by=${gpg_file}] ar+https://us-apt.pkg.dev/projects/abfs-binaries abfs-apt-alpha-public main" \
        > /etc/apt/sources.list.d/abfs.list && \
      apt-get update -y && \
      apt-get install -y --no-install-recommends \
        apt-transport-artifact-registry \
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

# Configure mcp-agent scripts
COPY --chmod=644 ./common-utils/mcp-agents/mcp_registry_client.py /usr/local/lib/horizon-mcp/mcp_registry_client.py
COPY --chmod=755 ./common-utils/mcp-agents/gemini_mcp_agent.py /usr/local/bin/gemini-mcp-agent
COPY --chmod=755 ./common-utils/mcp-agents/antigravity_mcp_agent.py /usr/local/bin/antigravity-mcp-agent
RUN sed -i 's/^\(HORIZON_DOMAIN = os.environ.get("HORIZON_DOMAIN", "\)##DOMAIN##\(")\)/\1'"${HORIZON_DOMAIN}"'\2/' /usr/local/lib/horizon-mcp/mcp_registry_client.py \
    && chmod 755 /usr/local/lib/horizon-mcp

# Install Antigravity CLI (agy)
COPY --chmod=755 ./common-utils/antigravity/install-antigravity-cli.sh /tmp/install-antigravity-cli.sh
RUN ANTIGRAVITY_CLI_VERSION="${ANTIGRAVITY_CLI_VERSION}" \
    ANTIGRAVITY_CLI_TARBALL_URL="${ANTIGRAVITY_CLI_TARBALL_URL}" \
    ANTIGRAVITY_CLI_SHA256="${ANTIGRAVITY_CLI_SHA256}" \
    /tmp/install-antigravity-cli.sh && \
    agy --version && \
    rm -f /tmp/install-antigravity-cli.sh