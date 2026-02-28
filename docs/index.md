# Service Documentation

## Overview

This is a Go microservice running on the KubeAdapt platform.

## Development

### Prerequisites

- Go 1.25+
- Docker

### Running Locally

```bash
go run ./cmd/server
```

### Running Tests

```bash
go test ./...
```

## Deployment

This service is deployed via ArgoCD with Argo Rollouts canary strategy.

- **Build**: GitHub Actions builds and pushes to ECR on merge to `main`
- **Deploy**: Helm chart image tag is updated, ArgoCD syncs automatically
