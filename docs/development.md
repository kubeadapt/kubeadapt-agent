# Development Guide

This guide covers local development setup, testing, linting, and CI for kubeadapt-agent contributors.

## Prerequisites

- **Go 1.26+** — the module requires `go 1.26.0` (see `go.mod`)
- **Docker** — required for building images and running E2E tests
- **kubectl** — required for E2E tests against a real cluster
- **golangci-lint** — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **lefthook** — `go install github.com/evilmartians/lefthook@latest`
- **commitizen** — `pip install commitizen` (enforces conventional commits)
- Access to a Kubernetes cluster (local kind/k3s works fine for E2E)

## Project Structure

```
kubeadapt-agent/
├── cmd/agent/          # Entry point — main.go
├── internal/
│   ├── agent/          # Top-level agent orchestration
│   ├── cloud/          # Cloud provider detection
│   ├── collector/      # Kubernetes resource collectors
│   ├── config/         # Configuration loading (env vars)
│   ├── convert/        # Type conversion helpers
│   ├── discovery/      # Cluster discovery
│   ├── enrichment/     # Ownership enrichment pipeline
│   ├── errors/         # Error types
│   ├── health/         # HTTP health check server
│   ├── observability/  # Prometheus metrics
│   ├── snapshot/       # Cluster state snapshot assembly
│   ├── store/          # In-memory Kubernetes object stores
│   └── transport/      # HTTP transport with zstd compression
├── pkg/model/          # Shared data models (public API)
└── tests/e2e/          # End-to-end tests (Kind cluster)
```

## Logger

kubeadapt-agent uses **`log/slog`** from the Go standard library. This is intentionally different from other Kubeadapt services, which use `go.uber.org/zap`. Don't introduce zap as a dependency here.

## Local Setup

```bash
git clone https://github.com/kubeadapt/kubeadapt-agent.git
cd kubeadapt-agent

# Install pre-commit hooks
lefthook install

# Build the binary
make build
# Output: bin/kubeadapt-agent
```

## Running Locally

Copy `.env.sample` to `.env` and fill in the required value:

```bash
cp .env.sample .env
```

The only required variable is:

| Variable | Description |
|---|---|
| `KUBEADAPT_API_KEY` | API key for authenticating with the Kubeadapt backend |

All other variables are optional with sensible defaults. Key ones:

| Variable | Default | Description |
|---|---|---|
| `KUBEADAPT_SNAPSHOT_INTERVAL` | `60s` | How often to snapshot cluster state |
| `KUBEADAPT_METRICS_INTERVAL` | `60s` | How often to collect metrics |
| `KUBEADAPT_HEALTH_PORT` | `8080` | Health check HTTP port |
| `KUBEADAPT_DEBUG_ENDPOINTS` | `false` | Enable pprof on health port |

Make sure your kubeconfig points to the target cluster, then run:

```bash
go run ./cmd/agent
```

The agent reads kubeconfig from the standard `KUBECONFIG` env var or `~/.kube/config`.

## Build Targets

```bash
make build      # Compile binary to bin/kubeadapt-agent (CGO_ENABLED=0)
make test       # Run unit tests with race detector (go test ./... -race -count=1)
make lint       # Run golangci-lint
make vet        # Run go vet
make bench      # Run benchmarks (go test ./... -bench=. -benchmem)
make docker     # Build multi-arch Docker image (linux/amd64, linux/arm64)
make clean      # Remove bin/
make test-e2e   # Build images + run E2E tests against Kind cluster
```

## Testing

### Unit Tests

```bash
make test
# Equivalent to: go test ./... -race -count=1
```

The race detector is always on. Don't disable it.

### Benchmarks

```bash
make bench
# Equivalent to: go test ./... -bench=. -benchmem -run=^$
```

### E2E Tests

E2E tests spin up a Kind cluster, deploy the agent and a backend stub, then verify end-to-end behavior.

```bash
make test-e2e
```

This runs two steps:

1. `make test-e2e-build` — builds the agent and backend stub Docker images for testing
2. `make test-e2e-run` — runs `go test -v -timeout 30m ./tests/e2e/...`

You need Docker and a working `kubectl` in `PATH`. The test framework provisions its own Kind cluster.

## Linting

```bash
make lint
# Equivalent to: golangci-lint run
```

Configuration is in `.golangci.yml` (Go 1.26, 5m timeout). Enabled linters include `errcheck`, `staticcheck`, `gosec`, `errorlint`, and others.

## Pre-commit Hooks

Install hooks once with `lefthook install`. They run in parallel on every commit:

| Hook | What it does |
|---|---|
| `trufflehog` | Scans for leaked secrets (blocks commit on match) |
| `gofmt` | Formats staged `.go` files in place |
| `goimports` | Fixes imports in staged `.go` files in place |
| `golangci-lint` | Lints new issues introduced since `HEAD~` |

The `commit-msg` hook validates your commit message against the conventional commits format using commitizen.

## Commit Convention

This repo uses [Conventional Commits](https://www.conventionalcommits.org/) enforced by commitizen.

Format: `<type>(<scope>): <description>`

Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `build`, `ci`, `perf`, `revert`

Allowed scopes: `cmd`, `docs`, `ci`, `deps`, `health`, `security`, `test`, `build`, `internal`, `api`, `pkg`

Examples:

```
feat(collector): add GPU metrics collection
fix(transport): retry on 429 with backoff
docs(internal): document enrichment pipeline
```

Use `cz commit` for an interactive prompt if you're unsure of the format.

## CI Pipeline

Two workflows run via reusable GitHub Actions:

- **`test.yml`** — triggers on pull requests to `main`. Runs the full test suite including linting and race detection.
- **`build.yml`** — triggers on pushes to `develop` and version tags (`v*`). Builds and pushes the Docker image.

Both workflows delegate to reusable workflows in `kubeadapt/public-actions`.
