package collector

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Registry manages the lifecycle of all registered collectors.
// It is thread-safe: Register, StartAll, WaitForSync, and StopAll
// can be called from different goroutines.
type Registry struct {
	collectors []Collector
	mu         sync.Mutex
	started    bool
}

// NewRegistry creates a new, empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a collector to the registry.
func (r *Registry) Register(c Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors = append(r.collectors, c)
}

// PartialStartError is returned when some (but not all) collectors fail to start.
// Callers can use errors.As to detect partial vs total failure.
type PartialStartError struct {
	Failed []string
	Total  int
}

func (e *PartialStartError) Error() string {
	return fmt.Sprintf("%d of %d collectors failed to start: %v", len(e.Failed), e.Total, e.Failed)
}

// StartAll starts all registered collectors in parallel using goroutines.
// Returns a PartialStartError if some collectors fail, or a plain error
// if all collectors fail.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.Lock()
	collectors := make([]Collector, len(r.collectors))
	copy(collectors, r.collectors)
	r.started = true
	r.mu.Unlock()

	if len(collectors) == 0 {
		return nil
	}

	type result struct {
		name string
		err  error
	}

	results := make(chan result, len(collectors))
	var wg sync.WaitGroup

	for _, c := range collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()
			err := c.Start(ctx)
			results <- result{name: c.Name(), err: err}
		}(c)
	}

	// Close results channel after all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	var failedNames []string
	for res := range results {
		if res.err != nil {
			failedNames = append(failedNames, res.name)
			slog.Error("collector failed to start", "collector", res.name, "error", res.err)
		}
	}

	if len(failedNames) == len(collectors) {
		return fmt.Errorf("all %d collectors failed to start", len(failedNames))
	}
	if len(failedNames) > 0 {
		return &PartialStartError{Failed: failedNames, Total: len(collectors)}
	}

	return nil
}

// WaitForSync waits for all registered collectors to sync their informer caches.
// Uses the context deadline/timeout. Returns an error if the context expires
// before all collectors sync.
func (r *Registry) WaitForSync(ctx context.Context) error {
	r.mu.Lock()
	collectors := make([]Collector, len(r.collectors))
	copy(collectors, r.collectors)
	r.mu.Unlock()

	if len(collectors) == 0 {
		return nil
	}

	errCh := make(chan error, len(collectors))
	var wg sync.WaitGroup

	for _, c := range collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()
			errCh <- c.WaitForSync(ctx)
		}(c)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return fmt.Errorf("collector sync failed: %w", err)
		}
	}

	return nil
}

// StopAll stops all registered collectors. Safe to call multiple times.
func (r *Registry) StopAll() {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	collectors := make([]Collector, len(r.collectors))
	copy(collectors, r.collectors)
	r.started = false
	r.mu.Unlock()

	for _, c := range collectors {
		c.Stop()
	}
}

// Collectors returns the registered collectors.
func (r *Registry) Collectors() []Collector {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Collector, len(r.collectors))
	copy(out, r.collectors)
	return out
}
