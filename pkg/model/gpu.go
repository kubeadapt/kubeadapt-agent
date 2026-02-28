package model

// GPUDeviceInfo holds per-device GPU metrics collected from dcgm-exporter.
type GPUDeviceInfo struct {
	UUID                string   `json:"uuid"`
	DeviceIndex         string   `json:"device_index"`
	ModelName           string   `json:"model_name,omitempty"`
	UtilizationPercent  *float64 `json:"utilization_percent,omitempty"`
	TensorActivePercent *float64 `json:"tensor_active_percent,omitempty"`
	MemoryUtilPercent   *float64 `json:"memory_util_percent,omitempty"`
	MemoryUsedBytes     *int64   `json:"memory_used_bytes,omitempty"`
	MemoryTotalBytes    *int64   `json:"memory_total_bytes,omitempty"`
	TemperatureCelsius  *float64 `json:"temperature_celsius,omitempty"`
	PowerWatts          *float64 `json:"power_watts,omitempty"`
	MIGProfile          string   `json:"mig_profile,omitempty"`
	MIGInstanceID       string   `json:"mig_instance_id,omitempty"`
}
