# kubeadapt-agent

**Lightweight Kubernetes resource metrics collector agent** — runs inside your cluster, snapshots workload state, and streams it to the Kubeadapt platform for cost analysis and right-sizing recommendations.

## What It Does

kubeadapt-agent is a single Go binary that you deploy as a Deployment in your Kubernetes cluster. Every configurable interval it:

1. Collects resource state across 22 Kubernetes resource types concurrently
2. Enriches workload ownership (resolving Pods back to their top-level controllers)
3. Compresses the snapshot with zstd and streams it to the Kubeadapt backend
4. Reports its own health so you can monitor collection reliability

The agent never buffers the full payload in memory. It pipes data through a streaming zstd encoder directly to the HTTP request body, keeping memory usage flat regardless of cluster size.

## Key Features

- **22 resource types** collected in parallel: Nodes, Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, HPAs, VPAs, PDBs, Services, Ingresses, PVs, PVCs, StorageClasses, PriorityClasses, LimitRanges, ResourceQuotas, Namespaces, custom workloads, and more
- **Metrics-server support** — when detected, collects live CPU and memory usage per Pod and Node
- **GPU monitoring** — integrates with DCGM Exporter to collect GPU utilization and memory metrics for NVIDIA workloads
- **Multi-cloud aware** — detects your cloud provider (AWS, GCP, Azure) and region automatically at startup
- **Karpenter support** — collects NodePool resources when Karpenter is present
- **VPA support** — collects VerticalPodAutoscaler resources when the VPA CRD is installed
- **Container-aware runtime** — uses `automemlimit` and `automaxprocs` to respect cgroup memory limits and CPU quotas automatically
- **Resilient state machine** — recovers from transient failures with exponential backoff before resuming collection

## Architecture Overview

```
Your Kubernetes Cluster
┌─────────────────────────────────────────────────────┐
│                                                     │
│  kubeadapt-agent (Deployment)                       │
│  ┌─────────────────────────────────────────────┐   │
│  │  Capability Detection                        │   │
│  │  (metrics-server, VPA, Karpenter, DCGM)     │   │
│  │                                              │   │
│  │  up to 23 Collectors (concurrent)               │   │
│  │  → Ownership Enrichment                      │   │
│  │  → zstd Streaming Transport                  │   │
│  └──────────────────────┬──────────────────────┘   │
│                         │ HTTPS                     │
└─────────────────────────┼───────────────────────────┘
                          │
                          ▼
              Kubeadapt Platform API
```

At startup the agent detects which optional capabilities your cluster has (metrics-server, VPA, Karpenter, DCGM Exporter) and enables the corresponding collectors automatically. No manual configuration needed for capability detection.

## Quick Start

Install via the official Helm chart at [github.com/kubeadapt/kubeadapt-helm](https://github.com/kubeadapt/kubeadapt-helm).

```bash
helm repo add kubeadapt https://kubeadapt.github.io/kubeadapt-helm
helm repo update

helm install kubeadapt-agent kubeadapt/kubeadapt-agent \
  --namespace kubeadapt \
  --create-namespace \
  --set agent.apiKey=<YOUR_API_KEY> \
  --set agent.backendURL=<YOUR_BACKEND_URL>
```

The Helm chart handles RBAC, ServiceAccount, and all required permissions. See the [kubeadapt-helm repository](https://github.com/kubeadapt/kubeadapt-helm) for the full values reference and upgrade instructions.

## Startup Behavior

When the agent starts, it logs its key configuration and detected capabilities:

```
kubeadapt-agent starting  version=v1.x.x  backend_url=https://...  snapshot_interval=5m0s
cluster capabilities detected  metrics_server=true  vpa=false  karpenter=false  dcgm_exporter=false  provider=aws
```

This output confirms which optional collectors are active. If `metrics_server=false`, live CPU/memory usage won't be included in snapshots — only requested resources from Pod specs.

## What's Next

| Page | Description |
|------|-------------|
| [Architecture](architecture.md) | State machine, collector pipeline, transport layer, and data flow |
| [Configuration](configuration.md) | All environment variables with defaults and descriptions |
| [Collected Resources](collected-resources.md) | Full list of resource types and fields in each snapshot |
| [Health Endpoints](health-endpoints.md) | `/healthz`, `/readyz`, and Prometheus metrics endpoints |
| [Troubleshooting](troubleshooting.md) | Common issues, log patterns, and diagnostic steps |
| [Development](development.md) | Building, testing, and running the agent locally |
| [Security](security.md) | RBAC permissions, network requirements, and security posture |
