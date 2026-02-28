package convert

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// makePod returns a fully populated running pod (owned by a ReplicaSet).
func makePod() *corev1.Pod {
	isController := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web-abc123-xyz",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels: map[string]string{
				"app":                          "web",
				"pod-template-hash":            "abc123",
				"app.kubernetes.io/managed-by": "Helm",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "8080",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "web-abc123",
					UID:        types.UID("rs-uid-12345"),
					Controller: &isController,
				},
			},
		},
		Spec: corev1.PodSpec{
			NodeName:           "ip-10-0-1-100.ec2.internal",
			PriorityClassName:  "high-priority",
			Priority:           int32Ptr(1000),
			SchedulerName:      "default-scheduler",
			ServiceAccountName: "web-sa",
			Containers: []corev1.Container{
				{
					Name:  "web",
					Image: "nginx:1.25",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:              resource.MustParse("250m"),
							corev1.ResourceMemory:           resource.MustParse("256Mi"),
							corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:              resource.MustParse("500m"),
							corev1.ResourceMemory:           resource.MustParse("512Mi"),
							corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
						},
					},
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
						{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
					},
				},
				{
					Name:  "sidecar",
					Image: "envoyproxy/envoy:v1.28",
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
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBurstable,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "web",
					ImageID: "docker-pullable://nginx@sha256:abc123",
					Ready:   true,
					Started: boolPtr(true),
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 30, 0, time.UTC)),
						},
					},
					RestartCount: 0,
				},
				{
					Name:    "sidecar",
					ImageID: "docker-pullable://envoyproxy/envoy@sha256:def456",
					Ready:   true,
					Started: boolPtr(true),
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 25, 0, time.UTC)),
						},
					},
					RestartCount: 1,
				},
			},
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, Reason: "Scheduled", Message: ""},
				{Type: corev1.PodInitialized, Status: corev1.ConditionTrue, Reason: "", Message: ""},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue, Reason: "", Message: ""},
				{Type: corev1.PodReady, Status: corev1.ConditionTrue, Reason: "", Message: ""},
			},
		},
	}
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }

// 1. Running pod with all fields
func TestPodToModel_RunningPod(t *testing.T) {
	pod := makePod()
	got := PodToModel(pod)

	// Identity
	assertEqual(t, "Name", got.Name, "web-abc123-xyz")
	assertEqual(t, "Namespace", got.Namespace, "production")
	assertEqual(t, "NodeName", got.NodeName, "ip-10-0-1-100.ec2.internal")

	// Status
	assertEqual(t, "Phase", got.Phase, "Running")
	assertEqual(t, "Reason", got.Reason, "")
	assertEqual(t, "QoSClass", got.QoSClass, "Burstable")

	// Owner
	assertEqual(t, "OwnerKind", got.OwnerKind, "ReplicaSet")
	assertEqual(t, "OwnerName", got.OwnerName, "web-abc123")
	assertEqual(t, "OwnerUID", got.OwnerUID, "rs-uid-12345")

	// Scheduling
	assertEqual(t, "PriorityClassName", got.PriorityClassName, "high-priority")
	if got.Priority == nil || *got.Priority != 1000 {
		t.Errorf("Priority = %v, want 1000", got.Priority)
	}
	assertEqual(t, "SchedulerName", got.SchedulerName, "default-scheduler")
	assertEqual(t, "ServiceAccountName", got.ServiceAccountName, "web-sa")

	// Labels — all, no filtering
	if len(got.Labels) != 3 {
		t.Errorf("expected 3 labels, got %d", len(got.Labels))
	}
	assertEqual(t, "Labels[app]", got.Labels["app"], "web")

	// Annotations — filtered
	if len(got.Annotations) != 2 {
		t.Errorf("expected 2 annotations, got %d", len(got.Annotations))
	}

	// CreationTimestamp
	expectedTS := time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC).UnixMilli()
	assertInt64(t, "CreationTimestamp", got.CreationTimestamp, expectedTS)

	// Containers
	if len(got.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(got.Containers))
	}

	web := got.Containers[0]
	assertEqual(t, "web.Name", web.Name, "web")
	assertEqual(t, "web.Image", web.Image, "nginx:1.25")
	assertEqual(t, "web.ImageID", web.ImageID, "docker-pullable://nginx@sha256:abc123")
	assertFloat64(t, "web.CPURequestCores", web.CPURequestCores, 0.25)
	assertInt64(t, "web.MemoryRequestBytes", web.MemoryRequestBytes, 256*1024*1024)
	assertFloat64(t, "web.CPULimitCores", web.CPULimitCores, 0.5)
	assertInt64(t, "web.MemoryLimitBytes", web.MemoryLimitBytes, 512*1024*1024)
	assertInt64(t, "web.EphemeralStorageRequest", web.EphemeralStorageRequest, 1*1024*1024*1024)
	assertInt64(t, "web.EphemeralStorageLimit", web.EphemeralStorageLimit, 2*1024*1024*1024)
	assertInt(t, "web.GPURequest", web.GPURequest, 0)
	assertInt(t, "web.GPULimit", web.GPULimit, 0)
	if web.CPUUsageCores != nil {
		t.Errorf("web.CPUUsageCores should be nil, got %v", *web.CPUUsageCores)
	}
	if web.MemoryUsageBytes != nil {
		t.Errorf("web.MemoryUsageBytes should be nil, got %v", *web.MemoryUsageBytes)
	}
	if !web.Ready {
		t.Error("web.Ready should be true")
	}
	if web.Started == nil || !*web.Started {
		t.Error("web.Started should be true")
	}
	assertInt(t, "web.RestartCount", int(web.RestartCount), 0)
	assertEqual(t, "web.State", web.State, "running")
	assertEqual(t, "web.StateReason", web.StateReason, "")
	assertEqual(t, "web.StateMessage", web.StateMessage, "")
	if web.ExitCode != nil {
		t.Errorf("web.ExitCode should be nil, got %v", *web.ExitCode)
	}
	assertEqual(t, "web.LastTerminationReason", web.LastTerminationReason, "")

	// Ports
	if len(web.Ports) != 2 {
		t.Fatalf("expected 2 ports on web container, got %d", len(web.Ports))
	}
	assertEqual(t, "Port[0].Name", web.Ports[0].Name, "http")
	assertInt(t, "Port[0].ContainerPort", int(web.Ports[0].ContainerPort), 8080)
	assertEqual(t, "Port[0].Protocol", web.Ports[0].Protocol, "TCP")
	assertEqual(t, "Port[1].Name", web.Ports[1].Name, "metrics")

	// Sidecar
	sidecar := got.Containers[1]
	assertEqual(t, "sidecar.Name", sidecar.Name, "sidecar")
	assertFloat64(t, "sidecar.CPURequestCores", sidecar.CPURequestCores, 0.1)
	assertInt64(t, "sidecar.MemoryRequestBytes", sidecar.MemoryRequestBytes, 64*1024*1024)
	assertInt(t, "sidecar.RestartCount", int(sidecar.RestartCount), 1)
	if len(sidecar.Ports) != 0 {
		t.Errorf("expected 0 ports on sidecar, got %d", len(sidecar.Ports))
	}

	// Conditions
	if len(got.Conditions) != 4 {
		t.Fatalf("expected 4 conditions, got %d", len(got.Conditions))
	}
	assertEqual(t, "Condition[0].Type", got.Conditions[0].Type, "PodScheduled")
	assertEqual(t, "Condition[0].Status", got.Conditions[0].Status, "True")
	assertEqual(t, "Condition[3].Type", got.Conditions[3].Type, "Ready")
	assertEqual(t, "Condition[3].Status", got.Conditions[3].Status, "True")
}

// 2. Pending pod (no NodeName, no container statuses)
func TestPodToModel_PendingPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pending-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "myapp:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodPending,
			QOSClass: corev1.PodQOSBurstable,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "0/3 nodes are available"},
			},
		},
	}

	got := PodToModel(pod)

	assertEqual(t, "Phase", got.Phase, "Pending")
	assertEqual(t, "NodeName", got.NodeName, "")

	// Container should still be present (from spec) but with no status
	if len(got.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got.Containers))
	}
	assertEqual(t, "container.Name", got.Containers[0].Name, "app")
	assertEqual(t, "container.Image", got.Containers[0].Image, "myapp:latest")
	assertFloat64(t, "container.CPURequestCores", got.Containers[0].CPURequestCores, 0.1)
	assertEqual(t, "container.State", got.Containers[0].State, "waiting")
	assertEqual(t, "container.ImageID", got.Containers[0].ImageID, "")
	if got.Containers[0].Ready {
		t.Error("container.Ready should be false for pending pod")
	}

	// Conditions
	if len(got.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(got.Conditions))
	}
	assertEqual(t, "Condition.Reason", got.Conditions[0].Reason, "Unschedulable")
	assertEqual(t, "Condition.Message", got.Conditions[0].Message, "0/3 nodes are available")
}

// 3. Failed pod with reason "Evicted"
func TestPodToModel_FailedEvicted(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "evicted-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 7, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1"},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodFailed,
			Reason:   "Evicted",
			QOSClass: corev1.PodQOSBestEffort,
		},
	}

	got := PodToModel(pod)

	assertEqual(t, "Phase", got.Phase, "Failed")
	assertEqual(t, "Reason", got.Reason, "Evicted")
	assertEqual(t, "QoSClass", got.QoSClass, "BestEffort")
}

// 4. Succeeded pod (completed job)
func TestPodToModel_Succeeded(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job-runner-abc",
			Namespace:         "batch",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 6, 0, 0, 0, time.UTC)),
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Job", Name: "job-runner", UID: "job-uid-999"},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "worker", Image: "batch-worker:v2"},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodSucceeded,
			QOSClass: corev1.PodQOSBestEffort,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "worker",
					ImageID: "docker-pullable://batch-worker@sha256:aaa",
					Ready:   false,
					Started: boolPtr(false),
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:   "Completed",
							ExitCode: 0,
						},
					},
				},
			},
		},
	}

	got := PodToModel(pod)

	assertEqual(t, "Phase", got.Phase, "Succeeded")
	assertEqual(t, "OwnerKind", got.OwnerKind, "Job")
	assertEqual(t, "OwnerName", got.OwnerName, "job-runner")

	if len(got.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got.Containers))
	}
	c := got.Containers[0]
	assertEqual(t, "State", c.State, "terminated")
	assertEqual(t, "StateReason", c.StateReason, "Completed")
	if c.ExitCode == nil || *c.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", c.ExitCode)
	}
}

// 5. Container in CrashLoopBackOff (Waiting state with reason)
func TestPodToModel_CrashLoopBackOff(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "crashloop-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 5, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
			Containers: []corev1.Container{
				{Name: "buggy", Image: "buggy:v1"},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBestEffort,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "buggy",
					ImageID: "docker-pullable://buggy@sha256:bbb",
					Ready:   false,
					Started: boolPtr(false),
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "back-off 5m0s restarting failed container=buggy",
						},
					},
					RestartCount: 12,
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:   "Error",
							ExitCode: 1,
						},
					},
				},
			},
		},
	}

	got := PodToModel(pod)

	if len(got.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got.Containers))
	}
	c := got.Containers[0]
	assertEqual(t, "State", c.State, "waiting")
	assertEqual(t, "StateReason", c.StateReason, "CrashLoopBackOff")
	assertEqual(t, "StateMessage", c.StateMessage, "back-off 5m0s restarting failed container=buggy")
	assertInt(t, "RestartCount", int(c.RestartCount), 12)
	assertEqual(t, "LastTerminationReason", c.LastTerminationReason, "Error")
	if c.ExitCode != nil {
		t.Errorf("ExitCode should be nil for waiting container, got %v", *c.ExitCode)
	}
	if c.Ready {
		t.Error("Ready should be false")
	}
	if c.Started == nil || *c.Started {
		t.Error("Started should be false")
	}
}

// 6. Container OOMKilled (Terminated with reason and exit code 137)
func TestPodToModel_OOMKilled(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "oom-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 4, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-2",
			Containers: []corev1.Container{
				{
					Name:  "memory-hog",
					Image: "memhog:v1",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("256Mi"),
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
					Name:  "memory-hog",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:   "OOMKilled",
							ExitCode: 137,
							Message:  "",
						},
					},
					RestartCount: 3,
				},
			},
		},
	}

	got := PodToModel(pod)

	c := got.Containers[0]
	assertEqual(t, "State", c.State, "terminated")
	assertEqual(t, "StateReason", c.StateReason, "OOMKilled")
	if c.ExitCode == nil || *c.ExitCode != 137 {
		t.Errorf("ExitCode = %v, want 137", c.ExitCode)
	}
	assertInt64(t, "MemoryLimitBytes", c.MemoryLimitBytes, 256*1024*1024)
}

// 7. Init containers
func TestPodToModel_InitContainers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "init-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 3, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
			InitContainers: []corev1.Container{
				{
					Name:  "db-migration",
					Image: "flyway:v9",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
				{
					Name:  "config-init",
					Image: "busybox:latest",
				},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1"},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBurstable,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "db-migration",
					ImageID: "docker-pullable://flyway@sha256:ccc",
					Ready:   false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:   "Completed",
							ExitCode: 0,
						},
					},
				},
				{
					Name:    "config-init",
					ImageID: "docker-pullable://busybox@sha256:ddd",
					Ready:   false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:   "Completed",
							ExitCode: 0,
						},
					},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "app",
					ImageID: "docker-pullable://myapp@sha256:eee",
					Ready:   true,
					Started: boolPtr(true),
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	got := PodToModel(pod)

	// Init containers
	if len(got.InitContainers) != 2 {
		t.Fatalf("expected 2 init containers, got %d", len(got.InitContainers))
	}
	assertEqual(t, "init[0].Name", got.InitContainers[0].Name, "db-migration")
	assertEqual(t, "init[0].Image", got.InitContainers[0].Image, "flyway:v9")
	assertFloat64(t, "init[0].CPURequestCores", got.InitContainers[0].CPURequestCores, 0.05)
	assertEqual(t, "init[0].State", got.InitContainers[0].State, "terminated")
	assertEqual(t, "init[0].StateReason", got.InitContainers[0].StateReason, "Completed")

	assertEqual(t, "init[1].Name", got.InitContainers[1].Name, "config-init")

	// Regular containers
	if len(got.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got.Containers))
	}
	assertEqual(t, "app.State", got.Containers[0].State, "running")
}

// 8. GPU container (nvidia.com/gpu in requests/limits)
func TestPodToModel_GPUContainer(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "gpu-pod",
			Namespace:         "ml",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 2, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PodSpec{
			NodeName: "gpu-node-1",
			Containers: []corev1.Container{
				{
					Name:  "trainer",
					Image: "pytorch:v2",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:                    resource.MustParse("4"),
							corev1.ResourceMemory:                 resource.MustParse("32Gi"),
							corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:                    resource.MustParse("8"),
							corev1.ResourceMemory:                 resource.MustParse("64Gi"),
							corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSGuaranteed,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "trainer",
					ImageID: "docker-pullable://pytorch@sha256:fff",
					Ready:   true,
					Started: boolPtr(true),
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	got := PodToModel(pod)

	c := got.Containers[0]
	assertFloat64(t, "CPURequestCores", c.CPURequestCores, 4.0)
	assertInt64(t, "MemoryRequestBytes", c.MemoryRequestBytes, 32*1024*1024*1024)
	assertFloat64(t, "CPULimitCores", c.CPULimitCores, 8.0)
	assertInt64(t, "MemoryLimitBytes", c.MemoryLimitBytes, 64*1024*1024*1024)
	assertInt(t, "GPURequest", c.GPURequest, 2)
	assertInt(t, "GPULimit", c.GPULimit, 2)
}

// 9. Pod with ownerReferences (ReplicaSet owner) — covered by RunningPod test
// Explicit test for owner extraction.
func TestPodToModel_OwnerReference(t *testing.T) {
	pod := makePod()
	got := PodToModel(pod)

	assertEqual(t, "OwnerKind", got.OwnerKind, "ReplicaSet")
	assertEqual(t, "OwnerName", got.OwnerName, "web-abc123")
	assertEqual(t, "OwnerUID", got.OwnerUID, "rs-uid-12345")
}

// 10. Pod with no ownerReferences (orphan)
func TestPodToModel_OrphanPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "orphan-pod",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 1, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1"},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBestEffort,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	got := PodToModel(pod)

	assertEqual(t, "OwnerKind", got.OwnerKind, "")
	assertEqual(t, "OwnerName", got.OwnerName, "")
	assertEqual(t, "OwnerUID", got.OwnerUID, "")
}

// 11. QoS classes (Guaranteed, Burstable, BestEffort)
func TestPodToModel_QoSClasses(t *testing.T) {
	tests := []struct {
		name     string
		qosClass corev1.PodQOSClass
		want     string
	}{
		{"Guaranteed", corev1.PodQOSGuaranteed, "Guaranteed"},
		{"Burstable", corev1.PodQOSBurstable, "Burstable"},
		{"BestEffort", corev1.PodQOSBestEffort, "BestEffort"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "qos-pod",
					Namespace:         "default",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "app:v1"}},
				},
				Status: corev1.PodStatus{
					Phase:    corev1.PodRunning,
					QOSClass: tt.qosClass,
				},
			}
			got := PodToModel(pod)
			assertEqual(t, "QoSClass", got.QoSClass, tt.want)
		})
	}
}

// 12. Pod conditions (PodScheduled, Initialized, ContainersReady, Ready)
func TestPodToModel_Conditions(t *testing.T) {
	pod := makePod()
	got := PodToModel(pod)

	if len(got.Conditions) != 4 {
		t.Fatalf("expected 4 conditions, got %d", len(got.Conditions))
	}

	expected := []struct {
		typ    string
		status string
	}{
		{"PodScheduled", "True"},
		{"Initialized", "True"},
		{"ContainersReady", "True"},
		{"Ready", "True"},
	}

	for i, exp := range expected {
		assertEqual(t, "Condition.Type", got.Conditions[i].Type, exp.typ)
		assertEqual(t, "Condition.Status", got.Conditions[i].Status, exp.status)
	}
}
