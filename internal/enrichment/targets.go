package enrichment

import (
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// TargetsEnricher resolves PDB and Service targets by matching label
// selectors against workload selectors in the same namespace.
type TargetsEnricher struct{}

// NewTargetsEnricher creates a new TargetsEnricher.
func NewTargetsEnricher() *TargetsEnricher {
	return &TargetsEnricher{}
}

// Name returns the enricher name.
func (te *TargetsEnricher) Name() string { return "targets" }

// workloadEntry groups a workload's selector with its identity.
type workloadEntry struct {
	kind      string
	name      string
	namespace string
	selector  map[string]string
}

// Enrich populates TargetWorkloads on PDBs and Services.
func (te *TargetsEnricher) Enrich(snapshot *model.ClusterSnapshot) error {
	workloads := te.collectWorkloads(snapshot)

	// Index workloads by namespace for efficient lookup.
	byNamespace := make(map[string][]workloadEntry)
	for _, w := range workloads {
		byNamespace[w.namespace] = append(byNamespace[w.namespace], w)
	}

	// Match PDBs.
	for i := range snapshot.PDBs {
		pdb := &snapshot.PDBs[i]
		if len(pdb.MatchLabels) == 0 {
			continue
		}
		pdb.TargetWorkloads = te.matchWorkloads(pdb.MatchLabels, byNamespace[pdb.Namespace])
	}

	// Match Services.
	for i := range snapshot.Services {
		svc := &snapshot.Services[i]
		if len(svc.Selector) == 0 {
			continue
		}
		svc.TargetWorkloads = te.matchWorkloads(svc.Selector, byNamespace[svc.Namespace])
	}

	return nil
}

// collectWorkloads gathers all workloads with their selectors.
func (te *TargetsEnricher) collectWorkloads(snapshot *model.ClusterSnapshot) []workloadEntry {
	var entries []workloadEntry

	for _, d := range snapshot.Deployments {
		if len(d.Selector) > 0 {
			entries = append(entries, workloadEntry{
				kind: "Deployment", name: d.Name, namespace: d.Namespace, selector: d.Selector,
			})
		}
	}
	for _, s := range snapshot.StatefulSets {
		if len(s.Selector) > 0 {
			entries = append(entries, workloadEntry{
				kind: "StatefulSet", name: s.Name, namespace: s.Namespace, selector: s.Selector,
			})
		}
	}
	for _, ds := range snapshot.DaemonSets {
		if len(ds.Selector) > 0 {
			entries = append(entries, workloadEntry{
				kind: "DaemonSet", name: ds.Name, namespace: ds.Namespace, selector: ds.Selector,
			})
		}
	}

	return entries
}

// matchWorkloads returns workload references for workloads whose selector
// contains ALL labels from the given selector. A workload matches if
// every key-value pair in the selector exists in the workload's selector.
func (te *TargetsEnricher) matchWorkloads(selector map[string]string, workloads []workloadEntry) []model.WorkloadReference {
	var refs []model.WorkloadReference
	for _, w := range workloads {
		if labelsMatch(selector, w.selector) {
			refs = append(refs, model.WorkloadReference{
				Kind:      w.kind,
				Name:      w.name,
				Namespace: w.namespace,
			})
		}
	}
	return refs
}

// labelsMatch returns true if all key-value pairs in selector exist
// in the target labels.
func labelsMatch(selector, target map[string]string) bool {
	for k, v := range selector {
		if target[k] != v {
			return false
		}
	}
	return true
}
