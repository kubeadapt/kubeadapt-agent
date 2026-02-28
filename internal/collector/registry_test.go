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
