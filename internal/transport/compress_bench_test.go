package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func benchSnapshot(numNodes, numPods int) *model.ClusterSnapshot {
	snap := &model.ClusterSnapshot{
		SnapshotID:   "bench-snapshot-id",
		ClusterID:    "bench-cluster",
		ClusterName:  "bench",
		Timestamp:    1700000000000,
		AgentVersion: "bench-0.0.1",
	}

	snap.Nodes = make([]model.NodeInfo, numNodes)
	for i := 0; i < numNodes; i++ {
		snap.Nodes[i] = model.NodeInfo{
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
			},
			CreationTimestamp: 1700000000000,
		}
	}

	snap.Pods = make([]model.PodInfo, numPods)
	for i := 0; i < numPods; i++ {
		snap.Pods[i] = model.PodInfo{
			Name:      fmt.Sprintf("pod-%d", i),
			Namespace: fmt.Sprintf("ns-%d", i%20),
			NodeName:  fmt.Sprintf("node-%d", i%numNodes),
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
			CreationTimestamp: 1700000000000,
		}
	}

	return snap
}

// BenchmarkStreamingCompress measures streaming zstd compression of a realistic
// ClusterSnapshot (100 nodes, 2000 pods) using io.Pipe, matching the production
// code path in Client.doSend.
func BenchmarkStreamingCompress(b *testing.B) {
	b.ReportAllocs()

	snap := benchSnapshot(100, 2000)

	// Pre-compute uncompressed size for comparison.
	uncompressedBuf, err := json.Marshal(snap)
	if err != nil {
		b.Fatal(err)
	}
	uncompressedSize := len(uncompressedBuf)
	b.Logf("uncompressed JSON size: %d bytes", uncompressedSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr, pw := io.Pipe()
		cw := NewCountingWriter(pw)

		zw, err := zstd.NewWriter(cw, zstd.WithEncoderLevel(zstd.SpeedDefault))
		if err != nil {
			b.Fatal(err)
		}

		// Writer goroutine: JSON → zstd → pipe
		errCh := make(chan error, 1)
		go func() {
			encErr := json.NewEncoder(zw).Encode(snap)
			closeErr := zw.Close()
			if encErr != nil {
				pw.CloseWithError(encErr)
				errCh <- encErr
			} else if closeErr != nil {
				pw.CloseWithError(closeErr)
				errCh <- closeErr
			} else {
				pw.Close()
				errCh <- nil
			}
		}()

		// Reader: drain the compressed output.
		var compressed bytes.Buffer
		if _, err := io.Copy(&compressed, pr); err != nil {
			b.Fatal(err)
		}

		if writeErr := <-errCh; writeErr != nil {
			b.Fatal(writeErr)
		}

		compressedSize := compressed.Len()
		b.ReportMetric(float64(compressedSize), "compressed-bytes")

		// Verify compression actually reduces size.
		if compressedSize >= uncompressedSize {
			b.Fatalf("compressed (%d) >= uncompressed (%d): compression not effective",
				compressedSize, uncompressedSize)
		}
	}
}
