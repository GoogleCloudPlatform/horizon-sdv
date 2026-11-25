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
ARG GO_VERSION
FROM --platform=linux/amd64 golang:${GO_VERSION}-alpine AS builder
# Built with images/ as the context, so paths are relative to it. The working directory
# mirrors the repo layout so go.mod's replace ../../shared/horizonauth resolves.
WORKDIR /src/horizon-api/horizon-api-app
COPY shared/horizonauth/ /src/shared/horizonauth/
COPY horizon-api/horizon-api-app/go.mod horizon-api/horizon-api-app/go.sum ./
RUN go mod download
COPY horizon-api/horizon-api-app/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /horizon-api .

FROM --platform=linux/amd64 alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder --chown=nobody:nobody /horizon-api /horizon-api
USER nobody
ENTRYPOINT ["/horizon-api"]
