package snapshot

import (
	"context"
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/internal/collector/gpu"
	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	"github.com/kubeadapt/kubeadapt-agent/internal/enrichment"
	"github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDeps() (*store.Store, *store.MetricsStore, *config.Config, *observability.Metrics, *errors.ErrorCollector) {
	s := store.NewStore()
	ms := store.NewMetricsStore()
	cfg := &config.Config{
		ClusterID:    "test-cluster-id",
		ClusterName:  "test-cluster",
		AgentVersion: "v0.1.0",
	}
	m := observability.NewMetrics()
	ec := errors.NewErrorCollector(errors.RealClock{})
	return s, ms, cfg, m, ec
}

func TestBuild_ProducesValidSnapshotWithUUID(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()
	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)

	snap := builder.Build(context.Background())

	require.NotNil(t, snap)
	assert.NotEmpty(t, snap.SnapshotID, "SnapshotID should be a UUID")
	assert.Len(t, snap.SnapshotID, 36, "UUID should be 36 chars")
	assert.Equal(t, "test-cluster-id", snap.ClusterID)
	assert.Equal(t, "test-cluster", snap.ClusterName)
	assert.Equal(t, "v0.1.0", snap.AgentVersion)
	assert.Greater(t, snap.Timestamp, int64(0))
}

func TestBuild_ReadsAllStoresCorrectly(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()

	// Populate stores with test data.
	s.Nodes.Set("n1", model.NodeInfo{Name: "n1"})
	s.Nodes.Set("n2", model.NodeInfo{Name: "n2"})
	s.Pods.Set("default/p1", model.PodInfo{Name: "p1", Namespace: "default", Phase: "Running"})
	s.Namespaces.Set("default", model.NamespaceInfo{Name: "default"})
	s.Deployments.Set("default/d1", model.DeploymentInfo{Name: "d1"})
	s.StatefulSets.Set("default/ss1", model.StatefulSetInfo{Name: "ss1"})
	s.DaemonSets.Set("default/ds1", model.DaemonSetInfo{Name: "ds1"})
	s.Jobs.Set("default/j1", model.JobInfo{Name: "j1"})
	s.CronJobs.Set("default/cj1", model.CronJobInfo{Name: "cj1"})
	s.HPAs.Set("default/hpa1", model.HPAInfo{Name: "hpa1"})
	s.Services.Set("default/svc1", model.ServiceInfo{Name: "svc1"})
	s.Ingresses.Set("default/ing1", model.IngressInfo{Name: "ing1"})
	s.PVs.Set("pv1", model.PVInfo{Name: "pv1"})
	s.PVCs.Set("default/pvc1", model.PVCInfo{Name: "pvc1"})

	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
	snap := builder.Build(context.Background())

	assert.Len(t, snap.Nodes, 2)
	assert.Len(t, snap.Pods, 1)
	assert.Len(t, snap.Namespaces, 1)
	assert.Len(t, snap.Deployments, 1)
	assert.Len(t, snap.StatefulSets, 1)
	assert.Len(t, snap.DaemonSets, 1)
	assert.Len(t, snap.Jobs, 1)
	assert.Len(t, snap.CronJobs, 1)
	assert.Len(t, snap.HPAs, 1)
	assert.Len(t, snap.Services, 1)
	assert.Len(t, snap.Ingresses, 1)
	assert.Len(t, snap.PVs, 1)
	assert.Len(t, snap.PVCs, 1)

	// Summary should reflect the store data.
	assert.Equal(t, 2, snap.Summary.NodeCount)
	assert.Equal(t, 1, snap.Summary.PodCount)
}

func TestBuild_MergesNodeMetrics(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()

	s.Nodes.Set("n1", model.NodeInfo{Name: "n1", CPUCapacityCores: 4.0})
	s.Nodes.Set("n2", model.NodeInfo{Name: "n2", CPUCapacityCores: 8.0})

	ms.NodeMetrics.Set("n1", model.NodeMetrics{Name: "n1", CPUUsageCores: 1.5, MemoryUsageBytes: 4_000_000_000})
	// n2 has no metrics — should remain nil.

	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
	snap := builder.Build(context.Background())

	// Find n1 and n2 in the snapshot.
	var n1, n2 *model.NodeInfo
	for i := range snap.Nodes {
		switch snap.Nodes[i].Name {
		case "n1":
			n1 = &snap.Nodes[i]
		case "n2":
			n2 = &snap.Nodes[i]
		}
	}

	require.NotNil(t, n1)
	require.NotNil(t, n1.CPUUsageCores)
	assert.InDelta(t, 1.5, *n1.CPUUsageCores, 0.001)
	require.NotNil(t, n1.MemoryUsageBytes)
	assert.Equal(t, int64(4_000_000_000), *n1.MemoryUsageBytes)

	require.NotNil(t, n2)
	assert.Nil(t, n2.CPUUsageCores, "n2 should have no CPU metrics")
	assert.Nil(t, n2.MemoryUsageBytes, "n2 should have no memory metrics")
}

func TestBuild_MergesPodContainerMetrics(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()

	s.Pods.Set("default/p1", model.PodInfo{
		Name:      "p1",
		Namespace: "default",
		Phase:     "Running",
		Containers: []model.ContainerInfo{
			{Name: "app", CPURequestCores: 0.5},
			{Name: "sidecar", CPURequestCores: 0.1},
		},
	})

	ms.PodMetrics.Set("default/p1", model.PodMetrics{
		Name:      "p1",
		Namespace: "default",
		Containers: []model.ContainerMetrics{
			{Name: "app", CPUUsageCores: 0.3, MemoryUsageBytes: 200_000_000},
			{Name: "sidecar", CPUUsageCores: 0.05, MemoryUsageBytes: 50_000_000},
		},
	})

	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
	snap := builder.Build(context.Background())

	require.Len(t, snap.Pods, 1)
	pod := snap.Pods[0]
	require.Len(t, pod.Containers, 2)

	for _, c := range pod.Containers {
		switch c.Name {
		case "app":
			require.NotNil(t, c.CPUUsageCores)
			assert.InDelta(t, 0.3, *c.CPUUsageCores, 0.001)
			require.NotNil(t, c.MemoryUsageBytes)
			assert.Equal(t, int64(200_000_000), *c.MemoryUsageBytes)
		case "sidecar":
			require.NotNil(t, c.CPUUsageCores)
			assert.InDelta(t, 0.05, *c.CPUUsageCores, 0.001)
			require.NotNil(t, c.MemoryUsageBytes)
			assert.Equal(t, int64(50_000_000), *c.MemoryUsageBytes)
		}
	}
}

func TestBuild_SummaryCountsMatchSliceLengths(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()

	s.Nodes.Set("n1", model.NodeInfo{Name: "n1"})
	s.Pods.Set("default/p1", model.PodInfo{
		Name: "p1", Namespace: "default", Phase: "Running",
		Containers: []model.ContainerInfo{{Name: "c1"}, {Name: "c2"}},
	})
	s.Pods.Set("default/p2", model.PodInfo{
		Name: "p2", Namespace: "default", Phase: "Pending",
		Containers: []model.ContainerInfo{{Name: "c1"}},
	})
	s.Deployments.Set("default/d1", model.DeploymentInfo{Name: "d1"})

	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
	snap := builder.Build(context.Background())

	assert.Equal(t, len(snap.Nodes), snap.Summary.NodeCount)
	assert.Equal(t, len(snap.Pods), snap.Summary.PodCount)
	assert.Equal(t, len(snap.Deployments), snap.Summary.DeploymentCount)
	assert.Equal(t, 3, snap.Summary.ContainerCount)
	assert.Equal(t, 1, snap.Summary.RunningPodCount)
	assert.Equal(t, 1, snap.Summary.PendingPodCount)
}

func TestBuild_SummaryResourceTotals(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()

	s.Nodes.Set("n1", model.NodeInfo{
		Name:                "n1",
		CPUCapacityCores:    4.0,
		CPUAllocatable:      3.8,
		MemoryCapacityBytes: 8_000_000_000,
		MemoryAllocatable:   7_500_000_000,
		GPUCapacity:         1,
	})
	s.Pods.Set("default/p1", model.PodInfo{
		Name: "p1", Namespace: "default", Phase: "Running",
		Containers: []model.ContainerInfo{
			{Name: "c1", CPURequestCores: 0.5, MemoryRequestBytes: 500_000_000, GPURequest: 1},
		},
	})
	s.PVs.Set("pv1", model.PVInfo{Name: "pv1", Capacity: 100_000_000_000})
	s.PVCs.Set("default/pvc1", model.PVCInfo{Name: "pvc1", RequestedBytes: 50_000_000_000})

	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
	snap := builder.Build(context.Background())

	assert.InDelta(t, 4.0, snap.Summary.TotalCPUCapacity, 0.001)
	assert.InDelta(t, 3.8, snap.Summary.TotalCPUAllocatable, 0.001)
	assert.Equal(t, int64(8_000_000_000), snap.Summary.TotalMemoryCapacity)
	assert.Equal(t, int64(7_500_000_000), snap.Summary.TotalMemoryAllocatable)
	assert.Equal(t, 1, snap.Summary.TotalGPUCapacity)
	assert.InDelta(t, 0.5, snap.Summary.TotalCPURequested, 0.001)
	assert.Equal(t, int64(500_000_000), snap.Summary.TotalMemoryRequested)
	assert.Equal(t, 1, snap.Summary.TotalGPURequested)
	assert.Equal(t, int64(100_000_000_000), snap.Summary.TotalStorageCapacity)
	assert.Equal(t, int64(50_000_000_000), snap.Summary.TotalStorageRequested)
}

func TestBuild_MetricsAvailableFlag(t *testing.T) {
	t.Run("no metrics", func(t *testing.T) {
		s, ms, cfg, m, ec := newTestDeps()
		s.Nodes.Set("n1", model.NodeInfo{Name: "n1"})
		pipeline := enrichment.NewPipeline(m)
		builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
		snap := builder.Build(context.Background())
		assert.False(t, snap.Summary.MetricsAvailable)
	})

	t.Run("with metrics", func(t *testing.T) {
		s, ms, cfg, m, ec := newTestDeps()
		s.Nodes.Set("n1", model.NodeInfo{Name: "n1"})
		ms.NodeMetrics.Set("n1", model.NodeMetrics{Name: "n1", CPUUsageCores: 1.0, MemoryUsageBytes: 1_000_000})
		pipeline := enrichment.NewPipeline(m)
		builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
		snap := builder.Build(context.Background())
		assert.True(t, snap.Summary.MetricsAvailable)
	})
}

func TestBuild_EmptyStores(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()
	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
	snap := builder.Build(context.Background())

	assert.NotEmpty(t, snap.SnapshotID)
	assert.Equal(t, 0, snap.Summary.NodeCount)
	assert.Equal(t, 0, snap.Summary.PodCount)
	assert.Equal(t, 0, snap.Summary.ContainerCount)
	assert.False(t, snap.Summary.MetricsAvailable)
	assert.Nil(t, snap.Summary.TotalCPUUsage)
	assert.Nil(t, snap.Summary.TotalMemoryUsage)
}

// --- GPU merge tests ---

type mockGPUProvider struct {
	metrics []gpu.GPUDeviceMetrics
}

func (m *mockGPUProvider) GetGPUMetrics() []gpu.GPUDeviceMetrics {
	return m.metrics
}

func TestBuild_MergesGPUNodeMetrics(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()

	s.Nodes.Set("n1", model.NodeInfo{Name: "n1", GPUCapacity: 2})
	s.Nodes.Set("n2", model.NodeInfo{Name: "n2", GPUCapacity: 0})

	util1 := 75.0
	util2 := 85.0
	memUsed1 := int64(20_000_000_000)
	memUsed2 := int64(30_000_000_000)
	memTotal := int64(80_000_000_000)
	temp1 := 60.0
	temp2 := 72.0
	power1 := 200.0
	power2 := 250.0

	gpuMock := &mockGPUProvider{
		metrics: []gpu.GPUDeviceMetrics{
			{
				GPU: "0", UUID: "GPU-aaa", ModelName: "A100", Hostname: "n1",
				GPUUtilization: &util1, MemoryUsedBytes: &memUsed1,
				MemoryTotalBytes: &memTotal, Temperature: &temp1, PowerUsage: &power1,
			},
			{
				GPU: "1", UUID: "GPU-bbb", ModelName: "A100", Hostname: "n1",
				GPUUtilization: &util2, MemoryUsedBytes: &memUsed2,
				MemoryTotalBytes: &memTotal, Temperature: &temp2, PowerUsage: &power2,
			},
		},
	}

	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, gpuMock)
	snap := builder.Build(context.Background())

	var n1, n2 *model.NodeInfo
	for i := range snap.Nodes {
		switch snap.Nodes[i].Name {
		case "n1":
			n1 = &snap.Nodes[i]
		case "n2":
			n2 = &snap.Nodes[i]
		}
	}

	require.NotNil(t, n1)
	require.NotNil(t, n1.GPUUtilizationPercent)
	assert.InDelta(t, 80.0, *n1.GPUUtilizationPercent, 0.001)
	require.NotNil(t, n1.GPUMemoryUsedBytes)
	assert.Equal(t, int64(50_000_000_000), *n1.GPUMemoryUsedBytes)
	require.NotNil(t, n1.GPUMemoryTotalBytes)
	assert.Equal(t, int64(160_000_000_000), *n1.GPUMemoryTotalBytes)
	require.NotNil(t, n1.GPUTemperatureCelsius)
	assert.InDelta(t, 72.0, *n1.GPUTemperatureCelsius, 0.001)
	require.NotNil(t, n1.GPUPowerWatts)
	assert.InDelta(t, 450.0, *n1.GPUPowerWatts, 0.001)
	assert.Equal(t, "A100", n1.GPUModel)
	assert.Len(t, n1.GPUDevices, 2)

	require.NotNil(t, n2)
	assert.Nil(t, n2.GPUUtilizationPercent)
	assert.Nil(t, n2.GPUMemoryUsedBytes)
	assert.Nil(t, n2.GPUDevices)
}

func TestBuild_MergesGPUContainerMetrics(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()

	s.Pods.Set("default/gpu-pod", model.PodInfo{
		Name: "gpu-pod", Namespace: "default", Phase: "Running",
		Containers: []model.ContainerInfo{
			{Name: "train", GPURequest: 1},
			{Name: "sidecar"},
		},
	})

	util := 90.0
	memUsed := int64(40_000_000_000)

	gpuMock := &mockGPUProvider{
		metrics: []gpu.GPUDeviceMetrics{
			{
				GPU: "0", UUID: "GPU-ccc", Hostname: "n1",
				PodName: "gpu-pod", Namespace: "default", ContainerName: "train",
				GPUUtilization: &util, MemoryUsedBytes: &memUsed,
			},
		},
	}

	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, gpuMock)
	snap := builder.Build(context.Background())

	require.Len(t, snap.Pods, 1)
	for _, c := range snap.Pods[0].Containers {
		switch c.Name {
		case "train":
			require.NotNil(t, c.GPUUtilizationPercent)
			assert.InDelta(t, 90.0, *c.GPUUtilizationPercent, 0.001)
			require.NotNil(t, c.GPUMemoryUsedBytes)
			assert.Equal(t, int64(40_000_000_000), *c.GPUMemoryUsedBytes)
		case "sidecar":
			assert.Nil(t, c.GPUUtilizationPercent)
			assert.Nil(t, c.GPUMemoryUsedBytes)
		}
	}
}

func TestBuild_NilGPUCollector_NoError(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()
	s.Nodes.Set("n1", model.NodeInfo{Name: "n1"})
	pipeline := enrichment.NewPipeline(m)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)
	snap := builder.Build(context.Background())
	require.NotNil(t, snap)
	assert.Nil(t, snap.Nodes[0].GPUUtilizationPercent)
}

// mockEnricher implements enrichment.Enricher for testing.
type mockEnricher struct {
	name   string
	called bool
}

func (e *mockEnricher) Name() string { return e.name }
func (e *mockEnricher) Enrich(snapshot *model.ClusterSnapshot) error {
	e.called = true
	return nil
}

func TestBuild_EnrichmentPipelineRuns(t *testing.T) {
	s, ms, cfg, m, ec := newTestDeps()
	mock := &mockEnricher{name: "test-enricher"}
	pipeline := enrichment.NewPipeline(m, mock)
	builder := NewSnapshotBuilder(s, ms, cfg, m, ec, pipeline, nil)

	_ = builder.Build(context.Background())

	assert.True(t, mock.called, "enricher should have been called during Build")
}
