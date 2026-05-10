.PHONY: clean test appjs docker push mock build-go build-assets

IMAGE            ?= go-kube-ops-view
VERSION          ?= $(shell git describe --tags --always --dirty)
TAG              ?= $(VERSION)
TTYFLAGS         = $(shell test -t 0 && echo "-it")
COMMIT           ?= $(shell git rev-parse HEAD)
BUILD_DATE       ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

default: build-go

.PHONY: install
install:
	go mod download

clean:
	rm -fr assets/static/build
	rm -fr static/build
	rm -f kube-ops-view

.PHONY: lint
lint:
	go fmt ./...
	go vet ./...
	golangci-lint run || echo "golangci-lint not installed, skipping"

test: lint install
	go test -v ./...
	go test -race ./...

version:
	sed -i "s/kube-ops-view:.*/kube-ops-view:$(VERSION)/" deploy/*.yaml

appjs:
	docker run $(TTYFLAGS) -u $$(id -u) -v $$(pwd):/workdir -w /workdir/app -e NPM_CONFIG_CACHE=/tmp node:22-slim npm install
	docker run $(TTYFLAGS) -u $$(id -u) -v $$(pwd):/workdir -w /workdir/app -e NPM_CONFIG_CACHE=/tmp node:22-slim npm run build

docker: appjs
	docker build --build-arg "VERSION=$(VERSION)" -t "$(IMAGE):$(TAG)" .
	@echo 'Docker image $(IMAGE):$(TAG) can now be used.'

docker-arm: appjs
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	docker buildx create --name arm-node --append --use --platform "linux/arm"
	docker buildx build --build-arg "VERSION=$(VERSION)" --platform "linux/arm" -t $(IMAGE):$(TAG) --load .
	docker buildx rm arm-node
	@echo 'Docker image $(IMAGE):$(TAG) can now be used.'

push: docker
	docker push "$(IMAGE):$(TAG)"
	docker tag "$(IMAGE):$(TAG)" "$(IMAGE):latest"
	docker push "$(IMAGE):latest"

mock:
	docker run $(TTYFLAGS) -p 8080:8080 "$(IMAGE):$(TAG)" --mock \
		--node-link-url-template "https://kube-web-view.example.org/clusters/{cluster}/nodes/{name}" \
		--pod-link-url-template "https://kube-web-view.example.org/clusters/{cluster}/namespaces/{namespace}/pods/{name}"
build-assets:
	./scripts/build-assets.sh

build-go: build-assets
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)" -o kube-ops-view .
