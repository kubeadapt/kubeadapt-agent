package store

import (
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"testing"
)

// BenchmarkTypedStore_ConcurrentReadWrite measures the throughput of
// concurrent read-heavy (80% Get, 20% Set) access to a TypedStore.
func BenchmarkTypedStore_ConcurrentReadWrite(b *testing.B) {
	b.ReportAllocs()

	const preload = 1000

	s := NewTypedStore[string]()
	for i := 0; i < preload; i++ {
		s.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	// Pre-generate keys so goroutines don't allocate during the hot loop.
	keys := make([]string, preload)
	for i := 0; i < preload; i++ {
		keys[i] = fmt.Sprintf("key-%d", i)
	}

	var ops atomic.Int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own local rng to avoid contention.
		localRng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
		for pb.Next() {
			idx := localRng.IntN(preload)
			if localRng.IntN(100) < 80 {
				// 80% reads
				s.Get(keys[idx])
			} else {
				// 20% writes
				s.Set(keys[idx], "updated")
			}
			ops.Add(1)
		}
	})

	totalOps := ops.Load()
	elapsed := b.Elapsed()
	if elapsed.Seconds() > 0 {
		b.ReportMetric(float64(totalOps)/elapsed.Seconds(), "items/sec")
	}
}
