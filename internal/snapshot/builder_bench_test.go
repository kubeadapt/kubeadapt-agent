package snapshot

import (
	"context"
	"fmt"
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	"github.com/kubeadapt/kubeadapt-agent/internal/enrichment"
	"github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// benchDeps creates all SnapshotBuilder dependencies for benchmarking.
func benchDeps() (*store.Store, *store.MetricsStore, *config.Config, *observability.Metrics, *errors.ErrorCollector) {
	cfg := &config.Config{
		ClusterID:    "bench-cluster",
		ClusterName:  "bench",
		AgentVersion: "bench-0.0.1",
	}
	metrics := observability.NewMetrics()
	errCollector := errors.NewErrorCollector(errors.RealClock{})
	return store.NewStore(), store.NewMetricsStore(), cfg, metrics, errCollector
}

func benchNodeInfo(i int) model.NodeInfo {
	return model.NodeInfo{
		Name:                  fmt.Sprintf("node-%d", i),
		ProviderID:            fmt.Sprintf("aws:///us-east-1a/i-%016d", i),
		InstanceID:            fmt.Sprintf("i-%016d", i),
		InstanceType:          "m5.xlarge",
		Region:                "us-east-1",
		Zone:                  "us-east-1a",
		CapacityType:          "on-demand",
		NodeGroup:             "default-pool",
		Architecture:          "amd64",
		OS:                    "linux",
		KubeletVersion:        "v1.29.1",
		ContainerRuntime:      "containerd://1.7.11",
		CPUCapacityCores:      4.0,
		MemoryCapacityBytes:   16 * 1024 * 1024 * 1024,
		EphemeralStorageBytes: 100 * 1024 * 1024 * 1024,
		PodCapacity:           110,
		CPUAllocatable:        3.92,
		MemoryAllocatable:     15 * 1024 * 1024 * 1024,
		PodAllocatable:        110,
		Ready:                 true,
		Taints: []model.TaintInfo{
			{Key: "dedicated", Value: "special", Effect: "NoSchedule"},
		},
		Conditions: []model.NodeConditionInfo{
			{Type: "Ready", Status: "True", Reason: "KubeletReady"},
		},
		Labels: map[string]string{
			"kubernetes.io/arch":               "amd64",
			"kubernetes.io/os":                 "linux",
			"node.kubernetes.io/instance-type": "m5.xlarge",
			"topology.kubernetes.io/zone":      "us-east-1a",
			"topology.kubernetes.io/region":    "us-east-1",
			"eks.amazonaws.com/capacityType":   "ON_DEMAND",
			"eks.amazonaws.com/nodegroup":      "default-pool",
		},
		CreationTimestamp: 1700000000000,
	}
}

func benchPodInfo(i int, nodeName string) model.PodInfo {
	return model.PodInfo{
		Name:      fmt.Sprintf("pod-%d", i),
		Namespace: fmt.Sprintf("ns-%d", i%20),
		NodeName:  nodeName,
		Phase:     "Running",
		QoSClass:  "Burstable",
		OwnerKind: "ReplicaSet",
		OwnerName: fmt.Sprintf("rs-%d", i%50),
		OwnerUID:  fmt.Sprintf("rs-uid-%d", i%50),
		Containers: []model.ContainerInfo{
			{
				Name:               "app",
				Image:              "nginx:1.21",
				CPURequestCores:    0.1,
				MemoryRequestBytes: 256 * 1024 * 1024,
				CPULimitCores:      0.5,
				MemoryLimitBytes:   512 * 1024 * 1024,
				Ready:              true,
				State:              "running",
				Ports: []model.ContainerPortInfo{
					{Name: "http", ContainerPort: 8080, Protocol: "TCP"},
				},
			},
			{
				Name:               "sidecar",
				Image:              "envoyproxy/envoy:v1.28",
				CPURequestCores:    0.05,
				MemoryRequestBytes: 64 * 1024 * 1024,
				CPULimitCores:      0.2,
				MemoryLimitBytes:   128 * 1024 * 1024,
				Ready:              true,
				State:              "running",
			},
		},
		Labels: map[string]string{
			"app":               fmt.Sprintf("app-%d", i%50),
			"version":           "v1",
			"pod-template-hash": "abc123",
		},
		Conditions: []model.PodConditionInfo{
			{Type: "Ready", Status: "True"},
			{Type: "PodScheduled", Status: "True"},
		},
		PriorityClassName:  "high-priority",
		SchedulerName:      "default-scheduler",
		ServiceAccountName: "default",
		CreationTimestamp:  1700000000000,
	}
}

func benchDeploymentInfo(i int) model.DeploymentInfo {
	return model.DeploymentInfo{
		Name:              fmt.Sprintf("deploy-%d", i),
		Namespace:         fmt.Sprintf("ns-%d", i%20),
		Replicas:          3,
		ReadyReplicas:     3,
		AvailableReplicas: 3,
		Strategy:          "RollingUpdate",
		Labels:            map[string]string{"app": fmt.Sprintf("app-%d", i)},
		CreationTimestamp: 1700000000000,
	}
}

func benchReplicaSetInfo(i int, deploymentName string) model.ReplicaSetInfo {
	return model.ReplicaSetInfo{
		Name:              fmt.Sprintf("rs-%d", i),
		Namespace:         fmt.Sprintf("ns-%d", i%20),
		Replicas:          3,
		ReadyReplicas:     3,
		OwnerKind:         "Deployment",
		OwnerName:         deploymentName,
		OwnerUID:          fmt.Sprintf("deploy-uid-%d", i),
		Labels:            map[string]string{"app": fmt.Sprintf("app-%d", i)},
		CreationTimestamp: 1700000000000,
	}
}

func benchServiceInfo(i int) model.ServiceInfo {
	return model.ServiceInfo{
		Name:      fmt.Sprintf("svc-%d", i),
		Namespace: fmt.Sprintf("ns-%d", i%20),
		Type:      "ClusterIP",
		ClusterIP: fmt.Sprintf("10.0.%d.%d", i/256, i%256),
		Ports: []model.ServicePortInfo{
			{Name: "http", Protocol: "TCP", Port: 80, TargetPort: "8080"},
		},
		Selector:          map[string]string{"app": fmt.Sprintf("app-%d", i)},
		Labels:            map[string]string{"app": fmt.Sprintf("app-%d", i)},
		CreationTimestamp: 1700000000000,
	}
}

func benchHPAInfo(i int) model.HPAInfo {
	min := int32(2)
	return model.HPAInfo{
		Name:            fmt.Sprintf("hpa-%d", i),
		Namespace:       fmt.Sprintf("ns-%d", i%20),
		TargetKind:      "Deployment",
		TargetName:      fmt.Sprintf("deploy-%d", i),
		MinReplicas:     &min,
		MaxReplicas:     10,
		CurrentReplicas: 3,
		DesiredReplicas: 3,
		Metrics: []model.HPAMetricInfo{
			{Type: "Resource", ResourceName: "cpu", TargetType: "Utilization", TargetValue: "70"},
		},
		Labels:            map[string]string{"app": fmt.Sprintf("app-%d", i)},
		CreationTimestamp: 1700000000000,
	}
}

// populateBenchStore fills the store with realistic data for snapshot builder benchmarking.
func populateBenchStore(s *store.Store, numNodes, numPods, numDeploys, numRS, numServices, numHPAs int) {
	for i := 0; i < numNodes; i++ {
		ni := benchNodeInfo(i)
		s.Nodes.Set(ni.Name, ni)
	}
	for i := 0; i < numPods; i++ {
		nodeName := fmt.Sprintf("node-%d", i%numNodes)
		pi := benchPodInfo(i, nodeName)
		key := fmt.Sprintf("%s/%s", pi.Namespace, pi.Name)
		s.Pods.Set(key, pi)
	}
	for i := 0; i < numDeploys; i++ {
		di := benchDeploymentInfo(i)
		key := fmt.Sprintf("%s/%s", di.Namespace, di.Name)
		s.Deployments.Set(key, di)
	}
	for i := 0; i < numRS; i++ {
		deployName := fmt.Sprintf("deploy-%d", i)
		ri := benchReplicaSetInfo(i, deployName)
		key := fmt.Sprintf("%s/%s", ri.Namespace, ri.Name)
		s.ReplicaSets.Set(key, ri)
	}
	for i := 0; i < numServices; i++ {
		si := benchServiceInfo(i)
		key := fmt.Sprintf("%s/%s", si.Namespace, si.Name)
		s.Services.Set(key, si)
	}
	for i := 0; i < numHPAs; i++ {
		hi := benchHPAInfo(i)
		key := fmt.Sprintf("%s/%s", hi.Namespace, hi.Name)
		s.HPAs.Set(key, hi)
	}
}

// BenchmarkBuild_100Nodes_2000Pods measures the full snapshot build pipeline
// with 100 nodes, 2000 pods, 50 deployments, 50 replicasets, 20 services, 10 HPAs.
func BenchmarkBuild_100Nodes_2000Pods(b *testing.B) {
	b.ReportAllocs()

	s, ms, cfg, metrics, errCollector := benchDeps()
	populateBenchStore(s, 100, 2000, 50, 50, 20, 10)

	// Build a simple enrichment pipeline with ownership resolution.
	replicaSets := s.ReplicaSets.Values()
	ownerEnricher := enrichment.NewOwnershipEnricher(replicaSets)
	pipeline := enrichment.NewPipeline(metrics, ownerEnricher)

	builder := NewSnapshotBuilder(s, ms, cfg, metrics, errCollector, pipeline, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snap := builder.Build(ctx)
		// Prevent compiler optimization.
		if snap.SnapshotID == "" {
			b.Fatal("snapshot ID is empty")
		}
	}
}
