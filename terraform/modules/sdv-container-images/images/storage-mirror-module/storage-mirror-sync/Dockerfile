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

# Storage Mirror workflow image: mirror-agent plus git, git-lfs and repo. No cloud CLIs.

FROM --platform=linux/amd64 golang:1.27.2-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /mirror-agent ./cmd/mirror-agent

FROM --platform=linux/amd64 debian:12-slim

# repo needs python3; gnupg verifies the repo launcher signature (same key as the Jenkins mirror image).
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends ca-certificates curl git git-lfs gnupg python3 && \
    export REPO=$(mktemp /tmp/repo.XXXXXXXXX) && \
    curl -fsSL -o ${REPO} https://storage.googleapis.com/git-repo-downloads/repo && \
    gpg --recv-key 8BB9AD793E8E6153AF0F9A4416530D5E920F5C65 && \
    curl -fsSL https://storage.googleapis.com/git-repo-downloads/repo.asc | gpg --verify - ${REPO} && \
    install -m 755 ${REPO} /usr/bin/repo && \
    rm -rf /var/lib/apt/lists/* /root/.gnupg ${REPO}

# UID/GID 1000 owns the mirror share (setgid group 1000), as for the Jenkins mirror jobs.
RUN useradd -u 1000 -ms /bin/bash horizon-sdv-mirror-ops && \
    git lfs install --system --skip-smudge && \
    git config --system user.email "horizon-sdv-mirror-ops@example.com" && \
    git config --system user.name "horizon-sdv-mirror-ops" && \
    git config --system color.ui false && \
    git config --system --add safe.directory '*'

COPY --from=build /mirror-agent /usr/local/bin/mirror-agent

USER horizon-sdv-mirror-ops
WORKDIR /home/horizon-sdv-mirror-ops

ENTRYPOINT ["/usr/local/bin/mirror-agent"]
