package gpu

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
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

	// API call counters — each poll() scrapes N endpoints (one HTTP call per endpoint).
	scrapeTotal  atomic.Int64
	scrapeFailed atomic.Int64

	// DCGM target tracking — updated each poll for health reporting.
	lastTargets   atomic.Int64
	lastUpTargets atomic.Int64
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
		c.lastTargets.Store(0)
		c.lastUpTargets.Store(0)
		slog.Debug("gpu collector: no dcgm-exporter endpoints configured")
		return
	}

	c.lastTargets.Store(int64(len(endpoints)))
	c.scrapeTotal.Add(int64(len(endpoints)))
	metrics, err := c.api.ScrapeGPUMetrics(ctx, endpoints)
	if err != nil {
		c.scrapeFailed.Add(int64(len(endpoints)))
		c.lastUpTargets.Store(0)
		slog.Warn("gpu collector: failed to scrape GPU metrics", "error", err)
		return
	}

	c.lastUpTargets.Store(int64(len(endpoints)))

	now := time.Now().UnixMilli()
	for i := range metrics {
		metrics[i].Timestamp = now
	}

	c.mu.Lock()
	c.metrics = metrics
	c.mu.Unlock()

	slog.Debug("gpu collector: poll complete", "gpu_count", len(metrics))
}

// IsHealthy implements collector.HealthChecker.
// Reports unhealthy if the polling goroutine exited unexpectedly.
func (c *GPUMetricsCollector) IsHealthy() (bool, string) {
	select {
	case <-c.done:
		select {
		case <-c.stopCh:
			return true, ""
		default:
			return false, "GPU metrics polling goroutine exited unexpectedly"
		}
	default:
		return true, ""
	}
}

// APICallStats returns cumulative API call counters.
// Each poll() makes one HTTP scrape call per dcgm-exporter endpoint.
func (c *GPUMetricsCollector) APICallStats() (total, failed int64) {
	return c.scrapeTotal.Load(), c.scrapeFailed.Load()
}

// DCGMTargetStats returns the number of dcgm-exporter targets and how many
// responded successfully in the most recent poll.
func (c *GPUMetricsCollector) DCGMTargetStats() (targets, upTargets int) {
	return int(c.lastTargets.Load()), int(c.lastUpTargets.Load())
}
