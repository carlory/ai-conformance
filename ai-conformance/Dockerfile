ARG BASE_IMAGE=golang:1.25
ARG BUILDER_IMAGE=golang:1.25

FROM ${BUILDER_IMAGE} AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG CGO_ENABLED=0
ARG GOPROXY=
ENV GOPROXY=${GOPROXY}

# Copy the go source
COPY ./pkg /src/pkg
WORKDIR /src
COPY go.* /src
ENV CGO_ENABLED=0
RUN go mod download
RUN go install github.com/jstemmer/go-junit-report/v2@latest && go install helm.sh/helm/v3/cmd/helm@latest

# Build the test binary
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go test -c -o e2e.test ./...

FROM ${BASE_IMAGE}
WORKDIR /
COPY --from=builder /src/e2e.test e2e.test
COPY --from=builder /go/bin/go-junit-report /usr/local/bin/go-junit-report
COPY --from=builder /go/bin/helm /usr/local/bin/helm
COPY pkg/testdata testdata
RUN mkdir -p /tmp/results

CMD ["bash", "-c", "./e2e.test -test.v 2>&1 | tee /tmp/results/e2e.log && /go-junit-report -set-exit-code < /tmp/results/e2e.log > /tmp/results/junit_01.xml"]
