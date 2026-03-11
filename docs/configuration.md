# Configuration Reference

All configuration is done through environment variables. The agent reads them at startup and fails fast if required values are missing or invalid.

The definitive source of truth is [`internal/config/config.go`](https://github.com/kubeadapt/kubeadapt-agent/blob/main/internal/config/config.go). This document is mechanically derived from that file.

---

## Core

| Variable | Description | Default | Required | Validation |
|---|---|---|---|---|
| `KUBEADAPT_API_KEY` | API key for authenticating with the Kubeadapt backend. Falls back to `KUBEADAPT_AGENT_TOKEN` if unset. | — | Yes | Must be non-empty |

### Legacy fallback names

`KUBEADAPT_API_KEY` is the canonical name. If it's not set, the agent checks `KUBEADAPT_AGENT_TOKEN` as a fallback. This exists for backward compatibility with older Helm chart versions. Set `KUBEADAPT_API_KEY` in new deployments.


---

## Intervals

**Note:** Interval values set via Helm or env vars may be overridden at runtime by the Kubeadapt platform based on your subscription tier.

All interval values accept Go duration strings (`60s`, `5m`, `1h30m`) or plain integers treated as seconds (`60` = 60 seconds).

| Variable | Description | Default | Required | Validation |
|---|---|---|---|---|
| `KUBEADAPT_SNAPSHOT_INTERVAL` | How often the agent sends a full cluster state snapshot to the backend. | `60s` | No | Must be >= 10s |
| `KUBEADAPT_METRICS_INTERVAL` | How often the agent collects resource metrics (CPU, memory, etc.). | `60s` | No | Must be >= 10s |
| `KUBEADAPT_INFORMER_RESYNC` | Kubernetes informer full resync period. Controls how often the local cache is reconciled with the API server. | `300s` | No | None |
| `KUBEADAPT_INFORMER_SYNC_TIMEOUT` | Timeout for the initial Kubernetes informer cache sync at startup. | `5m` | No | None |
| `KUBEADAPT_GPU_METRICS_INTERVAL` | How often GPU metrics are collected. Defaults to `KUBEADAPT_METRICS_INTERVAL` if unset. | Same as `KUBEADAPT_METRICS_INTERVAL` | No | None |

---

## Transport

| Variable | Description | Default | Required | Validation |
|---|---|---|---|---|
| `KUBEADAPT_COMPRESSION_LEVEL` | zstd compression level for data sent to the backend. Higher = smaller payload, more CPU. | `3` | No | Must be 1-4 |
| `KUBEADAPT_MAX_RETRIES` | Maximum retry attempts for failed backend requests. Set to `0` to disable retries. | `5` | No | Must be >= 0 |
| `KUBEADAPT_REQUEST_TIMEOUT` | HTTP request timeout for backend calls. | `30s` | No | None |
| `KUBEADAPT_BUFFER_MAX_BYTES` | Maximum in-memory buffer size in bytes. The agent uses streaming transport (io.Pipe + zstd), so this is a safety ceiling, not a pre-allocated buffer. | `52428800` (50 MB) | No | None |

---

## Health and Debug

| Variable | Description | Default | Required | Validation |
|---|---|---|---|---|
| `KUBEADAPT_HEALTH_PORT` | HTTP port for the `/healthz` and `/readyz` endpoints. | `8080` | No | Must be 1-65535 |
| `KUBEADAPT_DEBUG_ENDPOINTS` | Enable pprof and debug endpoints on the health port. Never enable in production. | `false` | No | Boolean (`true`/`false`, `1`/`0`) |
| `KUBEADAPT_DEBUG_ENDPOINTS` | Enable pprof and debug endpoints on the health port. Never enable in production. | `false` | No | Boolean (`true`/`false`, `1`/`0`) |

---

## GPU Monitoring

| Variable | Description | Default | Required | Validation |
|---|---|---|---|---|
| `KUBEADAPT_GPU_METRICS_ENABLED` | Enable GPU metrics collection via DCGM exporter. Disable if your cluster has no GPUs. | `true` | No | Boolean (`true`/`false`, `1`/`0`) |
| `KUBEADAPT_DCGM_PORT` | Port on which DCGM exporter pods expose metrics. | `9400` | No | None |
| `KUBEADAPT_DCGM_NAMESPACE` | Kubernetes namespace to search for DCGM exporter pods. Empty string means auto-detect across all namespaces. | `""` (auto-detect) | No | None |
| `KUBEADAPT_DCGM_ENDPOINTS` | Comma-separated list of DCGM exporter endpoints (e.g., `10.0.0.1:9400,10.0.0.2:9400`). Overrides auto-discovery. | `""` (auto-discover) | No | None |

---

## Kubernetes Metadata

These variables are injected automatically by the Helm chart using the Kubernetes [Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/). You don't set them manually in production.

| Variable | Description | Default | Required |
|---|---|---|---|
| `KUBEADAPT_CHART_VERSION` | Helm chart version that deployed this agent. Injected by Helm. | `""` | No |
| `HELM_RELEASE_NAME` | Helm release name. Injected by Helm. | `""` | No |
| `POD_NAME` | Name of the pod running the agent. Injected via Downward API. | `""` | No |
| `POD_NAMESPACE` | Namespace of the pod running the agent. Injected via Downward API. | `""` | No |
| `NODE_NAME` | Name of the node the agent pod is scheduled on. Injected via Downward API. | `""` | No |

---

## Version

| Variable | Description | Default | Required |
|---|---|---|---|
| `KUBEADAPT_AGENT_VERSION` | Agent version string. Used as a runtime override when the binary wasn't built with ldflags. | `""` | No |

### Version resolution precedence

The agent resolves its version using this order:

1. **Build-time ldflags** — `main.Version` injected at compile time by CI (`-ldflags "-X main.Version=v1.2.3"`). This is the normal production path.
2. **`KUBEADAPT_AGENT_VERSION` env var** — Helm chart or runtime override. Useful when running a pre-built image with a known version.
3. **`"dev"` fallback** — Used when running locally without ldflags.

---

## Validation rules

The agent calls `config.Validate()` at startup and exits immediately if any rule fails. The rules are:

- `KUBEADAPT_API_KEY` must be non-empty
- `KUBEADAPT_SNAPSHOT_INTERVAL` must be >= 10s
- `KUBEADAPT_METRICS_INTERVAL` must be >= 10s
- `KUBEADAPT_COMPRESSION_LEVEL` must be 1-4
- `KUBEADAPT_MAX_RETRIES` must be >= 0
- `KUBEADAPT_HEALTH_PORT` must be 1-65535

Invalid duration strings and non-integer values for integer fields silently fall back to their defaults rather than failing validation.

---

## Example configurations

### Minimal (required only)

```env
KUBEADAPT_API_KEY=ka_live_xxxxxxxxxxxxxxxxxxxx
```

### Production with custom intervals

```env
KUBEADAPT_API_KEY=ka_live_xxxxxxxxxxxxxxxxxxxx
KUBEADAPT_SNAPSHOT_INTERVAL=120s
KUBEADAPT_METRICS_INTERVAL=30s
KUBEADAPT_COMPRESSION_LEVEL=4
KUBEADAPT_MAX_RETRIES=3
```

### GPU cluster

```env
KUBEADAPT_API_KEY=ka_live_xxxxxxxxxxxxxxxxxxxx
KUBEADAPT_GPU_METRICS_ENABLED=true
KUBEADAPT_DCGM_PORT=9400
KUBEADAPT_DCGM_NAMESPACE=gpu-operator
KUBEADAPT_GPU_METRICS_INTERVAL=30s
```


---

## Helm values mapping

When deploying via the [kubeadapt-helm](https://github.com/kubeadapt/kubeadapt-helm) chart, most of these env vars are set through Helm values rather than directly. The chart translates values like `agent.apiKey`, `agent.snapshotInterval`, and `agent.gpuMetrics.enabled` into the corresponding environment variables in the pod spec.

See the [Helm chart values reference](https://github.com/kubeadapt/kubeadapt-helm) for the full mapping. The Kubernetes metadata variables (`POD_NAME`, `POD_NAMESPACE`, `NODE_NAME`) are always injected by the chart via the Downward API and don't need to be set manually.
