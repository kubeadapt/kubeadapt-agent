package enrichment

import (
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func TestTargets_PDBMatchesDeployment(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "web",
			Namespace: "default",
			Selector:  map[string]string{"app": "web", "tier": "frontend"},
		}},
		PDBs: []model.PDBInfo{{
			Name:        "web-pdb",
			Namespace:   "default",
			MatchLabels: map[string]string{"app": "web"},
		}},
	}

	e := NewTargetsEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pdb := snap.PDBs[0]
	if len(pdb.TargetWorkloads) != 1 {
		t.Fatalf("expected 1 target workload, got %d", len(pdb.TargetWorkloads))
	}
	ref := pdb.TargetWorkloads[0]
	if ref.Kind != "Deployment" || ref.Name != "web" || ref.Namespace != "default" {
		t.Errorf("unexpected target: %+v", ref)
	}
}

func TestTargets_ServiceMatchesDeployment(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "api",
			Namespace: "default",
			Selector:  map[string]string{"app": "api"},
		}},
		Services: []model.ServiceInfo{{
			Name:      "api-svc",
			Namespace: "default",
			Selector:  map[string]string{"app": "api"},
		}},
	}

	e := NewTargetsEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	svc := snap.Services[0]
	if len(svc.TargetWorkloads) != 1 {
		t.Fatalf("expected 1 target workload, got %d", len(svc.TargetWorkloads))
	}
	ref := svc.TargetWorkloads[0]
	if ref.Kind != "Deployment" || ref.Name != "api" || ref.Namespace != "default" {
		t.Errorf("unexpected target: %+v", ref)
	}
}

func TestTargets_NoMatchDifferentNamespace(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "web",
			Namespace: "prod",
			Selector:  map[string]string{"app": "web"},
		}},
		PDBs: []model.PDBInfo{{
			Name:        "web-pdb",
			Namespace:   "staging",
			MatchLabels: map[string]string{"app": "web"},
		}},
	}

	e := NewTargetsEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pdb := snap.PDBs[0]
	if len(pdb.TargetWorkloads) != 0 {
		t.Errorf("expected 0 targets (different namespace), got %d", len(pdb.TargetWorkloads))
	}
}

func TestTargets_NoMatchDifferentLabels(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "web",
			Namespace: "default",
			Selector:  map[string]string{"app": "web"},
		}},
		PDBs: []model.PDBInfo{{
			Name:        "api-pdb",
			Namespace:   "default",
			MatchLabels: map[string]string{"app": "api"},
		}},
	}

	e := NewTargetsEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pdb := snap.PDBs[0]
	if len(pdb.TargetWorkloads) != 0 {
		t.Errorf("expected 0 targets (labels don't match), got %d", len(pdb.TargetWorkloads))
	}
}

func TestTargets_MatchesMultipleWorkloads(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "web-v1",
			Namespace: "default",
			Selector:  map[string]string{"app": "web", "version": "v1"},
		}},
		StatefulSets: []model.StatefulSetInfo{{
			Name:      "web-cache",
			Namespace: "default",
			Selector:  map[string]string{"app": "web", "component": "cache"},
		}},
		Services: []model.ServiceInfo{{
			Name:      "web-svc",
			Namespace: "default",
			Selector:  map[string]string{"app": "web"},
		}},
	}

	e := NewTargetsEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	svc := snap.Services[0]
	if len(svc.TargetWorkloads) != 2 {
		t.Fatalf("expected 2 target workloads, got %d", len(svc.TargetWorkloads))
	}
}

func TestTargets_EmptySelector(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "web",
			Namespace: "default",
			Selector:  map[string]string{"app": "web"},
		}},
		PDBs: []model.PDBInfo{{
			Name:      "no-selector-pdb",
			Namespace: "default",
			// No MatchLabels
		}},
		Services: []model.ServiceInfo{{
			Name:      "headless",
			Namespace: "default",
			// No Selector
		}},
	}

	e := NewTargetsEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	if len(snap.PDBs[0].TargetWorkloads) != 0 {
		t.Error("expected 0 targets for PDB with no selector")
	}
	if len(snap.Services[0].TargetWorkloads) != 0 {
		t.Error("expected 0 targets for Service with no selector")
	}
}
