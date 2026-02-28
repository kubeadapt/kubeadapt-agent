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
