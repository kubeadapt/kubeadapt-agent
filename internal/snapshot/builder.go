package snapshot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kubeadapt/kubeadapt-agent/internal/collector/gpu"
	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	"github.com/kubeadapt/kubeadapt-agent/internal/enrichment"
	"github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// GPUMetricsProvider abstracts GPU metrics retrieval for testability.
type GPUMetricsProvider interface {
	GetGPUMetrics() []gpu.GPUDeviceMetrics
}

// SnapshotBuilder reads all stores, merges metrics, runs enrichment,
// computes summary, and returns a complete ClusterSnapshot.
type SnapshotBuilder struct {
	store          *store.Store
	metricsStore   *store.MetricsStore
	config         *config.Config
	metrics        *observability.Metrics
	errorCollector *errors.ErrorCollector
	pipeline       *enrichment.Pipeline
	gpuCollector   GPUMetricsProvider
}

// NewSnapshotBuilder creates a SnapshotBuilder with all required dependencies.
func NewSnapshotBuilder(
	store *store.Store,
	metricsStore *store.MetricsStore,
	cfg *config.Config,
	metrics *observability.Metrics,
	errCollector *errors.ErrorCollector,
	pipeline *enrichment.Pipeline,
	gpuCollector GPUMetricsProvider,
) *SnapshotBuilder {
	return &SnapshotBuilder{
		store:          store,
		metricsStore:   metricsStore,
		config:         cfg,
		metrics:        metrics,
		errorCollector: errCollector,
		pipeline:       pipeline,
		gpuCollector:   gpuCollector,
	}
}

// Build reads all stores concurrently, merges metrics, runs enrichment,
// computes summary, and returns the complete snapshot.
func (b *SnapshotBuilder) Build(ctx context.Context) *model.ClusterSnapshot {
	start := time.Now()

	snap := &model.ClusterSnapshot{}

	// Step 1: Read all TypedStores concurrently.
	replicaSets := b.readStores(snap)

	// Step 2: Read metrics.
	nodeMetrics := b.metricsStore.NodeMetrics.Values()
	podMetrics := b.metricsStore.PodMetrics.Values()

	// Step 3: Merge metrics into nodes and pods.
	mergeNodeMetrics(snap.Nodes, nodeMetrics)
	mergePodMetrics(snap.Pods, podMetrics)

	// Step 3b: Merge GPU metrics (from dcgm-exporter collector).
	if b.gpuCollector != nil {
		gpuMetrics := b.gpuCollector.GetGPUMetrics()
		mergeGPUNodeMetrics(snap.Nodes, gpuMetrics)
		mergeGPUContainerMetrics(snap.Pods, gpuMetrics)
	}

	// Step 4: Resolve ownership (ReplicaSet → Deployment, Job → CronJob)
	// before the main pipeline so aggregation uses top-level owners.
	ownershipEnricher := enrichment.NewOwnershipEnricher(replicaSets)
	if err := ownershipEnricher.Enrich(snap); err != nil {
		slog.Warn("ownership enrichment failed", "error", err)
	}

	// Step 5: Run enrichment pipeline (aggregation, targets, mounts).
	if b.pipeline != nil {
		b.pipeline.Run(snap)
	}

	// Step 6: Compute summary.
	snap.Summary = ComputeSummary(snap)

	// Step 7: Set identity fields.
	snap.SnapshotID = uuid.New().String()
	snap.ClusterID = b.config.ClusterID
	snap.ClusterName = b.config.ClusterName
	snap.Timestamp = time.Now().UnixMilli()
	snap.AgentVersion = b.config.AgentVersion

	if len(snap.Nodes) > 0 {
		snap.Provider = deriveProvider(snap.Nodes[0].ProviderID)
		snap.Region = snap.Nodes[0].Region
	}

	// Step 8: Check for stale resources (no update in >3x snapshot interval).
	stalenessThreshold := 3 * b.config.SnapshotInterval
	now := time.Now().UnixMilli()
	for resource, lastUpdated := range b.store.LastUpdatedTimes() {
		age := time.Duration(now-lastUpdated) * time.Millisecond
		if age > stalenessThreshold {
			snap.Health.StaleResources = append(snap.Health.StaleResources, resource)
		}
	}

	// Step 9: Track build duration.
	if b.metrics != nil {
		b.metrics.SnapshotBuildDuration.Observe(time.Since(start).Seconds())
	}

	return snap
}

// readStores reads all TypedStores concurrently via a WaitGroup.
// Returns ReplicaSets separately (not part of the snapshot) for ownership resolution.
func (b *SnapshotBuilder) readStores(snap *model.ClusterSnapshot) []model.ReplicaSetInfo {
	var wg sync.WaitGroup
	wg.Add(22)
	var replicaSets []model.ReplicaSetInfo

	go func() { defer wg.Done(); snap.Nodes = b.store.Nodes.Values() }()
	go func() { defer wg.Done(); snap.Pods = b.store.Pods.Values() }()
	go func() { defer wg.Done(); snap.Namespaces = b.store.Namespaces.Values() }()
	go func() { defer wg.Done(); snap.Deployments = b.store.Deployments.Values() }()
	go func() { defer wg.Done(); snap.StatefulSets = b.store.StatefulSets.Values() }()
	go func() { defer wg.Done(); snap.DaemonSets = b.store.DaemonSets.Values() }()
	go func() { defer wg.Done(); snap.Jobs = b.store.Jobs.Values() }()
	go func() { defer wg.Done(); snap.CronJobs = b.store.CronJobs.Values() }()
	go func() { defer wg.Done(); snap.CustomWorkloads = b.store.CustomWorkloads.Values() }()
	go func() { defer wg.Done(); snap.HPAs = b.store.HPAs.Values() }()
	go func() { defer wg.Done(); snap.VPAs = b.store.VPAs.Values() }()
	go func() { defer wg.Done(); snap.PDBs = b.store.PDBs.Values() }()
	go func() { defer wg.Done(); snap.Services = b.store.Services.Values() }()
	go func() { defer wg.Done(); snap.Ingresses = b.store.Ingresses.Values() }()
	go func() { defer wg.Done(); snap.PVs = b.store.PVs.Values() }()
	go func() { defer wg.Done(); snap.PVCs = b.store.PVCs.Values() }()
	go func() { defer wg.Done(); snap.StorageClasses = b.store.StorageClasses.Values() }()
	go func() { defer wg.Done(); snap.PriorityClasses = b.store.PriorityClasses.Values() }()
	go func() { defer wg.Done(); snap.LimitRanges = b.store.LimitRanges.Values() }()
	go func() { defer wg.Done(); snap.ResourceQuotas = b.store.ResourceQuotas.Values() }()
	go func() { defer wg.Done(); snap.NodePools = b.store.NodePools.Values() }()
	// ReplicaSets are not included in the snapshot (internal only), but we
	// still read them for ownership resolution (ReplicaSet → Deployment chain).
	go func() { defer wg.Done(); replicaSets = b.store.ReplicaSets.Values() }()

	wg.Wait()
	return replicaSets
}

// mergeNodeMetrics sets CPU and memory usage on nodes from metrics-server data.
func mergeNodeMetrics(nodes []model.NodeInfo, metrics []model.NodeMetrics) {
	if len(metrics) == 0 {
		return
	}
	lookup := make(map[string]model.NodeMetrics, len(metrics))
	for _, m := range metrics {
		lookup[m.Name] = m
	}
	for i := range nodes {
		if m, ok := lookup[nodes[i].Name]; ok {
			cpu := m.CPUUsageCores
			mem := m.MemoryUsageBytes
			nodes[i].CPUUsageCores = &cpu
			nodes[i].MemoryUsageBytes = &mem
		}
	}
}

// mergePodMetrics sets CPU and memory usage on pod containers from metrics-server data.
func mergePodMetrics(pods []model.PodInfo, metrics []model.PodMetrics) {
	if len(metrics) == 0 {
		return
	}
	lookup := make(map[string]model.PodMetrics, len(metrics))
	for _, m := range metrics {
		key := fmt.Sprintf("%s/%s", m.Namespace, m.Name)
		lookup[key] = m
	}
	for i := range pods {
		key := fmt.Sprintf("%s/%s", pods[i].Namespace, pods[i].Name)
		pm, ok := lookup[key]
		if !ok {
			continue
		}
		// Build container metrics lookup.
		cmLookup := make(map[string]model.ContainerMetrics, len(pm.Containers))
		for _, cm := range pm.Containers {
			cmLookup[cm.Name] = cm
		}
		for j := range pods[i].Containers {
			if cm, found := cmLookup[pods[i].Containers[j].Name]; found {
				cpu := cm.CPUUsageCores
				mem := cm.MemoryUsageBytes
				pods[i].Containers[j].CPUUsageCores = &cpu
				pods[i].Containers[j].MemoryUsageBytes = &mem
			}
		}
	}
}

func mergeGPUNodeMetrics(nodes []model.NodeInfo, metrics []gpu.GPUDeviceMetrics) {
	if len(metrics) == 0 {
		return
	}

	byNode := make(map[string][]gpu.GPUDeviceMetrics)
	for _, m := range metrics {
		if m.Hostname != "" {
			byNode[m.Hostname] = append(byNode[m.Hostname], m)
		}
	}

	for i := range nodes {
		devs, ok := byNode[nodes[i].Name]
		if !ok || len(devs) == 0 {
			continue
		}

		var (
			utilSum      float64
			utilCount    int
			tensorSum    float64
			tensorCount  int
			memUtilSum   float64
			memUtilCount int
			memUsed      int64
			memTotal     int64
			maxTemp      float64
			powerSum     float64
			hasTemp      bool
			hasPower     bool
			migDetected  bool
		)

		gpuDevices := make([]model.GPUDeviceInfo, 0, len(devs))
		for _, d := range devs {
			info := model.GPUDeviceInfo{
				UUID:          d.UUID,
				DeviceIndex:   d.GPU,
				ModelName:     d.ModelName,
				MIGProfile:    d.GPUProfile,
				MIGInstanceID: d.GPUInstanceID,
			}

			if d.GPUUtilization != nil {
				v := *d.GPUUtilization
				info.UtilizationPercent = &v
				utilSum += v
				utilCount++
			}
			if d.TensorActivePercent != nil {
				v := *d.TensorActivePercent
				info.TensorActivePercent = &v
				tensorSum += v
				tensorCount++
			}
			if d.MemCopyUtilPercent != nil {
				v := *d.MemCopyUtilPercent
				info.MemoryUtilPercent = &v
				memUtilSum += v
				memUtilCount++
			}
			if d.MemoryUsedBytes != nil {
				v := *d.MemoryUsedBytes
				info.MemoryUsedBytes = &v
				memUsed += v
			}
			if d.MemoryTotalBytes != nil {
				v := *d.MemoryTotalBytes
				info.MemoryTotalBytes = &v
				memTotal += v
			}
			if d.Temperature != nil {
				v := *d.Temperature
				info.TemperatureCelsius = &v
				if v > maxTemp {
					maxTemp = v
				}
				hasTemp = true
			}
			if d.PowerUsage != nil {
				v := *d.PowerUsage
				info.PowerWatts = &v
				powerSum += v
				hasPower = true
			}
			if d.MIGEnabled != nil && *d.MIGEnabled {
				migDetected = true
			}

			gpuDevices = append(gpuDevices, info)
		}

		nodes[i].GPUDevices = gpuDevices
		if devs[0].ModelName != "" {
			nodes[i].GPUModel = devs[0].ModelName
		}
		if devs[0].DriverVersion != "" {
			nodes[i].GPUDriverVersion = devs[0].DriverVersion
		}
		nodes[i].MIGEnabled = migDetected

		if utilCount > 0 {
			avg := utilSum / float64(utilCount)
			nodes[i].GPUUtilizationPercent = &avg
		}
		if tensorCount > 0 {
			avg := tensorSum / float64(tensorCount)
			nodes[i].GPUTensorActivePercent = &avg
		}
		if memUtilCount > 0 {
			avg := memUtilSum / float64(memUtilCount)
			nodes[i].GPUMemoryUtilPercent = &avg
		}
		if memUsed > 0 {
			nodes[i].GPUMemoryUsedBytes = &memUsed
		}
		if memTotal > 0 {
			nodes[i].GPUMemoryTotalBytes = &memTotal
		}
		if hasTemp {
			nodes[i].GPUTemperatureCelsius = &maxTemp
		}
		if hasPower {
			nodes[i].GPUPowerWatts = &powerSum
		}
	}
}

func mergeGPUContainerMetrics(pods []model.PodInfo, metrics []gpu.GPUDeviceMetrics) {
	if len(metrics) == 0 {
		return
	}

	type containerKey struct {
		namespace string
		pod       string
		container string
	}
	type containerGPU struct {
		utilSum   float64
		utilCount int
		memUsed   int64
		hasUtil   bool
		hasMem    bool
	}

	lookup := make(map[containerKey]*containerGPU)
	for _, m := range metrics {
		if m.PodName == "" || m.Namespace == "" || m.ContainerName == "" {
			continue
		}
		key := containerKey{namespace: m.Namespace, pod: m.PodName, container: m.ContainerName}
		cg, ok := lookup[key]
		if !ok {
			cg = &containerGPU{}
			lookup[key] = cg
		}
		if m.GPUUtilization != nil {
			cg.utilSum += *m.GPUUtilization
			cg.utilCount++
			cg.hasUtil = true
		}
		if m.MemoryUsedBytes != nil {
			cg.memUsed += *m.MemoryUsedBytes
			cg.hasMem = true
		}
	}

	for i := range pods {
		for j := range pods[i].Containers {
			key := containerKey{
				namespace: pods[i].Namespace,
				pod:       pods[i].Name,
				container: pods[i].Containers[j].Name,
			}
			cg, ok := lookup[key]
			if !ok {
				continue
			}
			if cg.hasUtil && cg.utilCount > 0 {
				avg := cg.utilSum / float64(cg.utilCount)
				pods[i].Containers[j].GPUUtilizationPercent = &avg
			}
			if cg.hasMem {
				pods[i].Containers[j].GPUMemoryUsedBytes = &cg.memUsed
			}
		}
	}
}

func deriveProvider(providerID string) string {
	switch {
	case strings.HasPrefix(providerID, "aws://"):
		return "aws"
	case strings.HasPrefix(providerID, "gce://"):
		return "gcp"
	case strings.HasPrefix(providerID, "azure://"):
		return "azure"
	default:
		return ""
	}
}
