package model

// ClusterSnapshot is the complete payload sent to the backend every 60 seconds.
type ClusterSnapshot struct {
	// Identity
	SnapshotID   string `json:"snapshot_id"`
	ClusterID    string `json:"cluster_id"`
	ClusterName  string `json:"cluster_name"`
	Timestamp    int64  `json:"timestamp"`
	AgentVersion string `json:"agent_version"`

	// Provider
	Provider          string `json:"provider"`
	Region            string `json:"region"`
	KubernetesVersion string `json:"kubernetes_version"`

	// Resources
	Nodes           []NodeInfo           `json:"nodes"`
	Pods            []PodInfo            `json:"pods"`
	Namespaces      []NamespaceInfo      `json:"namespaces"`
	Deployments     []DeploymentInfo     `json:"deployments"`
	StatefulSets    []StatefulSetInfo    `json:"statefulsets"`
	DaemonSets      []DaemonSetInfo      `json:"daemonsets"`
	Jobs            []JobInfo            `json:"jobs"`
	CronJobs        []CronJobInfo        `json:"cronjobs"`
	CustomWorkloads []CustomWorkloadInfo `json:"custom_workloads"`

	// Autoscaling & Disruption
	HPAs []HPAInfo `json:"hpas"`
	VPAs []VPAInfo `json:"vpas,omitempty"`
	PDBs []PDBInfo `json:"pdbs"`

	// Network
	Services  []ServiceInfo `json:"services"`
	Ingresses []IngressInfo `json:"ingresses"`

	// Storage
	PVs            []PVInfo           `json:"pvs"`
	PVCs           []PVCInfo          `json:"pvcs"`
	StorageClasses []StorageClassInfo `json:"storage_classes"`

	// Scheduling
	PriorityClasses []PriorityClassInfo `json:"priority_classes"`
	LimitRanges     []LimitRangeInfo    `json:"limit_ranges"`
	ResourceQuotas  []ResourceQuotaInfo `json:"resource_quotas"`

	// Karpenter (omitted if not present)
	NodePools []NodePoolInfo `json:"node_pools,omitempty"`

	// Computed
	Summary ClusterSummary `json:"summary"`

	// Agent health
	Health AgentHealth `json:"health"`
}

// ClusterSummary holds computed counts and resource totals.
type ClusterSummary struct {
	NodeCount           int `json:"node_count"`
	PodCount            int `json:"pod_count"`
	RunningPodCount     int `json:"running_pod_count"`
	PendingPodCount     int `json:"pending_pod_count"`
	FailedPodCount      int `json:"failed_pod_count"`
	NamespaceCount      int `json:"namespace_count"`
	DeploymentCount     int `json:"deployment_count"`
	StatefulSetCount    int `json:"statefulset_count"`
	DaemonSetCount      int `json:"daemonset_count"`
	JobCount            int `json:"job_count"`
	CronJobCount        int `json:"cronjob_count"`
	CustomWorkloadCount int `json:"custom_workload_count"`
	HPACount            int `json:"hpa_count"`
	ServiceCount        int `json:"service_count"`
	IngressCount        int `json:"ingress_count"`
	PVCount             int `json:"pv_count"`
	PVCCount            int `json:"pvc_count"`
	ContainerCount      int `json:"container_count"`

	TotalCPUCapacity    float64  `json:"total_cpu_capacity"`
	TotalCPUAllocatable float64  `json:"total_cpu_allocatable"`
	TotalCPURequested   float64  `json:"total_cpu_requested"`
	TotalCPUUsage       *float64 `json:"total_cpu_usage"`

	TotalMemoryCapacity    int64  `json:"total_memory_capacity"`
	TotalMemoryAllocatable int64  `json:"total_memory_allocatable"`
	TotalMemoryRequested   int64  `json:"total_memory_requested"`
	TotalMemoryUsage       *int64 `json:"total_memory_usage"`

	TotalGPUCapacity  int `json:"total_gpu_capacity"`
	TotalGPURequested int `json:"total_gpu_requested"`

	TotalGPUUsage        *float64 `json:"total_gpu_usage,omitempty"`
	TotalGPUTensorActive *float64 `json:"total_gpu_tensor_active,omitempty"`
	TotalGPUMemoryUtil   *float64 `json:"total_gpu_memory_util,omitempty"`
	TotalGPUMemoryUsed   *int64   `json:"total_gpu_memory_used,omitempty"`
	TotalGPUMemoryTotal  *int64   `json:"total_gpu_memory_total,omitempty"`
	GPUMetricsAvailable  bool     `json:"gpu_metrics_available"`

	TotalStorageCapacity  int64 `json:"total_storage_capacity"`
	TotalStorageRequested int64 `json:"total_storage_requested"`

	MetricsAvailable bool `json:"metrics_available"`
}

// AgentHealth is sent with every snapshot for ClickHouse storage.
type AgentHealth struct {
	// Cumulative counters
	SnapshotsSentTotal   uint64 `json:"snapshots_sent_total"`
	SnapshotsFailedTotal uint64 `json:"snapshots_failed_total"`
	SnapshotsTotalCount  uint64 `json:"snapshots_total"`

	// Agent state
	State       string `json:"state"`
	StateReason string `json:"state_reason,omitempty"`

	// Snapshot build performance
	LastBuildDurationMs          int64 `json:"last_build_duration_ms"`
	LastMetricsCollectDurationMs int64 `json:"last_metrics_collect_duration_ms"`
	LastSendDurationMs           int64 `json:"last_send_duration_ms"`

	// Payload size
	OriginalSizeBytes   int64   `json:"original_size_bytes"`
	CompressedSizeBytes int64   `json:"compressed_size_bytes"`
	CompressionRatio    float64 `json:"compression_ratio"`

	// Entity counts
	NodeCount      int `json:"node_count"`
	PodCount       int `json:"pod_count"`
	ContainerCount int `json:"container_count"`
	WorkloadCount  int `json:"workload_count"`
	ServiceCount   int `json:"service_count"`
	HPACount       int `json:"hpa_count"`
	PDBCount       int `json:"pdb_count"`
	PVCount        int `json:"pv_count"`

	// Data source status
	MetricsServerAvailable bool `json:"metrics_server_available"`
	VPAAvailable           bool `json:"vpa_available"`
	KarpenterAvailable     bool `json:"karpenter_available"`
	GPUMetricsAvailable    bool `json:"gpu_metrics_available"`
	DCGMExporterTargets    int  `json:"dcgm_exporter_targets"`
	DCGMExporterUpTargets  int  `json:"dcgm_exporter_up_targets"`

	// Informer health
	InformersSynced  bool     `json:"informers_synced"`
	InformersHealthy int      `json:"informers_healthy"`
	InformersTotal   int      `json:"informers_total"`
	StaleResources   []string `json:"stale_resources,omitempty"`

	// API calls
	APICallsTotal       int   `json:"api_calls_total"`
	APICallsFailedCount int   `json:"api_calls_failed"`
	MetricsAPICallMs    int64 `json:"metrics_api_call_ms"`

	// Errors
	ActiveErrorsCount   int      `json:"active_errors_count"`
	ActiveWarningsCount int      `json:"active_warnings_count"`
	ErrorCodes          []string `json:"error_codes,omitempty"`

	// Uptime
	UptimeSeconds int64 `json:"uptime_seconds"`
	StartedAt     int64 `json:"started_at"`

	// Quota (from last backend response)
	QuotaPlanType   string  `json:"quota_plan_type,omitempty"`
	QuotaCPULimit   float64 `json:"quota_cpu_limit,omitempty"`
	QuotaCPUCurrent float64 `json:"quota_cpu_current,omitempty"`
	QuotaIsWithin   bool    `json:"quota_is_within"`

	CollectedAt int64 `json:"collected_at"`
}
