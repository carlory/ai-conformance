#!/usr/bin/env bash

set -ex

REGISTRY=ghcr.io/carlory
IMG=sonobuoy-plugins/ai-conformance
TAG=v0.1.0

docker build . -t $REGISTRY/$IMG:$TAG
docker push $REGISTRY/$IMG:$TAG