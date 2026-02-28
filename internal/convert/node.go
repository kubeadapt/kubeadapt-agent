package convert

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// NodeToModel converts a Kubernetes Node object to a model.NodeInfo.
// Pure function — no side effects, no time.Now(), no external calls.
// CPU/Memory usage fields are left nil (merged later from metrics collector).
func NodeToModel(node *corev1.Node) model.NodeInfo {
	labels := node.Labels
	providerID := node.Spec.ProviderID

	instanceID, az, region := ParseProviderID(providerID)

	// Prefer topology labels over providerID for region/zone
	if v, ok := labels["topology.kubernetes.io/region"]; ok {
		region = v
	}
	if v, ok := labels["topology.kubernetes.io/zone"]; ok {
		az = v
	}

	info := model.NodeInfo{
		Name:             node.Name,
		UID:              string(node.UID),
		ProviderID:       providerID,
		InstanceID:       instanceID,
		Region:           region,
		Zone:             az,
		InstanceType:     labels["node.kubernetes.io/instance-type"],
		CapacityType:     ExtractCapacityType(labels),
		NodeGroup:        ExtractNodeGroup(labels),
		Architecture:     labels["kubernetes.io/arch"],
		OS:               labels["kubernetes.io/os"],
		KubeletVersion:   node.Status.NodeInfo.KubeletVersion,
		ContainerRuntime: node.Status.NodeInfo.ContainerRuntimeVersion,

		PodCIDR:          node.Spec.PodCIDR,
		PodCIDRs:         node.Spec.PodCIDRs,
		OSImage:          node.Status.NodeInfo.OSImage,
		KernelVersion:    node.Status.NodeInfo.KernelVersion,
		KubeProxyVersion: node.Status.NodeInfo.KubeProxyVersion,

		// Capacity
		CPUCapacityCores:            ParseQuantity(node.Status.Capacity[corev1.ResourceCPU]),
		MemoryCapacityBytes:         quantityValue(node.Status.Capacity, corev1.ResourceMemory),
		EphemeralStorageBytes:       quantityValue(node.Status.Capacity, corev1.ResourceEphemeralStorage),
		EphemeralStorageAllocatable: quantityValue(node.Status.Allocatable, corev1.ResourceEphemeralStorage),
		PodCapacity:                 int(quantityValue(node.Status.Capacity, corev1.ResourcePods)),
		GPUCapacity:                 int(quantityValue(node.Status.Capacity, "nvidia.com/gpu")),

		// Allocatable
		CPUAllocatable:    ParseQuantity(node.Status.Allocatable[corev1.ResourceCPU]),
		MemoryAllocatable: quantityValue(node.Status.Allocatable, corev1.ResourceMemory),
		PodAllocatable:    int(quantityValue(node.Status.Allocatable, corev1.ResourcePods)),
		GPUAllocatable:    int(quantityValue(node.Status.Allocatable, "nvidia.com/gpu")),

		// Usage left nil — merged later from metrics collector

		Ready:         nodeReady(node.Status.Conditions),
		Unschedulable: node.Spec.Unschedulable,
		Taints:        convertTaints(node.Spec.Taints),
		Conditions:    convertConditions(node.Status.Conditions),

		Labels:            labels,
		Annotations:       FilterAnnotations(node.Annotations),
		CreationTimestamp: node.CreationTimestamp.UnixMilli(),
	}

	return info
}

// nodeReady returns true if the node has a Ready condition with status True.
func nodeReady(conditions []corev1.NodeCondition) bool {
	for _, c := range conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// convertTaints converts K8s Taints to model TaintInfo slice.
func convertTaints(taints []corev1.Taint) []model.TaintInfo {
	if len(taints) == 0 {
		return nil
	}
	out := make([]model.TaintInfo, len(taints))
	for i, t := range taints {
		out[i] = model.TaintInfo{
			Key:    t.Key,
			Value:  t.Value,
			Effect: string(t.Effect),
		}
	}
	return out
}

// quantityValue extracts the int64 Value() from a resource in a ResourceList.
// Handles the pointer-receiver issue with resource.Quantity.Value().
func quantityValue(rl corev1.ResourceList, name corev1.ResourceName) int64 {
	q, ok := rl[name]
	if !ok {
		return 0
	}
	return q.Value()
}

// DetectMIGResources checks a node's Allocatable resources for nvidia.com/mig-* entries.
// Returns true with a map of MIG resource names to counts if any are found.
func DetectMIGResources(node *corev1.Node) (migEnabled bool, migDevices map[string]int) {
	for rName, q := range node.Status.Allocatable {
		if strings.HasPrefix(string(rName), "nvidia.com/mig-") {
			if migDevices == nil {
				migDevices = make(map[string]int)
			}
			migDevices[string(rName)] = int(q.Value())
			migEnabled = true
		}
	}
	return
}

// convertConditions converts K8s NodeConditions to model NodeConditionInfo slice.
func convertConditions(conditions []corev1.NodeCondition) []model.NodeConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.NodeConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.NodeConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}
