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
