# syntax=docker/dockerfile:1.4

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

# Horizon GNOME base image.
# Bootable GNOME + gnome-remote-desktop (RDP) + Apache Guacamole (DinD) desktop,
# consuming horizon-preflight via BASE_IMAGE (COPY --from=preflight-provider /export/ /).
# Upstream reference: sce-taid/cloud-workstations-custom-image-examples examples/images/gnome

ARG GCP_REGION=us-central1
ARG CWS_BASE_IMAGE_TAG=latest

ARG ALPINE_VERSION=3.19

# Published horizon-preflight image URI (Alpine /export/ provider).
ARG BASE_IMAGE

###############################################################
# Stage: asset-prep (Ensure correct permissions on assets)
###############################################################
FROM alpine:${ALPINE_VERSION} AS asset-prep
COPY assets/ /assets/
RUN find /assets -type d -exec chmod 755 {} + && \
    find /assets/etc -type f -exec chmod 644 {} +

###############################################################
# Stage: preflight-provider (Transferred from preflight)
###############################################################
FROM ${BASE_IMAGE} AS preflight-provider

###############################################################
# Stage: fetcher (Download container images and packages)
###############################################################
FROM alpine:${ALPINE_VERSION} AS fetcher

# Re-declare ARGs needed in this stage
ARG CURL_OPTS="-fsSL --retry 3 --connect-timeout 10 --max-time 300"
ARG GUACAMOLE_VERSION=1.6.0
ARG GUACAMOLE_EXTENSIONS=""
ARG GUACAMOLE_IMAGES="guacd guacamole"
ARG RETRIES=3
ARG RETRY_WAIT=5
ARG CONTAINER_REGISTRY="mirror.gcr.io"
ARG CRANE_TIMEOUT=300
ARG CRANE_VERSION=v0.21.5
ARG CRANE_URL="https://github.com/google/go-containerregistry/releases/download/${CRANE_VERSION}/go-containerregistry_Linux_x86_64.tar.gz"
ARG FETCHER_PKGS="bash ca-certificates curl tar"
ARG GUACAMOLE_BASE_URL="https://archive.apache.org/dist/guacamole/${GUACAMOLE_VERSION}/binary"

# Map to ENVs for the script
ENV CURL_OPTS=${CURL_OPTS}
ENV GUACAMOLE_VERSION=${GUACAMOLE_VERSION}
ENV GUACAMOLE_EXTENSIONS=${GUACAMOLE_EXTENSIONS}
ENV GUACAMOLE_IMAGES=${GUACAMOLE_IMAGES}
ENV RETRIES=${RETRIES}
ENV RETRY_WAIT=${RETRY_WAIT}
ENV CONTAINER_REGISTRY="${CONTAINER_REGISTRY}"
ENV CRANE_TIMEOUT=${CRANE_TIMEOUT}
ENV CRANE_URL=${CRANE_URL}
ENV GUACAMOLE_BASE_URL=${GUACAMOLE_BASE_URL}

WORKDIR /downloads

# Install dependencies
RUN apk add --no-cache ${FETCHER_PKGS}

# Copy the fetch script into the fetcher stage
COPY scripts/fetch_assets.sh /usr/local/bin/fetch_assets.sh

RUN chmod +x /usr/local/bin/fetch_assets.sh && /usr/local/bin/fetch_assets.sh

###############################################################
# Stage: Main System
###############################################################
FROM ${GCP_REGION}-docker.pkg.dev/cloud-workstations-images/predefined/base:${CWS_BASE_IMAGE_TAG}
# Re-declare global ARGs to make them available in this stage
ARG DEBIAN_FRONTEND=noninteractive
ARG GCP_REGION
ARG CWS_BASE_IMAGE_TAG
ARG INSTALL_TIGERVNC=false
ARG INSTALL_GEMINI_CLI=true
ARG INSTALL_AGENT_DEVELOPMENT_KIT_PYTHON=true
ARG INSTALL_CHROME_REMOTE_DESKTOP=false
ARG INSTALL_ANTIGRAVITY_AGENT=true
ARG ANTIGRAVITY_2_VERSION=2.2.1
ARG ANTIGRAVITY_2_INSTALL_MODE=tarball
ARG ANTIGRAVITY_2_TARBALL_URL=https://storage.googleapis.com/antigravity-public/antigravity-hub/2.2.1-5287492581195776/linux-x64/Antigravity.tar.gz
ARG ANTIGRAVITY_2_TARBALL_SHA256=a6ba77046f92aa1ce21d8a0c67495471af4c2be11ba624e5d935821969b20989
ARG GUACAMOLE_VERSION=1.6.0
# Keep non-empty so rendered guac*.service refs are registry/guacamole/... not /guacamole/...
ARG CONTAINER_REGISTRY="mirror.gcr.io"
ARG DEFAULT_ENABLE_AUDIO_INPUT=false
ARG GNOME_SHELL_EXTENSIONS="just-perfection-desktop@just-perfection:36"

ARG CURL_OPTS="-fsSL --retry 3 --connect-timeout 10 --max-time 300"
ARG DPKG_OPTS="-o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold"
ARG APT_OPTS="-y --no-install-recommends"

ARG BASE_PKGS="apt-transport-artifact-registry curl dbus-user-session gettext-base gnome-keyring gnome-remote-desktop gnupg libsecret-tools nginx openssh-server systemd unzip winpr3-utils"
ARG GUI_PKGS="fonts-dejavu fonts-liberation fonts-ubuntu gir1.2-dbusmenu-glib-0.4 gir1.2-dbusmenu-gtk3-0.4 gnome-control-center gvfs-backends ubuntu-desktop-minimal"
ARG CHROME_PKGS="google-chrome-stable"
ARG ADK_PKGS="pipx python3-venv"
# Horizon gemini-mcp-agent runtime deps (Ubuntu Noble packages).
ARG HORIZON_PKGS="python3-requests python3-dotenv"
ARG TIGERVNC_PKGS="tigervnc-common tigervnc-scraping-server tigervnc-standalone-server tigervnc-xorg-extension"
ARG CRD_PKGS="chrome-remote-desktop"
# Pinned Node.js for Gemini CLI (see assets/etc/apt/sources.list.d/nodesource.list).
ARG NODE_PKGS="nodejs"
ARG PURGE_PKGS="apache2* php*"
ARG HOLD_PKGS="apache2 apache2-bin apache2-data apache2-utils php*"

# Early copy of APT configuration (keys, sources) for the install block.
COPY --from=asset-prep /assets/etc/apt/sources.list.d/* /etc/apt/sources.list.d/
COPY --from=asset-prep /assets/etc/apt/keyrings/* /etc/apt/keyrings/

# Install and configure core system components (Systemd, GNOME, Chrome, etc.)
RUN INSTALL_PKGS="$BASE_PKGS $GUI_PKGS $CHROME_PKGS $HORIZON_PKGS"; \
    if [ "$INSTALL_TIGERVNC" = "true" ]; then \
      INSTALL_PKGS="$INSTALL_PKGS $TIGERVNC_PKGS"; \
    fi; \
    if [ "$INSTALL_CHROME_REMOTE_DESKTOP" = "true" ]; then \
      INSTALL_PKGS="$INSTALL_PKGS $CRD_PKGS"; \
    fi; \
    if [ "$INSTALL_AGENT_DEVELOPMENT_KIT_PYTHON" = "true" ]; then \
      INSTALL_PKGS="$INSTALL_PKGS $ADK_PKGS"; \
    fi; \
    if [ "$INSTALL_GEMINI_CLI" = "true" ]; then \
      INSTALL_PKGS="$INSTALL_PKGS $NODE_PKGS"; \
    fi; \
    apt-get update && \
    apt-get upgrade -y && \
    apt-get purge -y ${PURGE_PKGS} && \
    apt-mark hold ${HOLD_PKGS} && \
    apt-get install ${APT_OPTS} ${DPKG_OPTS} $INSTALL_PKGS && \
    dpkg-divert --add --rename --divert /usr/bin/google-chrome-stable.real /usr/bin/google-chrome-stable && \
    apt-get remove -y gnome-initial-setup && \
    apt-get remove -y --purge cloud-init && \
    rm -f /etc/xdg/autostart/update-notifier.desktop && \
    rm -rf /usr/lib/update-notifier/ && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Merge remaining assets into the container.
COPY --from=asset-prep /assets/ /

# Pull in modular preflight assets and logic.
RUN rm -rf /var/www/html/*
COPY --from=preflight-provider /export/ /

# Copy over the downloads from the fetcher.
COPY --from=fetcher /downloads/ /

# Bake build-time defaults into the environment for the hooks and entrypoint.
ARG HORIZON_DOMAIN
ARG GOOGLE_CLOUD_PROJECT
ENV GCP_REGION="${GCP_REGION}"
ENV CWS_BASE_IMAGE_TAG="${CWS_BASE_IMAGE_TAG}"
ENV GNOME_SHELL_EXTENSIONS="${GNOME_SHELL_EXTENSIONS}"
ENV CONTAINER_REGISTRY="${CONTAINER_REGISTRY}"
ENV GUACAMOLE_VERSION="${GUACAMOLE_VERSION}"
ENV INSTALL_GEMINI_CLI="${INSTALL_GEMINI_CLI}"
ENV INSTALL_AGENT_DEVELOPMENT_KIT_PYTHON="${INSTALL_AGENT_DEVELOPMENT_KIT_PYTHON}"
ENV INSTALL_ANTIGRAVITY_AGENT="${INSTALL_ANTIGRAVITY_AGENT}"
ENV ENABLE_TIGERVNC=${INSTALL_TIGERVNC}
ENV ENABLE_CRD=${INSTALL_CHROME_REMOTE_DESKTOP}
ENV DEFAULT_ENABLE_AUDIO_INPUT=${DEFAULT_ENABLE_AUDIO_INPUT}
ENV DEFAULT_PROTOCOL="RDP"
ENV GOOGLE_CLOUD_PROJECT="${GOOGLE_CLOUD_PROJECT}"
ENV HORIZON_MCP_LIB="/usr/local/lib/horizon-mcp" \
    GEMINI_MCP_AGENT_PATH="/usr/local/bin/gemini-mcp-agent" \
    ANTIGRAVITY_MCP_AGENT_PATH="/usr/local/bin/antigravity-mcp-agent"

# Perform centralized workstation configuration (hooks, packages, and integration).
RUN /google/scripts/build/configure_workstation.sh && \
    test -x /usr/local/bin/google-chrome-stable && \
    test -x /usr/bin/google-chrome-stable && \
    test -x /usr/bin/google-chrome-stable.real

# Horizon customizations inherited by all child IDE images.
# python3-requests / python3-dotenv are installed above via HORIZON_PKGS.
# Configure mcp-agent scripts
COPY --chmod=644 ./common-utils/mcp-agents/mcp_registry_client.py /usr/local/lib/horizon-mcp/mcp_registry_client.py
COPY --chmod=755 ./common-utils/mcp-agents/gemini_mcp_agent.py /usr/local/bin/gemini-mcp-agent
COPY --chmod=755 ./common-utils/mcp-agents/antigravity_mcp_agent.py /usr/local/bin/antigravity-mcp-agent
RUN sed -i 's/^\(HORIZON_DOMAIN = os.environ.get("HORIZON_DOMAIN", "\)##DOMAIN##\(")\)/\1'"${HORIZON_DOMAIN}"'\2/' /usr/local/lib/horizon-mcp/mcp_registry_client.py \
    && chmod 755 /usr/local/lib/horizon-mcp

# Antigravity 2.0 Agent (desktop app, not IDE) - inherited by GNOME thin-children
# (Android Studio, ASfP). GNOME/RDP provide XWayland for the GPU-safe X11 wrapper.
COPY --chmod=755 ./common-utils/antigravity/install-antigravity-agent.sh /tmp/install-antigravity-agent.sh
COPY ./common-utils/antigravity/icon/antigravity.png /tmp/icon/antigravity.png
RUN if [ "$INSTALL_ANTIGRAVITY_AGENT" = "true" ]; then \
      ANTIGRAVITY_2_INSTALL_MODE="${ANTIGRAVITY_2_INSTALL_MODE}" \
      ANTIGRAVITY_2_VERSION="${ANTIGRAVITY_2_VERSION}" \
      ANTIGRAVITY_2_TARBALL_URL="${ANTIGRAVITY_2_TARBALL_URL}" \
      ANTIGRAVITY_2_TARBALL_SHA256="${ANTIGRAVITY_2_TARBALL_SHA256}" \
      /tmp/install-antigravity-agent.sh && \
      test -x /usr/local/bin/antigravity && \
      test -f /usr/share/applications/antigravity.desktop && \
      ! test -f /etc/xdg/autostart/antigravity.desktop; \
    fi && \
    rm -rf /tmp/install-antigravity-agent.sh /tmp/icon

# Entrypoint hands over execution to start_systemd.sh for final initialization.
ENTRYPOINT ["/google/scripts/entrypoint.sh"]
