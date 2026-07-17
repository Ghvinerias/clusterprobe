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

.PHONY: build test test-race lint gosec govulncheck secret-scan image-scan docker-build docker-push kustomize-build helm-lint smoke-local smoke-ui local-smoke-k8s validate-local validate-local-k8s review test-integration

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

govulncheck:
	GOTOOLCHAIN=go1.25.12 govulncheck ./...

secret-scan:
	gitleaks detect --source . --no-git --redact

image-scan:
	@scanner=""; \
	if command -v trivy >/dev/null 2>&1; then scanner="trivy"; \
	elif command -v grype >/dev/null 2>&1; then scanner="grype"; \
	else echo "missing required image scanner: install trivy or grype" >&2; exit 1; fi; \
	for service in $(SERVICES); do \
		image="$(REGISTRY)/clusterprobe-$$service:$(TAG)"; \
		if [[ "$$scanner" == "trivy" ]]; then \
			trivy image --exit-code 1 --severity HIGH,CRITICAL "$$image"; \
		else \
			grype "$$image" --fail-on high; \
		fi; \
	done

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

validate-local-k8s:
	./scripts/validate-local-k8s.sh

review:
	./scripts/review/run.sh
