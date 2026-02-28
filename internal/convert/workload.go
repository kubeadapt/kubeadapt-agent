package convert

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// DeploymentToModel converts a Kubernetes Deployment to model.DeploymentInfo.
// Pure function — no side effects.
// TotalCPU/Memory fields are left at zero (populated by enrichment aggregation later).
func DeploymentToModel(dep *appsv1.Deployment) model.DeploymentInfo {
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}

	info := model.DeploymentInfo{
		Name:                dep.Name,
		UID:                 string(dep.UID),
		Namespace:           dep.Namespace,
		Replicas:            replicas,
		ReadyReplicas:       dep.Status.ReadyReplicas,
		AvailableReplicas:   dep.Status.AvailableReplicas,
		UnavailableReplicas: dep.Status.UnavailableReplicas,
		UpdatedReplicas:     dep.Status.UpdatedReplicas,
		Strategy:            string(dep.Spec.Strategy.Type),

		ContainerSpecs: extractContainerSpecs(dep.Spec.Template.Spec.Containers),

		Labels:            dep.Labels,
		Annotations:       FilterAnnotations(dep.Annotations),
		CreationTimestamp: dep.CreationTimestamp.UnixMilli(),

		Paused: dep.Spec.Paused,
	}

	// Selector
	if dep.Spec.Selector != nil {
		info.Selector = dep.Spec.Selector.MatchLabels
	}

	// RollingUpdate strategy params
	if dep.Spec.Strategy.RollingUpdate != nil {
		ru := dep.Spec.Strategy.RollingUpdate
		if ru.MaxSurge != nil {
			info.MaxSurge = ru.MaxSurge.String()
		}
		if ru.MaxUnavailable != nil {
			info.MaxUnavailable = ru.MaxUnavailable.String()
		}
	}

	// Conditions
	info.Conditions = convertDeploymentConditions(dep.Status.Conditions)

	return info
}

// StatefulSetToModel converts a Kubernetes StatefulSet to model.StatefulSetInfo.
// Pure function — no side effects.
// TotalCPU/Memory fields are left at zero (populated by enrichment aggregation later).
func StatefulSetToModel(ss *appsv1.StatefulSet) model.StatefulSetInfo {
	replicas := int32(1)
	if ss.Spec.Replicas != nil {
		replicas = *ss.Spec.Replicas
	}

	info := model.StatefulSetInfo{
		Name:                ss.Name,
		UID:                 string(ss.UID),
		Namespace:           ss.Namespace,
		Replicas:            replicas,
		ReadyReplicas:       ss.Status.ReadyReplicas,
		AvailableReplicas:   ss.Status.AvailableReplicas,
		UpdatedReplicas:     ss.Status.UpdatedReplicas,
		Strategy:            string(ss.Spec.UpdateStrategy.Type),
		ServiceName:         ss.Spec.ServiceName,
		PodManagementPolicy: string(ss.Spec.PodManagementPolicy),

		ContainerSpecs: extractContainerSpecs(ss.Spec.Template.Spec.Containers),

		Labels:            ss.Labels,
		Annotations:       FilterAnnotations(ss.Annotations),
		CreationTimestamp: ss.CreationTimestamp.UnixMilli(),
	}

	// Selector
	if ss.Spec.Selector != nil {
		info.Selector = ss.Spec.Selector.MatchLabels
	}

	// VolumeClaimTemplates — extract names
	if len(ss.Spec.VolumeClaimTemplates) > 0 {
		vcts := make([]string, len(ss.Spec.VolumeClaimTemplates))
		for i, pvc := range ss.Spec.VolumeClaimTemplates {
			vcts[i] = pvc.Name
		}
		info.VolumeClaimTemplates = vcts
	}

	// Conditions
	info.Conditions = convertStatefulSetConditions(ss.Status.Conditions)

	return info
}

// DaemonSetToModel converts a Kubernetes DaemonSet to model.DaemonSetInfo.
// Pure function — no side effects.
// TotalCPU/Memory fields are left at zero (populated by enrichment aggregation later).
func DaemonSetToModel(ds *appsv1.DaemonSet) model.DaemonSetInfo {
	info := model.DaemonSetInfo{
		Name:                   ds.Name,
		UID:                    string(ds.UID),
		Namespace:              ds.Namespace,
		DesiredNumberScheduled: ds.Status.DesiredNumberScheduled,
		CurrentNumberScheduled: ds.Status.CurrentNumberScheduled,
		NumberReady:            ds.Status.NumberReady,
		NumberMisscheduled:     ds.Status.NumberMisscheduled,
		UpdatedNumberScheduled: ds.Status.UpdatedNumberScheduled,
		Strategy:               string(ds.Spec.UpdateStrategy.Type),

		ContainerSpecs: extractContainerSpecs(ds.Spec.Template.Spec.Containers),

		Labels:            ds.Labels,
		Annotations:       FilterAnnotations(ds.Annotations),
		CreationTimestamp: ds.CreationTimestamp.UnixMilli(),
	}

	// Selector
	if ds.Spec.Selector != nil {
		info.Selector = ds.Spec.Selector.MatchLabels
	}

	// Conditions
	info.Conditions = convertDaemonSetConditions(ds.Status.Conditions)

	return info
}

// ReplicaSetToModel converts a Kubernetes ReplicaSet to model.ReplicaSetInfo.
// Pure function — no side effects.
func ReplicaSetToModel(rs *appsv1.ReplicaSet) model.ReplicaSetInfo {
	replicas := int32(1)
	if rs.Spec.Replicas != nil {
		replicas = *rs.Spec.Replicas
	}

	info := model.ReplicaSetInfo{
		Name:              rs.Name,
		Namespace:         rs.Namespace,
		Replicas:          replicas,
		ReadyReplicas:     rs.Status.ReadyReplicas,
		Labels:            rs.Labels,
		CreationTimestamp: rs.CreationTimestamp.UnixMilli(),
	}

	// Selector
	if rs.Spec.Selector != nil {
		info.Selector = rs.Spec.Selector.MatchLabels
	}

	// Owner — immediate ownerReferences[0] only
	if len(rs.OwnerReferences) > 0 {
		owner := rs.OwnerReferences[0]
		info.OwnerKind = owner.Kind
		info.OwnerName = owner.Name
		info.OwnerUID = string(owner.UID)
	}

	return info
}

// extractContainerSpecs converts a slice of K8s Containers to model.ContainerSpecInfo slice.
// Extracts resource requests/limits for each container.
func extractContainerSpecs(containers []corev1.Container) []model.ContainerSpecInfo {
	if len(containers) == 0 {
		return nil
	}
	out := make([]model.ContainerSpecInfo, len(containers))
	for i, c := range containers {
		out[i] = model.ContainerSpecInfo{
			Name:               c.Name,
			Image:              c.Image,
			CPURequestCores:    ParseQuantity(resourceQuantity(c.Resources.Requests, corev1.ResourceCPU)),
			MemoryRequestBytes: quantityValue(c.Resources.Requests, corev1.ResourceMemory),
			CPULimitCores:      ParseQuantity(resourceQuantity(c.Resources.Limits, corev1.ResourceCPU)),
			MemoryLimitBytes:   quantityValue(c.Resources.Limits, corev1.ResourceMemory),
			GPURequest:         int(quantityValue(c.Resources.Requests, "nvidia.com/gpu")),
			GPULimit:           int(quantityValue(c.Resources.Limits, "nvidia.com/gpu")),
		}
	}
	return out
}

// convertDeploymentConditions converts appsv1.DeploymentCondition to model.WorkloadConditionInfo.
func convertDeploymentConditions(conditions []appsv1.DeploymentCondition) []model.WorkloadConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.WorkloadConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.WorkloadConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}

// convertStatefulSetConditions converts appsv1.StatefulSetCondition to model.WorkloadConditionInfo.
func convertStatefulSetConditions(conditions []appsv1.StatefulSetCondition) []model.WorkloadConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.WorkloadConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.WorkloadConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}

// convertDaemonSetConditions converts appsv1.DaemonSetCondition to model.WorkloadConditionInfo.
func convertDaemonSetConditions(conditions []appsv1.DaemonSetCondition) []model.WorkloadConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.WorkloadConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.WorkloadConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}
