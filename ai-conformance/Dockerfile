ARG BASE_IMAGE=golang:1.25
ARG BUILDER_IMAGE=golang:1.25

FROM ${BUILDER_IMAGE} AS builder
ARG TARGETOS=linux
ARG TARGETARCH=arm64
ARG CGO_ENABLED=0
ARG GOPROXY=
ENV GOPROXY=${GOPROXY}

# Copy the go source
COPY ./pkg /src/pkg
WORKDIR /src
COPY go.* /src
ENV CGO_ENABLED=0
RUN go mod download

# Build the test binary
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go test -c -o e2e.test ./...

FROM ${BASE_IMAGE}
WORKDIR /
COPY --from=builder /src/e2e.test e2e.test
COPY pkg/testdata testdata

CMD ["bash", "-c", "go tool test2json ./e2e.test -test.v"]
