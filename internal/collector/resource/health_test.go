package resource

import "testing"

func TestInformerHealthy_Running(t *testing.T) {
	stopCh := make(chan struct{})
	done := make(chan struct{})

	healthy, reason := informerHealthy(stopCh, done)
	if !healthy {
		t.Errorf("expected healthy=true when goroutine is running, got reason=%q", reason)
	}
}

func TestInformerHealthy_GracefulStop(t *testing.T) {
	stopCh := make(chan struct{})
	done := make(chan struct{})

	// Simulate graceful shutdown: close stopCh, then done.
	close(stopCh)
	close(done)

	healthy, reason := informerHealthy(stopCh, done)
	if !healthy {
		t.Errorf("expected healthy=true after graceful stop, got reason=%q", reason)
	}
}

func TestInformerHealthy_Crashed(t *testing.T) {
	stopCh := make(chan struct{})
	done := make(chan struct{})

	// Simulate crash: done closes without stopCh being closed.
	close(done)

	healthy, reason := informerHealthy(stopCh, done)
	if healthy {
		t.Error("expected healthy=false when goroutine crashed")
	}
	if reason == "" {
		t.Error("expected non-empty reason for unhealthy collector")
	}
}
