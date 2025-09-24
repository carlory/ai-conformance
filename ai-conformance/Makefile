# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Project variables
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
ARTIFACTS ?= $(PROJECT_DIR)/bin
GO_VERSION := $(shell awk '/^go /{print $$2}' go.mod|head -n1)

# Build variables
BASE_IMAGE ?= golang:$(GO_VERSION)
DOCKER_BUILDX_CMD ?= docker buildx
IMAGE_BUILD_CMD ?= $(DOCKER_BUILDX_CMD) build
IMAGE_BUILD_EXTRA_OPTS ?=
IMAGE_REGISTRY ?= ghcr.io/carlory/sonobuoy-plugins
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
# Sonobuoy E2E variables
SONOBUOY_PLUGIN_FILE ?= $(PROJECT_DIR)/plugin.yaml
SONOBUOY_VERSION ?= v0.57.3
SONOBUOY_OUTPUT_DIR ?= /tmp/sonobuoy
# Hydrophone E2E variables
HYDROPHONE_VERSION ?= v0.7.0
HYDROPHONE_OUTPUT_DIR ?= /tmp/hydrophone

## Tool Binaries
KUBECTL ?= kubectl
HELM ?= helm
JQ ?= jq

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

.PHONY: build
build: ## Build manager binary.
	CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go test -c -o ./pkg/e2e.test ./pkg

.PHONY: image-build
image-build: ## Build the image
	$(IMAGE_BUILD_CMD) -t $(IMG) \
		--build-arg BASE_IMAGE=$(BASE_IMAGE) \
		--build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
		--build-arg CGO_ENABLED=$(CGO_ENABLED) \
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
.PHONY: test-e2e
test-e2e: build kind
	@echo "Running E2E tests for AI Conformance"
	E2E_TEST_RUNNER=$(E2E_TEST_RUNNER) USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) KIND=$(KIND) KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) E2E_KIND_NODE_VERSION=$(E2E_KIND_NODE_VERSION) IMG=$(IMG) KUBECTL=$(KUBECTL) HELM=$(HELM) JQ=$(JQ) ./hack/e2e-test.sh


SONOBUOY = $(shell pwd)/bin/sonobuoy
.PHONY: sonobuoy
sonobuoy:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on go install github.com/vmware-tanzu/sonobuoy@${SONOBUOY_VERSION}


.PHONY: generate-plugin
generate-plugin:  sonobuoy ## Generate the Sonobuoy plugin yaml file
	@echo "Generating Sonobuoy plugin yaml file"
	@$(SONOBUOY) gen plugin --name ai-conformance \
		--type Job \
		--format raw \
		--url https://raw.githubusercontent.com/carlory/sonobuoy-plugins/master/ai-conformance/plugin.yaml \
		--description "Running E2E tests for AI Conformance via Sonobuoy to answer the self-certification questionnaire template in https://github.com/cncf/ai-conformance." \
		--image $(IMG) \
		--cmd "bash" \
		--arg "-c" \
		--arg './e2e.test -test.v | tee $${SONOBUOY_RESULTS_DIR}/e2e.log ; echo $${SONOBUOY_RESULTS_DIR}/e2e.log > $${SONOBUOY_RESULTS_DIR}/done' \
		> $(SONOBUOY_PLUGIN_FILE)

.PHONY: test-sonobuoy
test-sonobuoy: sonobuoy kind-image-build generate-plugin ## Run E2E tests for AI Conformance via Sonobuoy
	@echo "Running E2E tests for AI Conformance via Sonobuoy"
	E2E_TEST_RUNNER="sonobuoy" USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) KIND=$(KIND) KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) E2E_KIND_NODE_VERSION=$(E2E_KIND_NODE_VERSION) IMG=$(IMG) KUBECTL=$(KUBECTL) HELM=$(HELM) SONOBUOY=$(SONOBUOY) SONOBUOY_PLUGIN_FILE=$(SONOBUOY_PLUGIN_FILE) OUTPUT_DIR=$(SONOBUOY_OUTPUT_DIR) JQ=$(JQ) ./hack/e2e-test.sh

HYDROPHONE = $(shell pwd)/bin/hydrophone
.PHONY: hydrophone
hydrophone:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on go install sigs.k8s.io/hydrophone@${HYDROPHONE_VERSION}

# FIXME: Migrate the current implementation to ginkgo, and generate e2e.log and junit_01.xml file for Hydrophone to download.
.PHONY: test-hydrophone 
test-hydrophone: hydrophone kind-image-build ## Run E2E tests for AI Conformance via Hydrophone
	@echo "Running E2E tests for AI Conformance via Hydrophone"
	E2E_TEST_RUNNER="hydrophone" USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) KIND=$(KIND) KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) E2E_KIND_NODE_VERSION=$(E2E_KIND_NODE_VERSION) IMG=$(IMG) KUBECTL=$(KUBECTL) HELM=$(HELM) HYDROPHONE=$(HYDROPHONE) JQ=$(JQ) ./hack/e2e-test.sh