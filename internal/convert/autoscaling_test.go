package convert

import (
	"testing"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ---- HPA Tests ----

func TestHPAToModel_ResourceMetric(t *testing.T) {
	minReplicas := int32(2)
	targetUtil := int32(80)
	lastScale := metav1.NewTime(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web-hpa",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"app": "web"},
			Annotations: map[string]string{
				"autoscaling.alpha.kubernetes.io/behavior": "custom",
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "web",
				APIVersion: "apps/v1",
			},
			MinReplicas: &minReplicas,
			MaxReplicas: 10,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &targetUtil,
						},
					},
				},
			},
		},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 3,
			DesiredReplicas: 4,
			LastScaleTime:   &lastScale,
			CurrentMetrics: []autoscalingv2.MetricStatus{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricStatus{
						Name: corev1.ResourceCPU,
						Current: autoscalingv2.MetricValueStatus{
							AverageUtilization: int32Ptr(75),
							AverageValue:       quantityPtr("375m"),
						},
					},
				},
			},
			Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{
					Type:    autoscalingv2.ScalingActive,
					Status:  corev1.ConditionTrue,
					Reason:  "ValidMetricFound",
					Message: "the HPA was able to successfully calculate a replica count",
				},
			},
		},
	}

	info := HPAToModel(hpa)

	assertEqual(t, "Name", info.Name, "web-hpa")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "TargetKind", info.TargetKind, "Deployment")
	assertEqual(t, "TargetName", info.TargetName, "web")
	assertEqual(t, "TargetAPIVersion", info.TargetAPIVersion, "apps/v1")

	if info.MinReplicas == nil || *info.MinReplicas != 2 {
		t.Errorf("MinReplicas: want 2, got %v", info.MinReplicas)
	}
	if info.MaxReplicas != 10 {
		t.Errorf("MaxReplicas: want 10, got %d", info.MaxReplicas)
	}
	if info.CurrentReplicas != 3 {
		t.Errorf("CurrentReplicas: want 3, got %d", info.CurrentReplicas)
	}
	if info.DesiredReplicas != 4 {
		t.Errorf("DesiredReplicas: want 4, got %d", info.DesiredReplicas)
	}

	// Metrics
	if len(info.Metrics) != 1 {
		t.Fatalf("Metrics len: want 1, got %d", len(info.Metrics))
	}
	m := info.Metrics[0]
	assertEqual(t, "Metric.Type", m.Type, "Resource")
	assertEqual(t, "Metric.ResourceName", m.ResourceName, "cpu")
	assertEqual(t, "Metric.TargetType", m.TargetType, "Utilization")
	assertEqual(t, "Metric.TargetValue", m.TargetValue, "80")

	// Current metrics
	if len(info.CurrentMetrics) != 1 {
		t.Fatalf("CurrentMetrics len: want 1, got %d", len(info.CurrentMetrics))
	}
	cm := info.CurrentMetrics[0]
	assertEqual(t, "CurrentMetric.Type", cm.Type, "Resource")
	assertEqual(t, "CurrentMetric.ResourceName", cm.ResourceName, "cpu")
	if cm.CurrentUtilization == nil || *cm.CurrentUtilization != 75 {
		t.Errorf("CurrentUtilization: want 75, got %v", cm.CurrentUtilization)
	}

	// LastScaleTime
	if info.LastScaleTime == nil {
		t.Fatal("LastScaleTime should not be nil")
	}
	expectedLST := lastScale.UnixMilli()
	if *info.LastScaleTime != expectedLST {
		t.Errorf("LastScaleTime: want %d, got %d", expectedLST, *info.LastScaleTime)
	}

	// Conditions
	if len(info.Conditions) != 1 {
		t.Fatalf("Conditions len: want 1, got %d", len(info.Conditions))
	}
	assertEqual(t, "Condition.Type", info.Conditions[0].Type, "ScalingActive")
	assertEqual(t, "Condition.Status", info.Conditions[0].Status, "True")

	// Labels
	assertEqual(t, "Labels[app]", info.Labels["app"], "web")
}

func TestHPAToModel_ScalingBehavior(t *testing.T) {
	stabilization := int32(300)
	selectMax := autoscalingv2.MaxChangePolicySelect

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "behavior-hpa",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment", Name: "app",
			},
			MaxReplicas: 20,
			Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &stabilization,
					SelectPolicy:               &selectMax,
					Policies: []autoscalingv2.HPAScalingPolicy{
						{
							Type:          autoscalingv2.PodsScalingPolicy,
							Value:         4,
							PeriodSeconds: 60,
						},
						{
							Type:          autoscalingv2.PercentScalingPolicy,
							Value:         100,
							PeriodSeconds: 60,
						},
					},
				},
				ScaleDown: &autoscalingv2.HPAScalingRules{
					Policies: []autoscalingv2.HPAScalingPolicy{
						{
							Type:          autoscalingv2.PodsScalingPolicy,
							Value:         1,
							PeriodSeconds: 300,
						},
					},
				},
			},
		},
	}

	info := HPAToModel(hpa)

	// Scale Up
	if info.ScaleUpBehavior == nil {
		t.Fatal("ScaleUpBehavior should not be nil")
	}
	if info.ScaleUpBehavior.StabilizationWindowSeconds == nil || *info.ScaleUpBehavior.StabilizationWindowSeconds != 300 {
		t.Errorf("ScaleUp.StabilizationWindowSeconds: want 300")
	}
	assertEqual(t, "ScaleUp.SelectPolicy", info.ScaleUpBehavior.SelectPolicy, "Max")
	if len(info.ScaleUpBehavior.Policies) != 2 {
		t.Fatalf("ScaleUp.Policies len: want 2, got %d", len(info.ScaleUpBehavior.Policies))
	}
	assertEqual(t, "ScaleUp.Policies[0].Type", info.ScaleUpBehavior.Policies[0].Type, "Pods")
	if info.ScaleUpBehavior.Policies[0].Value != 4 {
		t.Errorf("ScaleUp.Policies[0].Value: want 4, got %d", info.ScaleUpBehavior.Policies[0].Value)
	}

	// Scale Down
	if info.ScaleDownBehavior == nil {
		t.Fatal("ScaleDownBehavior should not be nil")
	}
	if len(info.ScaleDownBehavior.Policies) != 1 {
		t.Fatalf("ScaleDown.Policies len: want 1, got %d", len(info.ScaleDownBehavior.Policies))
	}
	if info.ScaleDownBehavior.Policies[0].PeriodSeconds != 300 {
		t.Errorf("ScaleDown.Policies[0].PeriodSeconds: want 300, got %d", info.ScaleDownBehavior.Policies[0].PeriodSeconds)
	}
}

// ---- VPA Tests ----

func TestVPAToModel_ContainerRecommendations(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling.k8s.io/v1",
			"kind":       "VerticalPodAutoscaler",
			"metadata": map[string]interface{}{
				"name":              "web-vpa",
				"namespace":         "production",
				"creationTimestamp": "2025-06-01T08:00:00Z",
				"labels":            map[string]interface{}{"app": "web"},
			},
			"spec": map[string]interface{}{
				"targetRef": map[string]interface{}{
					"kind":       "Deployment",
					"name":       "web",
					"apiVersion": "apps/v1",
				},
				"updatePolicy": map[string]interface{}{
					"updateMode": "Auto",
				},
			},
			"status": map[string]interface{}{
				"recommendation": map[string]interface{}{
					"containerRecommendations": []interface{}{
						map[string]interface{}{
							"containerName": "web",
							"lowerBound": map[string]interface{}{
								"cpu":    "100m",
								"memory": "128Mi",
							},
							"target": map[string]interface{}{
								"cpu":    "250m",
								"memory": "256Mi",
							},
							"uncappedTarget": map[string]interface{}{
								"cpu":    "300m",
								"memory": "300Mi",
							},
							"upperBound": map[string]interface{}{
								"cpu":    "1",
								"memory": "1Gi",
							},
						},
					},
				},
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "RecommendationProvided",
						"status":  "True",
						"reason":  "Ready",
						"message": "recommendations available",
					},
				},
			},
		},
	}

	info := VPAToModel(obj)

	assertEqual(t, "Name", info.Name, "web-vpa")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "TargetKind", info.TargetKind, "Deployment")
	assertEqual(t, "TargetName", info.TargetName, "web")
	assertEqual(t, "TargetAPIVersion", info.TargetAPIVersion, "apps/v1")
	assertEqual(t, "UpdateMode", info.UpdateMode, "Auto")

	if len(info.ContainerRecommendations) != 1 {
		t.Fatalf("ContainerRecommendations len: want 1, got %d", len(info.ContainerRecommendations))
	}
	rec := info.ContainerRecommendations[0]
	assertEqual(t, "ContainerName", rec.ContainerName, "web")

	// Target CPU: 250m = 0.25 cores
	if rec.Target.CPUCores < 0.24 || rec.Target.CPUCores > 0.26 {
		t.Errorf("Target.CPUCores: want ~0.25, got %f", rec.Target.CPUCores)
	}
	// Target Memory: 256Mi = 268435456 bytes
	if rec.Target.MemoryBytes != 268435456 {
		t.Errorf("Target.MemoryBytes: want 268435456, got %d", rec.Target.MemoryBytes)
	}
	// UpperBound CPU: 1 = 1.0 cores
	if rec.UpperBound.CPUCores < 0.99 || rec.UpperBound.CPUCores > 1.01 {
		t.Errorf("UpperBound.CPUCores: want ~1.0, got %f", rec.UpperBound.CPUCores)
	}
	// UpperBound Memory: 1Gi = 1073741824
	if rec.UpperBound.MemoryBytes != 1073741824 {
		t.Errorf("UpperBound.MemoryBytes: want 1073741824, got %d", rec.UpperBound.MemoryBytes)
	}

	// Conditions
	if len(info.Conditions) != 1 {
		t.Fatalf("Conditions len: want 1, got %d", len(info.Conditions))
	}
	assertEqual(t, "Condition.Type", info.Conditions[0].Type, "RecommendationProvided")
	assertEqual(t, "Condition.Status", info.Conditions[0].Status, "True")

	// Labels
	assertEqual(t, "Labels[app]", info.Labels["app"], "web")
}

// ---- PDB Tests ----

func TestPDBToModel_MatchLabels(t *testing.T) {
	minAvail := intstr.FromInt32(2)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web-pdb",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"app": "web"},
			Annotations:       map[string]string{"note": "important"},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvail,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
		},
		Status: policyv1.PodDisruptionBudgetStatus{
			CurrentHealthy:     3,
			DesiredHealthy:     2,
			DisruptionsAllowed: 1,
			ExpectedPods:       3,
			Conditions: []metav1.Condition{
				{
					Type:    "DisruptionAllowed",
					Status:  metav1.ConditionTrue,
					Reason:  "SufficientPods",
					Message: "The disruption budget has sufficient pods",
				},
			},
		},
	}

	info := PDBToModel(pdb)

	assertEqual(t, "Name", info.Name, "web-pdb")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "MinAvailable", info.MinAvailable, "2")
	assertEqual(t, "MaxUnavailable", info.MaxUnavailable, "")
	assertEqual(t, "MatchLabels[app]", info.MatchLabels["app"], "web")

	if info.CurrentHealthy != 3 {
		t.Errorf("CurrentHealthy: want 3, got %d", info.CurrentHealthy)
	}
	if info.DesiredHealthy != 2 {
		t.Errorf("DesiredHealthy: want 2, got %d", info.DesiredHealthy)
	}
	if info.DisruptionsAllowed != 1 {
		t.Errorf("DisruptionsAllowed: want 1, got %d", info.DisruptionsAllowed)
	}
	if info.ExpectedPods != 3 {
		t.Errorf("ExpectedPods: want 3, got %d", info.ExpectedPods)
	}

	// Conditions
	if len(info.Conditions) != 1 {
		t.Fatalf("Conditions len: want 1, got %d", len(info.Conditions))
	}
	assertEqual(t, "Condition.Type", info.Conditions[0].Type, "DisruptionAllowed")

	// TargetWorkloads should be empty (enrichment's job)
	if len(info.TargetWorkloads) != 0 {
		t.Errorf("TargetWorkloads should be empty, got %d", len(info.TargetWorkloads))
	}
}

// ---- Job Tests ----

func TestJobToModel_WithCompletionTime(t *testing.T) {
	completions := int32(3)
	parallelism := int32(2)
	backoff := int32(6)
	ttl := int32(100)
	ads := int64(600)
	startTime := metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC))
	completionTime := metav1.NewTime(time.Date(2025, 6, 1, 8, 5, 30, 0, time.UTC))

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "data-migration",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 7, 55, 0, 0, time.UTC)),
			Labels:            map[string]string{"job": "migration"},
			Annotations:       map[string]string{"batch": "v1"},
		},
		Spec: batchv1.JobSpec{
			Completions:             &completions,
			Parallelism:             &parallelism,
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &ads,
			TTLSecondsAfterFinished: &ttl,
		},
		Status: batchv1.JobStatus{
			Active:         0,
			Succeeded:      3,
			Failed:         1,
			StartTime:      &startTime,
			CompletionTime: &completionTime,
			Conditions: []batchv1.JobCondition{
				{
					Type:    batchv1.JobComplete,
					Status:  corev1.ConditionTrue,
					Reason:  "Completed",
					Message: "Job completed successfully",
				},
			},
		},
	}

	info := JobToModel(job)

	assertEqual(t, "Name", info.Name, "data-migration")
	assertEqual(t, "Namespace", info.Namespace, "production")

	if info.Completions == nil || *info.Completions != 3 {
		t.Errorf("Completions: want 3, got %v", info.Completions)
	}
	if info.Parallelism == nil || *info.Parallelism != 2 {
		t.Errorf("Parallelism: want 2, got %v", info.Parallelism)
	}
	if info.BackoffLimit == nil || *info.BackoffLimit != 6 {
		t.Errorf("BackoffLimit: want 6, got %v", info.BackoffLimit)
	}
	if info.ActiveDeadlineSeconds == nil || *info.ActiveDeadlineSeconds != 600 {
		t.Errorf("ActiveDeadlineSeconds: want 600, got %v", info.ActiveDeadlineSeconds)
	}
	if info.TTLSecondsAfterFinished == nil || *info.TTLSecondsAfterFinished != 100 {
		t.Errorf("TTLSecondsAfterFinished: want 100, got %v", info.TTLSecondsAfterFinished)
	}

	if info.Succeeded != 3 {
		t.Errorf("Succeeded: want 3, got %d", info.Succeeded)
	}
	if info.Failed != 1 {
		t.Errorf("Failed: want 1, got %d", info.Failed)
	}

	// Duration
	if info.DurationSeconds == nil {
		t.Fatal("DurationSeconds should not be nil")
	}
	// 5 min 30 sec = 330 sec
	if *info.DurationSeconds < 329.9 || *info.DurationSeconds > 330.1 {
		t.Errorf("DurationSeconds: want ~330, got %f", *info.DurationSeconds)
	}

	// StartTime / CompletionTime
	if info.StartTime == nil {
		t.Fatal("StartTime should not be nil")
	}
	if info.CompletionTime == nil {
		t.Fatal("CompletionTime should not be nil")
	}

	// OwnerCronJob should be empty
	assertEqual(t, "OwnerCronJob", info.OwnerCronJob, "")

	// TotalCPU/Memory should be zero (enrichment's job)
	if info.TotalCPURequest != 0 {
		t.Errorf("TotalCPURequest should be 0, got %f", info.TotalCPURequest)
	}

	// Conditions
	if len(info.Conditions) != 1 {
		t.Fatalf("Conditions len: want 1, got %d", len(info.Conditions))
	}
	assertEqual(t, "Condition.Type", info.Conditions[0].Type, "Complete")
}

func TestJobToModel_OwnedByCronJob(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily-backup-28123456",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "CronJob",
					Name: "daily-backup",
				},
			},
		},
		Spec: batchv1.JobSpec{},
		Status: batchv1.JobStatus{
			Active: 1,
		},
	}

	info := JobToModel(job)

	assertEqual(t, "OwnerCronJob", info.OwnerCronJob, "daily-backup")
	if info.DurationSeconds != nil {
		t.Errorf("DurationSeconds should be nil for running job, got %f", *info.DurationSeconds)
	}
}

// ---- CronJob Tests ----

func TestCronJobToModel_ActiveJobs(t *testing.T) {
	suspend := false
	lastSchedule := metav1.NewTime(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	lastSuccess := metav1.NewTime(time.Date(2025, 5, 31, 0, 0, 0, 0, time.UTC))

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily-backup",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"type": "backup"},
			Annotations:       map[string]string{"team": "platform"},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          "0 0 * * *",
			Suspend:           &suspend,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "backup",
									Image: "backup:v1",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("500m"),
											corev1.ResourceMemory: resource.MustParse("512Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: batchv1.CronJobStatus{
			LastScheduleTime:   &lastSchedule,
			LastSuccessfulTime: &lastSuccess,
			Active: []corev1.ObjectReference{
				{Name: "daily-backup-28123456"},
				{Name: "daily-backup-28123457"},
			},
		},
	}

	info := CronJobToModel(cj)

	assertEqual(t, "Name", info.Name, "daily-backup")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "Schedule", info.Schedule, "0 0 * * *")
	assertEqual(t, "ConcurrencyPolicy", info.ConcurrencyPolicy, "Forbid")
	if info.Suspend {
		t.Error("Suspend should be false")
	}

	if info.LastScheduleTime == nil {
		t.Fatal("LastScheduleTime should not be nil")
	}
	if info.LastSuccessfulTime == nil {
		t.Fatal("LastSuccessfulTime should not be nil")
	}

	// Active jobs
	if len(info.ActiveJobs) != 2 {
		t.Fatalf("ActiveJobs len: want 2, got %d", len(info.ActiveJobs))
	}
	assertEqual(t, "ActiveJobs[0]", info.ActiveJobs[0], "daily-backup-28123456")
	assertEqual(t, "ActiveJobs[1]", info.ActiveJobs[1], "daily-backup-28123457")

	// ContainerSpecs
	if len(info.ContainerSpecs) != 1 {
		t.Fatalf("ContainerSpecs len: want 1, got %d", len(info.ContainerSpecs))
	}
	assertEqual(t, "ContainerSpecs[0].Name", info.ContainerSpecs[0].Name, "backup")
	if info.ContainerSpecs[0].CPURequestCores < 0.49 || info.ContainerSpecs[0].CPURequestCores > 0.51 {
		t.Errorf("ContainerSpecs[0].CPURequestCores: want ~0.5, got %f", info.ContainerSpecs[0].CPURequestCores)
	}
}
