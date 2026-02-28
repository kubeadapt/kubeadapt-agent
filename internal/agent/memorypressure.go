package agent

import (
	"log/slog"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// MemStatsProvider abstracts runtime.MemStats reading for testability.
type MemStatsProvider interface {
	ReadMemStats(m *runtime.MemStats)
}

// runtimeMemStatsProvider uses the real runtime.ReadMemStats.
type runtimeMemStatsProvider struct{}

func (runtimeMemStatsProvider) ReadMemStats(m *runtime.MemStats) {
	runtime.ReadMemStats(m)
}

// MemoryPressureMonitor polls runtime.MemStats at a regular interval and
// invokes a callback when memory usage exceeds a configurable threshold
// relative to GOMEMLIMIT.
type MemoryPressureMonitor struct {
	threshold float64       // 0.8 = 80%
	callback  func()        // called when pressure detected
	interval  time.Duration // polling interval
	provider  MemStatsProvider
	stopOnce  sync.Once
	stopCh    chan struct{}
}

// NewMemoryPressureMonitor creates a monitor that calls callback when
// memory usage exceeds threshold * GOMEMLIMIT.
// If provider is nil, the real runtime.ReadMemStats is used.
func NewMemoryPressureMonitor(threshold float64, callback func(), interval time.Duration, provider MemStatsProvider) *MemoryPressureMonitor {
	if provider == nil {
		provider = runtimeMemStatsProvider{}
	}
	return &MemoryPressureMonitor{
		threshold: threshold,
		callback:  callback,
		interval:  interval,
		provider:  provider,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the background polling goroutine.
func (m *MemoryPressureMonitor) Start() {
	go m.run()
}

func (m *MemoryPressureMonitor) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if m.check() {
				slog.Warn("memory pressure detected, triggering callback")
				m.callback()
			}
		}
	}
}

// check returns true if memory usage exceeds the threshold relative to GOMEMLIMIT.
func (m *MemoryPressureMonitor) check() bool {
	limit := debug.SetMemoryLimit(-1) // read current limit without changing it
	if limit <= 0 {
		return false // GOMEMLIMIT not set
	}

	var stats runtime.MemStats
	m.provider.ReadMemStats(&stats)

	usage := stats.Sys - stats.HeapReleased
	ratio := float64(usage) / float64(limit)

	return ratio > m.threshold
}

// Stop halts the background polling goroutine. Safe to call multiple times.
func (m *MemoryPressureMonitor) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}
