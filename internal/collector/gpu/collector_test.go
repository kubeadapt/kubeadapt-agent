package gpu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testWaitTimeout  = 5 * time.Second
	testPollInterval = 50 * time.Millisecond
)

// mockGPUMetricsAPI implements GPUMetricsAPI for testing.
type mockGPUMetricsAPI struct {
	metrics []GPUDeviceMetrics
	err     error
}

func (m *mockGPUMetricsAPI) ScrapeGPUMetrics(_ context.Context, _ []string) ([]GPUDeviceMetrics, error) {
	return m.metrics, m.err
}

func TestGPUMetricsCollector_Name(t *testing.T) {
	c := NewGPUMetricsCollector(&mockGPUMetricsAPI{}, func() []string { return nil }, time.Minute)
	assert.Equal(t, "gpu", c.Name())
}

func TestGPUMetricsCollector_Lifecycle(t *testing.T) {
	util := 42.0
	mock := &mockGPUMetricsAPI{
		metrics: []GPUDeviceMetrics{
			{
				GPU:            "0",
				UUID:           "GPU-test123",
				GPUUtilization: &util,
			},
		},
	}

	c := NewGPUMetricsCollector(mock, func() []string {
		return []string{"http://localhost:9400"}
	}, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(c.GetGPUMetrics()) == 1
	}, testWaitTimeout, testPollInterval)

	metrics := c.GetGPUMetrics()
	require.Len(t, metrics, 1)
	assert.Equal(t, "GPU-test123", metrics[0].UUID)
	require.NotNil(t, metrics[0].GPUUtilization)
	assert.InDelta(t, 42.0, *metrics[0].GPUUtilization, 0.001)
	assert.NotZero(t, metrics[0].Timestamp)
}

func TestGPUMetricsCollector_StopsCleanly(t *testing.T) {
	c := NewGPUMetricsCollector(&mockGPUMetricsAPI{}, func() []string { return nil }, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	c.Stop()

	select {
	case <-c.done:
		// ok
	case <-time.After(testWaitTimeout):
		t.Fatal("collector goroutine did not exit after Stop()")
	}
}

func TestGPUMetricsCollector_UnreachableEndpoint(t *testing.T) {
	// Create a server and immediately close it to guarantee "connection refused"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	closedURL := server.URL
	server.Close()

	client := &http.Client{Timeout: 1 * time.Second}
	api := NewDCGMExporterClient(client)

	c := NewGPUMetricsCollector(api, func() []string {
		return []string{closedURL}
	}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), testWaitTimeout)
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	// WaitForSync should still return (poll completes, just with no data)
	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// No metrics should be stored (endpoint unreachable)
	assert.Empty(t, c.GetGPUMetrics())
}

func TestGPUMetricsCollector_SuccessfulScrapeWithMockHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(dcgmOutputNewStyleMultiGPU))
	}))
	defer server.Close()

	client := server.Client()
	api := NewDCGMExporterClient(client)

	c := NewGPUMetricsCollector(api, func() []string {
		return []string{server.URL}
	}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), testWaitTimeout)
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(c.GetGPUMetrics()) == 2
	}, testWaitTimeout, testPollInterval)

	metrics := c.GetGPUMetrics()
	require.Len(t, metrics, 2)

	// Verify timestamps are set
	for _, m := range metrics {
		assert.NotZero(t, m.Timestamp)
	}

	// Verify actual metric data was parsed
	byUUID := make(map[string]GPUDeviceMetrics)
	for _, m := range metrics {
		byUUID[m.UUID] = m
	}

	gpu0 := byUUID["GPU-abc123"]
	require.NotNil(t, gpu0.GPUUtilization)
	assert.InDelta(t, 42.0, *gpu0.GPUUtilization, 0.001)
	assert.Equal(t, "myapp-xyz", gpu0.PodName)
}

func TestGPUMetricsCollector_NoEndpoints(t *testing.T) {
	mock := &mockGPUMetricsAPI{}
	c := NewGPUMetricsCollector(mock, func() []string {
		return nil // no endpoints
	}, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// No endpoints means no metrics
	assert.Empty(t, c.GetGPUMetrics())
}

func TestGPUMetricsCollector_GetGPUMetrics_ReturnsCopy(t *testing.T) {
	util := 42.0
	mock := &mockGPUMetricsAPI{
		metrics: []GPUDeviceMetrics{
			{GPU: "0", UUID: "GPU-copy", GPUUtilization: &util},
		},
	}

	c := NewGPUMetricsCollector(mock, func() []string {
		return []string{"http://localhost:9400"}
	}, time.Hour) // long interval so only first poll runs

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// Get two copies and verify they're independent
	m1 := c.GetGPUMetrics()
	m2 := c.GetGPUMetrics()
	require.Len(t, m1, 1)
	require.Len(t, m2, 1)

	m1[0].GPU = "modified"
	assert.Equal(t, "0", m2[0].GPU, "modifying one copy should not affect the other")
}

func TestGPUMetricsCollector_APICallStats_CountsScrapes(t *testing.T) {
	util := 42.0
	mock := &mockGPUMetricsAPI{
		metrics: []GPUDeviceMetrics{
			{GPU: "0", UUID: "GPU-stats", GPUUtilization: &util},
		},
	}

	endpoints := []string{"http://10.0.0.1:9400", "http://10.0.0.2:9400"}
	c := NewGPUMetricsCollector(mock, func() []string {
		return endpoints
	}, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// First poll happens immediately. Wait for at least 2 polls.
	require.Eventually(t, func() bool {
		total, _ := c.APICallStats()
		return total >= 4 // 2 endpoints * 2 polls
	}, testWaitTimeout, testPollInterval)

	total, failed := c.APICallStats()
	assert.True(t, total >= 4, "expected at least 4 scrapes (2 endpoints * 2 polls), got %d", total)
	assert.Equal(t, int64(0), failed)
}

func TestGPUMetricsCollector_APICallStats_CountsFailures(t *testing.T) {
	mock := &mockGPUMetricsAPI{
		err: fmt.Errorf("scrape failed"),
	}

	endpoints := []string{"http://10.0.0.1:9400", "http://10.0.0.2:9400"}
	c := NewGPUMetricsCollector(mock, func() []string {
		return endpoints
	}, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		total, _ := c.APICallStats()
		return total >= 2
	}, testWaitTimeout, testPollInterval)

	total, failed := c.APICallStats()
	assert.True(t, total >= 2, "expected at least 2 scrape attempts, got %d", total)
	assert.Equal(t, total, failed, "all scrapes should be failures")
}

func TestGPUMetricsCollector_APICallStats_NoEndpoints(t *testing.T) {
	mock := &mockGPUMetricsAPI{}
	c := NewGPUMetricsCollector(mock, func() []string {
		return nil
	}, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	// With no endpoints, no API calls should be made.
	time.Sleep(150 * time.Millisecond)
	total, failed := c.APICallStats()
	assert.Equal(t, int64(0), total)
	assert.Equal(t, int64(0), failed)
}

func TestGPUMetricsCollector_DCGMTargetStats_Success(t *testing.T) {
	util := 42.0
	mock := &mockGPUMetricsAPI{
		metrics: []GPUDeviceMetrics{
			{GPU: "0", UUID: "GPU-target", GPUUtilization: &util},
		},
	}

	endpoints := []string{"http://10.0.0.1:9400", "http://10.0.0.2:9400", "http://10.0.0.3:9400"}
	c := NewGPUMetricsCollector(mock, func() []string {
		return endpoints
	}, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	targets, upTargets := c.DCGMTargetStats()
	assert.Equal(t, 3, targets, "should report 3 targets")
	assert.Equal(t, 3, upTargets, "all 3 should be up on successful scrape")
}

func TestGPUMetricsCollector_DCGMTargetStats_Failure(t *testing.T) {
	mock := &mockGPUMetricsAPI{
		err: fmt.Errorf("scrape failed"),
	}

	endpoints := []string{"http://10.0.0.1:9400", "http://10.0.0.2:9400"}
	c := NewGPUMetricsCollector(mock, func() []string {
		return endpoints
	}, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	targets, upTargets := c.DCGMTargetStats()
	assert.Equal(t, 2, targets, "should report 2 targets")
	assert.Equal(t, 0, upTargets, "0 up on failed scrape")
}

func TestGPUMetricsCollector_DCGMTargetStats_NoEndpoints(t *testing.T) {
	c := NewGPUMetricsCollector(&mockGPUMetricsAPI{}, func() []string {
		return nil
	}, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	defer c.Stop()

	err = c.WaitForSync(ctx)
	require.NoError(t, err)

	targets, upTargets := c.DCGMTargetStats()
	assert.Equal(t, 0, targets)
	assert.Equal(t, 0, upTargets)
}
