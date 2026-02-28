package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewMetrics_NoRegistrationPanic(t *testing.T) {
	// Creating metrics should not panic.
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.Registry == nil {
		t.Fatal("Registry is nil")
	}
}

func TestNewMetrics_CustomRegistry(t *testing.T) {
	m := NewMetrics()

	// Gather from our custom registry — should have metrics.
	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	// Gather from the default registry — our metrics should NOT be there.
	defaultFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("DefaultGatherer.Gather failed: %v", err)
	}

	customNames := make(map[string]bool)
	for _, f := range families {
		customNames[f.GetName()] = true
	}

	for _, f := range defaultFamilies {
		if customNames[f.GetName()] {
			t.Errorf("metric %q found in default registry — should only be in custom registry", f.GetName())
		}
	}
}

func TestNewMetrics_AllNamesHavePrefix(t *testing.T) {
	m := NewMetrics()

	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	if len(families) == 0 {
		t.Fatal("no metric families gathered")
	}

	for _, f := range families {
		name := f.GetName()
		// Prometheus adds _total suffix for counters and _bucket/_sum/_count for histograms,
		// but the family name should still start with our prefix.
		if len(name) < len("kubeadapt_agent_") || name[:16] != "kubeadapt_agent_" {
			t.Errorf("metric %q does not start with kubeadapt_agent_ prefix", name)
		}
	}
}

func TestNewMetrics_CounterIncrement(t *testing.T) {
	m := NewMetrics()

	// Increment a plain counter.
	m.TransportRetries.Inc()

	pb := &dto.Metric{}
	if err := m.TransportRetries.Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetCounter().GetValue(); got != 1 {
		t.Errorf("TransportRetries = %v, want 1", got)
	}

	// Increment a counter vec.
	m.SnapshotSendTotal.WithLabelValues("success").Inc()
	m.SnapshotSendTotal.WithLabelValues("success").Inc()
	m.SnapshotSendTotal.WithLabelValues("error").Inc()

	pb = &dto.Metric{}
	if err := m.SnapshotSendTotal.WithLabelValues("success").(prometheus.Metric).Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetCounter().GetValue(); got != 2 {
		t.Errorf("SnapshotSendTotal(success) = %v, want 2", got)
	}
}

func TestNewMetrics_HistogramObserve(t *testing.T) {
	m := NewMetrics()

	m.SnapshotBuildDuration.Observe(0.5)
	m.SnapshotBuildDuration.Observe(1.5)

	pb := &dto.Metric{}
	if err := m.SnapshotBuildDuration.Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetHistogram().GetSampleCount(); got != 2 {
		t.Errorf("SnapshotBuildDuration sample count = %v, want 2", got)
	}

	// HistogramVec
	m.SnapshotSizeBytes.WithLabelValues("original").Observe(2048)
	pb = &dto.Metric{}
	if err := m.SnapshotSizeBytes.WithLabelValues("original").(prometheus.Metric).Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetHistogram().GetSampleCount(); got != 1 {
		t.Errorf("SnapshotSizeBytes(original) sample count = %v, want 1", got)
	}
}

func TestNewMetrics_GaugeSet(t *testing.T) {
	m := NewMetrics()

	m.TransportBufferBytes.Set(4096)

	pb := &dto.Metric{}
	if err := m.TransportBufferBytes.Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetGauge().GetValue(); got != 4096 {
		t.Errorf("TransportBufferBytes = %v, want 4096", got)
	}

	m.CompressionRatio.Set(0.75)
	pb = &dto.Metric{}
	if err := m.CompressionRatio.Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetGauge().GetValue(); got != 0.75 {
		t.Errorf("CompressionRatio = %v, want 0.75", got)
	}
}

func TestNewMetrics_VecLabels(t *testing.T) {
	m := NewMetrics()

	// InformerEventsTotal has labels: resource, event
	m.InformerEventsTotal.WithLabelValues("pods", "add").Inc()
	m.InformerEventsTotal.WithLabelValues("pods", "update").Inc()
	m.InformerEventsTotal.WithLabelValues("deployments", "delete").Inc()

	pb := &dto.Metric{}
	if err := m.InformerEventsTotal.WithLabelValues("pods", "add").(prometheus.Metric).Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetCounter().GetValue(); got != 1 {
		t.Errorf("InformerEventsTotal(pods,add) = %v, want 1", got)
	}

	// StoreItems has label: resource
	m.StoreItems.WithLabelValues("services").Set(42)
	pb = &dto.Metric{}
	if err := m.StoreItems.WithLabelValues("services").(prometheus.Metric).Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetGauge().GetValue(); got != 42 {
		t.Errorf("StoreItems(services) = %v, want 42", got)
	}

	// EnricherDuration has label: enricher
	m.EnricherDuration.WithLabelValues("node-info").Observe(0.1)
	pb = &dto.Metric{}
	if err := m.EnricherDuration.WithLabelValues("node-info").(prometheus.Metric).Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetHistogram().GetSampleCount(); got != 1 {
		t.Errorf("EnricherDuration(node-info) sample count = %v, want 1", got)
	}

	// AgentState has label: state
	m.AgentState.WithLabelValues("running").Set(1)
	m.AgentState.WithLabelValues("starting").Set(0)
	pb = &dto.Metric{}
	if err := m.AgentState.WithLabelValues("running").(prometheus.Metric).Write(pb); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := pb.GetGauge().GetValue(); got != 1 {
		t.Errorf("AgentState(running) = %v, want 1", got)
	}
}

func TestNewMetrics_NoDuplicateRegistrationPanic(t *testing.T) {
	// Creating two separate Metrics instances should not panic
	// because each uses its own registry.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("creating Metrics twice panicked: %v", r)
		}
	}()

	_ = NewMetrics()
	_ = NewMetrics()
}

func TestNewMetrics_AllFieldsNonNil(t *testing.T) {
	m := NewMetrics()

	if m.SnapshotBuildDuration == nil {
		t.Error("SnapshotBuildDuration is nil")
	}
	if m.SnapshotSendDuration == nil {
		t.Error("SnapshotSendDuration is nil")
	}
	if m.SnapshotSizeBytes == nil {
		t.Error("SnapshotSizeBytes is nil")
	}
	if m.SnapshotSendTotal == nil {
		t.Error("SnapshotSendTotal is nil")
	}
	if m.InformerEventsTotal == nil {
		t.Error("InformerEventsTotal is nil")
	}
	if m.StoreItems == nil {
		t.Error("StoreItems is nil")
	}
	if m.EnricherDuration == nil {
		t.Error("EnricherDuration is nil")
	}
	if m.TransportRetries == nil {
		t.Error("TransportRetries is nil")
	}
	if m.TransportBufferBytes == nil {
		t.Error("TransportBufferBytes is nil")
	}
	if m.AgentState == nil {
		t.Error("AgentState is nil")
	}
	if m.MetricsAPIDuration == nil {
		t.Error("MetricsAPIDuration is nil")
	}
	if m.CompressionRatio == nil {
		t.Error("CompressionRatio is nil")
	}
	if m.CompressionDuration == nil {
		t.Error("CompressionDuration is nil")
	}
}
