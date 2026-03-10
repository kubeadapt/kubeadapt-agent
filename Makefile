VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -s -w -X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT_HASH) -X main.BuildTime=$(BUILD_TIME)

.PHONY: build test lint bench docker clean vet

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/kubeadapt-agent ./cmd/agent

test:
	go test ./... -race -count=1

vet:
	go vet ./...

lint:
	golangci-lint run

bench:
	go test ./... -bench=. -benchmem -run=^$$

docker:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT_HASH=$(COMMIT_HASH) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t ghcr.io/kubeadapt/agent:$(VERSION) .

clean:
	rm -rf bin/

# E2E Testing
E2E_AGENT_IMAGE ?= localhost/kubeadapt-agent:e2e-test
E2E_STUB_IMAGE  ?= localhost/ingestion-stub:e2e-test
E2E_TIMEOUT     ?= 30m

.PHONY: test-e2e test-e2e-build test-e2e-run

test-e2e: test-e2e-build test-e2e-run

test-e2e-build:
	@echo "→ Building agent image $(E2E_AGENT_IMAGE)"
	docker build -t $(E2E_AGENT_IMAGE) \
		--build-arg VERSION=e2e-test \
		--build-arg COMMIT_HASH=$(COMMIT_HASH) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		.
	@echo "→ Building ingestion stub image $(E2E_STUB_IMAGE)"
	docker build -t $(E2E_STUB_IMAGE) \
		-f tests/e2e/stub/Dockerfile .

test-e2e-run:
	go test -v -timeout $(E2E_TIMEOUT) ./tests/e2e/...
