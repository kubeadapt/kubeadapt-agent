package enrichment

import (
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// MountsEnricher resolves PVC → Pod mount relationships.
// Currently a no-op because PodInfo does not include volume data.
// The mount resolution will be implemented when pod volume information
// is added to the model.
type MountsEnricher struct{}

// NewMountsEnricher creates a new MountsEnricher.
func NewMountsEnricher() *MountsEnricher {
	return &MountsEnricher{}
}

// Name returns the enricher name.
func (m *MountsEnricher) Name() string { return "mounts" }

// Enrich is a no-op until pod volume data is available in the model.
func (m *MountsEnricher) Enrich(_ *model.ClusterSnapshot) error {
	return nil
}
