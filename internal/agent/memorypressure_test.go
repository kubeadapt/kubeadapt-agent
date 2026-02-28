package agent

import (
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMemStatsProvider returns pre-configured MemStats for testing.
type fakeMemStatsProvider struct {
	sys          uint64
	heapReleased uint64
}

func (f *fakeMemStatsProvider) ReadMemStats(m *runtime.MemStats) {
	m.Sys = f.sys
	m.HeapReleased = f.heapReleased
}

func TestMemoryPressureMonitor_ThresholdExceeded(t *testing.T) {
	// Set a temporary GOMEMLIMIT so the check can use it.
	origLimit := debug.SetMemoryLimit(-1)
	debug.SetMemoryLimit(100) // 100 bytes limit for test
	defer debug.SetMemoryLimit(origLimit)

	var called atomic.Int32
	provider := &fakeMemStatsProvider{
		sys:          90, // 90 bytes used
		heapReleased: 0,  // 0 released → usage = 90
		// ratio = 90/100 = 0.9 > 0.8 threshold
	}

	mon := NewMemoryPressureMonitor(0.8, func() {
		called.Add(1)
	}, 10*time.Millisecond, provider)

	mon.Start()
	// Wait for at least one tick.
	time.Sleep(50 * time.Millisecond)
	mon.Stop()

	assert.Greater(t, called.Load(), int32(0), "callback should have been called")
}

func TestMemoryPressureMonitor_BelowThreshold(t *testing.T) {
	origLimit := debug.SetMemoryLimit(-1)
	debug.SetMemoryLimit(100)
	defer debug.SetMemoryLimit(origLimit)

	var called atomic.Int32
	provider := &fakeMemStatsProvider{
		sys:          50, // 50 bytes used
		heapReleased: 0,  // usage = 50
		// ratio = 50/100 = 0.5 < 0.8 threshold
	}

	mon := NewMemoryPressureMonitor(0.8, func() {
		called.Add(1)
	}, 10*time.Millisecond, provider)

	mon.Start()
	time.Sleep(50 * time.Millisecond)
	mon.Stop()

	assert.Equal(t, int32(0), called.Load(), "callback should not have been called")
}

func TestMemoryPressureMonitor_NoMemLimit(t *testing.T) {
	// When GOMEMLIMIT is not set (math.MaxInt64), check should return false.
	// We can't easily unset GOMEMLIMIT in a test, but we can verify the
	// monitor handles high limits gracefully — usage/huge_limit ≈ 0 < threshold.
	origLimit := debug.SetMemoryLimit(-1)
	// Set an extremely high limit so the ratio is ~0.
	debug.SetMemoryLimit(1 << 62)
	defer debug.SetMemoryLimit(origLimit)

	var called atomic.Int32
	provider := &fakeMemStatsProvider{
		sys:          1000,
		heapReleased: 0,
	}

	mon := NewMemoryPressureMonitor(0.8, func() {
		called.Add(1)
	}, 10*time.Millisecond, provider)

	mon.Start()
	time.Sleep(50 * time.Millisecond)
	mon.Stop()

	assert.Equal(t, int32(0), called.Load(), "callback should not have been called with huge limit")
}

func TestMemoryPressureMonitor_StopsCleanly(t *testing.T) {
	origLimit := debug.SetMemoryLimit(-1)
	debug.SetMemoryLimit(100)
	defer debug.SetMemoryLimit(origLimit)

	provider := &fakeMemStatsProvider{
		sys:          90,
		heapReleased: 0,
	}

	var called atomic.Int32
	mon := NewMemoryPressureMonitor(0.8, func() {
		called.Add(1)
	}, 10*time.Millisecond, provider)

	mon.Start()
	time.Sleep(30 * time.Millisecond)
	mon.Stop()

	// Allow any in-flight callback to finish.
	time.Sleep(20 * time.Millisecond)
	countAfterStop := called.Load()

	// Wait and verify no more callbacks fire.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, countAfterStop, called.Load(), "callback should not be called after stop")
}

func TestMemoryPressureMonitor_DoubleStopSafe(t *testing.T) {
	provider := &fakeMemStatsProvider{sys: 50, heapReleased: 0}
	mon := NewMemoryPressureMonitor(0.8, func() {}, 10*time.Millisecond, provider)

	mon.Start()
	require.NotPanics(t, func() {
		mon.Stop()
		mon.Stop()
	})
}
