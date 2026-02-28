package model

// DeploymentInfo represents a Kubernetes Deployment.
type DeploymentInfo struct {
	Name                string `json:"name"`
	UID                 string `json:"uid"`
	Namespace           string `json:"namespace"`
	Replicas            int32  `json:"replicas"`
	ReadyReplicas       int32  `json:"ready_replicas"`
	AvailableReplicas   int32  `json:"available_replicas"`
	UnavailableReplicas int32  `json:"unavailable_replicas"`
	UpdatedReplicas     int32  `json:"updated_replicas"`
	Strategy            string `json:"strategy"`
	MaxSurge            string `json:"max_surge"`
	MaxUnavailable      string `json:"max_unavailable"`

	TotalCPURequest    float64  `json:"total_cpu_request"`
	TotalMemoryRequest int64    `json:"total_memory_request"`
	TotalCPULimit      float64  `json:"total_cpu_limit"`
	TotalMemoryLimit   int64    `json:"total_memory_limit"`
	TotalCPUUsage      *float64 `json:"total_cpu_usage,omitempty"`
	TotalMemoryUsage   *int64   `json:"total_memory_usage,omitempty"`

	ContainerSpecs []ContainerSpecInfo `json:"container_specs"`

	Selector          map[string]string `json:"selector"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`

	Conditions []WorkloadConditionInfo `json:"conditions"`
	Paused     bool                    `json:"paused"`
}

// StatefulSetInfo represents a Kubernetes StatefulSet.
type StatefulSetInfo struct {
	Name                 string   `json:"name"`
	UID                  string   `json:"uid"`
	Namespace            string   `json:"namespace"`
	Replicas             int32    `json:"replicas"`
	ReadyReplicas        int32    `json:"ready_replicas"`
	AvailableReplicas    int32    `json:"available_replicas"`
	UpdatedReplicas      int32    `json:"updated_replicas"`
	Strategy             string   `json:"strategy"`
	ServiceName          string   `json:"service_name"`
	PodManagementPolicy  string   `json:"pod_management_policy"`
	VolumeClaimTemplates []string `json:"volume_claim_templates"`

	TotalCPURequest    float64  `json:"total_cpu_request"`
	TotalMemoryRequest int64    `json:"total_memory_request"`
	TotalCPULimit      float64  `json:"total_cpu_limit"`
	TotalMemoryLimit   int64    `json:"total_memory_limit"`
	TotalCPUUsage      *float64 `json:"total_cpu_usage,omitempty"`
	TotalMemoryUsage   *int64   `json:"total_memory_usage,omitempty"`

	ContainerSpecs []ContainerSpecInfo `json:"container_specs"`

	Selector          map[string]string `json:"selector"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`

	Conditions []WorkloadConditionInfo `json:"conditions"`
}

// DaemonSetInfo represents a Kubernetes DaemonSet.
type DaemonSetInfo struct {
	Name                   string `json:"name"`
	UID                    string `json:"uid"`
	Namespace              string `json:"namespace"`
	DesiredNumberScheduled int32  `json:"desired_number_scheduled"`
	CurrentNumberScheduled int32  `json:"current_number_scheduled"`
	NumberReady            int32  `json:"number_ready"`
	NumberMisscheduled     int32  `json:"number_misscheduled"`
	UpdatedNumberScheduled int32  `json:"updated_number_scheduled"`
	Strategy               string `json:"strategy"`

	TotalCPURequest    float64  `json:"total_cpu_request"`
	TotalMemoryRequest int64    `json:"total_memory_request"`
	TotalCPULimit      float64  `json:"total_cpu_limit"`
	TotalMemoryLimit   int64    `json:"total_memory_limit"`
	TotalCPUUsage      *float64 `json:"total_cpu_usage,omitempty"`
	TotalMemoryUsage   *int64   `json:"total_memory_usage,omitempty"`

	ContainerSpecs []ContainerSpecInfo `json:"container_specs"`

	Selector          map[string]string `json:"selector"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`

	Conditions []WorkloadConditionInfo `json:"conditions"`
}

// ReplicaSetInfo represents a Kubernetes ReplicaSet (used internally for ownership resolution).
type ReplicaSetInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Replicas          int32             `json:"replicas"`
	ReadyReplicas     int32             `json:"ready_replicas"`
	OwnerKind         string            `json:"owner_kind"`
	OwnerName         string            `json:"owner_name"`
	OwnerUID          string            `json:"owner_uid"`
	Selector          map[string]string `json:"selector"`
	Labels            map[string]string `json:"labels"`
	CreationTimestamp int64             `json:"creation_timestamp"`
}

// ContainerSpecInfo represents a container spec from a workload's pod template.
type ContainerSpecInfo struct {
	Name               string  `json:"name"`
	Image              string  `json:"image"`
	CPURequestCores    float64 `json:"cpu_request_cores"`
	MemoryRequestBytes int64   `json:"memory_request_bytes"`
	CPULimitCores      float64 `json:"cpu_limit_cores"`
	MemoryLimitBytes   int64   `json:"memory_limit_bytes"`
	GPURequest         int     `json:"gpu_request"`
	GPULimit           int     `json:"gpu_limit"`
}

// WorkloadConditionInfo represents a workload condition (Available, Progressing, etc.).
type WorkloadConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}
