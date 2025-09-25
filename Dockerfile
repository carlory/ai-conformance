ARG BASE_IMAGE=golang:1.25
ARG BUILDER_IMAGE=golang:1.25

# Build the e2e binary
FROM ${BUILDER_IMAGE} AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG CGO_ENABLED=0
ARG GOPROXY=
ENV GOPROXY=${GOPROXY}

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY e2e/ e2e/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go test -c -o e2e.test ./...

## Install Helm and Ginkgo
RUN go install helm.sh/helm/v3/cmd/helm@latest
RUN go install github.com/onsi/ginkgo/v2/ginkgo@v2.25.3

FROM ${BASE_IMAGE}
WORKDIR /
COPY --from=builder /workspace/e2e.test /usr/local/bin/e2e.test
COPY --from=builder /go/bin/helm /usr/local/bin/helm
COPY --from=builder /go/bin/ginkgo /usr/local/bin/ginkgo
ENV RESULTS_DIR="/tmp/results"
ENV ARTIFACTS="/tmp/results"

RUN mkdir -p $RESULTS_DIR && chmod a+x /usr/local/bin/ginkgo && chmod a+x /usr/local/bin/helm
USER 65532:65532

ENTRYPOINT [ "sh", "-c", "ginkgo -v run /usr/local/bin/e2e.test | tee $RESULTS_DIR/e2e.log ; tar -czf $RESULTS_DIR/e2e.tar.gz -C $RESULTS_DIR e2e.log -C $ARTIFACTS junit_01.xml; echo $RESULTS_DIR/e2e.tar.gz > $RESULTS_DIR/done" ]

