# Copyright (c) 2026 Accenture, All Rights Reserved.
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
#
# Jenkins agent image for Horizon API pipelines: horizon CLI (Go) + bash/curl for scripts.
# Build context must be the repository root (see Sample/Environment/Docker Image Template Jenkinsfile).

# syntax=docker/dockerfile:1

ARG LINUX_DISTRIBUTION=alpine:3.19

FROM golang:1.27.2-alpine AS horizon-build
WORKDIR /src
COPY tools/horizon /src/h
WORKDIR /src/h
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /horizon .

FROM ${LINUX_DISTRIBUTION}
COPY --from=horizon-build /horizon /usr/local/bin/horizon

# coreutils: stdbuf for line-buffered log streaming (horizon-workflow-via-rest.sh).
RUN apk add --no-cache \
    bash \
    ca-certificates \
    coreutils \
    curl \
    jq \
    python3 \
    py3-pip \
  && ln -sf python3 /usr/bin/python \
  && rm -rf /var/cache/apk/*

WORKDIR /home/jenkins
