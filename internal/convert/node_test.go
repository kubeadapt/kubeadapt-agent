package convert

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// makeNode returns a fully populated test node (AWS EKS on-demand).
func makeNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ip-10-0-1-100.ec2.internal",
			UID:               "node-uid-abc123",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)),
			Labels: map[string]string{
				"kubernetes.io/arch":               "amd64",
				"kubernetes.io/os":                 "linux",
				"node.kubernetes.io/instance-type": "m5.xlarge",
				"topology.kubernetes.io/region":    "us-east-1",
				"topology.kubernetes.io/zone":      "us-east-1a",
				"eks.amazonaws.com/capacityType":   "ON_DEMAND",
				"eks.amazonaws.com/nodegroup":      "workers",
				"node.kubernetes.io/lifecycle":     "normal",
			},
			Annotations: map[string]string{
				"node.alpha.kubernetes.io/ttl":                           "0",
				"volumes.kubernetes.io/controller-managed-attach-detach": "true",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID:    "aws:///us-east-1a/i-0abcdef1234567890",
			Unschedulable: false,
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "special", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("4"),
				corev1.ResourceMemory:           resource.MustParse("16Gi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				corev1.ResourcePods:             resource.MustParse("58"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("3920m"),
				corev1.ResourceMemory:           resource.MustParse("15Gi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("95Gi"),
				corev1.ResourcePods:             resource.MustParse("58"),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady", Message: "kubelet is posting ready status"},
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.29.1",
				ContainerRuntimeVersion: "containerd://1.7.2",
			},
		},
	}
}

func TestNodeToModel_BasicNode(t *testing.T) {
	node := makeNode()
	got := NodeToModel(node)

	// Identity
	assertEqual(t, "Name", got.Name, "ip-10-0-1-100.ec2.internal")
	assertEqual(t, "UID", got.UID, "node-uid-abc123")
	assertEqual(t, "ProviderID", got.ProviderID, "aws:///us-east-1a/i-0abcdef1234567890")
	assertEqual(t, "InstanceID", got.InstanceID, "i-0abcdef1234567890")

	// Region/Zone prefer labels over providerID
	assertEqual(t, "Region", got.Region, "us-east-1")
	assertEqual(t, "Zone", got.Zone, "us-east-1a")

	// Cloud metadata
	assertEqual(t, "InstanceType", got.InstanceType, "m5.xlarge")
	assertEqual(t, "CapacityType", got.CapacityType, "on-demand")
	assertEqual(t, "NodeGroup", got.NodeGroup, "workers")

	// System info
	assertEqual(t, "Architecture", got.Architecture, "amd64")
	assertEqual(t, "OS", got.OS, "linux")
	assertEqual(t, "KubeletVersion", got.KubeletVersion, "v1.29.1")
	assertEqual(t, "ContainerRuntime", got.ContainerRuntime, "containerd://1.7.2")

	// Capacity
	assertFloat64(t, "CPUCapacityCores", got.CPUCapacityCores, 4.0)
	assertInt64(t, "MemoryCapacityBytes", got.MemoryCapacityBytes, 16*1024*1024*1024)
	assertInt64(t, "EphemeralStorageBytes", got.EphemeralStorageBytes, 100*1024*1024*1024)
	assertInt64(t, "EphemeralStorageAllocatable", got.EphemeralStorageAllocatable, 95*1024*1024*1024)
	assertInt(t, "PodCapacity", got.PodCapacity, 58)
	assertInt(t, "GPUCapacity", got.GPUCapacity, 0)

	// Allocatable
	assertFloat64(t, "CPUAllocatable", got.CPUAllocatable, 3.92)
	assertInt64(t, "MemoryAllocatable", got.MemoryAllocatable, 15*1024*1024*1024)
	assertInt(t, "PodAllocatable", got.PodAllocatable, 58)
	assertInt(t, "GPUAllocatable", got.GPUAllocatable, 0)

	// Usage should be nil (merged later)
	if got.CPUUsageCores != nil {
		t.Errorf("CPUUsageCores should be nil, got %v", *got.CPUUsageCores)
	}
	if got.MemoryUsageBytes != nil {
		t.Errorf("MemoryUsageBytes should be nil, got %v", *got.MemoryUsageBytes)
	}

	// Status
	if !got.Ready {
		t.Error("Ready should be true")
	}
	if got.Unschedulable {
		t.Error("Unschedulable should be false")
	}

	// Taints
	if len(got.Taints) != 1 {
		t.Fatalf("expected 1 taint, got %d", len(got.Taints))
	}
	assertEqual(t, "Taint.Key", got.Taints[0].Key, "dedicated")
	assertEqual(t, "Taint.Value", got.Taints[0].Value, "special")
	assertEqual(t, "Taint.Effect", got.Taints[0].Effect, "NoSchedule")

	// Conditions
	if len(got.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(got.Conditions))
	}
	assertEqual(t, "Condition.Type", got.Conditions[0].Type, "Ready")
	assertEqual(t, "Condition.Status", got.Conditions[0].Status, "True")
	assertEqual(t, "Condition.Reason", got.Conditions[0].Reason, "KubeletReady")

	// Labels — all labels, no filtering
	if len(got.Labels) != 8 {
		t.Errorf("expected 8 labels, got %d", len(got.Labels))
	}

	// Annotations — filtered
	if len(got.Annotations) != 2 {
		t.Errorf("expected 2 annotations, got %d", len(got.Annotations))
	}

	// CreationTimestamp
	expectedTS := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli()
	assertInt64(t, "CreationTimestamp", got.CreationTimestamp, expectedTS)
}

func TestNodeToModel_GPUNode(t *testing.T) {
	node := makeNode()
	node.Name = "gpu-node-1"
	node.Status.Capacity["nvidia.com/gpu"] = resource.MustParse("4")
	node.Status.Allocatable["nvidia.com/gpu"] = resource.MustParse("4")

	got := NodeToModel(node)

	assertInt(t, "GPUCapacity", got.GPUCapacity, 4)
	assertInt(t, "GPUAllocatable", got.GPUAllocatable, 4)
}

func TestNodeToModel_SpotInstance(t *testing.T) {
	node := makeNode()
	node.Labels["eks.amazonaws.com/capacityType"] = "SPOT"

	got := NodeToModel(node)

	assertEqual(t, "CapacityType", got.CapacityType, "spot")
}

func TestNodeToModel_FargateNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "fargate-ip-10-0-1-50.ec2.internal",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)),
			Labels: map[string]string{
				"eks.amazonaws.com/compute-type": "fargate",
				"topology.kubernetes.io/region":  "us-west-2",
				"topology.kubernetes.io/zone":    "us-west-2a",
				"kubernetes.io/arch":             "amd64",
				"kubernetes.io/os":               "linux",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: "aws:///us-west-2a/fargate-ip-10-0-1-50",
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
				corev1.ResourcePods:   resource.MustParse("1"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
				corev1.ResourcePods:   resource.MustParse("1"),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady", Message: "kubelet is posting ready status"},
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.29.1-eks-fargate",
				ContainerRuntimeVersion: "containerd://1.7.2",
			},
		},
	}

	got := NodeToModel(node)

	assertEqual(t, "CapacityType", got.CapacityType, "fargate")
	assertEqual(t, "InstanceID", got.InstanceID, "fargate-ip-10-0-1-50")
	assertEqual(t, "Region", got.Region, "us-west-2")
	assertEqual(t, "Zone", got.Zone, "us-west-2a")
	assertInt(t, "PodCapacity", got.PodCapacity, 1)
}

func TestNodeToModel_UnschedulableNode(t *testing.T) {
	node := makeNode()
	node.Spec.Unschedulable = true

	got := NodeToModel(node)

	if !got.Unschedulable {
		t.Error("Unschedulable should be true")
	}
}

func TestNodeToModel_MultipleTaints(t *testing.T) {
	node := makeNode()
	node.Spec.Taints = []corev1.Taint{
		{Key: "node-role.kubernetes.io/master", Value: "", Effect: corev1.TaintEffectNoSchedule},
		{Key: "node.kubernetes.io/not-ready", Value: "", Effect: corev1.TaintEffectNoExecute},
		{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectPreferNoSchedule},
	}

	got := NodeToModel(node)

	if len(got.Taints) != 3 {
		t.Fatalf("expected 3 taints, got %d", len(got.Taints))
	}
	assertEqual(t, "Taint[0].Key", got.Taints[0].Key, "node-role.kubernetes.io/master")
	assertEqual(t, "Taint[0].Effect", got.Taints[0].Effect, "NoSchedule")
	assertEqual(t, "Taint[1].Key", got.Taints[1].Key, "node.kubernetes.io/not-ready")
	assertEqual(t, "Taint[1].Effect", got.Taints[1].Effect, "NoExecute")
	assertEqual(t, "Taint[2].Key", got.Taints[2].Key, "dedicated")
	assertEqual(t, "Taint[2].Value", got.Taints[2].Value, "gpu")
	assertEqual(t, "Taint[2].Effect", got.Taints[2].Effect, "PreferNoSchedule")
}

func TestNodeToModel_MultipleConditions(t *testing.T) {
	node := makeNode()
	node.Status.Conditions = []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady", Message: "kubelet is posting ready status"},
		{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse, Reason: "KubeletHasSufficientMemory", Message: "kubelet has sufficient memory available"},
		{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, Reason: "KubeletHasNoDiskPressure", Message: "kubelet has no disk pressure"},
	}

	got := NodeToModel(node)

	if len(got.Conditions) != 3 {
		t.Fatalf("expected 3 conditions, got %d", len(got.Conditions))
	}
	// Ready
	assertEqual(t, "Condition[0].Type", got.Conditions[0].Type, "Ready")
	assertEqual(t, "Condition[0].Status", got.Conditions[0].Status, "True")
	// MemoryPressure
	assertEqual(t, "Condition[1].Type", got.Conditions[1].Type, "MemoryPressure")
	assertEqual(t, "Condition[1].Status", got.Conditions[1].Status, "False")
	// DiskPressure
	assertEqual(t, "Condition[2].Type", got.Conditions[2].Type, "DiskPressure")
	assertEqual(t, "Condition[2].Status", got.Conditions[2].Status, "False")

	// Ready should still be true since NodeReady=True
	if !got.Ready {
		t.Error("Ready should be true")
	}
}

func TestNodeToModel_NilLabelsAnnotations(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "bare-node",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
			Labels:            nil,
			Annotations:       nil,
		},
		Spec: corev1.NodeSpec{},
		Status: corev1.NodeStatus{
			Capacity:    corev1.ResourceList{},
			Allocatable: corev1.ResourceList{},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "NotReady", Message: "node not ready"},
			},
			NodeInfo: corev1.NodeSystemInfo{},
		},
	}

	got := NodeToModel(node)

	assertEqual(t, "Name", got.Name, "bare-node")
	// nil maps should produce nil/empty results, not panic
	assertEqual(t, "InstanceType", got.InstanceType, "")
	assertEqual(t, "CapacityType", got.CapacityType, "")
	assertEqual(t, "NodeGroup", got.NodeGroup, "")
	assertEqual(t, "Architecture", got.Architecture, "")
	assertEqual(t, "OS", got.OS, "")
	assertEqual(t, "Region", got.Region, "")
	assertEqual(t, "Zone", got.Zone, "")

	if got.Labels != nil {
		t.Errorf("Labels should be nil, got %v", got.Labels)
	}
	if got.Annotations != nil {
		t.Errorf("Annotations should be nil, got %v", got.Annotations)
	}

	// Not ready
	if got.Ready {
		t.Error("Ready should be false")
	}
}

func TestNodeToModel_AWSProviderIDParsing(t *testing.T) {
	node := makeNode()
	// Remove region/zone labels to test providerID fallback
	delete(node.Labels, "topology.kubernetes.io/region")
	delete(node.Labels, "topology.kubernetes.io/zone")

	got := NodeToModel(node)

	// Should fall back to providerID parsing
	assertEqual(t, "InstanceID", got.InstanceID, "i-0abcdef1234567890")
	assertEqual(t, "Region", got.Region, "us-east-1")
	assertEqual(t, "Zone", got.Zone, "us-east-1a")
}

// --- test helpers ---

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertFloat64(t *testing.T, field string, got, want float64) {
	t.Helper()
	// Allow small epsilon for float comparison
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.001 {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

func assertInt64(t *testing.T, field string, got, want int64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}

func TestDetectMIGResources_NoMIG(t *testing.T) {
	node := makeNode()
	migEnabled, migDevices := DetectMIGResources(node)
	if migEnabled {
		t.Error("expected migEnabled=false for node without MIG resources")
	}
	if migDevices != nil {
		t.Errorf("expected nil migDevices, got %v", migDevices)
	}
}

func TestDetectMIGResources_WithMIG(t *testing.T) {
	node := makeNode()
	node.Status.Allocatable["nvidia.com/mig-1g.5gb"] = resource.MustParse("3")
	node.Status.Allocatable["nvidia.com/mig-2g.10gb"] = resource.MustParse("2")
	node.Status.Allocatable["nvidia.com/gpu"] = resource.MustParse("0")

	migEnabled, migDevices := DetectMIGResources(node)
	if !migEnabled {
		t.Error("expected migEnabled=true for node with MIG resources")
	}
	if len(migDevices) != 2 {
		t.Fatalf("expected 2 MIG device types, got %d", len(migDevices))
	}
	if migDevices["nvidia.com/mig-1g.5gb"] != 3 {
		t.Errorf("mig-1g.5gb = %d, want 3", migDevices["nvidia.com/mig-1g.5gb"])
	}
	if migDevices["nvidia.com/mig-2g.10gb"] != 2 {
		t.Errorf("mig-2g.10gb = %d, want 2", migDevices["nvidia.com/mig-2g.10gb"])
	}
}

func TestDetectMIGResources_GPUWithoutMIG(t *testing.T) {
	node := makeNode()
	node.Status.Allocatable["nvidia.com/gpu"] = resource.MustParse("4")

	migEnabled, migDevices := DetectMIGResources(node)
	if migEnabled {
		t.Error("expected migEnabled=false for node with nvidia.com/gpu but no MIG resources")
	}
	if migDevices != nil {
		t.Errorf("expected nil migDevices, got %v", migDevices)
	}
}

func assertInt(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}
