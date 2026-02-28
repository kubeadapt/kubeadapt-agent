package convert

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func benchPod(i int, nodeName string) *corev1.Pod {
	started := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pod-%d", i),
			Namespace: "default",
			Labels: map[string]string{
				"app":                          fmt.Sprintf("app-%d", i%50),
				"version":                      "v1",
				"pod-template-hash":            "abc123",
				"app.kubernetes.io/managed-by": "helm",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "9090",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       fmt.Sprintf("rs-%d", i%50),
					UID:        types.UID(fmt.Sprintf("rs-uid-%d", i%50)),
				},
			},
			CreationTimestamp: metav1.Now(),
		},
		Spec: corev1.PodSpec{
			NodeName:           nodeName,
			PriorityClassName:  "high-priority",
			SchedulerName:      "default-scheduler",
			ServiceAccountName: "default",
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx:1.21",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
					},
				},
				{
					Name:  "sidecar",
					Image: "envoyproxy/envoy:v1.28",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					Ports: []corev1.ContainerPort{
						{Name: "admin", ContainerPort: 9901, Protocol: corev1.ProtocolTCP},
					},
				},
				{
					Name:  "log-collector",
					Image: "fluentd:v1.16",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("25m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBurstable,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					Ready:        true,
					Started:      &started,
					RestartCount: 0,
					ImageID:      "docker-pullable://nginx@sha256:abc123",
					State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:         "sidecar",
					Ready:        true,
					Started:      &started,
					RestartCount: 1,
					ImageID:      "docker-pullable://envoyproxy/envoy@sha256:def456",
					State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:         "log-collector",
					Ready:        true,
					Started:      &started,
					RestartCount: 0,
					ImageID:      "docker-pullable://fluentd@sha256:ghi789",
					State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
				{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
			},
		},
	}
}

// BenchmarkPodToModel measures the conversion of a realistic v1.Pod
// (3 containers with resources, conditions, owner references) to model.PodInfo.
func BenchmarkPodToModel(b *testing.B) {
	b.ReportAllocs()

	pod := benchPod(0, "node-0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PodToModel(pod)
	}
}
