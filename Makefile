# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Project variables
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
ARTIFACTS ?= $(PROJECT_DIR)/bin
GO_VERSION := $(shell awk '/^go /{print $$2}' go.mod|head -n1)

# Build variables
BASE_IMAGE ?= registry.k8s.io/conformance:v1.34.0
DOCKER_BUILDX_CMD ?= docker buildx
IMAGE_BUILD_CMD ?= $(DOCKER_BUILDX_CMD) build
IMAGE_BUILD_EXTRA_OPTS ?=
IMAGE_REGISTRY ?= ghcr.io/carlory
IMAGE_NAME ?= ai-conformance
IMAGE_REPO := $(IMAGE_REGISTRY)/$(IMAGE_NAME)
# GIT_TAG ?= $(shell git describe --tags --dirty --always)
GIT_TAG ?= latest
GOPROXY=${GOPROXY:-""}
IMG ?= $(IMAGE_REPO):$(GIT_TAG)
BUILDER_IMAGE ?= golang:$(GO_VERSION)
CGO_ENABLED ?= 0

# E2E variables
KIND_CLUSTER_NAME ?= kind-ai-conformance
E2E_KIND_NODE_VERSION ?= kindest/node:v1.34.0
E2E_KIND_VERSION ?= v0.30.0
USE_EXISTING_CLUSTER ?= false
GINKGO_VERSION ?= v2.22.1
GINKGO_FOCUS ?= \[AIConformance\]
GINKGO_SKIP ?= \[Disruptive\]|NoExecuteTaintManager
E2E_RESULTS_DIR ?= /tmp/results
# Additional parameters to be provided to the conformance container. These parameters should be specified as key-value pairs, separated by commas.
# Each parameter should start with -- (e.g., --clean-start=true,--allowed-not-ready-nodes=2)
E2E_EXTRA_ARGS ?="--ai.operator.chart=kuberay-operator,--ai.operator.repo=https://ray-project.github.io/kuberay-helm/,--ai.podAutoscaling.metricName=vllm:num_requests_running"
# Sonobuoy E2E variables
SONOBUOY_PLUGIN_FILE ?= $(PROJECT_DIR)/sonobuoy-plugin.yaml
SONOBUOY_VERSION ?= v0.57.3
# Hydrophone E2E variables
HYDROPHONE_VERSION ?= v0.7.0

## Tool Binaries
KUBECTL ?= kubectl
HELM ?= helm

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

.PHONY: build
build: ## Build manager binary.
	CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go test -c -o e2e.test ./e2e

.PHONY: image-build
image-build: ## Build the image
	$(IMAGE_BUILD_CMD) -t $(IMG) \
		--build-arg BASE_IMAGE=$(BASE_IMAGE) \
		--build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
		$(IMAGE_BUILD_EXTRA_OPTS) ./
image-load: IMAGE_BUILD_EXTRA_OPTS=--load
image-load: image-build
image-push: IMAGE_BUILD_EXTRA_OPTS=--push
image-push: image-build

KIND = $(shell pwd)/bin/kind
.PHONY: kind
kind:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on go install sigs.k8s.io/kind@${E2E_KIND_VERSION}

.PHONY: kind-image-build
kind-image-build: PLATFORMS=linux/amd64
kind-image-build: IMAGE_BUILD_EXTRA_OPTS=--load
kind-image-build: kind image-build

##@ Test
GINKGO = $(shell pwd)/bin/ginkgo
.PHONY: ginkgo
ginkgo:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: test-e2e
test-e2e: ginkgo kind
	@echo "Running E2E tests for AI Conformance"
	E2E_EXTRA_ARGS="$(E2E_EXTRA_ARGS)" GINKGO=$(GINKGO) GINKGO_FOCUS="$(GINKGO_FOCUS)" GINKGO_SKIP="$(GINKGO_SKIP)" E2E_TEST_RUNNER=$(E2E_TEST_RUNNER) USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) KIND=$(KIND) KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) E2E_KIND_NODE_VERSION=$(E2E_KIND_NODE_VERSION) IMG=$(IMG) KUBECTL=$(KUBECTL) HELM=$(HELM) ./hack/e2e-test.sh


SONOBUOY = $(shell pwd)/bin/sonobuoy
.PHONY: sonobuoy
sonobuoy:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on go install github.com/vmware-tanzu/sonobuoy@${SONOBUOY_VERSION}


.PHONY: generate-plugin
generate-plugin:  sonobuoy ## Generate the Sonobuoy plugin yaml file
	@echo "Generating Sonobuoy plugin yaml file"
	@PLUGIN_EXTRA_ARGS="--progress-report-url=http://localhost:8099/progress"; \
	if [ -n "$(E2E_EXTRA_ARGS)" ]; then \
		E2E_ARGS_SPACED=$$(echo "$(E2E_EXTRA_ARGS)" | sed 's/,/ /g'); \
		PLUGIN_EXTRA_ARGS="$$PLUGIN_EXTRA_ARGS $$E2E_ARGS_SPACED"; \
	fi; \
	$(SONOBUOY) gen plugin --name ai-conformance \
		--type Job \
		--format junit \
		--url https://raw.githubusercontent.com/carlory/sonobuoy-plugins/master/ai-conformance/plugin.yaml \
		--description "Running E2E tests for AI Conformance via Sonobuoy to answer the self-certification questionnaire template in https://github.com/cncf/ai-conformance." \
		--env "E2E_EXTRA_ARGS=$$PLUGIN_EXTRA_ARGS" \
		--env "E2E_FOCUS=$(GINKGO_FOCUS)" \
		--env "E2E_PARALLEL=false" \
		--env "E2E_SKIP=$(GINKGO_SKIP)" \
		--env "E2E_USE_GO_RUNNER=true" \
		--env "RESULTS_DIR=/tmp/sonobuoy/results" \
		--cmd "kubeconformance" \
		--node-selector "kubernetes.io/os=linux" \
		--image $(IMG) \
		--show-default-podspec \
		> $(SONOBUOY_PLUGIN_FILE)

.PHONY: test-sonobuoy
test-sonobuoy: sonobuoy kind-image-build generate-plugin ## Run E2E tests for AI Conformance via Sonobuoy
	@echo "Running E2E tests for AI Conformance via Sonobuoy"
	E2E_TEST_RUNNER="sonobuoy" USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) KIND=$(KIND) KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) E2E_KIND_NODE_VERSION=$(E2E_KIND_NODE_VERSION) IMG=$(IMG) KUBECTL=$(KUBECTL) HELM=$(HELM) SONOBUOY=$(SONOBUOY) SONOBUOY_PLUGIN_FILE=$(SONOBUOY_PLUGIN_FILE) E2E_RESULTS_DIR=$(E2E_RESULTS_DIR) ./hack/e2e-test.sh

HYDROPHONE = $(shell pwd)/bin/hydrophone
.PHONY: hydrophone
hydrophone:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on go install sigs.k8s.io/hydrophone@${HYDROPHONE_VERSION}

# FIXME: Migrate the current implementation to ginkgo in order to support other Hydrophone features.
.PHONY: test-hydrophone 
test-hydrophone: hydrophone kind-image-build ## Run E2E tests for AI Conformance via Hydrophone
	@echo "Running E2E tests for AI Conformance via Hydrophone"
	E2E_TEST_RUNNER="hydrophone" E2E_EXTRA_ARGS="$(E2E_EXTRA_ARGS)" GINKGO_FOCUS="$(GINKGO_FOCUS)" GINKGO_SKIP="$(GINKGO_SKIP)" USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) KIND=$(KIND) KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) E2E_KIND_NODE_VERSION=$(E2E_KIND_NODE_VERSION) IMG=$(IMG) KUBECTL=$(KUBECTL) HELM=$(HELM) HYDROPHONE=$(HYDROPHONE) E2E_RESULTS_DIR=$(E2E_RESULTS_DIR) ./hack/e2e-test.sh