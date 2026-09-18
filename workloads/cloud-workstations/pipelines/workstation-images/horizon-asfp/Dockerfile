# syntax=docker/dockerfile:1.4

# Copyright 2024-2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ARG GCP_REGION=us-central1

# Published horizon-gnome image URI
ARG BASE_IMAGE

###############################################################
# Stage: Base Provider (to get common.sh)
###############################################################
FROM ${BASE_IMAGE} AS base-provider

###############################################################
# Stage: Cuttlefish Builder
###############################################################
FROM ${GCP_REGION}-docker.pkg.dev/cloud-workstations-images/predefined/code-oss AS cuttlefish-builder
ARG DEBIAN_FRONTEND=noninteractive
ARG APT_OPTS="-y --no-install-recommends"
ARG CURL_OPTS="-fsSL --retry 3 --connect-timeout 10 --max-time 300"
ARG BUILDER_PKGS="config-package-dev curl debhelper-compat devscripts equivs git golang libtinfo6"
ARG RETRY_ATTEMPTS=3
ARG RETRY_DELAY=5
ARG CUTTLEFISH_RETRY_ATTEMPTS=8
ARG GCP_REGION

SHELL ["/bin/bash", "-c"]

# Use common.sh retry helper from base image
COPY --from=base-provider /google/scripts/common.sh /google/scripts/common.sh
COPY --from=base-provider /etc/apt/keyrings/ /etc/apt/keyrings/
COPY scripts/patch_sources.sh /google/scripts/patch_sources.sh
RUN . /google/scripts/common.sh && \
    export RETRY_ATTEMPTS=${RETRY_ATTEMPTS} && \
    export RETRY_DELAY=${RETRY_DELAY} && \
    retry "apt-get update" && \
    retry "apt-get install ${APT_OPTS} ${BUILDER_PKGS}" && \
    retry "${CUTTLEFISH_RETRY_ATTEMPTS}" "${RETRY_DELAY}" "git clone https://github.com/google/android-cuttlefish" && \
    /google/scripts/patch_sources.sh /android-cuttlefish && \
    retry 5 "${RETRY_DELAY}" "/android-cuttlefish/tools/buildutils/build_packages.sh" && \
    mkdir out && \
    cp /android-cuttlefish/cuttlefish-base_*_*64.deb /out && \
    cp /android-cuttlefish/cuttlefish-user_*_*64.deb /out && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

###############################################################
# Stage: asset-prep (Ensure correct permissions on ASfP assets)
###############################################################
FROM alpine:3.19 AS asset-prep
COPY assets/ /assets/
RUN find /assets -type d -exec chmod 755 {} +

###############################################################
# Stage: Main System
###############################################################
FROM ${BASE_IMAGE}
ARG GOOGLE_CLOUD_PROJECT
ARG ASFP_VERSION=canary
ARG INSTALL_ABFS_CLIENT=false
ARG DEBIAN_FRONTEND=noninteractive

ENV GOOGLE_CLOUD_PROJECT="${GOOGLE_CLOUD_PROJECT}"
ENV HORIZON_MCP_LIB="/usr/local/lib/horizon-mcp" \
    GEMINI_MCP_AGENT_PATH="/usr/local/bin/gemini-mcp-agent" \
    ANTIGRAVITY_MCP_AGENT_PATH="/usr/local/bin/antigravity-mcp-agent"

ARG APT_OPTS="-y --no-install-recommends"
ARG CURL_OPTS="-fsSL --retry 3 --connect-timeout 10 --max-time 300"
ARG AOSP_PKGS="bison build-essential curl flex fontconfig git-core gnome-terminal gnupg lib32z1-dev libc6-dev-i386 libgl1-mesa-dev libncurses6 libx11-dev libxml2-utils repo rsync unzip x11proto-core-dev xsltproc zip zlib1g-dev"
ARG ABFS_PKGS="abfs-client git psmisc rsync unzip zip"
ARG ASFP_DEB_URL="https://dl.google.com/android/asfp/asfp-${ASFP_VERSION}-current-linux.deb"

ENV ASFP_VERSION=${ASFP_VERSION}
ENV INSTALL_ABFS_CLIENT=${INSTALL_ABFS_CLIENT}
ENV APT_OPTS=${APT_OPTS}
ENV CURL_OPTS=${CURL_OPTS}
ENV EXTRA_PKGS="${AOSP_PKGS}"
ENV EXTRA_DEB_URLS="${ASFP_DEB_URL}"
ENV WORKSTATION_DEFAULT_APP="asfp-canary"

# Copy cuttlefish builder artifacts so they can be installed by hooks
COPY --from=cuttlefish-builder /out /cuttlefish

# Merge ASfP assets into the container from the clean staging area
COPY --from=asset-prep /assets/ /

# Perform centralized workstation configuration (hooks, packages, and integration).
RUN if [ "${INSTALL_ABFS_CLIENT}" = "true" ]; then \
      export EXTRA_PKGS="${EXTRA_PKGS} ${ABFS_PKGS}"; \
    fi && \
    /google/scripts/build/configure_workstation.sh

# Hard build assertions. configure_workstation.sh's run_hooks() logs a
# warning and continues on hook failure instead of failing the build, so the
# ASfP/Cuttlefish toolchain installed by post-install hooks must be verified
# explicitly here.
RUN test -x /opt/android-studio-for-platform-canary/bin/studio && \
    test -x /usr/local/bin/asfp-studio && \
    test -f /usr/share/applications/asfp-canary.desktop && \
    dpkg -l cuttlefish-base cuttlefish-user >/dev/null
