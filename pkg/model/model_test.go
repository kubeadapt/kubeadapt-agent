package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// roundTrip marshals v to JSON and unmarshals into a new value of the same type.
// Returns the deserialized value and any error.
func roundTrip[T any](t *testing.T, v T) T {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// assertEqual compares two values using reflect.DeepEqual and fails with a diff if they differ.
func assertEqual[T any](t *testing.T, name string, want, got T) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		t.Errorf("%s mismatch:\nwant: %s\ngot:  %s", name, wantJSON, gotJSON)
	}
}

// assertJSONFieldAbsent verifies that a JSON key is absent when a field is zero/nil (omitempty).
func assertJSONFieldAbsent(t *testing.T, data []byte, key string) {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if _, ok := m[key]; ok {
		t.Errorf("expected JSON key %q to be absent (omitempty), but it was present", key)
	}
}

// assertJSONFieldPresent verifies that a JSON key is present.
func assertJSONFieldPresent(t *testing.T, data []byte, key string) {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if _, ok := m[key]; !ok {
		t.Errorf("expected JSON key %q to be present, but it was absent", key)
	}
}

// --- Snapshot & top-level types ---

func TestClusterSnapshot_RoundTrip(t *testing.T) {
	cpu := 2.5
	mem := int64(1024 * 1024 * 512)
	replicas := int32(3)
	readyReplicas := int32(2)

	orig := ClusterSnapshot{
		SnapshotID:        "snap-001",
		ClusterID:         "cls-abc",
		ClusterName:       "prod-us-east",
		Timestamp:         1700000000000,
		AgentVersion:      "2.0.0",
		Provider:          "aws",
		Region:            "us-east-1",
		KubernetesVersion: "1.28.3",
		Nodes: []NodeInfo{{
			Name:                "node-1",
			CPUCapacityCores:    4.0,
			MemoryCapacityBytes: 8589934592,
			Ready:               true,
		}},
		Pods: []PodInfo{{
			Name:      "api-pod-1",
			Namespace: "default",
			Phase:     "Running",
		}},
		Namespaces:  []NamespaceInfo{{Name: "default", Phase: "Active"}},
		Deployments: []DeploymentInfo{{Name: "api", Namespace: "default", Replicas: 3}},
		StatefulSets: []StatefulSetInfo{{
			Name: "db", Namespace: "default", Replicas: 3,
			ServiceName: "db-headless",
		}},
		DaemonSets: []DaemonSetInfo{{Name: "monitor", Namespace: "kube-system"}},
		Jobs:       []JobInfo{{Name: "migrate", Namespace: "default"}},
		CronJobs:   []CronJobInfo{{Name: "backup", Namespace: "default", Schedule: "0 * * * *"}},
		CustomWorkloads: []CustomWorkloadInfo{{
			APIVersion:    "argoproj.io/v1alpha1",
			Kind:          "Rollout",
			Name:          "web",
			Namespace:     "default",
			Replicas:      &replicas,
			ReadyReplicas: &readyReplicas,
		}},
		HPAs:            []HPAInfo{{Name: "api-hpa", Namespace: "default", MaxReplicas: 10}},
		VPAs:            []VPAInfo{{Name: "api-vpa", Namespace: "default", UpdateMode: "Auto"}},
		PDBs:            []PDBInfo{{Name: "api-pdb", Namespace: "default", DesiredHealthy: 2}},
		Services:        []ServiceInfo{{Name: "api-svc", Namespace: "default", Type: "ClusterIP"}},
		Ingresses:       []IngressInfo{{Name: "api-ing", Namespace: "default"}},
		PVs:             []PVInfo{{Name: "pv-1", Phase: "Bound"}},
		PVCs:            []PVCInfo{{Name: "data-0", Namespace: "default", Phase: "Bound"}},
		StorageClasses:  []StorageClassInfo{{Name: "gp3", Provisioner: "ebs.csi.aws.com"}},
		PriorityClasses: []PriorityClassInfo{{Name: "high-priority", Value: 1000}},
		LimitRanges:     []LimitRangeInfo{{Name: "default-limits", Namespace: "default"}},
		ResourceQuotas:  []ResourceQuotaInfo{{Name: "compute-quota", Namespace: "default"}},
		NodePools:       []NodePoolInfo{{Name: "default", MinReplicas: intPtr(1), MaxReplicas: intPtr(10)}},
		Summary: ClusterSummary{
			NodeCount:        1,
			PodCount:         1,
			TotalCPUUsage:    &cpu,
			TotalMemoryUsage: &mem,
		},
		Health: AgentHealth{
			State:         "running",
			UptimeSeconds: 3600,
			StartedAt:     1700000000000,
		},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "ClusterSnapshot", orig, got)
}

func TestClusterSnapshot_OmitEmpty(t *testing.T) {
	snap := ClusterSnapshot{
		SnapshotID: "snap-002",
		ClusterID:  "cls-abc",
		Nodes:      []NodeInfo{},
		Pods:       []PodInfo{},
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}

	// VPAs should be omitted when nil
	assertJSONFieldAbsent(t, data, "vpas")
	// NodePools should be omitted when nil
	assertJSONFieldAbsent(t, data, "node_pools")
	// CustomWorkloads should be present even when nil (not omitempty per spec, but check the spec says omitempty for custom_workloads — actually it doesn't have omitempty)
	// nodes should be present (not omitempty)
	assertJSONFieldPresent(t, data, "nodes")
}

// --- Node types ---

func TestNodeInfo_RoundTrip(t *testing.T) {
	cpuUsage := 1.5
	memUsage := int64(4294967296)

	orig := NodeInfo{
		Name:                  "ip-10-0-1-100",
		ProviderID:            "aws:///us-east-1a/i-abc123",
		InstanceID:            "i-abc123",
		InstanceType:          "m5.xlarge",
		Region:                "us-east-1",
		Zone:                  "us-east-1a",
		CapacityType:          "on-demand",
		NodeGroup:             "default-ng",
		Architecture:          "amd64",
		OS:                    "linux",
		KubeletVersion:        "v1.28.3",
		ContainerRuntime:      "containerd://1.7.2",
		CPUCapacityCores:      4.0,
		MemoryCapacityBytes:   17179869184,
		EphemeralStorageBytes: 107374182400,
		PodCapacity:           110,
		GPUCapacity:           0,
		CPUAllocatable:        3.92,
		MemoryAllocatable:     16442450944,
		PodAllocatable:        110,
		GPUAllocatable:        0,
		CPUUsageCores:         &cpuUsage,
		MemoryUsageBytes:      &memUsage,
		Ready:                 true,
		Unschedulable:         false,
		Taints: []TaintInfo{{
			Key: "dedicated", Value: "gpu", Effect: "NoSchedule",
		}},
		Conditions: []NodeConditionInfo{{
			Type: "Ready", Status: "True", Reason: "KubeletReady",
			Message: "kubelet is posting ready status",
		}},
		Labels:            map[string]string{"node.kubernetes.io/instance-type": "m5.xlarge"},
		Annotations:       map[string]string{"node.alpha.kubernetes.io/ttl": "0"},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "NodeInfo", orig, got)
}

func TestNodeInfo_NilMetrics(t *testing.T) {
	node := NodeInfo{Name: "test-node"}
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONFieldAbsent(t, data, "cpu_usage_cores")
	assertJSONFieldAbsent(t, data, "memory_usage_bytes")
}

// --- Pod types ---

func TestPodInfo_RoundTrip(t *testing.T) {
	cpuUsage := 0.25
	memUsage := int64(134217728)
	started := true
	exitCode := int32(0)
	priority := int32(100)

	orig := PodInfo{
		Name:      "api-server-7b9f4c6d8-xyz12",
		Namespace: "production",
		NodeName:  "ip-10-0-1-100",
		Phase:     "Running",
		Reason:    "",
		QoSClass:  "Burstable",
		OwnerKind: "Deployment",
		OwnerName: "api-server",
		OwnerUID:  "uid-123",
		Containers: []ContainerInfo{{
			Name:               "api",
			Image:              "api-server:v2.1.0",
			ImageID:            "sha256:abc123",
			CPURequestCores:    0.5,
			MemoryRequestBytes: 268435456,
			CPULimitCores:      1.0,
			MemoryLimitBytes:   536870912,
			CPUUsageCores:      &cpuUsage,
			MemoryUsageBytes:   &memUsage,
			Ready:              true,
			Started:            &started,
			RestartCount:       0,
			State:              "running",
			ExitCode:           &exitCode,
			Ports: []ContainerPortInfo{{
				Name:          "http",
				ContainerPort: 8080,
				Protocol:      "TCP",
			}},
		}},
		InitContainers: []ContainerInfo{{
			Name:        "init-db",
			Image:       "db-init:v1",
			State:       "terminated",
			StateReason: "Completed",
		}},
		Labels:             map[string]string{"app": "api-server"},
		Annotations:        map[string]string{},
		CreationTimestamp:  1700000000000,
		PriorityClassName:  "high-priority",
		Priority:           &priority,
		SchedulerName:      "default-scheduler",
		ServiceAccountName: "api-sa",
		Conditions: []PodConditionInfo{{
			Type:   "Ready",
			Status: "True",
		}},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "PodInfo", orig, got)
}

// --- Workload types ---

func TestDeploymentInfo_RoundTrip(t *testing.T) {
	cpuUsage := 2.0
	memUsage := int64(2147483648)

	orig := DeploymentInfo{
		Name:                "api-server",
		Namespace:           "production",
		Replicas:            3,
		ReadyReplicas:       3,
		AvailableReplicas:   3,
		UnavailableReplicas: 0,
		UpdatedReplicas:     3,
		Strategy:            "RollingUpdate",
		MaxSurge:            "25%",
		MaxUnavailable:      "25%",
		TotalCPURequest:     1.5,
		TotalMemoryRequest:  805306368,
		TotalCPULimit:       3.0,
		TotalMemoryLimit:    1610612736,
		TotalCPUUsage:       &cpuUsage,
		TotalMemoryUsage:    &memUsage,
		ContainerSpecs: []ContainerSpecInfo{{
			Name:               "api",
			Image:              "api-server:v2.1.0",
			CPURequestCores:    0.5,
			MemoryRequestBytes: 268435456,
			CPULimitCores:      1.0,
			MemoryLimitBytes:   536870912,
		}},
		Selector:          map[string]string{"app": "api-server"},
		Labels:            map[string]string{"app": "api-server"},
		Annotations:       map[string]string{},
		CreationTimestamp: 1700000000000,
		Conditions: []WorkloadConditionInfo{{
			Type:    "Available",
			Status:  "True",
			Reason:  "MinimumReplicasAvailable",
			Message: "Deployment has minimum availability.",
		}},
		Paused: false,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "DeploymentInfo", orig, got)
}

func TestStatefulSetInfo_RoundTrip(t *testing.T) {
	orig := StatefulSetInfo{
		Name:                 "db",
		Namespace:            "production",
		Replicas:             3,
		ReadyReplicas:        3,
		AvailableReplicas:    3,
		UpdatedReplicas:      3,
		Strategy:             "RollingUpdate",
		ServiceName:          "db-headless",
		PodManagementPolicy:  "OrderedReady",
		TotalCPURequest:      3.0,
		TotalMemoryRequest:   3221225472,
		ContainerSpecs:       []ContainerSpecInfo{{Name: "db", Image: "postgres:15"}},
		Selector:             map[string]string{"app": "db"},
		Labels:               map[string]string{"app": "db"},
		CreationTimestamp:    1700000000000,
		VolumeClaimTemplates: []string{"data"},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "StatefulSetInfo", orig, got)
}

func TestDaemonSetInfo_RoundTrip(t *testing.T) {
	orig := DaemonSetInfo{
		Name:                   "node-exporter",
		Namespace:              "monitoring",
		DesiredNumberScheduled: 5,
		CurrentNumberScheduled: 5,
		NumberReady:            5,
		NumberMisscheduled:     0,
		UpdatedNumberScheduled: 5,
		Strategy:               "RollingUpdate",
		TotalCPURequest:        0.5,
		TotalMemoryRequest:     536870912,
		ContainerSpecs:         []ContainerSpecInfo{{Name: "exporter", Image: "prom/node-exporter:v1.6.1"}},
		Selector:               map[string]string{"app": "node-exporter"},
		Labels:                 map[string]string{"app": "node-exporter"},
		CreationTimestamp:      1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "DaemonSetInfo", orig, got)
}

func TestReplicaSetInfo_RoundTrip(t *testing.T) {
	orig := ReplicaSetInfo{
		Name:              "api-server-7b9f4c6d8",
		Namespace:         "production",
		Replicas:          3,
		ReadyReplicas:     3,
		OwnerKind:         "Deployment",
		OwnerName:         "api-server",
		OwnerUID:          "uid-deploy-123",
		Selector:          map[string]string{"app": "api-server", "pod-template-hash": "7b9f4c6d8"},
		Labels:            map[string]string{"app": "api-server"},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "ReplicaSetInfo", orig, got)
}

// --- Job types ---

func TestJobInfo_RoundTrip(t *testing.T) {
	completions := int32(1)
	parallelism := int32(1)
	backoffLimit := int32(6)
	startTime := int64(1700000000000)
	completionTime := int64(1700000060000)
	duration := 60.0
	cpuUsage := 0.5
	memUsage := int64(268435456)

	orig := JobInfo{
		Name:               "migrate-db",
		Namespace:          "production",
		OwnerCronJob:       "",
		Completions:        &completions,
		Parallelism:        &parallelism,
		BackoffLimit:       &backoffLimit,
		Active:             0,
		Succeeded:          1,
		Failed:             0,
		StartTime:          &startTime,
		CompletionTime:     &completionTime,
		DurationSeconds:    &duration,
		TotalCPURequest:    0.5,
		TotalMemoryRequest: 268435456,
		TotalCPUUsage:      &cpuUsage,
		TotalMemoryUsage:   &memUsage,
		Labels:             map[string]string{"job-name": "migrate-db"},
		CreationTimestamp:  1700000000000,
		Conditions: []JobConditionInfo{{
			Type:   "Complete",
			Status: "True",
			Reason: "BackoffLimitExceeded",
		}},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "JobInfo", orig, got)
}

func TestCronJobInfo_RoundTrip(t *testing.T) {
	lastSchedule := int64(1700000000000)

	orig := CronJobInfo{
		Name:              "backup",
		Namespace:         "production",
		Schedule:          "0 2 * * *",
		Suspend:           false,
		ConcurrencyPolicy: "Forbid",
		LastScheduleTime:  &lastSchedule,
		ActiveJobs:        []string{"backup-28350000"},
		ContainerSpecs:    []ContainerSpecInfo{{Name: "backup", Image: "backup:v1"}},
		Labels:            map[string]string{"app": "backup"},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "CronJobInfo", orig, got)
}

// --- Autoscaling types ---

func TestHPAInfo_RoundTrip(t *testing.T) {
	minReplicas := int32(2)
	lastScale := int64(1700000000000)
	stabilization := int32(300)
	currentUtil := int32(75)

	orig := HPAInfo{
		Name:             "api-hpa",
		Namespace:        "production",
		TargetKind:       "Deployment",
		TargetName:       "api-server",
		TargetAPIVersion: "apps/v1",
		MinReplicas:      &minReplicas,
		MaxReplicas:      10,
		CurrentReplicas:  3,
		DesiredReplicas:  4,
		Metrics: []HPAMetricInfo{{
			Type:         "Resource",
			ResourceName: "cpu",
			TargetType:   "Utilization",
			TargetValue:  "80",
		}},
		CurrentMetrics: []HPACurrentMetricInfo{{
			Type:               "Resource",
			ResourceName:       "cpu",
			CurrentUtilization: &currentUtil,
		}},
		ScaleUpBehavior: &HPAScalingBehavior{
			StabilizationWindowSeconds: &stabilization,
			Policies: []HPAScalingPolicy{{
				Type:          "Pods",
				Value:         4,
				PeriodSeconds: 60,
			}},
			SelectPolicy: "Max",
		},
		Conditions: []HPAConditionInfo{{
			Type:   "ScalingActive",
			Status: "True",
			Reason: "ValidMetricFound",
		}},
		Labels:            map[string]string{},
		Annotations:       map[string]string{},
		CreationTimestamp: 1700000000000,
		LastScaleTime:     &lastScale,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "HPAInfo", orig, got)
}

func TestVPAInfo_RoundTrip(t *testing.T) {
	orig := VPAInfo{
		Name:             "api-vpa",
		Namespace:        "production",
		TargetKind:       "Deployment",
		TargetName:       "api-server",
		TargetAPIVersion: "apps/v1",
		UpdateMode:       "Auto",
		ContainerRecommendations: []VPAContainerRecommendation{{
			ContainerName:  "api",
			LowerBound:     ResourceValues{CPUCores: 0.1, MemoryBytes: 67108864},
			Target:         ResourceValues{CPUCores: 0.5, MemoryBytes: 268435456},
			UncappedTarget: ResourceValues{CPUCores: 0.5, MemoryBytes: 268435456},
			UpperBound:     ResourceValues{CPUCores: 2.0, MemoryBytes: 1073741824},
		}},
		Conditions: []VPAConditionInfo{{
			Type:   "RecommendationProvided",
			Status: "True",
		}},
		Labels:            map[string]string{},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "VPAInfo", orig, got)
}

func TestPDBInfo_RoundTrip(t *testing.T) {
	orig := PDBInfo{
		Name:        "api-pdb",
		Namespace:   "production",
		MatchLabels: map[string]string{"app": "api-server"},
		MatchExpressions: []LabelSelectorRequirement{{
			Key:      "tier",
			Operator: "In",
			Values:   []string{"frontend", "backend"},
		}},
		TargetWorkloads: []WorkloadReference{{
			Kind: "Deployment", Name: "api-server", Namespace: "production",
		}},
		MinAvailable:       "2",
		MaxUnavailable:     "",
		CurrentHealthy:     3,
		DesiredHealthy:     2,
		DisruptionsAllowed: 1,
		ExpectedPods:       3,
		Conditions: []PDBConditionInfo{{
			Type: "SufficientPods", Status: "True",
		}},
		Labels:            map[string]string{},
		Annotations:       map[string]string{},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "PDBInfo", orig, got)
}

// --- Network types ---

func TestServiceInfo_RoundTrip(t *testing.T) {
	orig := ServiceInfo{
		Name:        "api-svc",
		Namespace:   "production",
		Type:        "LoadBalancer",
		ClusterIP:   "10.100.0.1",
		ClusterIPs:  []string{"10.100.0.1"},
		ExternalIPs: []string{},
		LoadBalancer: &LoadBalancerInfo{
			Ingress: []LoadBalancerIngress{{
				Hostname: "a1b2c3-1234567890.us-east-1.elb.amazonaws.com",
			}},
			Class:               "service.k8s.aws/nlb",
			AWSLoadBalancerType: "nlb",
			AWSScheme:           "internet-facing",
		},
		Ports: []ServicePortInfo{{
			Name:       "https",
			Protocol:   "TCP",
			Port:       443,
			TargetPort: "8443",
			NodePort:   30443,
		}},
		Selector:          map[string]string{"app": "api-server"},
		TargetWorkloads:   []WorkloadReference{{Kind: "Deployment", Name: "api-server", Namespace: "production"}},
		Labels:            map[string]string{},
		Annotations:       map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"},
		CreationTimestamp: 1700000000000,
		SessionAffinity:   "None",
	}

	got := roundTrip(t, orig)
	assertEqual(t, "ServiceInfo", orig, got)
}

func TestIngressInfo_RoundTrip(t *testing.T) {
	orig := IngressInfo{
		Name:             "api-ingress",
		Namespace:        "production",
		IngressClassName: "alb",
		Rules: []IngressRuleInfo{{
			Host: "api.example.com",
			Paths: []IngressPathInfo{{
				Path:           "/",
				PathType:       "Prefix",
				BackendService: "api-svc",
				BackendPort:    "https",
			}},
		}},
		TLS: []IngressTLSInfo{{
			Hosts:      []string{"api.example.com"},
			SecretName: "api-tls",
		}},
		DefaultBackend: &IngressBackendInfo{
			ServiceName: "default-backend",
			ServicePort: "80",
		},
		Labels:                map[string]string{},
		Annotations:           map[string]string{},
		CreationTimestamp:     1700000000000,
		LoadBalancerHostnames: []string{"a1b2c3.us-east-1.elb.amazonaws.com"},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "IngressInfo", orig, got)
}

// --- Storage types ---

func TestPVInfo_RoundTrip(t *testing.T) {
	orig := PVInfo{
		Name:             "pv-ebs-001",
		Capacity:         107374182400,
		AccessModes:      []string{"ReadWriteOnce"},
		ReclaimPolicy:    "Delete",
		StorageClassName: "gp3",
		VolumeMode:       "Filesystem",
		Phase:            "Bound",
		MountOptions:     []string{},
		Source: PVSourceInfo{
			Type:            "csi",
			CSIDriver:       "ebs.csi.aws.com",
			CSIVolumeHandle: "vol-abc123",
			CSIFSType:       "ext4",
		},
		ClaimRef: &PVClaimRefInfo{
			Namespace: "production",
			Name:      "data-db-0",
			UID:       "uid-pvc-123",
		},
		Labels:            map[string]string{},
		Annotations:       map[string]string{},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "PVInfo", orig, got)
}

func TestPVCInfo_RoundTrip(t *testing.T) {
	orig := PVCInfo{
		Name:              "data-db-0",
		Namespace:         "production",
		Phase:             "Bound",
		AccessModes:       []string{"ReadWriteOnce"},
		StorageClassName:  "gp3",
		VolumeMode:        "Filesystem",
		VolumeName:        "pv-ebs-001",
		RequestedBytes:    107374182400,
		CapacityBytes:     107374182400,
		MountedByPods:     []string{"production/db-0"},
		Labels:            map[string]string{},
		Annotations:       map[string]string{},
		CreationTimestamp: 1700000000000,
		Conditions: []PVCConditionInfo{{
			Type: "Resizing", Status: "True",
		}},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "PVCInfo", orig, got)
}

func TestStorageClassInfo_RoundTrip(t *testing.T) {
	orig := StorageClassInfo{
		Name:                 "gp3",
		Provisioner:          "ebs.csi.aws.com",
		ReclaimPolicy:        "Delete",
		VolumeBindingMode:    "WaitForFirstConsumer",
		AllowVolumeExpansion: true,
		Parameters:           map[string]string{"type": "gp3", "encrypted": "true"},
		MountOptions:         []string{},
		Labels:               map[string]string{},
		Annotations:          map[string]string{},
		IsDefault:            true,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "StorageClassInfo", orig, got)
}

// --- Scheduling types ---

func TestNamespaceInfo_RoundTrip(t *testing.T) {
	orig := NamespaceInfo{
		Name:              "production",
		Phase:             "Active",
		Labels:            map[string]string{"env": "production"},
		Annotations:       map[string]string{},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "NamespaceInfo", orig, got)
}

func TestPriorityClassInfo_RoundTrip(t *testing.T) {
	orig := PriorityClassInfo{
		Name:             "high-priority",
		Value:            1000000,
		GlobalDefault:    false,
		PreemptionPolicy: "PreemptLowerPriority",
		Description:      "For critical workloads",
	}

	got := roundTrip(t, orig)
	assertEqual(t, "PriorityClassInfo", orig, got)
}

func TestLimitRangeInfo_RoundTrip(t *testing.T) {
	orig := LimitRangeInfo{
		Name:      "default-limits",
		Namespace: "production",
		Limits: []LimitRangeItemInfo{{
			Type:           "Container",
			Default:        map[string]string{"cpu": "500m", "memory": "256Mi"},
			DefaultRequest: map[string]string{"cpu": "100m", "memory": "128Mi"},
			Max:            map[string]string{"cpu": "4", "memory": "8Gi"},
			Min:            map[string]string{"cpu": "50m", "memory": "64Mi"},
		}},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "LimitRangeInfo", orig, got)
}

func TestResourceQuotaInfo_RoundTrip(t *testing.T) {
	orig := ResourceQuotaInfo{
		Name:      "compute-quota",
		Namespace: "production",
		Hard:      map[string]string{"requests.cpu": "10", "limits.memory": "50Gi", "pods": "100"},
		Used:      map[string]string{"requests.cpu": "4.5", "limits.memory": "20Gi", "pods": "42"},
		Labels:    map[string]string{},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "ResourceQuotaInfo", orig, got)
}

// --- Custom types ---

func TestCustomWorkloadInfo_RoundTrip(t *testing.T) {
	replicas := int32(3)
	readyReplicas := int32(2)
	cpuUsage := 1.5
	memUsage := int64(1073741824)

	orig := CustomWorkloadInfo{
		APIVersion:         "kafka.strimzi.io/v1beta2",
		Kind:               "KafkaConnect",
		Name:               "my-connect",
		Namespace:          "kafka",
		Replicas:           &replicas,
		ReadyReplicas:      &readyReplicas,
		Status:             map[string]interface{}{"observedGeneration": float64(5)},
		PodCount:           3,
		TotalCPURequest:    1.5,
		TotalMemoryRequest: 3221225472,
		TotalCPUUsage:      &cpuUsage,
		TotalMemoryUsage:   &memUsage,
		Labels:             map[string]string{"strimzi.io/cluster": "my-kafka"},
		Annotations:        map[string]string{},
		CreationTimestamp:  1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "CustomWorkloadInfo", orig, got)
}

func TestNodePoolInfo_RoundTrip(t *testing.T) {
	min := 1
	max := 100

	orig := NodePoolInfo{
		Name:          "default",
		MinReplicas:   &min,
		MaxReplicas:   &max,
		NodeClassName: "default",
		Labels:        map[string]string{"karpenter.sh/nodepool": "default"},
		Annotations:   map[string]string{},
		Taints:        []TaintInfo{{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"}},
		Requirements: []NodeSelectorRequirement{{
			Key:      "karpenter.sh/capacity-type",
			Operator: "In",
			Values:   []string{"on-demand", "spot"},
		}},
		CreationTimestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "NodePoolInfo", orig, got)
}

// --- Response types ---

func TestSnapshotResponse_RoundTrip(t *testing.T) {
	retryAfter := 120
	gracePeriodEnds := int64(1700086400000)

	orig := SnapshotResponse{
		Success:     true,
		Message:     "Snapshot ingested successfully",
		ClusterID:   "cls-abc",
		ReceivedAt:  1700000000000,
		ProcessedAt: 1700000001000,
		Quota: QuotaStatus{
			PlanType:          "business",
			CPULimit:          100.0,
			CurrentCPUUsage:   45.5,
			RemainingCPU:      54.5,
			IsWithinQuota:     true,
			ClusterCPU:        12.5,
			GracePeriodActive: true,
			GracePeriodEndsAt: &gracePeriodEnds,
		},
		Directives: Directives{
			NextSnapshotInSeconds: 60,
			RetryAfterSeconds:     &retryAfter,
			CollectVPAs:           true,
			CollectKarpenter:      false,
		},
		Stats: IngestStats{
			NodesProcessed:     5,
			PodsProcessed:      150,
			WorkloadsProcessed: 25,
			ProcessingTimeMs:   42,
		},
	}

	got := roundTrip(t, orig)
	assertEqual(t, "SnapshotResponse", orig, got)
}

func TestSnapshotErrorResponse_RoundTrip(t *testing.T) {
	retryAfter := 300
	orig := SnapshotErrorResponse{
		Success:           false,
		Error:             "QUOTA_EXCEEDED",
		Message:           "Organization CPU quota exceeded",
		Quota:             &QuotaStatus{PlanType: "free", IsWithinQuota: false},
		RetryAfterSeconds: &retryAfter,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "SnapshotErrorResponse", orig, got)
}

// --- Metrics types ---

func TestNodeMetrics_RoundTrip(t *testing.T) {
	orig := NodeMetrics{
		Name:             "node-1",
		CPUUsageCores:    2.5,
		MemoryUsageBytes: 8589934592,
		Timestamp:        1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "NodeMetrics", orig, got)
}

func TestPodMetrics_RoundTrip(t *testing.T) {
	orig := PodMetrics{
		Name:      "api-pod-1",
		Namespace: "default",
		Containers: []ContainerMetrics{{
			Name:             "api",
			CPUUsageCores:    0.25,
			MemoryUsageBytes: 134217728,
		}},
		Timestamp: 1700000000000,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "PodMetrics", orig, got)
}

// --- AgentHealth omitempty ---

func TestAgentHealth_OmitEmpty(t *testing.T) {
	h := AgentHealth{
		State:         "running",
		UptimeSeconds: 3600,
		StartedAt:     1700000000000,
		CollectedAt:   1700000000000,
	}

	data, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}

	// StateReason should be omitted when empty
	assertJSONFieldAbsent(t, data, "state_reason")
	// ErrorCodes should be omitted when nil
	assertJSONFieldAbsent(t, data, "error_codes")
	// QuotaPlanType should be omitted when empty
	assertJSONFieldAbsent(t, data, "quota_plan_type")
}

// --- GPU types ---

func TestGPUDeviceInfo_RoundTrip(t *testing.T) {
	util := 75.5
	memUsed := int64(4 * 1024 * 1024 * 1024)
	memTotal := int64(80 * 1024 * 1024 * 1024)
	temp := 62.0
	power := 250.0

	orig := GPUDeviceInfo{
		UUID:               "GPU-abc-123",
		DeviceIndex:        "0",
		ModelName:          "NVIDIA A100-SXM4-80GB",
		UtilizationPercent: &util,
		MemoryUsedBytes:    &memUsed,
		MemoryTotalBytes:   &memTotal,
		TemperatureCelsius: &temp,
		PowerWatts:         &power,
		MIGProfile:         "1g.5gb",
		MIGInstanceID:      "3",
	}

	got := roundTrip(t, orig)
	assertEqual(t, "GPUDeviceInfo", orig, got)
}

func TestGPUDeviceInfo_OmitEmpty(t *testing.T) {
	info := GPUDeviceInfo{UUID: "GPU-xyz", DeviceIndex: "0"}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONFieldAbsent(t, data, "model_name")
	assertJSONFieldAbsent(t, data, "utilization_percent")
	assertJSONFieldAbsent(t, data, "memory_used_bytes")
	assertJSONFieldAbsent(t, data, "memory_total_bytes")
	assertJSONFieldAbsent(t, data, "temperature_celsius")
	assertJSONFieldAbsent(t, data, "power_watts")
	assertJSONFieldAbsent(t, data, "mig_profile")
	assertJSONFieldAbsent(t, data, "mig_instance_id")
}

func TestNodeInfo_NilGPUMetrics(t *testing.T) {
	node := NodeInfo{Name: "test-node"}
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONFieldAbsent(t, data, "gpu_model")
	assertJSONFieldAbsent(t, data, "gpu_utilization_percent")
	assertJSONFieldAbsent(t, data, "gpu_memory_used_bytes")
	assertJSONFieldAbsent(t, data, "gpu_memory_total_bytes")
	assertJSONFieldAbsent(t, data, "gpu_temperature_celsius")
	assertJSONFieldAbsent(t, data, "gpu_power_watts")
	assertJSONFieldAbsent(t, data, "gpu_devices")
	assertJSONFieldAbsent(t, data, "mig_devices")
}

func TestContainerInfo_NilGPUMetrics(t *testing.T) {
	c := ContainerInfo{Name: "app"}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONFieldAbsent(t, data, "gpu_utilization_percent")
	assertJSONFieldAbsent(t, data, "gpu_memory_used_bytes")
}

func TestClusterSummary_GPUOmitEmpty(t *testing.T) {
	s := ClusterSummary{NodeCount: 1}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONFieldAbsent(t, data, "total_gpu_usage")
	assertJSONFieldAbsent(t, data, "total_gpu_memory_used")
	assertJSONFieldAbsent(t, data, "total_gpu_memory_total")
	assertJSONFieldPresent(t, data, "gpu_metrics_available")
}

// --- GPU integration tests ---

func TestNodeInfo_GPUFields_RoundTrip(t *testing.T) {
	gpuUtil := 78.5
	gpuMemUsed := int64(40 * 1024 * 1024 * 1024)
	gpuMemTotal := int64(80 * 1024 * 1024 * 1024)
	gpuTemp := 67.0
	gpuPower := 275.5

	orig := NodeInfo{
		Name:                  "gpu-node-1",
		UID:                   "uid-gpu-1",
		InstanceType:          "p4d.24xlarge",
		CPUCapacityCores:      96.0,
		GPUCapacity:           8,
		GPUAllocatable:        8,
		GPUModel:              "NVIDIA A100-SXM4-80GB",
		GPUDriverVersion:      "535.104.12",
		MIGEnabled:            true,
		MIGDevices:            map[string]int{"1g.10gb": 4, "2g.20gb": 2},
		GPUUtilizationPercent: &gpuUtil,
		GPUMemoryUsedBytes:    &gpuMemUsed,
		GPUMemoryTotalBytes:   &gpuMemTotal,
		GPUTemperatureCelsius: &gpuTemp,
		GPUPowerWatts:         &gpuPower,
		GPUDevices: []GPUDeviceInfo{
			{
				UUID:               "GPU-aaa-bbb",
				DeviceIndex:        "0",
				ModelName:          "NVIDIA A100-SXM4-80GB",
				UtilizationPercent: &gpuUtil,
				MemoryUsedBytes:    &gpuMemUsed,
				MemoryTotalBytes:   &gpuMemTotal,
				TemperatureCelsius: &gpuTemp,
				PowerWatts:         &gpuPower,
			},
		},
		Ready: true,
	}

	got := roundTrip(t, orig)
	assertEqual(t, "NodeInfo_GPU", orig, got)
}

func TestNodeInfo_GPUFields_Omitempty(t *testing.T) {
	node := NodeInfo{
		Name:             "cpu-only-node",
		CPUCapacityCores: 4.0,
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}

	assertJSONFieldAbsent(t, data, "gpu_model")
	assertJSONFieldAbsent(t, data, "gpu_driver_version")
	assertJSONFieldAbsent(t, data, "mig_enabled")
	assertJSONFieldAbsent(t, data, "mig_devices")
	assertJSONFieldAbsent(t, data, "gpu_utilization_percent")
	assertJSONFieldAbsent(t, data, "gpu_memory_used_bytes")
	assertJSONFieldAbsent(t, data, "gpu_memory_total_bytes")
	assertJSONFieldAbsent(t, data, "gpu_temperature_celsius")
	assertJSONFieldAbsent(t, data, "gpu_power_watts")
	assertJSONFieldAbsent(t, data, "gpu_devices")
}

func TestClusterSnapshot_BackwardCompat_NoGPU(t *testing.T) {
	oldJSON := `{
		"snapshot_id": "snap-old-001",
		"cluster_id": "cls-legacy",
		"cluster_name": "legacy-cluster",
		"timestamp": 1700000000000,
		"agent_version": "0.9.0",
		"provider": "aws",
		"region": "us-east-1",
		"kubernetes_version": "1.27.0",
		"nodes": [{
			"name": "node-1",
			"uid": "uid-1",
			"cpu_capacity_cores": 4.0,
			"memory_capacity_bytes": 17179869184,
			"ready": true
		}],
		"pods": [],
		"namespaces": [],
		"deployments": [],
		"statefulsets": [],
		"daemonsets": [],
		"jobs": [],
		"cronjobs": [],
		"custom_workloads": [],
		"hpas": [],
		"pdbs": [],
		"services": [],
		"ingresses": [],
		"pvs": [],
		"pvcs": [],
		"storage_classes": [],
		"priority_classes": [],
		"limit_ranges": [],
		"resource_quotas": [],
		"summary": {
			"node_count": 1,
			"pod_count": 0,
			"total_cpu_capacity": 4.0,
			"total_memory_capacity": 17179869184
		},
		"health": {
			"state": "running",
			"uptime_seconds": 3600,
			"started_at": 1700000000000,
			"collected_at": 1700000000000
		}
	}`

	var snap ClusterSnapshot
	if err := json.Unmarshal([]byte(oldJSON), &snap); err != nil {
		t.Fatalf("unmarshal old-format snapshot: %v", err)
	}

	if snap.Nodes[0].GPUModel != "" {
		t.Errorf("GPUModel = %q, want empty", snap.Nodes[0].GPUModel)
	}
	if snap.Nodes[0].GPUUtilizationPercent != nil {
		t.Error("GPUUtilizationPercent should be nil for old snapshot")
	}
	if snap.Nodes[0].GPUMemoryUsedBytes != nil {
		t.Error("GPUMemoryUsedBytes should be nil for old snapshot")
	}
	if snap.Nodes[0].GPUMemoryTotalBytes != nil {
		t.Error("GPUMemoryTotalBytes should be nil for old snapshot")
	}
	if snap.Nodes[0].GPUDevices != nil {
		t.Error("GPUDevices should be nil for old snapshot")
	}
	if snap.Summary.TotalGPUUsage != nil {
		t.Error("TotalGPUUsage should be nil for old snapshot")
	}
	if snap.Summary.TotalGPUMemoryUsed != nil {
		t.Error("TotalGPUMemoryUsed should be nil for old snapshot")
	}
	if snap.Summary.GPUMetricsAvailable {
		t.Error("GPUMetricsAvailable should be false for old snapshot")
	}
}

// --- Helper ---

func intPtr(v int) *int {
	return &v
}
