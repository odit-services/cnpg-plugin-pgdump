BINARY ?= cnpg-plugin-pgdump
IMAGE ?= platform/cnpg-plugin-pgdump:latest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
E2E_PARALLELISM ?= 2
BIN_DIR ?= $(CURDIR)/.bin
SHIKAI ?= $(BIN_DIR)/shikai
SHIKAI_ARGS ?=

.PHONY: build test e2e docker-build fmt release install-shikai

build:
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) .

test:
	go test ./...

e2e:
	go test -tags=e2e ./test/e2e -count=1 -timeout=45m -postgres-versions="$(POSTGRES_VERSIONS)" -container-runtime="$(CONTAINER_RUNTIME)" -parallelism="$(E2E_PARALLELISM)"

fmt:
	gofmt -w .

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

install-shikai: $(SHIKAI)

$(SHIKAI):
	mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install github.com/nicolaiort/shikai@latest

release: $(SHIKAI)
	$(SHIKAI) $(SHIKAI_ARGS)
