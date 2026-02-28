package store

import "github.com/kubeadapt/kubeadapt-agent/pkg/model"

// MetricsStore holds metrics-server data separately from the main resource store.
type MetricsStore struct {
	NodeMetrics *TypedStore[model.NodeMetrics]
	PodMetrics  *TypedStore[model.PodMetrics]
}

// NewMetricsStore creates a MetricsStore with both typed stores initialized.
func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		NodeMetrics: NewTypedStore[model.NodeMetrics](),
		PodMetrics:  NewTypedStore[model.PodMetrics](),
	}
}
