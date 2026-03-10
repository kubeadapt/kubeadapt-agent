# Collected Kubernetes Resources

kubeadapt-agent watches your cluster and sends a structured snapshot to the Kubeadapt platform on each collection cycle. This page documents every resource type the agent collects, why it matters for cost optimization, and whether collection is always-on or conditional.

> **Privacy**: kubeadapt-agent collects Kubernetes resource metadata only. Application data, environment variable values, secret contents, and workload payload data are never collected or transmitted.

## How Collection Works

The agent runs up to 23 collectors concurrently. On startup, it probes the cluster's API groups to detect optional capabilities. Collectors for those capabilities are registered only when the corresponding API group or exporter is present. Every snapshot cycle, all active stores are read in parallel (22 goroutines) and merged into a single `ClusterSnapshot` payload.

## Conditional Activation

Four capabilities gate optional collectors:

| Capability | Detection | Collector Activated |
|---|---|---|
| `MetricsServer` | `metrics.k8s.io` API group present | Node and Pod metrics |
| `VPA` | `autoscaling.k8s.io` API group present | VerticalPodAutoscalers |
| `Karpenter` | `karpenter.sh` API group present | NodePools |
| `GPU` | DCGM exporter pods found on GPU nodes, or static endpoints configured | GPU device metrics |

If a capability is absent, the corresponding collector is not registered and the snapshot field is omitted (or sent as an empty array).

---

## Core Resources

Always collected. These three resource types form the foundation of every analysis.

### Nodes

**API group**: `v1/nodes`

Nodes are the primary cost unit. The agent collects capacity (CPU, memory, GPU), allocatable amounts, labels (instance type, zone, region, node pool), taints, conditions, and provider ID. Provider ID is used to derive the cloud provider and region automatically.

Cost relevance: node instance type and utilization drive right-sizing recommendations and discount matching (Reserved Instances, Savings Plans).

### Pods

**API group**: `v1/pods`

Pods are collected with their full container list, resource requests and limits, owner references, scheduling status, and QoS class. Owner references are used during enrichment to link pods back to their top-level workload (Deployment, StatefulSet, DaemonSet, Job, or CronJob).

Cost relevance: request/limit ratios reveal over-provisioned containers. Pod scheduling failures surface capacity gaps.

### Namespaces

**API group**: `v1/namespaces`

Namespaces are collected with their labels and status. They provide the organizational boundary for cost attribution and quota enforcement.

Cost relevance: namespace-level cost breakdowns and quota analysis.

---

## Workloads

Always collected. These resource types describe how workloads are defined and managed.

### Deployments

**API group**: `apps/v1/deployments`

Collected with replica counts (desired, ready, available, updated), selector, template spec, and strategy. Deployments are the most common workload type and the primary target for right-sizing recommendations.

Cost relevance: replica count and container requests determine total resource consumption.

### StatefulSets

**API group**: `apps/v1/statefulsets`

Collected with replica counts, volume claim templates, and update strategy. StatefulSets typically run databases and stateful services with different scaling characteristics than Deployments.

Cost relevance: persistent storage costs and replica sizing.

### DaemonSets

**API group**: `apps/v1/daemonsets`

Collected with desired, ready, and available node counts. DaemonSets run one pod per node, so their cost scales directly with cluster size.

Cost relevance: per-node overhead that grows with cluster expansion.

### ReplicaSets (internal only)

**API group**: `apps/v1/replicasets`

> **Note**: ReplicaSets are collected internally for ownership resolution but are **not included in the snapshot payload** sent to the platform.

The agent reads ReplicaSets to resolve the ownership chain from Pod to ReplicaSet to Deployment. This enrichment step runs before the snapshot is assembled, so the platform receives pods already annotated with their top-level Deployment owner. ReplicaSet data is discarded after enrichment.

### Jobs

**API group**: `batch/v1/jobs`

Collected with completion status, parallelism, active/succeeded/failed counts, and owner reference (for CronJob-spawned jobs). Jobs represent batch workloads with distinct cost profiles.

Cost relevance: batch job resource usage and completion efficiency.

### CronJobs

**API group**: `batch/v1/cronjobs`

Collected with schedule, concurrency policy, and job template. CronJobs are the top-level owner of periodically spawned Jobs.

Cost relevance: recurring batch costs and scheduling efficiency.

### CustomWorkloads

**Store**: populated via store, no dedicated collector registered.

The `custom_workloads` field in the snapshot is reserved for future extensibility. The store exists and is read during snapshot assembly, but no collector currently populates it. The field will be empty in all current deployments.

---

## Autoscaling and Disruption

### HorizontalPodAutoscalers (HPAs)

**API group**: `autoscaling/v2/horizontalpodautoscalers`

Always collected. HPAs are collected with their target reference, min/max replica bounds, current replica count, and metric targets (CPU, memory, custom). HPA configuration directly affects how workloads scale under load.

Cost relevance: HPA bounds determine the range of possible resource consumption. Misconfigured HPAs cause over-scaling or under-scaling.

### VerticalPodAutoscalers (VPAs): conditional

**API group**: `autoscaling.k8s.io/v1/verticalpodautoscalers`

**Condition**: collected only when the `autoscaling.k8s.io` API group is present in the cluster.

VPAs are collected with their target reference, update mode, and resource policy. VPA recommendations are compared against actual usage to validate or refine right-sizing suggestions.

Cost relevance: VPA recommendations provide a cluster-native baseline for container sizing. The agent uses them to cross-validate its own analysis.

### PodDisruptionBudgets (PDBs)

**API group**: `policy/v1/poddisruptionbudgets`

Always collected. PDBs are collected with their selector, min available, and max unavailable settings. PDBs constrain how aggressively workloads can be disrupted during node consolidation.

Cost relevance: PDB constraints affect the feasibility of node consolidation and bin-packing recommendations.

---

## Network

Always collected.

### Services

**API group**: `v1/services`

Collected with type (ClusterIP, NodePort, LoadBalancer, ExternalName), selector, ports, and load balancer status. LoadBalancer services incur cloud provider costs beyond compute.

Cost relevance: LoadBalancer service count and configuration contribute to networking costs.

### Ingresses

**API group**: `networking.k8s.io/v1/ingresses`

Collected with rules, TLS configuration, and ingress class. Ingresses expose services externally and may be backed by cloud load balancers.
Ingress rules and TLS configuration references are collected as metadata only. TLS certificate contents are not read.

Cost relevance: ingress controller resource usage and associated cloud load balancer costs.

---

## Storage

Always collected.

### PersistentVolumes (PVs)

**API group**: `v1/persistentvolumes`

Collected with capacity, access modes, reclaim policy, storage class, and binding status. PVs represent provisioned storage that incurs cost regardless of actual usage.

Cost relevance: unbound or unused PVs represent wasted storage spend.

### PersistentVolumeClaims (PVCs)

**API group**: `v1/persistentvolumeclaims`

Collected with requested capacity, access modes, storage class, and binding status. PVCs link workloads to their storage.

Cost relevance: over-provisioned PVCs and orphaned claims (no owning pod) are common sources of storage waste.

### StorageClasses

**API group**: `storage.k8s.io/v1/storageclasses`

Collected with provisioner, reclaim policy, volume binding mode, and parameters. StorageClasses determine the type and cost tier of dynamically provisioned storage.

Cost relevance: storage class selection affects per-GB pricing. Expensive storage classes used for non-critical workloads are a common optimization target.

---

## Scheduling

Always collected. These resources govern how workloads compete for cluster capacity.

### PriorityClasses

**API group**: `scheduling.k8s.io/v1/priorityclasses`

Collected with priority value and preemption policy. Priority classes determine which pods are evicted first under resource pressure.

Cost relevance: priority configuration affects which workloads survive node consolidation and which are preempted.

### LimitRanges

**API group**: `v1/limitranges`

Collected with default requests, default limits, and min/max bounds per container and pod. LimitRanges enforce resource guardrails at the namespace level.

Cost relevance: LimitRange defaults affect containers that omit explicit requests/limits, which can cause silent over-provisioning.

### ResourceQuotas

**API group**: `v1/resourcequotas`

Collected with hard limits and current usage across CPU, memory, storage, and object counts. ResourceQuotas cap total consumption per namespace.

Cost relevance: quota headroom analysis reveals whether teams are under- or over-allocated at the namespace level.

---

## Cloud-Native

### NodePools (Karpenter): conditional

**API group**: `karpenter.sh/v1/nodepools`

**Condition**: collected only when the `karpenter.sh` API group is present in the cluster.

NodePools are collected with their node class reference, disruption settings, resource limits, and template spec. NodePools define the instance types and constraints Karpenter uses when provisioning nodes.

Cost relevance: NodePool configuration directly controls which instance types Karpenter selects. Suboptimal NodePool constraints prevent Karpenter from choosing cheaper instance families or spot instances.

---

## Metrics

Metrics are collected separately from resource state and merged into the snapshot before it is sent.

### Node and Pod Metrics: conditional

**API group**: `metrics.k8s.io/v1beta1`

**Condition**: collected only when the `metrics.k8s.io` API group is present (requires metrics-server).

When available, the agent collects real-time CPU and memory usage for every node and pod. These values are merged into the `NodeInfo` and `PodInfo` structs before snapshot assembly. The `MetricsAvailable` flag in the snapshot summary indicates whether this data is present.

Cost relevance: actual usage vs. requested resources is the core signal for right-sizing. Without metrics-server, recommendations rely on requests and limits alone.

### GPU Metrics (DCGM): conditional

**Condition**: collected only when DCGM exporter pods are detected on GPU nodes, or when static DCGM endpoints are configured via `KUBEADAPT_DCGM_ENDPOINTS`.

The agent scrapes NVIDIA DCGM exporter endpoints to collect per-device GPU utilization, tensor core activity, memory utilization, memory used, and memory total. These metrics are merged into node and container records. The `GPUMetricsAvailable` flag in the snapshot summary indicates whether GPU data is present.

Cost relevance: GPU instances are among the most expensive in any cloud. Low GPU utilization on expensive instance types is a high-priority optimization target.

---

## Snapshot Payload Summary

| Category | Resource | Always-On | Conditional On |
|---|---|---|---|
| Core | Nodes | Yes | |
| Core | Pods | Yes | |
| Core | Namespaces | Yes | |
| Workloads | Deployments | Yes | |
| Workloads | StatefulSets | Yes | |
| Workloads | DaemonSets | Yes | |
| Workloads | ReplicaSets | Internal only* | |
| Workloads | Jobs | Yes | |
| Workloads | CronJobs | Yes | |
| Workloads | CustomWorkloads | Reserved | |
| Autoscaling | HPAs | Yes | |
| Autoscaling | VPAs | No | `autoscaling.k8s.io` API group |
| Disruption | PDBs | Yes | |
| Network | Services | Yes | |
| Network | Ingresses | Yes | |
| Storage | PVs | Yes | |
| Storage | PVCs | Yes | |
| Storage | StorageClasses | Yes | |
| Scheduling | PriorityClasses | Yes | |
| Scheduling | LimitRanges | Yes | |
| Scheduling | ResourceQuotas | Yes | |
| Cloud-Native | NodePools | No | `karpenter.sh` API group |
| Metrics | Node/Pod metrics | No | `metrics.k8s.io` API group (metrics-server) |
| Metrics | GPU metrics | No | DCGM exporter detected or configured |

*ReplicaSets are collected and used internally for ownership resolution (Pod -> ReplicaSet -> Deployment chain). They are not included in the snapshot payload sent to the platform.
