package errors

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockClock is a controllable clock for testing auto-expiry.
type mockClock struct {
	mu  sync.Mutex
	now time.Time
}

func newMockClock(t time.Time) *mockClock {
	return &mockClock{now: t}
}

func (m *mockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

func (m *mockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

func TestAgentError_Implements_Error(t *testing.T) {
	ae := AgentError{
		Code:      ErrMetricsUnavailable,
		Message:   "metrics-server not reachable",
		Component: "collector.metrics",
		Timestamp: time.Now().UnixMilli(),
	}

	// Must satisfy the error interface.
	var err error = &ae
	if err.Error() != "metrics-server not reachable" {
		t.Fatalf("expected Error() = %q, got %q", "metrics-server not reachable", err.Error())
	}
}

func TestErrorCollector_Report(t *testing.T) {
	clk := newMockClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	ec := NewErrorCollector(clk)

	ec.Report(AgentError{
		Code:      ErrBackendUnreachable,
		Message:   "connection refused",
		Component: "transport",
		Timestamp: clk.Now().UnixMilli(),
	})

	active := ec.GetActiveErrors()
	if len(active) != 1 {
		t.Fatalf("expected 1 active error, got %d", len(active))
	}
	if active[0].Code != ErrBackendUnreachable {
		t.Fatalf("expected code %s, got %s", ErrBackendUnreachable, active[0].Code)
	}
}

func TestErrorCollector_AutoExpiry(t *testing.T) {
	clk := newMockClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	ec := NewErrorCollector(clk)

	ec.Report(AgentError{
		Code:      ErrInformerSyncFailed,
		Message:   "sync failed",
		Component: "collector.pods",
		Timestamp: clk.Now().UnixMilli(),
	})

	// Advance 6 minutes — beyond the 5-minute TTL.
	clk.Advance(6 * time.Minute)

	active := ec.GetActiveErrors()
	if len(active) != 0 {
		t.Fatalf("expected 0 active errors after expiry, got %d", len(active))
	}
}

func TestErrorCollector_RefreshPreventsExpiry(t *testing.T) {
	clk := newMockClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	ec := NewErrorCollector(clk)

	ae := AgentError{
		Code:      ErrTimeout,
		Message:   "request timeout",
		Component: "transport",
		Timestamp: clk.Now().UnixMilli(),
	}
	ec.Report(ae)

	// Advance 3 minutes, re-report (refresh).
	clk.Advance(3 * time.Minute)
	ae.Timestamp = clk.Now().UnixMilli()
	ec.Report(ae)

	// Advance another 3 minutes (6 total from initial, but only 3 from last report).
	clk.Advance(3 * time.Minute)

	active := ec.GetActiveErrors()
	if len(active) != 1 {
		t.Fatalf("expected 1 active error (refreshed), got %d", len(active))
	}
}

func TestErrorCollector_ThreadSafe(t *testing.T) {
	clk := newMockClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	ec := NewErrorCollector(clk)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ec.Report(AgentError{
				Code:      Code(fmt.Sprintf("ERR_%d", idx%5)),
				Message:   fmt.Sprintf("error %d", idx),
				Component: fmt.Sprintf("comp_%d", idx%3),
				Timestamp: clk.Now().UnixMilli(),
			})
			_ = ec.GetActiveErrors()
			_ = ec.GetActiveErrorCodes()
		}(i)
	}
	wg.Wait()

	// Just verify no panics/races; content correctness tested elsewhere.
	active := ec.GetActiveErrors()
	if len(active) == 0 {
		t.Fatal("expected some active errors after concurrent writes")
	}
}

func TestErrorCollector_GetActiveErrorCodes(t *testing.T) {
	clk := newMockClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	ec := NewErrorCollector(clk)

	ec.Report(AgentError{Code: ErrAuthFailed, Message: "auth failed", Component: "transport", Timestamp: clk.Now().UnixMilli()})
	ec.Report(AgentError{Code: ErrBufferFull, Message: "buffer full", Component: "transport", Timestamp: clk.Now().UnixMilli()})
	ec.Report(AgentError{Code: ErrCRDNotFound, Message: "crd missing", Component: "collector.vpa", Timestamp: clk.Now().UnixMilli()})

	// Same code, different component — should still show as one code.
	ec.Report(AgentError{Code: ErrAuthFailed, Message: "auth failed again", Component: "metrics", Timestamp: clk.Now().UnixMilli()})

	codes := ec.GetActiveErrorCodes()
	if len(codes) != 3 {
		t.Fatalf("expected 3 unique codes, got %d: %v", len(codes), codes)
	}

	codeSet := make(map[string]bool)
	for _, c := range codes {
		codeSet[c] = true
	}
	for _, expected := range []string{string(ErrAuthFailed), string(ErrBufferFull), string(ErrCRDNotFound)} {
		if !codeSet[expected] {
			t.Fatalf("expected code %s in results", expected)
		}
	}
}

func TestErrorCollector_Clear(t *testing.T) {
	clk := newMockClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	ec := NewErrorCollector(clk)

	ec.Report(AgentError{Code: ErrPartialData, Message: "partial", Component: "snapshot", Timestamp: clk.Now().UnixMilli()})
	ec.Report(AgentError{Code: ErrDiscoveryFailed, Message: "discovery", Component: "discovery", Timestamp: clk.Now().UnixMilli()})

	ec.Clear()

	if len(ec.GetActiveErrors()) != 0 {
		t.Fatal("expected 0 errors after Clear()")
	}
	if len(ec.GetActiveErrorCodes()) != 0 {
		t.Fatal("expected 0 error codes after Clear()")
	}
}
