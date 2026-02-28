package gpu

// GPUDeviceMetrics represents metrics for a single GPU device scraped from dcgm-exporter.
type GPUDeviceMetrics struct {
	GPU           string `json:"gpu"`
	UUID          string `json:"uuid"`
	Device        string `json:"device,omitempty"`
	ModelName     string `json:"model_name,omitempty"`
	DriverVersion string `json:"driver_version,omitempty"`
	Hostname      string `json:"hostname,omitempty"`

	PodName       string `json:"pod_name"`
	Namespace     string `json:"namespace"`
	ContainerName string `json:"container_name"`

	MIGEnabled    *bool  `json:"mig_enabled,omitempty"`
	GPUInstanceID string `json:"gpu_instance_id,omitempty"`
	GPUProfile    string `json:"gpu_profile,omitempty"`

	GPUUtilization      *float64 `json:"gpu_utilization,omitempty"`
	TensorActivePercent *float64 `json:"tensor_active_percent,omitempty"`
	MemCopyUtilPercent  *float64 `json:"mem_copy_util_percent,omitempty"`

	MemoryUsedBytes  *int64 `json:"memory_used_bytes,omitempty"`
	MemoryFreeBytes  *int64 `json:"memory_free_bytes,omitempty"`
	MemoryTotalBytes *int64 `json:"memory_total_bytes,omitempty"`

	Temperature *float64 `json:"temperature,omitempty"`
	PowerUsage  *float64 `json:"power_usage,omitempty"`

	Timestamp int64 `json:"timestamp"`
}
