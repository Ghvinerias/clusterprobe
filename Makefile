SHELL := /bin/bash

IMAGE ?= clusterprobe
TAG ?= latest
KUSTOMIZE_DIR ?= deploy/kustomize/base
HELM_DIR ?= deploy/helm/clusterprobe

.PHONY: build test lint gosec docker-build docker-push kustomize-build helm-lint review test-integration

build:
	go build ./...

test:
	go test ./...

test-integration:
	go test ./integration -tags=integration

lint:
	golangci-lint run

gosec:
	gosec ./...

docker-build:
	docker build -t $(IMAGE):$(TAG) .

docker-push:
	docker push $(IMAGE):$(TAG)

kustomize-build:
	kustomize build $(KUSTOMIZE_DIR)

helm-lint:
	helm lint $(HELM_DIR)

review: lint test gosec
