# Copyright 2023 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# This Dockerfile expects the build context to be the repo root.

# NOTE: don't put anything in /tmp here. It will work locally,
# but Cloud Run mounts something else to /tmp, so anything
# installed here will be shadowed.

FROM golang:1.20

LABEL maintainer="Go Telemetry Team <go-telemetry-team@google.com>"

#### Preliminaries

WORKDIR /

# Create some directories.

# The telemetrygodev binary and related files live here.
RUN mkdir /app

#### Building binaries

# Set the working directory outside $GOPATH to ensure module mode is enabled.
WORKDIR /telemetry

# Copy go.mods and go.sums into the container.
# If they don't change, which is the common case, then docker can
# cache these COPYs and the subsequent RUN.
COPY go.mod go.sum ./

WORKDIR /telemetry/godev

COPY go.mod go.sum ./

# Download the dependencies.
RUN go mod download

WORKDIR /telemetry

# Copy the repo from local machine into Docker client’s current working
# directory, so that we can use it to build the binary.
# See .dockerignore at the repo root for excluded files.
COPY . ./

WORKDIR /telemetry/godev

# Build the telemetrygodev binary and put it in /app.
RUN go build -mod=readonly -o /app/telemetrygodev ./cmd/telemetrygodev

#### telemetrygodev init

WORKDIR /app

CMD ["./telemetrygodev"]
