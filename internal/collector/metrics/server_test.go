package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

const (
	waitTimeout  = 5 * time.Second
	pollInterval = 50 * time.Millisecond
)

// mockMetricsAPI implements MetricsAPI for testing.
type mockMetricsAPI struct {
	nodeMetrics []metricsv1beta1.NodeMetrics
	podMetrics  []metricsv1beta1.PodMetrics
	nodeErr     error
	podErr      error
}

func (m *mockMetricsAPI) ListNodeMetrics(_ context.Context) ([]metricsv1beta1.NodeMetrics, error) {
	return m.nodeMetrics, m.nodeErr
}

func (m *mockMetricsAPI) ListPodMetrics(_ context.Context) ([]metricsv1beta1.PodMetrics, error) {
	return m.podMetrics, m.podErr
}

func TestMetricsCollector_Name(t *testing.T) {
	c := NewMetricsCollector(
		&mockMetricsAPI{},
		store.NewMetricsStore(),
		observability.NewMetrics(),
		time.Minute,
	)
	assert.Equal(t, "metrics", c.Name())
}

func TestMetricsCollector_PollsAndStoresNodeMetrics(t *testing.T) {
	ts := metav1.Now()
	mock := &mockMetricsAPI{
		nodeMetrics: []metricsv1beta1.NodeMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Timestamp:  ts,
				Usage: map[corev1.ResourceName]resource.Quantity{
					"cpu":    resource.MustParse("500m"),
					"memory": resource.MustParse("2Gi"),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
				Timestamp:  ts,
				Usage: map[corev1.ResourceName]resource.Quantity{
					"cpu":    resource.MustParse("1"),
					"memory": resource.MustParse("4Gi"),
				},
			},
		},
	}

	ms := store.NewMetricsStore()
	c := NewMetricsCollector(mock, ms, observability.NewMetrics(), 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return ms.NodeMetrics.Len() == 2
	}, waitTimeout, pollInterval, "expected 2 node metrics in store")

	nm1, ok := ms.NodeMetrics.Get("node-1")
	require.True(t, ok)
	assert.Equal(t, "node-1", nm1.Name)
	assert.InDelta(t, 0.5, nm1.CPUUsageCores, 0.001)
	assert.Equal(t, int64(2*1024*1024*1024), nm1.MemoryUsageBytes)
	assert.Equal(t, ts.UnixMilli(), nm1.Timestamp)

	nm2, ok := ms.NodeMetrics.Get("node-2")
	require.True(t, ok)
	assert.Equal(t, "node-2", nm2.Name)
	assert.InDelta(t, 1.0, nm2.CPUUsageCores, 0.001)
}

func TestMetricsCollector_PollsAndStoresPodMetrics(t *testing.T) {
	ts := metav1.Now()
	mock := &mockMetricsAPI{
		podMetrics: []metricsv1beta1.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
				},
				Timestamp: ts,
				Containers: []metricsv1beta1.ContainerMetrics{
					{
						Name: "app",
						Usage: map[corev1.ResourceName]resource.Quantity{
							"cpu":    resource.MustParse("100m"),
							"memory": resource.MustParse("256Mi"),
						},
					},
					{
						Name: "sidecar",
						Usage: map[corev1.ResourceName]resource.Quantity{
							"cpu":    resource.MustParse("50m"),
							"memory": resource.MustParse("64Mi"),
						},
					},
				},
			},
		},
	}

	ms := store.NewMetricsStore()
	c := NewMetricsCollector(mock, ms, observability.NewMetrics(), 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return ms.PodMetrics.Len() == 1
	}, waitTimeout, pollInterval, "expected 1 pod metrics in store")

	pm, ok := ms.PodMetrics.Get("default/pod-1")
	require.True(t, ok)
	assert.Equal(t, "pod-1", pm.Name)
	assert.Equal(t, "default", pm.Namespace)
	assert.Equal(t, ts.UnixMilli(), pm.Timestamp)
	require.Len(t, pm.Containers, 2)

	assert.Equal(t, "app", pm.Containers[0].Name)
	assert.InDelta(t, 0.1, pm.Containers[0].CPUUsageCores, 0.001)
	assert.Equal(t, int64(256*1024*1024), pm.Containers[0].MemoryUsageBytes)

	assert.Equal(t, "sidecar", pm.Containers[1].Name)
	assert.InDelta(t, 0.05, pm.Containers[1].CPUUsageCores, 0.001)
	assert.Equal(t, int64(64*1024*1024), pm.Containers[1].MemoryUsageBytes)
}

func TestMetricsCollector_StopsCleanly(t *testing.T) {
	mock := &mockMetricsAPI{}
	c := NewMetricsCollector(mock, store.NewMetricsStore(), observability.NewMetrics(), 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// Stop should not block or panic.
	c.Stop()

	// Wait for done channel to confirm goroutine exited.
	select {
	case <-c.done:
		// ok
	case <-time.After(waitTimeout):
		t.Fatal("collector goroutine did not exit after Stop()")
	}
}

func TestMetricsCollector_WaitForSyncWaitsForFirstPoll(t *testing.T) {
	ts := metav1.Now()
	mock := &mockMetricsAPI{
		nodeMetrics: []metricsv1beta1.NodeMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Timestamp:  ts,
				Usage: map[corev1.ResourceName]resource.Quantity{
					"cpu":    resource.MustParse("1"),
					"memory": resource.MustParse("1Gi"),
				},
			},
		},
	}

	ms := store.NewMetricsStore()
	// Use a long interval to ensure WaitForSync only returns after the first poll,
	// not after waiting for a tick.
	c := NewMetricsCollector(mock, ms, observability.NewMetrics(), 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	// WaitForSync should return quickly (first poll happens immediately on start).
	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// After WaitForSync, data should already be in store.
	assert.Equal(t, 1, ms.NodeMetrics.Len(), "node metrics should be in store after WaitForSync")
}

func TestMetricsCollector_HandlesAPIErrors(t *testing.T) {
	mock := &mockMetricsAPI{
		nodeErr: fmt.Errorf("node metrics unavailable"),
		podErr:  fmt.Errorf("pod metrics unavailable"),
	}

	ms := store.NewMetricsStore()
	c := NewMetricsCollector(mock, ms, observability.NewMetrics(), 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	// WaitForSync should still return (first poll completes, even if with errors).
	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// Store should remain empty since API returned errors.
	assert.Equal(t, 0, ms.NodeMetrics.Len())
	assert.Equal(t, 0, ms.PodMetrics.Len())
}
