// Package gpu implements a collector for NVIDIA GPU metrics from dcgm-exporter.
//
// It scrapes Prometheus-format metrics endpoints exposed by dcgm-exporter pods
// and parses DCGM GPU metrics including utilization, memory, temperature, and power.
//
// The collector handles both old-style (pod_name, pod_namespace, container_name)
// and new-style (pod, namespace, container) dcgm-exporter label schemas, normalizing
// them into a consistent set of fields. DCGM sentinel values (~1.8e19) are detected
// and rejected to prevent data corruption.
//
// For GPUs that support profiling metrics (Volta+), DCGM_FI_PROF_GR_ENGINE_ACTIVE
// is preferred over DCGM_FI_DEV_GPU_UTIL for utilization reporting.
package gpu
