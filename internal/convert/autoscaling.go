package convert

import (
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// HPAToModel converts a Kubernetes HorizontalPodAutoscaler (v2) to model.HPAInfo.
// Pure function — no side effects.
func HPAToModel(hpa *autoscalingv2.HorizontalPodAutoscaler) model.HPAInfo {
	info := model.HPAInfo{
		Name:      hpa.Name,
		UID:       string(hpa.UID),
		Namespace: hpa.Namespace,

		TargetKind:       hpa.Spec.ScaleTargetRef.Kind,
		TargetName:       hpa.Spec.ScaleTargetRef.Name,
		TargetAPIVersion: hpa.Spec.ScaleTargetRef.APIVersion,

		MinReplicas:     hpa.Spec.MinReplicas,
		MaxReplicas:     hpa.Spec.MaxReplicas,
		CurrentReplicas: hpa.Status.CurrentReplicas,
		DesiredReplicas: hpa.Status.DesiredReplicas,

		Labels:            hpa.Labels,
		Annotations:       FilterAnnotations(hpa.Annotations),
		CreationTimestamp: hpa.CreationTimestamp.UnixMilli(),
	}

	// Metrics
	info.Metrics = convertHPAMetrics(hpa.Spec.Metrics)

	// Current metrics
	info.CurrentMetrics = convertHPACurrentMetrics(hpa.Status.CurrentMetrics)

	// Behavior
	if hpa.Spec.Behavior != nil {
		if hpa.Spec.Behavior.ScaleUp != nil {
			info.ScaleUpBehavior = convertHPAScalingRules(hpa.Spec.Behavior.ScaleUp)
		}
		if hpa.Spec.Behavior.ScaleDown != nil {
			info.ScaleDownBehavior = convertHPAScalingRules(hpa.Spec.Behavior.ScaleDown)
		}
	}

	// Conditions
	info.Conditions = convertHPAConditions(hpa.Status.Conditions)

	// LastScaleTime
	if hpa.Status.LastScaleTime != nil {
		t := hpa.Status.LastScaleTime.UnixMilli()
		info.LastScaleTime = &t
	}

	return info
}

func convertHPAMetrics(metrics []autoscalingv2.MetricSpec) []model.HPAMetricInfo {
	if len(metrics) == 0 {
		return nil
	}
	out := make([]model.HPAMetricInfo, len(metrics))
	for i, m := range metrics {
		mi := model.HPAMetricInfo{
			Type: string(m.Type),
		}
		switch m.Type {
		case autoscalingv2.ResourceMetricSourceType:
			if m.Resource != nil {
				mi.ResourceName = string(m.Resource.Name)
				mi.TargetType = string(m.Resource.Target.Type)
				mi.TargetValue = metricTargetValue(m.Resource.Target)
			}
		case autoscalingv2.ContainerResourceMetricSourceType:
			if m.ContainerResource != nil {
				mi.ContainerName = m.ContainerResource.Container
				mi.ResourceName = string(m.ContainerResource.Name)
				mi.TargetType = string(m.ContainerResource.Target.Type)
				mi.TargetValue = metricTargetValue(m.ContainerResource.Target)
			}
		case autoscalingv2.PodsMetricSourceType:
			if m.Pods != nil {
				mi.MetricName = m.Pods.Metric.Name
				mi.TargetType = string(m.Pods.Target.Type)
				mi.TargetValue = metricTargetValue(m.Pods.Target)
			}
		case autoscalingv2.ObjectMetricSourceType:
			if m.Object != nil {
				mi.MetricName = m.Object.Metric.Name
				mi.TargetType = string(m.Object.Target.Type)
				mi.TargetValue = metricTargetValue(m.Object.Target)
			}
		case autoscalingv2.ExternalMetricSourceType:
			if m.External != nil {
				mi.MetricName = m.External.Metric.Name
				mi.TargetType = string(m.External.Target.Type)
				mi.TargetValue = metricTargetValue(m.External.Target)
			}
		}
		out[i] = mi
	}
	return out
}

func metricTargetValue(target autoscalingv2.MetricTarget) string {
	switch target.Type {
	case autoscalingv2.UtilizationMetricType:
		if target.AverageUtilization != nil {
			return fmt.Sprintf("%d", *target.AverageUtilization)
		}
	case autoscalingv2.AverageValueMetricType:
		if target.AverageValue != nil {
			return target.AverageValue.String()
		}
	case autoscalingv2.ValueMetricType:
		if target.Value != nil {
			return target.Value.String()
		}
	}
	return ""
}

func convertHPACurrentMetrics(metrics []autoscalingv2.MetricStatus) []model.HPACurrentMetricInfo {
	if len(metrics) == 0 {
		return nil
	}
	out := make([]model.HPACurrentMetricInfo, len(metrics))
	for i, m := range metrics {
		cm := model.HPACurrentMetricInfo{
			Type: string(m.Type),
		}
		switch m.Type {
		case autoscalingv2.ResourceMetricSourceType:
			if m.Resource != nil {
				cm.ResourceName = string(m.Resource.Name)
				if m.Resource.Current.AverageValue != nil {
					cm.CurrentAverageValue = m.Resource.Current.AverageValue.String()
				}
				if m.Resource.Current.Value != nil {
					cm.CurrentValue = m.Resource.Current.Value.String()
				}
				cm.CurrentUtilization = m.Resource.Current.AverageUtilization
			}
		case autoscalingv2.PodsMetricSourceType:
			if m.Pods != nil {
				if m.Pods.Current.AverageValue != nil {
					cm.CurrentAverageValue = m.Pods.Current.AverageValue.String()
				}
			}
		case autoscalingv2.ObjectMetricSourceType:
			if m.Object != nil {
				if m.Object.Current.Value != nil {
					cm.CurrentValue = m.Object.Current.Value.String()
				}
			}
		case autoscalingv2.ExternalMetricSourceType:
			if m.External != nil {
				if m.External.Current.Value != nil {
					cm.CurrentValue = m.External.Current.Value.String()
				}
				if m.External.Current.AverageValue != nil {
					cm.CurrentAverageValue = m.External.Current.AverageValue.String()
				}
			}
		}
		out[i] = cm
	}
	return out
}

func convertHPAScalingRules(rules *autoscalingv2.HPAScalingRules) *model.HPAScalingBehavior {
	b := &model.HPAScalingBehavior{
		StabilizationWindowSeconds: rules.StabilizationWindowSeconds,
	}
	if rules.SelectPolicy != nil {
		b.SelectPolicy = string(*rules.SelectPolicy)
	}
	if len(rules.Policies) > 0 {
		b.Policies = make([]model.HPAScalingPolicy, len(rules.Policies))
		for i, p := range rules.Policies {
			b.Policies[i] = model.HPAScalingPolicy{
				Type:          string(p.Type),
				Value:         p.Value,
				PeriodSeconds: p.PeriodSeconds,
			}
		}
	}
	return b
}

func convertHPAConditions(conditions []autoscalingv2.HorizontalPodAutoscalerCondition) []model.HPAConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.HPAConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.HPAConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}

// VPAToModel converts an unstructured VPA object to model.VPAInfo.
// VPA uses dynamic client, so input is unstructured.
func VPAToModel(obj *unstructured.Unstructured) model.VPAInfo {
	info := model.VPAInfo{
		Name:              obj.GetName(),
		Namespace:         obj.GetNamespace(),
		Labels:            obj.GetLabels(),
		CreationTimestamp: obj.GetCreationTimestamp().UnixMilli(),
	}

	// spec.targetRef
	if spec, ok := nestedMap(obj.Object, "spec"); ok {
		if targetRef, ok := nestedMap(spec, "targetRef"); ok {
			info.TargetKind, _ = targetRef["kind"].(string)
			info.TargetName, _ = targetRef["name"].(string)
			info.TargetAPIVersion, _ = targetRef["apiVersion"].(string)
		}
		// spec.updatePolicy.updateMode
		if updatePolicy, ok := nestedMap(spec, "updatePolicy"); ok {
			info.UpdateMode, _ = updatePolicy["updateMode"].(string)
		}
	}

	// status.recommendation.containerRecommendations
	if status, ok := nestedMap(obj.Object, "status"); ok {
		if recommendation, ok := nestedMap(status, "recommendation"); ok {
			if containers, ok := recommendation["containerRecommendations"].([]interface{}); ok {
				info.ContainerRecommendations = parseVPAContainerRecommendations(containers)
			}
		}
		// status.conditions
		if conditions, ok := status["conditions"].([]interface{}); ok {
			info.Conditions = parseVPAConditions(conditions)
		}
	}

	return info
}

func nestedMap(obj map[string]interface{}, key string) (map[string]interface{}, bool) {
	v, ok := obj[key]
	if !ok {
		return nil, false
	}
	m, ok := v.(map[string]interface{})
	return m, ok
}

func parseVPAContainerRecommendations(containers []interface{}) []model.VPAContainerRecommendation {
	if len(containers) == 0 {
		return nil
	}
	out := make([]model.VPAContainerRecommendation, 0, len(containers))
	for _, c := range containers {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		rec := model.VPAContainerRecommendation{
			ContainerName: stringVal(cm, "containerName"),
		}
		rec.LowerBound = parseResourceValues(cm, "lowerBound")
		rec.Target = parseResourceValues(cm, "target")
		rec.UncappedTarget = parseResourceValues(cm, "uncappedTarget")
		rec.UpperBound = parseResourceValues(cm, "upperBound")
		out = append(out, rec)
	}
	return out
}

func parseResourceValues(m map[string]interface{}, key string) model.ResourceValues {
	bound, ok := m[key].(map[string]interface{})
	if !ok {
		return model.ResourceValues{}
	}
	rv := model.ResourceValues{}
	if cpuStr, ok := bound["cpu"].(string); ok {
		rv.CPUCores = ParseQuantityString(cpuStr)
	}
	if memStr, ok := bound["memory"].(string); ok {
		rv.MemoryBytes = int64(ParseQuantityString(memStr))
	}
	return rv
}

func parseVPAConditions(conditions []interface{}) []model.VPAConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.VPAConditionInfo, 0, len(conditions))
	for _, c := range conditions {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, model.VPAConditionInfo{
			Type:    stringVal(cm, "type"),
			Status:  stringVal(cm, "status"),
			Reason:  stringVal(cm, "reason"),
			Message: stringVal(cm, "message"),
		})
	}
	return out
}

func stringVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// PDBToModel converts a Kubernetes PodDisruptionBudget to model.PDBInfo.
// Pure function — no side effects.
func PDBToModel(pdb *policyv1.PodDisruptionBudget) model.PDBInfo {
	info := model.PDBInfo{
		Name:      pdb.Name,
		UID:       string(pdb.UID),
		Namespace: pdb.Namespace,

		CurrentHealthy:     pdb.Status.CurrentHealthy,
		DesiredHealthy:     pdb.Status.DesiredHealthy,
		DisruptionsAllowed: pdb.Status.DisruptionsAllowed,
		ExpectedPods:       pdb.Status.ExpectedPods,

		Labels:            pdb.Labels,
		Annotations:       FilterAnnotations(pdb.Annotations),
		CreationTimestamp: pdb.CreationTimestamp.UnixMilli(),
	}

	// Selector
	if pdb.Spec.Selector != nil {
		info.MatchLabels = pdb.Spec.Selector.MatchLabels
		if len(pdb.Spec.Selector.MatchExpressions) > 0 {
			info.MatchExpressions = make([]model.LabelSelectorRequirement, len(pdb.Spec.Selector.MatchExpressions))
			for i, expr := range pdb.Spec.Selector.MatchExpressions {
				info.MatchExpressions[i] = model.LabelSelectorRequirement{
					Key:      expr.Key,
					Operator: string(expr.Operator),
					Values:   expr.Values,
				}
			}
		}
	}

	// MinAvailable / MaxUnavailable
	if pdb.Spec.MinAvailable != nil {
		info.MinAvailable = pdb.Spec.MinAvailable.String()
	}
	if pdb.Spec.MaxUnavailable != nil {
		info.MaxUnavailable = pdb.Spec.MaxUnavailable.String()
	}

	// Conditions
	info.Conditions = convertPDBConditions(pdb.Status.Conditions)

	return info
}

func convertPDBConditions(conditions []metav1.Condition) []model.PDBConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.PDBConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.PDBConditionInfo{
			Type:    c.Type,
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}

// JobToModel converts a Kubernetes Job to model.JobInfo.
// Pure function — no side effects.
// TotalCPU/Memory fields are left at zero (populated by enrichment later).
func JobToModel(job *batchv1.Job) model.JobInfo {
	info := model.JobInfo{
		Name:      job.Name,
		UID:       string(job.UID),
		Namespace: job.Namespace,

		Completions:             job.Spec.Completions,
		Parallelism:             job.Spec.Parallelism,
		BackoffLimit:            job.Spec.BackoffLimit,
		ActiveDeadlineSeconds:   job.Spec.ActiveDeadlineSeconds,
		TTLSecondsAfterFinished: job.Spec.TTLSecondsAfterFinished,

		Active:    job.Status.Active,
		Succeeded: job.Status.Succeeded,
		Failed:    job.Status.Failed,

		Labels:            job.Labels,
		Annotations:       FilterAnnotations(job.Annotations),
		CreationTimestamp: job.CreationTimestamp.UnixMilli(),
	}

	// Owner CronJob
	for _, ref := range job.OwnerReferences {
		if ref.Kind == "CronJob" {
			info.OwnerCronJob = ref.Name
			break
		}
	}

	// StartTime
	if job.Status.StartTime != nil {
		t := job.Status.StartTime.UnixMilli()
		info.StartTime = &t
	}

	// CompletionTime
	if job.Status.CompletionTime != nil {
		t := job.Status.CompletionTime.UnixMilli()
		info.CompletionTime = &t
	}

	// DurationSeconds
	if info.StartTime != nil && info.CompletionTime != nil {
		d := float64(*info.CompletionTime-*info.StartTime) / 1000.0
		info.DurationSeconds = &d
	}

	// Conditions
	info.Conditions = convertJobConditions(job.Status.Conditions)

	return info
}

func convertJobConditions(conditions []batchv1.JobCondition) []model.JobConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.JobConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.JobConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}

// CronJobToModel converts a Kubernetes CronJob to model.CronJobInfo.
// Pure function — no side effects.
func CronJobToModel(cj *batchv1.CronJob) model.CronJobInfo {
	info := model.CronJobInfo{
		Name:              cj.Name,
		UID:               string(cj.UID),
		Namespace:         cj.Namespace,
		Schedule:          cj.Spec.Schedule,
		ConcurrencyPolicy: string(cj.Spec.ConcurrencyPolicy),

		ContainerSpecs: extractContainerSpecs(cj.Spec.JobTemplate.Spec.Template.Spec.Containers),

		Labels:            cj.Labels,
		Annotations:       FilterAnnotations(cj.Annotations),
		CreationTimestamp: cj.CreationTimestamp.UnixMilli(),
	}

	// Suspend
	if cj.Spec.Suspend != nil {
		info.Suspend = *cj.Spec.Suspend
	}

	// LastScheduleTime
	if cj.Status.LastScheduleTime != nil {
		t := cj.Status.LastScheduleTime.UnixMilli()
		info.LastScheduleTime = &t
	}

	// LastSuccessfulTime
	if cj.Status.LastSuccessfulTime != nil {
		t := cj.Status.LastSuccessfulTime.UnixMilli()
		info.LastSuccessfulTime = &t
	}

	// Active jobs
	if len(cj.Status.Active) > 0 {
		info.ActiveJobs = make([]string, len(cj.Status.Active))
		for i, ref := range cj.Status.Active {
			info.ActiveJobs[i] = ref.Name
		}
	}

	return info
}
