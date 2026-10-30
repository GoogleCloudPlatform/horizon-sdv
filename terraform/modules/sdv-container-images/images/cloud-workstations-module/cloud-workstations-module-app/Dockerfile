# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.

# Build the Cloud Workstations Module controller binary
FROM --platform=linux/amd64 golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /manager ./cmd/manager

# Distroless nonroot runtime image
FROM --platform=linux/amd64 gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /manager /manager
USER 65532:65532

ENTRYPOINT ["/manager"]

