SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c

.DEFAULT_GOAL := build

IMAGE ?= go-kube-ops-view
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
TAG ?= $(VERSION)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
NODE_VERSION ?= 24
NODE_IMAGE ?= node:$(NODE_VERSION)-slim
TTYFLAGS ?= $(shell test -t 0 && echo "-it")

HELM ?= helm
KUBECTL ?= kubectl
KUSTOMIZE ?= $(KUBECTL) kustomize
HELM_CHART ?= deploy/helm/kube-ops-view
HELM_RELEASE ?= kube-ops-view
HELM_NAMESPACE ?= kube-ops-view
HELM_VALUES ?=
HELM_SET ?=
HELM_VALUE_ARGS = $(if $(HELM_VALUES),-f $(HELM_VALUES),) $(HELM_SET)
KUSTOMIZE_PATH ?= deploy/production
KUSTOMIZE_DIRS ?= deploy deploy/production deploy/overlays/development deploy/overlays/staging

SMOKE_PORT ?= 18080
SMOKE_CONTAINER ?= go-kube-ops-view-smoke

DOCKER_BUILD_ARGS = --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(BUILD_DATE)

.PHONY: help clean install fmt fmt-check lint vet test test-race go-ci frontend-install frontend-lint frontend-build frontend-audit frontend-ci frontend-container appjs build-assets build-go build ci manifests-check helm-lint helm-template kustomize-build docker-build docker-smoke docker-run-mock docker-push install-kustomize uninstall-kustomize install-helm uninstall-helm

help:
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Download Go modules.
	go mod download

clean: ## Remove local build outputs.
	rm -fr assets/static/build static/build kube-ops-view app/node_modules

fmt: ## Format Go source files.
	find . -path './app/node_modules' -prune -o -name '*.go' -print0 | xargs -0 gofmt -w

fmt-check: ## Verify Go source files are formatted.
	@files="$$(find . -path './app/node_modules' -prune -o -name '*.go' -print0 | xargs -0 gofmt -l)"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needed for:"; \
		echo "$$files"; \
		exit 1; \
	fi

vet: ## Run go vet.
	go vet $$(go list ./... | grep -v '/app/node_modules/')

lint: fmt-check vet frontend-lint ## Run Go and frontend lint checks.

test: ## Run Go tests.
	go test $$(go list ./... | grep -v '/app/node_modules/')

test-race: ## Run Go tests with the race detector.
	go test -race $$(go list ./... | grep -v '/app/node_modules/')

go-ci: install fmt-check vet test ## Run Go checks used by CI.

frontend-install: ## Install frontend dependencies with npm ci.
	cd app && npm ci

frontend-lint: ## Run frontend lint.
	cd app && npm run lint

frontend-build: ## Build frontend assets.
	cd app && npm run build

frontend-audit: ## Run npm audit for production and tooling dependencies.
	cd app && npm audit --audit-level=moderate

frontend-ci: frontend-install frontend-lint frontend-build frontend-audit ## Run frontend checks used by CI.

frontend-container: ## Run frontend checks in the pinned Node container image.
	docker run --rm $(TTYFLAGS) -u "$$(id -u):$$(id -g)" -v "$$(pwd):/workdir" -w /workdir/app -e NPM_CONFIG_CACHE=/tmp/npm-cache $(NODE_IMAGE) sh -lc 'node --version && npm --version && npm ci && npm run lint && npm run build && npm audit --audit-level=moderate'

appjs: frontend-container ## Build frontend assets with the pinned Node container image.

build-assets: ## Build frontend assets using the local Node toolchain.
	./scripts/build-assets.sh

build-go: build-assets ## Build the Go binary with embedded frontend assets.
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)" -o kube-ops-view .

build: build-go ## Build the local application binary.

kustomize-build: ## Render all supported kustomize bases and overlays.
	@for dir in $(KUSTOMIZE_DIRS); do \
		echo "Rendering kustomize: $$dir"; \
		$(KUSTOMIZE) "$$dir" >/dev/null; \
	done

helm-lint: ## Lint the Helm chart.
	$(HELM) lint $(HELM_CHART)

helm-template: ## Render the Helm chart.
	$(HELM) template $(HELM_RELEASE) $(HELM_CHART) --namespace $(HELM_NAMESPACE) $(HELM_VALUE_ARGS) >/dev/null

manifests-check: kustomize-build helm-lint helm-template ## Validate kustomize and Helm install manifests.

ci: frontend-ci go-ci manifests-check ## Run the non-Docker CI checks.

docker-build: ## Build the Docker image.
	docker buildx build --load $(DOCKER_BUILD_ARGS) -t "$(IMAGE):$(TAG)" .

docker-smoke: docker-build ## Run the Docker image in mock mode and verify health, HTML, and JS asset serving.
	@docker rm -f "$(SMOKE_CONTAINER)" >/dev/null 2>&1 || true; \
	cleanup() { docker rm -f "$(SMOKE_CONTAINER)" >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT; \
	docker run -d --name "$(SMOKE_CONTAINER)" -p "$(SMOKE_PORT):8080" "$(IMAGE):$(TAG)" --mock >/dev/null || exit 1; \
	for _ in $$(seq 1 30); do \
		if curl -fsS "http://localhost:$(SMOKE_PORT)/health" >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 1; \
	done; \
	curl -fsS "http://localhost:$(SMOKE_PORT)/health" >/dev/null; \
	index="$$(curl -fsS "http://localhost:$(SMOKE_PORT)/")"; \
	asset="$$(printf '%s' "$$index" | sed -n 's/.*src="\([^"]*\/static\/build\/[^"]*\.js\)".*/\1/p' | head -1)"; \
	if [ -z "$$asset" ]; then \
		echo "could not find frontend asset in rendered index"; \
		exit 1; \
	fi; \
	curl -fsS "http://localhost:$(SMOKE_PORT)$$asset" >/dev/null; \
	echo "Docker smoke passed for $(IMAGE):$(TAG)"

docker-run-mock: docker-build ## Run the Docker image locally in mock mode.
	docker run --rm $(TTYFLAGS) -p 8080:8080 "$(IMAGE):$(TAG)" --mock

docker-push: docker-build ## Push the Docker image tag.
	docker push "$(IMAGE):$(TAG)"

install-kustomize: ## Install with kustomize. Override KUSTOMIZE_PATH=deploy/overlays/development if needed.
	$(KUBECTL) apply -k $(KUSTOMIZE_PATH)

uninstall-kustomize: ## Uninstall the selected kustomize path.
	$(KUBECTL) delete -k $(KUSTOMIZE_PATH) --ignore-not-found=true

install-helm: ## Install or upgrade with Helm. Override HELM_VALUES or HELM_SET as needed.
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART) --namespace $(HELM_NAMESPACE) --create-namespace $(HELM_VALUE_ARGS)

uninstall-helm: ## Uninstall the Helm release.
	$(HELM) uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)
