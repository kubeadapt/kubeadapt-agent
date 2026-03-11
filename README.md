# kubeadapt-agent

![Go](https://img.shields.io/badge/Go-1.26-blue)

Lightweight Kubernetes resource metrics collector agent for the [Kubeadapt](https://kubeadapt.io) platform.

- Collects Kubernetes resource state across workloads, networking, storage, and autoscaling resources
- Supports metrics-server integration for live CPU/memory usage and GPU monitoring
- Multi-cloud aware: enriches node metadata for AWS, GCP, and Azure
- Streams compressed snapshots to the Kubeadapt platform over HTTPS

## Quick Start

Install via Helm from the [kubeadapt-helm](https://github.com/kubeadapt/kubeadapt-helm) repository:

```bash
helm repo add kubeadapt https://kubeadapt.github.io/kubeadapt-helm
helm install kubeadapt-agent kubeadapt/kubeadapt-agent \
  --set agent.apiKey=<KUBEADAPT_API_KEY>
```

The agent requires a `KUBEADAPT_API_KEY` environment variable to authenticate with the Kubeadapt backend.

## Documentation

Full documentation is in the `docs/` directory:

- [Architecture](docs/architecture.md) - how the agent collects and ships metrics
- [Configuration](docs/configuration.md) - all environment variables and Helm values
- [Collected Resources](docs/collected-resources.md) - full list of collected resource types and fields
- [Health Endpoints](docs/health-endpoints.md) - liveness and readiness probes
- [Troubleshooting](docs/troubleshooting.md) - common issues and fixes
- [Development](docs/development.md) - local setup and contribution guide
- [Security](docs/security.md) - RBAC, network policies, and threat model

## Development

```bash
make build   # compile binary to bin/kubeadapt-agent
make test    # run unit tests with race detector
make lint    # run golangci-lint
make vet     # run go vet
```

E2E tests require Docker:

```bash
make test-e2e
```

## License

Apache 2.0. See [LICENSE](LICENSE).
