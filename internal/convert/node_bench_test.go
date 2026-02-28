package convert

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func benchNode(i int) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("node-%d", i),
			Labels: map[string]string{
				"kubernetes.io/arch":               "amd64",
				"kubernetes.io/os":                 "linux",
				"node.kubernetes.io/instance-type": "m5.xlarge",
				"topology.kubernetes.io/zone":      "us-east-1a",
				"topology.kubernetes.io/region":    "us-east-1",
				"eks.amazonaws.com/capacityType":   "ON_DEMAND",
				"eks.amazonaws.com/nodegroup":      "default-pool",
				"app.kubernetes.io/managed-by":     "eks",
			},
			Annotations: map[string]string{
				"node.alpha.kubernetes.io/ttl":                           "0",
				"volumes.kubernetes.io/controller-managed-attach-detach": "true",
			},
			CreationTimestamp: metav1.Now(),
		},
		Spec: corev1.NodeSpec{
			ProviderID:    fmt.Sprintf("aws:///us-east-1a/i-%016d", i),
			Unschedulable: false,
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "special", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("4"),
				corev1.ResourceMemory:           resource.MustParse("16Gi"),
				corev1.ResourcePods:             resource.MustParse("110"),
				corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3920m"),
				corev1.ResourceMemory: resource.MustParse("15Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady", Message: "kubelet is posting ready status"},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse, Reason: "KubeletHasSufficientMemory"},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, Reason: "KubeletHasNoDiskPressure"},
				{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse, Reason: "KubeletHasSufficientPID"},
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.29.1",
				ContainerRuntimeVersion: "containerd://1.7.11",
			},
		},
	}
}

// BenchmarkNodeToModel measures the conversion of a realistic v1.Node
// to model.NodeInfo.
func BenchmarkNodeToModel(b *testing.B) {
	b.ReportAllocs()

	node := benchNode(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NodeToModel(node)
	}
}
