# syntax=docker/dockerfile:1.4

# Copyright (C) 2026 The Qt Company Ltd.
# All rights reserved.
#
# This file is provided AS IS with NO WARRANTY OF ANY KIND, INCLUDING THE
# WARRANTY OF DESIGN, MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE.

# This image layers the Squish application onto the GNOME foundation.

# The BASE_IMAGE is resolved by Skaffold.
ARG BASE_IMAGE

FROM ${BASE_IMAGE}

ARG CURL_OPTS="-fsSL --retry 3"
ARG EXTRA_PKGS="\
  libxcb-icccm4 \
  libxcb-image0 \
  libxcb-keysyms1 \
  libxcb-render-util0 \
  libxcb-xinerama0 \
  libxcb-xkb1 \
  libxkbcommon-x11-0"
ARG SQUISH_DOWNLOAD_URL="https://d13lb3tujbc8s0.cloudfront.net/squish/squish-9.2.2-android-linux64.run"
ARG SQUISH_GCP_LICENSE_SECRET
ARG SQUISH_SHA256="202758cf795b6cae939d5ba714fd1d31c83478746f0fb1b2ea33ca3881e7669d"

ENV CURL_OPTS="${CURL_OPTS}"
ENV EXTRA_PKGS="${EXTRA_PKGS}"
ENV SQUISH_DOWNLOAD_URL="${SQUISH_DOWNLOAD_URL}"
ENV SQUISH_GCP_LICENSE_SECRET="${SQUISH_GCP_LICENSE_SECRET}"
ENV SQUISH_SHA256="${SQUISH_SHA256}"

# Merge assets into the container
COPY assets/ /

# Perform centralized workstation configuration (hooks, packages, and integration).
RUN /google/scripts/build/configure_workstation.sh
