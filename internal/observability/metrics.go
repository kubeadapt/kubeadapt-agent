package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for agent self-monitoring.
// It uses a custom registry to avoid polluting the global default.
type Metrics struct {
	Registry *prometheus.Registry

	// Snapshot metrics
	SnapshotBuildDuration prometheus.Histogram
	SnapshotSendDuration  prometheus.Histogram
	SnapshotSizeBytes     *prometheus.HistogramVec
	SnapshotSendTotal     *prometheus.CounterVec

	// Informer metrics
	InformerEventsTotal *prometheus.CounterVec

	// Store metrics
	StoreItems *prometheus.GaugeVec

	// Enrichment metrics
	EnricherDuration *prometheus.HistogramVec

	// Transport metrics
	TransportRetries     prometheus.Counter
	TransportBufferBytes prometheus.Gauge

	// State metrics
	AgentState *prometheus.GaugeVec

	// Metrics API metrics
	MetricsAPIDuration prometheus.Histogram

	// Compression metrics
	CompressionRatio    prometheus.Gauge
	CompressionDuration prometheus.Histogram
}

// NewMetrics creates a new Metrics instance with all Prometheus metrics
// registered on a custom registry.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	sizeBuckets := prometheus.ExponentialBuckets(1024, 4, 10)

	m := &Metrics{
		Registry: reg,

		SnapshotBuildDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "kubeadapt_agent_snapshot_build_duration_seconds",
			Help:    "Duration of snapshot build operations in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
		SnapshotSendDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "kubeadapt_agent_snapshot_send_duration_seconds",
			Help:    "Duration of snapshot send operations in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
		SnapshotSizeBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kubeadapt_agent_snapshot_size_bytes",
			Help:    "Size of snapshots in bytes.",
			Buckets: sizeBuckets,
		}, []string{"type"}),
		SnapshotSendTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kubeadapt_agent_snapshot_send_total",
			Help: "Total number of snapshot send attempts.",
		}, []string{"status"}),

		InformerEventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kubeadapt_agent_informer_events_total",
			Help: "Total number of informer events received.",
		}, []string{"resource", "event"}),

		StoreItems: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kubeadapt_agent_store_items",
			Help: "Current number of items in the store.",
		}, []string{"resource"}),

		EnricherDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kubeadapt_agent_enricher_duration_seconds",
			Help:    "Duration of enricher operations in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"enricher"}),

		TransportRetries: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "kubeadapt_agent_transport_retries_total",
			Help: "Total number of transport retry attempts.",
		}),
		TransportBufferBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kubeadapt_agent_transport_buffer_bytes",
			Help: "Current size of the transport buffer in bytes.",
		}),

		AgentState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kubeadapt_agent_state",
			Help: "Current agent state (1 = active, 0 = inactive).",
		}, []string{"state"}),

		MetricsAPIDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "kubeadapt_agent_metrics_api_duration_seconds",
			Help:    "Duration of metrics API calls in seconds.",
			Buckets: prometheus.DefBuckets,
		}),

		CompressionRatio: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kubeadapt_agent_compression_ratio",
			Help: "Current compression ratio (compressed/original).",
		}),
		CompressionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "kubeadapt_agent_compression_duration_seconds",
			Help:    "Duration of compression operations in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
	}

	// Register all metrics with the custom registry.
	reg.MustRegister(
		m.SnapshotBuildDuration,
		m.SnapshotSendDuration,
		m.SnapshotSizeBytes,
		m.SnapshotSendTotal,
		m.InformerEventsTotal,
		m.StoreItems,
		m.EnricherDuration,
		m.TransportRetries,
		m.TransportBufferBytes,
		m.AgentState,
		m.MetricsAPIDuration,
		m.CompressionRatio,
		m.CompressionDuration,
	)

	return m
}
