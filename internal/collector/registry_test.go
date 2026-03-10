package collector

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockCollector implements Collector for testing.
type mockCollector struct {
	mu       sync.Mutex
	name     string
	startErr error
	syncErr  error
	started  bool
	synced   bool
	stopped  bool
	// startDelay adds artificial latency to Start to test parallelism.
	startDelay time.Duration
	// syncDelay adds artificial latency to WaitForSync.
	syncDelay time.Duration
}

func (m *mockCollector) Name() string { return m.name }

func (m *mockCollector) Start(_ context.Context) error {
	if m.startDelay > 0 {
		time.Sleep(m.startDelay)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *mockCollector) WaitForSync(ctx context.Context) error {
	if m.syncDelay > 0 {
		select {
		case <-time.After(m.syncDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.syncErr != nil {
		return m.syncErr
	}
	m.synced = true
	return nil
}

func (m *mockCollector) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

func (m *mockCollector) isStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

func (m *mockCollector) isSynced() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.synced
}

func (m *mockCollector) isStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

func TestRegistry_RegisterMultipleCollectors(t *testing.T) {
	r := NewRegistry()

	c1 := &mockCollector{name: "nodes"}
	c2 := &mockCollector{name: "pods"}
	c3 := &mockCollector{name: "deployments"}

	r.Register(c1)
	r.Register(c2)
	r.Register(c3)

	collectors := r.Collectors()
	if len(collectors) != 3 {
		t.Fatalf("expected 3 collectors, got %d", len(collectors))
	}
	if collectors[0].Name() != "nodes" {
		t.Errorf("expected first collector name 'nodes', got %q", collectors[0].Name())
	}
	if collectors[1].Name() != "pods" {
		t.Errorf("expected second collector name 'pods', got %q", collectors[1].Name())
	}
	if collectors[2].Name() != "deployments" {
		t.Errorf("expected third collector name 'deployments', got %q", collectors[2].Name())
	}
}

func TestRegistry_StartAllParallel(t *testing.T) {
	r := NewRegistry()

	// Each collector has a delay; if run sequentially, total > 300ms.
	// Parallel execution should complete in ~100ms.
	c1 := &mockCollector{name: "nodes", startDelay: 100 * time.Millisecond}
	c2 := &mockCollector{name: "pods", startDelay: 100 * time.Millisecond}
	c3 := &mockCollector{name: "deployments", startDelay: 100 * time.Millisecond}

	r.Register(c1)
	r.Register(c2)
	r.Register(c3)

	start := time.Now()
	err := r.StartAll(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should complete well under 250ms if truly parallel (3*100ms sequential = 300ms).
	if elapsed > 250*time.Millisecond {
		t.Errorf("StartAll took %v, expected parallel execution under 250ms", elapsed)
	}

	if !c1.isStarted() || !c2.isStarted() || !c3.isStarted() {
		t.Error("expected all collectors to be started")
	}
}

func TestRegistry_StartAllOneFailsOthersSucceed(t *testing.T) {
	r := NewRegistry()

	c1 := &mockCollector{name: "nodes"}
	c2 := &mockCollector{name: "pods", startErr: errors.New("pod informer failed")}
	c3 := &mockCollector{name: "deployments"}

	r.Register(c1)
	r.Register(c2)
	r.Register(c3)

	err := r.StartAll(context.Background())
	if err == nil {
		t.Fatal("expected PartialStartError when one collector fails, got nil")
	}

	var partial *PartialStartError
	if !errors.As(err, &partial) {
		t.Fatalf("expected PartialStartError, got %T: %v", err, err)
	}
	if len(partial.Failed) != 1 {
		t.Errorf("expected 1 failed collector, got %d", len(partial.Failed))
	}
	if partial.Total != 3 {
		t.Errorf("expected Total=3, got %d", partial.Total)
	}

	if !c1.isStarted() {
		t.Error("expected 'nodes' collector to be started")
	}
	if c2.isStarted() {
		t.Error("expected 'pods' collector NOT to be started (it returned error)")
	}
	if !c3.isStarted() {
		t.Error("expected 'deployments' collector to be started")
	}
}

func TestRegistry_StartAllAllFail(t *testing.T) {
	r := NewRegistry()

	c1 := &mockCollector{name: "nodes", startErr: errors.New("fail1")}
	c2 := &mockCollector{name: "pods", startErr: errors.New("fail2")}

	r.Register(c1)
	r.Register(c2)

	err := r.StartAll(context.Background())
	if err == nil {
		t.Fatal("expected error when all collectors fail")
	}
}

func TestRegistry_WaitForSyncAllSync(t *testing.T) {
	r := NewRegistry()

	c1 := &mockCollector{name: "nodes"}
	c2 := &mockCollector{name: "pods"}

	r.Register(c1)
	r.Register(c2)

	err := r.WaitForSync(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !c1.isSynced() {
		t.Error("expected 'nodes' to be synced")
	}
	if !c2.isSynced() {
		t.Error("expected 'pods' to be synced")
	}
}

func TestRegistry_WaitForSyncContextTimeout(t *testing.T) {
	r := NewRegistry()

	// Collector with a sync delay longer than the context timeout.
	c1 := &mockCollector{name: "nodes", syncDelay: 5 * time.Second}

	r.Register(c1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := r.WaitForSync(ctx)
	if err == nil {
		t.Fatal("expected error on context timeout")
	}
}

func TestRegistry_StopAll(t *testing.T) {
	r := NewRegistry()

	c1 := &mockCollector{name: "nodes"}
	c2 := &mockCollector{name: "pods"}

	r.Register(c1)
	r.Register(c2)

	// Must start before stopping.
	if err := r.StartAll(context.Background()); err != nil {
		t.Fatalf("StartAll failed: %v", err)
	}

	r.StopAll()

	if !c1.isStopped() {
		t.Error("expected 'nodes' to be stopped")
	}
	if !c2.isStopped() {
		t.Error("expected 'pods' to be stopped")
	}
}

func TestRegistry_StopAllIdempotent(t *testing.T) {
	r := NewRegistry()

	var stopCount atomic.Int32
	c := &countingCollector{name: "nodes", stopCount: &stopCount}

	r.Register(c)

	if err := r.StartAll(context.Background()); err != nil {
		t.Fatalf("StartAll failed: %v", err)
	}

	r.StopAll()
	r.StopAll() // second call should be a no-op

	if stopCount.Load() != 1 {
		t.Errorf("expected Stop called once, got %d", stopCount.Load())
	}
}

// countingCollector counts how many times Stop is called.
type countingCollector struct {
	name      string
	stopCount *atomic.Int32
}

func (c *countingCollector) Name() string                        { return c.name }
func (c *countingCollector) Start(_ context.Context) error       { return nil }
func (c *countingCollector) WaitForSync(_ context.Context) error { return nil }
func (c *countingCollector) Stop()                               { c.stopCount.Add(1) }

func TestRegistry_ConcurrentRegisterAndStartAll(t *testing.T) {
	r := NewRegistry()

	// Pre-register some collectors.
	for i := 0; i < 10; i++ {
		r.Register(&mockCollector{name: "pre"})
	}

	var wg sync.WaitGroup

	// Concurrent registers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			r.Register(&mockCollector{name: "concurrent"})
		}
	}()

	// Concurrent StartAll.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = r.StartAll(context.Background())
	}()

	// Concurrent Collectors read.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = r.Collectors()
		}
	}()

	wg.Wait()

	// Verify no panic occurred and collectors are registered.
	collectors := r.Collectors()
	if len(collectors) < 10 {
		t.Errorf("expected at least 10 collectors, got %d", len(collectors))
	}
}

func TestRegistry_StartAllEmpty(t *testing.T) {
	r := NewRegistry()
	err := r.StartAll(context.Background())
	if err != nil {
		t.Fatalf("expected no error for empty registry, got %v", err)
	}
}

func TestRegistry_WaitForSyncEmpty(t *testing.T) {
	r := NewRegistry()
	err := r.WaitForSync(context.Background())
	if err != nil {
		t.Fatalf("expected no error for empty registry, got %v", err)
	}
}

func TestRegistry_StopAllEmpty(t *testing.T) {
	r := NewRegistry()
	// Should not panic.
	r.StopAll()
}

// healthyCollector implements both Collector and HealthChecker, always healthy.
type healthyCollector struct {
	name string
}

func (c *healthyCollector) Name() string                        { return c.name }
func (c *healthyCollector) Start(_ context.Context) error       { return nil }
func (c *healthyCollector) WaitForSync(_ context.Context) error { return nil }
func (c *healthyCollector) Stop()                               {}
func (c *healthyCollector) IsHealthy() (bool, string)           { return true, "" }

// unhealthyCollector implements both Collector and HealthChecker, always unhealthy.
type unhealthyCollector struct {
	name   string
	reason string
}

func (c *unhealthyCollector) Name() string                        { return c.name }
func (c *unhealthyCollector) Start(_ context.Context) error       { return nil }
func (c *unhealthyCollector) WaitForSync(_ context.Context) error { return nil }
func (c *unhealthyCollector) Stop()                               {}
func (c *unhealthyCollector) IsHealthy() (bool, string)           { return false, c.reason }

func TestRegistry_HealthReportAllHealthy(t *testing.T) {
	r := NewRegistry()

	r.Register(&healthyCollector{name: "nodes"})
	r.Register(&healthyCollector{name: "pods"})
	r.Register(&healthyCollector{name: "deployments"})

	healthy, total, stale := r.HealthReport()
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if healthy != 3 {
		t.Errorf("expected healthy=3, got %d", healthy)
	}
	if len(stale) != 0 {
		t.Errorf("expected no stale resources, got %v", stale)
	}
}

func TestRegistry_HealthReportSomeUnhealthy(t *testing.T) {
	r := NewRegistry()

	r.Register(&healthyCollector{name: "nodes"})
	r.Register(&unhealthyCollector{name: "pods", reason: "goroutine crashed"})
	r.Register(&healthyCollector{name: "deployments"})
	r.Register(&unhealthyCollector{name: "services", reason: "connection lost"})

	healthy, total, stale := r.HealthReport()
	if total != 4 {
		t.Errorf("expected total=4, got %d", total)
	}
	if healthy != 2 {
		t.Errorf("expected healthy=2, got %d", healthy)
	}
	if len(stale) != 2 {
		t.Errorf("expected 2 stale resources, got %d: %v", len(stale), stale)
	}
}

func TestRegistry_HealthReportWithNonHealthChecker(t *testing.T) {
	r := NewRegistry()

	// mockCollector does NOT implement HealthChecker — should be assumed healthy.
	r.Register(&mockCollector{name: "legacy"})
	r.Register(&healthyCollector{name: "nodes"})
	r.Register(&unhealthyCollector{name: "pods", reason: "crashed"})

	healthy, total, stale := r.HealthReport()
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if healthy != 2 {
		t.Errorf("expected healthy=2 (legacy assumed healthy + nodes), got %d", healthy)
	}
	if len(stale) != 1 {
		t.Errorf("expected 1 stale resource, got %d: %v", len(stale), stale)
	}
}

func TestRegistry_HealthReportEmpty(t *testing.T) {
	r := NewRegistry()

	healthy, total, stale := r.HealthReport()
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if healthy != 0 {
		t.Errorf("expected healthy=0, got %d", healthy)
	}
	if len(stale) != 0 {
		t.Errorf("expected no stale, got %v", stale)
	}
}

// apiCallCollector implements Collector + APICallCounter.
type apiCallCollector struct {
	name       string
	callTotal  int64
	callFailed int64
}

func (c *apiCallCollector) Name() string                        { return c.name }
func (c *apiCallCollector) Start(_ context.Context) error       { return nil }
func (c *apiCallCollector) WaitForSync(_ context.Context) error { return nil }
func (c *apiCallCollector) Stop()                               {}
func (c *apiCallCollector) APICallStats() (total, failed int64) {
	return c.callTotal, c.callFailed
}

func TestRegistry_APICallReportAggregate(t *testing.T) {
	r := NewRegistry()

	// MetricsCollector-like: 100 total, 2 failed.
	r.Register(&apiCallCollector{name: "metrics", callTotal: 100, callFailed: 2})
	// GPUMetricsCollector-like: 50 total, 5 failed.
	r.Register(&apiCallCollector{name: "gpu", callTotal: 50, callFailed: 5})
	// Regular informer collector — no APICallCounter.
	r.Register(&mockCollector{name: "nodes"})

	total, failed := r.APICallReport()
	if total != 150 {
		t.Errorf("expected total=150, got %d", total)
	}
	if failed != 7 {
		t.Errorf("expected failed=7, got %d", failed)
	}
}

func TestRegistry_APICallReportEmpty(t *testing.T) {
	r := NewRegistry()

	total, failed := r.APICallReport()
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if failed != 0 {
		t.Errorf("expected failed=0, got %d", failed)
	}
}

func TestRegistry_APICallReportNoAPICallers(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockCollector{name: "nodes"})
	r.Register(&mockCollector{name: "pods"})

	total, failed := r.APICallReport()
	if total != 0 {
		t.Errorf("expected total=0 when no APICallCounter collectors, got %d", total)
	}
	if failed != 0 {
		t.Errorf("expected failed=0, got %d", failed)
	}
}

// dcgmCollector implements Collector + DCGMTargetReporter.
type dcgmCollector struct {
	name      string
	targets   int
	upTargets int
}

func (c *dcgmCollector) Name() string                        { return c.name }
func (c *dcgmCollector) Start(_ context.Context) error       { return nil }
func (c *dcgmCollector) WaitForSync(_ context.Context) error { return nil }
func (c *dcgmCollector) Stop()                               {}
func (c *dcgmCollector) DCGMTargetStats() (targets, upTargets int) {
	return c.targets, c.upTargets
}

func TestRegistry_DCGMTargetReport(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockCollector{name: "nodes"})
	r.Register(&dcgmCollector{name: "gpu", targets: 3, upTargets: 2})

	targets, upTargets := r.DCGMTargetReport()
	if targets != 3 {
		t.Errorf("expected targets=3, got %d", targets)
	}
	if upTargets != 2 {
		t.Errorf("expected upTargets=2, got %d", upTargets)
	}
}

func TestRegistry_DCGMTargetReportNone(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockCollector{name: "nodes"})
	r.Register(&mockCollector{name: "pods"})

	targets, upTargets := r.DCGMTargetReport()
	if targets != 0 {
		t.Errorf("expected targets=0, got %d", targets)
	}
	if upTargets != 0 {
		t.Errorf("expected upTargets=0, got %d", upTargets)
	}
}
