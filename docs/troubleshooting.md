# Troubleshooting kubeadapt-agent

This guide covers the most common issues you'll encounter running kubeadapt-agent, organized around the agent's state machine. Understanding the state machine helps you diagnose problems quickly.

## State Machine Overview

The agent moves through five states:

```
Starting --> Running --> Backoff --> Running (retry)
                |
                +--> Stopped  (terminal: auth failure)
                +--> Exiting  (terminal: agent deprecated)
```

| State | Meaning |
|-------|---------|
| `starting` | Collectors starting, informers syncing |
| `running` | Normal operation, snapshots being sent |
| `backoff` | Temporarily paused, will retry automatically |
| `stopped` | Permanently halted due to auth failure |
| `exiting` | Shutting down due to deprecated agent version |

The current state is always visible in the `/readyz` endpoint and in the agent's log output.

---

## Issue 1: Authentication Failure (401/403)

**State transition:** `running` --> `stopped` (terminal)

### Symptoms

The agent stops sending data and logs:

```
level=INFO msg="agent exiting" state=stopped reason="authentication failed"
```

The `/readyz` endpoint returns HTTP 503:

```json
{"ready": false}
```

### Cause

The backend rejected the API key. This happens when:

- `KUBEADAPT_API_KEY` is wrong, missing, or has extra whitespace
- The API key was rotated or revoked in the Kubeadapt dashboard
- The key belongs to a different organization than the cluster

### Resolution

1. Verify the API key in your Helm values or secret:

   ```bash
   kubectl get secret kubeadapt-agent-secret -n kubeadapt -o jsonpath='{.data.apiKey}' | base64 -d
   ```

2. Compare it against the key shown in the Kubeadapt dashboard under **Settings > API Keys**.

3. If the key is wrong, update the secret and restart the agent:

   ```bash
   kubectl rollout restart daemonset/kubeadapt-agent -n kubeadapt
   ```

4. Watch logs to confirm the agent transitions to `running`:

   ```bash
   kubectl logs -n kubeadapt -l app=kubeadapt-agent -f
   ```

**Note:** Once the agent reaches `stopped`, it does not retry. A pod restart is required after fixing the API key.

---

## Issue 2: Quota Exceeded (402)

**State transition:** `running` --> `backoff` (temporary)

### Symptoms

The agent pauses sending and logs:

```
level=DEBUG msg="in backoff, skipping snapshot" remaining=4m32s
```

When the backoff expires, the agent automatically retries. If the quota is still exceeded, it re-enters backoff.

### Cause

Your Kubeadapt plan's node or cluster quota has been reached. The backend responds with HTTP 402 and a `Retry-After` header indicating when to try again.

Default backoff when no `Retry-After` is provided: **5 minutes**.

### Resolution

1. Check your current plan usage in the Kubeadapt dashboard under **Billing > Usage**.

2. If you've exceeded your plan limits, upgrade your plan or remove unused clusters.

3. The agent recovers automatically once the quota resets or the plan is upgraded. No restart needed.

4. To confirm recovery, watch for the state to return to `running`:

   ```bash
   kubectl logs -n kubeadapt -l app=kubeadapt-agent -f | grep "state="
   ```

---

## Issue 3: Rate Limited (429)

**State transition:** `running` --> `backoff` (temporary)

### Symptoms

The agent pauses and logs:

```
level=DEBUG msg="in backoff, skipping snapshot" remaining=28s
```

### Cause

The agent is sending snapshots too frequently relative to the backend's rate limits. This is uncommon with default settings but can happen if `KUBEADAPT_SNAPSHOT_INTERVAL` is set very low.

Default backoff when no `Retry-After` is provided: **30 seconds**.

### Resolution

1. Check your snapshot interval configuration:

   ```bash
   kubectl get configmap kubeadapt-agent-config -n kubeadapt -o yaml | grep SNAPSHOT_INTERVAL
   ```

2. The recommended minimum interval is `60s`. Increase it if you've set it lower:

   ```yaml
   env:
     - name: KUBEADAPT_SNAPSHOT_INTERVAL
       value: "60s"
   ```

3. The agent recovers automatically after the backoff expires. No restart needed.

---

## Issue 4: Agent Deprecated (410)

**State transition:** `running` --> `exiting` (terminal)

### Symptoms

The agent shuts down and logs:

```
level=INFO msg="agent exiting" state=exiting reason="agent deprecated"
```

The pod exits cleanly (exit code 0). Kubernetes restarts it, and it exits again immediately.

### Cause

The backend has rejected this agent version as too old. HTTP 410 is a permanent signal that the agent must be upgraded before it can send data again.

### Resolution

1. Check the currently running agent version:

   ```bash
   kubectl get pods -n kubeadapt -l app=kubeadapt-agent -o jsonpath='{.items[0].spec.containers[0].image}'
   ```

2. Upgrade to the latest version via Helm:

   ```bash
   helm repo update
   helm upgrade kubeadapt-agent kubeadapt/kubeadapt-agent -n kubeadapt
   ```

3. After upgrading, confirm the agent reaches `running` state:

   ```bash
   kubectl rollout status daemonset/kubeadapt-agent -n kubeadapt
   kubectl logs -n kubeadapt -l app=kubeadapt-agent -f
   ```

**Note:** Like `stopped`, the `exiting` state is terminal. The agent will not retry with the same binary. Upgrade is required.

---

## Issue 5: Partial Data (Informer Sync Timeout)

**State transition:** `starting` continues to `running` with partial data

### Symptoms

During startup, the agent logs a warning instead of the normal sync completion message:

```
level=WARN msg="informer sync incomplete, continuing with partial data" error="context deadline exceeded" timeout=5m0s elapsed=5m0s
```

The agent continues running but some resource types may be missing from snapshots.

After sync, the agent logs store counts. Low or zero counts for certain resource types indicate which collectors didn't sync:

```
level=INFO msg="post-sync store counts" nodes=12 pods=87 namespaces=8 deployments=15 statefulsets=2 daemonsets=4 jobs=0 cronjobs=0 hpas=3 services=22 ingresses=5 pvs=8 pvcs=12
```

### Cause

The Kubernetes API server was slow to respond during the initial informer sync. The default timeout is 5 minutes. Large clusters or API server load spikes can cause this.

The agent deliberately continues with partial data rather than failing completely.

### Resolution

1. Check if the Kubernetes API server is under load:

   ```bash
   kubectl get --raw /healthz
   kubectl top nodes
   ```

2. Increase the sync timeout if your cluster is large:

   ```yaml
   env:
     - name: KUBEADAPT_INFORMER_SYNC_TIMEOUT
       value: "10m"
   ```

3. Restart the agent to trigger a fresh sync attempt:

   ```bash
   kubectl rollout restart daemonset/kubeadapt-agent -n kubeadapt
   ```

4. After restart, watch for the success message:

   ```
   level=INFO msg="informer sync completed" elapsed=45.2s
   ```

---

## Issue 6: Backend Unreachable

**Error code:** `BACKEND_UNREACHABLE`

### Symptoms

The agent logs repeated send failures:

```
level=ERROR msg="snapshot send failed" error="transport: HTTP request failed: ..."
```

The agent keeps retrying (with exponential backoff between attempts) but the state stays `running`. The error is recorded in the agent's health data and visible via `/debug/snapshot` under `health.error_codes`.

### Cause

The agent can't reach the Kubeadapt backend. Common causes:

- DNS resolution failure for the backend URL
- Firewall or network policy blocking egress on port 443
- The backend URL is misconfigured (`KUBEADAPT_BACKEND_URL`)
- Temporary network partition

### Resolution

1. Verify the backend URL is correct:

   ```bash
   kubectl get configmap kubeadapt-agent-config -n kubeadapt -o yaml | grep BACKEND_URL
   ```

2. Test connectivity from inside the cluster:

   ```bash
   kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n kubeadapt -- \
     curl -v https://api.kubeadapt.io/healthz
   ```

3. Check for network policies that might block egress:

   ```bash
   kubectl get networkpolicies -n kubeadapt
   ```

4. If your cluster requires an HTTP proxy, set it via standard environment variables:

   ```yaml
   env:
     - name: HTTPS_PROXY
       value: "http://proxy.example.com:3128"
     - name: NO_PROXY
       value: "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
   ```

The agent retries automatically. Once connectivity is restored, it resumes normal operation without a restart.

---

## Issue 7: Memory Pressure

**Error code:** `BUFFER_FULL` (if snapshot build fails due to OOM)

### Symptoms

The agent logs a warning when memory usage exceeds the configured threshold:

```
level=WARN msg="memory pressure detected, triggering callback"
```

This triggers a GC cycle. If memory pressure persists, the agent may be OOMKilled by Kubernetes.

### Cause

The agent uses `automemlimit` to set `GOMEMLIMIT` based on the container's memory limit. Memory pressure is detected when in-use memory exceeds 80% of `GOMEMLIMIT`.

Large clusters with many pods, nodes, or workloads increase snapshot size and memory usage.

### Resolution

1. Check current memory usage:

   ```bash
   kubectl top pods -n kubeadapt -l app=kubeadapt-agent
   ```

2. Increase the memory limit in your Helm values:

   ```yaml
   resources:
     limits:
       memory: "256Mi"
     requests:
       memory: "128Mi"
   ```

3. If the agent is being OOMKilled, check events:

   ```bash
   kubectl describe pod -n kubeadapt -l app=kubeadapt-agent | grep -A5 OOMKilled
   ```

4. For very large clusters (1000+ pods), consider increasing the limit to 512Mi or more.

---

## Checking Agent Health

### Readiness endpoint

```bash
kubectl exec -n kubeadapt <pod-name> -- wget -qO- http://localhost:8080/readyz
```

Returns `{"ready": true}` when the agent has completed initial sync and is actively sending snapshots. Returns HTTP 503 with `{"ready": false}` otherwise.

### Liveness endpoint

```bash
kubectl exec -n kubeadapt <pod-name> -- wget -qO- http://localhost:8080/healthz
```

Always returns `{"status": "ok"}` as long as the process is running.

### Debug endpoints

Debug endpoints require `KUBEADAPT_DEBUG_ENDPOINTS=true`. They're disabled by default.

**Store item counts** -- shows how many objects each informer has cached:

```bash
kubectl exec -n kubeadapt <pod-name> -- wget -qO- http://localhost:8080/debug/store
```

Example output:

```json
{
  "nodes": 12,
  "pods": 87,
  "deployments": 15,
  "services": 22
}
```

Zero counts for a resource type indicate that informer didn't sync. This is the fastest way to diagnose partial data issues.

**Latest snapshot** -- returns the full cluster snapshot the agent last built:

```bash
kubectl exec -n kubeadapt <pod-name> -- wget -qO- http://localhost:8080/debug/snapshot
```

The `health` field in the snapshot contains:

- `state` and `state_reason` -- current agent state
- `error_codes` -- active error codes (e.g., `BACKEND_UNREACHABLE`, `INFORMER_SYNC_TIMEOUT`)
- `snapshots_sent_total`, `snapshots_failed_total` -- cumulative counters
- `informers_synced`, `informers_healthy`, `informers_total` -- informer health
- `uptime_seconds` -- how long the agent has been running

### Prometheus metrics

```bash
kubectl exec -n kubeadapt <pod-name> -- wget -qO- http://localhost:8080/metrics
```

Key metrics to watch:

| Metric | Description |
|--------|-------------|
| `kubeadapt_snapshot_send_total{result="success"}` | Successful sends |
| `kubeadapt_snapshot_send_total{result="error"}` | Failed sends |
| `kubeadapt_snapshot_send_duration_seconds` | Send latency histogram |
| `kubeadapt_transport_retries_total` | Retry count (rising = connectivity issues) |

---

## How to File a Bug Report

If you've worked through this guide and the agent is still misbehaving, open an issue at [github.com/kubeadapt/kubeadapt-helm](https://github.com/kubeadapt/kubeadapt-helm/issues) with the following information.

### Required information

**1. Agent version**

```bash
kubectl get pods -n kubeadapt -l app=kubeadapt-agent -o jsonpath='{.items[0].spec.containers[0].image}'
```

**2. Cluster info**

```bash
kubectl version --short
kubectl get nodes -o wide | head -5
```

**3. Agent logs (last 200 lines)**

```bash
kubectl logs -n kubeadapt -l app=kubeadapt-agent --tail=200
```

**4. Pod events**

```bash
kubectl describe pod -n kubeadapt -l app=kubeadapt-agent
```

**5. Debug snapshot** (if debug endpoints are enabled)

```bash
kubectl exec -n kubeadapt <pod-name> -- wget -qO- http://localhost:8080/debug/snapshot | python3 -m json.tool
```

**6. Active error codes from the snapshot**

Look for the `health.error_codes` field in the debug snapshot output. Include the full list.

**7. Helm values** (redact the API key)

```bash
helm get values kubeadapt-agent -n kubeadapt
```

### What to redact

Before sharing logs or values, remove or replace:

- `KUBEADAPT_API_KEY` -- replace with `[REDACTED]`
- Any internal hostnames or IP addresses you don't want public
- Node names if your naming convention reveals internal infrastructure details
