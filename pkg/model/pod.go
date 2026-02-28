package model

// PodInfo represents a Kubernetes pod with ownership, containers, and scheduling info.
type PodInfo struct {
	Name      string `json:"name"`
	UID       string `json:"uid"`
	Namespace string `json:"namespace"`
	NodeName  string `json:"node_name"`
	Phase     string `json:"phase"`
	Reason    string `json:"reason"`
	QoSClass  string `json:"qos_class"`

	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
	OwnerUID  string `json:"owner_uid"`

	Containers     []ContainerInfo `json:"containers"`
	InitContainers []ContainerInfo `json:"init_containers"`

	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`

	PriorityClassName  string `json:"priority_class_name"`
	Priority           *int32 `json:"priority,omitempty"`
	SchedulerName      string `json:"scheduler_name"`
	ServiceAccountName string `json:"service_account_name"`

	PodIP       string `json:"pod_ip"`
	HostIP      string `json:"host_ip"`
	HostNetwork bool   `json:"host_network"`
	HasHostPath bool   `json:"has_hostpath"`
	HasEmptyDir bool   `json:"has_emptydir"`

	Conditions []PodConditionInfo `json:"conditions"`
}

// ContainerInfo represents a container within a pod including spec, status, and metrics.
type ContainerInfo struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	ImageID string `json:"image_id"`

	CPURequestCores         float64 `json:"cpu_request_cores"`
	MemoryRequestBytes      int64   `json:"memory_request_bytes"`
	CPULimitCores           float64 `json:"cpu_limit_cores"`
	MemoryLimitBytes        int64   `json:"memory_limit_bytes"`
	EphemeralStorageRequest int64   `json:"ephemeral_storage_request"`
	EphemeralStorageLimit   int64   `json:"ephemeral_storage_limit"`
	GPURequest              int     `json:"gpu_request"`
	GPULimit                int     `json:"gpu_limit"`

	CPUUsageCores    *float64 `json:"cpu_usage_cores,omitempty"`
	MemoryUsageBytes *int64   `json:"memory_usage_bytes,omitempty"`

	GPUUtilizationPercent *float64 `json:"gpu_utilization_percent,omitempty"`
	GPUMemoryUsedBytes    *int64   `json:"gpu_memory_used_bytes,omitempty"`

	Ready                 bool   `json:"ready"`
	Started               *bool  `json:"started,omitempty"`
	RestartCount          int32  `json:"restart_count"`
	State                 string `json:"state"`
	StateReason           string `json:"state_reason"`
	StateMessage          string `json:"state_message"`
	LastTerminationReason string `json:"last_termination_reason"`
	ExitCode              *int32 `json:"exit_code,omitempty"`

	Ports []ContainerPortInfo `json:"ports"`
}

// ContainerPortInfo represents a port exposed by a container.
type ContainerPortInfo struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// PodConditionInfo represents a pod condition (Ready, PodScheduled, etc.).
type PodConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// PodMetrics represents metrics-server data for a pod.
type PodMetrics struct {
	Name       string             `json:"name"`
	Namespace  string             `json:"namespace"`
	Containers []ContainerMetrics `json:"containers"`
	Timestamp  int64              `json:"timestamp"`
}

// ContainerMetrics represents metrics-server data for a single container.
type ContainerMetrics struct {
	Name             string  `json:"name"`
	CPUUsageCores    float64 `json:"cpu_usage_cores"`
	MemoryUsageBytes int64   `json:"memory_usage_bytes"`
}
