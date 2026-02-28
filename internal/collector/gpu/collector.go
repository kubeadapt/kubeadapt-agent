package gpu

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// GPUMetricsCollector polls dcgm-exporter endpoints on a timer and collects
// GPU device metrics. It implements the collector.Collector interface.
type GPUMetricsCollector struct {
	api         GPUMetricsAPI
	endpointsFn func() []string
	interval    time.Duration
	stopCh      chan struct{}
	done        chan struct{}

	syncOnce sync.Once
	synced   chan struct{}

	mu      sync.RWMutex
	metrics []GPUDeviceMetrics
}

// NewGPUMetricsCollector creates a GPUMetricsCollector that polls dcgm-exporter endpoints.
// endpointsFn is called on each poll to get the current list of endpoints to scrape.
func NewGPUMetricsCollector(api GPUMetricsAPI, endpointsFn func() []string, interval time.Duration) *GPUMetricsCollector {
	return &GPUMetricsCollector{
		api:         api,
		endpointsFn: endpointsFn,
		interval:    interval,
		stopCh:      make(chan struct{}),
		done:        make(chan struct{}),
		synced:      make(chan struct{}),
	}
}

// Name returns the collector name.
func (c *GPUMetricsCollector) Name() string { return "gpu" }

// Start launches the background polling goroutine.
func (c *GPUMetricsCollector) Start(ctx context.Context) error {
	go c.run(ctx)
	return nil
}

// WaitForSync blocks until the first poll completes or the context is canceled.
func (c *GPUMetricsCollector) WaitForSync(ctx context.Context) error {
	select {
	case <-c.synced:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *GPUMetricsCollector) Stop() {
	close(c.stopCh)
	<-c.done
}

// GetGPUMetrics returns a copy of the latest collected GPU metrics.
func (c *GPUMetricsCollector) GetGPUMetrics() []GPUDeviceMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]GPUDeviceMetrics, len(c.metrics))
	copy(out, c.metrics)
	return out
}

func (c *GPUMetricsCollector) run(ctx context.Context) {
	defer close(c.done)

	// Poll immediately on start.
	c.poll(ctx)
	c.syncOnce.Do(func() { close(c.synced) })

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.poll(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *GPUMetricsCollector) poll(ctx context.Context) {
	endpoints := c.endpointsFn()
	if len(endpoints) == 0 {
		slog.Debug("gpu collector: no dcgm-exporter endpoints configured")
		return
	}

	metrics, err := c.api.ScrapeGPUMetrics(ctx, endpoints)
	if err != nil {
		slog.Warn("gpu collector: failed to scrape GPU metrics", "error", err)
		return
	}

	now := time.Now().UnixMilli()
	for i := range metrics {
		metrics[i].Timestamp = now
	}

	c.mu.Lock()
	c.metrics = metrics
	c.mu.Unlock()

	slog.Debug("gpu collector: poll complete", "gpu_count", len(metrics))
}
