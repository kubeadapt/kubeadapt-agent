package enrichment

import (
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func TestMounts_NoOp(t *testing.T) {
	snap := &model.ClusterSnapshot{
		PVCs: []model.PVCInfo{{
			Name:      "data-pvc",
			Namespace: "default",
		}},
		Pods: []model.PodInfo{{
			Name:      "app-pod",
			Namespace: "default",
		}},
	}

	e := NewMountsEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatalf("expected no error from no-op enricher, got %v", err)
	}

	// MountedByPods should remain nil/empty since this is a no-op.
	if len(snap.PVCs[0].MountedByPods) != 0 {
		t.Errorf("expected MountedByPods to be empty, got %v", snap.PVCs[0].MountedByPods)
	}
}

func TestMounts_Name(t *testing.T) {
	e := NewMountsEnricher()
	if e.Name() != "mounts" {
		t.Errorf("expected name=mounts, got %s", e.Name())
	}
}

func TestMounts_ImplementsEnricher(t *testing.T) {
	// Compile-time check that MountsEnricher implements Enricher.
	var _ Enricher = (*MountsEnricher)(nil)
}
