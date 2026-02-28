package gpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dcgmOutputNewStyleMultiGPU = `# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-abc123",device="nvidia0",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="main",namespace="default",pod="myapp-xyz"} 42
DCGM_FI_DEV_GPU_UTIL{gpu="1",UUID="GPU-def456",device="nvidia1",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="",namespace="",pod=""} 15
# HELP DCGM_FI_DEV_FB_USED Framebuffer memory used (in MiB).
# TYPE DCGM_FI_DEV_FB_USED gauge
DCGM_FI_DEV_FB_USED{gpu="0",UUID="GPU-abc123",device="nvidia0",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="main",namespace="default",pod="myapp-xyz"} 32000
DCGM_FI_DEV_FB_USED{gpu="1",UUID="GPU-def456",device="nvidia1",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="",namespace="",pod=""} 1024
# HELP DCGM_FI_DEV_FB_FREE Framebuffer memory free (in MiB).
# TYPE DCGM_FI_DEV_FB_FREE gauge
DCGM_FI_DEV_FB_FREE{gpu="0",UUID="GPU-abc123",device="nvidia0",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="main",namespace="default",pod="myapp-xyz"} 49920
DCGM_FI_DEV_FB_FREE{gpu="1",UUID="GPU-def456",device="nvidia1",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="",namespace="",pod=""} 80896
# HELP DCGM_FI_DEV_FB_TOTAL Total framebuffer memory (in MiB).
# TYPE DCGM_FI_DEV_FB_TOTAL gauge
DCGM_FI_DEV_FB_TOTAL{gpu="0",UUID="GPU-abc123",device="nvidia0",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="main",namespace="default",pod="myapp-xyz"} 81920
DCGM_FI_DEV_FB_TOTAL{gpu="1",UUID="GPU-def456",device="nvidia1",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="",namespace="",pod=""} 81920
# HELP DCGM_FI_DEV_GPU_TEMP GPU temperature (in C).
# TYPE DCGM_FI_DEV_GPU_TEMP gauge
DCGM_FI_DEV_GPU_TEMP{gpu="0",UUID="GPU-abc123",device="nvidia0",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="main",namespace="default",pod="myapp-xyz"} 65
DCGM_FI_DEV_GPU_TEMP{gpu="1",UUID="GPU-def456",device="nvidia1",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="",namespace="",pod=""} 58
# HELP DCGM_FI_DEV_POWER_USAGE Power draw (in W).
# TYPE DCGM_FI_DEV_POWER_USAGE gauge
DCGM_FI_DEV_POWER_USAGE{gpu="0",UUID="GPU-abc123",device="nvidia0",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="main",namespace="default",pod="myapp-xyz"} 250.5
DCGM_FI_DEV_POWER_USAGE{gpu="1",UUID="GPU-def456",device="nvidia1",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="",namespace="",pod=""} 120.3
# HELP DCGM_FI_DEV_MIG_MODE MIG mode (0: disabled, 1: enabled).
# TYPE DCGM_FI_DEV_MIG_MODE gauge
DCGM_FI_DEV_MIG_MODE{gpu="0",UUID="GPU-abc123",device="nvidia0",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="main",namespace="default",pod="myapp-xyz"} 0
DCGM_FI_DEV_MIG_MODE{gpu="1",UUID="GPU-def456",device="nvidia1",modelName="NVIDIA A100-SXM4-80GB",Hostname="gpu-node-1",DCGM_FI_DRIVER_VERSION="535.129.03",container="",namespace="",pod=""} 0
`

func TestParseDCGMMetrics_NewStyleLabels_MultiGPU(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(dcgmOutputNewStyleMultiGPU))
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	byUUID := make(map[string]GPUDeviceMetrics)
	for _, m := range metrics {
		byUUID[m.UUID] = m
	}

	// GPU 0 — attributed to myapp-xyz
	gpu0 := byUUID["GPU-abc123"]
	assert.Equal(t, "0", gpu0.GPU)
	assert.Equal(t, "nvidia0", gpu0.Device)
	assert.Equal(t, "NVIDIA A100-SXM4-80GB", gpu0.ModelName)
	assert.Equal(t, "gpu-node-1", gpu0.Hostname)
	assert.Equal(t, "myapp-xyz", gpu0.PodName)
	assert.Equal(t, "default", gpu0.Namespace)
	assert.Equal(t, "main", gpu0.ContainerName)

	require.NotNil(t, gpu0.GPUUtilization)
	assert.InDelta(t, 42.0, *gpu0.GPUUtilization, 0.001)

	require.NotNil(t, gpu0.MemoryUsedBytes)
	assert.Equal(t, int64(32000*mibToBytes), *gpu0.MemoryUsedBytes)

	require.NotNil(t, gpu0.MemoryFreeBytes)
	assert.Equal(t, int64(49920*mibToBytes), *gpu0.MemoryFreeBytes)

	require.NotNil(t, gpu0.MemoryTotalBytes)
	assert.Equal(t, int64(81920*mibToBytes), *gpu0.MemoryTotalBytes)

	require.NotNil(t, gpu0.Temperature)
	assert.InDelta(t, 65.0, *gpu0.Temperature, 0.001)

	require.NotNil(t, gpu0.PowerUsage)
	assert.InDelta(t, 250.5, *gpu0.PowerUsage, 0.001)

	require.NotNil(t, gpu0.MIGEnabled)
	assert.False(t, *gpu0.MIGEnabled)

	// GPU 1 — unattributed (empty labels)
	gpu1 := byUUID["GPU-def456"]
	assert.Equal(t, "1", gpu1.GPU)
	assert.Equal(t, "", gpu1.PodName)
	assert.Equal(t, "", gpu1.Namespace)
	assert.Equal(t, "", gpu1.ContainerName)

	require.NotNil(t, gpu1.GPUUtilization)
	assert.InDelta(t, 15.0, *gpu1.GPUUtilization, 0.001)

	require.NotNil(t, gpu1.MemoryUsedBytes)
	assert.Equal(t, int64(1024*mibToBytes), *gpu1.MemoryUsedBytes)

	require.NotNil(t, gpu1.MemoryFreeBytes)
	assert.Equal(t, int64(80896*mibToBytes), *gpu1.MemoryFreeBytes)

	require.NotNil(t, gpu1.MIGEnabled)
	assert.False(t, *gpu1.MIGEnabled)
}

const dcgmOutputOldStyleLabels = `# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-old123",device="nvidia0",modelName="Tesla V100",Hostname="old-node",DCGM_FI_DRIVER_VERSION="450.80.02",container_name="worker",pod_namespace="ml-team",pod_name="training-abc"} 75
# HELP DCGM_FI_DEV_FB_USED Framebuffer memory used (in MiB).
# TYPE DCGM_FI_DEV_FB_USED gauge
DCGM_FI_DEV_FB_USED{gpu="0",UUID="GPU-old123",device="nvidia0",modelName="Tesla V100",Hostname="old-node",DCGM_FI_DRIVER_VERSION="450.80.02",container_name="worker",pod_namespace="ml-team",pod_name="training-abc"} 12000
`

func TestParseDCGMMetrics_OldStyleLabels(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(dcgmOutputOldStyleLabels))
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	gpu := metrics[0]
	assert.Equal(t, "GPU-old123", gpu.UUID)
	assert.Equal(t, "Tesla V100", gpu.ModelName)
	assert.Equal(t, "training-abc", gpu.PodName)
	assert.Equal(t, "ml-team", gpu.Namespace)
	assert.Equal(t, "worker", gpu.ContainerName)

	require.NotNil(t, gpu.GPUUtilization)
	assert.InDelta(t, 75.0, *gpu.GPUUtilization, 0.001)

	require.NotNil(t, gpu.MemoryUsedBytes)
	assert.Equal(t, int64(12000*mibToBytes), *gpu.MemoryUsedBytes)
}

const dcgmOutputSentinel = `# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-sent123",device="nvidia0",modelName="A100",Hostname="node1",container="",namespace="",pod=""} 18446744073709551615
# HELP DCGM_FI_DEV_FB_USED Framebuffer memory used (in MiB).
# TYPE DCGM_FI_DEV_FB_USED gauge
DCGM_FI_DEV_FB_USED{gpu="0",UUID="GPU-sent123",device="nvidia0",modelName="A100",Hostname="node1",container="",namespace="",pod=""} 32000
# HELP DCGM_FI_DEV_GPU_TEMP GPU temperature (in C).
# TYPE DCGM_FI_DEV_GPU_TEMP gauge
DCGM_FI_DEV_GPU_TEMP{gpu="0",UUID="GPU-sent123",device="nvidia0",modelName="A100",Hostname="node1",container="",namespace="",pod=""} 18446744073709551615
# HELP DCGM_FI_DEV_POWER_USAGE Power draw (in W).
# TYPE DCGM_FI_DEV_POWER_USAGE gauge
DCGM_FI_DEV_POWER_USAGE{gpu="0",UUID="GPU-sent123",device="nvidia0",modelName="A100",Hostname="node1",container="",namespace="",pod=""} 18446744073709551615
`

func TestParseDCGMMetrics_SentinelValueRejection(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(dcgmOutputSentinel))
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	gpu := metrics[0]
	assert.Nil(t, gpu.GPUUtilization, "sentinel GPU util should be nil")
	assert.Nil(t, gpu.Temperature, "sentinel temperature should be nil")
	assert.Nil(t, gpu.PowerUsage, "sentinel power usage should be nil")

	// Non-sentinel value should still be present
	require.NotNil(t, gpu.MemoryUsedBytes)
	assert.Equal(t, int64(32000*mibToBytes), *gpu.MemoryUsedBytes)
}

const dcgmOutputPROFAndUTIL = `# HELP DCGM_FI_PROF_GR_ENGINE_ACTIVE Ratio of time the graphics engine is active.
# TYPE DCGM_FI_PROF_GR_ENGINE_ACTIVE gauge
DCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu="0",UUID="GPU-prof123",device="nvidia0",modelName="A100",Hostname="node1",container="main",namespace="default",pod="ml-pod"} 0.85
# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-prof123",device="nvidia0",modelName="A100",Hostname="node1",container="main",namespace="default",pod="ml-pod"} 80
`

func TestParseDCGMMetrics_PROFMetricPreferred(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(dcgmOutputPROFAndUTIL))
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	gpu := metrics[0]
	require.NotNil(t, gpu.GPUUtilization)
	// PROF_GR_ENGINE_ACTIVE = 0.85 -> 85% (not the UTIL value of 80)
	assert.InDelta(t, 85.0, *gpu.GPUUtilization, 0.001)
}

const dcgmOutputUTILOnly = `# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-util123",device="nvidia0",modelName="Tesla T4",Hostname="node1",container="main",namespace="default",pod="inference-pod"} 55
`

func TestParseDCGMMetrics_UTILFallback(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(dcgmOutputUTILOnly))
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	gpu := metrics[0]
	require.NotNil(t, gpu.GPUUtilization)
	assert.InDelta(t, 55.0, *gpu.GPUUtilization, 0.001)
}

const dcgmOutputMIG = `# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-mig123",device="nvidia0",modelName="A100",Hostname="mig-node",GPU_I_ID="0",GPU_I_PROFILE="1g.10gb",container="train",namespace="ml",pod="mig-pod"} 90
# HELP DCGM_FI_DEV_MIG_MODE Whether MIG mode is enabled.
# TYPE DCGM_FI_DEV_MIG_MODE gauge
DCGM_FI_DEV_MIG_MODE{gpu="0",UUID="GPU-mig123",device="nvidia0",modelName="A100",Hostname="mig-node",GPU_I_ID="0",GPU_I_PROFILE="1g.10gb",container="train",namespace="ml",pod="mig-pod"} 1
`

func TestParseDCGMMetrics_MIGInstance(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(dcgmOutputMIG))
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	gpu := metrics[0]
	assert.Equal(t, "GPU-mig123", gpu.UUID)
	assert.Equal(t, "0", gpu.GPUInstanceID)
	assert.Equal(t, "1g.10gb", gpu.GPUProfile)

	require.NotNil(t, gpu.MIGEnabled)
	assert.True(t, *gpu.MIGEnabled)

	require.NotNil(t, gpu.GPUUtilization)
	assert.InDelta(t, 90.0, *gpu.GPUUtilization, 0.001)
}

const dcgmOutputEmptyLabels = `# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-empty123",device="nvidia0",modelName="A100",Hostname="node1",container="",namespace="",pod=""} 30
`

func TestParseDCGMMetrics_EmptyLabels(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(dcgmOutputEmptyLabels))
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	gpu := metrics[0]
	// Empty labels are preserved, NOT dropped
	assert.Equal(t, "", gpu.PodName)
	assert.Equal(t, "", gpu.Namespace)
	assert.Equal(t, "", gpu.ContainerName)

	require.NotNil(t, gpu.GPUUtilization)
	assert.InDelta(t, 30.0, *gpu.GPUUtilization, 0.001)
}

func TestParseDCGMMetrics_EmptyInput(t *testing.T) {
	metrics, err := ParseDCGMMetrics([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, metrics)
}

func TestParseDCGMMetrics_CaseInsensitiveUUID(t *testing.T) {
	// Test lowercase "uuid" label key
	input := `# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",uuid="GPU-lower123",device="nvidia0",modelName="A100",Hostname="node1",container="",namespace="",pod=""} 50
`
	metrics, err := ParseDCGMMetrics([]byte(input))
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	assert.Equal(t, "GPU-lower123", metrics[0].UUID)
}

func TestIsSentinel(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected bool
	}{
		{"normal value", 42.0, false},
		{"zero", 0.0, false},
		{"large but valid", 999999.0, false},
		{"sentinel uint64 max", 18446744073709551615.0, true},
		{"just above threshold", 1e15 + 1, true},
		{"at threshold", 1e15, false},
		{"negative", -1.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSentinel(tt.value))
		})
	}
}

func TestParseDCGMMetrics_MixedPROFAndUTIL_MultiGPU(t *testing.T) {
	// GPU-A has both PROF and UTIL (PROF should win)
	// GPU-B has only UTIL (UTIL should be used)
	input := `# TYPE DCGM_FI_PROF_GR_ENGINE_ACTIVE gauge
DCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu="0",UUID="GPU-A",device="nvidia0",modelName="A100",Hostname="n1",container="c1",namespace="ns1",pod="p1"} 0.75
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-A",device="nvidia0",modelName="A100",Hostname="n1",container="c1",namespace="ns1",pod="p1"} 70
DCGM_FI_DEV_GPU_UTIL{gpu="1",UUID="GPU-B",device="nvidia1",modelName="T4",Hostname="n1",container="c2",namespace="ns1",pod="p2"} 60
`
	metrics, err := ParseDCGMMetrics([]byte(input))
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	byUUID := make(map[string]GPUDeviceMetrics)
	for _, m := range metrics {
		byUUID[m.UUID] = m
	}

	// GPU-A: PROF wins (0.75 * 100 = 75%)
	gpuA := byUUID["GPU-A"]
	require.NotNil(t, gpuA.GPUUtilization)
	assert.InDelta(t, 75.0, *gpuA.GPUUtilization, 0.001)

	// GPU-B: UTIL fallback (60%)
	gpuB := byUUID["GPU-B"]
	require.NotNil(t, gpuB.GPUUtilization)
	assert.InDelta(t, 60.0, *gpuB.GPUUtilization, 0.001)
}

func TestParseSampleLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantName  string
		wantValue float64
	}{
		{
			name:      "metric with labels",
			line:      `DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-123"} 42`,
			wantOK:    true,
			wantName:  "DCGM_FI_DEV_GPU_UTIL",
			wantValue: 42,
		},
		{
			name:      "metric without labels",
			line:      `some_metric 3.14`,
			wantOK:    true,
			wantName:  "some_metric",
			wantValue: 3.14,
		},
		{
			name:      "metric with timestamp",
			line:      `DCGM_FI_DEV_GPU_UTIL{gpu="0"} 42 1234567890`,
			wantOK:    true,
			wantName:  "DCGM_FI_DEV_GPU_UTIL",
			wantValue: 42,
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:   "malformed",
			line:   "no_value_here",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok := parseSampleLine(tt.line)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantName, s.name)
				assert.InDelta(t, tt.wantValue, s.value, 0.001)
			}
		})
	}
}
