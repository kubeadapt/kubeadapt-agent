package enrichment

import (
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// AggregationEnricher sums pod resource requests, limits, and usage
// per workload (Deployment, StatefulSet, DaemonSet, Job).
type AggregationEnricher struct{}

// NewAggregationEnricher creates a new AggregationEnricher.
func NewAggregationEnricher() *AggregationEnricher {
	return &AggregationEnricher{}
}

// Name returns the enricher name.
func (a *AggregationEnricher) Name() string { return "aggregation" }

// Enrich aggregates pod resources into workload totals.
func (a *AggregationEnricher) Enrich(snapshot *model.ClusterSnapshot) error {
	// Index pods by namespace+ownerKind+ownerName.
	type workloadKey struct {
		namespace string
		kind      string
		name      string
	}
	podsByWorkload := make(map[workloadKey][]model.PodInfo)
	for _, pod := range snapshot.Pods {
		if pod.OwnerKind == "" {
			continue
		}
		key := workloadKey{
			namespace: pod.Namespace,
			kind:      pod.OwnerKind,
			name:      pod.OwnerName,
		}
		podsByWorkload[key] = append(podsByWorkload[key], pod)
	}

	// Aggregate Deployments.
	for i := range snapshot.Deployments {
		d := &snapshot.Deployments[i]
		key := workloadKey{namespace: d.Namespace, kind: "Deployment", name: d.Name}
		pods := podsByWorkload[key]
		cpu, mem, cpuLim, memLim, cpuUse, memUse := sumPodResources(pods)
		d.TotalCPURequest = cpu
		d.TotalMemoryRequest = mem
		d.TotalCPULimit = cpuLim
		d.TotalMemoryLimit = memLim
		d.TotalCPUUsage = cpuUse
		d.TotalMemoryUsage = memUse
	}

	// Aggregate StatefulSets.
	for i := range snapshot.StatefulSets {
		s := &snapshot.StatefulSets[i]
		key := workloadKey{namespace: s.Namespace, kind: "StatefulSet", name: s.Name}
		pods := podsByWorkload[key]
		cpu, mem, cpuLim, memLim, cpuUse, memUse := sumPodResources(pods)
		s.TotalCPURequest = cpu
		s.TotalMemoryRequest = mem
		s.TotalCPULimit = cpuLim
		s.TotalMemoryLimit = memLim
		s.TotalCPUUsage = cpuUse
		s.TotalMemoryUsage = memUse
	}

	// Aggregate DaemonSets.
	for i := range snapshot.DaemonSets {
		ds := &snapshot.DaemonSets[i]
		key := workloadKey{namespace: ds.Namespace, kind: "DaemonSet", name: ds.Name}
		pods := podsByWorkload[key]
		cpu, mem, cpuLim, memLim, cpuUse, memUse := sumPodResources(pods)
		ds.TotalCPURequest = cpu
		ds.TotalMemoryRequest = mem
		ds.TotalCPULimit = cpuLim
		ds.TotalMemoryLimit = memLim
		ds.TotalCPUUsage = cpuUse
		ds.TotalMemoryUsage = memUse
	}

	// Aggregate Jobs.
	for i := range snapshot.Jobs {
		j := &snapshot.Jobs[i]
		key := workloadKey{namespace: j.Namespace, kind: "Job", name: j.Name}
		pods := podsByWorkload[key]
		cpu, mem, _, _, cpuUse, memUse := sumPodResources(pods)
		j.TotalCPURequest = cpu
		j.TotalMemoryRequest = mem
		j.TotalCPUUsage = cpuUse
		j.TotalMemoryUsage = memUse
	}

	return nil
}

// sumPodResources totals resource requests, limits, and usage across all
// containers in the given pods. Usage pointers are only set if at least one
// container has metrics.
func sumPodResources(pods []model.PodInfo) (
	cpuReq float64, memReq int64,
	cpuLim float64, memLim int64,
	cpuUsage *float64, memUsage *int64,
) {
	hasUsage := false
	var cpuUseTotal float64
	var memUseTotal int64

	for _, pod := range pods {
		for _, c := range pod.Containers {
			cpuReq += c.CPURequestCores
			memReq += c.MemoryRequestBytes
			cpuLim += c.CPULimitCores
			memLim += c.MemoryLimitBytes

			if c.CPUUsageCores != nil {
				cpuUseTotal += *c.CPUUsageCores
				hasUsage = true
			}
			if c.MemoryUsageBytes != nil {
				memUseTotal += *c.MemoryUsageBytes
				hasUsage = true
			}
		}
	}

	if hasUsage {
		cpuUsage = &cpuUseTotal
		memUsage = &memUseTotal
	}
	return
}
