package enrichment

import (
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func TestAggregation_DeploymentWithThreePods(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "web",
			Namespace: "default",
		}},
		Pods: []model.PodInfo{
			makePod("web-1", "default", "Deployment", "web", 0.5, 256*1024*1024, 1.0, 512*1024*1024),
			makePod("web-2", "default", "Deployment", "web", 0.5, 256*1024*1024, 1.0, 512*1024*1024),
			makePod("web-3", "default", "Deployment", "web", 0.5, 256*1024*1024, 1.0, 512*1024*1024),
		},
	}

	e := NewAggregationEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	d := snap.Deployments[0]
	if d.TotalCPURequest != 1.5 {
		t.Errorf("expected TotalCPURequest=1.5, got %f", d.TotalCPURequest)
	}
	if d.TotalMemoryRequest != 3*256*1024*1024 {
		t.Errorf("expected TotalMemoryRequest=%d, got %d", 3*256*1024*1024, d.TotalMemoryRequest)
	}
	if d.TotalCPULimit != 3.0 {
		t.Errorf("expected TotalCPULimit=3.0, got %f", d.TotalCPULimit)
	}
	if d.TotalMemoryLimit != 3*512*1024*1024 {
		t.Errorf("expected TotalMemoryLimit=%d, got %d", 3*512*1024*1024, d.TotalMemoryLimit)
	}
	if d.TotalCPUUsage != nil {
		t.Errorf("expected TotalCPUUsage=nil (no metrics), got %v", *d.TotalCPUUsage)
	}
}

func TestAggregation_DeploymentWithMetrics(t *testing.T) {
	cpuUse1 := 0.3
	memUse1 := int64(100 * 1024 * 1024)
	cpuUse2 := 0.7
	memUse2 := int64(200 * 1024 * 1024)

	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "api",
			Namespace: "default",
		}},
		Pods: []model.PodInfo{
			{
				Name:      "api-1",
				Namespace: "default",
				OwnerKind: "Deployment",
				OwnerName: "api",
				Containers: []model.ContainerInfo{{
					CPURequestCores:    0.5,
					MemoryRequestBytes: 256 * 1024 * 1024,
					CPULimitCores:      1.0,
					MemoryLimitBytes:   512 * 1024 * 1024,
					CPUUsageCores:      &cpuUse1,
					MemoryUsageBytes:   &memUse1,
				}},
			},
			{
				Name:      "api-2",
				Namespace: "default",
				OwnerKind: "Deployment",
				OwnerName: "api",
				Containers: []model.ContainerInfo{{
					CPURequestCores:    0.5,
					MemoryRequestBytes: 256 * 1024 * 1024,
					CPULimitCores:      1.0,
					MemoryLimitBytes:   512 * 1024 * 1024,
					CPUUsageCores:      &cpuUse2,
					MemoryUsageBytes:   &memUse2,
				}},
			},
		},
	}

	e := NewAggregationEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	d := snap.Deployments[0]
	if d.TotalCPUUsage == nil {
		t.Fatal("expected TotalCPUUsage to be set")
	}
	if *d.TotalCPUUsage != 1.0 {
		t.Errorf("expected TotalCPUUsage=1.0, got %f", *d.TotalCPUUsage)
	}
	if d.TotalMemoryUsage == nil {
		t.Fatal("expected TotalMemoryUsage to be set")
	}
	if *d.TotalMemoryUsage != 300*1024*1024 {
		t.Errorf("expected TotalMemoryUsage=%d, got %d", 300*1024*1024, *d.TotalMemoryUsage)
	}
}

func TestAggregation_DeploymentWithNoPods(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "empty",
			Namespace: "default",
		}},
		// No pods
	}

	e := NewAggregationEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	d := snap.Deployments[0]
	if d.TotalCPURequest != 0 {
		t.Errorf("expected TotalCPURequest=0, got %f", d.TotalCPURequest)
	}
	if d.TotalMemoryRequest != 0 {
		t.Errorf("expected TotalMemoryRequest=0, got %d", d.TotalMemoryRequest)
	}
	if d.TotalCPUUsage != nil {
		t.Errorf("expected TotalCPUUsage=nil, got %v", *d.TotalCPUUsage)
	}
}

func TestAggregation_StatefulSet(t *testing.T) {
	snap := &model.ClusterSnapshot{
		StatefulSets: []model.StatefulSetInfo{{
			Name:      "redis",
			Namespace: "default",
		}},
		Pods: []model.PodInfo{
			makePod("redis-0", "default", "StatefulSet", "redis", 0.25, 128*1024*1024, 0.5, 256*1024*1024),
			makePod("redis-1", "default", "StatefulSet", "redis", 0.25, 128*1024*1024, 0.5, 256*1024*1024),
		},
	}

	e := NewAggregationEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	s := snap.StatefulSets[0]
	if s.TotalCPURequest != 0.5 {
		t.Errorf("expected TotalCPURequest=0.5, got %f", s.TotalCPURequest)
	}
	if s.TotalMemoryRequest != 2*128*1024*1024 {
		t.Errorf("expected TotalMemoryRequest=%d, got %d", 2*128*1024*1024, s.TotalMemoryRequest)
	}
}

func TestAggregation_DaemonSet(t *testing.T) {
	snap := &model.ClusterSnapshot{
		DaemonSets: []model.DaemonSetInfo{{
			Name:      "monitor",
			Namespace: "kube-system",
		}},
		Pods: []model.PodInfo{
			makePod("monitor-abc", "kube-system", "DaemonSet", "monitor", 0.1, 64*1024*1024, 0.2, 128*1024*1024),
		},
	}

	e := NewAggregationEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	ds := snap.DaemonSets[0]
	if ds.TotalCPURequest != 0.1 {
		t.Errorf("expected TotalCPURequest=0.1, got %f", ds.TotalCPURequest)
	}
}

func TestAggregation_PodsInDifferentNamespace(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Deployments: []model.DeploymentInfo{{
			Name:      "app",
			Namespace: "prod",
		}},
		Pods: []model.PodInfo{
			// Same name but different namespace — should not match.
			makePod("app-pod", "staging", "Deployment", "app", 1.0, 1024*1024*1024, 2.0, 2*1024*1024*1024),
		},
	}

	e := NewAggregationEnricher()
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	d := snap.Deployments[0]
	if d.TotalCPURequest != 0 {
		t.Errorf("expected TotalCPURequest=0 (different namespace), got %f", d.TotalCPURequest)
	}
}

// makePod creates a PodInfo with a single container for testing.
func makePod(name, ns, ownerKind, ownerName string, cpuReq float64, memReq int64, cpuLim float64, memLim int64) model.PodInfo {
	return model.PodInfo{
		Name:      name,
		Namespace: ns,
		OwnerKind: ownerKind,
		OwnerName: ownerName,
		Containers: []model.ContainerInfo{{
			CPURequestCores:    cpuReq,
			MemoryRequestBytes: memReq,
			CPULimitCores:      cpuLim,
			MemoryLimitBytes:   memLim,
		}},
	}
}
