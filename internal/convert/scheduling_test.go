package convert

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---- PriorityClass Tests ----

func TestPriorityClassToModel(t *testing.T) {
	preempt := corev1.PreemptNever

	pc := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "high-priority",
		},
		Value:            1000000,
		GlobalDefault:    false,
		PreemptionPolicy: &preempt,
		Description:      "High priority for critical workloads",
	}

	info := PriorityClassToModel(pc)

	assertEqual(t, "Name", info.Name, "high-priority")
	if info.Value != 1000000 {
		t.Errorf("Value: want 1000000, got %d", info.Value)
	}
	if info.GlobalDefault {
		t.Error("GlobalDefault should be false")
	}
	assertEqual(t, "PreemptionPolicy", info.PreemptionPolicy, "Never")
	assertEqual(t, "Description", info.Description, "High priority for critical workloads")
}

// ---- LimitRange Tests ----

func TestLimitRangeToModel_ContainerLimits(t *testing.T) {
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "container-limits",
			Namespace: "production",
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Max: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
					Min: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
		},
	}

	info := LimitRangeToModel(lr)

	assertEqual(t, "Name", info.Name, "container-limits")
	assertEqual(t, "Namespace", info.Namespace, "production")

	if len(info.Limits) != 1 {
		t.Fatalf("Limits len: want 1, got %d", len(info.Limits))
	}

	lim := info.Limits[0]
	assertEqual(t, "Type", lim.Type, "Container")

	assertEqual(t, "Default[cpu]", lim.Default["cpu"], "500m")
	assertEqual(t, "Default[memory]", lim.Default["memory"], "512Mi")
	assertEqual(t, "DefaultRequest[cpu]", lim.DefaultRequest["cpu"], "100m")
	assertEqual(t, "DefaultRequest[memory]", lim.DefaultRequest["memory"], "128Mi")
	assertEqual(t, "Max[cpu]", lim.Max["cpu"], "4")
	assertEqual(t, "Max[memory]", lim.Max["memory"], "8Gi")
	assertEqual(t, "Min[cpu]", lim.Min["cpu"], "50m")
	assertEqual(t, "Min[memory]", lim.Min["memory"], "64Mi")
}

// ---- ResourceQuota Tests ----

func TestResourceQuotaToModel_HardUsed(t *testing.T) {
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team-quota",
			Namespace: "production",
			Labels:    map[string]string{"team": "platform"},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20"),
				corev1.ResourceMemory: resource.MustParse("40Gi"),
				corev1.ResourcePods:   resource.MustParse("50"),
			},
		},
		Status: corev1.ResourceQuotaStatus{
			Used: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				corev1.ResourcePods:   resource.MustParse("12"),
			},
		},
	}

	info := ResourceQuotaToModel(rq)

	assertEqual(t, "Name", info.Name, "team-quota")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "Labels[team]", info.Labels["team"], "platform")

	assertEqual(t, "Hard[cpu]", info.Hard["cpu"], "20")
	assertEqual(t, "Hard[memory]", info.Hard["memory"], "40Gi")
	assertEqual(t, "Hard[pods]", info.Hard["pods"], "50")

	assertEqual(t, "Used[cpu]", info.Used["cpu"], "8")
	assertEqual(t, "Used[memory]", info.Used["memory"], "16Gi")
	assertEqual(t, "Used[pods]", info.Used["pods"], "12")
}

// ---- Namespace Tests ----

func TestNamespaceToModel(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
			Labels: map[string]string{
				"name":                               "production",
				"kubernetes.io/metadata.name":        "production",
				"pod-security.kubernetes.io/enforce": "restricted",
			},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": `{"big":"json"}`,
				"team": "platform",
			},
		},
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		},
	}

	info := NamespaceToModel(ns)

	assertEqual(t, "Name", info.Name, "production")
	assertEqual(t, "Phase", info.Phase, "Active")
	assertEqual(t, "Labels[name]", info.Labels["name"], "production")

	// Annotations should be filtered (no last-applied-configuration)
	if _, ok := info.Annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Error("last-applied-configuration should be filtered out")
	}
	assertEqual(t, "Annotations[team]", info.Annotations["team"], "platform")

	expectedTS := metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)).UnixMilli()
	if info.CreationTimestamp != expectedTS {
		t.Errorf("CreationTimestamp: want %d, got %d", expectedTS, info.CreationTimestamp)
	}
}
