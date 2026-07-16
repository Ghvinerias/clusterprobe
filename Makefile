SHELL := /bin/bash

REGISTRY ?= docker.io/slickg
TAG ?= latest
SERVICES ?= api worker ui chaos-ctrl
KUSTOMIZE_DIR ?= deploy/kustomize/base
KUSTOMIZE_FLAGS ?= --load-restrictor=LoadRestrictionsNone
HELM_DIR ?= deploy/helm/clusterprobe
VERSION ?= dev
COMMIT_SHA ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build test test-race lint gosec docker-build docker-push kustomize-build helm-lint smoke-local smoke-ui local-smoke-k8s validate-local review test-integration

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -vet=off ./... -race

test-integration:
	go test -tags=integration ./integration ./internal/db ./internal/messaging

lint:
	golangci-lint run

gosec:
	gosec ./...

docker-build:
	for service in $(SERVICES); do \
		docker build \
			-f cmd/$$service/Dockerfile \
			-t $(REGISTRY)/clusterprobe-$$service:$(TAG) \
			--build-arg VERSION=$(VERSION) \
			--build-arg COMMIT_SHA=$(COMMIT_SHA) \
			--build-arg BUILD_DATE=$(BUILD_DATE) \
			. ; \
	done

docker-push:
	for service in $(SERVICES); do \
		docker push $(REGISTRY)/clusterprobe-$$service:$(TAG) ; \
	done

kustomize-build:
	kubectl kustomize $(KUSTOMIZE_DIR) $(KUSTOMIZE_FLAGS)

helm-lint:
	helm lint $(HELM_DIR)

smoke-local:
	./scripts/smoke-local.sh

smoke-ui:
	npm run test:ui

local-smoke-k8s: smoke-local smoke-ui

validate-local:
	./scripts/validate-local.sh

review: lint test test-race helm-lint kustomize-build gosec
