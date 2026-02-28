package convert

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// --- helper builders ---

func makeDeployment() *appsv1.Deployment {
	replicas := int32(3)
	maxSurge := intstr.FromString("25%")
	maxUnavailable := intstr.FromString("25%")
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels: map[string]string{
				"app":     "web",
				"version": "v1",
			},
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision":                "3",
				"kubectl.kubernetes.io/last-applied-configuration": `{"big":"json"}`,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "web",
							Image: "nginx:1.25",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("250m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
						{
							Name:  "sidecar",
							Image: "envoy:v1.28",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            3,
			ReadyReplicas:       2,
			AvailableReplicas:   2,
			UnavailableReplicas: 1,
			UpdatedReplicas:     3,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:    appsv1.DeploymentAvailable,
					Status:  corev1.ConditionTrue,
					Reason:  "MinimumReplicasAvailable",
					Message: "Deployment has minimum availability.",
				},
				{
					Type:    appsv1.DeploymentProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "NewReplicaSetAvailable",
					Message: "ReplicaSet \"web-abc\" has successfully progressed.",
				},
			},
		},
	}
}

// 1. Deployment with RollingUpdate strategy
func TestDeploymentToModel_RollingUpdate(t *testing.T) {
	dep := makeDeployment()
	got := DeploymentToModel(dep)

	// Identity
	assertEqual(t, "Name", got.Name, "web")
	assertEqual(t, "Namespace", got.Namespace, "production")

	// Replicas
	assertInt(t, "Replicas", int(got.Replicas), 3)
	assertInt(t, "ReadyReplicas", int(got.ReadyReplicas), 2)
	assertInt(t, "AvailableReplicas", int(got.AvailableReplicas), 2)
	assertInt(t, "UnavailableReplicas", int(got.UnavailableReplicas), 1)
	assertInt(t, "UpdatedReplicas", int(got.UpdatedReplicas), 3)

	// Strategy
	assertEqual(t, "Strategy", got.Strategy, "RollingUpdate")
	assertEqual(t, "MaxSurge", got.MaxSurge, "25%")
	assertEqual(t, "MaxUnavailable", got.MaxUnavailable, "25%")

	// TotalCPU/Memory should be 0 (enrichment later)
	assertFloat64(t, "TotalCPURequest", got.TotalCPURequest, 0)
	assertInt64(t, "TotalMemoryRequest", got.TotalMemoryRequest, 0)
	assertFloat64(t, "TotalCPULimit", got.TotalCPULimit, 0)
	assertInt64(t, "TotalMemoryLimit", got.TotalMemoryLimit, 0)
	if got.TotalCPUUsage != nil {
		t.Errorf("TotalCPUUsage should be nil, got %v", *got.TotalCPUUsage)
	}
	if got.TotalMemoryUsage != nil {
		t.Errorf("TotalMemoryUsage should be nil, got %v", *got.TotalMemoryUsage)
	}

	// ContainerSpecs
	if len(got.ContainerSpecs) != 2 {
		t.Fatalf("expected 2 container specs, got %d", len(got.ContainerSpecs))
	}
	web := got.ContainerSpecs[0]
	assertEqual(t, "cs[0].Name", web.Name, "web")
	assertEqual(t, "cs[0].Image", web.Image, "nginx:1.25")
	assertFloat64(t, "cs[0].CPURequestCores", web.CPURequestCores, 0.25)
	assertInt64(t, "cs[0].MemoryRequestBytes", web.MemoryRequestBytes, 256*1024*1024)
	assertFloat64(t, "cs[0].CPULimitCores", web.CPULimitCores, 0.5)
	assertInt64(t, "cs[0].MemoryLimitBytes", web.MemoryLimitBytes, 512*1024*1024)

	sidecar := got.ContainerSpecs[1]
	assertEqual(t, "cs[1].Name", sidecar.Name, "sidecar")
	assertFloat64(t, "cs[1].CPURequestCores", sidecar.CPURequestCores, 0.1)
	assertInt64(t, "cs[1].MemoryRequestBytes", sidecar.MemoryRequestBytes, 64*1024*1024)

	// Selector
	if got.Selector["app"] != "web" {
		t.Errorf("Selector[app] = %q, want %q", got.Selector["app"], "web")
	}

	// Labels — ALL labels
	if len(got.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(got.Labels))
	}
	assertEqual(t, "Labels[app]", got.Labels["app"], "web")

	// Annotations — filtered (last-applied-configuration removed)
	if len(got.Annotations) != 1 {
		t.Errorf("expected 1 annotation (filtered), got %d", len(got.Annotations))
	}
	if _, ok := got.Annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Error("last-applied-configuration should be filtered out")
	}

	// CreationTimestamp
	expectedTS := time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC).UnixMilli()
	assertInt64(t, "CreationTimestamp", got.CreationTimestamp, expectedTS)

	// Conditions
	if len(got.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(got.Conditions))
	}
	assertEqual(t, "Condition[0].Type", got.Conditions[0].Type, "Available")
	assertEqual(t, "Condition[0].Status", got.Conditions[0].Status, "True")
	assertEqual(t, "Condition[0].Reason", got.Conditions[0].Reason, "MinimumReplicasAvailable")
	assertEqual(t, "Condition[1].Type", got.Conditions[1].Type, "Progressing")
	assertEqual(t, "Condition[1].Status", got.Conditions[1].Status, "True")

	// Not paused
	if got.Paused {
		t.Error("Paused should be false")
	}
}

// 2. Deployment with Recreate strategy
func TestDeploymentToModel_Recreate(t *testing.T) {
	dep := makeDeployment()
	dep.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}

	got := DeploymentToModel(dep)

	assertEqual(t, "Strategy", got.Strategy, "Recreate")
	assertEqual(t, "MaxSurge", got.MaxSurge, "")
	assertEqual(t, "MaxUnavailable", got.MaxUnavailable, "")
}

// 3. Deployment paused
func TestDeploymentToModel_Paused(t *testing.T) {
	dep := makeDeployment()
	dep.Spec.Paused = true

	got := DeploymentToModel(dep)

	if !got.Paused {
		t.Error("Paused should be true")
	}
}

// 4. Deployment with nil Replicas defaults to 1
func TestDeploymentToModel_NilReplicas(t *testing.T) {
	dep := makeDeployment()
	dep.Spec.Replicas = nil

	got := DeploymentToModel(dep)

	assertInt(t, "Replicas", int(got.Replicas), 1)
}

// 5. StatefulSet with VolumeClaimTemplates and OrderedReady policy
func TestStatefulSetToModel_OrderedReady(t *testing.T) {
	replicas := int32(3)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "redis",
			Namespace:         "cache",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 7, 1, 10, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"app": "redis"},
			Annotations: map[string]string{
				"helm.sh/chart": "redis-17.0.0",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:            &replicas,
			ServiceName:         "redis-headless",
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "redis"},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:7.2",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "logs"}},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:          3,
			ReadyReplicas:     3,
			AvailableReplicas: 3,
			UpdatedReplicas:   3,
			Conditions:        []appsv1.StatefulSetCondition{
				// StatefulSets rarely have conditions, but model supports it
			},
		},
	}

	got := StatefulSetToModel(ss)

	assertEqual(t, "Name", got.Name, "redis")
	assertEqual(t, "Namespace", got.Namespace, "cache")
	assertInt(t, "Replicas", int(got.Replicas), 3)
	assertInt(t, "ReadyReplicas", int(got.ReadyReplicas), 3)
	assertInt(t, "AvailableReplicas", int(got.AvailableReplicas), 3)
	assertInt(t, "UpdatedReplicas", int(got.UpdatedReplicas), 3)

	assertEqual(t, "Strategy", got.Strategy, "RollingUpdate")
	assertEqual(t, "ServiceName", got.ServiceName, "redis-headless")
	assertEqual(t, "PodManagementPolicy", got.PodManagementPolicy, "OrderedReady")

	// VolumeClaimTemplates
	if len(got.VolumeClaimTemplates) != 2 {
		t.Fatalf("expected 2 volume claim templates, got %d", len(got.VolumeClaimTemplates))
	}
	assertEqual(t, "VCT[0]", got.VolumeClaimTemplates[0], "data")
	assertEqual(t, "VCT[1]", got.VolumeClaimTemplates[1], "logs")

	// ContainerSpecs
	if len(got.ContainerSpecs) != 1 {
		t.Fatalf("expected 1 container spec, got %d", len(got.ContainerSpecs))
	}
	assertFloat64(t, "cs.CPURequestCores", got.ContainerSpecs[0].CPURequestCores, 0.5)
	assertInt64(t, "cs.MemoryRequestBytes", got.ContainerSpecs[0].MemoryRequestBytes, 1*1024*1024*1024)

	// Selector
	if got.Selector["app"] != "redis" {
		t.Errorf("Selector[app] = %q, want %q", got.Selector["app"], "redis")
	}

	// Labels
	assertEqual(t, "Labels[app]", got.Labels["app"], "redis")

	// Annotations
	assertEqual(t, "Annotations[helm.sh/chart]", got.Annotations["helm.sh/chart"], "redis-17.0.0")

	// CreationTimestamp
	expectedTS := time.Date(2025, 7, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	assertInt64(t, "CreationTimestamp", got.CreationTimestamp, expectedTS)

	// TotalCPU/Memory should be 0 (enrichment)
	assertFloat64(t, "TotalCPURequest", got.TotalCPURequest, 0)
	assertInt64(t, "TotalMemoryRequest", got.TotalMemoryRequest, 0)
}

// 6. StatefulSet with Parallel pod management
func TestStatefulSetToModel_Parallel(t *testing.T) {
	replicas := int32(5)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "kafka",
			Namespace:         "streaming",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 7, 2, 12, 0, 0, 0, time.UTC)),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:            &replicas,
			ServiceName:         "kafka-headless",
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "kafka"},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "kafka", Image: "kafka:3.5"},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      5,
			ReadyReplicas: 4,
		},
	}

	got := StatefulSetToModel(ss)

	assertEqual(t, "PodManagementPolicy", got.PodManagementPolicy, "Parallel")
	assertEqual(t, "Strategy", got.Strategy, "OnDelete")
	assertInt(t, "Replicas", int(got.Replicas), 5)
	assertInt(t, "ReadyReplicas", int(got.ReadyReplicas), 4)

	// No VolumeClaimTemplates
	if len(got.VolumeClaimTemplates) != 0 {
		t.Errorf("expected 0 VCTs, got %d", len(got.VolumeClaimTemplates))
	}
}

// 7. StatefulSet with nil Replicas defaults to 1
func TestStatefulSetToModel_NilReplicas(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-ss",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "app:v1"}},
				},
			},
		},
	}

	got := StatefulSetToModel(ss)
	assertInt(t, "Replicas", int(got.Replicas), 1)
}

// 8. DaemonSet with all status fields
func TestDaemonSetToModel_AllFields(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "fluentd",
			Namespace:         "logging",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 5, 15, 6, 0, 0, 0, time.UTC)),
			Labels: map[string]string{
				"app":  "fluentd",
				"tier": "logging",
			},
			Annotations: map[string]string{
				"deprecated.daemonset.template.generation": "2",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "fluentd"},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "fluentd",
							Image: "fluentd:v1.16",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 10,
			CurrentNumberScheduled: 10,
			NumberReady:            9,
			NumberMisscheduled:     1,
			UpdatedNumberScheduled: 8,
			Conditions:             []appsv1.DaemonSetCondition{},
		},
	}

	got := DaemonSetToModel(ds)

	assertEqual(t, "Name", got.Name, "fluentd")
	assertEqual(t, "Namespace", got.Namespace, "logging")

	assertInt(t, "DesiredNumberScheduled", int(got.DesiredNumberScheduled), 10)
	assertInt(t, "CurrentNumberScheduled", int(got.CurrentNumberScheduled), 10)
	assertInt(t, "NumberReady", int(got.NumberReady), 9)
	assertInt(t, "NumberMisscheduled", int(got.NumberMisscheduled), 1)
	assertInt(t, "UpdatedNumberScheduled", int(got.UpdatedNumberScheduled), 8)

	assertEqual(t, "Strategy", got.Strategy, "RollingUpdate")

	// TotalCPU/Memory should be 0
	assertFloat64(t, "TotalCPURequest", got.TotalCPURequest, 0)
	assertInt64(t, "TotalMemoryRequest", got.TotalMemoryRequest, 0)

	// ContainerSpecs
	if len(got.ContainerSpecs) != 1 {
		t.Fatalf("expected 1 container spec, got %d", len(got.ContainerSpecs))
	}
	assertFloat64(t, "cs.CPURequestCores", got.ContainerSpecs[0].CPURequestCores, 0.2)
	assertInt64(t, "cs.MemoryRequestBytes", got.ContainerSpecs[0].MemoryRequestBytes, 512*1024*1024)
	assertFloat64(t, "cs.CPULimitCores", got.ContainerSpecs[0].CPULimitCores, 0.5)
	assertInt64(t, "cs.MemoryLimitBytes", got.ContainerSpecs[0].MemoryLimitBytes, 1*1024*1024*1024)

	// Selector
	if got.Selector["app"] != "fluentd" {
		t.Errorf("Selector[app] = %q, want %q", got.Selector["app"], "fluentd")
	}

	// Labels
	if len(got.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(got.Labels))
	}

	// Annotations
	assertEqual(t, "Annotations", got.Annotations["deprecated.daemonset.template.generation"], "2")

	// CreationTimestamp
	expectedTS := time.Date(2025, 5, 15, 6, 0, 0, 0, time.UTC).UnixMilli()
	assertInt64(t, "CreationTimestamp", got.CreationTimestamp, expectedTS)
}

// 9. ReplicaSet with owner (Deployment)
func TestReplicaSetToModel_WithOwner(t *testing.T) {
	replicas := int32(3)
	isController := true
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web-abc123",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels: map[string]string{
				"app":               "web",
				"pod-template-hash": "abc123",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "web",
					UID:        types.UID("deploy-uid-999"),
					Controller: &isController,
				},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web", "pod-template-hash": "abc123"},
			},
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:      3,
			ReadyReplicas: 2,
		},
	}

	got := ReplicaSetToModel(rs)

	assertEqual(t, "Name", got.Name, "web-abc123")
	assertEqual(t, "Namespace", got.Namespace, "production")
	assertInt(t, "Replicas", int(got.Replicas), 3)
	assertInt(t, "ReadyReplicas", int(got.ReadyReplicas), 2)

	// Owner
	assertEqual(t, "OwnerKind", got.OwnerKind, "Deployment")
	assertEqual(t, "OwnerName", got.OwnerName, "web")
	assertEqual(t, "OwnerUID", got.OwnerUID, "deploy-uid-999")

	// Selector
	if got.Selector["app"] != "web" {
		t.Errorf("Selector[app] = %q, want %q", got.Selector["app"], "web")
	}
	if got.Selector["pod-template-hash"] != "abc123" {
		t.Errorf("Selector[pod-template-hash] = %q, want %q", got.Selector["pod-template-hash"], "abc123")
	}

	// Labels
	if len(got.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(got.Labels))
	}

	// CreationTimestamp
	expectedTS := time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC).UnixMilli()
	assertInt64(t, "CreationTimestamp", got.CreationTimestamp, expectedTS)
}

// 10. ReplicaSet standalone (no owner)
func TestReplicaSetToModel_Standalone(t *testing.T) {
	replicas := int32(2)
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "standalone-rs",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 2, 9, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"app": "standalone"},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "standalone"},
			},
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
		},
	}

	got := ReplicaSetToModel(rs)

	assertEqual(t, "Name", got.Name, "standalone-rs")
	assertEqual(t, "OwnerKind", got.OwnerKind, "")
	assertEqual(t, "OwnerName", got.OwnerName, "")
	assertEqual(t, "OwnerUID", got.OwnerUID, "")
	assertInt(t, "Replicas", int(got.Replicas), 2)
	assertInt(t, "ReadyReplicas", int(got.ReadyReplicas), 2)
}

// 11. ReplicaSet with nil Replicas defaults to 1
func TestReplicaSetToModel_NilReplicas(t *testing.T) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "nil-replicas-rs",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}

	got := ReplicaSetToModel(rs)
	assertInt(t, "Replicas", int(got.Replicas), 1)
}

// 12. Workload conditions (Available, Progressing, ReplicaFailure)
func TestDeploymentToModel_Conditions(t *testing.T) {
	dep := makeDeployment()
	dep.Status.Conditions = append(dep.Status.Conditions, appsv1.DeploymentCondition{
		Type:    appsv1.DeploymentReplicaFailure,
		Status:  corev1.ConditionTrue,
		Reason:  "FailedCreate",
		Message: "quota exceeded",
	})

	got := DeploymentToModel(dep)

	if len(got.Conditions) != 3 {
		t.Fatalf("expected 3 conditions, got %d", len(got.Conditions))
	}
	assertEqual(t, "Condition[0].Type", got.Conditions[0].Type, "Available")
	assertEqual(t, "Condition[1].Type", got.Conditions[1].Type, "Progressing")
	assertEqual(t, "Condition[2].Type", got.Conditions[2].Type, "ReplicaFailure")
	assertEqual(t, "Condition[2].Status", got.Conditions[2].Status, "True")
	assertEqual(t, "Condition[2].Reason", got.Conditions[2].Reason, "FailedCreate")
	assertEqual(t, "Condition[2].Message", got.Conditions[2].Message, "quota exceeded")
}

// 13. ContainerSpecs extraction with multiple containers (edge cases)
func TestExtractContainerSpecs(t *testing.T) {
	containers := []corev1.Container{
		{
			Name:  "main",
			Image: "app:v1",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
		},
		{
			Name:  "sidecar",
			Image: "proxy:v2",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		},
		{
			// No resources at all
			Name:  "no-resources",
			Image: "busybox:latest",
		},
	}

	got := extractContainerSpecs(containers)

	if len(got) != 3 {
		t.Fatalf("expected 3 container specs, got %d", len(got))
	}

	// main
	assertEqual(t, "main.Name", got[0].Name, "main")
	assertEqual(t, "main.Image", got[0].Image, "app:v1")
	assertFloat64(t, "main.CPURequestCores", got[0].CPURequestCores, 1.0)
	assertInt64(t, "main.MemoryRequestBytes", got[0].MemoryRequestBytes, 2*1024*1024*1024)
	assertFloat64(t, "main.CPULimitCores", got[0].CPULimitCores, 2.0)
	assertInt64(t, "main.MemoryLimitBytes", got[0].MemoryLimitBytes, 4*1024*1024*1024)

	// sidecar — has requests but no limits
	assertEqual(t, "sidecar.Name", got[1].Name, "sidecar")
	assertFloat64(t, "sidecar.CPURequestCores", got[1].CPURequestCores, 0.1)
	assertInt64(t, "sidecar.MemoryRequestBytes", got[1].MemoryRequestBytes, 64*1024*1024)
	assertFloat64(t, "sidecar.CPULimitCores", got[1].CPULimitCores, 0)
	assertInt64(t, "sidecar.MemoryLimitBytes", got[1].MemoryLimitBytes, 0)

	// no-resources — all zeros
	assertEqual(t, "no-resources.Name", got[2].Name, "no-resources")
	assertFloat64(t, "no-resources.CPURequestCores", got[2].CPURequestCores, 0)
	assertInt64(t, "no-resources.MemoryRequestBytes", got[2].MemoryRequestBytes, 0)
	assertFloat64(t, "no-resources.CPULimitCores", got[2].CPULimitCores, 0)
	assertInt64(t, "no-resources.MemoryLimitBytes", got[2].MemoryLimitBytes, 0)
}

// 14. extractContainerSpecs with nil/empty slice
func TestExtractContainerSpecs_Empty(t *testing.T) {
	got := extractContainerSpecs(nil)
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}

	got = extractContainerSpecs([]corev1.Container{})
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

// 15. Deployment with MaxSurge/MaxUnavailable as integers
func TestDeploymentToModel_RollingUpdateIntegers(t *testing.T) {
	dep := makeDeployment()
	maxSurge := intstr.FromInt32(1)
	maxUnavailable := intstr.FromInt32(0)
	dep.Spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{
		MaxSurge:       &maxSurge,
		MaxUnavailable: &maxUnavailable,
	}

	got := DeploymentToModel(dep)

	assertEqual(t, "MaxSurge", got.MaxSurge, "1")
	assertEqual(t, "MaxUnavailable", got.MaxUnavailable, "0")
}

// 16. DaemonSet conditions
func TestDaemonSetToModel_WithConditions(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "node-exporter",
			Namespace:         "monitoring",
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "node-exporter"},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.OnDeleteDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "exporter", Image: "prom/node-exporter:v1.7"},
					},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 5,
			NumberReady:            5,
		},
	}

	got := DaemonSetToModel(ds)

	assertEqual(t, "Strategy", got.Strategy, "OnDelete")
	assertInt(t, "DesiredNumberScheduled", int(got.DesiredNumberScheduled), 5)
	assertInt(t, "NumberReady", int(got.NumberReady), 5)
}
