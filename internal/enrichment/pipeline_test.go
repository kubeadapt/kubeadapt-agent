package enrichment

import (
	"errors"
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// stubEnricher records calls and optionally returns an error.
type stubEnricher struct {
	name   string
	err    error
	called bool
	// callIdx removed — was unused
}

func (s *stubEnricher) Name() string { return s.name }

func (s *stubEnricher) Enrich(_ *model.ClusterSnapshot) error {
	s.called = true
	return s.err
}

// orderTracker records the order enrichers were called.
type orderTracker struct {
	order []string
}

type trackingEnricher struct {
	name    string
	err     error
	tracker *orderTracker
}

func (t *trackingEnricher) Name() string { return t.name }

func (t *trackingEnricher) Enrich(_ *model.ClusterSnapshot) error {
	t.tracker.order = append(t.tracker.order, t.name)
	return t.err
}

func TestPipeline_AllEnrichersRunInOrder(t *testing.T) {
	metrics := observability.NewMetrics()
	tracker := &orderTracker{}

	e1 := &trackingEnricher{name: "first", tracker: tracker}
	e2 := &trackingEnricher{name: "second", tracker: tracker}
	e3 := &trackingEnricher{name: "third", tracker: tracker}

	p := NewPipeline(metrics, e1, e2, e3)
	snap := &model.ClusterSnapshot{}
	p.Run(snap)

	if len(tracker.order) != 3 {
		t.Fatalf("expected 3 enrichers called, got %d", len(tracker.order))
	}
	if tracker.order[0] != "first" || tracker.order[1] != "second" || tracker.order[2] != "third" {
		t.Fatalf("unexpected order: %v", tracker.order)
	}
}

func TestPipeline_OneEnricherFails_OthersStillRun(t *testing.T) {
	metrics := observability.NewMetrics()

	e1 := &stubEnricher{name: "ok-before"}
	e2 := &stubEnricher{name: "failing", err: errors.New("boom")}
	e3 := &stubEnricher{name: "ok-after"}

	p := NewPipeline(metrics, e1, e2, e3)
	snap := &model.ClusterSnapshot{}
	p.Run(snap)

	if !e1.called {
		t.Error("expected first enricher to be called")
	}
	if !e2.called {
		t.Error("expected failing enricher to be called")
	}
	if !e3.called {
		t.Error("expected third enricher to be called after failure")
	}
}

func TestPipeline_NilMetrics(t *testing.T) {
	e := &stubEnricher{name: "test"}
	p := NewPipeline(nil, e)
	snap := &model.ClusterSnapshot{}

	// Should not panic with nil metrics.
	p.Run(snap)

	if !e.called {
		t.Error("expected enricher to be called even with nil metrics")
	}
}

func TestPipeline_NoEnrichers(t *testing.T) {
	metrics := observability.NewMetrics()
	p := NewPipeline(metrics)
	snap := &model.ClusterSnapshot{}

	// Should not panic with no enrichers.
	p.Run(snap)
}
