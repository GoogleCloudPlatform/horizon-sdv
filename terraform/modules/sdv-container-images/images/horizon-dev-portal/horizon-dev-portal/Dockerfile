# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

# SPDX-License-Identifier: Apache-2.0

# CLI sources are staged into ./cli (terraform local_file, or copy tools/horizon/*.{go,mod,sum}).
FROM golang:1.25-alpine AS cli
WORKDIR /src
COPY cli/ ./
ARG HORIZON_CLI_VERSION=dev
ARG HORIZON_CLI_COMMIT=unknown
ENV HORIZON_CLI_COMMIT=$HORIZON_CLI_COMMIT
RUN go run mkdist.go /out "$HORIZON_CLI_VERSION"

FROM node:22-alpine AS frontend
ENV HUSKY=0
WORKDIR /app
COPY package.json package-lock.json* .npmrc* ./
RUN npm ci
COPY . .
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY proxy/go.mod proxy/go.sum* ./
RUN go mod download
COPY proxy/*.go ./
COPY --from=frontend /app/dist ./dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /horizon-dev-portal .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /horizon-dev-portal /horizon-dev-portal
COPY --from=cli /out /cli-dist
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/horizon-dev-portal"]
