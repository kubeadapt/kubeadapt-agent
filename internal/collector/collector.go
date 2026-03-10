package collector

import "context"

// Collector is the interface that all resource collectors implement.
type Collector interface {
	// Name returns the collector's name (e.g., "nodes", "pods", "deployments").
	Name() string
	// Start sets up the informer and begins watching for events.
	Start(ctx context.Context) error
	// WaitForSync waits for the informer cache to sync.
	WaitForSync(ctx context.Context) error
	// Stop stops the collector and cleans up resources.
	Stop()
}

// HealthChecker is an optional interface that collectors can implement to
// report runtime health beyond initial sync. If a collector does not implement
// this interface, it is assumed healthy as long as it started successfully.
type HealthChecker interface {
	// IsHealthy reports whether the collector is functioning correctly.
	// Returns false and a short reason string if the collector is unhealthy
	// (e.g., informer goroutine crashed, poll failures).
	IsHealthy() (healthy bool, reason string)
}

// APICallCounter is an optional interface that collectors can implement to
// report cumulative API call statistics. Only polling collectors (MetricsCollector,
// GPUMetricsCollector) implement this — informer-based collectors do not make
// individual API calls.
type APICallCounter interface {
	// APICallStats returns cumulative totals of API calls made and failed.
	APICallStats() (total, failed int64)
}

// DCGMTargetReporter is an optional interface for collectors that track
// dcgm-exporter scrape targets. Only GPUMetricsCollector implements this.
type DCGMTargetReporter interface {
	// DCGMTargetStats returns the number of configured targets and how many
	// responded successfully in the most recent poll.
	DCGMTargetStats() (targets, upTargets int)
}
