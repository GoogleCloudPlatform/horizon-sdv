# syntax=docker/dockerfile:1.4

# Copyright (c) 2026 Accenture and Google LLC, All Rights Reserved.
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

# Horizon Preflight provider image.
# Alpine image that exports nginx Preflight loader assets, systemd orchestration,
# machine-id / Cloud Logging setup, and config_rendering under /export/.
# Consumed by horizon-gnome via: COPY --from=preflight-provider /export/ /
#
# Build context is this directory (image-dir), not the monorepo root.
# Optional build-args PREFLIGHT_WEB_REPO / PREFLIGHT_WEB_DIR override the
# vendored preflight-web/ SPA (see build_web.sh).

ARG PREFLIGHT_WEB_REPO=""
ARG PREFLIGHT_WEB_DIR=""

ARG ALPINE_VERSION=3.19
ARG NODE_VERSION=22-slim

###############################################################
# Stage: web-builder
###############################################################
FROM node:${NODE_VERSION} AS web-builder
ARG PREFLIGHT_WEB_REPO
ARG PREFLIGHT_WEB_DIR
WORKDIR /build/web
RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*
# Path must be ./source/ so build_web.sh finds /build/web/source/package.json
COPY preflight-web/ ./source/
COPY docs/privacy_notice.md ./preflight/docs/privacy_notice.md
COPY assets/google/scripts/build/build_web.sh ./build_web.sh
RUN chmod +x ./build_web.sh && ./build_web.sh "${PREFLIGHT_WEB_REPO}" "${PREFLIGHT_WEB_DIR}"

###############################################################
# Stage: Preflight Provider
###############################################################
FROM alpine:${ALPINE_VERSION}
COPY assets/ /export/
COPY --from=web-builder /build/web/dist/ /export/var/www/html/
