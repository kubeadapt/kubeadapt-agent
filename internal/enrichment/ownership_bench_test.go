package enrichment

import (
	"fmt"
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// BenchmarkOwnershipResolution_10000Pods measures the ownership enrichment
// pipeline resolving Pod → ReplicaSet → Deployment chains for 10000 pods.
// 500 Deployments, 500 ReplicaSets, 10000 Pods.
func BenchmarkOwnershipResolution_10000Pods(b *testing.B) {
	b.ReportAllocs()

	const (
		numDeploys = 500
		numRS      = 500
		numPods    = 10000
	)

	// Build ReplicaSets, each owned by a Deployment.
	replicaSets := make([]model.ReplicaSetInfo, numRS)
	for i := 0; i < numRS; i++ {
		replicaSets[i] = model.ReplicaSetInfo{
			Name:      fmt.Sprintf("rs-%d", i),
			Namespace: fmt.Sprintf("ns-%d", i%20),
			Replicas:  int32(numPods / numRS),
			OwnerKind: "Deployment",
			OwnerName: fmt.Sprintf("deploy-%d", i%numDeploys),
			OwnerUID:  fmt.Sprintf("deploy-uid-%d", i%numDeploys),
			Labels:    map[string]string{"app": fmt.Sprintf("app-%d", i)},
		}
	}

	// Build a snapshot template with pods and jobs.
	// We'll clone it each iteration to avoid mutation effects.
	buildSnapshot := func() *model.ClusterSnapshot {
		snap := &model.ClusterSnapshot{}

		snap.Pods = make([]model.PodInfo, numPods)
		for i := 0; i < numPods; i++ {
			rsIdx := i % numRS
			snap.Pods[i] = model.PodInfo{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: fmt.Sprintf("ns-%d", rsIdx%20),
				NodeName:  fmt.Sprintf("node-%d", i%100),
				Phase:     "Running",
				OwnerKind: "ReplicaSet",
				OwnerName: fmt.Sprintf("rs-%d", rsIdx),
				OwnerUID:  fmt.Sprintf("rs-uid-%d", rsIdx),
				Containers: []model.ContainerInfo{
					{
						Name:               "app",
						Image:              "nginx:1.21",
						CPURequestCores:    0.1,
						MemoryRequestBytes: 256 * 1024 * 1024,
						Ready:              true,
						State:              "running",
					},
				},
				Labels: map[string]string{
					"app": fmt.Sprintf("app-%d", rsIdx),
				},
			}
		}

		return snap
	}

	enricher := NewOwnershipEnricher(replicaSets)

	// Pre-build snapshot once to verify it works.
	testSnap := buildSnapshot()
	if err := enricher.Enrich(testSnap); err != nil {
		b.Fatal(err)
	}
	// Verify resolution actually happened.
	if testSnap.Pods[0].OwnerKind != "Deployment" {
		b.Fatalf("expected OwnerKind=Deployment, got %s", testSnap.Pods[0].OwnerKind)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snap := buildSnapshot()
		if err := enricher.Enrich(snap); err != nil {
			b.Fatal(err)
		}
	}
}
