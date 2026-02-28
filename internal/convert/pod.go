package convert

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// PodToModel converts a Kubernetes Pod object to a model.PodInfo.
// Pure function — no side effects, no time.Now(), no external calls.
// CPUUsageCores/MemoryUsageBytes on containers are left nil (merged later from metrics).
func PodToModel(pod *corev1.Pod) model.PodInfo {
	info := model.PodInfo{
		Name:      pod.Name,
		UID:       string(pod.UID),
		Namespace: pod.Namespace,
		NodeName:  pod.Spec.NodeName,
		Phase:     string(pod.Status.Phase),
		Reason:    pod.Status.Reason,
		QoSClass:  string(pod.Status.QOSClass),

		Labels:            pod.Labels,
		Annotations:       FilterAnnotations(pod.Annotations),
		CreationTimestamp: pod.CreationTimestamp.UnixMilli(),

		PriorityClassName:  pod.Spec.PriorityClassName,
		Priority:           pod.Spec.Priority,
		SchedulerName:      pod.Spec.SchedulerName,
		ServiceAccountName: pod.Spec.ServiceAccountName,

		PodIP:       pod.Status.PodIP,
		HostIP:      pod.Status.HostIP,
		HostNetwork: pod.Spec.HostNetwork,
		HasHostPath: hasHostPathVolume(pod.Spec.Volumes),
		HasEmptyDir: hasEmptyDirVolume(pod.Spec.Volumes),
	}

	// Owner — immediate ownerReferences[0] only
	if len(pod.OwnerReferences) > 0 {
		owner := pod.OwnerReferences[0]
		info.OwnerKind = owner.Kind
		info.OwnerName = owner.Name
		info.OwnerUID = string(owner.UID)
	}

	// Build status lookup maps by container name
	statusMap := buildStatusMap(pod.Status.ContainerStatuses)
	initStatusMap := buildStatusMap(pod.Status.InitContainerStatuses)

	// Convert containers (spec is the source of truth, status is matched by name)
	info.Containers = convertContainers(pod.Spec.Containers, statusMap)
	info.InitContainers = convertContainers(pod.Spec.InitContainers, initStatusMap)

	// Conditions
	info.Conditions = convertPodConditions(pod.Status.Conditions)

	return info
}

// buildStatusMap creates a name → ContainerStatus lookup from a status slice.
func buildStatusMap(statuses []corev1.ContainerStatus) map[string]corev1.ContainerStatus {
	m := make(map[string]corev1.ContainerStatus, len(statuses))
	for _, s := range statuses {
		m[s.Name] = s
	}
	return m
}

// convertContainers converts spec containers, matching each with its status by name.
func convertContainers(specs []corev1.Container, statusMap map[string]corev1.ContainerStatus) []model.ContainerInfo {
	if len(specs) == 0 {
		return nil
	}
	out := make([]model.ContainerInfo, len(specs))
	for i, spec := range specs {
		status, hasStatus := statusMap[spec.Name]
		out[i] = containerToModel(status, spec, hasStatus)
	}
	return out
}

// containerToModel converts a single container spec + status pair to model.ContainerInfo.
// If hasStatus is false, status fields default to zero values (pod may be freshly created).
func containerToModel(status corev1.ContainerStatus, spec corev1.Container, hasStatus bool) model.ContainerInfo {
	c := model.ContainerInfo{
		Name:  spec.Name,
		Image: spec.Image,

		// Resources from spec
		CPURequestCores:         ParseQuantity(resourceQuantity(spec.Resources.Requests, corev1.ResourceCPU)),
		MemoryRequestBytes:      quantityValue(spec.Resources.Requests, corev1.ResourceMemory),
		CPULimitCores:           ParseQuantity(resourceQuantity(spec.Resources.Limits, corev1.ResourceCPU)),
		MemoryLimitBytes:        quantityValue(spec.Resources.Limits, corev1.ResourceMemory),
		EphemeralStorageRequest: quantityValue(spec.Resources.Requests, corev1.ResourceEphemeralStorage),
		EphemeralStorageLimit:   quantityValue(spec.Resources.Limits, corev1.ResourceEphemeralStorage),
		GPURequest:              int(quantityValue(spec.Resources.Requests, "nvidia.com/gpu")),
		GPULimit:                int(quantityValue(spec.Resources.Limits, "nvidia.com/gpu")),

		// Ports from spec
		Ports: convertContainerPorts(spec.Ports),
	}

	if hasStatus {
		c.ImageID = status.ImageID
		c.Ready = status.Ready
		c.Started = status.Started
		c.RestartCount = status.RestartCount

		// Determine state from status.State
		switch {
		case status.State.Running != nil:
			c.State = "running"
		case status.State.Waiting != nil:
			c.State = "waiting"
			c.StateReason = status.State.Waiting.Reason
			c.StateMessage = status.State.Waiting.Message
		case status.State.Terminated != nil:
			c.State = "terminated"
			c.StateReason = status.State.Terminated.Reason
			c.StateMessage = status.State.Terminated.Message
			c.ExitCode = &status.State.Terminated.ExitCode
		}

		// Last termination reason
		if status.LastTerminationState.Terminated != nil {
			c.LastTerminationReason = status.LastTerminationState.Terminated.Reason
		}
	} else {
		// No status yet — container is effectively waiting
		c.State = "waiting"
	}

	return c
}

// resourceQuantity extracts a resource.Quantity from a ResourceList.
// Returns a zero Quantity if not found.
func resourceQuantity(rl corev1.ResourceList, name corev1.ResourceName) resource.Quantity {
	if rl == nil {
		return resource.Quantity{}
	}
	q, ok := rl[name]
	if !ok {
		return resource.Quantity{}
	}
	return q
}

// convertContainerPorts converts K8s ContainerPorts to model ContainerPortInfo slice.
func convertContainerPorts(ports []corev1.ContainerPort) []model.ContainerPortInfo {
	if len(ports) == 0 {
		return nil
	}
	out := make([]model.ContainerPortInfo, len(ports))
	for i, p := range ports {
		out[i] = model.ContainerPortInfo{
			Name:          p.Name,
			ContainerPort: p.ContainerPort,
			Protocol:      string(p.Protocol),
		}
	}
	return out
}

// hasHostPathVolume returns true if any volume uses HostPath.
func hasHostPathVolume(volumes []corev1.Volume) bool {
	for _, v := range volumes {
		if v.HostPath != nil {
			return true
		}
	}
	return false
}

// hasEmptyDirVolume returns true if any volume uses EmptyDir.
func hasEmptyDirVolume(volumes []corev1.Volume) bool {
	for _, v := range volumes {
		if v.EmptyDir != nil {
			return true
		}
	}
	return false
}

// convertPodConditions converts K8s PodConditions to model PodConditionInfo slice.
func convertPodConditions(conditions []corev1.PodCondition) []model.PodConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.PodConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.PodConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}
