BINARY ?= cnpg-plugin-pgdump
IMAGE ?= platform/cnpg-plugin-pgdump:latest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test docker-build fmt

build:
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) .

test:
	go test ./...

fmt:
	gofmt -w .

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .
