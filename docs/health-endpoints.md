# Health Server Endpoints

The agent runs an HTTP server that exposes health, readiness, Prometheus metrics, and optional debug endpoints. All endpoints share a single port.

## Port

**Default: `8080`**

Set `KUBEADAPT_HEALTH_PORT` to override:

```yaml
env:
  - name: KUBEADAPT_HEALTH_PORT
    value: "8080"
```

The Helm chart exposes this port as a named container port (`health`).

---

## Endpoints

### `GET /healthz`: Liveness

Always returns `200 OK`. Kubernetes uses this to decide whether to restart the container. The agent never returns a non-200 here; if the process is alive, it's healthy.

**Response**

```
HTTP/1.1 200 OK
Content-Type: application/json

{"status": "ok"}
```

---

### `GET /readyz`: Readiness

Returns `200 OK` when the agent has completed its first full sync with the Kubernetes API server. Returns `503 Service Unavailable` before that sync finishes.

During startup, the agent needs time to list all resources from the API server before it can produce a valid snapshot. The readiness probe signals that initial sync is complete and valid snapshots are being produced.

**Response — ready**

```
HTTP/1.1 200 OK
Content-Type: application/json

{"ready": true}
```

**Response — not ready**

```
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{"ready": false}
```

---

### `GET /metrics`: Prometheus Metrics

Exposes the agent's internal Prometheus metrics in the standard text exposition format. Scraped by your cluster's Prometheus instance.

**Response**

```
HTTP/1.1 200 OK
Content-Type: text/plain; version=0.0.4; charset=utf-8

# HELP ...
# TYPE ...
...
```

The Helm chart creates a `ServiceMonitor` (if Prometheus Operator is installed) or annotates the pod for scraping.

---

### `GET /debug/pprof/*`: Go pprof Profiles

**Only available when `KUBEADAPT_DEBUG_ENDPOINTS=true`.**

Exposes the standard Go `net/http/pprof` index and sub-profiles:

| Path | Description |
|------|-------------|
| `/debug/pprof/` | Profile index page |
| `/debug/pprof/cmdline` | Command line arguments |
| `/debug/pprof/profile` | 30-second CPU profile |
| `/debug/pprof/symbol` | Symbol lookup |
| `/debug/pprof/trace` | Execution trace |

Use `go tool pprof` to fetch and analyze profiles:

```bash
go tool pprof http://localhost:8080/debug/pprof/profile
```

**Never enable this in production.** pprof endpoints expose internal runtime details and add CPU overhead during profiling.

---

### `GET /debug/snapshot`: Latest Snapshot

**Only available when `KUBEADAPT_DEBUG_ENDPOINTS=true`.**

Returns the most recent cluster snapshot the agent has collected, serialized as JSON. Useful for verifying what data the agent sees before it's sent to the backend.

**Response — snapshot available**

```
HTTP/1.1 200 OK
Content-Type: application/json

{ ...snapshot JSON... }
```

**Response — no snapshot yet**

```
HTTP/1.1 204 No Content
```

Returns `204` if the agent hasn't completed its first collection cycle yet.

---

### `GET /debug/store`: Store Item Counts

**Only available when `KUBEADAPT_DEBUG_ENDPOINTS=true`.**

Returns a JSON object with the count of items currently held in the agent's in-memory store, broken down by resource type. Useful for confirming the agent is seeing the expected number of pods, nodes, deployments, etc.

**Response**

```
HTTP/1.1 200 OK
Content-Type: application/json

{
  "pods": 42,
  "nodes": 3,
  "deployments": 15,
  ...
}
```

---

## Kubernetes Probe Configuration

The Helm chart does not configure probes by default. For manual deployments, use:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 6
```

The readiness probe uses a higher `failureThreshold` and shorter `periodSeconds` because the agent needs time to sync with the Kubernetes API server on startup. Adjust `initialDelaySeconds` for large clusters where the initial list operation takes longer.

---

## Debug Endpoints

Debug endpoints are disabled by default. Enable them only in non-production environments for troubleshooting:

```yaml
env:
  - name: KUBEADAPT_DEBUG_ENDPOINTS
    value: "true"
```

When enabled, the following endpoints become available on the same port:

- `GET /debug/pprof/` and sub-paths
- `GET /debug/snapshot`
- `GET /debug/store`

**Do not enable debug endpoints in production.** They expose internal runtime state and add profiling overhead.
