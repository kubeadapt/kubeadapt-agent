package resource

import (
	"log/slog"
	"time"

	"k8s.io/client-go/tools/cache"
)

const informerMaxRetries = 3

// runInformerWithRecovery runs an informer with crash recovery: on unexpected
// exit retries informerMaxRetries times with linear backoff, then closes done.
func runInformerWithRecovery(informer cache.SharedIndexInformer, name string, stopCh <-chan struct{}, done chan<- struct{}) {
	go func() {
		defer close(done)
		for attempt := 0; attempt <= informerMaxRetries; attempt++ {
			if attempt > 0 {
				slog.Warn("restarting informer after crash",
					"collector", name,
					"attempt", attempt,
					"max_retries", informerMaxRetries,
				)
				time.Sleep(time.Duration(attempt) * 5 * time.Second)
			}
			informer.Run(stopCh)
			select {
			case <-stopCh:
				return
			default:
			}
		}
		slog.Error("informer exhausted retries, giving up",
			"collector", name,
			"max_retries", informerMaxRetries,
		)
	}()
}
