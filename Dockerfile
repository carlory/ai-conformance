ARG BASE_IMAGE=registry.k8s.io/conformance:v1.34.0
ARG BUILDER_IMAGE=golang:1.25

# Build the e2e binary
FROM ${BUILDER_IMAGE} AS builder
WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY e2e e2e

# Build
RUN go test -c -o e2e.test /workspace/e2e

## Install Helm
RUN curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

FROM ${BASE_IMAGE}
COPY --from=builder /workspace/e2e.test /usr/local/bin/
COPY --from=builder /usr/local/bin/helm /usr/local/bin/
ENV RESULTS_DIR="/tmp/results"
ENV ARTIFACTS="/tmp/results"