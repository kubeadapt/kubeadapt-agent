package enrichment

import (
	"log/slog"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// Enricher transforms a ClusterSnapshot in-place, adding derived data.
type Enricher interface {
	Name() string
	Enrich(snapshot *model.ClusterSnapshot) error
}

// Pipeline runs a sequence of enrichers against a snapshot.
type Pipeline struct {
	enrichers []Enricher
	metrics   *observability.Metrics
}

// NewPipeline creates a pipeline that runs the given enrichers in order.
func NewPipeline(metrics *observability.Metrics, enrichers ...Enricher) *Pipeline {
	return &Pipeline{
		enrichers: enrichers,
		metrics:   metrics,
	}
}

// Run executes every enricher in order. If one fails, it logs a warning
// and continues with the remaining enrichers.
func (p *Pipeline) Run(snapshot *model.ClusterSnapshot) {
	for _, e := range p.enrichers {
		start := time.Now()
		if err := e.Enrich(snapshot); err != nil {
			slog.Warn("enricher failed", "enricher", e.Name(), "error", err)
		}
		duration := time.Since(start).Seconds()
		if p.metrics != nil {
			p.metrics.EnricherDuration.WithLabelValues(e.Name()).Observe(duration)
		}
	}
}
