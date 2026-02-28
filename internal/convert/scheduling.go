package convert

import (
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// PriorityClassToModel converts a Kubernetes PriorityClass to model.PriorityClassInfo.
// Pure function — no side effects.
func PriorityClassToModel(pc *schedulingv1.PriorityClass) model.PriorityClassInfo {
	info := model.PriorityClassInfo{
		Name:          pc.Name,
		Value:         pc.Value,
		GlobalDefault: pc.GlobalDefault,
		Description:   pc.Description,
	}

	if pc.PreemptionPolicy != nil {
		info.PreemptionPolicy = string(*pc.PreemptionPolicy)
	}

	return info
}

// LimitRangeToModel converts a Kubernetes LimitRange to model.LimitRangeInfo.
// Pure function — no side effects.
func LimitRangeToModel(lr *corev1.LimitRange) model.LimitRangeInfo {
	info := model.LimitRangeInfo{
		Name:      lr.Name,
		Namespace: lr.Namespace,
	}

	if len(lr.Spec.Limits) > 0 {
		info.Limits = make([]model.LimitRangeItemInfo, len(lr.Spec.Limits))
		for i, l := range lr.Spec.Limits {
			info.Limits[i] = model.LimitRangeItemInfo{
				Type:                 string(l.Type),
				Default:              resourceListToStringMap(l.Default),
				DefaultRequest:       resourceListToStringMap(l.DefaultRequest),
				Max:                  resourceListToStringMap(l.Max),
				Min:                  resourceListToStringMap(l.Min),
				MaxLimitRequestRatio: resourceListToStringMap(l.MaxLimitRequestRatio),
			}
		}
	}

	return info
}

func resourceListToStringMap(rl corev1.ResourceList) map[string]string {
	if len(rl) == 0 {
		return nil
	}
	m := make(map[string]string, len(rl))
	for k, v := range rl {
		m[string(k)] = v.String()
	}
	return m
}

// ResourceQuotaToModel converts a Kubernetes ResourceQuota to model.ResourceQuotaInfo.
// Pure function — no side effects.
func ResourceQuotaToModel(rq *corev1.ResourceQuota) model.ResourceQuotaInfo {
	info := model.ResourceQuotaInfo{
		Name:      rq.Name,
		Namespace: rq.Namespace,
		Labels:    rq.Labels,
	}

	info.Hard = resourceListToStringMap(rq.Spec.Hard)
	info.Used = resourceListToStringMap(rq.Status.Used)

	return info
}

// NamespaceToModel converts a Kubernetes Namespace to model.NamespaceInfo.
// Pure function — no side effects.
func NamespaceToModel(ns *corev1.Namespace) model.NamespaceInfo {
	return model.NamespaceInfo{
		Name:              ns.Name,
		Phase:             string(ns.Status.Phase),
		Labels:            ns.Labels,
		Annotations:       FilterAnnotations(ns.Annotations),
		CreationTimestamp: ns.CreationTimestamp.UnixMilli(),
	}
}
