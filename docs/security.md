# Security Model

kubeadapt-agent is designed with a minimal-privilege, read-only security posture. It collects Kubernetes resource state and sends it to the Kubeadapt backend. It never modifies cluster resources.

---

## Kubernetes RBAC

### ServiceAccount

The Helm chart creates a dedicated `ServiceAccount` for the agent. All cluster access runs under this identity. The ServiceAccount name follows the pattern `<release-name>-agent`.

### ClusterRole: read-only list and watch

The agent's ClusterRole grants only `list` and `watch` verbs. No `get`, `create`, `update`, `delete`, or `patch` permissions exist at the cluster level.

| API Group | Resources | Verbs |
|---|---|---|
| `""` (core) | nodes, namespaces, persistentvolumes, services | list, watch |
| `""` (core) | pods, persistentvolumeclaims, resourcequotas, limitranges | list, watch |
| `apps` | deployments, replicasets, statefulsets, daemonsets | list, watch |
| `batch` | jobs, cronjobs | list, watch |
| `autoscaling` | horizontalpodautoscalers | list, watch |
| `policy` | poddisruptionbudgets | list, watch |
| `networking.k8s.io` | ingresses, networkpolicies | list, watch |
| `storage.k8s.io` | storageclasses | list, watch |
| `scheduling.k8s.io` | priorityclasses | list, watch |
| `metrics.k8s.io` | pods, nodes | list, watch (requires metrics-server) |
| `autoscaling.k8s.io` | verticalpodautoscalers | list, watch (optional, VPA only) |
| `karpenter.sh` | nodepools, nodeclaims | list, watch (optional, Karpenter only) |

The optional resources (metrics-server, VPA, Karpenter) are only collected when the corresponding API group is detected at startup. If the group is absent, the collector is skipped entirely.

### 3-Phase Capability Check

Before collecting any optional resource, the agent runs a 3-phase check (`internal/discovery/rbac.go`):

1. **API group exists** — queries `ServerGroups` to confirm the group is registered with the cluster.
2. **Resource exists** — queries `ServerResourcesForGroupVersion` to confirm the specific resource type is present.
3. **RBAC allows list and watch** — issues a `SelfSubjectAccessReview` for each verb (`list`, `watch`) against the agent's own ServiceAccount.

If any phase fails, the resource is silently skipped. No error is raised for missing optional capabilities. Errors are only returned for unexpected failures such as network issues.

```go
// Phase 3: Verify RBAC allows list+watch.
canAccess, err := CanListWatch(ctx, client, group, resource)
```

This means the agent self-validates its own permissions at startup. If the ClusterRole is misconfigured, the affected collector is disabled rather than crashing.

### ClusterRoleBinding

A `ClusterRoleBinding` binds the ClusterRole to the agent's ServiceAccount in the release namespace.

---

## Authentication to the Backend

Every HTTP request the agent sends to the Kubeadapt backend includes an `Authorization: Bearer <token>` header. This is implemented in `internal/transport/middleware.go`:

```go
req.Header.Set("Authorization", "Bearer "+a.token)
```

The token is the API key set via `KUBEADAPT_API_KEY`. It's required at startup — the agent exits immediately if the key is missing.

Authentication failures (HTTP 401 or 403) are not retried. The retry transport explicitly skips auth errors:

```go
// It does NOT retry on 401/403 (auth failures).
```

---

## TLS Enforcement

The backend URL must use `https://` by default. The config validator (`internal/config/validate.go`) rejects any non-HTTPS URL at startup:

```go
if !c.AllowInsecure && !strings.HasPrefix(c.BackendURL, "https://") {
    return fmt.Errorf("config: KUBEADAPT_BACKEND_URL must use https://...")
}
```

The default backend URL is `https://api.kubeadapt.io`.

To allow an HTTP URL (local development only), set:

```
KUBEADAPT_ALLOW_INSECURE=true
```

This flag should never be set in production.

---

## Container Security

### Distroless Runtime Image

The runtime stage uses `gcr.io/distroless/static-debian12` (`Dockerfile` line 31). Distroless images contain only the application binary and its runtime dependencies. There is no shell, no package manager, and no OS utilities. This significantly reduces the attack surface compared to a standard base image.

### Non-Root User

The container runs as `nonroot:nonroot` (`Dockerfile` line 39):

```dockerfile
USER nonroot:nonroot
```

The agent process never runs as root inside the container.

### Read-Only Filesystem

The binary is copied to `/` in the runtime image. No writable paths are required at runtime. The agent holds all state in memory.

### Debug Endpoints

pprof and debug endpoints on the health port are disabled by default. Enable only for local debugging:

```
KUBEADAPT_DEBUG_ENDPOINTS=true
```

Never enable this in production.

---

## Secret Scanning

The repository uses [Lefthook](https://github.com/evilmartians/lefthook) for pre-commit hooks. TruffleHog runs on every commit to scan for verified secrets before they reach the repository:

```yaml
trufflehog:
  run: trufflehog git file://. --since-commit HEAD --only-verified --fail
```

The `--only-verified` flag means TruffleHog only fails on secrets it can confirm are active, reducing false positives.

---

## Summary

| Property | Value |
|---|---|
| K8s access | Read-only (list, watch only) |
| Auth to backend | Bearer token (API key) |
| Transport | HTTPS required by default |
| Runtime image | distroless/static-debian12 |
| Container user | nonroot:nonroot |
| Debug endpoints | Disabled by default |
| Secret scanning | TruffleHog on pre-commit |
