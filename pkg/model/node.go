package model

// NodeInfo represents a Kubernetes node with capacity, allocatable resources,
// usage metrics, and cloud provider metadata.
type NodeInfo struct {
	Name             string `json:"name"`
	UID              string `json:"uid"`
	ProviderID       string `json:"provider_id"`
	InstanceID       string `json:"instance_id"`
	InstanceType     string `json:"instance_type"`
	Region           string `json:"region"`
	Zone             string `json:"zone"`
	CapacityType     string `json:"capacity_type"`
	NodeGroup        string `json:"node_group"`
	Architecture     string `json:"architecture"`
	OS               string `json:"os"`
	KubeletVersion   string `json:"kubelet_version"`
	ContainerRuntime string `json:"container_runtime"`

	PodCIDR          string   `json:"pod_cidr"`
	PodCIDRs         []string `json:"pod_cidrs"`
	OSImage          string   `json:"os_image"`
	KernelVersion    string   `json:"kernel_version"`
	KubeProxyVersion string   `json:"kube_proxy_version"`

	CPUCapacityCores            float64 `json:"cpu_capacity_cores"`
	MemoryCapacityBytes         int64   `json:"memory_capacity_bytes"`
	EphemeralStorageBytes       int64   `json:"ephemeral_storage_bytes"`
	EphemeralStorageAllocatable int64   `json:"ephemeral_storage_allocatable"`
	PodCapacity                 int     `json:"pod_capacity"`
	GPUCapacity                 int     `json:"gpu_capacity"`

	CPUAllocatable    float64 `json:"cpu_allocatable"`
	MemoryAllocatable int64   `json:"memory_allocatable"`
	PodAllocatable    int     `json:"pod_allocatable"`
	GPUAllocatable    int     `json:"gpu_allocatable"`

	GPUModel               string          `json:"gpu_model,omitempty"`
	GPUDriverVersion       string          `json:"gpu_driver_version,omitempty"`
	MIGEnabled             bool            `json:"mig_enabled,omitempty"`
	MIGDevices             map[string]int  `json:"mig_devices,omitempty"`
	GPUUtilizationPercent  *float64        `json:"gpu_utilization_percent,omitempty"`
	GPUTensorActivePercent *float64        `json:"gpu_tensor_active_percent,omitempty"`
	GPUMemoryUtilPercent   *float64        `json:"gpu_memory_util_percent,omitempty"`
	GPUMemoryUsedBytes     *int64          `json:"gpu_memory_used_bytes,omitempty"`
	GPUMemoryTotalBytes    *int64          `json:"gpu_memory_total_bytes,omitempty"`
	GPUTemperatureCelsius  *float64        `json:"gpu_temperature_celsius,omitempty"`
	GPUPowerWatts          *float64        `json:"gpu_power_watts,omitempty"`
	GPUDevices             []GPUDeviceInfo `json:"gpu_devices,omitempty"`

	CPUUsageCores    *float64 `json:"cpu_usage_cores,omitempty"`
	MemoryUsageBytes *int64   `json:"memory_usage_bytes,omitempty"`

	Ready         bool                `json:"ready"`
	Unschedulable bool                `json:"unschedulable"`
	Taints        []TaintInfo         `json:"taints"`
	Conditions    []NodeConditionInfo `json:"conditions"`

	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`
}

// TaintInfo represents a Kubernetes node taint.
type TaintInfo struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

// NodeConditionInfo represents a node condition (Ready, MemoryPressure, etc.).
type NodeConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// NodeMetrics represents metrics-server data for a node.
type NodeMetrics struct {
	Name             string  `json:"name"`
	CPUUsageCores    float64 `json:"cpu_usage_cores"`
	MemoryUsageBytes int64   `json:"memory_usage_bytes"`
	Timestamp        int64   `json:"timestamp"`
}
