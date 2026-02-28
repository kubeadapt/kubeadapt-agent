package snapshot

import (
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }

func TestComputeSummary_EntityCounts(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Nodes:           make([]model.NodeInfo, 3),
		Pods:            make([]model.PodInfo, 5),
		Namespaces:      make([]model.NamespaceInfo, 2),
		Deployments:     make([]model.DeploymentInfo, 4),
		StatefulSets:    make([]model.StatefulSetInfo, 1),
		DaemonSets:      make([]model.DaemonSetInfo, 2),
		Jobs:            make([]model.JobInfo, 3),
		CronJobs:        make([]model.CronJobInfo, 1),
		CustomWorkloads: make([]model.CustomWorkloadInfo, 2),
		HPAs:            make([]model.HPAInfo, 1),
		Services:        make([]model.ServiceInfo, 4),
		Ingresses:       make([]model.IngressInfo, 2),
		PVs:             make([]model.PVInfo, 3),
		PVCs:            make([]model.PVCInfo, 2),
	}

	s := ComputeSummary(snap)

	assert.Equal(t, 3, s.NodeCount)
	assert.Equal(t, 5, s.PodCount)
	assert.Equal(t, 2, s.NamespaceCount)
	assert.Equal(t, 4, s.DeploymentCount)
	assert.Equal(t, 1, s.StatefulSetCount)
	assert.Equal(t, 2, s.DaemonSetCount)
	assert.Equal(t, 3, s.JobCount)
	assert.Equal(t, 1, s.CronJobCount)
	assert.Equal(t, 2, s.CustomWorkloadCount)
	assert.Equal(t, 1, s.HPACount)
	assert.Equal(t, 4, s.ServiceCount)
	assert.Equal(t, 2, s.IngressCount)
	assert.Equal(t, 3, s.PVCount)
	assert.Equal(t, 2, s.PVCCount)
}

func TestComputeSummary_PodPhases(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{
			{Phase: "Running"},
			{Phase: "Running"},
			{Phase: "Pending"},
			{Phase: "Failed"},
			{Phase: "Succeeded"},
		},
	}

	s := ComputeSummary(snap)

	assert.Equal(t, 2, s.RunningPodCount)
	assert.Equal(t, 1, s.PendingPodCount)
	assert.Equal(t, 1, s.FailedPodCount)
}

func TestComputeSummary_ContainerCount(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{
			{Containers: make([]model.ContainerInfo, 2)},
			{Containers: make([]model.ContainerInfo, 3)},
		},
	}

	s := ComputeSummary(snap)
	assert.Equal(t, 5, s.ContainerCount)
}

func TestComputeSummary_ResourceTotals(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Nodes: []model.NodeInfo{
			{
				CPUCapacityCores:    4.0,
				CPUAllocatable:      3.8,
				MemoryCapacityBytes: 8_000_000_000,
				MemoryAllocatable:   7_500_000_000,
				GPUCapacity:         2,
				CPUUsageCores:       ptrFloat64(1.5),
				MemoryUsageBytes:    ptrInt64(4_000_000_000),
			},
			{
				CPUCapacityCores:    8.0,
				CPUAllocatable:      7.6,
				MemoryCapacityBytes: 16_000_000_000,
				MemoryAllocatable:   15_000_000_000,
				GPUCapacity:         0,
				CPUUsageCores:       ptrFloat64(3.2),
				MemoryUsageBytes:    ptrInt64(10_000_000_000),
			},
		},
		Pods: []model.PodInfo{
			{
				Containers: []model.ContainerInfo{
					{CPURequestCores: 0.5, MemoryRequestBytes: 500_000_000, GPURequest: 1},
					{CPURequestCores: 1.0, MemoryRequestBytes: 1_000_000_000, GPURequest: 0},
				},
			},
		},
		PVs: []model.PVInfo{
			{Capacity: 100_000_000_000},
			{Capacity: 200_000_000_000},
		},
		PVCs: []model.PVCInfo{
			{RequestedBytes: 50_000_000_000},
			{RequestedBytes: 150_000_000_000},
		},
	}

	s := ComputeSummary(snap)

	assert.InDelta(t, 12.0, s.TotalCPUCapacity, 0.001)
	assert.InDelta(t, 11.4, s.TotalCPUAllocatable, 0.001)
	assert.Equal(t, int64(24_000_000_000), s.TotalMemoryCapacity)
	assert.Equal(t, int64(22_500_000_000), s.TotalMemoryAllocatable)
	assert.Equal(t, 2, s.TotalGPUCapacity)

	assert.InDelta(t, 1.5, s.TotalCPURequested, 0.001)
	assert.Equal(t, int64(1_500_000_000), s.TotalMemoryRequested)
	assert.Equal(t, 1, s.TotalGPURequested)

	assert.NotNil(t, s.TotalCPUUsage)
	assert.InDelta(t, 4.7, *s.TotalCPUUsage, 0.001)
	assert.NotNil(t, s.TotalMemoryUsage)
	assert.Equal(t, int64(14_000_000_000), *s.TotalMemoryUsage)

	assert.Equal(t, int64(300_000_000_000), s.TotalStorageCapacity)
	assert.Equal(t, int64(200_000_000_000), s.TotalStorageRequested)
}

func TestComputeSummary_MetricsAvailable(t *testing.T) {
	t.Run("no metrics", func(t *testing.T) {
		snap := &model.ClusterSnapshot{
			Nodes: []model.NodeInfo{{Name: "n1"}},
		}
		s := ComputeSummary(snap)
		assert.False(t, s.MetricsAvailable)
		assert.Nil(t, s.TotalCPUUsage)
		assert.Nil(t, s.TotalMemoryUsage)
	})

	t.Run("with metrics", func(t *testing.T) {
		snap := &model.ClusterSnapshot{
			Nodes: []model.NodeInfo{
				{Name: "n1", CPUUsageCores: ptrFloat64(1.0)},
			},
		}
		s := ComputeSummary(snap)
		assert.True(t, s.MetricsAvailable)
	})
}

func TestComputeSummary_GPUMetrics(t *testing.T) {
	gpuUtil1 := 75.0
	gpuUtil2 := 85.0
	gpuMemUsed1 := int64(20_000_000_000)
	gpuMemUsed2 := int64(30_000_000_000)
	gpuMemTotal1 := int64(80_000_000_000)
	gpuMemTotal2 := int64(80_000_000_000)

	snap := &model.ClusterSnapshot{
		Nodes: []model.NodeInfo{
			{
				Name:                  "gpu-node-1",
				GPUCapacity:           2,
				GPUUtilizationPercent: &gpuUtil1,
				GPUMemoryUsedBytes:    &gpuMemUsed1,
				GPUMemoryTotalBytes:   &gpuMemTotal1,
			},
			{
				Name:                  "gpu-node-2",
				GPUCapacity:           2,
				GPUUtilizationPercent: &gpuUtil2,
				GPUMemoryUsedBytes:    &gpuMemUsed2,
				GPUMemoryTotalBytes:   &gpuMemTotal2,
			},
			{
				Name: "cpu-node",
			},
		},
	}

	s := ComputeSummary(snap)

	assert.True(t, s.GPUMetricsAvailable)
	require.NotNil(t, s.TotalGPUUsage)
	assert.InDelta(t, 80.0, *s.TotalGPUUsage, 0.001)
	require.NotNil(t, s.TotalGPUMemoryUsed)
	assert.Equal(t, int64(50_000_000_000), *s.TotalGPUMemoryUsed)
	require.NotNil(t, s.TotalGPUMemoryTotal)
	assert.Equal(t, int64(160_000_000_000), *s.TotalGPUMemoryTotal)
	assert.Equal(t, 4, s.TotalGPUCapacity)
}

func TestComputeSummary_NoGPUMetrics(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Nodes: []model.NodeInfo{{Name: "n1"}},
	}
	s := ComputeSummary(snap)
	assert.False(t, s.GPUMetricsAvailable)
	assert.Nil(t, s.TotalGPUUsage)
	assert.Nil(t, s.TotalGPUMemoryUsed)
	assert.Nil(t, s.TotalGPUMemoryTotal)
}

func TestComputeSummary_EmptySnapshot(t *testing.T) {
	snap := &model.ClusterSnapshot{}
	s := ComputeSummary(snap)

	assert.Equal(t, 0, s.NodeCount)
	assert.Equal(t, 0, s.PodCount)
	assert.Equal(t, 0, s.ContainerCount)
	assert.Equal(t, 0, s.RunningPodCount)
	assert.False(t, s.MetricsAvailable)
	assert.InDelta(t, 0.0, s.TotalCPUCapacity, 0.001)
	assert.Equal(t, int64(0), s.TotalMemoryCapacity)
	assert.Nil(t, s.TotalCPUUsage)
	assert.Nil(t, s.TotalMemoryUsage)
}
