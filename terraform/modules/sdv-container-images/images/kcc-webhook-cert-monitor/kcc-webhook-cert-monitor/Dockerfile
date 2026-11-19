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
# Minimal image for kcc-webhook-cert-monitor CronJob: openssl, curl, jq (no runtime apk).
# Matches Kubernetes pod securityContext runAsUser 65534 (nobody).
# Alpine 3.24 is still supported. USER nobody matches runAsUser 65534.
FROM --platform=linux/amd64 alpine:3.24

RUN apk add --no-cache openssl curl jq \
  && rm -rf /var/cache/apk/*

USER nobody
WORKDIR /tmp
