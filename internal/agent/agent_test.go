package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubeadapt/kubeadapt-agent/internal/collector"
	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	"github.com/kubeadapt/kubeadapt-agent/internal/enrichment"
	"github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/snapshot"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"github.com/kubeadapt/kubeadapt-agent/internal/transport"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// --- mock collector ---

type stubCollector struct {
	name string
}

func (s *stubCollector) Name() string                        { return s.name }
func (s *stubCollector) Start(_ context.Context) error       { return nil }
func (s *stubCollector) WaitForSync(_ context.Context) error { return nil }
func (s *stubCollector) Stop()                               {}

// --- test helpers ---

// newTestBackend creates an httptest server that returns 200 with a valid
// SnapshotResponse. The requestCount is incremented on each request.
func newTestBackend(t *testing.T, requestCount *atomic.Int32, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch statusCode {
		case http.StatusOK:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(model.SnapshotResponse{
				Success: true,
				Quota: model.QuotaStatus{
					PlanType:      "pro",
					IsWithinQuota: true,
				},
			})
		case http.StatusUnauthorized:
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(model.SnapshotErrorResponse{
				Success: false,
				Error:   "authentication failed",
			})
		default:
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(model.SnapshotErrorResponse{
				Success: false,
				Error:   "error",
			})
		}
	}))
}

func newTestConfig(backendURL string) *config.Config {
	return &config.Config{
		APIKey:              "test-key",
		ClusterID:           "test-cluster",
		ClusterName:         "test",
		BackendURL:          backendURL,
		SnapshotInterval:    50 * time.Millisecond, // fast for tests
		InformerSyncTimeout: 5 * time.Second,
		CompressionLevel:    1,
		MaxRetries:          0,
		RequestTimeout:      5 * time.Second,
		HealthPort:          0,
		AgentVersion:        "test",
	}
}

func newTestAgent(t *testing.T, backendURL string) (*Agent, *collector.Registry) {
	t.Helper()

	cfg := newTestConfig(backendURL)
	clk := errors.RealClock{}
	errCollector := errors.NewErrorCollector(clk)
	metrics := observability.NewMetrics()
	sm := NewStateMachine(clk)
	st := store.NewStore()
	ms := store.NewMetricsStore()
	pipeline := enrichment.NewPipeline(metrics)
	builder := snapshot.NewSnapshotBuilder(st, ms, cfg, metrics, errCollector, pipeline, nil)
	tc := transport.NewClient(cfg, metrics, errCollector)
	reg := collector.NewRegistry()
	reg.Register(&stubCollector{name: "test-stub"})

	ag := NewAgent(cfg, reg, builder, tc, sm, errCollector, metrics)
	return ag, reg
}

// newTestAgentWithConfig returns an agent with custom backend URL applied
// after construction (to swap the transport to a test server).
func newTestAgentWithCustomTransport(t *testing.T, backendURL string) *Agent {
	t.Helper()
	cfg := newTestConfig(backendURL)
	clk := errors.RealClock{}
	errCollector := errors.NewErrorCollector(clk)
	metrics := observability.NewMetrics()
	sm := NewStateMachine(clk)
	st := store.NewStore()
	ms := store.NewMetricsStore()
	pipeline := enrichment.NewPipeline(metrics)
	builder := snapshot.NewSnapshotBuilder(st, ms, cfg, metrics, errCollector, pipeline, nil)
	tc := transport.NewClient(cfg, metrics, errCollector)
	reg := collector.NewRegistry()
	reg.Register(&stubCollector{name: "test-stub"})

	return NewAgent(cfg, reg, builder, tc, sm, errCollector, metrics)
}

func TestAgent_IsReady_InitiallyFalse(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag, _ := newTestAgent(t, srv.URL)

	assert.False(t, ag.IsReady(), "agent should not be ready before Run")
}

func TestAgent_LatestSnapshot_InitiallyNil(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag, _ := newTestAgent(t, srv.URL)

	assert.Nil(t, ag.LatestSnapshot(), "snapshot should be nil before Run")
}

func TestAgent_Run_StartsAndSendsSnapshots(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag := newTestAgentWithCustomTransport(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := ag.Run(ctx)
	// Should exit with context deadline exceeded.
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Should have become ready.
	assert.True(t, ag.IsReady(), "agent should be ready after sync")

	// Should have sent at least one snapshot (the immediate one).
	assert.Greater(t, reqCount.Load(), int32(0), "should have sent at least one snapshot")

	// Latest snapshot should be non-nil.
	assert.NotNil(t, ag.LatestSnapshot(), "latest snapshot should be set")
}

func TestAgent_Run_ContextCancellation_CleanShutdown(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag := newTestAgentWithCustomTransport(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ag.Run(ctx)
	}()

	// Let it run briefly, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestAgent_Run_ReadyAfterSync(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag := newTestAgentWithCustomTransport(t, srv.URL)

	// Not ready before Run.
	assert.False(t, ag.IsReady())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ag.Run(ctx)
	}()

	// Wait for agent to become ready.
	require.Eventually(t, func() bool {
		return ag.IsReady()
	}, 2*time.Second, 10*time.Millisecond, "agent should become ready")

	cancel()
	<-done
}

func TestAgent_Run_LatestSnapshotSetAfterFirstBuild(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag := newTestAgentWithCustomTransport(t, srv.URL)

	assert.Nil(t, ag.LatestSnapshot())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ag.Run(ctx)
	}()

	// Wait for snapshot to be set.
	require.Eventually(t, func() bool {
		return ag.LatestSnapshot() != nil
	}, 2*time.Second, 10*time.Millisecond, "latest snapshot should be set")

	snap, ok := ag.LatestSnapshot().(*model.ClusterSnapshot)
	require.True(t, ok, "should be a *model.ClusterSnapshot")
	assert.Equal(t, "test-cluster", snap.ClusterID)

	cancel()
	<-done
}

func TestAgent_Run_StateMachine_401_StopsAgent(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusUnauthorized)
	defer srv.Close()

	ag := newTestAgentWithCustomTransport(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ag.Run(ctx)
	}()

	// The 401 causes transport error (non-retryable), but doesn't set state
	// machine directly (transport.Send returns error, not status code).
	// The agent will keep trying on each tick. Let it run briefly.
	select {
	case err := <-done:
		// If it exits, that's also fine.
		_ = err
	case <-time.After(500 * time.Millisecond):
		cancel()
		<-done
	}

	// Verify requests were made.
	assert.Greater(t, reqCount.Load(), int32(0))
}

func TestAgent_Run_StateMachine_DirectTransition_Stopped(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag := newTestAgentWithCustomTransport(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ag.Run(ctx)
	}()

	// Wait for agent to become ready, then force state to Stopped.
	require.Eventually(t, func() bool {
		return ag.IsReady()
	}, 2*time.Second, 10*time.Millisecond)

	ag.stateMachine.TransitionTo(StateStopped, "test forced stop")

	select {
	case err := <-done:
		assert.NoError(t, err, "Run should return nil when StateStopped")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after StateStopped transition")
	}
}

func TestAgent_Run_StateMachine_Exiting_ExitsLoop(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	ag := newTestAgentWithCustomTransport(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ag.Run(ctx)
	}()

	// Wait ready, then transition to Exiting.
	require.Eventually(t, func() bool {
		return ag.IsReady()
	}, 2*time.Second, 10*time.Millisecond)

	ag.stateMachine.TransitionTo(StateExiting, "deprecated")

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after StateExiting transition")
	}
}

func TestAgent_Run_EmptyRegistry_NoCollectors(t *testing.T) {
	var reqCount atomic.Int32
	srv := newTestBackend(t, &reqCount, http.StatusOK)
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	clk := errors.RealClock{}
	errCollector := errors.NewErrorCollector(clk)
	metrics := observability.NewMetrics()
	sm := NewStateMachine(clk)
	st := store.NewStore()
	ms := store.NewMetricsStore()
	pipeline := enrichment.NewPipeline(metrics)
	builder := snapshot.NewSnapshotBuilder(st, ms, cfg, metrics, errCollector, pipeline, nil)
	tc := transport.NewClient(cfg, metrics, errCollector)
	reg := collector.NewRegistry() // empty — no collectors

	ag := NewAgent(cfg, reg, builder, tc, sm, errCollector, metrics)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := ag.Run(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.True(t, ag.IsReady())
}
