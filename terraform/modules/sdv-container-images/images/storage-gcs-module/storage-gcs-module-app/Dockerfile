# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.

FROM --platform=linux/amd64 golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /manager ./cmd/manager

# Distroless nonroot: no shell or debug tools in the runtime image (use ephemeral debug containers if needed).
FROM --platform=linux/amd64 gcr.io/distroless/static-debian12:nonroot

COPY --from=build /manager /manager

USER nonroot:nonroot

ENTRYPOINT ["/manager"]
