# syntax=docker/dockerfile:1.4

# Copyright (c) 2026 The Qt Company Ltd., All Rights Reserved.
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

# Squish layer on top of the Horizon GNOME Cloud Workstations base image
# (workloads/cloud-workstations/pipelines/workstation-images/horizon-gnome).
# Built by the squish-workstation-init Argo WorkflowTemplate via the common-docker-image-build
# ClusterWorkflowTemplate. Squish itself is proprietary software of The Qt Company: it is downloaded
# at build time (sha256-verified) and is not part of this repository.

# Full URI of the published horizon-gnome image (passed by squish-workstation-init).
ARG BASE_IMAGE

FROM ${BASE_IMAGE}

ARG CURL_OPTS="-fsSL --retry 3"
ARG EXTRA_PKGS="\
  jq \
  libxcb-icccm4 \
  libxcb-image0 \
  libxcb-keysyms1 \
  libxcb-render-util0 \
  libxcb-xinerama0 \
  libxcb-xkb1 \
  libxkbcommon-x11-0"
ARG GCP_REGION="us-central1"
ARG SQUISH_DOWNLOAD_URL="https://d13lb3tujbc8s0.cloudfront.net/squish/squish-9.2.2-android-linux64.run"
ARG SQUISH_GCP_LICENSE_SECRET="squish-license-key"
ARG SQUISH_SHA256="202758cf795b6cae939d5ba714fd1d31c83478746f0fb1b2ea33ca3881e7669d"

ENV CURL_OPTS="${CURL_OPTS}"
ENV EXTRA_PKGS="${EXTRA_PKGS}"
ENV GCP_REGION="${GCP_REGION}"
ENV SQUISH_DOWNLOAD_URL="${SQUISH_DOWNLOAD_URL}"
ENV SQUISH_GCP_LICENSE_SECRET="${SQUISH_GCP_LICENSE_SECRET}"
ENV SQUISH_SHA256="${SQUISH_SHA256}"

# Merge assets into the container
COPY assets/ /

# Perform centralized workstation configuration (hooks, packages, and integration).
RUN /google/scripts/build/configure_workstation.sh
